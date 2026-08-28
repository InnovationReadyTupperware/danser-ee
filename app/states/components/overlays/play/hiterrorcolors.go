package play

import (
	"math"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/rulesets/osu"
	color2 "github.com/wieku/danser-go/framework/math/color"
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

// hitErrorColorFor returns the line color used by the legacy horizontal bar.
// Lazer can classify slider heads as LargeTickHit under Classic, so Lazer-mode
// lines use the actual result rather than inferring a color from the offset.
// Stable slider heads do not carry an accuracy result and retain the historical
// integer timing-window classification instead.
func hitErrorColorFor(diff *difficulty.Difficulty, result osu.HitResult, offset float64) color2.Color {
	if result == osu.PositionalMiss {
		return hitErrorPositionalMissColor
	}

	if diff != nil && diff.IsLazer() {
		switch result &^ osu.Additions {
		case osu.Hit50:
			return legacyHitErrorMehColor
		case osu.Hit100:
			return legacyHitErrorOkColor
		case osu.Hit300, osu.LargeTickHit, osu.SliderStart:
			return legacyHitErrorGreatColor
		default:
			// The feed normally excludes misses and ignored results. Blue is
			// Lazer's fallback color for an otherwise successful result.
			return legacyHitErrorGreatColor
		}
	}

	if diff == nil {
		return legacyHitErrorGreatColor
	}

	absoluteOffset := int64(math.Abs(offset))
	switch {
	case absoluteOffset < diff.Hit300:
		return legacyHitErrorGreatColor
	case absoluteOffset < diff.Hit100:
		return legacyHitErrorOkColor
	default:
		// Stable slider heads use SliderStart for every successful head, so
		// offsets outside Hit100 still need the outer successful band color.
		return legacyHitErrorMehColor
	}
}
