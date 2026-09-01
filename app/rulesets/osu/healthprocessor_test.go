package osu

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestHealthProcessorSelectionRespectsClassicHealth(t *testing.T) {
	tests := []struct {
		name          string
		mode          difficulty.GameplayMode
		classicHealth bool
		wantV2        bool
	}{
		{name: "Stable", mode: difficulty.GameplayStable},
		{name: "Lazer", mode: difficulty.GameplayLazer, wantV2: true},
		{name: "Lazer ClassicHealth", mode: difficulty.GameplayLazer, classicHealth: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := difficulty.NewDifficulty(5, 5, 5, 5)
			diff.SetGameplayMode(test.mode)
			player := &difficultyPlayer{diff: diff, classicHealth: test.classicHealth}
			beatMap := &beatmap.BeatMap{HitObjects: []objects.IHitObject{
				objects.DummyCircle(vector.NewVec2f(0, 0), 1000),
			}}

			_, gotV2 := newHealthProcessor(beatMap, player, true).(*HealthProcessorV2)
			if gotV2 != test.wantV2 {
				t.Fatalf("health processor is V2 = %t, want %t", gotV2, test.wantV2)
			}
		})
	}
}

func TestHealthProcessorsDoNotDrainBeforeGameplayStarts(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	beatMap := &beatmap.BeatMap{HitObjects: []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(0, 0), 10000),
	}}
	player := &difficultyPlayer{diff: diff}

	legacy := NewHealthProcessor(beatMap, player, true)
	legacy.PassiveDrain = 0.1
	legacy.ResetHp()
	legacy.Update(10000)

	wantLegacyHealth := (MaxHp - legacy.PassiveDrain*float64(diff.Preempt)) / MaxHp
	if got := legacy.GetHealth(); got != wantLegacyHealth {
		t.Fatalf("legacy health after a late first update = %g, want %g", got, wantLegacyHealth)
	}

	lazer := NewHealthProcessorV2(beatMap, player)
	lazer.passiveDrain = 0.0001
	lazer.ResetHp()
	lazer.Update(10000)

	if got := lazer.GetHealth(); got != 1 {
		t.Fatalf("Lazer health after a late first update = %g, want 1", got)
	}
}

func TestHealthProcessorsDrainIntervalsAcrossSkippedFrames(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	player := &difficultyPlayer{diff: diff}
	periods := []drainPeriod{{start: 100, end: 200}, {start: 300, end: 350}}

	legacy := &HealthProcessor{
		player:         player,
		PassiveDrain:   0.1,
		health:         MaxHp,
		healthUncapped: MaxHp,
		drains:         periods,
	}
	legacy.Update(400)

	if got, want := legacy.GetHealth(), 0.925; math.Abs(got-want) > 1e-12 {
		t.Fatalf("legacy health after crossing all drain periods = %g, want %g", got, want)
	}

	lazer := &HealthProcessorV2{
		player:       player,
		passiveDrain: 0.001,
		health:       1,
		drains:       periods,
	}
	lazer.Update(400)

	if got, want := lazer.GetHealth(), 0.85; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Lazer health after crossing all drain periods = %g, want %g", got, want)
	}
}

func TestLazerHealthUsesParticipantSliderEndTime(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayLazer)
	player := &difficultyPlayer{diff: diff}
	slider := &objects.Slider{
		HitObject:    &objects.HitObject{StartTime: 100, EndTime: 200},
		EndTimeLazer: 240.75,
	}
	beatMap := &beatmap.BeatMap{HitObjects: []objects.IHitObject{slider}}

	processor := NewHealthProcessorV2(beatMap, player)
	processor.passiveDrain = 0.001
	processor.ResetHp()
	processor.Update(220)

	if got, want := processor.GetHealth(), 0.88; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Lazer health through the true slider end = %g, want %g", got, want)
	}
	if got, want := processor.drains[0].end, int64(240); got != want {
		t.Fatalf("Lazer drain period end = %d, want %d", got, want)
	}

	legacy := NewHealthProcessor(beatMap, player, true)
	if got, want := legacy.drains[0].end, int64(240); got != want {
		t.Fatalf("legacy Lazer drain period end = %d, want %d", got, want)
	}

	classicPlayer := &difficultyPlayer{diff: diff, classicHealth: true}
	if got, want := healthObjectEndTime(slider, classicPlayer), slider.EndTime; got != want {
		t.Fatalf("ClassicHealth end time = %g, want Stable end time %g", got, want)
	}

	classicLegacy := NewHealthProcessor(beatMap, classicPlayer, true)
	if got, want := classicLegacy.drains[0].end, int64(slider.EndTime); got != want {
		t.Fatalf("legacy ClassicHealth drain period end = %d, want %d", got, want)
	}
}

func TestClassicCursorDanceIgnoresPathologicalSliderDetailsForHealth(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayLazer)
	slider := objects.NewSlider([]string{
		"167", "300", "1000", "6", "0",
		"L|167:-65535", "1", "102400", "12", "0:0|0:0",
	})
	if slider == nil || !slider.IsPathological() {
		t.Fatal("test slider was not classified as pathological")
	}

	tests := []struct {
		name          string
		cursorDance   bool
		classicHealth bool
		wantHealth    float64
	}{
		{name: "generated CL", cursorDance: true, classicHealth: true, wantHealth: 1},
		{name: "replay CL", cursorDance: false, classicHealth: true, wantHealth: 0.925},
		{name: "generated non-CL", cursorDance: true, wantHealth: 0.925},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			player := &difficultyPlayer{
				cursor:        &graphics.Cursor{IsCursorDance: test.cursorDance},
				diff:          diff,
				classicHealth: test.classicHealth,
			}
			processor := &HealthProcessor{
				player:         player,
				health:         MaxHp,
				healthUncapped: MaxHp,
			}
			runtimeSlider := &Slider{hitSlider: slider}

			processor.AddResult(JudgementResult{
				HitResult:  LargeTickMiss,
				MaxResult:  LargeTickHit,
				object:     runtimeSlider,
				sliderPart: sliderPartTick,
			})

			if got := processor.GetHealth(); got != test.wantHealth {
				t.Fatalf("health after pathological nested miss = %g, want %g", got, test.wantHealth)
			}
		})
	}
}
