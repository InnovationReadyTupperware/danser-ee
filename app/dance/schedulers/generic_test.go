package schedulers

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/vector"
)

type recordingMover struct {
	windowLengths []int
}

func (mover *recordingMover) Reset(*difficulty.Difficulty, int) {}

func (mover *recordingMover) SetObjects(objs []objects.IHitObject) int {
	mover.windowLengths = append(mover.windowLengths, len(objs))
	return 2
}

func (mover *recordingMover) Update(float64) vector.Vector2f { return vector.Vector2f{} }

func (mover *recordingMover) GetObjectsStartTime(objects.IHitObject) float64 { return 0 }

func (mover *recordingMover) GetObjectsEndTime(objects.IHitObject) float64 { return 0 }

func (mover *recordingMover) GetObjectsStartPosition(objects.IHitObject) vector.Vector2f {
	return vector.Vector2f{}
}

func (mover *recordingMover) GetObjectsEndPosition(objects.IHitObject) vector.Vector2f {
	return vector.Vector2f{}
}

func (mover *recordingMover) GetObjectsPosition(float64, objects.IHitObject) vector.Vector2f {
	return vector.Vector2f{}
}

func (mover *recordingMover) GetStartTime() float64 { return 0 }

func (mover *recordingMover) GetEndTime() float64 { return 0 }

func TestGenericSchedulerHandlesEmptyTagQueueWithoutCallingMover(t *testing.T) {
	previous := settings.CursorDance
	t.Cleanup(func() {
		settings.CursorDance = previous
	})
	settings.CursorDance = settings.NewConfigFile().CursorDance

	mover := &recordingMover{}
	scheduler := &GenericScheduler{mover: mover}

	scheduler.Init(nil, nil, nil, nil, false)

	if len(scheduler.queue) != 0 {
		t.Fatalf("empty scheduler queue = %d, want 0", len(scheduler.queue))
	}
	if len(mover.windowLengths) != 0 {
		t.Fatalf("mover received windows %v for an empty TAG queue", mover.windowLengths)
	}
}

func TestGenericSchedulerGivesMoverTwoObjectsForSingleObjectQueue(t *testing.T) {
	previous := settings.CursorDance
	t.Cleanup(func() {
		settings.CursorDance = previous
	})
	settings.CursorDance = settings.NewConfigFile().CursorDance

	mover := &recordingMover{}
	scheduler := &GenericScheduler{mover: mover}

	scheduler.Init([]objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(256, 192), 1000),
	}, nil, nil, nil, false)

	if len(mover.windowLengths) != 1 || mover.windowLengths[0] != 2 {
		t.Fatalf("mover received windows %v, want one two-object window", mover.windowLengths)
	}
	if len(scheduler.queue) != 1 {
		t.Fatalf("scheduler queue after initialization = %d, want 1", len(scheduler.queue))
	}
}

func TestGenericSchedulerHandlesRepeatedOverlapCollapse(t *testing.T) {
	previous := settings.CursorDance
	t.Cleanup(func() {
		settings.CursorDance = previous
	})
	settings.CursorDance = settings.NewConfigFile().CursorDance

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	mover := &recordingMover{}
	scheduler := &GenericScheduler{mover: mover}
	objectsToCollapse := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(256, 192), 1000),
		objects.DummyCircle(vector.NewVec2f(256, 192), 1001),
		objects.DummyCircle(vector.NewVec2f(256, 192), 1002),
		objects.DummyCircle(vector.NewVec2f(256, 192), 1003),
	}

	scheduler.Init(objectsToCollapse, diff, nil, nil, false)

	if len(mover.windowLengths) != 1 || mover.windowLengths[0] != 3 {
		t.Fatalf("mover received windows %v, want one three-object window", mover.windowLengths)
	}
}
