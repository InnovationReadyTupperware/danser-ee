package spinners

import (
	"strings"

	"github.com/wieku/danser-go/framework/math/vector"
)

const (
	// baseSpinnerRotationsPerMillisecond is the historical danser spinner
	// movement rate. Keeping it as the phase base means Stable-mode movement
	// remains bit-for-bit compatible while Lazer mode can scale the same
	// mover shapes to its OD-specific completion rate.
	baseSpinnerRotationsPerMillisecond = 0.00795
	legacySpinnerRPM                   = baseSpinnerRotationsPerMillisecond * 60000
)

var center = vector.NewVec2f(256, 192)

// SpinnerMover supplies a cursor path for a spinner in cursor dance mode.
// Implementations embed BaseMover so the selected RPM and speed-modifier
// behavior are applied consistently across all available shapes.
type SpinnerMover interface {
	Init(start, end float64, id int, speed, rpm float64)
	GetPositionAt(time float64) vector.Vector2f
	GetSDelta(time float64) float32
}

// BaseMover stores the timing and rate shared by the shape-specific spinner
// movers. The rate is represented as a multiplier over the historical phase
// so changing RPM does not require each shape to duplicate speed handling.
type BaseMover struct {
	start, end     float64
	speed          float64
	rateMultiplier float64
	id             int
}

func (mover *BaseMover) Init(start, end float64, id int, speed, rpm float64) {
	mover.start = start
	mover.end = end
	mover.id = id
	mover.speed = speed
	mover.rateMultiplier = rpm / legacySpinnerRPM
}

func (mover *BaseMover) GetSDelta(time float64) float32 {
	return float32((time - mover.start) / min(1, mover.speed) * mover.rateMultiplier)
}

func GetMoverByName(name string) SpinnerMover {
	switch strings.ToLower(name) {
	case "heart":
		return NewHeartMover()
	case "triangle":
		return NewTriangleMover()
	case "square":
		return NewSquareMover()
	case "cube":
		return NewCubeMover()
	default:
		return NewCircleMover()
	}
}

func GetMoverCtorByName(name string) func() SpinnerMover {
	return func() SpinnerMover {
		return GetMoverByName(name)
	}
}
