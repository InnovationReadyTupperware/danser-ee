package curves

import (
	"github.com/wieku/danser-go/framework/math/vector"
)

// BSplineSolver retains the work buffers used by Solve. A mover can keep one
// solver alongside its reusable curve storage so dense queues do not allocate
// a temporary linear-system graph on every window rebuild.
type BSplineSolver struct {
	points       []vector.Vector2f
	d            []vector.Vector2f
	a            []vector.Vector2f
	bi           []float32
	bezierPoints []vector.Vector2f
}

func resizeBSplineBuffer[T any](buffer []T, length int) []T {
	if cap(buffer) < length {
		return make([]T, length)
	}

	return buffer[:length]
}

// Solve fills output with the non-approximated cubic Beziers for points. The
// returned slice aliases solver's retained work buffers and output's backing
// storage; callers must consume or copy it before the next Solve call.
func (solver *BSplineSolver) Solve(points1 []vector.Vector2f, output []Bezier) []Bezier {
	pointsLen := len(points1)
	if pointsLen < 4 {
		return output[:0]
	}

	points := resizeBSplineBuffer(solver.points, pointsLen)
	solver.points = points
	copy(points, points1[:1])
	copy(points[1:], points1[2:pointsLen-2])
	points[pointsLen-3] = points1[pointsLen-1]
	points[pointsLen-2] = points1[1]
	points[pointsLen-1] = points1[pointsLen-2]

	n := pointsLen - 2
	d := resizeBSplineBuffer(solver.d, n)
	a := resizeBSplineBuffer(solver.a, pointsLen)
	bi := resizeBSplineBuffer(solver.bi, pointsLen)
	solver.d, solver.a, solver.bi = d, a, bi

	d[0] = points[n].Sub(points[0])
	d[n-1] = points[n+1].Sub(points[n-1]).Scl(-1)

	bi[1] = -0.25
	a[1] = points[2].Sub(points[0]).Sub(d[0]).Scl(0.25)
	for i := 2; i < n-1; i++ {
		bi[i] = -1 / (4 + bi[i-1])
		a[i] = points[i+1].Sub(points[i-1]).Sub(a[i-1]).Scl(-1 * bi[i])
	}

	for i := n - 2; i > 0; i-- {
		d[i] = a[i].Add(d[i+1].Scl(bi[i]))
	}

	bezierPointCount := 3*n - 2
	bezierPoints := resizeBSplineBuffer(solver.bezierPoints, bezierPointCount)
	solver.bezierPoints = bezierPoints
	bezierPoints[0] = points[0]
	bezierPoints[1] = points[0].Add(d[0])

	pointIndex := 2
	for i := 1; i < n-1; i++ {
		bezierPoints[pointIndex] = points[i].Sub(d[i])
		bezierPoints[pointIndex+1] = points[i]
		bezierPoints[pointIndex+2] = points[i].Add(d[i])
		pointIndex += 3
	}

	bezierPoints[pointIndex] = points[n-1].Sub(d[n-1])
	bezierPoints[pointIndex+1] = points[n-1]

	bezierCount := (bezierPointCount - 1) / 3
	if cap(output) < bezierCount {
		output = make([]Bezier, bezierCount)
	} else {
		output = output[:bezierCount]
	}

	for i := range output {
		output[i].SetPoints(bezierPoints[i*3 : i*3+4])
	}

	return output
}

// NewBSpline creates a spline that goes through all given control points.
// points[1] and points[len(points)-2] are terminal tangents.
func NewBSpline(points []vector.Vector2f) *Spline {
	beziers := SolveBSpline(points)
	beziersC := make([]Curve, len(beziers))

	for i, b := range beziers {
		b.CalculateLength()
		beziersC[i] = b
	}

	return NewSpline(beziersC)
}

// NewBSplineW creates a spline that goes through all given control points with forced weights(lengths), useful when control points have to be passed at certain times.
// points[1] and points[len(points)-2] are terminal tangents.
func NewBSplineW(points []vector.Vector2f, weights []float32) *Spline {
	beziers := SolveBSpline(points)
	beziersC := make([]Curve, len(beziers))

	for i, b := range beziers {
		beziersC[i] = b
	}

	return NewSplineW(beziersC, weights)
}

// SolveBSpline calculates the spline that goes through all given control points.
// points[1] and points[len(points)-2] are terminal tangents
// Returns an array of bezier curves in NA (non-approximated) version for performance considerations.
func SolveBSpline(points1 []vector.Vector2f) []*Bezier {
	solver := &BSplineSolver{}
	values := solver.Solve(points1, nil)
	beziers := make([]*Bezier, len(values))
	for i := range values {
		beziers[i] = &values[i]
	}

	return beziers
}
