package pp260706

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
)

func TestLazerSliderJudgementsReduceAimPerformance(t *testing.T) {
	beatMap := loadRegressionBeatmap(t, difficulty.None, difficulty.GameplayLazer)
	attributes := NewDifficultyCalculator().CalculateSingle(beatMap, beatMap.Diff)
	calculator := NewPPCalculator()

	perfectScore := perfectPerfScore(attributes)
	perfect := calculator.Calculate(attributes, perfectScore, beatMap.Diff)

	tailDroppedScore := perfectScore
	tailDroppedScore.SliderTailHits = attributes.Sliders - 1
	tailDropped := calculator.Calculate(attributes, tailDroppedScore, beatMap.Diff)

	tickMissScore := perfectScore
	tickMissScore.SliderTickMisses = 1
	tickMissScore.MaxCombo = attributes.MaxCombo - 1
	tickMiss := calculator.Calculate(attributes, tickMissScore, beatMap.Diff)

	if !(tailDropped.Aim < perfect.Aim && tailDropped.Total < perfect.Total) {
		t.Fatalf("dropped tail PP = aim %.6f total %.6f, want below perfect aim %.6f total %.6f", tailDropped.Aim, tailDropped.Total, perfect.Aim, perfect.Total)
	}
	if !(tickMiss.Aim < perfect.Aim && tickMiss.Total < perfect.Total) {
		t.Fatalf("large tick miss PP = aim %.6f total %.6f, want below perfect aim %.6f total %.6f", tickMiss.Aim, tickMiss.Total, perfect.Aim, perfect.Total)
	}
}

func TestClassicSliderAccuracyMatchesLazerReplayAndModSemantics(t *testing.T) {
	tests := []struct {
		name      string
		mods      difficulty.Modifier
		mode      difficulty.GameplayMode
		configure func(*difficulty.Difficulty)
		want      bool
	}{
		{name: "modern default", mode: difficulty.GameplayLazer},
		{name: "modern Classic default", mods: difficulty.Classic, mode: difficulty.GameplayLazer, want: true},
		{
			name: "modern Classic slider accuracy enabled",
			mods: difficulty.Classic,
			mode: difficulty.GameplayLazer,
			configure: func(diff *difficulty.Difficulty) {
				settings := difficulty.NewClassicSettings()
				settings.NoSliderHeadAccuracy = false
				difficulty.SetModConfig(diff, settings)
			},
		},
		{name: "Stable replay provenance", mode: difficulty.GameplayStable, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := difficulty.NewDifficulty(5, 5, 5, 5)
			diff.SetGameplayMode(test.mode)
			diff.SetMods(test.mods)
			if test.configure != nil {
				test.configure(diff)
			}

			if got := usesClassicSliderAccuracy(diff); got != test.want {
				t.Fatalf("usesClassicSliderAccuracy() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestLegacyTotalScoreFollowsClassicScoreV1Contract(t *testing.T) {
	legacyTotal := int64(12_000)
	baseScore := api.PerfScore{
		Accuracy:       0.925,
		CountGreat:     17,
		CountOk:        2,
		CountMiss:      1,
		SliderTailHits: 4,
	}

	tests := []struct {
		name           string
		mods           difficulty.Modifier
		mode           difficulty.GameplayMode
		configure      func(*difficulty.Difficulty)
		wantDifference bool
	}{
		{name: "Stable ScoreV1", mode: difficulty.GameplayStable, wantDifference: true},
		{name: "Stable ScoreV2", mods: difficulty.ScoreV2, mode: difficulty.GameplayStable, wantDifference: false},
		{name: "modern default", mode: difficulty.GameplayLazer, wantDifference: false},
		{name: "modern Classic with legacy total", mods: difficulty.Classic, mode: difficulty.GameplayLazer, wantDifference: true},
		{
			name: "modern Classic with slider head accuracy",
			mods: difficulty.Classic,
			mode: difficulty.GameplayLazer,
			configure: func(diff *difficulty.Difficulty) {
				settings := difficulty.NewClassicSettings()
				settings.NoSliderHeadAccuracy = false
				difficulty.SetModConfig(diff, settings)
			},
			wantDifference: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beatMap := loadRegressionBeatmap(t, test.mods, test.mode)
			if test.configure != nil {
				test.configure(beatMap.Diff)
			}
			attributes := NewDifficultyCalculator().CalculateSingle(beatMap, beatMap.Diff)
			score := baseScore
			score.MaxCombo = attributes.MaxCombo / 2
			score.SliderTailHits = attributes.Sliders

			withoutLegacy := NewPPCalculator().Calculate(attributes, score, beatMap.Diff)
			score.LegacyTotalScore = &legacyTotal
			withLegacy := NewPPCalculator().Calculate(attributes, score, beatMap.Diff)

			different := math.Abs(withLegacy.Total-withoutLegacy.Total) > 1e-9
			if different != test.wantDifference {
				t.Fatalf("legacy total changed PP = %v (without %.12f, with %.12f), want %v", different, withoutLegacy.Total, withLegacy.Total, test.wantDifference)
			}
		})
	}
}

func TestZeroLegacyTotalScoreUsesUpstreamLegacyMissPath(t *testing.T) {
	beatMap := loadRegressionBeatmap(t, difficulty.Classic, difficulty.GameplayLazer)
	attributes := NewDifficultyCalculator().CalculateSingle(beatMap, beatMap.Diff)
	zero := int64(0)
	score := api.PerfScore{
		Accuracy:         0.9,
		MaxCombo:         attributes.MaxCombo / 2,
		CountGreat:       max(0, attributes.ObjectCount-2),
		CountOk:          1,
		CountMiss:        1,
		SliderTailHits:   attributes.Sliders,
		LegacyTotalScore: &zero,
	}

	got := CalculateMissCount(score, attributes, beatMap.Diff)
	want := calculateMaximumComboBasedMissCount(score, attributes)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("zero legacy total miss count = %.15f, want upstream legacy fallback %.15f", got, want)
	}
}

func TestPerformanceClampsAccuracyComboAndSliderStatistics(t *testing.T) {
	beatMap := loadRegressionBeatmap(t, difficulty.None, difficulty.GameplayLazer)
	attributes := NewDifficultyCalculator().CalculateSingle(beatMap, beatMap.Diff)

	normalized := api.PerfScore{
		Accuracy:       1,
		MaxCombo:       attributes.MaxCombo,
		CountGreat:     attributes.ObjectCount,
		SliderTailHits: attributes.Sliders,
	}
	impossible := normalized
	impossible.Accuracy = 2
	impossible.MaxCombo = attributes.MaxCombo + 10_000
	impossible.SliderTickMisses = -100
	impossible.SliderTailHits = attributes.Sliders + 100

	want := NewPPCalculator().Calculate(attributes, normalized, beatMap.Diff)
	got := NewPPCalculator().Calculate(attributes, impossible, beatMap.Diff)

	assertNear(t, "clamped total PP", got.Total, want.Total)
	assertNear(t, "clamped aim PP", got.Aim, want.Aim)
	assertNear(t, "clamped speed PP", got.Speed, want.Speed)
	assertNear(t, "clamped accuracy PP", got.Acc, want.Acc)
	assertNear(t, "clamped reading PP", got.Reading, want.Reading)
}

func TestReleasedModComponentSemantics(t *testing.T) {
	tests := []struct {
		name  string
		mods  difficulty.Modifier
		check func(t *testing.T, result api.PPv2Results)
	}{
		{
			name: "Relax removes speed and accuracy",
			mods: difficulty.Relax,
			check: func(t *testing.T, result api.PPv2Results) {
				if result.Speed != 0 || result.Acc != 0 {
					t.Fatalf("Relax components = speed %.12f accuracy %.12f, want both zero", result.Speed, result.Acc)
				}
			},
		},
		{
			name: "Autopilot removes aim",
			mods: difficulty.Autopilot,
			check: func(t *testing.T, result api.PPv2Results) {
				if result.Aim != 0 {
					t.Fatalf("Autopilot aim PP = %.12f, want zero", result.Aim)
				}
			},
		},
		{
			name: "Flashlight keeps reading and flashlight separate",
			mods: difficulty.Flashlight,
			check: func(t *testing.T, result api.PPv2Results) {
				if result.Reading <= 0 || result.Flashlight <= 0 {
					t.Fatalf("Flashlight components = reading %.12f flashlight %.12f, want both positive", result.Reading, result.Flashlight)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beatMap := loadRegressionBeatmap(t, test.mods, difficulty.GameplayLazer)
			attributes := NewDifficultyCalculator().CalculateSingle(beatMap, beatMap.Diff)
			result := NewPPCalculator().Calculate(attributes, perfectPerfScore(attributes), beatMap.Diff)
			if math.IsNaN(result.Total) || math.IsInf(result.Total, 0) {
				t.Fatalf("total PP = %v, want finite", result.Total)
			}
			test.check(t, result)
		})
	}
}

func TestTraceableChangesPerformanceWithoutChangingStarRating(t *testing.T) {
	normalMap := loadRegressionBeatmap(t, difficulty.None, difficulty.GameplayLazer)
	normalAttributes := NewDifficultyCalculator().CalculateSingle(normalMap, normalMap.Diff)
	normalPP := NewPPCalculator().Calculate(normalAttributes, perfectPerfScore(normalAttributes), normalMap.Diff)

	traceableMap := loadRegressionBeatmap(t, difficulty.Traceable, difficulty.GameplayLazer)
	traceableAttributes := NewDifficultyCalculator().CalculateSingle(traceableMap, traceableMap.Diff)
	traceablePP := NewPPCalculator().Calculate(traceableAttributes, perfectPerfScore(traceableAttributes), traceableMap.Diff)

	assertNear(t, "Traceable SR", traceableAttributes.Total, normalAttributes.Total)
	if traceablePP.Aim <= normalPP.Aim || traceablePP.Acc <= normalPP.Acc || traceablePP.Total <= normalPP.Total {
		t.Fatalf("Traceable PP = aim %.12f acc %.12f total %.12f, want each above NM %.12f/%.12f/%.12f",
			traceablePP.Aim, traceablePP.Acc, traceablePP.Total, normalPP.Aim, normalPP.Acc, normalPP.Total)
	}
}
