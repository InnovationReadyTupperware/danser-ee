package ffmpeg

import (
	"errors"
	"fmt"
	"math"
)

// RecordingTimelineEvent describes one deterministic offline clock boundary.
type RecordingTimelineEvent struct {
	SimulationSample int64
	AudioSample      int64
	RenderSource     bool
	EmitOutput       bool
	OutputIndex      int64
}

// RecordingTimeline merges simulation, audio, source-render, and output-frame
// boundaries on the integer mixer-sample clock.
type RecordingTimeline struct {
	sampleRate      int64
	updateRate      int64
	sourceRate      int64
	outputRate      int64
	oversample      int64
	outputFrames    int64
	contentSamples  int64
	audioEndSample  int64
	endClockSample  int64
	sourcePhaseHalf int64
	firstOutputAt   int64
	lastSourceIndex int64
	nextUpdate      int64
	nextSource      int64
	finished        bool
}

// NewRecordingTimeline creates a centered-shutter timeline for one prepared
// recording. durationMillis is the player's complete recording interval.
func NewRecordingTimeline(durationMillis float64, config *RecordingSessionConfig) (*RecordingTimeline, error) {
	if config == nil {
		return nil, errors.New("recording timeline configuration is nil")
	}
	if durationMillis < 0 || math.IsNaN(durationMillis) || math.IsInf(durationMillis, 0) {
		return nil, fmt.Errorf("recording duration must be finite and non-negative, got %v", durationMillis)
	}

	durationNanosFloat := math.Ceil(durationMillis * 1_000_000)
	if durationNanosFloat > math.MaxInt64 {
		return nil, errors.New("recording duration exceeds supported range")
	}
	durationNanos := int64(durationNanosFloat)
	outputFrames, err := ceilMulDiv(durationNanos, int64(config.outputFPS), 1_000_000_000)
	if err != nil {
		return nil, fmt.Errorf("calculate output frame count: %w", err)
	}
	if durationNanos > 0 && outputFrames == 0 {
		outputFrames = 1
	}

	sampleRate := int64(config.audioSampleRate)
	contentSamples, err := ceilMulDiv(durationNanos, sampleRate, 1_000_000_000)
	if err != nil {
		return nil, fmt.Errorf("calculate content sample count: %w", err)
	}
	audioEndSample := int64(0)
	if outputFrames > 0 {
		audioEndSample, err = ceilMulDiv(outputFrames, sampleRate, int64(config.outputFPS))
		if err != nil {
			return nil, fmt.Errorf("calculate audio endpoint: %w", err)
		}
	}

	oversample := int64(config.oversample)
	blendFrames := int64(config.blendFrames)
	centerSum := oversample + blendFrames - 1
	firstOutputAt := centerSum / 2
	phaseHalf := centerSum % 2
	lastSourceIndex := int64(-1)
	lastSourceSample := int64(0)
	if outputFrames > 0 {
		if outputFrames-1 > (math.MaxInt64-firstOutputAt)/oversample {
			return nil, errors.New("recording source frame count overflows int64")
		}
		lastSourceIndex = firstOutputAt + (outputFrames-1)*oversample
		lastSourceSample, err = sourceSamplePosition(lastSourceIndex, phaseHalf, sampleRate, int64(config.sourceFPS))
		if err != nil {
			return nil, err
		}
	}

	return &RecordingTimeline{
		sampleRate: sampleRate, updateRate: int64(max(config.sourceFPS, 1000)),
		sourceRate: int64(config.sourceFPS), outputRate: int64(config.outputFPS),
		oversample: oversample, outputFrames: outputFrames,
		contentSamples: contentSamples, audioEndSample: audioEndSample,
		endClockSample:  max(audioEndSample, lastSourceSample),
		sourcePhaseHalf: phaseHalf, firstOutputAt: firstOutputAt,
		lastSourceIndex: lastSourceIndex, nextUpdate: 1,
	}, nil
}

// SampleRate returns the integer clock frequency used by timeline events.
func (timeline *RecordingTimeline) SampleRate() int64 {
	return timeline.sampleRate
}

// OutputFrames returns the exact number of video frames due.
func (timeline *RecordingTimeline) OutputFrames() int64 {
	return timeline.outputFrames
}

// OutputRate returns the configured encoded frame rate.
func (timeline *RecordingTimeline) OutputRate() int64 {
	return timeline.outputRate
}

// Next returns the next merged clock boundary.
func (timeline *RecordingTimeline) Next() (RecordingTimelineEvent, bool) {
	if timeline.finished {
		return RecordingTimelineEvent{}, false
	}

	nextUpdateSample := int64(math.MaxInt64)
	if timeline.nextUpdate > 0 {
		nextUpdateSample = roundedMulDiv(timeline.nextUpdate, timeline.sampleRate, timeline.updateRate)
		if nextUpdateSample > timeline.contentSamples {
			nextUpdateSample = math.MaxInt64
		}
	}
	nextSourceSample := int64(math.MaxInt64)
	if timeline.nextSource <= timeline.lastSourceIndex {
		nextSourceSample, _ = sourceSamplePosition(timeline.nextSource, timeline.sourcePhaseHalf, timeline.sampleRate, timeline.sourceRate)
	}

	clockSample := min(nextUpdateSample, nextSourceSample, timeline.endClockSample)
	if clockSample == math.MaxInt64 {
		timeline.finished = true
		return RecordingTimelineEvent{}, false
	}

	event := RecordingTimelineEvent{
		SimulationSample: min(clockSample, timeline.contentSamples),
		AudioSample:      min(clockSample, timeline.audioEndSample),
		OutputIndex:      -1,
	}
	if nextUpdateSample == clockSample {
		timeline.nextUpdate++
	}
	if nextSourceSample == clockSample {
		event.RenderSource = true
		if timeline.nextSource >= timeline.firstOutputAt && (timeline.nextSource-timeline.firstOutputAt)%timeline.oversample == 0 {
			event.EmitOutput = true
			event.OutputIndex = (timeline.nextSource - timeline.firstOutputAt) / timeline.oversample
		}
		timeline.nextSource++
	}
	if clockSample == timeline.endClockSample && timeline.nextSource > timeline.lastSourceIndex {
		timeline.finished = true
	}

	return event, true
}

func sourceSamplePosition(index, phaseHalf, sampleRate, sourceRate int64) (int64, error) {
	if index < 0 || sampleRate <= 0 || sourceRate <= 0 {
		return 0, errors.New("invalid source sample position inputs")
	}
	if index > (math.MaxInt64-phaseHalf)/2 {
		return 0, errors.New("source sample position overflows int64")
	}
	return roundedMulDiv(2*index+phaseHalf, sampleRate, 2*sourceRate), nil
}

func ceilMulDiv(value, multiplier, divisor int64) (int64, error) {
	if value < 0 || multiplier < 0 || divisor <= 0 {
		return 0, errors.New("invalid multiply-divide operands")
	}
	if value != 0 && multiplier > math.MaxInt64/value {
		return 0, errors.New("multiply-divide overflows int64")
	}
	product := value * multiplier
	return product/divisor + btoi64(product%divisor != 0), nil
}

func roundedMulDiv(value, multiplier, divisor int64) int64 {
	if value == 0 {
		return 0
	}
	return (value*multiplier + divisor/2) / divisor
}

func btoi64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
