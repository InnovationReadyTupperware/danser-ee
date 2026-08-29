package color

import (
	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/danser-go/framework/math/mutils"
)

const (
	perceivedBrightnessRedWeight   float32 = 0.299
	perceivedBrightnessGreenWeight float32 = 0.587
	perceivedBrightnessBlueWeight  float32 = 0.114
	targetPerceivedBrightness      float32 = 0.6
)

// hspaColor mirrors osu!lazer's HSPAColour value. Its P component is
// perceived brightness rather than HSV value, which lets combo colors retain
// their hue and saturation while moving toward a consistent visual intensity.
type hspaColor struct {
	hue                 float32
	saturation          float32
	perceivedBrightness float32
	alpha               float32
}

// NormalizePerceivedBrightness blends a color toward osu!lazer's fixed
// perceived-brightness target. An amount of zero returns the original color;
// an amount of one fully applies the target while preserving hue, saturation,
// and alpha.
func NormalizePerceivedBrightness(c Color, amount float32) Color {
	if amount == 0 || math32.IsNaN(amount) {
		return c
	}

	// Lazer takes the absolute value before interpolation. The UI exposes a
	// non-negative range, but retaining that behavior also makes direct JSON or
	// programmatic configuration safe and deterministic.
	amount = mutils.Clamp(math32.Abs(amount), float32(0), float32(1))

	target := hspaFromColor(c)
	target.perceivedBrightness = targetPerceivedBrightness

	return c.Mix(target.toColor(), amount)
}

func hspaFromColor(c Color) hspaColor {
	result := hspaColor{
		perceivedBrightness: math32.Sqrt(
			c.R*c.R*perceivedBrightnessRedWeight +
				c.G*c.G*perceivedBrightnessGreenWeight +
				c.B*c.B*perceivedBrightnessBlueWeight,
		),
		alpha: c.A,
	}

	// Equal channels have no defined hue, but zero saturation makes hue
	// irrelevant during reconstruction. Keeping the default hue is therefore
	// the same convention used by Lazer's HSPAColour constructor.
	if c.R == c.G && c.R == c.B {
		return result
	}

	switch {
	case c.R >= c.G && c.R >= c.B:
		if c.B >= c.G {
			result.hue = 1 - 1.0/6.0*(c.B-c.G)/(c.R-c.G)
			result.saturation = 1 - c.G/c.R
		} else {
			result.hue = 1.0 / 6.0 * (c.G - c.B) / (c.R - c.B)
			result.saturation = 1 - c.B/c.R
		}
	case c.G >= c.R && c.G >= c.B:
		if c.R >= c.B {
			result.hue = 2.0/6.0 - 1.0/6.0*(c.R-c.B)/(c.G-c.B)
			result.saturation = 1 - c.B/c.G
		} else {
			result.hue = 2.0/6.0 + 1.0/6.0*(c.B-c.R)/(c.G-c.R)
			result.saturation = 1 - c.R/c.G
		}
	default:
		if c.G >= c.R {
			result.hue = 4.0/6.0 - 1.0/6.0*(c.G-c.R)/(c.B-c.R)
			result.saturation = 1 - c.R/c.B
		} else {
			result.hue = 4.0/6.0 + 1.0/6.0*(c.R-c.G)/(c.B-c.G)
			result.saturation = 1 - c.G/c.B
		}
	}

	return result
}

func (c hspaColor) toColor() Color {
	minOverMax := 1 - c.saturation
	result := Color{A: c.alpha}
	hue := c.hue

	if minOverMax > 0 {
		switch {
		case hue < 1.0/6.0:
			hue = 6 * (hue - 0.0/6.0)
			part := 1 + hue*(1/minOverMax-1)
			result.B = c.perceivedBrightness / math32.Sqrt(
				perceivedBrightnessRedWeight/minOverMax/minOverMax+
					perceivedBrightnessGreenWeight*part*part+
					perceivedBrightnessBlueWeight,
			)
			result.R = result.B / minOverMax
			result.G = result.B + hue*(result.R-result.B)
		case hue < 2.0/6.0:
			hue = 6 * (-hue + 2.0/6.0)
			part := 1 + hue*(1/minOverMax-1)
			result.B = c.perceivedBrightness / math32.Sqrt(
				perceivedBrightnessGreenWeight/minOverMax/minOverMax+
					perceivedBrightnessRedWeight*part*part+
					perceivedBrightnessBlueWeight,
			)
			result.G = result.B / minOverMax
			result.R = result.B + hue*(result.G-result.B)
		case hue < 3.0/6.0:
			hue = 6 * (hue - 2.0/6.0)
			part := 1 + hue*(1/minOverMax-1)
			result.R = c.perceivedBrightness / math32.Sqrt(
				perceivedBrightnessGreenWeight/minOverMax/minOverMax+
					perceivedBrightnessBlueWeight*part*part+
					perceivedBrightnessRedWeight,
			)
			result.G = result.R / minOverMax
			result.B = result.R + hue*(result.G-result.R)
		case hue < 4.0/6.0:
			hue = 6 * (-hue + 4.0/6.0)
			part := 1 + hue*(1/minOverMax-1)
			result.R = c.perceivedBrightness / math32.Sqrt(
				perceivedBrightnessBlueWeight/minOverMax/minOverMax+
					perceivedBrightnessGreenWeight*part*part+
					perceivedBrightnessRedWeight,
			)
			result.B = result.R / minOverMax
			result.G = result.R + hue*(result.B-result.R)
		case hue < 5.0/6.0:
			hue = 6 * (hue - 4.0/6.0)
			part := 1 + hue*(1/minOverMax-1)
			result.G = c.perceivedBrightness / math32.Sqrt(
				perceivedBrightnessBlueWeight/minOverMax/minOverMax+
					perceivedBrightnessRedWeight*part*part+
					perceivedBrightnessGreenWeight,
			)
			result.B = result.G / minOverMax
			result.R = result.G + hue*(result.B-result.G)
		default:
			hue = 6 * (-hue + 6.0/6.0)
			part := 1 + hue*(1/minOverMax-1)
			result.G = c.perceivedBrightness / math32.Sqrt(
				perceivedBrightnessRedWeight/minOverMax/minOverMax+
					perceivedBrightnessBlueWeight*part*part+
					perceivedBrightnessGreenWeight,
			)
			result.R = result.G / minOverMax
			result.B = result.G + hue*(result.R-result.G)
		}

		return result
	}

	// Saturation 1 represents the pure-color edges of the HSPA gamut. These
	// branches avoid dividing by zero while matching Lazer's reconstruction.
	switch {
	case hue < 1.0/6.0:
		hue = 6 * (hue - 0.0/6.0)
		result.R = math32.Sqrt(c.perceivedBrightness * c.perceivedBrightness /
			(perceivedBrightnessRedWeight + perceivedBrightnessGreenWeight*hue*hue))
		result.G = result.R * hue
	case hue < 2.0/6.0:
		hue = 6 * (-hue + 2.0/6.0)
		result.G = math32.Sqrt(c.perceivedBrightness * c.perceivedBrightness /
			(perceivedBrightnessGreenWeight + perceivedBrightnessRedWeight*hue*hue))
		result.R = result.G * hue
	case hue < 3.0/6.0:
		hue = 6 * (hue - 2.0/6.0)
		result.G = math32.Sqrt(c.perceivedBrightness * c.perceivedBrightness /
			(perceivedBrightnessGreenWeight + perceivedBrightnessBlueWeight*hue*hue))
		result.B = result.G * hue
	case hue < 4.0/6.0:
		hue = 6 * (-hue + 4.0/6.0)
		result.B = math32.Sqrt(c.perceivedBrightness * c.perceivedBrightness /
			(perceivedBrightnessBlueWeight + perceivedBrightnessGreenWeight*hue*hue))
		result.G = result.B * hue
	case hue < 5.0/6.0:
		hue = 6 * (hue - 4.0/6.0)
		result.B = math32.Sqrt(c.perceivedBrightness * c.perceivedBrightness /
			(perceivedBrightnessBlueWeight + perceivedBrightnessRedWeight*hue*hue))
		result.R = result.B * hue
	default:
		hue = 6 * (-hue + 6.0/6.0)
		result.R = math32.Sqrt(c.perceivedBrightness * c.perceivedBrightness /
			(perceivedBrightnessRedWeight + perceivedBrightnessBlueWeight*hue*hue))
		result.B = result.R * hue
	}

	return result
}
