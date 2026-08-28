package play

import (
	"testing"

	"github.com/wieku/danser-go/app/rulesets/osu"
)

func TestSliderJudgmentMarkerFor(t *testing.T) {
	tests := []struct {
		name        string
		result      osu.HitResult
		nested      bool
		head        bool
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
			textureName: "sliderendmiss",
		},
		{name: "small tick miss", result: osu.SmallTickMiss, nested: true},
		{name: "large tick hit", result: osu.LargeTickHit, nested: true},
		{name: "ordinary miss", result: osu.Miss},
		{name: "large tick miss outside slider", result: osu.LargeTickMiss},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, ok := sliderJudgmentMarkerFor(tt.result, tt.nested, tt.head)
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
