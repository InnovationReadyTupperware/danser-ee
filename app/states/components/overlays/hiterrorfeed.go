package overlays

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
)

type hitErrorEvent struct {
	time   float64
	offset float64
	result osu.HitResult
}

// buildHitErrorEvent translates danser's gameplay result into the timing event
// osu!lazer would expose for its hit-error and UR consumers. Stable replay
// provenance is treated like osu!lazer's Classic path rather than preserving
// danser's historical integer-window presentation.
func buildHitErrorEvent(object objects.IHitObject, result osu.JudgementResult, diff *difficulty.Difficulty) (hitErrorEvent, bool) {
	if object == nil {
		return hitErrorEvent{}, false
	}

	isCircle := false
	isSlider := false
	switch object.(type) {
	case *objects.Circle:
		isCircle = true
	case *objects.Slider:
		isSlider = true
	default:
		return hitErrorEvent{}, false
	}

	positionalMiss := result.HitResult == osu.PositionalMiss
	if positionalMiss {
		// Positional misses are emitted for the object head and are useful as
		// diagnostic marks, but they are not successful timing samples.
		if !isCircle && !(isSlider && isSliderHeadResult(result)) {
			return hitErrorEvent{}, false
		}
	} else {
		if !result.HitResult.IsHit() {
			return hitErrorEvent{}, false
		}

		if isSlider && !isSliderHeadResult(result) {
			return hitErrorEvent{}, false
		}
	}

	endTime := object.GetEndTime()
	if isSlider && isSliderHeadResult(result) {
		// The parent Slider ends at its tail, but Lazer's slider-head
		// judgement is a separate timing object whose end is its start.
		endTime = object.GetStartTime()
	}

	offset := float64(result.Time) - endTime
	// Lazer caps TimeOffset at MaximumJudgementOffset. Circles and slider
	// heads use osu!standard's fixed 400 ms miss window. The upstream clamp is
	// intentionally one-sided: very early offsets are not clamped here.
	offset = min(offset, difficulty.HittableRange)

	timingResult := result.HitResult
	if !positionalMiss {
		var ok bool
		timingResult, ok = lazerHitErrorResult(diff, result.HitResult, isSlider, offset)
		if !ok {
			return hitErrorEvent{}, false
		}
	}

	return hitErrorEvent{
		time:   float64(result.Time),
		offset: offset,
		result: timingResult,
	}, true
}

func lazerHitErrorResult(diff *difficulty.Difficulty, result osu.HitResult, sliderHead bool, offset float64) (osu.HitResult, bool) {
	if diff == nil {
		return 0, false
	}

	delta := math.Abs(offset)
	var timingResult osu.HitResult
	switch {
	case delta <= diff.Hit300U:
		timingResult = osu.Hit300
	case delta <= diff.Hit100U:
		timingResult = osu.Hit100
	case delta <= diff.Hit50U:
		timingResult = osu.Hit50
	default:
		return 0, false
	}

	if sliderHead {
		switch result &^ osu.Additions {
		case osu.SliderStart, osu.LargeTickHit:
			// A Stable slider head corresponds to Classic's binary
			// LargeTickHit in osu!lazer. Non-Classic Lazer heads retain their
			// normal timing result below.
			return osu.LargeTickHit, true
		}
	}

	return timingResult, true
}

func isSliderHeadResult(result osu.JudgementResult) bool {
	// Stable heads use SliderStart without an explicit Lazer part. Lazer
	// heads carry sliderPartHead, including Classic LargeTickHit heads.
	return result.IsSliderHead() || result.HitResult&osu.SliderStart != 0 || result.MaxResult&osu.SliderStart != 0
}
