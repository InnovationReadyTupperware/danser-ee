package color

import (
	"math"
	"testing"
)

// overbrightBlue is a bright-blue combo color after perceived-brightness
// normalization; its blue channel exceeds 1, the packing saturation boundary.
var overbrightBlue = NewRGBA(0.296, 0.535, 1.019, 1.0)

// saturatedBlue is fully saturated; normalized at 0.2, its blue channel exceeds 1.
var saturatedBlue = NewIRGB(74, 134, 255)

func TestPackIntSaturatesOverbrightChannels(t *testing.T) {
	got := overbrightBlue.PackInt()

	r := uint8(got & 0xFF)
	g := uint8((got >> 8) & 0xFF)
	b := uint8((got >> 16) & 0xFF)
	a := uint8((got >> 24) & 0xFF)

	if r != 75 || g != 136 || b != 255 || a != 255 {
		t.Fatalf("packed = (%d,%d,%d,%d), want (75,136,255,255)", r, g, b, a)
	}
}

func TestPackIntSaturatesNegativeChannels(t *testing.T) {
	got := PackInt(-0.5, 0.5, 2.0, -1.0)

	if r := uint8(got & 0xFF); r != 0 {
		t.Fatalf("red byte = %d, want 0", r)
	}

	if b := uint8((got >> 16) & 0xFF); b != 255 {
		t.Fatalf("blue byte = %d, want 255", b)
	}

	if a := uint8((got >> 24) & 0xFF); a != 0 {
		t.Fatalf("alpha byte = %d, want 0", a)
	}
}

func TestPackFloatSaturatesOverbrightBlue(t *testing.T) {
	bits := math.Float32bits(overbrightBlue.PackFloat())

	if got := uint8((bits >> 16) & 0xFF); got != 255 {
		t.Fatalf("blue byte = %d, want 255", got)
	}
}

func TestNormalizedBluePacksBrightBlue(t *testing.T) {
	normalized := NormalizePerceivedBrightness(saturatedBlue, 0.2)
	packed := normalized.PackInt()

	r := uint8(packed & 0xFF)
	g := uint8((packed >> 8) & 0xFF)
	b := uint8((packed >> 16) & 0xFF)

	if b != 255 {
		t.Fatalf("blue byte = %d, want 255 (full color = (%d,%d,%d))", b, r, g, b)
	}

	if r < 60 || r > 90 || g < 120 || g > 150 {
		t.Fatalf("red/green bytes = (%d,%d), want a blue-dominant color", r, g)
	}
}

func TestToIntArraySaturatesOutOfRangeChannels(t *testing.T) {
	got := NewRGBA(1.5, 0.5, -0.25, 1.0).ToIntArray()
	want := []uint8{255, 127, 0, 255}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("channel %d = %d, want %d (full = %v)", i, got[i], want[i], got)
		}
	}
}
