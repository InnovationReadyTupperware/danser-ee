package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/util/pixconv"
)

func TestRecordingColorContractIntegration(t *testing.T) {
	if os.Getenv("DANSER_RECORDING_INTEGRATION") != "1" {
		t.Skip("set DANSER_RECORDING_INTEGRATION=1 to exercise bundled FFmpeg")
	}
	env.Init("danser")

	output := filepath.Join(t.TempDir(), "color-contract.mp4")
	config := &RecordingSessionConfig{
		width: 64, height: 64, outputFPS: 60,
		encoder: "libx264", audioCodec: "aac", outputFormat: "yuv420p",
		parsedFormat: pixconv.I420, audioSampleRate: 48_000, audioChannels: 2,
	}
	cmd, err := prepareFFmpeg("ffmpeg", buildRecordingProbeArgs(config, output)...)
	if err != nil {
		t.Fatal(err)
	}
	if err = runFFmpegCommand(cmd, new(diagnosticTail)); err != nil {
		t.Fatal(err)
	}

	probe, err := prepareFFmpeg(
		"ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=color_range,color_space,color_transfer,color_primaries",
		"-of", "default=nw=1", output,
	)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := probe.Output()
	if err != nil {
		t.Fatal(err)
	}
	got := string(metadata)
	for _, want := range []string{
		"color_range=tv",
		"color_space=bt709",
		"color_transfer=bt709",
		"color_primaries=bt709",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ffprobe metadata %q does not contain %q", got, want)
		}
	}
}
