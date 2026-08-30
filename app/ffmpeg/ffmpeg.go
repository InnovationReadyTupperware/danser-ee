package ffmpeg

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
)

const diagnosticTailLimit = 64 * 1024

var prepareFFmpeg = platform.PrepareFFMpeg

var (
	outputRoot      string
	sessionDir      string
	finalOutputPath string
)

type stickyError struct {
	mu  sync.Mutex
	err error
}

func (s *stickyError) set(err error) {
	if err == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err == nil {
		s.err = err
	}
}

func (s *stickyError) get() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}

type diagnosticTail struct {
	mu   sync.Mutex
	data []byte
}

func (t *diagnosticTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.data = append(t.data, p...)
	if len(t.data) > diagnosticTailLimit {
		t.data = append(t.data[:0], t.data[len(t.data)-diagnosticTailLimit:]...)
	}

	return len(p), nil
}

func (t *diagnosticTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return strings.TrimSpace(string(t.data))
}

func diagnosticWriters(tail *diagnosticTail) (io.Writer, io.Writer) {
	stdout := io.Writer(tail)
	stderr := io.Writer(tail)
	if settings.Recording.ShowFFmpegLogs {
		stdout = io.MultiWriter(tail, os.Stdout)
		stderr = io.MultiWriter(tail, os.Stderr)
	}

	return stdout, stderr
}

// ValidateOutputName verifies that name is a file base name, not a path.
// Empty names remain valid because recording startup replaces them with a
// timestamped default.
func ValidateOutputName(name string) error {
	if name == "" {
		return nil
	}
	if strings.TrimSpace(name) != name {
		return errors.New("output name must not start or end with whitespace")
	}
	if name == "." || name == ".." {
		return errors.New("output name must not be . or ..")
	}
	if strings.HasSuffix(name, ".") {
		return errors.New("output name must not end with a dot")
	}
	if filepath.Base(name) != name || filepath.VolumeName(name) != "" || strings.ContainsAny(name, `/\\`) {
		return errors.New("output name must be a base name without path separators")
	}

	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			return fmt.Errorf("output name contains invalid character %q", r)
		}
	}

	deviceName := name
	if before, _, ok := strings.Cut(name, "."); ok {
		deviceName = before
	}
	deviceName = strings.ToUpper(strings.TrimRight(deviceName, " ."))
	if deviceName == "CON" || deviceName == "PRN" || deviceName == "AUX" || deviceName == "NUL" ||
		isReservedNumberedDevice(deviceName) {
		return fmt.Errorf("output name uses reserved Windows device %q", deviceName)
	}

	return nil
}

func isReservedNumberedDevice(name string) bool {
	if !strings.HasPrefix(name, "COM") && !strings.HasPrefix(name, "LPT") {
		return false
	}

	suffix := strings.TrimPrefix(strings.TrimPrefix(name, "COM"), "LPT")
	return suffix == "1" || suffix == "2" || suffix == "3" || suffix == "4" || suffix == "5" ||
		suffix == "6" || suffix == "7" || suffix == "8" || suffix == "9" ||
		suffix == "¹" || suffix == "²" || suffix == "³"
}

func validateContainer(container string) error {
	switch container {
	case "mp4", "mkv":
		return nil
	default:
		return fmt.Errorf("unsupported recording container %q", container)
	}
}

func preCheck() error {
	cmd, err := prepareFFmpeg("ffmpeg", "-encoders")
	if err != nil {
		return fmt.Errorf("prepare FFmpeg encoder probe: %w", err)
	}

	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "127") || strings.Contains(strings.ToLower(err.Error()), "0xc0000135") {
			return fmt.Errorf("FFmpeg installation is incomplete; make sure the required libraries are installed: %w", err)
		}

		return fmt.Errorf("query FFmpeg encoders: %w", err)
	}

	vcodec := settings.Recording.Encoder
	acodec := settings.Recording.AudioCodec
	vfound := false
	afound := false

	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) < 4 || fields[0][3] == 'X' {
			continue
		}

		switch fields[0][0] {
		case 'V':
			vfound = vfound || fields[1] == vcodec
		case 'A':
			afound = afound || fields[1] == acodec
		}
	}

	if !vfound {
		return fmt.Errorf("video codec %q does not exist", vcodec)
	}
	if !afound {
		return fmt.Errorf("audio codec %q does not exist", acodec)
	}

	return nil
}

// StartFFmpeg validates recording output and starts the video and audio
// encoders. Any failure preserves the unique session directory for recovery.
func StartFFmpeg(fps, width, height int, audioFPS float64, requestedOutput string) error {
	outputRoot = ""
	sessionDir = ""
	finalOutputPath = ""

	if requestedOutput == "" {
		requestedOutput = "danser_" + time.Now().Format("2006-01-02_15-04-05")
	}
	if err := ValidateOutputName(requestedOutput); err != nil {
		return recordingError("invalid output name", err)
	}
	if err := validateContainer(settings.Recording.Container); err != nil {
		return recordingError("invalid recording container", err)
	}
	if err := preCheck(); err != nil {
		return recordingError("preflight failed", err)
	}

	root, err := filepath.Abs(settings.Recording.GetOutputDir())
	if err != nil {
		return recordingError("resolve output directory", err)
	}
	if err = os.MkdirAll(root, 0o755); err != nil {
		return recordingError("create output directory", err)
	}

	finalPath := filepath.Join(root, requestedOutput+"."+settings.Recording.Container)
	if _, err = os.Lstat(finalPath); err == nil {
		return recordingError("refusing to overwrite existing output", fmt.Errorf("%q already exists", finalPath))
	} else if !errors.Is(err, os.ErrNotExist) {
		return recordingError("inspect final output", err)
	}

	tempDir, err := os.MkdirTemp(root, ".danser-recording-")
	if err != nil {
		return recordingError("create recording session directory", err)
	}

	outputRoot = root
	sessionDir = tempDir
	finalOutputPath = finalPath

	log.Printf("Recorder: Starting encoding session in %q", sessionDir)

	if err = startVideo(fps, width, height); err != nil {
		return recordingError("start video encoder", err)
	}
	if err = startAudio(audioFPS); err != nil {
		videoErr := stopVideo()
		return recordingError("start audio encoder", errors.Join(err, videoErr))
	}

	return nil
}

// StopFFmpeg finalizes both encoders and publishes the completed recording.
// It returns the final path only after publication succeeds.
func StopFFmpeg() (string, error) {
	log.Println("Recorder: Finishing rendering...")

	videoErr := stopVideo()
	audioErr := stopAudio()
	if err := errors.Join(videoErr, audioErr); err != nil {
		return "", recordingError("encoder finalization failed", err)
	}

	if err := verifyIntermediate("video"); err != nil {
		return "", recordingError("video intermediate is unusable", err)
	}
	if err := verifyIntermediate("audio"); err != nil {
		return "", recordingError("audio intermediate is unusable", err)
	}

	stagedOutput, err := combine()
	if err != nil {
		return "", recordingError("mux failed", err)
	}
	if err = publishRecording(stagedOutput, finalOutputPath); err != nil {
		return "", recordingError("publish final output", err)
	}

	log.Println("Video is available at:", finalOutputPath)

	if err = cleanupSession(); err != nil {
		log.Printf("Recorder: Warning: final output is complete, but session cleanup failed for %q: %v", sessionDir, err)
	}

	return finalOutputPath, nil
}

func verifyIntermediate(kind string) error {
	path := filepath.Join(sessionDir, kind+"."+settings.Recording.Container)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("%s is not a non-empty regular file", path)
	}

	return nil
}

func combine() (string, error) {
	stagedOutput := filepath.Join(sessionDir, "final."+settings.Recording.Container)
	options := []string{
		"-y",
		"-i", filepath.Join(sessionDir, "video."+settings.Recording.Container),
		"-i", filepath.Join(sessionDir, "audio."+settings.Recording.Container),
		"-c:v", "copy",
		"-c:a", "copy", "-strict", "-2",
	}

	if settings.Recording.Container == "mp4" {
		options = append(options, "-movflags", "+faststart")
	}
	options = append(options, stagedOutput)

	log.Println("Recorder: Starting audio/video mux")
	log.Println("Recorder: Running FFmpeg with options:", options)

	cmd, err := prepareFFmpeg("ffmpeg", options...)
	if err != nil {
		return "", fmt.Errorf("prepare mux process: %w", err)
	}

	tail := new(diagnosticTail)
	if err = runFFmpegCommand(cmd, tail); err != nil {
		return "", fmt.Errorf("run mux process: %w", err)
	}

	info, err := os.Stat(stagedOutput)
	if err != nil {
		return "", fmt.Errorf("inspect staged output: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", fmt.Errorf("staged output %q is empty or not a regular file", stagedOutput)
	}

	return stagedOutput, nil
}

func cleanupSession() error {
	rel, err := filepath.Rel(outputRoot, sessionDir)
	if err != nil {
		return fmt.Errorf("resolve session path: %w", err)
	}
	if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("session directory %q is not a child of output root %q", sessionDir, outputRoot)
	}

	return os.RemoveAll(sessionDir)
}

func recordingError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if sessionDir == "" {
		return fmt.Errorf("Recorder: %s: %w", operation, err)
	}

	return fmt.Errorf("Recorder: %s: %w; intermediate files retained in %q; recovery mux: %s", operation, err, sessionDir, recoveryCommand())
}

func recoveryCommand() string {
	if sessionDir == "" {
		return "unavailable"
	}

	args := []string{
		"ffmpeg", "-n",
		"-i", filepath.Join(sessionDir, "video."+settings.Recording.Container),
		"-i", filepath.Join(sessionDir, "audio."+settings.Recording.Container),
		"-c:v", "copy", "-c:a", "copy", "-strict", "-2",
	}
	if settings.Recording.Container == "mp4" {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, finalOutputPath)

	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = strconv.Quote(arg)
	}

	return strings.Join(quoted, " ")
}

func withDiagnostics(operation string, err error, tail *diagnosticTail) error {
	diagnostics := tail.String()
	if diagnostics == "" {
		return fmt.Errorf("%s: %w", operation, err)
	}

	return fmt.Errorf("%s: %w; FFmpeg output: %s", operation, err, diagnostics)
}

func runFFmpegCommand(cmd *exec.Cmd, tail *diagnosticTail) error {
	cmd.Stdout, cmd.Stderr = diagnosticWriters(tail)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return withDiagnostics("wait for process", err, tail)
	}

	return nil
}

func commandDescription(cmd *exec.Cmd) string {
	quoted := make([]string, len(cmd.Args))
	for i, arg := range cmd.Args {
		quoted[i] = strconv.Quote(arg)
	}

	return strings.Join(quoted, " ")
}
