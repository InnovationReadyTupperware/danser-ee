package overlays

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
)

type hitErrorEvent struct {
	time   float64
	offset float64
	result osu.HitResult
}

// buildHitErrorEvent keeps the hit-error meter's object and result filter in
// one place. The ruleset result predicate knows which result kinds are real
// timing hits; this layer supplies the object boundary because a base hit on a
// slider summary must not be mistaken for a slider-head hit.
func buildHitErrorEvent(object objects.IHitObject, result osu.JudgementResult) (hitErrorEvent, bool) {
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
		if !result.AffectsHitError() {
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
	// heads currently share the 400 ms osu!standard miss window in danser;
	// use a symmetric cap so an out-of-radius positional diagnostic cannot
	// create an unbounded HUD coordinate or future statistic sample.
	offset = max(-difficulty.HittableRange, min(offset, difficulty.HittableRange))

	return hitErrorEvent{
		time:   float64(result.Time),
		offset: offset,
		result: result.HitResult,
	}, true
}

func isSliderHeadResult(result osu.JudgementResult) bool {
	// Stable heads use SliderStart without an explicit Lazer part. Lazer
	// heads carry sliderPartHead, including Classic LargeTickHit heads.
	return result.IsSliderHead() || result.HitResult&osu.SliderStart != 0 || result.MaxResult&osu.SliderStart != 0
}
