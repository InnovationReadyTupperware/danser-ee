package osu

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/graphics"
)

func TestCatchUpJudgmentReachesListenerWithPresentationMarker(t *testing.T) {
	cursor := &graphics.Cursor{}
	ruleset := &OsuRuleSet{
		cursors: map[*graphics.Cursor]*subSet{
			cursor: {
				player: &difficultyPlayer{diff: &difficulty.Difficulty{}},
				score:  &Score{},
			},
		},
	}

	var received JudgementResult
	ruleset.SetListener(func(_ *graphics.Cursor, result JudgementResult, _ Score) {
		received = result
	})

	previous := ruleset.SetCatchUp(true)
	ruleset.SendResult(cursor, JudgementResult{HitResult: PositionalMiss})
	ruleset.SetCatchUp(previous)

	if !received.IsCatchUp() {
		t.Fatal("catch-up judgment reached listener without catch-up marker")
	}
	if ruleset.catchUp {
		t.Fatal("restoring the previous catch-up state left catch-up enabled")
	}
}
