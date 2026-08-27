package spinners

import (
	"github.com/go-gl/mathgl/mgl32"

	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/danser-go/framework/math/vector"
)

var (
	indicesTriangle  = []mgl32.Vec3{{-0.86602540378, -0.5, 0}, {0.86602540378, -0.5, 0}, {0, 1, 0}}
	triangleVertices = []vector.Vector2f{
		vector.NewVec2f(-0.86602540378, -0.5),
		vector.NewVec2f(0.86602540378, -0.5),
		vector.NewVec2f(0, 1),
	}
)

// TriangleMover traces either the historical Stable path or a perimeter
// whose polar phase is controlled directly by the selected Lazer RPM.
type TriangleMover struct {
	*BaseMover
}

func newTriangleMover(profile SpinnerProfile) *TriangleMover {
	return &TriangleMover{BaseMover: &BaseMover{profile: profile}}
}

// NewTriangleMover creates a centered triangle mover for callers that do not
// need a custom cursor-dance profile.
func NewTriangleMover() *TriangleMover {
	return newTriangleMover(DefaultSpinnerProfile())
}

func (mover *TriangleMover) PositionAt(time float64) vector.Vector2f {
	if mover.mode.IsLazer() {
		angle := mover.angleAt(time)
		radius := mover.profile.Radius * polygonRadiusAtAngle(triangleVertices, angle)
		return mover.polarPosition(angle, radius)
	}

	phase := mover.phaseAt(time)
	mat := mgl32.Rotate3DZ(phase / 2000 * 2 * math32.Pi).Mul3(mgl32.Scale2D(float32(mover.profile.Radius), float32(mover.profile.Radius)))

	startIndex := (int64(max(float32(0), phase)) / 10) % int64(len(indicesTriangle))
	endIndex := (startIndex + 1) % int64(len(indicesTriangle))

	start := mat.Mul3x1(indicesTriangle[startIndex])
	end := mat.Mul3x1(indicesTriangle[endIndex])
	t := float32(int64(phase)%10) / 10

	path := vector.NewVec2f((end.X()-start.X())*t+start.X(), (end.Y()-start.Y())*t+start.Y())
	return path.Add(mover.profileCenter())
}
