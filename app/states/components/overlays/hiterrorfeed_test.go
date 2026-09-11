package overlays

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestBuildHitErrorEventFiltersObjectsAndUsesHeadTime(t *testing.T) {
	circle := objects.DummyCircle(vector.NewVec2f(64, 64), 100)
	fractionalCircle := objects.DummyCircle(vector.NewVec2f(64, 64), 100.25)
	slider := &objects.Slider{HitObject: &objects.HitObject{StartTime: 100, EndTime: 500}}
	spinner := &objects.Spinner{HitObject: &objects.HitObject{StartTime: 100, EndTime: 500}}
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)

	tests := []struct {
		name       string
		object     objects.IHitObject
		result     osu.JudgementResult
		wantOK     bool
		wantOffset float64
		wantResult osu.HitResult
	}{
		{
			name:       "circle hit",
			object:     circle,
			result:     osu.JudgementResult{HitResult: osu.Hit300, Time: 110},
			wantOK:     true,
			wantOffset: 10,
			wantResult: osu.Hit300,
		},
		{
			name:   "circle miss",
			object: circle,
			result: osu.JudgementResult{HitResult: osu.Miss, Time: 110},
			wantOK: false,
		},
		{
			name:       "circle positional miss",
			object:     circle,
			result:     osu.JudgementResult{HitResult: osu.PositionalMiss, Time: 1000},
			wantOK:     true,
			wantOffset: 400,
			wantResult: osu.PositionalMiss,
		},
		{
			name:       "stable slider head",
			object:     slider,
			result:     osu.JudgementResult{HitResult: osu.SliderStart, MaxResult: osu.SliderStart, Time: 125},
			wantOK:     true,
			wantOffset: 25,
			wantResult: osu.LargeTickHit,
		},
		{
			name:       "slider positional miss",
			object:     slider,
			result:     osu.JudgementResult{HitResult: osu.PositionalMiss, MaxResult: osu.SliderStart, Time: -1000},
			wantOK:     true,
			wantOffset: -1100,
			wantResult: osu.PositionalMiss,
		},
		{
			name:       "lazer slider head",
			object:     slider,
			result:     osu.JudgementResult{HitResult: osu.Hit300, MaxResult: osu.SliderStart, Time: 125},
			wantOK:     true,
			wantOffset: 25,
			wantResult: osu.Hit300,
		},
		{
			name:       "stable fractional great boundary uses Lazer window",
			object:     fractionalCircle,
			result:     osu.JudgementResult{HitResult: osu.Hit300, Time: 150},
			wantOK:     true,
			wantOffset: 49.75,
			wantResult: osu.Hit100,
		},
		{
			name:   "stable fractional outer boundary is not a Lazer timing hit",
			object: fractionalCircle,
			result: osu.JudgementResult{HitResult: osu.Hit50, Time: 250},
			wantOK: false,
		},
		{
			name:   "slider summary base hit",
			object: slider,
			result: osu.JudgementResult{HitResult: osu.Hit300, Time: 125},
			wantOK: false,
		},
		{
			name:   "slider nested tick",
			object: slider,
			result: osu.JudgementResult{HitResult: osu.LargeTickHit, Time: 125},
			wantOK: false,
		},
		{
			name:   "spinner hit",
			object: spinner,
			result: osu.JudgementResult{HitResult: osu.Hit300, Time: 125},
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := buildHitErrorEvent(test.object, test.result, diff)
			if ok != test.wantOK {
				t.Fatalf("buildHitErrorEvent() ok = %t, want %t", ok, test.wantOK)
			}
			if !ok {
				return
			}
			if got.offset != test.wantOffset {
				t.Fatalf("event offset = %g, want %g", got.offset, test.wantOffset)
			}
			if got.result != test.wantResult {
				t.Fatalf("event result = %v, want %v", got.result, test.wantResult)
			}
		})
	}
}
