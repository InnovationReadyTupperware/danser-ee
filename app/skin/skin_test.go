package skin

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/color"
)

func TestGetObjectColorNormalizesResolvedComboPaletteOnly(t *testing.T) {
	originalObjects := settings.Objects
	originalSkin := settings.Skin
	originalInfo := info
	originalBeatmapColors := beatmapColors

	t.Cleanup(func() {
		settings.Objects = originalObjects
		settings.Skin = originalSkin
		info = originalInfo
		beatmapColors = originalBeatmapColors
	})

	config := settings.NewConfigFile()
	settings.Objects = config.Objects
	settings.Skin = config.Skin
	info = newDefaultInfo()
	beatmapColors = nil

	settings.Objects.Colors.ComboColorNormalization = 1
	settings.Objects.Colors.UseComboColors = true
	settings.Objects.Colors.ComboColors = []*settings.HSV{{
		Hue:        0,
		Saturation: 1,
		Value:      1,
	}}

	base := color.NewRGB(0.2, 0.3, 0.4)
	comboColor := color.NewRGB(1, 0, 0)
	got := GetObjectColor(0, 0, base)
	want := color.NormalizePerceivedBrightness(comboColor, 1)

	if !almostEqualSkinColor(got, want) {
		t.Fatalf("normalized combo color = %#v, want %#v", got, want)
	}

	settings.Objects.Colors.UseComboColors = false
	got = GetObjectColor(0, 0, base)
	want = color.NewRGB(base.R, base.G, base.B)
	if got != want {
		t.Fatalf("base color fallback = %#v, want %#v", got, want)
	}
}

func almostEqualSkinColor(got, want color.Color) bool {
	const tolerance = float32(0.00001)

	return absSkinColorValue(got.R-want.R) <= tolerance &&
		absSkinColorValue(got.G-want.G) <= tolerance &&
		absSkinColorValue(got.B-want.B) <= tolerance &&
		absSkinColorValue(got.A-want.A) <= tolerance
}

func absSkinColorValue(value float32) float32 {
	if value < 0 {
		return -value
	}

	return value
}
