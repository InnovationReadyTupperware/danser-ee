package objects

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestSliderPointFadeDuration(t *testing.T) {
	tests := []struct {
		name         string
		isEnd        bool
		clicked      bool
		spanDuration float64
		want         float64
	}{
		{name: "repeat miss clamps to span", spanDuration: 180, want: 180},
		{name: "repeat miss caps at 300", spanDuration: 450, want: 300},
		{name: "repeat hit caps at 300", clicked: true, spanDuration: 450, want: 300},
		{name: "repeat invalid span is immediate", spanDuration: -1, want: 0},
		{name: "tail miss", isEnd: true, want: 100},
		{name: "tail hit without animation", isEnd: true, clicked: true, want: 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliderPointFadeDuration(tt.isEnd, tt.clicked, tt.spanDuration); got != tt.want {
				t.Fatalf("sliderPointFadeDuration() = %g, want %g", got, tt.want)
			}
		})
	}
}

func TestCircleArmMissUsesShortFade(t *testing.T) {
	circle := testCircleWithHitSprites()
	circle.Arm(false, 1000)
	circle.hitCircle.Update(1050)

	if got := circle.hitCircle.GetAlpha(); got <= 0 || got >= 1 {
		t.Fatalf("ordinary miss alpha at 50 ms = %g, want a partial fade", got)
	}

	circle.hitCircle.Update(1100)
	if got := circle.hitCircle.GetAlpha(); got > 0.001 {
		t.Fatalf("ordinary miss alpha at 100 ms = %g, want zero", got)
	}
}

func TestCircleSliderPointMissFadeDurations(t *testing.T) {
	repeat := testCircleWithHitSprites()
	repeat.SliderPoint = true
	repeat.ArmSliderPoint(false, 1000, 400)
	repeat.hitCircle.Update(1299)
	if got := repeat.hitCircle.GetAlpha(); got <= 0 {
		t.Fatalf("repeat miss alpha before 300 ms = %g, want visible", got)
	}
	repeat.hitCircle.Update(1300)
	if got := repeat.hitCircle.GetAlpha(); got > 0.001 {
		t.Fatalf("repeat miss alpha at 300 ms = %g, want zero", got)
	}

	tail := testCircleWithHitSprites()
	tail.SliderPoint = true
	tail.SliderPointEnd = true
	tail.ArmSliderPoint(false, 1000, 400)
	tail.hitCircle.Update(1100)
	if got := tail.hitCircle.GetAlpha(); got > 0.001 {
		t.Fatalf("tail miss alpha at 100 ms = %g, want zero", got)
	}
}

func TestCircleSuccessfulSliderRepeatLatchesPosition(t *testing.T) {
	circle := testCircleWithHitSprites()
	circle.SliderPoint = true

	first := vector.NewVec2f(100, 200)
	second := vector.NewVec2f(300, 400)
	circle.setSliderPosition(first, 0.5)
	circle.ArmSliderPoint(true, 1000, 400)
	circle.setSliderPosition(second, 1.5)

	if !circle.sliderPointLatched {
		t.Fatal("successful repeat was not latched")
	}
	if circle.StartPosRaw != first {
		t.Fatalf("latched repeat position = %#v, want %#v", circle.StartPosRaw, first)
	}
	if circle.ArrowRotation != 0.5 {
		t.Fatalf("latched repeat rotation = %g, want 0.5", circle.ArrowRotation)
	}
}

func testCircleWithHitSprites() *Circle {
	position := vector.NewVec2d(0, 0)
	return &Circle{
		HitObject:        &HitObject{},
		hitCircle:        sprite.NewSpriteSingle(nil, 0, position, vector.Centre),
		hitCircleOverlay: sprite.NewSpriteSingle(nil, 0, position, vector.Centre),
	}
}
