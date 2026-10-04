package osu

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
)

func newFailModTestRuleset(t *testing.T, mods difficulty.Modifier) (*OsuRuleSet, *difficultyPlayer, *subSet) {
	t.Helper()
	set, player, subset := newFailurePolicyTestRuleset(t)
	player.diff.SetMods(mods)
	player.difficultyCacheKey = difficultyCacheKey{mode: difficulty.GameplayLazer, mods: player.diff.GetModStringMasked()}
	subset.hp = &HealthProcessorV2{player: player, health: 1}
	subset.ppv2 = &recordingPerformanceCalculator{}
	subset.scoreProcessor = &recordingScoreProcessor{}
	set.beatMap = &beatmap.BeatMap{}
	set.oppDiffs = map[difficultyCacheKey][]api.Attributes{player.difficultyCacheKey: {{ObjectCount: 1}, {ObjectCount: 2}}}
	return set, player, subset
}

func TestLazerFailModsUseNestedJudgmentsWithoutReplacingThem(t *testing.T) {
	for _, test := range []struct {
		name            string
		mod             difficulty.Modifier
		result, maximum HitResult
		part            sliderJudgementPart
		tailFail        bool
		wantFail        bool
	}{
		{name: "perfect 100", mod: difficulty.Perfect, result: Hit100, maximum: Hit300, wantFail: true},
		{name: "perfect large tick", mod: difficulty.Perfect, result: LargeTickMiss, maximum: LargeTickHit, part: sliderPartTick, wantFail: true},
		{name: "perfect small tick", mod: difficulty.Perfect, result: SmallTickMiss, maximum: SmallTickHit, part: sliderPartTail, wantFail: true},
		{name: "perfect ignored tail", mod: difficulty.Perfect, result: IgnoreMiss, maximum: SliderTailHit, part: sliderPartTail, wantFail: true},
		{name: "perfect successful tail", mod: difficulty.Perfect, result: SliderTailHit, maximum: SliderTailHit, part: sliderPartTail},
		{name: "perfect spinner bonus", mod: difficulty.Perfect, result: SpinnerBonus, maximum: SpinnerBonus},
		{name: "sudden death 100", mod: difficulty.SuddenDeath, result: Hit100, maximum: Hit300},
		{name: "sudden death tick", mod: difficulty.SuddenDeath, result: LargeTickMiss, maximum: LargeTickHit, part: sliderPartTick, wantFail: true},
		{name: "sudden death small tail default", mod: difficulty.SuddenDeath, result: SmallTickMiss, maximum: SmallTickHit, part: sliderPartTail},
		{name: "sudden death small tail enabled", mod: difficulty.SuddenDeath, result: SmallTickMiss, maximum: SmallTickHit, part: sliderPartTail, tailFail: true, wantFail: true},
		{name: "sudden death ignored tail default", mod: difficulty.SuddenDeath, result: IgnoreMiss, maximum: SliderTailHit, part: sliderPartTail},
		{name: "sudden death ignored tail enabled", mod: difficulty.SuddenDeath, result: IgnoreMiss, maximum: SliderTailHit, part: sliderPartTail, tailFail: true, wantFail: true},
		{name: "sudden death successful tail", mod: difficulty.SuddenDeath, result: SliderTailHit, maximum: SliderTailHit, part: sliderPartTail, tailFail: true},
		{name: "classic perfect 100", mod: difficulty.Classic | difficulty.Perfect, result: Hit100, maximum: Hit300, part: sliderPartSummary, wantFail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			set, player, subset := newFailModTestRuleset(t, test.mod)
			if test.mod.Active(difficulty.SuddenDeath) {
				difficulty.SetModConfig(player.diff, difficulty.SuddenDeathSettings{FailOnSliderTail: test.tailFail})
			}
			notified := 0
			set.SetFailListener(func(*graphics.Cursor) { notified++ })
			var received JudgementResult
			set.SetListener(func(_ *graphics.Cursor, result JudgementResult, _ Score) { received = result })
			set.SendResult(player.cursor, JudgementResult{HitResult: test.result, MaxResult: test.maximum, ComboResult: Hold, sliderPart: test.part})
			if subset.failed != test.wantFail || (notified == 1) != test.wantFail {
				t.Fatalf("failure = (%t, %d), want %t", subset.failed, notified, test.wantFail)
			}
			if received.HitResult&^Additions != test.result {
				t.Fatalf("judgment changed to %v, want %v", received.HitResult, test.result)
			}
			if subset.hp.GetHealth() <= 0 {
				t.Fatal("fail mod zeroed normal judgment health")
			}
			if test.result == Hit100 && subset.score.Count100 != 1 {
				t.Fatal("100 was replaced with a miss in score statistics")
			}
		})
	}
}

func TestLazerPerfectFailureHonorsReplayGateAndCatchUp(t *testing.T) {
	for _, catchUp := range []bool{false, true} {
		set, player, subset := newFailModTestRuleset(t, difficulty.Perfect)
		player.cursor.IsReplay = true
		set.SetCatchUp(catchUp)
		set.SendResult(player.cursor, JudgementResult{HitResult: Hit100, MaxResult: Hit300, ComboResult: Increase})
		if subset.failed || subset.failureRecorded {
			t.Fatalf("replay failed during catch-up=%t before completion", catchUp)
		}
		set.PlayerStopped(player.cursor, 1000)
		set.SendResult(player.cursor, JudgementResult{HitResult: Hit300, MaxResult: Hit300, ComboResult: Increase})
		if subset.failed == catchUp {
			t.Fatalf("completed replay failed = %t, catch-up=%t", subset.failed, catchUp)
		}
	}
}

func TestStableRelaxAndAutopilotSuppressFailureWhileInputRemains(t *testing.T) {
	for _, mod := range []difficulty.Modifier{difficulty.Relax, difficulty.Autopilot} {
		set, player, subset := newFailurePolicyTestRuleset(t)
		player.diff.SetGameplayMode(difficulty.GameplayStable)
		player.diff.SetMods(mod)
		set.failInternal(player)
		if subset.failed {
			t.Fatalf("Stable %s failed while input remained", mod.String())
		}
		set.PlayerStopped(player.cursor, 1000)
		set.failInternal(player)
		if !subset.failed {
			t.Fatalf("Stable %s did not permit failure after input ended", mod.String())
		}
	}
}
