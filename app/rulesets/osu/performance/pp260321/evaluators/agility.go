package evaluators

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260321/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
)

const (
	distanceCap float64 = preprocessing.NormalizedDiameter * 1.25
)

func EvaluateAgility(current *preprocessing.DifficultyObject) float64 {
	if current.IsSpinner {
		return 0
	}

	osuCurrObj := current
	osuPrevObj := current.Previous(0)

	travelDistance := 0.0
	if osuPrevObj != nil {
		travelDistance = osuPrevObj.LazyTravelDistance
	}

	distance := travelDistance + osuCurrObj.LazyJumpDistance

	distanceScaled := min(distance, distanceCap) / distanceCap

	strain := distanceScaled * 1000 / osuCurrObj.AdjustedDeltaTime

	strain *= osuCurrObj.SmallCircleBonus

	strain *= aimHighBpmBonus(osuCurrObj.AdjustedDeltaTime)

	return strain * putils.Smootherstep(distance, 0, preprocessing.NormalizedRadius)
}

func aimHighBpmBonus(ms float64) float64 {
	return 1 / (1 - math.Pow(0.15, ms/1000))
}
