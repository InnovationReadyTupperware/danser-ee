package evaluators

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
)

func EvaluateAgility(current *preprocessing.DifficultyObject) float64 {
	if current.IsSpinner {
		return 0
	}

	const distanceCap = preprocessing.NormalizedDiameter * 1.2
	previous := current.Previous(0)
	travelDistance := 0.0
	if previous != nil {
		travelDistance = previous.LazyTravelDistance
	}

	distance := travelDistance + current.LazyJumpDistance
	distanceScaled := min(distance, distanceCap) / distanceCap
	difficulty := distanceScaled * 1000 / current.AdjustedDeltaTime
	difficulty *= math.Pow(current.SmallCircleBonus, 1.5)
	difficulty *= 1 / (1 - math.Pow(0.2, current.AdjustedDeltaTime/1000))
	return difficulty
}
