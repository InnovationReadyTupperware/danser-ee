package difficulty

import (
	"math"
	"testing"

	"github.com/wieku/rplpa"
)

// Expectations follow osu!'s OsuScoreMultiplierCalculatorV1/V2 at
// 1a84c00929fa70b23a20e8109af1289f34b50af2, not the legacy ScoreV2 mod
func TestLazerScoreMultiplierRevisions(t *testing.T) {
	tests := []struct {
		name   string
		mods   []rplpa.ModInfo
		v1, v2 float64
	}{
		{"HR", []rplpa.ModInfo{{Acronym: "HR"}}, 1.06, 1.09},
		{"NF V2", []rplpa.ModInfo{{Acronym: "NF"}, {Acronym: "V2"}}, 0.5, 0.5},
		{"EZ default", []rplpa.ModInfo{{Acronym: "EZ"}}, 0.5, 0.8},
		{"EZ fewer retries", []rplpa.ModInfo{{Acronym: "EZ", Settings: map[string]any{"retries": 0}}}, 0.5, 0.8},
		{"EZ extra retry", []rplpa.ModInfo{{Acronym: "EZ", Settings: map[string]any{"retries": 3}}}, 0.5, 0.7},
		{"EZ floor", []rplpa.ModInfo{{Acronym: "EZ", Settings: map[string]any{"retries": 8}}}, 0.5, 0.4},
		{"SO", []rplpa.ModInfo{{Acronym: "SO"}}, 0.9, 0.95},
		{"TC", []rplpa.ModInfo{{Acronym: "TC"}}, 1, 1.02},
		{"HD", []rplpa.ModInfo{{Acronym: "HD"}}, 1.06, 1.04},
		{"HD approach circles", []rplpa.ModInfo{{Acronym: "HD", Settings: map[string]any{"only_fade_approach_circles": true}}}, 1, 1.02},
		{"CL", []rplpa.ModInfo{{Acronym: "CL"}}, 0.96, 0.985},
		{"CL without note lock", []rplpa.ModInfo{{Acronym: "CL", Settings: map[string]any{"classic_note_lock": false}}}, 0.96, 0.96},
		{"FL", []rplpa.ModInfo{{Acronym: "FL"}}, 1.12, 1.2},
		{"FL follow delay", []rplpa.ModInfo{{Acronym: "FL", Settings: map[string]any{"follow_delay": 0.0}}}, 1, 1.2},
		{"FL small", []rplpa.ModInfo{{Acronym: "FL", Settings: map[string]any{"size_multiplier": 0.5}}}, 1, 1.2},
		{"FL large", []rplpa.ModInfo{{Acronym: "FL", Settings: map[string]any{"size_multiplier": 1.5}}}, 1, 1.1},
		{"FL floor", []rplpa.ModInfo{{Acronym: "FL", Settings: map[string]any{"size_multiplier": 2.0}}}, 1, 1.02},
		{"FL fixed size", []rplpa.ModInfo{{Acronym: "FL", Settings: map[string]any{"combo_based_size": false}}}, 1, 1.04},
		{"FL large fixed size", []rplpa.ModInfo{{Acronym: "FL", Settings: map[string]any{"size_multiplier": 1.5, "combo_based_size": false}}}, 1, 1.02},
		{"DA unchanged", []rplpa.ModInfo{{Acronym: "DA"}}, 0.5, 1},
		{"DA product", []rplpa.ModInfo{{Acronym: "DA", Settings: map[string]any{"circle_size": 5.5, "drain_rate": 5.2, "overall_difficulty": 4.5, "approach_rate": 6.0}}}, 0.5, 0.253125},
		{"DA floor", []rplpa.ModInfo{{Acronym: "DA", Settings: map[string]any{"circle_size": 8.0, "drain_rate": 8.0}}}, 0.5, 0.1},
		{"DA HR base", []rplpa.ModInfo{{Acronym: "DA", Settings: map[string]any{"circle_size": 6.0}}, {Acronym: "HR"}}, 0.53, 0.545},
		{"DT", []rplpa.ModInfo{{Acronym: "DT"}}, 1.1, 1.23},
		{"NC custom", []rplpa.ModInfo{{Acronym: "NC", Settings: map[string]any{"speed_change": 1.29}}}, 1.04, 1.082},
		{"HT", []rplpa.ModInfo{{Acronym: "HT"}}, 0.3, 0.55},
		{"DC custom", []rplpa.ModInfo{{Acronym: "DC", Settings: map[string]any{"speed_change": 0.79}}}, 0.3, 0.55},
		{"RX CL", []rplpa.ModInfo{{Acronym: "RX"}, {Acronym: "CL"}}, 0.096, 0.0985},
		{"AP", []rplpa.ModInfo{{Acronym: "AP"}}, 0.1, 0.1},
		{"HDDTCL", []rplpa.ModInfo{{Acronym: "HD"}, {Acronym: "DT"}, {Acronym: "CL"}}, 1.11936, 1.260012},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, version := range []int32{30000001, 30000016, 30000017, 30000018, 0} {
				diff := NewDifficulty(5, 5, 5, 5)
				diff.SetMods2(test.mods)
				diff.ScoreVersion = version
				want := test.v2
				if version != 0 && version < 30000017 {
					want = test.v1
				}
				if got := diff.GetScoreMultiplier(); math.Abs(got-want) > 1e-12 {
					t.Errorf("version %d multiplier = %.12f, want %.12f", version, got, want)
				}
			}
		})
	}
}

func TestReplayScoreVersionSurvivesModOverrides(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetModsFromReplay(&rplpa.Replay{OsuVersion: 30000016, Mods: uint32(Flashlight)})
	clone := diff.Clone()
	clone.SetMods(Hidden)
	clone.SetMods2([]rplpa.ModInfo{{Acronym: "HR"}})
	if clone.ScoreVersion != 30000016 || math.Abs(clone.GetScoreMultiplier()-1.06) > 1e-12 {
		t.Fatalf("overridden clone lost V1 scoring: version %d, multiplier %g", clone.ScoreVersion, clone.GetScoreMultiplier())
	}
	clone.ScoreVersion = 0
	oldRevision := clone.Clone()
	oldRevision.ScoreVersion = 30000016
	if !diff.Clone().Equals(diff) || !clone.Equals(oldRevision) {
		t.Fatal("score revision must not change difficulty equality/cache identity")
	}
	diff.SetModsFromReplay(nil)
	if diff.ScoreVersion != 0 {
		t.Fatal("removing replay retained its score revision")
	}
}

func TestPlaybackRateWithoutSpeedModHasNoLazerScoreBonus(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetMods(Hidden)
	diff.Speed = 1.5
	if got := diff.GetScoreMultiplier(); got != 1.04 {
		t.Fatalf("rate override multiplier = %g, want 1.04", got)
	}
}
