package pp251020

import (
	"strings"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp251020/preprocessing"
)

func TestFirstSliderCountsNestedScoreAndCombo(t *testing.T) {
	slider := objects.NewSlider(strings.Split("256,192,1000,2,0,L|356:192,2,100", ","))
	slider.ScorePointsLazer = []objects.TickPoint{{}, {IsReverse: true}, {}, {LastPoint: true}}
	for _, object := range []objects.IHitObject{slider, &preprocessing.LazySlider{Slider: slider}} {
		sim := &scoreSimulator{ScoreMultiplier: 1}
		sim.AddFirst(object)
		if sim.sliders != 1 || sim.circles != 0 || sim.combo != 5 || sim.NestedScorePerObject != 110 || sim.ComboScore != 48 {
			t.Fatalf("first slider score = %+v", sim)
		}
	}
}
