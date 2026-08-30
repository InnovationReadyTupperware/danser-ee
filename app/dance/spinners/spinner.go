package spinners

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

type DanceSpinner struct {
	*objects.HitObject

	mover SpinnerMover
}

// NewSpinner converts a gameplay spinner into a generated cursor path. The
// selected profile and RPM are captured by the mover so later settings edits
// cannot change an object that is already queued.
func NewSpinner(spinner *objects.Spinner, diff *difficulty.Difficulty, moverCtor func() SpinnerMover) *DanceSpinner {
	hO := *spinner.HitObject

	if moverCtor == nil {
		moverCtor = GetMoverCtor(DefaultSpinnerProfile())
	}

	mover := moverCtor()
	if mover == nil {
		mover = GetMoverByName("circle")
	}

	speed := 1.0
	mode := difficulty.GameplayLazer
	if diff != nil {
		speed = diff.GetSpeed()
		mode = diff.GetGameplayMode()
	}

	mover.Initialize(hO.StartTime, speed, difficulty.SpinnerAutoplayRPM(diff, spinAtLowestRPMEnabled()), mode)

	danceSpinner := &DanceSpinner{
		HitObject: &hO,
		mover:     mover,
	}

	danceSpinner.PositionDelegate = mover.PositionAt
	danceSpinner.StartPosRaw = mover.PositionAt(danceSpinner.StartTime)
	danceSpinner.EndPosRaw = mover.PositionAt(danceSpinner.EndTime)

	return danceSpinner
}

func spinAtLowestRPMEnabled() bool {
	return settings.CursorDance != nil && settings.CursorDance.SpinnerBehavior != nil && settings.CursorDance.SpinnerBehavior.SpinAtLowestRPM
}

func (spinner *DanceSpinner) GetStartAngleMod(diff *difficulty.Difficulty) float32 {
	duration := spinner.GetDuration()
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		duration = 0
	}

	return spinner.GetStackedStartPositionMod(diff).AngleRV(spinner.GetStackedPositionAtMod(spinner.StartTime+min(10, duration), diff))
}

func (spinner *DanceSpinner) GetEndAngleMod(diff *difficulty.Difficulty) float32 {
	duration := spinner.GetDuration()
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		duration = 0
	}

	return spinner.GetStackedEndPositionMod(diff).AngleRV(spinner.GetStackedPositionAtMod(spinner.EndTime-min(10, duration), diff))
}

func (spinner *DanceSpinner) GetPartLen() float32 {
	duration := spinner.GetDuration()
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0
	}

	return 20 * spinner.mover.Radius() / float32(duration)
}

func (spinner *DanceSpinner) GetStackedPositionAtMod(time float64, _ *difficulty.Difficulty) vector.Vector2f {
	return spinner.GetPositionAt(time)
}

func (spinner *DanceSpinner) GetStackedStartPositionMod(_ *difficulty.Difficulty) vector.Vector2f {
	return spinner.GetStartPosition()
}

func (spinner *DanceSpinner) GetStackedEndPositionMod(_ *difficulty.Difficulty) vector.Vector2f {
	return spinner.GetEndPosition()
}

func (spinner *DanceSpinner) GetType() objects.Type {
	return objects.SPINNER
}
