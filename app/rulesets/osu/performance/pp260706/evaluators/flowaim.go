package evaluators

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

func EvaluateFlowAim(current *preprocessing.DifficultyObject, withSliderTravelDistance bool) float64 {
	previous := current.Previous(0)
	if current.IsSpinner || current.Index <= 1 || previous == nil || previous.IsSpinner {
		return 0
	}

	const velocityChangeMultiplier = 0.52
	lastLast := current.Previous(1)

	currDistance := current.JumpDistance
	prevDistance := previous.JumpDistance
	if withSliderTravelDistance {
		currDistance = current.LazyJumpDistance
		prevDistance = previous.LazyJumpDistance
	}

	currVelocity := currDistance / current.AdjustedDeltaTime
	if previous.IsSlider && withSliderTravelDistance {
		sliderDistance := previous.LazyTravelDistance + current.LazyJumpDistance
		currVelocity = max(currVelocity, sliderDistance/current.AdjustedDeltaTime)
	}
	prevVelocity := prevDistance / previous.AdjustedDeltaTime

	flowDifficulty := currVelocity
	flowDifficulty *= math.Sqrt(current.SmallCircleBonus)
	flowDifficulty *= 1 + min(0.25,
		putils.PowInt((max(current.AdjustedDeltaTime, previous.AdjustedDeltaTime)-min(current.AdjustedDeltaTime, previous.AdjustedDeltaTime))/50, 4))

	if !math.IsNaN(current.Angle) && !math.IsNaN(previous.Angle) {
		angleDifference := math.Abs(current.Angle - previous.Angle)
		angleDifferenceAdjusted := math.Sin(angleDifference/2) * 180
		angularVelocity := angleDifferenceAdjusted / (current.AdjustedDeltaTime * 0.1)
		flowDifficulty *= 0.8 + math.Sqrt(angularVelocity/270)
	}

	overlappedNotesWeight := 1.0
	if current.Index > 2 && lastLast != nil {
		o1 := calculateOverlapFactor(current, previous)
		o2 := calculateOverlapFactor(current, lastLast)
		o3 := calculateOverlapFactor(previous, lastLast)
		overlappedNotesWeight = 1 - o1*o2*o3
	}

	if !math.IsNaN(current.Angle) {
		flowDifficulty += currVelocity * calcAcuteAngleBonus(current.Angle) * overlappedNotesWeight
	}

	if max(prevVelocity, currVelocity) != 0 {
		if withSliderTravelDistance {
			currVelocity = currDistance / current.AdjustedDeltaTime
		}
		distRatio := putils.Smoothstep(math.Abs(prevVelocity-currVelocity)/max(prevVelocity, currVelocity), 0, 1)
		overlapVelocityBuff := min(preprocessing.NormalizedDiameter*1.25/min(current.AdjustedDeltaTime, previous.AdjustedDeltaTime), math.Abs(prevVelocity-currVelocity))
		flowDifficulty += overlapVelocityBuff * distRatio * overlappedNotesWeight * velocityChangeMultiplier
	}

	if current.IsSlider && withSliderTravelDistance {
		flowDifficulty += current.TravelDistance / current.TravelTime
	}

	flowDifficulty = math.Pow(flowDifficulty, 1.45)
	return flowDifficulty * putils.Smootherstep(currDistance, 0, preprocessing.NormalizedRadius)
}

func calculateOverlapFactor(first, second *preprocessing.DifficultyObject) float64 {
	objectRadius := first.Diff.CircleRadiusL
	distance := float64(first.BaseObject.GetStackedStartPositionMod(first.Diff).Dst(second.BaseObject.GetStackedStartPositionMod(second.Diff)))
	return mutils.Clamp(1-putils.PowInt(max(distance-objectRadius, 0)/objectRadius, 2), 0, 1)
}
