package spinners

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestSpinnerRPMSelectionUsesLazerCompletionRange(t *testing.T) {
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := difficulty.NewDifficulty(5, 5, test.od, 5)
			if got := difficulty.SpinnerAutoplayRPM(diff, true); math.Abs(got-test.want) > 0.000001 {
				t.Fatalf("SpinnerAutoplayRPM(OD %.1f) = %v, want %v", test.od, got, test.want)
			}
		})
	}
}

func TestSpinnerRPMSelectionUsesFixedMaximumForLazer(t *testing.T) {
	for _, od := range []float64{0, 5, 10, 11} {
		diff := difficulty.NewDifficulty(5, 5, od, 5)
		if got := difficulty.SpinnerAutoplayRPM(diff, false); got != difficulty.LazerSpinnerMaximumCompletionRPM {
			t.Fatalf("SpinnerAutoplayRPM(OD %.1f) = %v, want fixed maximum %v", od, got, difficulty.LazerSpinnerMaximumCompletionRPM)
		}
	}
}

func TestSpinnerRPMSelectionPreservesStableModeWithClassic(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 11, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)
	diff.SetMods(difficulty.Classic)

	if got := difficulty.SpinnerAutoplayRPM(diff, true); got != difficulty.LegacySpinnerRPM {
		t.Fatalf("SpinnerAutoplayRPM(Stable + Classic) = %v, want legacy rate %v", got, difficulty.LegacySpinnerRPM)
	}
}

func TestSpinnerMoverRejectsInvalidTimingAndRate(t *testing.T) {
	mover := NewCircleMover()
	mover.Initialize(100, 0, math.NaN(), difficulty.GameplayLazer)

	for _, time := range []float64{math.Inf(-1), math.NaN(), 0, 99, 100, 101} {
		position := mover.PositionAt(time)
		if !finitePosition(position) {
			t.Fatalf("PositionAt(%v) = %v, want finite position", time, position)
		}
	}
}

func TestSpinnerMoverProfilesFallBackToSafeDefaults(t *testing.T) {
	previous := settings.CursorDance
	t.Cleanup(func() {
		settings.CursorDance = previous
	})

	settings.CursorDance = nil
	if got := ResolveSpinnerProfile(0); got != DefaultSpinnerProfile() {
		t.Fatalf("nil CursorDance profile = %+v, want %+v", got, DefaultSpinnerProfile())
	}

	config := settings.NewConfigFile()
	config.CursorDance.Spinners = nil
	settings.CursorDance = config.CursorDance
	if got := ResolveSpinnerProfile(0); got != DefaultSpinnerProfile() {
		t.Fatalf("empty spinner list profile = %+v, want %+v", got, DefaultSpinnerProfile())
	}

	config = settings.NewConfigFile()
	configured := config.CursorDance.Spinners[0]
	configured.Mover = "not-a-mover"
	configured.Radius = 0
	configured.CenterOffsetX = math.NaN()
	configured.CenterOffsetY = math.Inf(1)
	settings.CursorDance = config.CursorDance

	if got := ResolveSpinnerProfile(0); got != DefaultSpinnerProfile() {
		t.Fatalf("invalid spinner profile = %+v, want %+v", got, DefaultSpinnerProfile())
	}

	config.CursorDance.Spinners[0] = nil
	if got := ResolveSpinnerProfile(0); got != DefaultSpinnerProfile() {
		t.Fatalf("nil spinner entry profile = %+v, want %+v", got, DefaultSpinnerProfile())
	}
}

func TestLazerPolygonAndCubeMoversFollowRequestedPolarRPM(t *testing.T) {
	const (
		targetRPM = 250.0
		duration  = 240.0
	)

	profiles := []SpinnerProfile{
		{Mover: "circle", Radius: 100},
		{Mover: "triangle", Radius: 100},
		{Mover: "square", Radius: 100},
		{Mover: "cube", Radius: 100},
	}

	for _, profile := range profiles {
		t.Run(profile.Mover, func(t *testing.T) {
			mover := GetMoverCtor(profile)()
			mover.Initialize(0, 1, targetRPM, difficulty.GameplayLazer)

			previousAngle := positionAngle(mover.PositionAt(0), vector.NewVec2f(256, 192))
			unwrapped := 0.0

			for time := 1.0; time <= duration; time++ {
				angle := positionAngle(mover.PositionAt(time), vector.NewVec2f(256, 192))
				delta := angle - previousAngle
				if delta > math.Pi {
					delta -= 2 * math.Pi
				} else if delta < -math.Pi {
					delta += 2 * math.Pi
				}
				unwrapped += delta
				previousAngle = angle
			}

			want := 2 * math.Pi * targetRPM * duration / 60000
			if math.Abs(unwrapped-want) > 0.02 {
				t.Fatalf("unwrapped polar angle = %v, want %v", unwrapped, want)
			}
		})
	}
}

func TestLazerHeartUsesTheSharedAngularPhase(t *testing.T) {
	mover := NewHeartMover()
	mover.Initialize(0, 1, 250, difficulty.GameplayLazer)

	first := mover.PositionAt(0)
	second := mover.PositionAt(120)
	if first == second {
		t.Fatal("heart mover did not advance after one requested rotation")
	}

	if !finitePosition(first) || !finitePosition(second) {
		t.Fatalf("heart positions must remain finite: %v, %v", first, second)
	}
}

func finitePosition(position vector.Vector2f) bool {
	return !math.IsNaN(float64(position.X)) && !math.IsNaN(float64(position.Y)) &&
		!math.IsInf(float64(position.X), 0) && !math.IsInf(float64(position.Y), 0)
}

func positionAngle(position, center vector.Vector2f) float64 {
	return math.Atan2(float64(position.Y-center.Y), float64(position.X-center.X))
}
