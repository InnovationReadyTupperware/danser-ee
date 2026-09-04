package schedulers

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	dancesliders "github.com/innovationreadytupperware/danser-ee/app/dance/sliders"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

type recordingMover struct {
	windowLengths []int
	windows       [][]objects.IHitObject
}

func (mover *recordingMover) Reset(*difficulty.Difficulty, int) {}

func (mover *recordingMover) SetObjects(objs []objects.IHitObject) int {
	mover.windowLengths = append(mover.windowLengths, len(objs))
	mover.windows = append(mover.windows, append([]objects.IHitObject{}, objs...))
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

func TestGenericSchedulerKeepsSliderDanceAsOneLongObject(t *testing.T) {
	previous := settings.CursorDance
	t.Cleanup(func() { settings.CursorDance = previous })
	settings.CursorDance = settings.NewConfigFile().CursorDance
	settings.CursorDance.Movers[0].SliderDance = true

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	slider := newSchedulerTestSlider(t)
	mover := &recordingMover{}
	scheduler := &GenericScheduler{mover: mover}
	scheduler.Init([]objects.IHitObject{
		slider,
		objects.DummyCircle(vector.NewVec2f(128, 128), slider.EndTimeLazer+500),
	}, diff, nil, nil, false)

	if len(mover.windows) == 0 || len(mover.windows[0]) < 2 {
		t.Fatalf("mover windows = %v, want the sentinel and slider", mover.windowLengths)
	}
	target, ok := mover.windows[0][1].(*dancesliders.Target)
	if !ok {
		t.Fatalf("slider movement object has type %T, want *sliders.Target", mover.windows[0][1])
	}
	if target.GetDuration() <= 0 {
		t.Fatalf("slider movement duration = %g, want positive", target.GetDuration())
	}
}

func newSchedulerTestSlider(t *testing.T) *objects.Slider {
	t.Helper()

	slider := objects.NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|320:128|384:192", "1", "180", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("test slider was rejected")
	}
	timings := objects.NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	return slider
}
