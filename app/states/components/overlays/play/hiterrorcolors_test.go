package play

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
)

func TestHitErrorColorForUsesLazerJudgmentResult(t *testing.T) {
	tests := []struct {
		name   string
		result osu.HitResult
		want   color2.Color
	}{
		{name: "great", result: osu.Hit300, want: legacyHitErrorGreatColor},
		{name: "ok", result: osu.Hit100, want: legacyHitErrorOkColor},
		{name: "meh", result: osu.Hit50, want: legacyHitErrorMehColor},
		{name: "classic slider head", result: osu.LargeTickHit, want: legacyHitErrorGreatColor},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hitErrorColorFor(test.result); got != test.want {
				t.Fatalf("hitErrorColorFor() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestHitErrorColorForUsesLegacyDiagnosticRed(t *testing.T) {
	if got := hitErrorColorFor(osu.PositionalMiss); got != hitErrorPositionalMissColor {
		t.Fatalf("positional miss color = %#v, want %#v", got, hitErrorPositionalMissColor)
	}
}
