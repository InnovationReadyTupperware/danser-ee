package settings

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/utils"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

type HSV struct {
	Hue, Saturation, Value float64
}

func (d *defaultsFactory) InitHSV() *HSV {
	return &HSV{
		Hue:        0,
		Saturation: 1,
		Value:      1,
	}
}

type color struct {
	EnableRainbow         bool    `label:"Enable Rainbow"`                                                //true
	RainbowSpeed          float64 `label:"Rainbow Speed" min:"-360" max:"360" format:"%.0f°/s"`           //8, degrees per second
	BaseColor             *HSV    `label:"Basic Color" short:"true"`                                      //0..360, if EnableRainbow is disabled then this value will be used to calculate base color
	EnableCustomHueOffset bool    `label:"Enable Custom Hue Offset"`                                      //false, false means that every iteration has an offset of i*360/n
	HueOffset             float64 `min:"-360" max:"360" format:"%.0f°" label:"Mirror Collage Hue Offset"` //0, custom hue offset for mirror collages
	FlashToTheBeat        bool    //true, objects size is changing with music peak amplitude
	FlashAmplitude        float64 `min:"-360" max:"360" format:"%.0f°"` //50, hue offset for flashes
	currentHue            float64
}

func (cl *color) Update(delta float64) {
	if cl.EnableRainbow {
		cl.currentHue += cl.RainbowSpeed / 1000.0 * delta

		cl.currentHue = mutils.Sanitize(cl.currentHue, 360)
	} else {
		cl.currentHue = 0
	}
}

func (cl *color) GetColors(divides int, beatScale, alpha float64) []color2.Color {
	if divides <= 0 {
		return nil
	}

	baseColor := HSV{Hue: 0, Saturation: 1, Value: 1}
	if cl.BaseColor != nil {
		baseColor = *cl.BaseColor
	}

	flashOffset := 0.0
	if cl.FlashToTheBeat {
		flashOffset = beatFlashOffset(beatScale, Audio.BeatScale, cl.FlashAmplitude)
	}

	hue := mutils.Sanitize(
		finiteOr(baseColor.Hue, 0)+finiteOr(cl.currentHue, 0)+flashOffset,
		360,
	)

	offset := 360.0 / float64(divides)
	if cl.EnableCustomHueOffset {
		offset = finiteOr(cl.HueOffset, 0)
	}

	return utils.GetColorsSV(
		hue,
		offset,
		divides,
		finiteOr(baseColor.Saturation, 1),
		finiteOr(baseColor.Value, 1),
		finiteOr(alpha, 1),
	)
}

func beatFlashOffset(beatScale, configuredScale, amplitude float64) float64 {
	if !isFinite(beatScale) || !isFinite(configuredScale) || !isFinite(amplitude) ||
		math.Abs(configuredScale-1) <= 1e-6 {
		return 0
	}

	offset := amplitude * (beatScale - 1) / (configuredScale - 1)
	return finiteOr(offset, 0)
}

func finiteOr(value, fallback float64) float64 {
	if !isFinite(value) {
		return fallback
	}

	return value
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
