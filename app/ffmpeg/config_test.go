package ffmpeg

import (
	"math"
	"strings"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/util/pixconv"
)

func TestBuildRecordingConfigValidatesSessionInvariants(t *testing.T) {
	original := *settings.Recording
	originalMotionBlur := *settings.Recording.MotionBlur
	t.Cleanup(func() {
		restoredMotionBlur := originalMotionBlur
		original.MotionBlur = &restoredMotionBlur
		*settings.Recording = original
	})
	settings.Recording.OutputDir = t.TempDir()
	settings.Recording.FrameWidth = 1920
	settings.Recording.FrameHeight = 1080
	settings.Recording.FPS = 60
	settings.Recording.PixelFormat = "yuv420p"
	settings.Recording.MotionBlur.Enabled = true
	settings.Recording.MotionBlur.OversampleMultiplier = 16
	settings.Recording.MotionBlur.BlendFrames = 24

	config, err := buildRecordingConfig(RecordingSessionRequest{Output: "safe", Width: 1920, Height: 1080})
	if err != nil {
		t.Fatal(err)
	}
	if config.sourceFPS != 960 || config.outputFPS != 60 {
		t.Fatalf("rates = source %d/output %d, want 960/60", config.sourceFPS, config.outputFPS)
	}
	if config.pboCount < minPBOCount || config.pboCount > maxPBOCount {
		t.Fatalf("PBO count = %d, want %d..%d", config.pboCount, minPBOCount, maxPBOCount)
	}
	if config.estimatedBytes > recorderMemoryLimit {
		t.Fatalf("estimated memory = %d, exceeds limit", config.estimatedBytes)
	}

	tests := []struct {
		name   string
		mutate func()
		width  int
		height int
		match  string
	}{
		{name: "zero FPS", mutate: func() { settings.Recording.FPS = 0 }, width: 1920, height: 1080, match: "FPS must be positive"},
		{name: "odd 420 dimensions", mutate: func() {}, width: 1919, height: 1080, match: "requires even dimensions"},
		{name: "zero oversample", mutate: func() { settings.Recording.MotionBlur.OversampleMultiplier = 0 }, width: 1920, height: 1080, match: "oversample multiplier"},
		{name: "source rate above clock", mutate: func() { settings.Recording.FPS = 100; settings.Recording.MotionBlur.OversampleMultiplier = 512 }, width: 1920, height: 1080, match: "output-clock resolution"},
		{name: "unsupported format", mutate: func() { settings.Recording.PixelFormat = "p010le" }, width: 1920, height: 1080, match: "unsupported recording pixel format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoredMotionBlur := originalMotionBlur
			original.MotionBlur = &restoredMotionBlur
			*settings.Recording = original
			settings.Recording.OutputDir = t.TempDir()
			settings.Recording.MotionBlur.Enabled = true
			tt.mutate()
			_, err := buildRecordingConfig(RecordingSessionRequest{Output: "safe", Width: tt.width, Height: tt.height})
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("error = %v, want text %q", err, tt.match)
			}
		})
	}
}

func TestEstimateRecorderMemoryRejectsExtremeBlurHistory(t *testing.T) {
	frameBytes, err := packedFrameBytes(7680, 4320, pixconv.I420)
	if err != nil {
		t.Fatal(err)
	}
	estimate, err := estimateRecorderMemory(7680, 4320, frameBytes, minPBOCount, 512, pixconv.I420)
	if err != nil {
		t.Fatal(err)
	}
	if estimate <= recorderMemoryLimit {
		t.Fatalf("estimate = %d, want above %d", estimate, recorderMemoryLimit)
	}
}

func TestCalculateWeightsOneFrameIsFiniteIdentity(t *testing.T) {
	weights := calculateWeights(1, 27, 1.5)
	if len(weights) != 1 || weights[0] != 1 || math.IsNaN(float64(weights[0])) {
		t.Fatalf("weights = %#v, want [1]", weights)
	}
}

func TestBuildRecordingProbeArgsUsesExactSessionContract(t *testing.T) {
	config := &RecordingSessionConfig{
		width: 1920, height: 1080, outputFPS: 60,
		encoder: "h264_nvenc", audioCodec: "aac", outputFormat: "yuv420p",
		videoFilters: "scale=1280:720", audioFilters: "volume=0.5",
		videoOptions: []string{"-preset", "p7"}, audioOptions: []string{"-b:a", "256k"},
		audioSampleRate: 48_000, audioChannels: 2,
	}
	args := buildRecordingProbeArgs(config, `C:\output\probe.mp4`)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"h264_nvenc", "aac", "yuv420p", "scale=1280:720," + bt709SetParams, "volume=0.5", "-preset p7", "-b:a 256k",
		"-color_range:v tv", "-colorspace:v bt709", "-color_trc:v bt709", "-color_primaries:v bt709", "+write_colr",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("probe arguments %q do not contain %q", joined, want)
		}
	}
	if got := args[len(args)-1]; got != `C:\output\probe.mp4` {
		t.Fatalf("probe output = %q", got)
	}
}
