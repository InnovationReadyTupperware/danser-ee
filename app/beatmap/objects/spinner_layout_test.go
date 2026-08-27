package objects

import (
	"math"
	"testing"
)

func TestSpinnerRPMCounterPositionMatchesLazerLegacyLayout(t *testing.T) {
	tests := []struct {
		name         string
		scaledWidth  float64
		scaledHeight float64
		wantX        float64
		wantY        float64
	}{
		{name: "4:3 logical HUD", scaledWidth: 1024, scaledHeight: 768, wantX: 640, wantY: 716.8},
		{name: "16:9 logical HUD", scaledWidth: 1365.3333333333333, scaledHeight: 768, wantX: 810.6666666666666, wantY: 716.8},
		{name: "1440p logical HUD", scaledWidth: 2560, scaledHeight: 1440, wantX: 1520, wantY: 1344},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := spinnerRPMCounterPosition(test.scaledWidth, test.scaledHeight)

			if math.Abs(got.X-test.wantX) > 1e-9 || math.Abs(got.Y-test.wantY) > 1e-9 {
				t.Fatalf("spinner RPM counter position = %#v, want (%v, %v)", got, test.wantX, test.wantY)
			}
		})
	}
}
