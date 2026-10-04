package objects

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/wieku/rplpa"

	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestHiddenApproachOnlyBodyFadeUsesPreempt(t *testing.T) {
	for _, ar := range []float64{3, 5, 9, 10} {
		for _, rate := range []float64{0.75, 1, 1.5} {
			diff := difficulty.NewDifficulty(5, 5, 5, ar)
			diff.SetMods2([]rplpa.ModInfo{
				{Acronym: "HD", Settings: map[string]any{"only_fade_approach_circles": true}},
				{Acronym: "DT", Settings: map[string]any{"speed_change": rate}},
			})
			circle := testCircleWithHitSprites()
			circle.StartTime = 3000
			circle.hitCircle.SetAlpha(0)
			circle.addBodyFades(diff, []sprite.ISprite{circle.hitCircle}, 3000-diff.Preempt, 3000)
			circle.hitCircle.Update(3000 - diff.Preempt*0.8)
			if alpha := circle.hitCircle.GetAlpha(); math.Abs(alpha-0.5) > 1e-6 {
				t.Fatalf("AR %g rate %g body alpha = %g at 20%% of preempt, want 0.5", ar, rate, alpha)
			}
			circle.hitCircle.Update(3000 - diff.Preempt*0.6)
			if alpha := circle.hitCircle.GetAlpha(); math.Abs(alpha-1) > 1e-6 {
				t.Fatalf("AR %g rate %g body alpha = %g at 40%% of preempt, want 1", ar, rate, alpha)
			}
			circle.hitCircle.Update(3000)
			if circle.hitCircle.GetAlpha() != 1 {
				t.Fatal("approach-only body faded before judgment")
			}
		}
	}
}

func TestHiddenApproachOnlySliderPointFadeOverrides(t *testing.T) {
	oldSnaking := settings.Objects.Sliders.Snaking.In
	t.Cleanup(func() { settings.Objects.Sliders.Snaking.In = oldSnaking })
	settings.Objects.Sliders.Snaking.In = true
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetMods2([]rplpa.ModInfo{{Acronym: "HD", Settings: map[string]any{"only_fade_approach_circles": true}}})
	for _, first := range []bool{false, true} {
		circle := testCircleWithHitSprites()
		circle.StartTime = 3000
		circle.SliderPoint = true
		circle.appearTime = 1000
		circle.firstEndCircle = first
		circle.hitCircle.SetAlpha(0)
		circle.addBodyFades(diff, []sprite.ISprite{circle.hitCircle}, circle.appearTime, circle.StartTime)
		if first {
			circle.hitCircle.Update(1000 + diff.Preempt/3 - 1)
			if circle.hitCircle.GetAlpha() != 0 {
				t.Fatal("first end circle appeared before snaking offset")
			}
			circle.hitCircle.Update(1000 + diff.Preempt/3 + diff.Preempt*0.2)
			if math.Abs(circle.hitCircle.GetAlpha()-0.5) > 1e-6 {
				t.Fatal("first end circle lost its Hidden fade duration")
			}
		} else {
			circle.hitCircle.Update(1001)
			if circle.hitCircle.GetAlpha() != 1 {
				t.Fatal("later slider point lost immediate appearance")
			}
		}
		circle.hitCircle.Update(3001)
		if circle.hitCircle.GetAlpha() != 0 {
			t.Fatal("slider point remained visible past its end")
		}
	}
}

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

func TestDisabledHitAnimationsRetainSkinTransformsUnderParentFade(t *testing.T) {

	previousGeneral, previousSkin := settings.General, settings.Skin
	config := settings.NewConfigFile()
	settings.General, settings.Skin = config.General, config.Skin
	settings.General.OsuSkinsDir = t.TempDir()
	settings.Skin.CurrentSkin = "hit-animation-test"
	skinDir := filepath.Join(settings.General.OsuSkinsDir, settings.Skin.CurrentSkin)
	if err := os.Mkdir(skinDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skinDir, "skin.ini"), []byte("[General]\nVersion: 2.5\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { settings.General, settings.Skin = previousGeneral, previousSkin })
	previous := settings.Objects
	settings.Objects = settings.NewConfigFile().Objects
	settings.Objects.HitAnimations = false
	t.Cleanup(func() { settings.Objects = previous })
	for _, kind := range []string{"circle", "slider head", "slider tail"} {
		t.Run(kind, func(t *testing.T) {
			circle := testCircleWithHitSprites()
			circle.diff = difficulty.NewDifficulty(5, 5, 5, 5)
			circle.silent = true
			circle.sprites = []sprite.ISprite{circle.hitCircle, circle.hitCircleOverlay}
			if kind != "circle" {
				circle.SliderPoint = true
				circle.SliderPointStart = kind == "slider head"
				circle.SliderPointEnd = kind == "slider tail"
				circle.hitCircleTexture = new(texture.TextureRegion{})
			}
			if circle.SliderPointEnd {
				circle.ArmSliderPoint(true, 1000, 400)
			} else {
				circle.Arm(true, 1000)
			}
			circle.Update(1030)
			if circle.hitFade == nil || math.Abs(circle.hitFade.GetValue()-0.25) > 1e-9 {
				t.Fatal("disabled hit animation did not use a 60 ms OutQuad parent fade")
			}
			if circle.hitCircle.GetScale().X <= 1 || circle.hitCircle.GetAlpha() <= 0.25 {
				t.Fatal("short parent fade replaced the skin's scale or alpha transforms")
			}
			circle.Update(1060)
			if circle.hitFade.GetValue() != 0 {
				t.Fatal("disabled hit animation remained visible after 60 ms")
			}
			circle.Arm(false, 2000)
			if circle.hitFade != nil {
				t.Fatal("re-arming a miss retained the previous hit's parent fade")
			}
		})
	}
}

func TestDisabledHitAnimationsKeepSliderRepeatLifecycle(t *testing.T) {
	previous := settings.Objects
	settings.Objects = settings.NewConfigFile().Objects
	settings.Objects.HitAnimations = false
	t.Cleanup(func() { settings.Objects = previous })
	circle := testCircleWithHitSprites()
	circle.diff = difficulty.NewDifficulty(5, 5, 5, 5)
	circle.SliderPoint = true
	circle.hitCircleTexture = new(texture.TextureRegion{})
	circle.ArmSliderPoint(true, 1000, 400)
	circle.hitCircle.Update(1200)
	if circle.hitFade != nil || circle.hitCircle.GetScale().X != 1 || circle.hitCircle.GetAlpha() <= 0 {
		t.Fatal("slider repeat lost its span-limited fade without hit transforms")
	}
	circle.hitCircle.Update(1300)
	if circle.hitCircle.GetAlpha() > 0.001 || !circle.sliderPointLatched {
		t.Fatal("slider repeat did not expire at 300 ms or latch its position")
	}
}

func TestSliderAnimationSwitchStillSuppressesSkinTransforms(t *testing.T) {
	previous := settings.Objects
	settings.Objects = settings.NewConfigFile().Objects
	settings.Objects.Sliders.HitAnimations = false
	t.Cleanup(func() { settings.Objects = previous })
	circle := testCircleWithHitSprites()
	circle.diff = difficulty.NewDifficulty(5, 5, 5, 5)
	circle.SliderPoint, circle.SliderPointStart = true, true
	circle.hitCircleTexture = new(texture.TextureRegion{})
	circle.Arm(true, 1000)
	circle.hitCircle.Update(1030)
	if circle.hitCircle.GetScale().X != 1 || circle.hitFade != nil {
		t.Fatal("slider animation switch no longer suppressed slider hit transforms")
	}
}
