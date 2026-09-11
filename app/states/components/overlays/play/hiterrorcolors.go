package play

import (
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
)

var (
	// These are the osu!standard colors returned by osu!lazer's
	// OsuColour.ForHitResult. Keep them local to the hit-error bar: aim-error
	// diagnostics intentionally retain their own palette.
	legacyHitErrorGreatColor = color2.NewIRGB(102, 204, 255)
	legacyHitErrorOkColor    = color2.NewIRGB(136, 179, 0)
	legacyHitErrorMehColor   = color2.NewIRGB(255, 204, 34)

	// Positional misses are danser-only diagnostics, not Lazer hit results.
	// Keep their historical red so the Lazer palette change does not alter the
	// Stable diagnostic overlay.
	hitErrorPositionalMissColor = color2.NewRGBA(0.98, 0.11, 0.011, 1)
)

// hitErrorColorFor returns the osu!lazer line color for a timing result.
func hitErrorColorFor(result osu.HitResult) color2.Color {
	if result == osu.PositionalMiss {
		return hitErrorPositionalMissColor
	}

	switch result &^ osu.Additions {
	case osu.Hit50:
		return legacyHitErrorMehColor
	case osu.Hit100:
		return legacyHitErrorOkColor
	case osu.Hit300, osu.LargeTickHit, osu.SliderStart:
		return legacyHitErrorGreatColor
	default:
		// The feed excludes misses and ignored results. Blue is osu!lazer's
		// fallback for an otherwise successful result not listed above.
		return legacyHitErrorGreatColor
	}
}
