package osu

import (
	"math"
	"testing"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
)

func TestCalculateSpinnerRequirementsUsesExactLazerDuration(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)

	requirements := calculateSpinnerRequirements(diff, 1000.9, 1400.8)
	if requirements.required != 0 {
		t.Fatalf("Lazer required spins = %d, want 0 for a 399.9 ms spinner", requirements.required)
	}

	requirements = calculateSpinnerRequirements(diff, 1000.9, 1401.0)
	if requirements.required != 1 {
		t.Fatalf("Lazer required spins = %d, want 1 for a 400.1 ms spinner", requirements.required)
	}
}

func TestCalculateSpinnerRequirementsPreservesStableIntegerDuration(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)

	requirements := calculateSpinnerRequirements(diff, 1000.9, 1400.8)
	if requirements.required != 2 {
		t.Fatalf("Stable required spins = %d, want 2 from integer duration 400 ms", requirements.required)
	}
	if requirements.maximumBonus != 0 {
		t.Fatalf("Stable maximum bonus spins = %d, want 0", requirements.maximumBonus)
	}
}

func TestCalculateHealthSpinnerRequirementPreservesStableFractionalDuration(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)

	got := calculateHealthSpinnerRequirement(diff, 1000.9, 1400.8)
	if got != 1 {
		// The old health path evaluated 399.9 / 1000 * 5 and truncated the
		// result, unlike the runtime Stable path's integer-millisecond rule.
		t.Fatalf("Stable health required spins = %d, want 1", got)
	}
}

func TestSpinnerStateTreatsZeroRequirementAsComplete(t *testing.T) {
	state := spinnerState{}
	if got := state.getCompletion(); got != 1 {
		t.Fatalf("zero-requirement completion = %v, want 1", got)
	}
}

func TestSpinnerStateIgnoresNonFiniteRotationDelta(t *testing.T) {
	state := spinnerState{}
	reportLazerRotationDelta(&state, float32(math.NaN()))
	if state.rotationCountF != 0 || state.totalAccumulatedRotation != 0 {
		t.Fatalf("non-finite delta changed spinner state: %+v", state)
	}
}
