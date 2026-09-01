package objects

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestSliderPositionAtForDiffUsesTheParticipantGameplayPath(t *testing.T) {
	slider := newProvenanceTestSlider(t)
	stable := difficulty.NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(difficulty.GameplayStable)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)
	sampleTime := slider.StartTime + (slider.EndTimeLazer-slider.StartTime)*0.37

	stablePosition := slider.PositionAt(sampleTime)
	lazerPosition := slider.PositionAtLazer(sampleTime)
	if stablePosition == lazerPosition {
		t.Fatalf("test slider produced identical Stable and Lazer positions at %g", sampleTime)
	}

	if got := slider.PositionAtForDiff(sampleTime, stable); got != stablePosition {
		t.Fatalf("Stable PositionAtForDiff() = %v, want %v", got, stablePosition)
	}
	if got := slider.PositionAtForDiff(sampleTime, lazer); got != lazerPosition {
		t.Fatalf("Lazer PositionAtForDiff() = %v, want %v", got, lazerPosition)
	}
	if got := GetStackedPositionAtModForDiff(slider, sampleTime, lazer); got != slider.GetStackedPositionAtModLazer(sampleTime, lazer) {
		t.Fatalf("generic Lazer stacked position = %v, want the Lazer slider position", got)
	}
	if got := GetEndTimeForDiff(slider, lazer); got != slider.EndTimeLazer {
		t.Fatalf("Lazer end time = %g, want %g", got, slider.EndTimeLazer)
	}
	if got := GetEndTimeForDiff(slider, stable); got != slider.EndTime {
		t.Fatalf("Stable end time = %g, want %g", got, slider.EndTime)
	}
}

func TestSliderEndPositionForDiffUsesTheParticipantGameplayPath(t *testing.T) {
	slider := newProvenanceTestSlider(t)
	stable := difficulty.NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(difficulty.GameplayStable)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)

	stableWant := slider.GetStackedPositionAtMod(slider.EndTime, stable)
	lazerWant := slider.GetStackedPositionAtModLazer(slider.EndTimeLazer, lazer)

	if got := slider.GetStackedEndPositionModForDiff(stable); got != stableWant {
		t.Fatalf("Stable endpoint = %v, want %v", got, stableWant)
	}
	if got := GetStackedEndPositionModForDiff(slider, lazer); got != lazerWant {
		t.Fatalf("generic Lazer endpoint = %v, want %v", got, lazerWant)
	}
}

func TestSliderExitAngleUsesTheParticipantGameplayEndpoint(t *testing.T) {
	slider := newProvenanceTestSlider(t)
	stable := difficulty.NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(difficulty.GameplayStable)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)

	for _, test := range []struct {
		name string
		diff *difficulty.Difficulty
	}{
		{name: "Stable", diff: stable},
		{name: "Lazer", diff: lazer},
	} {
		t.Run(test.name, func(t *testing.T) {
			endTime := GetEndTimeForDiff(slider, test.diff)
			sampleTime := endTime - min(10.0, slider.partDurationForDiff(test.diff))
			want := GetStackedEndPositionModForDiff(slider, test.diff).AngleRV(
				GetStackedPositionAtModForDiff(slider, sampleTime, test.diff),
			)

			if got := slider.GetEndAngleMod(test.diff); math.Abs(float64(got-want)) > 1e-5 {
				t.Fatalf("exit angle = %g, want %g", got, want)
			}
		})
	}
}

func TestSliderDummyCirclesUseTheParticipantScorePointTimeline(t *testing.T) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	t.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	slider := newProvenanceTestSlider(t)
	stable := difficulty.NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(difficulty.GameplayStable)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)

	stableCircles := slider.GetAsDummyCirclesForDiff(stable)
	if got, want := len(stableCircles), len(slider.ScorePoints)+1; got != want {
		t.Fatalf("Stable cursor points = %d, want %d", got, want)
	}
	for i, point := range slider.ScorePoints {
		circle := stableCircles[i+1]
		if circle.GetStartTime() != point.Time {
			t.Fatalf("Stable cursor point %d time = %g, want %g", i, circle.GetStartTime(), point.Time)
		}
		if circle.GetStartPosition() != slider.PositionAt(point.Time) {
			t.Fatalf("Stable cursor point %d position = %v, want %v", i, circle.GetStartPosition(), slider.PositionAt(point.Time))
		}
	}

	lazerCircles := slider.GetAsDummyCirclesForDiff(lazer)
	if got, want := len(lazerCircles), len(slider.ScorePointsLazer)+1; got != want {
		t.Fatalf("Lazer cursor points = %d, want %d", got, want)
	}
	for i, point := range slider.ScorePointsLazer {
		circle := lazerCircles[i+1]
		if circle.GetStartTime() != point.Time {
			t.Fatalf("Lazer cursor point %d time = %g, want %g", i, circle.GetStartTime(), point.Time)
		}
		if circle.GetStartPosition() != slider.PositionAtLazer(point.Time) {
			t.Fatalf("Lazer cursor point %d position = %v, want %v", i, circle.GetStartPosition(), slider.PositionAtLazer(point.Time))
		}
	}
}

func TestRepeatedAnchorSliderHasFiniteLazerTraversal(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|300:160|300:160|360:220|420:220|420:220|460:180|500:180|500:180|460:140|400:140",
		"1", "300", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("repeated-anchor slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 2.06
	timings.TickRate = 0.5
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)

	if slider.IsSingular() {
		t.Fatalf("repeated-anchor slider workload class = %v, want a traversable slider", slider.WorkloadClass())
	}

	lazer := difficulty.NewDifficulty(5, 5, 5, 5)
	for i := range 17 {
		time := slider.StartTime + (slider.EndTimeLazer-slider.StartTime)*float64(i)/16
		position := slider.PositionAtForDiff(time, lazer)
		if math.IsNaN(float64(position.X)) || math.IsInf(float64(position.X), 0) ||
			math.IsNaN(float64(position.Y)) || math.IsInf(float64(position.Y), 0) {
			t.Fatalf("Lazer position at %g ms = %v, want finite coordinates", time, position)
		}
	}
}

func newProvenanceTestSlider(t *testing.T) *Slider {
	t.Helper()

	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|320:240|380:192", "1", "160", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("provenance test slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	return slider
}
