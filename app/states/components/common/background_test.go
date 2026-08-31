package common

import "testing"

func TestNDCToPixels(t *testing.T) {
	tests := []struct {
		name          string
		width, height float64
		x, y          float64
		wantX, wantY  float64
	}{
		{name: "center of 4:3", width: 1024, height: 768, wantX: 512, wantY: 384},
		{name: "top right of 16:9", width: 1920, height: 1080, x: 1, y: 1, wantX: 1920, wantY: 1080},
		{name: "bottom left of 16:10", width: 1920, height: 1200, x: -1, y: -1},
		{name: "quarter point of ultrawide", width: 3440, height: 1440, x: -0.5, y: 0.5, wantX: 860, wantY: 1080},
		{name: "portrait Y uses height", width: 1080, height: 1920, y: 0.5, wantX: 540, wantY: 1440},
		{name: "recording resolution", width: 3840, height: 2160, x: 0.5, y: -0.5, wantX: 2880, wantY: 540},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ndcToPixels(tt.x, tt.y, tt.width, tt.height)
			if got.X != tt.wantX || got.Y != tt.wantY {
				t.Fatalf("ndcToPixels() = (%v, %v), want (%v, %v)", got.X, got.Y, tt.wantX, tt.wantY)
			}
		})
	}
}
