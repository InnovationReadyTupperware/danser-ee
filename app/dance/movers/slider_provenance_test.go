package movers

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestSliderMoversUseLazerTraversalForLazerParticipants(t *testing.T) {
	previousCursorDance := settings.CursorDance
	settings.CursorDance = settings.NewConfigFile().CursorDance
	t.Cleanup(func() { settings.CursorDance = previousCursorDance })

	slider := newMoverProvenanceTestSlider(t)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)
	sampleTime := slider.StartTime + (slider.EndTimeLazer-slider.StartTime)*0.37
	want := slider.GetStackedPositionAtModLazer(sampleTime, lazer)

	for _, test := range []struct {
		name string
		new  func() MultiPointMover
	}{
		{name: "linear", new: NewLinearMoverSimple},
		{name: "spline", new: NewSplineMover},
	} {
		t.Run(test.name, func(t *testing.T) {
			mover := test.new()
			mover.Reset(lazer, 0)

			if got := mover.GetObjectsPosition(sampleTime, slider); got != want {
				t.Fatalf("Lazer slider position = %v, want %v", got, want)
			}
			if got := mover.GetObjectsEndTime(slider); got != slider.EndTimeLazer {
				t.Fatalf("Lazer slider end time = %g, want %g", got, slider.EndTimeLazer)
			}
		})
	}
}

func newMoverProvenanceTestSlider(t *testing.T) *objects.Slider {
	t.Helper()

	slider := objects.NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|320:240|380:192", "1", "160", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("mover provenance test slider was rejected")
	}

	timings := objects.NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	return slider
}
