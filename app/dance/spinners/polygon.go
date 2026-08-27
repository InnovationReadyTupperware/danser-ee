package spinners

import (
	"math"

	"github.com/wieku/danser-go/framework/math/vector"
)

// polygonRadiusAtAngle returns the distance from a polygon's center to its
// outline along the supplied ray. Driving a polygon this way keeps its polar
// angle authoritative: one complete angular turn always means one complete
// perimeter traversal, regardless of the edge lengths.
func polygonRadiusAtAngle(vertices []vector.Vector2f, angle float64) float64 {
	if len(vertices) < 3 {
		return 1
	}

	direction := vector.NewVec2f(float32(math.Cos(angle)), float32(math.Sin(angle)))
	bestDistance := math.Inf(1)

	for i, start := range vertices {
		end := vertices[(i+1)%len(vertices)]
		edge := end.Sub(start)
		denominator := crossProduct(direction, edge)

		if math.Abs(denominator) < 1e-9 {
			continue
		}

		distance := crossProduct(start, edge) / denominator
		edgePosition := crossProduct(start, direction) / denominator

		if distance > 0 && edgePosition >= -1e-6 && edgePosition <= 1+1e-6 {
			bestDistance = min(bestDistance, distance)
		}
	}

	if math.IsInf(bestDistance, 1) || math.IsNaN(bestDistance) {
		return 1
	}

	return bestDistance
}

func crossProduct(a, b vector.Vector2f) float64 {
	return float64(a.X)*float64(b.Y) - float64(a.Y)*float64(b.X)
}
