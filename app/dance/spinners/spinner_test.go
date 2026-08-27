package spinners

import (
	"math"
	"testing"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/settings"
)

func TestSpinnerRPMSelectionUsesLazerCompletionRange(t *testing.T) {
	if settings.CursorDance.SpinnerBehavior == nil {
		t.Fatal("CursorDance.SpinnerBehavior is nil")
	}

	previous := settings.CursorDance.SpinnerBehavior.SpinAtLowestRPM
	t.Cleanup(func() {
		settings.CursorDance.SpinnerBehavior.SpinAtLowestRPM = previous
	})

	tests := []struct {
		name string
		od   float64
		want float64
	}{
		{name: "OD 0", od: 0, want: 250},
		{name: "OD 5", od: 5, want: 380},
		{name: "OD 10", od: 10, want: 430},
		{name: "OD 11", od: 11, want: 440},
	}

	settings.CursorDance.SpinnerBehavior.SpinAtLowestRPM = true
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := difficulty.NewDifficulty(5, 5, test.od, 5)
			if got := spinnerRPM(diff); math.Abs(got-test.want) > 0.000001 {
				t.Fatalf("spinnerRPM(OD %.1f) = %v, want %v", test.od, got, test.want)
			}
		})
	}
}

func TestSpinnerRPMSelectionUsesFixedMaximumForLazer(t *testing.T) {
	behavior := settings.CursorDance.SpinnerBehavior
	if behavior == nil {
		t.Fatal("CursorDance.SpinnerBehavior is nil")
	}

	previous := behavior.SpinAtLowestRPM
	t.Cleanup(func() {
		behavior.SpinAtLowestRPM = previous
	})

	behavior.SpinAtLowestRPM = false

	for _, od := range []float64{0, 5, 10, 11} {
		diff := difficulty.NewDifficulty(5, 5, od, 5)
		if got := spinnerRPM(diff); got != difficulty.LazerSpinnerMaximumCompletionRPM {
			t.Fatalf("spinnerRPM(OD %.1f) = %v, want fixed maximum %v", od, got, difficulty.LazerSpinnerMaximumCompletionRPM)
		}
	}
}

func TestSpinnerRPMSelectionPreservesStableModeWithClassic(t *testing.T) {
	behavior := settings.CursorDance.SpinnerBehavior
	if behavior == nil {
		t.Fatal("CursorDance.SpinnerBehavior is nil")
	}

	previous := behavior.SpinAtLowestRPM
	t.Cleanup(func() {
		behavior.SpinAtLowestRPM = previous
	})

	behavior.SpinAtLowestRPM = true

	diff := difficulty.NewDifficulty(5, 5, 11, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)
	diff.SetMods(difficulty.Classic)

	if got := spinnerRPM(diff); got != legacySpinnerRPM {
		t.Fatalf("spinnerRPM(Stable + Classic) = %v, want legacy rate %v", got, legacySpinnerRPM)
	}
}

func TestSpinnerMoverScalesSDeltaToRequestedRPM(t *testing.T) {
	mover := NewCircleMover()
	mover.Init(0, 1000, 0, 1, 250)

	want := float32(1000 * 250 / legacySpinnerRPM)
	if got := mover.GetSDelta(1000); math.Abs(float64(got-want)) > 0.00001 {
		t.Fatalf("GetSDelta(1000) = %v, want %v", got, want)
	}
}
