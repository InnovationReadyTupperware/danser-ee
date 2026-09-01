package play

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
)

func TestSliderJudgmentMarkerFor(t *testing.T) {
	tests := []struct {
		name        string
		result      osu.HitResult
		nested      bool
		head        bool
		tail        bool
		textureName string
	}{
		{
			name:        "large tick miss",
			result:      osu.LargeTickMiss,
			nested:      true,
			textureName: "slidertickmiss",
		},
		{
			name:        "classic slider head miss",
			result:      osu.LargeTickMiss,
			head:        true,
			textureName: "slidertickmiss",
		},
		{
			name:        "ignored tail",
			result:      osu.IgnoreMiss,
			nested:      true,
			tail:        true,
			textureName: "sliderendmiss",
		},
		{
			name:        "stable slider tick miss",
			result:      osu.SliderMiss,
			nested:      true,
			textureName: "slidertickmiss",
		},
		{
			name:        "stable slider repeat miss",
			result:      osu.SliderMiss,
			nested:      true,
			textureName: "slidertickmiss",
		},
		{
			name:        "stable slider tail miss",
			result:      osu.SliderMiss,
			nested:      true,
			tail:        true,
			textureName: "sliderendmiss",
		},
		{name: "small tick miss", result: osu.SmallTickMiss, nested: true},
		{name: "large tick hit", result: osu.LargeTickHit, nested: true},
		{name: "ordinary miss", result: osu.Miss},
		{name: "large tick miss outside slider", result: osu.LargeTickMiss},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, ok := sliderJudgmentMarkerForPart(tt.result, tt.nested, tt.head, tt.tail)
			if ok != (tt.textureName != "") {
				t.Fatalf("sliderJudgmentMarkerFor() ok = %t, want %t", ok, tt.textureName != "")
			}
			if !ok {
				return
			}

			if marker.textureName != tt.textureName {
				t.Fatalf("marker texture = %q, want %q", marker.textureName, tt.textureName)
			}
		})
	}
}

func TestJudgmentPresentationForOrdinaryResults(t *testing.T) {
	tests := []struct {
		name   string
		result osu.HitResult
		want   judgmentPresentationKind
	}{
		{name: "hit 300", result: osu.Hit300, want: judgmentPresentationHit},
		{name: "ordinary miss", result: osu.Miss, want: judgmentPresentationHit},
		{name: "stable slider head miss", result: osu.SliderMiss, want: judgmentPresentationHit},
		{name: "slider start", result: osu.SliderStart, want: judgmentPresentationNone},
		{name: "nested hit without metadata", result: osu.LargeTickHit, want: judgmentPresentationNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			judgement := osu.JudgementResult{HitResult: tt.result}
			if got := classifyJudgment(judgement); got != tt.want {
				t.Fatalf("classifyJudgment(%v) = %d, want %d", tt.result, got, tt.want)
			}
		})
	}
}
