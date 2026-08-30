package utils

import (
	"sort"

	"github.com/wieku/danser-go/app/beatmap/objects"
)

func objectPreProcess(hitobject objects.IHitObject, sliderDance bool) ([]objects.IHitObject, bool) {
	if s1, ok1 := hitobject.(*objects.Slider); ok1 && (sliderDance || s1.IsPathological() || s1.IsSingular()) {
		return s1.GetAsDummyCircles(), true
	}

	return nil, false
}

func PreprocessQueue(index int, queue []objects.IHitObject, sliderDance bool) []objects.IHitObject {
	if index < 0 || index >= len(queue) {
		return queue
	}

	if arr, ok := objectPreProcess(queue[index], sliderDance); ok {
		queue1 := make([]objects.IHitObject, 0, len(queue)-1+len(arr))
		queue1 = append(queue1, queue[:index]...)
		queue1 = append(queue1, arr...)
		queue1 = append(queue1, queue[index+1:]...)

		sort.SliceStable(queue1, func(i, j int) bool { return queue1[i].GetStartTime() < queue1[j].GetStartTime() })

		return queue1
	}

	return queue
}

// ExpandSliderDanceQueue replaces every slider with its cursor-dance point
// sequence in one pass. Pathological and singular sliders intentionally
// contribute only their head point; ordinary sliders retain every generated
// score point. The old repeated splice-and-sort loop made the cost quadratic
// when a dense slider section was expanded.
func ExpandSliderDanceQueue(queue []objects.IHitObject) []objects.IHitObject {
	if len(queue) == 0 {
		return queue
	}

	capacity := len(queue)
	for _, hitObject := range queue {
		slider, ok := hitObject.(*objects.Slider)
		if !ok {
			continue
		}

		points := 1
		if !slider.IsPathological() && !slider.IsSingular() {
			points += len(slider.ScorePoints)
		}
		if points > int(^uint(0)>>1)-capacity {
			capacity = len(queue)
			break
		}
		capacity += points - 1
	}

	expanded := make([]objects.IHitObject, 0, capacity)
	for _, hitObject := range queue {
		if slider, ok := hitObject.(*objects.Slider); ok {
			expanded = append(expanded, slider.GetAsDummyCircles()...)
		} else {
			expanded = append(expanded, hitObject)
		}
	}

	sort.SliceStable(expanded, func(i, j int) bool { return expanded[i].GetStartTime() < expanded[j].GetStartTime() })
	return expanded
}
