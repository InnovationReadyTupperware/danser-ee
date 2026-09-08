package evaluators

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
)

func EvaluateSnapAim(current *preprocessing.DifficultyObject, withSliderTravelDistance bool) float64 {
	previous := current.Previous(0)
	if current.IsSpinner || current.Index <= 1 || previous == nil || previous.IsSpinner {
		return 0
	}

	const (
		wideAngleMultiplier      = 9.67
		acuteAngleMultiplier     = 2.41
		sliderMultiplier         = 1.5
		velocityChangeMultiplier = 0.9
		wiggleMultiplier         = 1.02
	)

	last2 := current.Previous(2)
	const (
		radius   = preprocessing.NormalizedRadius
		diameter = preprocessing.NormalizedDiameter
	)

	currDistance := current.JumpDistance
	if withSliderTravelDistance {
		currDistance = current.LazyJumpDistance
	}
	currVelocity := currDistance / current.AdjustedDeltaTime

	if previous.IsSlider && withSliderTravelDistance {
		sliderDistance := previous.LazyTravelDistance + current.LazyJumpDistance
		currVelocity = max(currVelocity, sliderDistance/current.AdjustedDeltaTime)
	}

	prevDistance := previous.JumpDistance
	if withSliderTravelDistance {
		prevDistance = previous.LazyJumpDistance
	}
	prevVelocity := prevDistance / previous.AdjustedDeltaTime

	snapDifficulty := currVelocity
	snapDifficulty *= vectorAngleRepetition(current, previous)

	if !math.IsNaN(current.Angle) && !math.IsNaN(previous.Angle) {
		currAngle := current.Angle
		lastAngle := previous.Angle
		velocityInfluence := min(currVelocity, prevVelocity)

		acuteAngleBonus := 0.0
		if max(current.AdjustedDeltaTime, previous.AdjustedDeltaTime) < 1.25*min(current.AdjustedDeltaTime, previous.AdjustedDeltaTime) {
			acuteAngleBonus = calcAcuteAngleBonus(currAngle)
			acuteAngleBonus *= 0.08 + 0.92*(1-min(acuteAngleBonus, putils.PowInt(calcAcuteAngleBonus(lastAngle), 3)))
			acuteAngleBonus *= velocityInfluence *
				putils.Smootherstep(putils.MillisecondsToBPM(current.AdjustedDeltaTime, 2), 300, 400) *
				putils.Smootherstep(currDistance, 0, diameter*2)
		}

		wideAngleBonus := calcWideAngleBonus(currAngle)
		wideAngleBonus *= 0.25 + 0.75*(1-min(wideAngleBonus, putils.PowInt(calcWideAngleBonus(lastAngle), 3)))

		const wideAngleTimeScale = 1.45
		wideAngleCurrVelocity := currDistance / math.Pow(current.AdjustedDeltaTime, wideAngleTimeScale)
		wideAnglePrevVelocity := prevDistance / math.Pow(previous.AdjustedDeltaTime, wideAngleTimeScale)
		if previous.IsSlider && withSliderTravelDistance {
			sliderDistance := previous.LazyTravelDistance + current.LazyJumpDistance
			wideAngleCurrVelocity = max(wideAngleCurrVelocity, sliderDistance/math.Pow(current.AdjustedDeltaTime, wideAngleTimeScale))
		}
		wideAngleBonus *= min(wideAngleCurrVelocity, wideAnglePrevVelocity)

		if last2 != nil {
			lastBase := previous.BaseObject
			last2Base := last2.BaseObject
			distance := float64(last2Base.GetStackedStartPositionMod(last2.Diff).Dst(lastBase.GetStackedStartPositionMod(previous.Diff)))
			if distance < 1 {
				wideAngleBonus *= 1 - 0.55*(1-distance)
			}
		}

		snapDifficulty += max(acuteAngleBonus*acuteAngleMultiplier, wideAngleBonus*wideAngleMultiplier)

		wiggleBonus := velocityInfluence *
			putils.Smootherstep(currDistance, radius, diameter) *
			math.Pow(putils.ReverseLerp(currDistance, diameter*3, diameter), 1.8) *
			putils.Smootherstep(currAngle, putils.DegreesToRadians(110), putils.DegreesToRadians(60)) *
			putils.Smootherstep(prevDistance, radius, diameter) *
			math.Pow(putils.ReverseLerp(prevDistance, diameter*3, diameter), 1.8) *
			putils.Smootherstep(lastAngle, putils.DegreesToRadians(110), putils.DegreesToRadians(60))
		snapDifficulty += wiggleBonus * wiggleMultiplier
	}

	if max(prevVelocity, currVelocity) != 0 {
		if withSliderTravelDistance {
			currVelocity = currDistance / current.AdjustedDeltaTime
		}
		distRatio := putils.Smoothstep(math.Abs(prevVelocity-currVelocity)/max(prevVelocity, currVelocity), 0, 1)
		overlapVelocityBuff := min(diameter*1.25/min(current.AdjustedDeltaTime, previous.AdjustedDeltaTime), math.Abs(prevVelocity-currVelocity))
		velocityChangeBonus := overlapVelocityBuff * distRatio
		velocityChangeBonus *= putils.PowInt(min(current.AdjustedDeltaTime, previous.AdjustedDeltaTime)/max(current.AdjustedDeltaTime, previous.AdjustedDeltaTime), 2)
		snapDifficulty += velocityChangeBonus * velocityChangeMultiplier
	}

	if current.IsSlider && withSliderTravelDistance {
		sliderBonus := current.TravelDistance / current.TravelTime
		if sliderBonus < 1 {
			snapDifficulty += sliderBonus * sliderMultiplier
		} else {
			snapDifficulty += math.Pow(sliderBonus, 0.75) * sliderMultiplier
		}
	}

	snapDifficulty *= current.SmallCircleBonus
	snapDifficulty *= 1 / (1 - math.Pow(0.03, math.Pow(current.AdjustedDeltaTime/1000, 0.65)))
	return snapDifficulty
}

func vectorAngleRepetition(current, previous *preprocessing.DifficultyObject) float64 {
	if math.IsNaN(current.Angle) || math.IsNaN(previous.Angle) {
		return 1
	}

	const (
		noteLimit              = 6
		maximumRepetitionNerf  = 0.15
		maximumVectorInfluence = 0.5
	)

	constantAngleCount := 0.0
	for index := 0; index < noteLimit; index++ {
		prevObj := current.Previous(index)
		if prevObj == nil {
			break
		}
		if max(current.AdjustedDeltaTime, prevObj.AdjustedDeltaTime) > 1.1*min(current.AdjustedDeltaTime, prevObj.AdjustedDeltaTime) {
			break
		}
		if !math.IsNaN(prevObj.NormalisedVectorAngle) && !math.IsNaN(current.NormalisedVectorAngle) {
			angleDifference := math.Abs(current.NormalisedVectorAngle - prevObj.NormalisedVectorAngle)
			constantAngleCount += math.Cos(8 * min(putils.DegreesToRadians(11.25), angleDifference))
		}
	}

	vectorRepetition := putils.PowInt(min(0.5/constantAngleCount, 1), 2)
	stackFactor := putils.Smootherstep(current.LazyJumpDistance, 0, preprocessing.NormalizedDiameter)
	angleDifferenceAdjusted := math.Cos(2 * min(putils.DegreesToRadians(45), math.Abs(current.Angle-previous.Angle)*stackFactor))
	baseNerf := 1 - maximumRepetitionNerf*calcAcuteAngleBonus(previous.Angle)*angleDifferenceAdjusted
	return putils.PowInt(baseNerf+(1-baseNerf)*vectorRepetition*maximumVectorInfluence*stackFactor, 2)
}

func calcWideAngleBonus(angle float64) float64 {
	return putils.Smoothstep(angle, putils.DegreesToRadians(40), putils.DegreesToRadians(140))
}

func calcAcuteAngleBonus(angle float64) float64 {
	return putils.Smoothstep(angle, putils.DegreesToRadians(140), putils.DegreesToRadians(40))
}
