package osu

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/wieku/rplpa"
)

func TestLazerScoreProcessorUsesParticipantScoreRevision(t *testing.T) {
	beatMap := &beatmap.BeatMap{HitObjects: []objects.IHitObject{objects.NewCircle([]string{"256", "192", "1000", "1", "0"})}}
	for _, forceClassic := range []bool{false, true} {
		for _, test := range []struct {
			version int32
			want    int64
		}{{30000016, 1120000}, {30000017, 1200000}, {0, 1200000}} {
			diff := difficulty.NewDifficulty(5, 5, 5, 5)
			diff.SetModsFromReplay(&rplpa.Replay{OsuVersion: test.version, Mods: uint32(difficulty.Flashlight)})
			if test.version == 0 {
				// Generated playback has no replay decoder to inject Classic
				diff.SetMods(difficulty.Flashlight)
			}
			processor := newScoreV3Processor(forceClassic)
			processor.Init(beatMap, &difficultyPlayer{diff: diff})
			processor.AddResult(createJudgementResult(Hit300, Hit300, Increase, 1000, beatMap.HitObjects[0].GetStartPosition(), nil))
			want := test.want
			if forceClassic {
				want = ClassicDisplayScore(want, 1)
			}
			if got := processor.GetScore(); got != want {
				t.Fatalf("version %d Classic display %t score = %d, want %d", test.version, forceClassic, got, want)
			}
		}
	}
}

func TestStableScoreMigrationUsesCurrentMultiplier(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)
	diff.SetModsFromReplay(&rplpa.Replay{OsuVersion: 20250101, Mods: uint32(difficulty.Flashlight)})
	beatMap := &beatmap.BeatMap{HitObjects: []objects.IHitObject{objects.NewCircle([]string{"256", "192", "1000", "1", "0"})}}
	replay := RecordedStableScore{TotalScore: 300, MaxCombo: 1, Count300: 1}
	if got := standardisedStableScore(replay, beatMap, diff, 1); got != 1182000 {
		t.Fatalf("migrated FL CL perfect score = %d, want current 1.2 * 0.985 scale", got)
	}
	if diff.ScoreVersion != 20250101 || diff.GetGameplayMode() != difficulty.GameplayStable {
		t.Fatal("score migration changed source replay provenance")
	}
}
