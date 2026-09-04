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
	for b.Loop() {
		mover.SetObjects(path)
	}
	b.StopTimer()
	benchmarkMoverPosition = mover.Update(4000)
}
