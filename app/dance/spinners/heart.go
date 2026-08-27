package spinners

import (
	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/danser-go/framework/math/vector"
)

// HeartMover uses the same angular phase as CircleMover and changes only the
// radius as a function of that phase. Keeping one phase source prevents shape
// selection from changing the configured RPM.
type HeartMover struct {
	*BaseMover
}

func newHeartMover(profile SpinnerProfile) *HeartMover {
	return &HeartMover{BaseMover: &BaseMover{profile: profile}}
}

// NewHeartMover creates a centered heart mover for callers that do not need a
// custom cursor-dance profile.
func NewHeartMover() *HeartMover {
	return newHeartMover(DefaultSpinnerProfile())
}

func (mover *HeartMover) PositionAt(time float64) vector.Vector2f {
	rad := float32(mover.angleAt(time))
	x := math32.Pow(math32.Sin(rad), 3)
	y := (13*math32.Cos(rad) - 5*math32.Cos(2*rad) - 2*math32.Cos(3*rad) - math32.Cos(4*rad)) / 16

	path := vector.NewVec2f(x, y).Mult(vector.NewVec2f(float32(mover.profile.Radius), -float32(mover.profile.Radius)))
	return path.Add(mover.profileCenter())
}
