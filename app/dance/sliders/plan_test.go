package sliders

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
)

func TestMotionPlanStaysInsideSliderBodyAndTrackingArea(t *testing.T) {
	slider := newCurvedSlider(t, 1000, 2)
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	target := NewTarget(slider, diff)

	for time := target.GetStartTime(); time <= target.GetEndTime(); time += 0.5 {
		position := target.GetPositionAt(time)
		ball := slider.GetStackedPositionAtModForDiff(time, diff)
		if math.IsNaN(float64(position.X)) || math.IsNaN(float64(position.Y)) {
			t.Fatalf("position at %g is not finite: %v", time, position)
		}
		if distance := position.Dst(ball); distance > target.plan.trackRadius+0.001 {
			t.Fatalf("tracking distance at %g = %g, want <= %g", time, distance, target.plan.trackRadius)
		}
		if distance := float32(math.Sqrt(float64(target.plan.distanceToBodySq(position)))); distance > target.plan.bodyRadius+0.001 {
			t.Fatalf("body distance at %g = %g, want <= %g", time, distance, target.plan.bodyRadius)
		}
	}
}

func TestMotionPlanUsesAvailableTrackingSlack(t *testing.T) {
	slider := newCurvedSlider(t, 1000, 1)
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	target := NewTarget(slider, diff)

	var plannedDistance float32
	var authoredDistance float32
	previousPlanned := target.GetPositionAt(target.GetStartTime())
	previousAuthored := slider.GetStackedPositionAtModForDiff(target.GetStartTime(), diff)
	for time := target.GetStartTime() + 1; time <= target.GetEndTime(); time++ {
		planned := target.GetPositionAt(time)
		authored := slider.GetStackedPositionAtModForDiff(time, diff)
		plannedDistance += previousPlanned.Dst(planned)
		authoredDistance += previousAuthored.Dst(authored)
		previousPlanned = planned
		previousAuthored = authored
	}

	if plannedDistance >= authoredDistance*0.9 {
		t.Fatalf("planned distance = %g, authored distance = %g; expected a meaningful shortcut", plannedDistance, authoredDistance)
	}
}

func TestMotionPlanUsesParticipantTimeline(t *testing.T) {
	slider := newCurvedSlider(t, 1000, 3)
	stable := difficulty.NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(difficulty.GameplayStable)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)

	stableTarget := NewTarget(slider, stable)
	lazerTarget := NewTarget(slider, lazer)
	if stableTarget.GetEndTime() != slider.EndTime {
		t.Fatalf("Stable end time = %g, want %g", stableTarget.GetEndTime(), slider.EndTime)
	}
	if lazerTarget.GetEndTime() != slider.EndTimeLazer {
		t.Fatalf("Lazer end time = %g, want %g", lazerTarget.GetEndTime(), slider.EndTimeLazer)
	}
}

func newCurvedSlider(t *testing.T, startTime float64, repeats int) *objects.Slider {
	t.Helper()

	slider := objects.NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|384:64|448:192|384:320|256:192", "1", "500", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("test slider was rejected")
	}
	slider.StartTime = startTime
	slider.RepeatCount = repeats

	timings := objects.NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	return slider
}
