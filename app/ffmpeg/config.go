package ffmpeg

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/bass"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/util/pixconv"
)

const (
	recorderMemoryLimit = uint64(2 * 1024 * 1024 * 1024)
	targetPBOBytes      = uint64(128 * 1024 * 1024)
	minPBOCount         = 3
	maxPBOCount         = 10
	maxAudioSampleRate  = 48_000
	bt709SetParams      = "setparams=range=limited:color_primaries=bt709:color_trc=bt709:colorspace=bt709"
)

// RecordingSessionRequest identifies the output and capture dimensions for
// one recording. SourceFPS is optional and exists for the legacy StartFFmpeg
// compatibility wrapper; normal callers derive it from recording settings.
type RecordingSessionRequest struct {
	Output         string
	Width          int
	Height         int
	SourceFPS      int
	AudioBlockRate float64
}

// RecordingSessionConfig is an immutable snapshot of validated recording
// settings. Its fields remain private so settings reloads cannot alter an
// active encoder session.
type RecordingSessionConfig struct {
	outputName          string
	outputRoot          string
	finalOutputPath     string
	container           string
	width               int
	height              int
	outputFPS           int
	sourceFPS           int
	encodingFPSCap      int
	encoder             string
	audioCodec          string
	outputFormat        string
	inputPixelFormat    string
	parsedFormat        pixconv.PixFmt
	videoFilters        string
	audioFilters        string
	videoOptions        []string
	audioOptions        []string
	audioSampleRate     int
	audioChannels       int
	audioBytesPerSample int
	audioBlockRate      float64
	motionBlur          bool
	oversample          int
	blendFrames         int
	blendFunctionID     int
	gaussWeightsMult    float64
	pboCount            int
	frameBytes          int
	estimatedBytes      uint64
	showFFmpegLogs      bool
}

// EstimatedMemoryBytes returns the recorder-owned allocation estimate used by
// preflight.
func (config *RecordingSessionConfig) EstimatedMemoryBytes() uint64 {
	return config.estimatedBytes
}

// FFmpegLogsEnabled reports whether this session mirrors encoder diagnostics
// to the console in addition to retaining the bounded diagnostic tail.
func (config *RecordingSessionConfig) FFmpegLogsEnabled() bool {
	return config.showFFmpegLogs
}

var probeRecordingConfig = runRecordingProbe

// PrepareRecording validates a complete recording snapshot, checks modern GL
// limits, and runs a short exact FFmpeg probe before persistent resources are
// allocated.
func PrepareRecording(request RecordingSessionRequest) (*RecordingSessionConfig, error) {
	settings.NormalizeRecording()
	config, err := buildRecordingConfig(request)
	if err != nil {
		return nil, err
	}
	if err = validateRecordingGLLimits(config); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(config.outputRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create recording output directory: %w", err)
	}
	if err = probeRecordingConfig(config); err != nil {
		return nil, fmt.Errorf("probe exact recording configuration: %w", err)
	}

	return config, nil
}

func buildRecordingConfig(request RecordingSessionRequest) (*RecordingSessionConfig, error) {
	var validationErrors []error
	if err := ValidateOutputName(request.Output); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("output name: %w", err))
	}
	if request.Width <= 0 || request.Height <= 0 {
		validationErrors = append(validationErrors, fmt.Errorf("capture dimensions must be positive, got %dx%d", request.Width, request.Height))
	}
	if err := validateContainer(settings.Recording.Container); err != nil {
		validationErrors = append(validationErrors, err)
	}

	outputName := request.Output
	if outputName == "" {
		outputName = defaultOutputName()
	}
	root, err := filepath.Abs(settings.Recording.GetOutputDir())
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("resolve output directory: %w", err))
	}

	outputFPS := settings.Recording.FPS
	if outputFPS <= 0 {
		validationErrors = append(validationErrors, fmt.Errorf("recording FPS must be positive, got %d", outputFPS))
	}
	motionBlur := settings.Recording.MotionBlur != nil && settings.Recording.MotionBlur.Enabled
	oversample := 1
	blendFrames := 1
	blendFunctionID := 0
	gaussWeightsMult := 0.0
	if settings.Recording.MotionBlur == nil {
		validationErrors = append(validationErrors, errors.New("motion blur settings are unavailable"))
	} else {
		oversample = settings.Recording.MotionBlur.OversampleMultiplier
		blendFrames = settings.Recording.MotionBlur.BlendFrames
		blendFunctionID = settings.Recording.MotionBlur.BlendFunctionID
		gaussWeightsMult = settings.Recording.MotionBlur.GaussWeightsMult
		if oversample <= 0 {
			validationErrors = append(validationErrors, fmt.Errorf("motion-blur oversample multiplier must be positive, got %d", oversample))
		}
		if blendFrames <= 0 {
			validationErrors = append(validationErrors, fmt.Errorf("motion-blur frame count must be positive, got %d", blendFrames))
		}
		if math.IsNaN(gaussWeightsMult) || math.IsInf(gaussWeightsMult, 0) || gaussWeightsMult < 0 {
			validationErrors = append(validationErrors, fmt.Errorf("motion-blur Gaussian multiplier must be finite and non-negative, got %v", gaussWeightsMult))
		}
	}
	if !motionBlur {
		oversample = 1
		blendFrames = 1
	}

	sourceFPS := request.SourceFPS
	if sourceFPS == 0 && outputFPS > 0 && oversample > 0 {
		if outputFPS > math.MaxInt/oversample {
			validationErrors = append(validationErrors, errors.New("recording source frame rate overflows int"))
		} else {
			sourceFPS = outputFPS * oversample
		}
	}
	if sourceFPS <= 0 {
		validationErrors = append(validationErrors, fmt.Errorf("recording source frame rate must be positive, got %d", sourceFPS))
	} else if sourceFPS > maxAudioSampleRate {
		validationErrors = append(validationErrors, fmt.Errorf("recording source frame rate %d exceeds the %d Hz output-clock resolution", sourceFPS, maxAudioSampleRate))
	}
	audioBlockRate := request.AudioBlockRate
	if audioBlockRate == 0 {
		audioBlockRate = 1000
	}
	if audioBlockRate <= 0 || math.IsNaN(audioBlockRate) || math.IsInf(audioBlockRate, 0) {
		validationErrors = append(validationErrors, fmt.Errorf("audio block rate must be positive and finite, got %v", audioBlockRate))
	}

	encoder := strings.ToLower(strings.TrimSpace(settings.Recording.Encoder))
	audioCodec := strings.ToLower(strings.TrimSpace(settings.Recording.AudioCodec))
	if encoder == "" {
		validationErrors = append(validationErrors, errors.New("video encoder must not be empty"))
	}
	if audioCodec == "" {
		validationErrors = append(validationErrors, errors.New("audio encoder must not be empty"))
	}

	outputFormat := strings.ToLower(strings.TrimSpace(settings.Recording.PixelFormat))
	if strings.HasSuffix(encoder, "_qsv") {
		outputFormat = "nv12"
	} else if encoder == "libsvtav1" {
		outputFormat = "yuv420p"
	}
	parsedFormat := pixconv.ARGB
	inputPixelFormat := "rgb24"
	switch outputFormat {
	case "yuv420p":
		parsedFormat = pixconv.I420
		inputPixelFormat = outputFormat
	case "yuv444p":
		parsedFormat = pixconv.I444
		inputPixelFormat = outputFormat
	case "nv12":
		parsedFormat = pixconv.NV12
		inputPixelFormat = outputFormat
	default:
		validationErrors = append(validationErrors, fmt.Errorf("unsupported recording pixel format %q", outputFormat))
	}
	if (parsedFormat == pixconv.I420 || parsedFormat == pixconv.NV12) && (request.Width%2 != 0 || request.Height%2 != 0) {
		validationErrors = append(validationErrors, fmt.Errorf("pixel format %s requires even dimensions, got %dx%d", outputFormat, request.Width, request.Height))
	}

	videoOptions, videoErr := settings.Recording.GetEncoderOptions().GenerateFFmpegArgs()
	if videoErr != nil {
		validationErrors = append(validationErrors, fmt.Errorf("video encoder options: %w", videoErr))
	}
	audioOptions, audioErr := settings.Recording.GetAudioOptions().GenerateFFmpegArgs()
	if audioErr != nil {
		validationErrors = append(validationErrors, fmt.Errorf("audio encoder options: %w", audioErr))
	}

	outputInfo := bass.GetOutputInfo()
	if outputInfo.SampleRate <= 0 {
		outputInfo.SampleRate = 48_000
	}
	if outputInfo.Channels <= 0 {
		outputInfo.Channels = 2
	}
	if outputInfo.BytesPerSample <= 0 {
		outputInfo.BytesPerSample = 4
	}
	if outputInfo.SampleRate > maxAudioSampleRate {
		validationErrors = append(validationErrors, fmt.Errorf("audio sample rate %d exceeds supported recording clock %d", outputInfo.SampleRate, maxAudioSampleRate))
	}

	frameBytes, frameErr := packedFrameBytes(request.Width, request.Height, parsedFormat)
	if frameErr != nil {
		validationErrors = append(validationErrors, frameErr)
	}
	pboCount := maxPBOCount
	if frameBytes > 0 {
		pboCount = int(targetPBOBytes / uint64(frameBytes))
		pboCount = min(max(pboCount, minPBOCount), maxPBOCount)
	}
	estimatedBytes, memoryErr := estimateRecorderMemory(request.Width, request.Height, frameBytes, pboCount, blendFrames, parsedFormat)
	if memoryErr != nil {
		validationErrors = append(validationErrors, memoryErr)
	} else if estimatedBytes > recorderMemoryLimit {
		validationErrors = append(validationErrors, fmt.Errorf("estimated recorder memory %s exceeds the 2 GiB limit", formatBytes(estimatedBytes)))
	}

	if err := errors.Join(validationErrors...); err != nil {
		return nil, fmt.Errorf("invalid recording configuration: %w", err)
	}

	return &RecordingSessionConfig{
		outputName: outputName, outputRoot: root,
		finalOutputPath: filepath.Join(root, outputName+"."+settings.Recording.Container),
		container:       settings.Recording.Container, width: request.Width, height: request.Height,
		outputFPS: outputFPS, sourceFPS: sourceFPS, encodingFPSCap: settings.Recording.EncodingFPSCap,
		encoder: encoder, audioCodec: audioCodec, outputFormat: outputFormat,
		inputPixelFormat: inputPixelFormat, parsedFormat: parsedFormat,
		videoFilters: strings.TrimSpace(settings.Recording.Filters), audioFilters: strings.TrimSpace(settings.Recording.AudioFilters),
		videoOptions: append([]string(nil), videoOptions...), audioOptions: append([]string(nil), audioOptions...),
		audioSampleRate: outputInfo.SampleRate, audioChannels: outputInfo.Channels, audioBytesPerSample: outputInfo.BytesPerSample,
		audioBlockRate: audioBlockRate,
		motionBlur:     motionBlur, oversample: oversample, blendFrames: blendFrames,
		blendFunctionID: blendFunctionID, gaussWeightsMult: gaussWeightsMult,
		pboCount: pboCount, frameBytes: frameBytes, estimatedBytes: estimatedBytes,
		showFFmpegLogs: settings.Recording.ShowFFmpegLogs,
	}, nil
}

func packedFrameBytes(width, height int, format pixconv.PixFmt) (int, error) {
	if width <= 0 || height <= 0 || width > math.MaxInt/height {
		return 0, errors.New("recording dimensions overflow frame area")
	}
	pixels := width * height
	multiplier := 3
	divisor := 1
	if format == pixconv.I420 || format == pixconv.NV12 {
		multiplier = 3
		divisor = 2
	}
	if pixels > math.MaxInt/multiplier {
		return 0, errors.New("recording dimensions overflow packed frame size")
	}
	return pixels * multiplier / divisor, nil
}

func estimateRecorderMemory(width, height, frameBytes, pboCount, blendFrames int, format pixconv.PixFmt) (uint64, error) {
	if width <= 0 || height <= 0 || frameBytes <= 0 || pboCount <= 0 || blendFrames <= 0 {
		return 0, errors.New("cannot estimate recorder memory from non-positive values")
	}
	pixels := uint64(width) * uint64(height)
	parts := []uint64{
		pixels * 4, // capture renderbuffer
		uint64(frameBytes) * uint64(pboCount),
	}
	if blendFrames > 1 {
		parts = append(parts, pixels*3*uint64(blendFrames))
	}
	if format != pixconv.ARGB {
		parts = append(parts, pixels*4, pixels*3)
		if format == pixconv.I420 || format == pixconv.NV12 {
			parts = append(parts, pixels*3/4)
		}
	}
	var total uint64
	for _, part := range parts {
		if math.MaxUint64-total < part {
			return 0, errors.New("recorder memory estimate overflows uint64")
		}
		total += part
	}
	return total, nil
}

func validateRecordingGLLimits(config *RecordingSessionConfig) error {
	var maxTexture, maxRenderbuffer, maxLayers int32
	var majorVersion, minorVersion int32
	var maxViewport [2]int32
	extensions := make(map[string]struct{})
	goroutines.CallMain(func() {
		gl.GetIntegerv(gl.MAJOR_VERSION, &majorVersion)
		gl.GetIntegerv(gl.MINOR_VERSION, &minorVersion)
		var extensionCount int32
		gl.GetIntegerv(gl.NUM_EXTENSIONS, &extensionCount)
		for i := range extensionCount {
			extensions[gl.GoStr(gl.GetStringi(gl.EXTENSIONS, uint32(i)))] = struct{}{}
		}
		gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxTexture)
		gl.GetIntegerv(gl.MAX_RENDERBUFFER_SIZE, &maxRenderbuffer)
		gl.GetIntegerv(gl.MAX_ARRAY_TEXTURE_LAYERS, &maxLayers)
		gl.GetIntegerv(gl.MAX_VIEWPORT_DIMS, &maxViewport[0])
	})

	var errs []error
	if majorVersion < 4 || majorVersion == 4 && minorVersion < 5 {
		for _, extension := range []string{
			"GL_ARB_buffer_storage",
			"GL_ARB_copy_image",
			"GL_ARB_direct_state_access",
			"GL_ARB_get_texture_sub_image",
		} {
			if _, ok := extensions[extension]; !ok {
				errs = append(errs, fmt.Errorf("recording requires modern GPU capability %s", extension))
			}
		}
	}
	if maxTexture <= 0 || config.width > int(maxTexture) || config.height > int(maxTexture) {
		errs = append(errs, fmt.Errorf("capture %dx%d exceeds GL texture limit %d", config.width, config.height, maxTexture))
	}
	if maxRenderbuffer <= 0 || config.width > int(maxRenderbuffer) || config.height > int(maxRenderbuffer) {
		errs = append(errs, fmt.Errorf("capture %dx%d exceeds GL renderbuffer limit %d", config.width, config.height, maxRenderbuffer))
	}
	if maxViewport[0] <= 0 || maxViewport[1] <= 0 || config.width > int(maxViewport[0]) || config.height > int(maxViewport[1]) {
		errs = append(errs, fmt.Errorf("capture %dx%d exceeds GL viewport limit %dx%d", config.width, config.height, maxViewport[0], maxViewport[1]))
	}
	if config.motionBlur && (maxLayers <= 0 || config.blendFrames > int(maxLayers)) {
		errs = append(errs, fmt.Errorf("motion-blur history %d exceeds GL array-layer limit %d", config.blendFrames, maxLayers))
	}
	return errors.Join(errs...)
}

func runRecordingProbe(config *RecordingSessionConfig) error {
	probeDir, err := os.MkdirTemp(config.outputRoot, ".danser-probe-")
	if err != nil {
		return fmt.Errorf("create probe directory: %w", err)
	}
	defer os.RemoveAll(probeDir)

	probePath := filepath.Join(probeDir, "probe."+config.container)
	options := buildRecordingProbeArgs(config, probePath)

	cmd, err := prepareFFmpeg("ffmpeg", options...)
	if err != nil {
		return fmt.Errorf("prepare probe: %w", err)
	}
	if err = runFFmpegCommand(cmd, new(diagnosticTail)); err != nil {
		return err
	}
	info, err := os.Stat(probePath)
	if err != nil {
		return fmt.Errorf("inspect probe output: %w", err)
	}
	if info.Size() == 0 {
		return errors.New("probe output is empty")
	}
	return nil
}

func buildRecordingProbeArgs(config *RecordingSessionConfig, probePath string) []string {
	frameRate := strconv.Itoa(config.outputFPS)
	options := []string{
		"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%dx%d:r=%s", config.width, config.height, frameRate),
		"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=r=%d:cl=%dc,aformat=sample_fmts=flt", config.audioSampleRate, config.audioChannels),
		"-frames:v", "2", "-t", "0.05",
	}
	videoFilters := bt709SetParams
	if config.videoFilters != "" {
		videoFilters = config.videoFilters + "," + videoFilters
	}
	options = append(options, "-vf", videoFilters)
	options = append(options,
		"-c:v", config.encoder,
		"-pix_fmt", config.outputFormat,
		"-color_range:v", "tv",
		"-colorspace:v", "bt709",
		"-color_trc:v", "bt709",
		"-color_primaries:v", "bt709",
		"-movflags", "+write_colr",
	)
	options = append(options, config.videoOptions...)
	if config.audioFilters != "" {
		options = append(options, "-af", config.audioFilters)
	}
	options = append(options, "-c:a", config.audioCodec, "-strict", "-2")
	options = append(options, config.audioOptions...)
	options = append(options, probePath)
	return options
}

func formatBytes(bytes uint64) string {
	return fmt.Sprintf("%.2f GiB", float64(bytes)/(1024*1024*1024))
}
