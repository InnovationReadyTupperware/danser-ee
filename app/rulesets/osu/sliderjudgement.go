package osu

import (
	"github.com/wieku/danser-go/app/beatmap/objects"
)

const lazerSliderTailLeniency = 36.0

// sliderPointKind identifies the gameplay role of a generated slider event.
// Keeping this separate from HitResult is important: a successful repeat and
// a successful tick share the same Lazer result, but their health treatment
// and edge sound ownership are different from the final tail.
type sliderPointKind uint8

const (
	sliderPointTick sliderPointKind = iota
	sliderPointRepeat
	sliderPointTail
)

type sliderEvent struct {
	time      float64
	kind      sliderPointKind
	maxResult HitResult
	edgeIndex int

	judged    bool
	hitResult HitResult
}

func (event sliderEvent) isTail() bool {
	return event.kind == sliderPointTail
}

func (event sliderEvent) resultPart() sliderJudgementPart {
	switch event.kind {
	case sliderPointTick:
		return sliderPartTick
	case sliderPointRepeat:
		return sliderPartRepeat
	case sliderPointTail:
		return sliderPartTail
	default:
		return sliderPartNone
	}
}

func hasUnjudgedSliderEvents(events []sliderEvent) bool {
	for _, event := range events {
		if !event.judged {
			return true
		}
	}

	return false
}

// buildStableSliderEvents preserves the stable event schedule. In
// particular, the final stable point remains at the historical max(duration /
// 2, end - 36) timestamp. Replacing this with the Lazer event schedule would
// change replay-visible result timestamps and audio timing for stable scores.
func buildStableSliderEvents(slider *objects.Slider) []sliderEvent {
	events := make([]sliderEvent, 0, len(slider.ScorePoints))

	for _, point := range slider.ScorePoints {
		event := sliderEvent{
			// The old Stable state stored every point as int64, so preserve
			// truncation for non-final points as well as the final-point rule
			// below.
			time:      float64(int64(point.Time)),
			kind:      sliderPointTick,
			maxResult: SliderPoint,
			edgeIndex: point.EdgeIndex,
		}

		if point.LastPoint {
			event.kind = sliderPointTail
			event.maxResult = SliderEnd
		} else if point.IsReverse {
			event.kind = sliderPointRepeat
			event.maxResult = SliderRepeat
		}

		events = append(events, event)
	}

	if len(events) > 0 {
		last := &events[len(events)-1]
		// Match the historical integer truncation used by the old runtime
		// path, including for malformed or fractional beatmap timestamps.
		last.time = float64(max(
			int64(slider.StartTime)+int64(slider.EndTime-slider.StartTime)/2,
			int64(slider.EndTime)-int64(lazerSliderTailLeniency),
		))
	}

	return events
}

// buildLazerSliderEvents maps the upstream ScorePointsLazer representation to
// the result-bearing events consumed by the runtime. ScorePointsLazer already
// excludes the legacy last tick and stores the final tail at the true end;
// both details are required for Lazer's -36 ms tail window and classic tail
// result type.
func buildLazerSliderEvents(slider *objects.Slider, classicNoSliderHeadAccuracy bool) []sliderEvent {
	events := make([]sliderEvent, 0, len(slider.ScorePointsLazer))
	edgeIndex := 1

	for _, point := range slider.ScorePointsLazer {
		event := sliderEvent{
			time:      point.Time,
			kind:      sliderPointTick,
			maxResult: LargeTickHit,
			edgeIndex: -1,
		}

		if point.LastPoint {
			event.kind = sliderPointTail
			event.edgeIndex = edgeIndex
			event.maxResult = SliderTailHit
			if classicNoSliderHeadAccuracy {
				event.maxResult = SmallTickHit
			}
		} else if point.IsReverse {
			event.kind = sliderPointRepeat
			event.edgeIndex = edgeIndex
		}

		if event.edgeIndex >= 0 {
			edgeIndex++
		}

		events = append(events, event)
	}

	return events
}

// lazerSliderEndTime returns the end used by the Lazer event schedule. The
// embedded HitObject end is the floored Stable duration, so using it for Lazer
// completion can finalize a slider before its true tail event on a fractional
// millisecond boundary. The fallback keeps manually constructed or otherwise
// uninitialized sliders useful to pure callers and tests.
func lazerSliderEndTime(slider *objects.Slider) float64 {
	if slider.EndTimeLazer > slider.StartTime {
		return slider.EndTimeLazer
	}

	return slider.GetEndTime()
}

func lazerSliderHeadResults(classicNoSliderHeadAccuracy bool, startResult HitResult) (HitResult, HitResult) {
	if classicNoSliderHeadAccuracy {
		if startResult.IsHit() {
			return LargeTickHit, LargeTickHit
		}

		return LargeTickMiss, LargeTickHit
	}

	if startResult.IsHit() {
		return startResult, Hit300
	}

	return Miss, Hit300
}

func stableSliderHeadResults(startResult HitResult) (HitResult, HitResult) {
	if startResult.IsHit() {
		return SliderStart, SliderStart
	}

	return SliderMiss, SliderStart
}

func lazerSliderEventResult(event sliderEvent, allowable bool) HitResult {
	if allowable {
		return event.maxResult
	}

	if event.isTail() {
		if event.maxResult == SmallTickHit {
			return SmallTickMiss
		}

		return IgnoreMiss
	}

	return LargeTickMiss
}

func lazerSliderEventDue(event sliderEvent, time float64) bool {
	if event.isTail() {
		return time >= event.time-lazerSliderTailLeniency
	}

	return time >= event.time
}

func lazerSliderTailMayBeJudged(event sliderEvent, time float64, previousEventsJudged bool) bool {
	return event.isTail() && previousEventsJudged && lazerSliderEventDue(event, time)
}

func classicSliderCollapse(hitCount, totalCount int) HitResult {
	if hitCount <= 0 || totalCount <= 0 {
		return Miss
	}

	if hitCount >= totalCount {
		return Hit300
	}

	if hitCount*2 >= totalCount {
		return Hit100
	}

	return Hit50
}
