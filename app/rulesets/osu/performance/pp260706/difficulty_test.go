package pp260706

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestDifficultyOnlyParsingUsesLazerStackingIndependentOfGameplayMode(t *testing.T) {
	beatMap := loadRegressionBeatmap(t, difficulty.None, difficulty.GameplayLazer)

	// The repeat slider at 4250ms is stacked by osu!lazer because its fractional
	// slider end extends into the following stack window. Difficulty-only parsing
	// must keep that stack geometry independent of the selected gameplay mode.
	const sliderIndex = 10
	if got := beatMap.HitObjects[sliderIndex].GetStackIndexMod(beatMap.Diff); got != 1 {
		t.Fatalf("lazer stack index = %d, want 1", got)
	}

	// osu!lazer applies the same stacking pass to Stable replays carrying
	// Classic, so changing gameplay mode must not select another stack map.
	beatMap.Diff.SetGameplayMode(difficulty.GameplayStable)
	beatMap.Diff.SetMods(difficulty.Classic)
	if got := beatMap.HitObjects[sliderIndex].GetStackIndexMod(beatMap.Diff); got != 1 {
		t.Fatalf("Stable + Classic stack index = %d, want shared Lazer index 1", got)
	}
}

func TestCalculateStepHandlesSingleSpinnerMap(t *testing.T) {
	beatMap := beatmap.NewBeatMap()
	beatMap.HitObjects = []objects.IHitObject{objects.NewDummySpinner(1000, 2000)}

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	attributes := NewDifficultyCalculator().CalculateStep(beatMap, diff)

	if len(attributes) != 1 {
		t.Fatalf("step attributes = %d, want one result for one hit object", len(attributes))
	}
}

func TestCalculateStepFinalAttributesMatchSingleCalculation(t *testing.T) {
	beatMap := beatmap.NewBeatMap()
	beatMap.HitObjects = []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(64, 64), 1000),
		objects.DummyCircle(vector.NewVec2f(256, 192), 1250),
		objects.DummyCircle(vector.NewVec2f(384, 96), 1500),
		objects.DummyCircle(vector.NewVec2f(128, 288), 1750),
		objects.DummyCircle(vector.NewVec2f(448, 192), 2000),
	}

	diff := difficulty.NewDifficulty(5, 4, 8, 9)
	calculator := NewDifficultyCalculator()
	final := calculator.CalculateSingle(beatMap, diff)
	steps := calculator.CalculateStep(beatMap, diff)
	if len(steps) != len(beatMap.HitObjects) {
		t.Fatalf("step attributes = %d, want %d", len(steps), len(beatMap.HitObjects))
	}

	last := steps[len(steps)-1]
	assertNear := func(name string, got, want float64) {
		t.Helper()
		const tolerance = 1e-12
		if math.Abs(got-want) > tolerance {
			t.Fatalf("%s = %.15f, want %.15f", name, got, want)
		}
	}

	assertNear("Total", last.Total, final.Total)
	assertNear("Aim", last.Aim, final.Aim)
	assertNear("AimNoSliders", last.AimNoSliders, final.AimNoSliders)
	assertNear("Speed", last.Speed, final.Speed)
	assertNear("Reading", last.Reading, final.Reading)
	assertNear("Flashlight", last.Flashlight, final.Flashlight)
	assertNear("SpeedNoteCount", last.SpeedNoteCount, final.SpeedNoteCount)
	assertNear("AimDifficultStrainCount", last.AimDifficultStrainCount, final.AimDifficultStrainCount)
	assertNear("SpeedDifficultStrainCount", last.SpeedDifficultStrainCount, final.SpeedDifficultStrainCount)
	assertNear("ReadingDifficultNoteCount", last.ReadingDifficultNoteCount, final.ReadingDifficultNoteCount)

	if last.MaxCombo != final.MaxCombo || last.ObjectCount != final.ObjectCount {
		t.Fatalf("final step counts = combo %d objects %d, want combo %d objects %d", last.MaxCombo, last.ObjectCount, final.MaxCombo, final.ObjectCount)
	}
}

func TestRepresentativeFixtureTimedFinalParityAcrossMods(t *testing.T) {
	tests := []struct {
		name string
		mods difficulty.Modifier
	}{
		{name: "NM", mods: difficulty.None},
		{name: "HD", mods: difficulty.Hidden},
		{name: "HR", mods: difficulty.HardRock},
		{name: "DT", mods: difficulty.DoubleTime},
		{name: "FL", mods: difficulty.Flashlight},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beatMap := loadRegressionBeatmap(t, test.mods, difficulty.GameplayLazer)
			calculator := NewDifficultyCalculator()
			final := calculator.CalculateSingle(beatMap, beatMap.Diff)
			steps := calculator.CalculateStep(beatMap, beatMap.Diff)
			if len(steps) != len(beatMap.HitObjects) {
				t.Fatalf("step attributes = %d, want %d", len(steps), len(beatMap.HitObjects))
			}

			for i, step := range steps {
				assertFiniteAttributes(t, i, step)
			}

			last := steps[len(steps)-1]
			assertNear(t, "final step total", last.Total, final.Total)
			assertNear(t, "final step aim", last.Aim, final.Aim)
			assertNear(t, "final step speed", last.Speed, final.Speed)
			assertNear(t, "final step reading", last.Reading, final.Reading)
			assertNear(t, "final step flashlight", last.Flashlight, final.Flashlight)
			assertNear(t, "final step slider factor", last.SliderFactor, final.SliderFactor)
			if last.MaxCombo != final.MaxCombo || last.ObjectCount != final.ObjectCount {
				t.Fatalf("final step counts = combo %d objects %d, want combo %d objects %d", last.MaxCombo, last.ObjectCount, final.MaxCombo, final.ObjectCount)
			}
		})
	}
}

func TestHiddenOnlyFadeApproachCirclesDoesNotApplyReadingHiddenBonus(t *testing.T) {
	nmMap := loadRegressionBeatmap(t, difficulty.None, difficulty.GameplayLazer)
	hdMap := loadRegressionBeatmap(t, difficulty.Hidden, difficulty.GameplayLazer)
	hiddenSettings := difficulty.NewHiddenSettings()
	hiddenSettings.OnlyFadeApproachCircles = true
	difficulty.SetModConfig(hdMap.Diff, hiddenSettings)

	calculator := NewDifficultyCalculator()
	nm := calculator.CalculateSingle(nmMap, nmMap.Diff)
	hdOnlyApproach := calculator.CalculateSingle(hdMap, hdMap.Diff)

	if math.Abs(nm.Reading-hdOnlyApproach.Reading) > 1e-12 {
		t.Fatalf("HD only-fade Reading = %.15f, NM %.15f; want identical Reading difficulty", hdOnlyApproach.Reading, nm.Reading)
	}
}

func TestDifficultyEdgeCasesRemainFinite(t *testing.T) {
	tests := []struct {
		name    string
		objects []objects.IHitObject
	}{
		{name: "single circle", objects: []objects.IHitObject{objects.DummyCircle(vector.NewVec2f(256, 192), 1000)}},
		{name: "single spinner", objects: []objects.IHitObject{objects.NewDummySpinner(1000, 2000)}},
		{name: "simultaneous circles", objects: []objects.IHitObject{
			objects.DummyCircle(vector.NewVec2f(64, 64), 1000),
			objects.DummyCircle(vector.NewVec2f(256, 192), 1000),
			objects.DummyCircle(vector.NewVec2f(448, 320), 1000),
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beatMap := beatmap.NewBeatMap()
			beatMap.HitObjects = test.objects
			diff := difficulty.NewDifficulty(5, 4, 8, 9)
			attributes := NewDifficultyCalculator().CalculateSingle(beatMap, diff)
			assertFiniteAttributes(t, -1, attributes)
		})
	}

	calculator := NewDifficultyCalculator()
	if got := calculator.CalculateSingle(&beatmap.BeatMap{}, difficulty.NewDifficulty(5, 5, 5, 5)); got != (api.Attributes{}) {
		t.Fatalf("empty-map attributes = %#v, want zero attributes", got)
	}
	if got := calculator.CalculateStep(&beatmap.BeatMap{}, difficulty.NewDifficulty(5, 5, 5, 5)); got != nil {
		t.Fatalf("empty-map step attributes = %#v, want nil", got)
	}
}

func assertFiniteAttributes(t *testing.T, step int, attributes api.Attributes) {
	t.Helper()

	values := map[string]float64{
		"total":                        attributes.Total,
		"aim":                          attributes.Aim,
		"aim no sliders":               attributes.AimNoSliders,
		"speed":                        attributes.Speed,
		"reading":                      attributes.Reading,
		"flashlight":                   attributes.Flashlight,
		"slider factor":                attributes.SliderFactor,
		"speed note count":             attributes.SpeedNoteCount,
		"aim difficult strain count":   attributes.AimDifficultStrainCount,
		"speed difficult strain count": attributes.SpeedDifficultStrainCount,
		"reading difficult note count": attributes.ReadingDifficultNoteCount,
	}
	for name, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatalf("step %d %s = %v, want finite", step, name, value)
		}
	}
}

func TestObjectAfterSpinnerKeepsMinimumJumpTimeForRhythmHistory(t *testing.T) {
	beatMap := beatmap.NewBeatMap()
	beatMap.HitObjects = []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(64, 64), 1000),
		objects.NewDummySpinner(1500, 2500),
		objects.DummyCircle(vector.NewVec2f(256, 192), 3000),
		objects.DummyCircle(vector.NewVec2f(448, 320), 3250),
	}

	diff := difficulty.NewDifficulty(5, 4, 8, 9)
	diffObjects := preprocessing.CreateDifficultyObjects(beatMap.HitObjects, diff)
	if len(diffObjects) != 3 {
		t.Fatalf("difficulty objects = %d, want 3", len(diffObjects))
	}

	afterSpinner := diffObjects[1]
	if afterSpinner.MinimumJumpTime <= 0 || afterSpinner.MinimumJumpTime != afterSpinner.AdjustedDeltaTime {
		t.Fatalf("post-spinner minimum jump time = %v, want adjusted delta %v", afterSpinner.MinimumJumpTime, afterSpinner.AdjustedDeltaTime)
	}
}
