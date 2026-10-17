package common

import (
	"errors"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/effects"
)

func TestBackgroundBlurCreatedWhenEnabled(t *testing.T) {
	previousEnabled := settings.Playfield.Background.Blur.Enabled
	t.Cleanup(func() { settings.Playfield.Background.Blur.Enabled = previousEnabled })
	settings.Playfield.Background.Blur.Enabled = false

	bg := &Background{allowBlur: true}
	created := 0
	create := func(_, _ int) (*effects.BlurEffect, error) {
		created++
		return &effects.BlurEffect{}, nil
	}

	bg.initializeBlur(create)
	if created != 0 || bg.blur != nil {
		t.Fatal("disabled blur allocated a render target")
	}

	settings.Playfield.Background.Blur.Enabled = true
	bg.initializeBlur(create)
	if created != 1 || bg.blur == nil {
		t.Fatal("enabling blur did not allocate its render targets")
	}

	settings.Playfield.Background.Blur.Enabled = false
	bg.initializeBlur(create)
	settings.Playfield.Background.Blur.Enabled = true
	bg.initializeBlur(create)
	if created != 1 {
		t.Fatalf("blur was allocated %d times, want once", created)
	}
}

func TestBackgroundBlurDisabledForLauncher(t *testing.T) {
	previousEnabled := settings.Playfield.Background.Blur.Enabled
	t.Cleanup(func() { settings.Playfield.Background.Blur.Enabled = previousEnabled })
	settings.Playfield.Background.Blur.Enabled = true

	bg := &Background{}
	bg.initializeBlur(func(_, _ int) (*effects.BlurEffect, error) {
		t.Fatal("launcher background must not allocate blur")
		return nil, nil
	})
}

func TestBackgroundBlurFailureRetryRequiresOffOnToggle(t *testing.T) {
	previousEnabled := settings.Playfield.Background.Blur.Enabled
	t.Cleanup(func() { settings.Playfield.Background.Blur.Enabled = previousEnabled })
	settings.Playfield.Background.Blur.Enabled = true

	bg := &Background{allowBlur: true}
	attempts := 0
	create := func(_, _ int) (*effects.BlurEffect, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("test render target failure")
		}
		return &effects.BlurEffect{}, nil
	}
	bg.initializeBlur(create)
	for range 4 {
		bg.initializeBlur(create)
	}
	if attempts != 1 || bg.blur != nil {
		t.Fatalf("blur failure produced %d attempts and blur %v, want one attempt and nil blur", attempts, bg.blur)
	}
	settings.Playfield.Background.Blur.Enabled = false
	bg.initializeBlur(create)
	settings.Playfield.Background.Blur.Enabled = true
	bg.initializeBlur(create)
	if attempts != 2 || bg.blur == nil {
		t.Fatalf("blur re-enable produced %d attempts and blur %v, want a successful second attempt", attempts, bg.blur)
	}
	bg.initializeBlur(create)
	if attempts != 2 {
		t.Fatalf("working blur allocated %d times, want once after recovery", attempts)
	}
}

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
