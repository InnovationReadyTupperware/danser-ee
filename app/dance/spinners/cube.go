package spinners

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/innovationreadytupperware/danser-ee/framework/math/math32"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

var cubeVertices = []mgl32.Vec4{
	{-1, -1, -1, 1},
	{-1, 1, -1, 1},
	{1, 1, -1, 1},
	{1, -1, -1, 1},
	{-1, -1, 1, 1},
	{-1, 1, 1, 1},
	{1, 1, 1, 1},
	{1, -1, 1, 1},
}

var cubeIndices = []int{0, 1, 2, 3, 0, 4, 5, 1, 5, 6, 2, 6, 7, 3, 7, 4}

// CubeMover preserves the old projected cube for Stable gameplay. Lazer
// gameplay keeps the cube's changing radius and depth illusion, then places
// that radius on the authoritative polar angle so the projection cannot
// silently alter the requested spinner RPM.
type CubeMover struct {
	*BaseMover
}

func newCubeMover(profile SpinnerProfile) *CubeMover {
	return &CubeMover{BaseMover: &BaseMover{profile: profile}}
}

// NewCubeMover creates a centered cube mover for callers that do not need a
// custom cursor-dance profile.
func NewCubeMover() *CubeMover {
	return newCubeMover(DefaultSpinnerProfile())
}

func (mover *CubeMover) PositionAt(time float64) vector.Vector2f {
	phase := mover.phaseAt(time)
	radY := math32.Sin(phase/9000*2*math32.Pi) * 3.0 / 18 * math32.Pi
	radX := math32.Sin(phase/5000*2*math32.Pi) * 3.0 / 18 * math32.Pi

	scale := (1.0 + math32.Sin(phase/4500*2*math32.Pi)*0.3) * float32(mover.profile.Radius)

	mat := mgl32.HomogRotate3DY(radY).Mul4(mgl32.HomogRotate3DX(radX)).Mul4(mgl32.Scale3D(scale, scale, scale))

	startIndex := (int64(max(float32(0), phase)) / 4) % int64(len(cubeIndices))
	start := cubeVertices[cubeIndices[startIndex]]

	endIndex := (startIndex + 1) % int64(len(cubeIndices))
	end := cubeVertices[cubeIndices[endIndex]]

	t := float32(int64(phase)%4) / 4
	point := mgl32.Vec4{
		(end.X()-start.X())*t + start.X(),
		(end.Y()-start.Y())*t + start.Y(),
		(end.Z()-start.Z())*t + start.Z(),
		1,
	}

	point = mat.Mul4x1(point)
	point[0] *= 1 + point[2]/scale/10
	point[1] *= 1 + point[2]/scale/10

	if !mover.mode.IsLazer() {
		return vector.NewVec2f(point.X(), point.Y()).Add(mover.profileCenter())
	}

	projectedRadius := math.Hypot(float64(point.X()), float64(point.Y()))
	if projectedRadius <= 0 || math.IsNaN(projectedRadius) || math.IsInf(projectedRadius, 0) {
		projectedRadius = mover.profile.Radius
	}

	return mover.polarPosition(mover.angleAt(time), projectedRadius)
}
