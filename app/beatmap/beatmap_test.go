package beatmap

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/framework/math/vector"
)

func TestUpdateRemovesExactlyTheExpiredProcessedObjects(t *testing.T) {
	beatMap := NewBeatMap()
	beatMap.Diff.Preempt = 0
	beatMap.Timings.AddPoint(0, 500, 1, 1, 1, 4, false, false, false)

	first := objects.DummyCircle(vector.NewVec2f(0, 0), 0)
	second := objects.DummyCircle(vector.NewVec2f(0, 0), 400)
	third := objects.DummyCircle(vector.NewVec2f(0, 0), 400)
	beatMap.Queue = []objects.IHitObject{first, second, third}

	beatMap.Update(0)
	beatMap.Update(400)

	if len(beatMap.processed) != 2 {
		t.Fatalf("processed object count = %d, want 2", len(beatMap.processed))
	}

	for _, object := range beatMap.processed {
		if object.GetStartTime() != 400 {
			t.Fatalf("processed object at %v ms was removed or retained incorrectly", object.GetStartTime())
		}
	}
}
