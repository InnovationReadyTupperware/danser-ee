package evaluators

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	readingWindowSize                 = 3000.0
	readingDistanceInfluenceThreshold = preprocessing.NormalizedDiameter * 1.5
)

func EvaluateReading(current *preprocessing.DifficultyObject, hidden bool) float64 {
	if current.IsSpinner || current.Index == 0 {
		return 0
	}

	nextObj := current.Next(0)
	velocity := max(1, current.LazyJumpDistance/current.AdjustedDeltaTime)
	currentVisibleObjectDensity := retrieveCurrentVisibleObjectDensity(current)
	pastObjectDifficultyInfluence := getPastObjectDifficultyInfluence(current)
	constantAngleNerfFactor := getConstantAngleNerfFactor(current)

	noteDensityDifficulty := calculateDensityDifficulty(nextObj, velocity, constantAngleNerfFactor, pastObjectDifficultyInfluence, currentVisibleObjectDensity)
	hiddenDifficulty := 0.0
	if hidden {
		hiddenDifficulty = calculateHiddenDifficulty(current, pastObjectDifficultyInfluence, currentVisibleObjectDensity, velocity, constantAngleNerfFactor)
	}
	preemptDifficulty := calculatePreemptDifficulty(velocity, constantAngleNerfFactor, current.Preempt)

	readingDifficulty := putils.Norm(1.5, preemptDifficulty, hiddenDifficulty, noteDensityDifficulty)
	readingDifficulty *= 1 / (1 - math.Pow(0.8, current.AdjustedDeltaTime/1000))
	return readingDifficulty
}

func calculateDensityDifficulty(nextObj *preprocessing.DifficultyObject, velocity, constantAngleNerfFactor, pastObjectDifficultyInfluence, currentVisibleObjectDensity float64) float64 {
	const (
		densityMultiplier     = 2.4
		densityDifficultyBase = 2.5
	)

	futureObjectDifficultyInfluence := math.Sqrt(currentVisibleObjectDensity)
	if nextObj != nil {
		futureObjectDifficultyInfluence *= putils.Smootherstep(nextObj.LazyJumpDistance, 15, readingDistanceInfluenceThreshold)
	}

	noteDensityDifficulty := math.Pow(pastObjectDifficultyInfluence+futureObjectDifficultyInfluence, 1.7) * 0.4 * constantAngleNerfFactor * velocity
	noteDensityDifficulty = max(0, noteDensityDifficulty-densityDifficultyBase)
	return math.Pow(noteDensityDifficulty, 0.45) * densityMultiplier
}

func calculatePreemptDifficulty(velocity, constantAngleNerfFactor, preempt float64) float64 {
	const (
		preemptBalancingFactor = 140000.0
		preemptStartingPoint   = 500.0
	)
	value := math.Pow((preemptStartingPoint-preempt+math.Abs(preempt-preemptStartingPoint))/2, 2.5) / preemptBalancingFactor
	return value * constantAngleNerfFactor * velocity
}

func calculateHiddenDifficulty(current *preprocessing.DifficultyObject, pastObjectDifficultyInfluence, currentVisibleObjectDensity, velocity, constantAngleNerfFactor float64) float64 {
	const hiddenMultiplier = 0.28
	preemptFactor := math.Pow(current.Preempt, 2.2) * 0.01
	densityFactor := math.Pow(currentVisibleObjectDensity+pastObjectDifficultyInfluence, 3.3) * 3
	hiddenDifficulty := (preemptFactor + densityFactor) * constantAngleNerfFactor * velocity * 0.01
	hiddenDifficulty = math.Pow(hiddenDifficulty, 0.4) * hiddenMultiplier

	previous := current.Previous(0)
	if previous != nil && current.LazyJumpDistance == 0 && current.OpacityAt(previous.BaseObject.GetStartTime(), true) == 0 && previous.StartTime > current.StartTime-current.Preempt {
		hiddenDifficulty += hiddenMultiplier * 2500 / math.Pow(current.AdjustedDeltaTime, 1.5)
	}
	return hiddenDifficulty
}

func getPastObjectDifficultyInfluence(current *preprocessing.DifficultyObject) float64 {
	var influence float64
	for _, loopObj := range retrievePastVisibleObjects(current) {
		loopDifficulty := current.OpacityAt(loopObj.BaseObject.GetStartTime(), false)
		loopDifficulty *= putils.Smootherstep(loopObj.LazyJumpDistance, 15, readingDistanceInfluenceThreshold)
		loopDifficulty *= getTimeNerfFactor(current.StartTime - loopObj.StartTime)
		influence += loopDifficulty
	}
	return influence
}

func retrievePastVisibleObjects(current *preprocessing.DifficultyObject) []*preprocessing.DifficultyObject {
	objects := make([]*preprocessing.DifficultyObject, 0)
	for i := 0; i < current.Index; i++ {
		hitObject := current.Previous(i)
		if hitObject == nil || current.StartTime-hitObject.StartTime > readingWindowSize || hitObject.StartTime < current.StartTime-current.Preempt {
			break
		}
		objects = append(objects, hitObject)
	}
	return objects
}

func retrieveCurrentVisibleObjectDensity(current *preprocessing.DifficultyObject) float64 {
	var visibleObjectCount float64
	for hitObject := current.Next(0); hitObject != nil; hitObject = hitObject.Next(0) {
		if hitObject.StartTime-current.StartTime > readingWindowSize || current.StartTime < hitObject.StartTime-hitObject.Preempt {
			break
		}
		timeNerfFactor := getTimeNerfFactor(hitObject.StartTime - current.StartTime)
		visibleObjectCount += hitObject.OpacityAt(current.BaseObject.GetStartTime(), false) * timeNerfFactor
	}
	return visibleObjectCount
}

func getConstantAngleNerfFactor(current *preprocessing.DifficultyObject) float64 {
	const (
		minimumAngleRelevancyTime = 2000.0
		maximumAngleRelevancyTime = 200.0
	)

	constantAngleCount := 0.0
	index := 0
	currentTimeGap := 0.0
	loopObjPrev0 := current
	var loopObjPrev1 *preprocessing.DifficultyObject
	var loopObjPrev2 *preprocessing.DifficultyObject

	for currentTimeGap < minimumAngleRelevancyTime {
		loopObj := current.Previous(index)
		if loopObj == nil {
			break
		}

		longIntervalFactor := 1 - putils.ReverseLerp(loopObj.AdjustedDeltaTime, maximumAngleRelevancyTime, minimumAngleRelevancyTime)
		if !math.IsNaN(loopObj.Angle) && !math.IsNaN(current.Angle) {
			angleDifference := math.Abs(current.Angle - loopObj.Angle)
			angleDifferenceAlternating := math.Pi

			if loopObjPrev0 != nil && loopObjPrev1 != nil && loopObjPrev2 != nil &&
				!math.IsNaN(loopObjPrev0.Angle) && !math.IsNaN(loopObjPrev1.Angle) && !math.IsNaN(loopObjPrev2.Angle) {
				angleDifferenceAlternating = math.Abs(loopObjPrev1.Angle-loopObj.Angle) + math.Abs(loopObjPrev2.Angle-loopObjPrev0.Angle)
				weight := putils.ReverseLerp(min(loopObj.Angle, loopObjPrev0.Angle)*180/math.Pi, 20, 5)
				weight *= putils.ReverseLerp(max(loopObj.Angle, loopObjPrev0.Angle)*180/math.Pi, 60, 120)
				angleDifferenceAlternating = mutils.Lerp(math.Pi, 0.1*angleDifferenceAlternating, weight)
			}

			stackFactor := putils.Smootherstep(loopObj.LazyJumpDistance, 0, preprocessing.NormalizedRadius)
			constantAngleCount += math.Cos(3*min(putils.DegreesToRadians(30), min(angleDifference, angleDifferenceAlternating)*stackFactor)) * longIntervalFactor
		}

		currentTimeGap = current.StartTime - loopObj.StartTime
		index++
		loopObjPrev2 = loopObjPrev1
		loopObjPrev1 = loopObjPrev0
		loopObjPrev0 = loopObj
	}

	return mutils.Clamp(2/constantAngleCount, 0.2, 1)
}

func getTimeNerfFactor(deltaTime float64) float64 {
	return mutils.Clamp(2-deltaTime/(readingWindowSize/2), 0, 1)
}
