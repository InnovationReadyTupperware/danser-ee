package spinners

import "github.com/wieku/danser-go/framework/math/vector"

// CircleMover traces the simplest spinner path: a constant-radius orbit.
type CircleMover struct {
	*BaseMover
}

func newCircleMover(profile SpinnerProfile) *CircleMover {
	return &CircleMover{BaseMover: &BaseMover{profile: profile}}
}

// NewCircleMover creates a centered circle mover for callers that do not need
// a custom cursor-dance profile.
func NewCircleMover() *CircleMover {
	return newCircleMover(DefaultSpinnerProfile())
}

func (mover *CircleMover) PositionAt(time float64) vector.Vector2f {
	return mover.polarPosition(mover.angleAt(time), mover.profile.Radius)
}
