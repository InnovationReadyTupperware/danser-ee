package spinners

import (
	"github.com/go-gl/mathgl/mgl32"

	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/danser-go/framework/math/vector"
)

var (
	indicesSquare  = []mgl32.Vec3{{-1, -1, 0}, {1, -1, 0}, {1, 1, 0}, {-1, 1, 0}}
	squareVertices = []vector.Vector2f{
		vector.NewVec2f(-1, -1),
		vector.NewVec2f(1, -1),
		vector.NewVec2f(1, 1),
		vector.NewVec2f(-1, 1),
	}
)

// SquareMover traces either the historical Stable path or a perimeter whose
// polar phase is controlled directly by the selected Lazer RPM.
type SquareMover struct {
	*BaseMover
}

func newSquareMover(profile SpinnerProfile) *SquareMover {
	return &SquareMover{BaseMover: &BaseMover{profile: profile}}
}

// NewSquareMover creates a centered square mover for callers that do not need
// a custom cursor-dance profile.
func NewSquareMover() *SquareMover {
	return newSquareMover(DefaultSpinnerProfile())
}

func (mover *SquareMover) PositionAt(time float64) vector.Vector2f {
	if mover.mode.IsLazer() {
		angle := mover.angleAt(time)
		radius := mover.profile.Radius * polygonRadiusAtAngle(squareVertices, angle)
		return mover.polarPosition(angle, radius)
	}

	phase := mover.phaseAt(time)
	mat := mgl32.Rotate3DZ(phase / 2000 * 2 * math32.Pi).Mul3(mgl32.Scale2D(float32(mover.profile.Radius), float32(mover.profile.Radius)))

	startIndex := (int64(max(float32(0), phase)) / 10) % int64(len(indicesSquare))
	endIndex := (startIndex + 1) % int64(len(indicesSquare))

	start := mat.Mul3x1(indicesSquare[startIndex])
	end := mat.Mul3x1(indicesSquare[endIndex])
	t := float32(int64(phase)%10) / 10

	path := vector.NewVec2f((end.X()-start.X())*t+start.X(), (end.Y()-start.Y())*t+start.Y())
	return path.Add(mover.profileCenter())
}
