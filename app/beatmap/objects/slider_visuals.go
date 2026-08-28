package objects

import (
	"math"

	"github.com/wieku/danser-go/framework/math/mutils"
)

// sliderSnakeSettings contains the visual controls which affect the body
// range. The duration and fade multipliers are intentionally kept separate
// from gameplay timing: they are Danser visualizer preferences and must never
// change the generated judgement events.
type sliderSnakeSettings struct {
	in                 bool
	out                bool
	durationMultiplier float64
	fadeMultiplier     float64
}

// sliderBodyRange is the pair of path progress values consumed by the slider
// body renderer. The values are not sorted. Keeping the logical head and tail
// separate is important for final-span snaking, where the visible body can
// retract from either end depending on repeat parity.
type sliderBodyRange struct {
	head float64
	tail float64
}

// sliderBodyRangeAt mirrors osu!lazer's SnakingSliderBody.UpdateProgress. It
// uses the true Lazer slider duration and computes the range for every frame,
// rather than queueing one-shot animations which can become stale after a
// seek, late hit, or frame skip.
func sliderBodyRangeAt(time, startTime, endTime, preempt, spanDuration float64, spanCount int, settings sliderSnakeSettings) sliderBodyRange {
	if spanCount <= 0 || !isFinite(time) || !isFinite(startTime) || !isFinite(endTime) || endTime <= startTime {
		return sliderBodyRange{}
	}

	completion := mutils.Clamp((time-startTime)/(endTime-startTime), 0.0, 1.0)
	span, spanProgress := sliderSpanProgress(completion, spanCount)

	snakeInProgress := 1.0
	if settings.in {
		snakeInStart := startTime - preempt
		snakeInEnd := startTime - preempt*2.0/3.0*(1.0-clampSnakeMultiplier(settings.fadeMultiplier)) +
			max(0.0, spanDuration)*clampSnakeMultiplier(settings.durationMultiplier)

		if snakeInEnd <= snakeInStart {
			snakeInProgress = 0
			if time >= snakeInEnd {
				snakeInProgress = 1
			}
		} else {
			snakeInProgress = mutils.Clamp((time-snakeInStart)/(snakeInEnd-snakeInStart), 0.0, 1.0)
		}
	}

	bodyRange := sliderBodyRange{tail: snakeInProgress}
	if span >= spanCount-1 {
		// SpanAt(1) is SpanCount in lazer. Clamping only the parity check keeps
		// the final path position correct for both odd and even span counts.
		lastSpan := min(span, spanCount-1)
		if lastSpan%2 == 1 {
			bodyRange.tail = 1
			if settings.out {
				bodyRange.tail = spanProgress
			}
		} else if settings.out {
			bodyRange.head = spanProgress
		}
	}

	bodyRange.head = mutils.Clamp(bodyRange.head, 0.0, 1.0)
	bodyRange.tail = mutils.Clamp(bodyRange.tail, 0.0, 1.0)

	return bodyRange
}

// sliderSpanProgress is the direct equivalent of lazer's SpanAt and
// ProgressAt pair. In particular, it preserves SpanAt(1)'s parity instead of
// clamping the span index before reversing the final path progress.
func sliderSpanProgress(completion float64, spanCount int) (int, float64) {
	if spanCount <= 0 {
		return 0, 0
	}

	if !isFinite(completion) {
		completion = 0
	}

	completion = mutils.Clamp(completion, 0.0, 1.0)
	scaled := completion * float64(spanCount)
	span := int(math.Floor(scaled))
	progress := scaled - math.Floor(scaled)

	if completion >= 1 {
		span = spanCount
		progress = 0
	}

	if span%2 == 1 {
		progress = 1 - progress
	}

	return span, mutils.Clamp(progress, 0.0, 1.0)
}

func clampSnakeMultiplier(value float64) float64 {
	if !isFinite(value) {
		return 0
	}

	return mutils.Clamp(value, 0.0, 1.0)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
