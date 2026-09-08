package osu

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
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

func TestClassicModKeepsLazerSpinnerRequirements(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetMods(difficulty.Classic)

	if !diff.IsLazer() {
		t.Fatal("Classic changed the default gameplay mode away from Lazer")
	}

	requirements := calculateSpinnerRequirements(diff, 1000.9, 1400.8)
	if requirements.required != 0 {
		t.Fatalf("Lazer Classic required spins = %d, want 0 for a 399.9 ms spinner", requirements.required)
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

func TestLazerSpinnerActiveWindowIsEndExclusive(t *testing.T) {
	tests := []struct {
		name      string
		time      float64
		startTime float64
		endTime   float64
		want      bool
	}{
		{name: "before start", time: 99.9, startTime: 100, endTime: 200},
		{name: "at start", time: 100, startTime: 100, endTime: 200, want: true},
		{name: "inside", time: 199.9, startTime: 100, endTime: 200, want: true},
		{name: "at integer end", time: 200, startTime: 100, endTime: 200},
		{name: "after integer end", time: 200.1, startTime: 100, endTime: 200},
		{name: "fractional end remains active before it", time: 200.8, startTime: 100, endTime: 200.9, want: true},
		{name: "fractional end is exclusive", time: 200.9, startTime: 100, endTime: 200.9},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := lazerSpinnerActiveAt(test.time, test.startTime, test.endTime); got != test.want {
				t.Fatalf("lazerSpinnerActiveAt(%v) = %t, want %t", test.time, got, test.want)
			}
		})
	}
}

func TestSpinnerEndBoundaryPreservesLazerFractionalTime(t *testing.T) {
	tests := []struct {
		name      string
		time      int64
		endTime   float64
		lazer     bool
		wantEnded bool
	}{
		{name: "Lazer before fractional end", time: 200, endTime: 200.9, lazer: true},
		{name: "Lazer at next integer", time: 201, endTime: 200.9, lazer: true, wantEnded: true},
		{name: "Lazer at exact end", time: 200, endTime: 200, lazer: true, wantEnded: true},
		{name: "Stable retains integer end", time: 200, endTime: 200.9, wantEnded: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := spinnerEndReached(test.time, test.endTime, test.lazer); got != test.wantEnded {
				t.Fatalf("spinnerEndReached(%d, %v, %t) = %t, want %t", test.time, test.endTime, test.lazer, got, test.wantEnded)
			}
		})
	}
}

func TestProcessLazerIgnoresRotationAtEndTime(t *testing.T) {
	spinner, player, state := newLazerSpinnerProcessTest()

	spinner.processLazer(player, 100)
	player.cursor.RawPosition = vector.NewVec2f(0, 1)
	spinner.processLazer(player, 199)
	rotationBeforeEnd := state.totalAccumulatedRotation
	if rotationBeforeEnd == 0 {
		t.Fatal("test setup did not produce an in-window rotation")
	}

	player.cursor.RawPosition = vector.NewVec2f(-1, 0)
	spinner.processLazer(player, 200)
	if state.totalAccumulatedRotation != rotationBeforeEnd {
		t.Fatalf("rotation at EndTime = %v, want unchanged %v", state.totalAccumulatedRotation, rotationBeforeEnd)
	}
}

func TestProcessLazerRelaxReportsWithoutButton(t *testing.T) {
	spinner, player, state := newLazerSpinnerProcessTest()
	player.gameDownState = false
	player.diff.SetMods(difficulty.Relax)

	spinner.processLazer(player, 100)
	player.cursor.RawPosition = vector.NewVec2f(0, 1)
	spinner.processLazer(player, 150)

	if state.totalAccumulatedRotation == 0 {
		t.Fatal("Relax did not report an in-window rotation without a button")
	}
}

func TestProcessLazerAutopilotDoesNotSynthesizeSpinnerRotation(t *testing.T) {
	spinner, player, state := newLazerSpinnerProcessTest()
	player.diff.SetMods(difficulty.Autopilot)
	player.gameDownState = false

	spinner.processLazer(player, 100)
	player.cursor.RawPosition = vector.NewVec2f(0, 1)
	spinner.processLazer(player, 150)

	if state.totalAccumulatedRotation != 0 {
		t.Fatalf("Autopilot synthesized %v degrees of Lazer spinner rotation", state.totalAccumulatedRotation)
	}
}

func TestReportLazerRotationDeltaCountsMultipleSpins(t *testing.T) {
	state := spinnerState{}
	reportLazerRotationDelta(&state, 720)

	if state.rotationCount != 2 || state.totalRotation() != 720 {
		t.Fatalf("multi-spin delta produced %d rotations and %v total rotation, want 2 and 720", state.rotationCount, state.totalRotation())
	}
}

func TestReportLazerRotationDeltaHoldsMaximumDuringDirectionReversal(t *testing.T) {
	state := spinnerState{}
	reportLazerRotationDelta(&state, 40)
	reportLazerRotationDelta(&state, -90)
	reportLazerRotationDelta(&state, 110)

	if got := state.totalRotation(); got != 60 {
		t.Fatalf("direction reversal total rotation = %v, want 60", got)
	}
}

func newLazerSpinnerProcessTest() (*Spinner, *difficultyPlayer, *spinnerState) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayLazer)

	player := &difficultyPlayer{
		cursor:        &graphics.Cursor{RawPosition: vector.NewVec2f(1, 0)},
		diff:          diff,
		gameDownState: true,
	}
	state := &spinnerState{requirement: 10}
	spinner := &Spinner{
		hitSpinner: objects.NewDummySpinner(100, 200),
		state:      map[*difficultyPlayer]*spinnerState{player: state},
	}

	return spinner, player, state
}
