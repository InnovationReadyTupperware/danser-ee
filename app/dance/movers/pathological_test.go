package movers

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

var benchmarkMoverPosition vector.Vector2f

func TestBezierAndSplineMoversRemainFiniteOnDenseOutOfPlayfieldPath(t *testing.T) {
	previous := settings.CursorDance
	t.Cleanup(func() { settings.CursorDance = previous })
	settings.CursorDance = settings.NewConfigFile().CursorDance

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	path := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(256, 192), 0),
		objects.DummyCircle(vector.NewVec2f(255, 1768), 100),
		objects.DummyCircle(vector.NewVec2f(255, -29156), 200),
		objects.DummyCircle(vector.NewVec2f(3322, -29156), 300),
		objects.DummyCircle(vector.NewVec2f(256, 192), 400),
	}

	tests := []struct {
		name string
		new  func() MultiPointMover
	}{
		{name: "bezier", new: NewBezierMover},
		{name: "spline", new: NewSplineMover},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mover := test.new()
			mover.Reset(diff, 0)

			if consumed := mover.SetObjects(path); consumed < 2 {
				t.Fatalf("SetObjects consumed %d objects, want at least 2", consumed)
			}

			for _, time := range []float64{0, 50, 100, 150, 200, 250, 300, 350, 400} {
				position := mover.Update(time)
				if math.IsNaN(float64(position.X)) || math.IsInf(float64(position.X), 0) ||
					math.IsNaN(float64(position.Y)) || math.IsInf(float64(position.Y), 0) {
					t.Fatalf("position at %g ms = %#v, want finite coordinates", time, position)
				}
			}
		})
	}
}

func TestBezierAndSplineMoversTreatPathologicalSliderAsOneHitPoint(t *testing.T) {
	previous := settings.CursorDance
	previousKnockout := settings.KNOCKOUT
	t.Cleanup(func() {
		settings.CursorDance = previous
		settings.KNOCKOUT = previousKnockout
	})
	settings.CursorDance = settings.NewConfigFile().CursorDance
	settings.KNOCKOUT = false

	timings := objects.NewTimings()
	timings.SliderMult = 2.06
	timings.TickRate = 0.5
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()

	slider := objects.NewSlider([]string{
		"256", "192", "113227", "102", "0",
		"B|255:1768|255:1768|255:-29156", "1", "32496.5", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("pathological slider was rejected")
	}
	slider.SetTiming(timings, 14, false)

	dummyPoints := slider.GetAsDummyCircles()
	if len(dummyPoints) != 1 {
		t.Fatalf("pathological slider dance points = %d, want one", len(dummyPoints))
	}

	queue := append([]objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(256, 192), slider.StartTime-100),
	}, dummyPoints...)
	queue = append(queue, objects.DummyCircle(vector.NewVec2f(256, 192), slider.StartTime+100))
	if len(queue) != 3 {
		t.Fatalf("pathological slider queue has %d points, want three", len(queue))
	}
	for _, hitObject := range queue {
		if _, isSlider := hitObject.(*objects.Slider); isSlider {
			t.Fatal("pathological slider remained in mover queue")
		}
	}

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	for _, test := range []struct {
		name string
		new  func() MultiPointMover
	}{
		{name: "bezier", new: NewBezierMover},
		{name: "spline", new: NewSplineMover},
	} {
		t.Run(test.name, func(t *testing.T) {
			mover := test.new()
			mover.Reset(diff, 0)
			mover.SetObjects(queue)

			for i := 0; i+1 < len(queue); i++ {
				start := queue[i].GetEndTime()
				end := queue[i+1].GetStartTime()
				for _, time := range []float64{start, (start + end) / 2, end} {
					position := mover.Update(time)
					if math.IsNaN(float64(position.X)) || math.IsInf(float64(position.X), 0) ||
						math.IsNaN(float64(position.Y)) || math.IsInf(float64(position.Y), 0) {
						t.Fatalf("position at %g ms = %#v, want finite coordinates", time, position)
					}
				}
			}
		})
	}
}

func BenchmarkBezierMoverSetObjectsDensePath(b *testing.B) {
	benchmarkMoverSetObjects(b, NewBezierMover)
}

func BenchmarkSplineMoverSetObjectsDensePath(b *testing.B) {
	benchmarkMoverSetObjects(b, NewSplineMover)
}

func benchmarkMoverSetObjects(b *testing.B, newMover func() MultiPointMover) {
	previous := settings.CursorDance
	b.Cleanup(func() { settings.CursorDance = previous })
	settings.CursorDance = settings.NewConfigFile().CursorDance

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	path := make([]objects.IHitObject, 0, 1024)
	for i := range 1024 {
		path = append(path, objects.DummyCircle(
			vector.NewVec2f(float32(i%512), float32((i*37)%384)),
			float64(i*8),
		))
	}

	mover := newMover()
	mover.Reset(diff, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		mover.SetObjects(path)
	}
	b.StopTimer()
	benchmarkMoverPosition = mover.Update(4000)
}
