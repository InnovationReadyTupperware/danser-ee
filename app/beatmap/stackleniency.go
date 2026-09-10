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

// processStacking follows osu!lazer's beatmap processor: the beatmap format
// version selects the stacking algorithm. Replay provenance and Classic do not.
func processStacking(hitObjects []objects.IHitObject, version int, stackThreshold float64, stackKey int64) {
	for _, object := range hitObjects {
		object.SetStackIndex(stackKey, 0)
	}

	if version >= 6 {
		applyNewStacking(hitObjects, stackThreshold, stackKey)
	} else {
		applyOldStacking(hitObjects, stackThreshold, stackKey)
	}

	for _, object := range hitObjects {
		if isSpinner(object) {
			object.SetStackIndex(stackKey, 0)
		}
	}
}

func applyNewStacking(hitObjects []objects.IHitObject, stackThreshold float64, stackKey int64) {
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

			if objectN.GetStartTime()-stackingEndTime(stackBaseObject) > stackThreshold {
				break
			}

			if stackBaseObject.GetStartPosition().Dst(objectN.GetStartPosition()) < stackDistance ||
				isSlider(stackBaseObject) && stackingEndPosition(stackBaseObject).Dst(objectN.GetStartPosition()) < stackDistance {
				stackBaseIndex = n
				objectN.SetStackIndex(stackKey, 0)
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

		if objectI.GetStackIndex(stackKey) != 0 || isSpinner(objectI) {
			continue
		}

		if !isSlider(objectI) {
			for n--; n >= 0; n-- {
				objectN := hitObjects[n]
				if isSpinner(objectN) {
					continue
				}

				if stackRangeExceeded(objectI.GetStartTime(), stackingEndTime(objectN), stackThreshold) {
					break
				}

				if n < extendedStartIndex {
					objectN.SetStackIndex(stackKey, 0)
					extendedStartIndex = n
				}

				endPosition := stackingEndPosition(objectN)
				if isSlider(objectN) && endPosition.Dst(objectI.GetStartPosition()) < stackDistance {
					offset := objectI.GetStackIndex(stackKey) - objectN.GetStackIndex(stackKey) + 1
					for j := n + 1; j <= i; j++ {
						objectJ := hitObjects[j]
						if endPosition.Dst(objectJ.GetStartPosition()) < stackDistance {
							objectJ.SetStackIndex(stackKey, objectJ.GetStackIndex(stackKey)-offset)
						}
					}
					break
				}

				if objectN.GetStartPosition().Dst(objectI.GetStartPosition()) < stackDistance {
					objectN.SetStackIndex(stackKey, objectI.GetStackIndex(stackKey)+1)
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

				if stackingEndPosition(objectN).Dst(objectI.GetStartPosition()) < stackDistance {
					objectN.SetStackIndex(stackKey, objectI.GetStackIndex(stackKey)+1)
					objectI = objectN
				}
			}
		}
	}
}

func applyOldStacking(hitObjects []objects.IHitObject, stackThreshold float64, stackKey int64) {
	for i, objectI := range hitObjects {
		if objectI.GetStackIndex(stackKey) != 0 && !isSlider(objectI) {
			continue
		}

		startTime := stackingEndTime(objectI)
		sliderStack := int64(0)
		for n := i + 1; n < len(hitObjects); n++ {
			objectN := hitObjects[n]
			if objectN.GetStartTime()-stackThreshold > startTime {
				break
			}

			if objectN.GetStartPosition().Dst(objectI.GetStartPosition()) < stackDistance {
				objectI.SetStackIndex(stackKey, objectI.GetStackIndex(stackKey)+1)
				startTime = objectN.GetStartTime()
			} else if objectN.GetStartPosition().Dst(stackingEndPosition(objectI)) < stackDistance {
				sliderStack++
				objectN.SetStackIndex(stackKey, objectN.GetStackIndex(stackKey)-sliderStack)
				startTime = objectN.GetStartTime()
			}
		}
	}
}

func stackRangeExceeded(startTime, endTime, stackThreshold float64) bool {
	return float64(int64(startTime)-int64(endTime)) > stackThreshold
}

func stackingEndTime(object objects.IHitObject) float64 {
	if slider, ok := object.(*objects.Slider); ok {
		return slider.EndTimeLazer
	}

	return object.GetEndTime()
}

func stackingEndPosition(object objects.IHitObject) vector.Vector2f {
	if slider, ok := object.(*objects.Slider); ok {
		return slider.PositionAtLazer(slider.EndTimeLazer)
	}

	return object.GetEndPosition()
}
