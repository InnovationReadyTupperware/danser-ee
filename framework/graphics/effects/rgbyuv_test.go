package effects

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestConvertBT709LimitedReferencePatches(t *testing.T) {
	tests := []struct {
		name string
		rgb  mgl32.Vec3
		want mgl32.Vec3
	}{
		{name: "black", rgb: mgl32.Vec3{0, 0, 0}, want: mgl32.Vec3{16.0 / 255, 128.0 / 255, 128.0 / 255}},
		{name: "white", rgb: mgl32.Vec3{1, 1, 1}, want: mgl32.Vec3{235.0 / 255, 128.0 / 255, 128.0 / 255}},
		{name: "red", rgb: mgl32.Vec3{1, 0, 0}, want: mgl32.Vec3{62.0 / 255, 102.0 / 255, 240.0 / 255}},
		{name: "green", rgb: mgl32.Vec3{0, 1, 0}, want: mgl32.Vec3{173.0 / 255, 42.0 / 255, 26.0 / 255}},
		{name: "blue", rgb: mgl32.Vec3{0, 0, 1}, want: mgl32.Vec3{32.0 / 255, 240.0 / 255, 118.0 / 255}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertBT709Limited(tt.rgb)
			for channel := range 3 {
				if math.Abs(float64(got[channel]-tt.want[channel])) > 1.5/255 {
					t.Fatalf("channel %d = %.6f, want %.6f", channel, got[channel], tt.want[channel])
				}
			}
		})
	}
}
