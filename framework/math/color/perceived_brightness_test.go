package color

import (
	"math"
	"testing"
)

func TestNormalizePerceivedBrightnessUsesLazerHSPAFormula(t *testing.T) {
	input := NewRGBA(1, 0, 0, 0.37)
	got := NormalizePerceivedBrightness(input, 1)
	wantRed := float32(0.6 / math.Sqrt(float64(perceivedBrightnessRedWeight)))

	if !almostEqualColorValue(got.R, wantRed) {
		t.Fatalf("red channel = %v, want %v", got.R, wantRed)
	}
	if got.G != 0 || got.B != 0 {
		t.Fatalf("pure-red reconstruction = (%v, %v, %v), want zero green and blue", got.R, got.G, got.B)
	}
	if got.A != input.A {
		t.Fatalf("alpha = %v, want %v", got.A, input.A)
	}
	if brightness := testPerceivedBrightness(got); !almostEqualColorValue(brightness, targetPerceivedBrightness) {
		t.Fatalf("perceived brightness = %v, want %v", brightness, targetPerceivedBrightness)
	}
}

func TestNormalizePerceivedBrightnessInterpolatesWithoutChangingAlpha(t *testing.T) {
	input := NewRGBA(0.2, 0.4, 0.9, 0.63)
	full := NormalizePerceivedBrightness(input, 1)
	half := NormalizePerceivedBrightness(input, 0.5)
	want := input.Mix(full, 0.5)

	if !almostEqualColor(half, want) {
		t.Fatalf("half normalization = %#v, want interpolation result %#v", half, want)
	}
	if half.A != input.A {
		t.Fatalf("half normalization alpha = %v, want %v", half.A, input.A)
	}
}

func TestNormalizePerceivedBrightnessReturnsOriginalAtZero(t *testing.T) {
	input := NewRGBA(0.2, 0.4, 0.9, 0.63)

	if got := NormalizePerceivedBrightness(input, 0); got != input {
		t.Fatalf("zero normalization = %#v, want original %#v", got, input)
	}
}

func TestNormalizePerceivedBrightnessHandlesGrayscale(t *testing.T) {
	input := NewRGBA(0.2, 0.2, 0.2, 0.5)
	got := NormalizePerceivedBrightness(input, 1)

	for channel, value := range map[string]float32{"R": got.R, "G": got.G, "B": got.B} {
		if !almostEqualColorValue(value, targetPerceivedBrightness) {
			t.Fatalf("%s channel = %v, want %v", channel, value, targetPerceivedBrightness)
		}
	}
}

func TestNormalizePerceivedBrightnessNormalizesAmount(t *testing.T) {
	input := NewRGBA(0.2, 0.4, 0.9, 0.63)
	want := NormalizePerceivedBrightness(input, 1)

	tests := []struct {
		name   string
		amount float32
	}{
		{name: "negative amount uses absolute value", amount: -1},
		{name: "amount above one is capped", amount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePerceivedBrightness(input, tt.amount)
			if !almostEqualColor(got, want) {
				t.Fatalf("normalized color = %#v, want %#v", got, want)
			}
		})
	}
}

func TestNormalizePerceivedBrightnessLeavesInvalidAmountFinite(t *testing.T) {
	input := NewRGBA(0.2, 0.4, 0.9, 0.63)

	if got := NormalizePerceivedBrightness(input, float32(math.NaN())); got != input {
		t.Fatalf("NaN normalization = %#v, want original %#v", got, input)
	}
}

func testPerceivedBrightness(c Color) float32 {
	return float32(math.Sqrt(float64(
		c.R*c.R*perceivedBrightnessRedWeight +
			c.G*c.G*perceivedBrightnessGreenWeight +
			c.B*c.B*perceivedBrightnessBlueWeight,
	)))
}

func almostEqualColor(got, want Color) bool {
	return almostEqualColorValue(got.R, want.R) &&
		almostEqualColorValue(got.G, want.G) &&
		almostEqualColorValue(got.B, want.B) &&
		almostEqualColorValue(got.A, want.A)
}

func almostEqualColorValue(got, want float32) bool {
	return math.Abs(float64(got-want)) <= 0.00001
}
