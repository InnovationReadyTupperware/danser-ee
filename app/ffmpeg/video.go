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

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
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

func createPBO(format pixconv.PixFmt) *PBO {
	pbo := new(PBO)
	pbo.convFormat = format

	glSize := w * h * 3
	if pbo.convFormat == pixconv.I420 || pbo.convFormat == pixconv.NV12 {
		glSize = w * h * 3 / 2
	}

	gl.CreateBuffers(1, &pbo.handle)
	gl.NamedBufferStorage(pbo.handle, glSize, gl.Ptr(nil), gl.MAP_PERSISTENT_BIT|gl.MAP_COHERENT_BIT|gl.MAP_READ_BIT)
	pbo.memPointer = gl.MapNamedBufferRange(pbo.handle, 0, glSize, gl.MAP_PERSISTENT_BIT|gl.MAP_COHERENT_BIT|gl.MAP_READ_BIT)
	pbo.data = unsafe.Slice((*byte)(pbo.memPointer), glSize)

	return pbo
}

func startVideo(fps, width, height int) error {
	w, h = width, height
	frameNumber = -1
	frameReadQueue = frameReadQueue[:0]
	videoFailure = stickyError{}
	videoDiagnostics = new(diagnosticTail)
	videoWriteGroup = sync.WaitGroup{}

	if settings.Recording.MotionBlur.Enabled {
		fps /= settings.Recording.MotionBlur.OversampleMultiplier
	}

	encoder := strings.ToLower(settings.Recording.Encoder)
	outputFormat := strings.ToLower(settings.Recording.PixelFormat)
	if strings.HasSuffix(encoder, "_qsv") {
		outputFormat = "nv12"
	} else if encoder == "libsvtav1" {
		outputFormat = "yuv420p"
	}

	parsedFormat = pixconv.ARGB
	switch outputFormat {
	case "yuv420p":
		parsedFormat = pixconv.I420
	case "yuv444p":
		parsedFormat = pixconv.I444
	case "nv12":
		parsedFormat = pixconv.NV12
	}

	var filters []string
	inputPixFmt := "rgb24"
	if parsedFormat != pixconv.ARGB {
		inputPixFmt = outputFormat
	} else {
		filters = append(filters, "vflip")
	}
	if videoFilters := strings.TrimSpace(settings.Recording.Filters); videoFilters != "" {
		filters = append(filters, videoFilters)
	}

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
		"-r", strconv.Itoa(fps),
	}
	if inputPixFmt != "rgb24" {
		options = append(options,
			"-color_range", "1",
			"-colorspace", "1",
			"-color_trc", "1",
			"-color_primaries", "1",
		)
	}
	options = append(options, "-i", inputName)
	if len(filters) > 0 {
		options = append(options, "-vf", strings.Join(filters, ","))
	}
	options = append(options,
		"-c:v", encoder,
		"-color_range", "1",
		"-colorspace", "1",
		"-color_trc", "1",
		"-color_primaries", "1",
		"-movflags", "+write_colr",
	)
	if parsedFormat == pixconv.ARGB {
		options = append(options, "-pix_fmt", outputFormat)
	}

	encOptions, err := settings.Recording.GetEncoderOptions().GenerateFFmpegArgs()
	if err != nil {
		closeVideoPipe()
		return fmt.Errorf("video encoder %q options: %w", encoder, err)
	}
	options = append(options, encOptions...)
	options = append(options, filepath.Join(sessionDir, "video."+settings.Recording.Container))

	log.Println("Recorder: Running video FFmpeg with options:", options)
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

	freePBOPool = make(chan *PBO, MaxVideoBuffers)
	videoWriteQueue = make(chan *PBO, MaxVideoBuffers)
	limiter = frame.NewLimiter(settings.Recording.EncodingFPSCap)

	goroutines.CallMain(func() {
		if parsedFormat != pixconv.ARGB {
			rgbToYuvConverter = effects.NewRGBYUV(w, h, parsedFormat != pixconv.I444 && parsedFormat != pixconv.I422)
		}
		for range MaxVideoBuffers {
			freePBOPool <- createPBO(parsedFormat)
		}
		if settings.Recording.MotionBlur.Enabled {
			bFrames := settings.Recording.MotionBlur.BlendFrames
			blend = effects.NewBlend(w, h, bFrames, calculateWeights(bFrames))
		}
	})

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
	if settings.Recording.MotionBlur.Enabled {
		blend.Begin()
	} else if rgbToYuvConverter != nil {
		rgbToYuvConverter.Begin()
	}
}

// MakeFrame reads back and submits one rendered frame when it is due.
func MakeFrame() error {
	if err := videoFailure.get(); err != nil {
		return err
	}

	frameNumber++
	if settings.Recording.MotionBlur.Enabled {
		blend.End()
		if frameNumber%int64(settings.Recording.MotionBlur.OversampleMultiplier) != 0 {
			return nil
		}
		if rgbToYuvConverter != nil {
			rgbToYuvConverter.Begin()
		}
		blend.Blend()
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
