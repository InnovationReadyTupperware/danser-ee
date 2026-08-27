package pp251020

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
)

func TestCalculateStepHandlesSingleSpinnerMap(t *testing.T) {
	beatMap := beatmap.NewBeatMap()
	beatMap.HitObjects = []objects.IHitObject{objects.NewDummySpinner(1000, 2000)}

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	attributes := NewDifficultyCalculator().CalculateStep(beatMap, diff)

	if len(attributes) != 1 {
		t.Fatalf("step attributes = %d, want one result for one hit object", len(attributes))
	}
}
