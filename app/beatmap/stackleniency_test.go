package beatmap

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestOldStackingUsesLazerStartTimeWindow(t *testing.T) {
	position := vector.NewVec2f(256, 192)
	newHitObject := func(startTime, endTime float64) *objects.HitObject {
		return &objects.HitObject{
			StartPosRaw:   position,
			EndPosRaw:     position,
			StartTime:     startTime,
			EndTime:       endTime,
			StackIndexMap: make(map[int64]int64),
		}
	}

	first := &objects.Circle{HitObject: newHitObject(0, 0)}
	slider := &objects.Slider{
		HitObject:    newHitObject(100, 1000),
		EndTimeLazer: 1000,
	}
	last := &objects.Circle{HitObject: newHitObject(500, 500)}
	beatMap := NewBeatMap()
	beatMap.Version = 5
	beatMap.StackLeniency = 0.2
	beatMap.Diff.Preempt = 1000
	beatMap.HitObjects = []objects.IHitObject{first, slider, last}
	for _, object := range beatMap.HitObjects {
		object.SetStackLeniency(beatMap.StackLeniency)
	}

	beatMap.CalculateStackLeniency(beatMap.Diff)

	// osu!lazer intentionally advances the old-map stacking window to the
	// matched object's start time. Stable historically used its end time here,
	// which would incorrectly pull the last object into this stack.
	if got := first.GetStackIndexMod(beatMap.Diff); got != 1 {
		t.Fatalf("old-map stack index = %d, want 1", got)
	}
}
