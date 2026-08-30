package ffmpeg

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/innovationreadytupperware/danser-ee/framework/bass"
	"github.com/innovationreadytupperware/danser-ee/framework/files"
)

const MaxAudioBuffers = 2000

var (
	cmdAudio         *exec.Cmd
	audioPipe        io.WriteCloser
	audioPool        chan []byte
	audioWriteQueue  chan []byte
	audioWriteGroup  sync.WaitGroup
	audioFailure     stickyError
	audioDiagnostics *diagnosticTail
	audioFrameBytes  int
	audioBufferBytes int
)

func startAudio(config *RecordingSessionConfig) error {
	audioFailure = stickyError{}
	audioDiagnostics = new(diagnosticTail)

	inputName := "-"
	if runtime.GOOS != "windows" {
		pipe, err := files.NewNamedPipe(sessionDir, "")
		if err != nil {
			return fmt.Errorf("create audio input pipe: %w", err)
		}

		inputName = pipe.Path()
		audioPipe = pipe
	}

	options := []string{
		"-y",
		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ar", strconv.Itoa(config.audioSampleRate),
		"-ac", strconv.Itoa(config.audioChannels),
		"-i", inputName,
		"-nostats",
		"-vn",
	}

	if config.audioFilters != "" {
		options = append(options, "-af", config.audioFilters)
	}
	options = append(options, "-c:a", config.audioCodec, "-strict", "-2")
	options = append(options, config.audioOptions...)
	options = append(options, filepath.Join(sessionDir, "audio."+config.container))

	log.Println("Recorder: Running audio FFmpeg with options:", options)

	var err error
	cmdAudio, err = prepareFFmpeg("ffmpeg", options...)
	if err != nil {
		closeAudioPipe()
		return fmt.Errorf("prepare audio encoder: %w", err)
	}
	if runtime.GOOS == "windows" {
		audioPipe, err = cmdAudio.StdinPipe()
		if err != nil {
			return fmt.Errorf("create audio encoder stdin: %w", err)
		}
	}
	cmdAudio.Stdout, cmdAudio.Stderr = diagnosticWriters(audioDiagnostics)

	if err = cmdAudio.Start(); err != nil {
		closeAudioPipe()
		return fmt.Errorf("start audio encoder %s: %w", commandDescription(cmdAudio), err)
	}

	blockRate := max(config.audioBlockRate, 1000)
	audioBufSize := bass.GetMixerRequiredBufferSize(1 / blockRate)
	if audioBufSize <= 0 {
		closeErr := closeAudioPipe()
		waitErr := cmdAudio.Wait()
		return errors.Join(fmt.Errorf("audio mixer returned invalid buffer size %d", audioBufSize), closeErr, waitErr)
	}
	audioFrameBytes = config.audioChannels * config.audioBytesPerSample
	if audioFrameBytes <= 0 || audioBufSize < audioFrameBytes {
		closeErr := closeAudioPipe()
		waitErr := cmdAudio.Wait()
		return errors.Join(fmt.Errorf("audio frame size %d is invalid for buffer %d", audioFrameBytes, audioBufSize), closeErr, waitErr)
	}
	audioBufferBytes = audioBufSize

	audioPool = make(chan []byte, MaxAudioBuffers)
	for range MaxAudioBuffers {
		audioPool <- make([]byte, audioBufSize)
	}
	audioWriteQueue = make(chan []byte, MaxAudioBuffers)

	audioWriteGroup.Go(func() {
		writeFailed := false
		for data := range audioWriteQueue {
			if !writeFailed {
				if _, writeErr := audioPipe.Write(data); writeErr != nil {
					audioFailure.set(fmt.Errorf("write audio encoder input: %w", writeErr))
					writeFailed = true
					_ = audioPipe.Close()
				}
			}

			audioPool <- data[:cap(data)]
		}
	})

	return nil
}

func stopAudio() error {
	if cmdAudio == nil {
		return nil
	}

	log.Println("Recorder: Waiting for audio encoder input to finish")
	close(audioWriteQueue)
	audioWriteGroup.Wait()

	closeErr := closeAudioPipe()
	waitErr := cmdAudio.Wait()
	if waitErr != nil {
		waitErr = fmt.Errorf("wait for audio encoder: %w", waitErr)
	}

	cmdAudio = nil
	stageErr := errors.Join(audioFailure.get(), closeErr, waitErr)
	if stageErr != nil {
		return withDiagnostics("audio encoder failed", stageErr, audioDiagnostics)
	}

	return nil
}

func closeAudioPipe() error {
	if audioPipe == nil {
		return nil
	}

	err := audioPipe.Close()
	audioPipe = nil
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("close audio encoder input: %w", err)
	}

	return nil
}

// PushAudio submits the next mixer block to the audio encoder.
func PushAudio() error {
	return pushAudioBytes(0)
}

// PushAudioFrames submits an exact number of interleaved mixer sample frames.
func PushAudioFrames(sampleFrames int) error {
	if sampleFrames <= 0 {
		return nil
	}
	if audioFrameBytes <= 0 || audioBufferBytes < audioFrameBytes {
		return errors.New("audio encoder buffer is not initialized")
	}
	if sampleFrames > math.MaxInt/audioFrameBytes {
		return fmt.Errorf("audio sample-frame count %d overflows buffer size", sampleFrames)
	}
	maxFrames := audioBufferBytes / audioFrameBytes
	for sampleFrames > 0 {
		frames := min(sampleFrames, maxFrames)
		if err := pushAudioBytes(frames * audioFrameBytes); err != nil {
			return err
		}
		sampleFrames -= frames
	}
	return nil
}

func pushAudioBytes(byteCount int) error {
	if err := audioFailure.get(); err != nil {
		return err
	}

	data := <-audioPool
	data = data[:cap(data)]
	if err := audioFailure.get(); err != nil {
		audioPool <- data
		return err
	}
	if byteCount > len(data) {
		audioPool <- data
		return fmt.Errorf("audio block requires %d bytes, buffer holds %d", byteCount, len(data))
	}
	if byteCount > 0 {
		data = data[:byteCount]
	}

	bass.ProcessMixer(data)
	audioWriteQueue <- data

	return audioFailure.get()
}
