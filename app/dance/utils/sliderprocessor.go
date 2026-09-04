package utils

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/dance/sliders"
)

func objectPreProcess(hitobject objects.IHitObject, sliderDance bool, diff *difficulty.Difficulty) ([]objects.IHitObject, bool) {
	slider, ok := hitobject.(*objects.Slider)
	if !ok {
		return nil, false
	}

	if slider.IsSingular() ||
		(sliderDance && slider.IsPathological()) ||
		(!sliderDance && slider.NeedsGeneratedMovementFallback()) {
		return []objects.IHitObject{sliders.NewHeadTarget(slider, diff)}, true
	}

	if sliderDance {
		return []objects.IHitObject{sliders.NewTarget(slider, diff)}, true
	}

	return nil, false
}

func PreprocessQueue(index int, queue []objects.IHitObject, sliderDance bool) []objects.IHitObject {
	return PreprocessQueueForDiff(index, queue, sliderDance, nil)
}

// PreprocessQueueForDiff resolves one slider for generated movement.
func PreprocessQueueForDiff(index int, queue []objects.IHitObject, sliderDance bool, diff *difficulty.Difficulty) []objects.IHitObject {
	if index < 0 || index >= len(queue) {
		return queue
	}

	if arr, ok := objectPreProcess(queue[index], sliderDance, diff); ok {
		queue1 := make([]objects.IHitObject, 0, len(queue)-1+len(arr))
		queue1 = append(queue1, queue[:index]...)
		queue1 = append(queue1, arr...)
		queue1 = append(queue1, queue[index+1:]...)

		return queue1
	}

	return queue
}

// ApplySliderDance replaces sliders with semantic movement targets.
func ApplySliderDance(queue []objects.IHitObject) []objects.IHitObject {
	return ApplySliderDanceForDiff(queue, nil)
}

// ApplySliderDanceForDiff uses the participant's gameplay traversal.
func ApplySliderDanceForDiff(queue []objects.IHitObject, diff *difficulty.Difficulty) []objects.IHitObject {
	if len(queue) == 0 {
		return queue
	}

	processed := make([]objects.IHitObject, len(queue))
	for i, hitObject := range queue {
		processed[i] = hitObject
		if replacement, ok := objectPreProcess(hitObject, true, diff); ok {
			processed[i] = replacement[0]
		}
	}

	return processed
}
