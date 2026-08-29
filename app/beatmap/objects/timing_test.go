package objects

import (
	"math"
	"testing"
)

func TestTimingsUseDefaultPointWhenSpinnerHasNoTimingPoints(t *testing.T) {
	timings := NewTimings()
	want := timings.GetDefault()

	if got := timings.GetPointAt(1000); got != want {
		t.Fatalf("GetPointAt without timing points = %#v, want default %#v", got, want)
	}
	if got := want.GetBeatLength(); got != defaultTimingBeatLength {
		t.Fatalf("default stable beat length = %g, want %g", got, defaultTimingBeatLength)
	}
	if got := want.GetBeatLengthLazer(); got != defaultTimingBeatLength {
		t.Fatalf("default Lazer beat length = %g, want %g", got, defaultTimingBeatLength)
	}

	timings.Reset()
	if timings.Current != want {
		t.Fatalf("Reset without timing points = %#v, want default %#v", timings.Current, want)
	}
}

func TestNilTimingsReturnZeroPointInsteadOfPanicking(t *testing.T) {
	var timings *Timings

	if got := timings.GetPointAt(1000); got != (TimingPoint{}) {
		t.Fatalf("nil Timings.GetPointAt = %#v, want zero point", got)
	}

	if got := timings.GetOriginalPointAt(1000); got != (TimingPoint{}) {
		t.Fatalf("nil Timings.GetOriginalPointAt = %#v, want zero point", got)
	}

	timings.Reset()
}

func TestTimingsClampUnrealisticallyShortBeatLengths(t *testing.T) {
	timings := NewTimings()
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.AddPoint(100, 1e-298, 1, 1, 1, 4, false, false, false)
	timings.AddPoint(100, -1e-298, 1, 1, 1, 4, true, false, false)
	timings.FinalizePoints()

	point := timings.GetPointAt(100)
	if got := point.GetBaseBeatLength(); got != minimumTimingBeatLength {
		t.Fatalf("base beat length = %g, want %d", got, minimumTimingBeatLength)
	}

	if got := point.GetBeatLength(); math.Abs(got-0.6) > 1e-6 {
		t.Fatalf("stable beat length = %g, want 0.6", got)
	}

	if got := point.GetBeatLengthLazer(); math.Abs(got-0.6) > 1e-6 {
		t.Fatalf("Lazer beat length = %g, want 0.6", got)
	}

	initial := NewTimings()
	initial.AddPoint(0, -1e-298, 1, 1, 1, 4, true, false, false)
	initial.FinalizePoints()

	point = initial.GetPointAt(0)
	if got := point.GetBaseBeatLength(); got != defaultTimingBeatLength {
		t.Fatalf("inherited-only timing base beat length = %g, want %g", got, defaultTimingBeatLength)
	}
	if got := point.GetBeatLengthLazer(); math.Abs(got-100) > 1e-5 {
		t.Fatalf("inherited-only Lazer beat length = %g, want 100", got)
	}
}

func TestMalformedLongSliderDoesNotCollapseToItsStartTime(t *testing.T) {
	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.AddPoint(181365, 1e-298, 1, 1, 1, 4, false, false, false)
	timings.AddPoint(181365, -1e-298, 1, 1, 1, 4, true, false, false)
	timings.FinalizePoints()

	slider := NewSlider([]string{
		"167", "300", "181365", "6", "0",
		"L|167:-65535", "1", "102400", "12", "0:0|0:0",
	})
	if slider == nil {
		t.Fatal("NewSlider returned nil")
	}

	slider.SetTiming(timings, 14, false)

	if got := slider.EndTimeLazer - slider.StartTime; got < 400 || got > 500 {
		t.Fatalf("Lazer slider duration = %g, want a duration near 439 ms", got)
	}

	if len(slider.ScorePointsLazer) == 0 || slider.ScorePointsLazer[len(slider.ScorePointsLazer)-1].Time <= slider.StartTime+400 {
		t.Fatalf("Lazer slider tail was scheduled at or near the head: %#v", slider.ScorePointsLazer)
	}
}
