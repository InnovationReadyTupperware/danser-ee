package objects

import "testing"

func TestTimingsUseDefaultPointWhenSpinnerHasNoTimingPoints(t *testing.T) {
	timings := NewTimings()
	want := timings.GetDefault()

	if got := timings.GetPointAt(1000); got != want {
		t.Fatalf("GetPointAt without timing points = %#v, want default %#v", got, want)
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
