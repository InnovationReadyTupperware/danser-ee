package osu

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/graphics"
)

func newFailurePolicyTestRuleset() (*OsuRuleSet, *difficultyPlayer, *subSet) {
	cursor := &graphics.Cursor{}
	player := &difficultyPlayer{
		cursor: cursor,
		diff:   difficulty.NewDifficulty(5, 5, 5, 5),
	}
	subset := &subSet{player: player}
	ruleset := &OsuRuleSet{
		cursors: map[*graphics.Cursor]*subSet{
			cursor: subset,
		},
	}

	return ruleset, player, subset
}

func TestFailurePolicyAllowNotifiesNormalFailure(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset()
	notified := 0

	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 1 {
		t.Fatalf("normal failure notifications = %d, want 1", notified)
	}
	if !subSet.failed {
		t.Fatal("normal failure did not mark the participant failed")
	}
}

func TestFailurePolicySuppressKeepsParticipantActive(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset()
	notified := 0

	ruleset.SetFailurePolicy(player.cursor, FailurePolicySuppress)
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 0 {
		t.Fatalf("suppressed failure notifications = %d, want 0", notified)
	}
	if subSet.failed {
		t.Fatal("suppressed failure marked the participant failed")
	}
}

func TestFailurePolicyDelegateHonorsHandlerDecision(t *testing.T) {
	tests := []struct {
		name       string
		accepted   bool
		wantFailed bool
	}{
		{name: "accepted", accepted: true, wantFailed: true},
		{name: "rejected", accepted: false, wantFailed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleset, player, subSet := newFailurePolicyTestRuleset()
			normalNotified := 0
			delegated := 0

			ruleset.SetFailurePolicy(player.cursor, FailurePolicyDelegate)
			ruleset.SetFailListener(func(cursor *graphics.Cursor) {
				normalNotified++
			})
			ruleset.SetPlayerFailHandler(func(cursor *graphics.Cursor) bool {
				delegated++
				return tt.accepted
			})
			ruleset.failInternal(player)

			if delegated != 1 {
				t.Fatalf("delegated failure calls = %d, want 1", delegated)
			}
			if normalNotified != 0 {
				t.Fatalf("normal failure notifications = %d, want 0", normalNotified)
			}
			if subSet.failed != tt.wantFailed {
				t.Fatalf("participant failed = %t, want %t", subSet.failed, tt.wantFailed)
			}
		})
	}
}

func TestFailurePolicyRetainsDanserRelaxSuppression(t *testing.T) {
	tests := []struct {
		name string
		mod  difficulty.Modifier
	}{
		{name: "no fail", mod: difficulty.NoFail},
		{name: "relax", mod: difficulty.Relax},
		{name: "autopilot", mod: difficulty.Relax2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleset, player, subSet := newFailurePolicyTestRuleset()
			notified := 0
			player.diff.SetMods(tt.mod)

			ruleset.SetFailListener(func(cursor *graphics.Cursor) {
				notified++
			})
			ruleset.failInternal(player)

			if notified != 0 {
				t.Fatalf("failure notifications = %d, want 0", notified)
			}
			if subSet.failed {
				t.Fatal("Danser's current mod suppression marked the participant failed")
			}
		})
	}
}
