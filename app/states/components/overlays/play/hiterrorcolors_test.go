package play

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
)

func TestHitErrorColorForUsesLazerJudgmentResult(t *testing.T) {
	diff := lazerDifficulty(5)

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
			if got := hitErrorColorFor(diff, test.result, 400); got != test.want {
				t.Fatalf("hitErrorColorFor() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestHitErrorColorForRetainsStableIntegerThresholds(t *testing.T) {
	diff := stableDifficulty(5)

	tests := []struct {
		name   string
		offset float64
		want   color2.Color
	}{
		{name: "inside great", offset: 0, want: legacyHitErrorGreatColor},
		{name: "great boundary", offset: float64(diff.Hit300), want: legacyHitErrorOkColor},
		{name: "ok boundary", offset: float64(diff.Hit100), want: legacyHitErrorMehColor},
		{name: "outer slider head", offset: 300, want: legacyHitErrorMehColor},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hitErrorColorFor(diff, osu.SliderStart, test.offset); got != test.want {
				t.Fatalf("hitErrorColorFor() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestHitErrorColorForUsesLegacyDiagnosticRed(t *testing.T) {
	if got := hitErrorColorFor(lazerDifficulty(5), osu.PositionalMiss, 0); got != hitErrorPositionalMissColor {
		t.Fatalf("positional miss color = %#v, want %#v", got, hitErrorPositionalMissColor)
	}
}
