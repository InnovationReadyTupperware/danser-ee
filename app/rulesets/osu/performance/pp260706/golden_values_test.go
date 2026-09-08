package pp260706

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
)

type difficultyGolden struct {
	total      float64
	aim        float64
	speed      float64
	reading    float64
	flashlight float64
}

type performanceGolden struct {
	total      float64
	aim        float64
	speed      float64
	accuracy   float64
	reading    float64
	flashlight float64
}

// These values come from running the representative fixture through the
// pinned ppy/osu reference source snapshot 48c4800e3ae4ee752452cdff83bd3787ccf3105f
// (osu!standard difficulty version 20260706) in a temporary C# oracle harness.
func TestRepresentativeFixtureMatchesReleasedModelGoldens(t *testing.T) {
	tests := []struct {
		name      string
		mods      difficulty.Modifier
		configure func(*difficulty.Difficulty)
		sr        difficultyGolden
		pp        performanceGolden
	}{
		{
			name: "NM",
			mods: difficulty.None,
			sr: difficultyGolden{
				total:   3.110238997338473,
				aim:     1.863418591209333,
				speed:   0.7024921687393265,
				reading: 0.2757642668360255,
			},
			pp: performanceGolden{
				total:    52.85722643425355,
				aim:      24.67811371065857,
				speed:    1.1580918320425684,
				accuracy: 24.767662016364564,
				reading:  0.08388300148186674,
			},
		},
		{
			name: "HD",
			mods: difficulty.Hidden,
			sr: difficultyGolden{
				total:   3.172826548047947,
				aim:     1.863418591209333,
				speed:   0.7024921687393265,
				reading: 0.8417526039293682,
			},
			pp: performanceGolden{
				total:    54.61124014233189,
				aim:      24.67811371065857,
				speed:    1.1580918320425684,
				accuracy: 24.767662016364564,
				reading:  2.3856866314590586,
			},
		},
		{
			name: "HD only fade approach circles",
			mods: difficulty.Hidden,
			configure: func(diff *difficulty.Difficulty) {
				settings := difficulty.NewHiddenSettings()
				settings.OnlyFadeApproachCircles = true
				difficulty.SetModConfig(diff, settings)
			},
			sr: difficultyGolden{
				total:   3.110238997338473,
				aim:     1.863418591209333,
				speed:   0.7024921687393265,
				reading: 0.2757642668360255,
			},
			pp: performanceGolden{
				total:    52.85722643425355,
				aim:      24.67811371065857,
				speed:    1.1580918320425684,
				accuracy: 24.767662016364564,
				reading:  0.08388300148186674,
			},
		},
		{
			name: "HR",
			mods: difficulty.HardRock,
			sr: difficultyGolden{
				total:   3.4161549230124297,
				aim:     2.05307706696789,
				speed:   0.7024921687393265,
				reading: 0.3183981526702439,
			},
			pp: performanceGolden{
				total:    96.34917928573401,
				aim:      33.0062697320124,
				speed:    1.363190666466907,
				accuracy: 57.3460008466334,
				reading:  0.12911348667347272,
			},
		},
		{
			name: "DT",
			mods: difficulty.DoubleTime,
			sr: difficultyGolden{
				total:   3.862553496349877,
				aim:     2.2574984420829547,
				speed:   1.0674767492049828,
				reading: 0.8888479819795916,
			},
			pp: performanceGolden{
				total:    105.93890270239588,
				aim:      43.87964107955972,
				speed:    4.571807853243835,
				accuracy: 51.6328230854214,
				reading:  2.808940006110978,
			},
		},
		{
			name: "FL",
			mods: difficulty.Flashlight,
			sr: difficultyGolden{
				total:      3.1434937960460596,
				aim:        1.863418591209333,
				speed:      0.7024921687393265,
				reading:    0.2757642668360255,
				flashlight: 0.22795116529802584,
			},
			pp: performanceGolden{
				total:      53.77989514225143,
				aim:        24.67811371065857,
				speed:      1.1580918320425684,
				accuracy:   24.767662016364564,
				reading:    0.08388300148186674,
				flashlight: 1.2990433440181974,
			},
		},
		{
			name: "HDDT",
			mods: difficulty.Hidden | difficulty.DoubleTime,
			sr: difficultyGolden{
				total:   3.900261787624183,
				aim:     2.2574984420829547,
				speed:   1.0674767492049828,
				reading: 1.0618149339326879,
			},
			pp: performanceGolden{
				total:    107.5436462360068,
				aim:      43.87964107955972,
				speed:    4.571807853243835,
				accuracy: 51.6328230854214,
				reading:  4.788577040606169,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beatMap := loadRegressionBeatmap(t, test.mods, difficulty.GameplayLazer)
			if test.configure != nil {
				test.configure(beatMap.Diff)
			}
			attributes := NewDifficultyCalculator().CalculateSingle(beatMap, beatMap.Diff)
			results := NewPPCalculator().Calculate(attributes, perfectPerfScore(attributes), beatMap.Diff)

			const difficultyTolerance = 1e-8
			assertOracleNear(t, "SR total", attributes.Total, test.sr.total, difficultyTolerance)
			assertOracleNear(t, "SR aim", attributes.Aim, test.sr.aim, difficultyTolerance)
			assertOracleNear(t, "SR speed", attributes.Speed, test.sr.speed, difficultyTolerance)
			assertOracleNear(t, "SR reading", attributes.Reading, test.sr.reading, difficultyTolerance)
			assertOracleNear(t, "SR flashlight", attributes.Flashlight, test.sr.flashlight, difficultyTolerance)

			// Fractional Math.Pow/math.Pow implementations differ by a few ULPs
			// across .NET and Go. PP amplifies those tiny SR component differences.
			const performanceTolerance = 1e-6
			assertOracleNear(t, "PP total", results.Total, test.pp.total, performanceTolerance)
			assertOracleNear(t, "PP aim", results.Aim, test.pp.aim, performanceTolerance)
			assertOracleNear(t, "PP speed", results.Speed, test.pp.speed, performanceTolerance)
			assertOracleNear(t, "PP accuracy", results.Acc, test.pp.accuracy, performanceTolerance)
			assertOracleNear(t, "PP reading", results.Reading, test.pp.reading, performanceTolerance)
			assertOracleNear(t, "PP flashlight", results.Flashlight, test.pp.flashlight, performanceTolerance)

			if attributes.ObjectCount != 20 || attributes.Circles != 15 || attributes.Sliders != 4 || attributes.Spinners != 1 || attributes.MaxCombo != 41 {
				t.Fatalf("fixture counts = objects %d circles %d sliders %d spinners %d combo %d, want 20/15/4/1/41",
					attributes.ObjectCount, attributes.Circles, attributes.Sliders, attributes.Spinners, attributes.MaxCombo)
			}
		})
	}
}

func perfectPerfScore(attributes api.Attributes) api.PerfScore {
	return api.PerfScore{
		Accuracy:       1,
		MaxCombo:       -1,
		CountGreat:     -1,
		SliderTailHits: -1,
	}
}

func assertOracleNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %.15f, upstream %.15f (+/-%g)", name, got, want, tolerance)
	}
}

func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()

	const tolerance = 1e-8
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %.15f, want %.15f (+/-%g)", name, got, want, tolerance)
	}
}
