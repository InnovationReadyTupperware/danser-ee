package settings

import (
	"math"
	"testing"
)

func TestBeatFlashOffset(t *testing.T) {
	tests := []struct {
		name                       string
		beatScale, configured, amp float64
		want                       float64
	}{
		{name: "scaled flash", beatScale: 1.25, configured: 1.5, amp: 50, want: 25},
		{name: "neutral configured scale", beatScale: 1, configured: 1, amp: 50},
		{name: "near neutral configured scale", beatScale: 2, configured: 1 + 1e-7, amp: 50},
		{name: "nonfinite beat scale", beatScale: math.NaN(), configured: 1.5, amp: 50},
		{name: "nonfinite configured scale", beatScale: 1.2, configured: math.Inf(1), amp: 50},
		{name: "nonfinite amplitude", beatScale: 1.2, configured: 1.5, amp: math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := beatFlashOffset(tt.beatScale, tt.configured, tt.amp); got != tt.want {
				t.Fatalf("beatFlashOffset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetColorsRepairsInvalidInputs(t *testing.T) {
	cl := color{
		BaseColor:             nil,
		EnableCustomHueOffset: true,
		HueOffset:             math.NaN(),
		currentHue:            math.Inf(1),
	}

	colors := cl.GetColors(2, math.NaN(), math.NaN())
	if len(colors) != 2 {
		t.Fatalf("GetColors() returned %d colors, want 2", len(colors))
	}

	for i, value := range colors {
		if !isFinite(float64(value.R)) || !isFinite(float64(value.G)) ||
			!isFinite(float64(value.B)) || !isFinite(float64(value.A)) {
			t.Fatalf("GetColors()[%d] is not finite: %#v", i, value)
		}
	}
}
