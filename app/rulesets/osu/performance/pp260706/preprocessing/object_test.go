package preprocessing

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestDifficultyObjectUsesLazerIntegralPreempt(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 4, 7, 8.3)
	objects := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(0, 0), 4000),
		objects.DummyCircle(vector.NewVec2f(100, 0), 4750),
	}

	difficultyObjects := CreateDifficultyObjects(objects, diff)
	if len(difficultyObjects) != 1 {
		t.Fatalf("difficulty objects = %d, want 1", len(difficultyObjects))
	}

	object := difficultyObjects[0]
	wantPreempt := math.Trunc(diff.PreemptU)
	if object.Preempt != wantPreempt {
		t.Fatalf("preempt = %.17g, want integral lazer preempt %.17g", object.Preempt, wantPreempt)
	}

	if got := object.OpacityAt(object.BaseObject.GetStartTime()-wantPreempt-0.5, false); got != 0 {
		t.Fatalf("opacity before integral preempt = %.17g, want 0", got)
	}
}

func TestHiddenOpacityUsesAdjustedNonSliderFadeIn(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 4, 7, 8.3)
	hitObjects := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(0, 0), 4000),
		objects.DummyCircle(vector.NewVec2f(100, 0), 4750),
	}

	object := CreateDifficultyObjects(hitObjects, diff)[0]
	preempt := math.Trunc(diff.PreemptU)
	fadeInDuration := 400 * min(1.0, preempt/450)
	fadeInStartTime := object.BaseObject.GetStartTime() - preempt
	time := fadeInStartTime + fadeInDuration
	fadeOutStartTime := fadeInStartTime + preempt*0.4
	fadeOutDuration := preempt * 0.3
	want := min(
		1.0,
		1.0-max(0.0, min(1.0, (time-fadeOutStartTime)/fadeOutDuration)),
	)

	if got := object.OpacityAt(time, true); math.Abs(got-want) > 1e-12 {
		t.Fatalf("Hidden opacity = %.17g, want %.17g", got, want)
	}
}
