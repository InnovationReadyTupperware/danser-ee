package evaluators

import (
	"math"
	"slices"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	rhythmHistoryTimeMax    = 5000.0
	rhythmHistoryObjectsMax = 32
	rhythmOverallMultiplier = 0.95
)

func EvaluateRhythm(current *preprocessing.DifficultyObject) float64 {
	if current.IsSpinner {
		return 0
	}

	rhythmComplexitySum := 0.0
	deltaDifferenceEpsilon := current.GreatWindow * 0.3
	island := newIsland(math.MaxInt)
	previousIsland := newIsland(math.MaxInt)
	islands := make([]*rhythmIsland, 0)
	startDifficulty := 0.0
	firstDeltaSwitch := false

	historicalNoteCount := min(current.Index, rhythmHistoryObjectsMax)
	rhythmStart := 0
	for rhythmStart < historicalNoteCount-2 && current.StartTime-current.Previous(rhythmStart).StartTime < rhythmHistoryTimeMax {
		rhythmStart++
	}

	prevObj := current.Previous(rhythmStart)
	prevPrevObj := current.Previous(rhythmStart + 1)
	if prevObj == nil || prevPrevObj == nil {
		return 1
	}

	for i := rhythmStart; i > 0; i-- {
		currObj := current.Previous(i - 1)
		if currObj == nil || currObj.IsSpinner {
			continue
		}

		timeDecay := (rhythmHistoryTimeMax - (current.StartTime - currObj.StartTime)) / rhythmHistoryTimeMax
		noteDecay := float64(historicalNoteCount-i) / float64(historicalNoteCount)
		currHistoricalDecay := min(noteDecay, timeDecay)

		const deltaMinValue = 1e-7
		currDelta := max(currObj.DeltaTime, deltaMinValue)
		prevDelta := max(prevObj.DeltaTime, deltaMinValue)
		deltaDifference := math.Abs(prevDelta - currDelta)

		if island.delta == math.MaxInt {
			island = newIsland(int(currDelta))
		}

		deltaDifferenceRatio := max(prevDelta, currDelta) / min(prevDelta, currDelta)
		differenceMultiplier := mutils.Clamp(2.0-deltaDifferenceRatio/8.0, 0, 1)
		windowPenalty := mutils.Clamp((deltaDifference-deltaDifferenceEpsilon)/deltaDifferenceEpsilon, 0, 1)
		effectiveDifficulty := getEffectiveDifficulty(deltaDifferenceRatio) * windowPenalty * differenceMultiplier

		if prevObj.IsSlider {
			sliderLazyEndDelta := currObj.MinimumJumpTime
			sliderLazyRatio := max(sliderLazyEndDelta, currDelta) / min(sliderLazyEndDelta, currDelta)
			sliderRealEndDelta := currObj.LastObjectEndDeltaTime
			sliderRealRatio := max(sliderRealEndDelta, currDelta) / min(sliderRealEndDelta, currDelta)
			sliderDifficulty := min(getEffectiveDifficulty(sliderLazyRatio), getEffectiveDifficulty(sliderRealRatio))
			effectiveDifficulty = min(sliderDifficulty, effectiveDifficulty)
		}

		if deltaDifference < deltaDifferenceEpsilon {
			island.addDelta(int(currDelta))
		}

		if firstDeltaSwitch {
			if deltaDifference > deltaDifferenceEpsilon {
				if currObj.IsSlider {
					effectiveDifficulty *= 0.5
				}
				if island.isSimilarPolarity(previousIsland, deltaDifferenceEpsilon) {
					effectiveDifficulty *= 0.5
				}
				if prevPrevObj != nil && max(prevPrevObj.DeltaTime, deltaMinValue) > prevDelta+deltaDifferenceEpsilon && prevDelta > currDelta+deltaDifferenceEpsilon {
					effectiveDifficulty *= 0.125
				}
				if previousIsland.deltaCount == island.deltaCount {
					effectiveDifficulty *= 0.5
				}
				if prevDelta > currDelta+deltaDifferenceEpsilon {
					effectiveDifficulty *= 0.65
				}

				found := false
				if index := slices.IndexFunc(islands, func(existing *rhythmIsland) bool {
					return existing.almostEquals(island, deltaDifferenceEpsilon)
				}); index >= 0 {
					existing := islands[index]
					if previousIsland.almostEquals(island, deltaDifferenceEpsilon) {
						existing.occurrences++
					}
					power := putils.Logistic(float64(island.delta), 58.33, 0.24, 2.75)
					occurrences := float64(existing.occurrences)
					effectiveDifficulty *= min(3/occurrences, math.Pow(1/occurrences, power))
					found = true
				}
				if !found && island.deltaCount > 0 {
					islands = append(islands, island)
				}

				effectiveDifficulty *= 1 - prevObj.GetDoubletapness(currObj)*0.75
				if island.deltaCount > 1 {
					rhythmComplexitySum += math.Sqrt(effectiveDifficulty*startDifficulty) * currHistoricalDecay
				} else {
					rhythmComplexitySum += 0.7 * currHistoricalDecay
				}

				startDifficulty = effectiveDifficulty
				if prevDelta+deltaDifferenceEpsilon < currDelta {
					firstDeltaSwitch = false
				}
				previousIsland = island
				island = newIsland(int(currDelta))
			}
		} else if prevDelta > currDelta+deltaDifferenceEpsilon {
			firstDeltaSwitch = true
			if currObj.IsSlider {
				effectiveDifficulty *= 0.6
			}
			if prevObj.IsSlider {
				effectiveDifficulty *= 0.6
			}
			startDifficulty = effectiveDifficulty
			island = newIsland(int(currDelta))
		}

		prevPrevObj = prevObj
		prevObj = currObj
	}

	rhythmComplexitySum *= putils.ReverseLerp(float64(island.deltaCount), 22, 3)
	return math.Sqrt(4+rhythmComplexitySum*rhythmOverallMultiplier) / 2
}

func getEffectiveDifficulty(deltaDifferenceRatio float64) float64 {
	const rhythmRatioDifficultyMultiplier = 26.0
	fraction := deltaDifferenceRatio - math.Trunc(deltaDifferenceRatio)
	return 1 + rhythmRatioDifficultyMultiplier*min(0.5, putils.SmoothstepBellCurve(fraction, 0.5, 0.5))
}

type rhythmIsland struct {
	delta       int
	deltaCount  int
	occurrences int
}

func newIsland(delta int) *rhythmIsland {
	return &rhythmIsland{
		delta:       max(delta, preprocessing.MinDeltaTime),
		deltaCount:  1,
		occurrences: 1,
	}
}

func (island *rhythmIsland) addDelta(delta int) {
	if island.delta == math.MaxInt {
		island.delta = max(delta, preprocessing.MinDeltaTime)
	}
	island.deltaCount++
}

func (island *rhythmIsland) isSimilarPolarity(other *rhythmIsland, epsilon float64) bool {
	if island == nil || other == nil || island.deltaCount <= 1 || other.deltaCount <= 1 {
		return false
	}
	return math.Abs(float64(island.delta-other.delta)) < epsilon && island.deltaCount%2 == other.deltaCount%2
}

func (island *rhythmIsland) almostEquals(other *rhythmIsland, epsilon float64) bool {
	if island == nil || other == nil {
		return false
	}
	return math.Abs(float64(island.delta-other.delta)) < epsilon && island.deltaCount == other.deltaCount
}
