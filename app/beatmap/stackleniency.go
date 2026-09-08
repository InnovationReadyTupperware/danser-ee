package beatmap

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

// Original code by: https://github.com/ppy/osu/blob/master/osu.Game.Rulesets.Osu/Beatmaps/OsuBeatmapProcessor.cs

const stackDistance = 3.0

func isSpinner(obj objects.IHitObject) bool {
	_, ok := obj.(*objects.Spinner)
	return ok
}

func isSlider(obj objects.IHitObject) bool {
	_, ok := obj.(*objects.Slider)
	return ok
}

func processStacking(hitObjects []objects.IHitObject, version int, stackThreshold float64, lazer bool) {
	for _, object := range hitObjects {
		object.SetStackIndexForMode(stackThreshold, lazer, 0)
	}

	if version >= 6 {
		applyNewStacking(hitObjects, stackThreshold, lazer)
	} else if lazer {
		applyOldStackingLazer(hitObjects, stackThreshold)
	} else {
		applyOldStackingStable(hitObjects, stackThreshold)
	}

	for _, object := range hitObjects {
		if isSpinner(object) {
			object.SetStackIndexForMode(stackThreshold, lazer, 0)
		}
	}
}

func applyNewStacking(hitObjects []objects.IHitObject, stackThreshold float64, lazer bool) {
	extendedEndIndex := len(hitObjects) - 1
	for i := len(hitObjects) - 1; i >= 0; i-- {
		stackBaseIndex := i

		for n := stackBaseIndex + 1; n < len(hitObjects); n++ {
			stackBaseObject := hitObjects[stackBaseIndex]
			if isSpinner(stackBaseObject) {
				break
			}

			objectN := hitObjects[n]
			if isSpinner(objectN) {
				continue
			}

			if objectN.GetStartTime()-stackingEndTime(stackBaseObject, lazer) > stackThreshold {
				break
			}

			if stackBaseObject.GetStartPosition().Dst(objectN.GetStartPosition()) < stackDistance ||
				isSlider(stackBaseObject) && stackingEndPosition(stackBaseObject, lazer).Dst(objectN.GetStartPosition()) < stackDistance {
				stackBaseIndex = n
				objectN.SetStackIndexForMode(stackThreshold, lazer, 0)
			}
		}

		if stackBaseIndex > extendedEndIndex {
			extendedEndIndex = stackBaseIndex
			if extendedEndIndex == len(hitObjects)-1 {
				break
			}
		}
	}

	extendedStartIndex := 0
	for i := extendedEndIndex; i > 0; i-- {
		n := i
		objectI := hitObjects[i]

		if objectI.GetStackIndexForMode(stackThreshold, lazer) != 0 || isSpinner(objectI) {
			continue
		}

		if !isSlider(objectI) {
			for n--; n >= 0; n-- {
				objectN := hitObjects[n]
				if isSpinner(objectN) {
					continue
				}

				if stackRangeExceeded(objectI.GetStartTime(), stackingEndTime(objectN, lazer), stackThreshold, lazer) {
					break
				}

				if n < extendedStartIndex {
					objectN.SetStackIndexForMode(stackThreshold, lazer, 0)
					extendedStartIndex = n
				}

				endPosition := stackingEndPosition(objectN, lazer)
				if isSlider(objectN) && endPosition.Dst(objectI.GetStartPosition()) < stackDistance {
					offset := objectI.GetStackIndexForMode(stackThreshold, lazer) - objectN.GetStackIndexForMode(stackThreshold, lazer) + 1
					for j := n + 1; j <= i; j++ {
						objectJ := hitObjects[j]
						if endPosition.Dst(objectJ.GetStartPosition()) < stackDistance {
							objectJ.SetStackIndexForMode(stackThreshold, lazer, objectJ.GetStackIndexForMode(stackThreshold, lazer)-offset)
						}
					}
					break
				}

				if objectN.GetStartPosition().Dst(objectI.GetStartPosition()) < stackDistance {
					objectN.SetStackIndexForMode(stackThreshold, lazer, objectI.GetStackIndexForMode(stackThreshold, lazer)+1)
					objectI = objectN
				}
			}
		} else {
			for n--; n >= 0; n-- {
				objectN := hitObjects[n]
				if isSpinner(objectN) {
					continue
				}

				if objectI.GetStartTime()-objectN.GetStartTime() > stackThreshold {
					break
				}

				if stackingEndPosition(objectN, lazer).Dst(objectI.GetStartPosition()) < stackDistance {
					objectN.SetStackIndexForMode(stackThreshold, lazer, objectI.GetStackIndexForMode(stackThreshold, lazer)+1)
					objectI = objectN
				}
			}
		}
	}
}

func applyOldStackingStable(hitObjects []objects.IHitObject, stackThreshold float64) {
	for i, objectI := range hitObjects {
		startTime := objectI.GetEndTime()

		if objectI.GetStackIndexForMode(stackThreshold, false) == 0 || isSlider(objectI) {
			sliderStack := int64(0)
			for n := i + 1; n < len(hitObjects); n++ {
				objectN := hitObjects[n]
				if objectN.GetStartTime()-stackThreshold > startTime {
					break
				}

				if objectN.GetStartPosition().Dst(objectI.GetStartPosition()) < stackDistance {
					objectI.SetStackIndexForMode(stackThreshold, false, objectI.GetStackIndexForMode(stackThreshold, false)+1)
					startTime = objectN.GetEndTime()
				} else if objectN.GetStartPosition().Dst(objectI.GetEndPosition()) < stackDistance {
					sliderStack++
					objectN.SetStackIndexForMode(stackThreshold, false, objectN.GetStackIndexForMode(stackThreshold, false)-sliderStack)
					startTime = objectN.GetEndTime()
				}
			}
		}
	}
}

func applyOldStackingLazer(hitObjects []objects.IHitObject, stackThreshold float64) {
	for i, objectI := range hitObjects {
		if objectI.GetStackIndexForMode(stackThreshold, true) != 0 && !isSlider(objectI) {
			continue
		}

		startTime := stackingEndTime(objectI, true)
		sliderStack := int64(0)
		for n := i + 1; n < len(hitObjects); n++ {
			objectN := hitObjects[n]
			if objectN.GetStartTime()-stackThreshold > startTime {
				break
			}

			if objectN.GetStartPosition().Dst(objectI.GetStartPosition()) < stackDistance {
				objectI.SetStackIndexForMode(stackThreshold, true, objectI.GetStackIndexForMode(stackThreshold, true)+1)
				startTime = objectN.GetStartTime()
			} else if objectN.GetStartPosition().Dst(stackingEndPosition(objectI, true)) < stackDistance {
				sliderStack++
				objectN.SetStackIndexForMode(stackThreshold, true, objectN.GetStackIndexForMode(stackThreshold, true)-sliderStack)
				startTime = objectN.GetStartTime()
			}
		}
	}
}

func stackRangeExceeded(startTime, endTime, stackThreshold float64, lazer bool) bool {
	if lazer {
		return float64(int64(startTime)-int64(endTime)) > stackThreshold
	}

	return startTime-endTime > stackThreshold
}

func stackingEndTime(object objects.IHitObject, lazer bool) float64 {
	if lazer {
		if slider, ok := object.(*objects.Slider); ok {
			return slider.EndTimeLazer
		}
	}

	return object.GetEndTime()
}

func stackingEndPosition(object objects.IHitObject, lazer bool) vector.Vector2f {
	if lazer {
		if slider, ok := object.(*objects.Slider); ok {
			return slider.PositionAtLazer(slider.EndTimeLazer)
		}
	}

	return object.GetEndPosition()
}
