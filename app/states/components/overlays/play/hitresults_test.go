package play

import (
	"testing"

	"github.com/wieku/danser-go/app/rulesets/osu"
	color2 "github.com/wieku/danser-go/framework/math/color"
)

func TestSliderJudgmentMarkerFor(t *testing.T) {
	tests := []struct {
		name        string
		result      osu.HitResult
		nested      bool
		head        bool
		textureName string
		fallback    color2.Color
	}{
		{
			name:        "large tick miss",
			result:      osu.LargeTickMiss,
			nested:      true,
			textureName: "slidertickmiss",
			fallback:    color2.NewIRGB(237, 17, 33),
		},
		{
			name:        "classic slider head miss",
			result:      osu.LargeTickMiss,
			head:        true,
			textureName: "slidertickmiss",
			fallback:    color2.NewIRGB(237, 17, 33),
		},
		{
			name:        "ignored tail",
			result:      osu.IgnoreMiss,
			nested:      true,
			textureName: "sliderendmiss",
			fallback:    color2.NewIRGB(128, 128, 128),
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
			if marker.fallbackColor != tt.fallback {
				t.Fatalf("marker fallback color = %#v, want %#v", marker.fallbackColor, tt.fallback)
			}
		})
	}
}
