package ffmpeg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOutputName(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{name: "generated default", output: ""},
		{name: "plain", output: "ranked run"},
		{name: "unicode", output: "試合"},
		{name: "dot within name", output: "run.v2"},
		{name: "parent path", output: `..\outside`, wantErr: true},
		{name: "forward path", output: "nested/out", wantErr: true},
		{name: "dot", output: ".", wantErr: true},
		{name: "dot dot", output: "..", wantErr: true},
		{name: "leading whitespace", output: " run", wantErr: true},
		{name: "trailing whitespace", output: "run ", wantErr: true},
		{name: "trailing dot", output: "run.", wantErr: true},
		{name: "invalid character", output: "run*", wantErr: true},
		{name: "control character", output: "run\x00", wantErr: true},
		{name: "reserved device", output: "CON", wantErr: true},
		{name: "reserved device extension", output: "lpt9.txt", wantErr: true},
		{name: "reserved superscript device", output: "COM¹", wantErr: true},
		{name: "reserved device before extension whitespace", output: "CON .txt", wantErr: true},
		{name: "non-device prefix", output: "console"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputName(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateOutputName(%q) error = %v, wantErr %t", tt.output, err, tt.wantErr)
			}
		})
	}
}

func TestValidateContainer(t *testing.T) {
	for _, container := range []string{"mp4", "mkv"} {
		if err := validateContainer(container); err != nil {
			t.Fatalf("validateContainer(%q): %v", container, err)
		}
	}
	if err := validateContainer(`..\outside`); err == nil {
		t.Fatal("validateContainer accepted a path")
	}
}

func TestDiagnosticTailKeepsBoundedSuffix(t *testing.T) {
	tail := new(diagnosticTail)
	prefix := strings.Repeat("a", diagnosticTailLimit)
	suffix := strings.Repeat("z", 128)

	if _, err := tail.Write([]byte(prefix + suffix)); err != nil {
		t.Fatal(err)
	}

	got := tail.String()
	if len(got) != diagnosticTailLimit {
		t.Fatalf("diagnostic length = %d, want %d", len(got), diagnosticTailLimit)
	}
	if !strings.HasSuffix(got, suffix) {
		t.Fatal("diagnostic tail did not retain the newest output")
	}
}

func TestRunFFmpegCommandReportsUnrecognizedLargeDiagnostics(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestFFmpegHelperProcess", "--", "fail-large")
	cmd.Env = append(os.Environ(), "DANSER_FFMPEG_HELPER=1")
	tail := new(diagnosticTail)

	err := runFFmpegCommand(cmd, tail)
	if err == nil {
		t.Fatal("runFFmpegCommand returned nil for a failing process")
	}
	if !strings.Contains(err.Error(), "unrecognized diagnostic tail") {
		t.Fatalf("error does not contain diagnostic tail: %v", err)
	}
	if len(tail.String()) > diagnosticTailLimit {
		t.Fatalf("diagnostic tail exceeded limit: %d", len(tail.String()))
	}
}

func TestRunFFmpegCommandReportsStartFailure(t *testing.T) {
	cmd := exec.Command(filepath.Join(t.TempDir(), "missing-ffmpeg"))
	err := runFFmpegCommand(cmd, new(diagnosticTail))
	if err == nil || !strings.Contains(err.Error(), "start process") {
		t.Fatalf("start failure = %v, want contextual error", err)
	}
}

func TestCleanupSessionRejectsOutputRoot(t *testing.T) {
	oldRoot, oldSession := outputRoot, sessionDir
	t.Cleanup(func() {
		outputRoot, sessionDir = oldRoot, oldSession
	})

	outputRoot = t.TempDir()
	sessionDir = outputRoot
	if err := cleanupSession(); err == nil {
		t.Fatal("cleanupSession accepted the output root")
	}
	if _, err := os.Stat(outputRoot); err != nil {
		t.Fatalf("output root was removed: %v", err)
	}
}

func TestPublishRecordingDoesNotReplaceExistingOutput(t *testing.T) {
	dir := t.TempDir()
	finalPath := filepath.Join(dir, "final.mp4")
	stagedPath := filepath.Join(dir, "staged.mp4")
	if err := os.WriteFile(finalPath, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := publishRecording(stagedPath, finalPath)
	if err == nil {
		t.Fatal("publishRecording replaced an existing output")
	}
	data, readErr := os.ReadFile(finalPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "existing" {
		t.Fatalf("existing output = %q, want unchanged", data)
	}
}

func TestPublishRecordingMovesStagedOutput(t *testing.T) {
	dir := t.TempDir()
	finalPath := filepath.Join(dir, "final.mp4")
	stagedPath := filepath.Join(dir, "staged.mp4")
	if err := os.WriteFile(stagedPath, []byte("complete"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := publishRecording(stagedPath, finalPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "complete" {
		t.Fatalf("published output = %q, want complete", data)
	}
	if _, err = os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged output still exists or returned unexpected error: %v", err)
	}
}

func TestFFmpegHelperProcess(t *testing.T) {
	if os.Getenv("DANSER_FFMPEG_HELPER") != "1" {
		return
	}

	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "fail-large" {
		fmt.Fprintln(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}

	fmt.Fprint(os.Stderr, strings.Repeat("noise", diagnosticTailLimit))
	fmt.Fprintln(os.Stderr, "unrecognized diagnostic tail")
	os.Exit(7)
}

func TestStickyErrorKeepsFirstFailure(t *testing.T) {
	first := errors.New("first")
	var failure stickyError
	failure.set(first)
	failure.set(errors.New("second"))

	if !errors.Is(failure.get(), first) {
		t.Fatalf("sticky error = %v, want first failure", failure.get())
	}
}
