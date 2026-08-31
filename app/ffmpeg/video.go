package ffmpeg

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/go-gl/gl/v4.5-core/gl"

	"github.com/innovationreadytupperware/danser-ee/framework/files"
	"github.com/innovationreadytupperware/danser-ee/framework/frame"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/effects"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/util/pixconv"
)

const MaxVideoBuffers = 10

var (
	cmdVideo          *exec.Cmd
	videoPipe         io.WriteCloser
	videoWriteQueue   chan *PBO
	videoWriteGroup   sync.WaitGroup
	videoFailure      stickyError
	videoDiagnostics  *diagnosticTail
	freePBOPool       chan *PBO
	frameReadQueue    = make([]*PBO, 0)
	blend             *effects.Blend
	w, h              int
	limiter           *frame.Limiter
	parsedFormat      pixconv.PixFmt
	rgbToYuvConverter *effects.RGBYUV
	frameNumber       int64 = -1
)

type PBO struct {
	handle     uint32
	memPointer unsafe.Pointer
	data       []byte
	convFormat pixconv.PixFmt
	sync       uintptr
}

func createPBO(format pixconv.PixFmt) (*PBO, error) {
	pbo := new(PBO)
	pbo.convFormat = format

	glSize := activeConfig.frameBytes

	gl.CreateBuffers(1, &pbo.handle)
	if pbo.handle == 0 {
		return nil, errors.New("OpenGL did not create a recording pixel buffer")
	}
	gl.NamedBufferStorage(pbo.handle, glSize, gl.Ptr(nil), gl.MAP_PERSISTENT_BIT|gl.MAP_COHERENT_BIT|gl.MAP_READ_BIT)
	pbo.memPointer = gl.MapNamedBufferRange(pbo.handle, 0, glSize, gl.MAP_PERSISTENT_BIT|gl.MAP_COHERENT_BIT|gl.MAP_READ_BIT)
	if pbo.memPointer == nil {
		gl.DeleteBuffers(1, &pbo.handle)
		return nil, fmt.Errorf("map %d-byte recording pixel buffer", glSize)
	}
	pbo.data = unsafe.Slice((*byte)(pbo.memPointer), glSize)

	return pbo, nil
}

func startVideo(config *RecordingSessionConfig) error {
	w, h = config.width, config.height
	frameNumber = -1
	frameReadQueue = frameReadQueue[:0]
	videoFailure = stickyError{}
	videoDiagnostics = new(diagnosticTail)
	videoWriteGroup = sync.WaitGroup{}

	encoder := config.encoder
	outputFormat := config.outputFormat
	parsedFormat = config.parsedFormat

	var filters []string
	inputPixFmt := config.inputPixelFormat
	if parsedFormat != pixconv.ARGB {
	} else {
		filters = append(filters, "vflip")
	}
	if config.videoFilters != "" {
		filters = append(filters, config.videoFilters)
	}
	filters = append(filters, bt709SetParams)

	inputName := "-"
	if runtime.GOOS != "windows" {
		pipe, err := files.NewNamedPipe(sessionDir, "")
		if err != nil {
			return fmt.Errorf("create video input pipe: %w", err)
		}

		inputName = pipe.Path()
		videoPipe = pipe
	}

	options := []string{
		"-y",
		"-an",
		"-f", "rawvideo",
		"-c:v", "rawvideo",
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-pix_fmt", inputPixFmt,
		"-framerate", strconv.Itoa(config.outputFPS),
	}
	if inputPixFmt != "rgb24" {
		options = append(options,
			"-color_range", "tv",
			"-colorspace", "bt709",
			"-color_trc", "bt709",
			"-color_primaries", "bt709",
		)
	}
	options = append(options, "-i", inputName)
	if len(filters) > 0 {
		options = append(options, "-vf", strings.Join(filters, ","))
	}
	options = append(options,
		"-c:v", encoder,
		"-color_range:v", "tv",
		"-colorspace:v", "bt709",
		"-color_trc:v", "bt709",
		"-color_primaries:v", "bt709",
		"-movflags", "+write_colr",
	)
	if parsedFormat == pixconv.ARGB {
		options = append(options, "-pix_fmt", outputFormat)
	}

	options = append(options, config.videoOptions...)
	options = append(options, filepath.Join(sessionDir, "video."+config.container))

	log.Println("Recorder: Running video FFmpeg with options:", options)
	var err error
	cmdVideo, err = prepareFFmpeg("ffmpeg", options...)
	if err != nil {
		closeVideoPipe()
		return fmt.Errorf("prepare video encoder: %w", err)
	}
	if runtime.GOOS == "windows" {
		videoPipe, err = cmdVideo.StdinPipe()
		if err != nil {
			return fmt.Errorf("create video encoder stdin: %w", err)
		}
	}
	cmdVideo.Stdout, cmdVideo.Stderr = diagnosticWriters(videoDiagnostics)

	freePBOPool = make(chan *PBO, config.pboCount)
	videoWriteQueue = make(chan *PBO, config.pboCount)
	limiter = frame.NewLimiter(config.encodingFPSCap)

	var resourceErr error
	goroutines.CallMain(func() {
		if parsedFormat != pixconv.ARGB {
			rgbToYuvConverter = effects.NewRGBYUV(w, h, parsedFormat != pixconv.I444 && parsedFormat != pixconv.I422)
		}
		for range config.pboCount {
			pbo, err := createPBO(parsedFormat)
			if err != nil {
				resourceErr = err
				break
			}
			freePBOPool <- pbo
		}
		if config.motionBlur {
			bFrames := config.blendFrames
			blend = effects.NewBlend(w, h, bFrames, calculateWeights(bFrames, config.blendFunctionID, config.gaussWeightsMult))
		}
	})
	if resourceErr != nil {
		closeVideoPipe()
		return fmt.Errorf("allocate recording GPU resources: %w", resourceErr)
	}

	if err = cmdVideo.Start(); err != nil {
		closeVideoPipe()
		return fmt.Errorf("start video encoder %s: %w", commandDescription(cmdVideo), err)
	}

	videoWriteGroup.Go(func() {
		writeFailed := false
		for pbo := range videoWriteQueue {
			if !writeFailed {
				if _, writeErr := videoPipe.Write(pbo.data); writeErr != nil {
					videoFailure.set(fmt.Errorf("write video encoder input: %w", writeErr))
					writeFailed = true
					_ = videoPipe.Close()
				}
			}

			freePBOPool <- pbo
		}
	})

	return nil
}

func stopVideo() error {
	if cmdVideo == nil {
		return nil
	}

	log.Println("Recorder: Waiting for video encoder input to finish")
	readErr := checkData(true, true)
	close(videoWriteQueue)
	videoWriteGroup.Wait()

	closeErr := closeVideoPipe()
	waitErr := cmdVideo.Wait()
	if waitErr != nil {
		waitErr = fmt.Errorf("wait for video encoder: %w", waitErr)
	}

	cmdVideo = nil
	stageErr := errors.Join(readErr, videoFailure.get(), closeErr, waitErr)
	if stageErr != nil {
		return withDiagnostics("video encoder failed", stageErr, videoDiagnostics)
	}

	return nil
}

func closeVideoPipe() error {
	if videoPipe == nil {
		return nil
	}

	err := videoPipe.Close()
	videoPipe = nil
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("close video encoder input: %w", err)
	}

	return nil
}

func PreFrame() {
	if activeConfig.motionBlur {
		blend.Begin()
	} else if rgbToYuvConverter != nil {
		rgbToYuvConverter.Begin()
	}
}

// MakeFrame reads back and submits one rendered frame when it is due.
func MakeFrame() error {
	nextFrame := frameNumber + 1
	emit := !activeConfig.motionBlur || nextFrame%int64(activeConfig.oversample) == 0
	return makeFrame(emit)
}

// MakeFrameDue submits the rendered source sample and emits an encoded frame
// only when the rational recording timeline marks one due.
func MakeFrameDue(emit bool) error {
	return makeFrame(emit)
}

func makeFrame(emit bool) error {
	if err := videoFailure.get(); err != nil {
		return err
	}

	frameNumber++
	if activeConfig.motionBlur {
		blend.End()
		if !emit {
			return nil
		}
		if rgbToYuvConverter != nil {
			rgbToYuvConverter.Begin()
		}
		blend.Blend()
	} else if !emit {
		return nil
	}

	var yuvFull, yuvHalf []texture.Texture
	if rgbToYuvConverter != nil {
		rgbToYuvConverter.End()
		yuvFull, yuvHalf = rgbToYuvConverter.Draw()
	}

	if err := checkData(len(freePBOPool) == 0, false); err != nil {
		return err
	}
	pbo := <-freePBOPool
	if err := videoFailure.get(); err != nil {
		freePBOPool <- pbo
		return err
	}

	gl.BindBuffer(gl.PIXEL_PACK_BUFFER, pbo.handle)
	gl.PixelStorei(gl.PACK_ALIGNMENT, 1)

	if pbo.convFormat == pixconv.NV12 {
		gl.GetTextureSubImage(yuvFull[0].GetID(), 0, 0, 0, 0, int32(w), int32(h), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h), gl.Ptr(nil))
		gl.GetTextureSubImage(yuvHalf[0].GetID(), 0, 0, 0, 0, int32(w/2), int32(h/2), 1, gl.RG, gl.UNSIGNED_BYTE, int32(w*h/2), gl.PtrOffset(w*h))
	} else if pbo.convFormat == pixconv.I420 {
		gl.GetTextureSubImage(yuvFull[0].GetID(), 0, 0, 0, 0, int32(w), int32(h), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h), gl.Ptr(nil))
		gl.GetTextureSubImage(yuvHalf[0].GetID(), 0, 0, 0, 0, int32(w/2), int32(h/2), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h/4), gl.PtrOffset(w*h))
		gl.GetTextureSubImage(yuvHalf[1].GetID(), 0, 0, 0, 0, int32(w/2), int32(h/2), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h/4), gl.PtrOffset(w*h*5/4))
	} else if pbo.convFormat != pixconv.ARGB {
		gl.GetTextureSubImage(yuvFull[0].GetID(), 0, 0, 0, 0, int32(w), int32(h), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h), gl.Ptr(nil))
		gl.GetTextureSubImage(yuvFull[1].GetID(), 0, 0, 0, 0, int32(w), int32(h), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h), gl.PtrOffset(w*h))
		gl.GetTextureSubImage(yuvFull[2].GetID(), 0, 0, 0, 0, int32(w), int32(h), 1, gl.RED, gl.UNSIGNED_BYTE, int32(w*h), gl.PtrOffset(w*h*2))
	} else {
		gl.ReadPixels(0, 0, int32(w), int32(h), uint32(gl.RGB), gl.UNSIGNED_BYTE, gl.Ptr(nil))
	}

	pbo.sync = gl.FenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0)
	if pbo.sync == 0 {
		freePBOPool <- pbo
		return errors.New("create recording GPU fence")
	}
	gl.Flush()
	frameReadQueue = append(frameReadQueue, pbo)

	if err := checkData(false, false); err != nil {
		return err
	}
	limiter.Sync()

	return videoFailure.get()
}

func checkData(waitForFirst, waitForAll bool) error {
	for i := 0; len(frameReadQueue) > 0; i++ {
		pbo := frameReadQueue[0]
		status := int32(gl.SIGNALED)

		if i == 0 && waitForFirst || waitForAll {
			for {
				iStat := gl.ClientWaitSync(pbo.sync, 0, gl.TIMEOUT_IGNORED)
				if iStat == gl.ALREADY_SIGNALED || iStat == gl.CONDITION_SATISFIED {
					break
				}
				if iStat == gl.WAIT_FAILED {
					return errors.New("wait for recording GPU fence")
				}
			}
		} else {
			gl.GetSynciv(pbo.sync, gl.SYNC_STATUS, 1, nil, &status)
		}

		if status != gl.SIGNALED {
			return videoFailure.get()
		}

		gl.DeleteSync(pbo.sync)
		frameReadQueue = frameReadQueue[1:]
		if videoFailure.get() != nil {
			freePBOPool <- pbo
		} else {
			videoWriteQueue <- pbo
		}
	}

	return videoFailure.get()
}
