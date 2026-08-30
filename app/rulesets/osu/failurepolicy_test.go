package osu

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func newFailurePolicyTestRuleset(t *testing.T) (*OsuRuleSet, *difficultyPlayer, *subSet) {
	t.Helper()

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
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
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
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
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
			ruleset, player, subSet := newFailurePolicyTestRuleset(t)
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

func TestFailurePolicyUsesLazerModFailureOverrides(t *testing.T) {
	tests := []struct {
		name        string
		mod         difficulty.Modifier
		wantNotify  bool
		wantFailure bool
	}{
		{name: "no fail", mod: difficulty.NoFail},
		{name: "relax", mod: difficulty.Relax, wantNotify: true, wantFailure: true},
		{name: "autopilot", mod: difficulty.Autopilot, wantNotify: true, wantFailure: true},
		{name: "autoplay", mod: difficulty.Autoplay, wantNotify: true, wantFailure: true},
		{name: "classic", mod: difficulty.Classic, wantNotify: true, wantFailure: true},
		{name: "cinema", mod: difficulty.Cinema},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleset, player, subSet := newFailurePolicyTestRuleset(t)
			notified := 0
			player.diff.SetMods(tt.mod)

			ruleset.SetFailListener(func(cursor *graphics.Cursor) {
				notified++
			})
			ruleset.failInternal(player)

			if got := notified > 0; got != tt.wantNotify {
				t.Fatalf("failure notified = %t, want %t", got, tt.wantNotify)
			}
			if subSet.failed != tt.wantFailure {
				t.Fatalf("participant failed = %t, want %t", subSet.failed, tt.wantFailure)
			}
		})
	}
}

func TestCanBeHitAutopilotUsesClassicLazerNoteLock(t *testing.T) {
	tests := []struct {
		name        string
		mode        difficulty.GameplayMode
		mods        difficulty.Modifier
		classicLock bool
		wantClick   bool
	}{
		{name: "stable", mode: difficulty.GameplayStable, mods: difficulty.Autopilot},
		{name: "lazer without classic", mode: difficulty.GameplayLazer, mods: difficulty.Autopilot, wantClick: true},
		{name: "lazer classic note lock", mode: difficulty.GameplayLazer, mods: difficulty.Autopilot | difficulty.Classic, classicLock: true},
		{name: "lazer classic note lock disabled", mode: difficulty.GameplayLazer, mods: difficulty.Autopilot | difficulty.Classic, classicLock: false, wantClick: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := &difficultyPlayer{
				cursor: &graphics.Cursor{},
				diff:   difficulty.NewDifficulty(5, 5, 5, 5),
			}
			player.diff.SetGameplayMode(tt.mode)
			player.diff.SetMods(tt.mods)
			player.classicNoteLock = tt.classicLock

			object := &Circle{hitCircle: objects.DummyCircle(vector.NewVec2f(0, 0), 1000)}
			ruleset := &OsuRuleSet{}
			got := ruleset.CanBeHit(1250, object, player) == Click
			if got != tt.wantClick {
				t.Fatalf("CanBeHit at 250ms = %t, want %t", got, tt.wantClick)
			}
		})
	}
}

func TestFailurePolicyNoFailBlocksAfterReplayEnds(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
	player.cursor.IsReplay = true
	player.diff.SetMods(difficulty.NoFail)
	ruleset.PlayerStopped(player.cursor, 0)

	notified := 0
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 0 {
		t.Fatalf("No Fail failure notifications = %d, want 0", notified)
	}
	if subSet.failed {
		t.Fatal("No Fail marked the participant failed after replay completion")
	}
}

func TestFailurePolicyWaitsForNormalReplayToEnd(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
	player.cursor.IsReplay = true

	notified := 0
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 0 || subSet.failed {
		t.Fatal("normal replay failed before its final input frame")
	}

	ruleset.PlayerStopped(player.cursor, 0)
	ruleset.failInternal(player)

	if notified != 1 || !subSet.failed {
		t.Fatalf("completed replay failure = (%d, %t), want (1, true)", notified, subSet.failed)
	}
}

func TestFailurePolicyAutoplayReplayCanFailBeforeInputEnds(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
	player.cursor.IsReplay = true
	player.diff.SetMods(difficulty.Autoplay)

	notified := 0
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 1 || !subSet.failed {
		t.Fatalf("Autoplay replay failure = (%d, %t), want (1, true)", notified, subSet.failed)
	}
}

func TestFailurePolicyIgnoreFailsInReplaysSuppressesCompletedReplay(t *testing.T) {
	previous := settings.Gameplay.IgnoreFailsInReplays
	settings.Gameplay.IgnoreFailsInReplays = true
	t.Cleanup(func() {
		settings.Gameplay.IgnoreFailsInReplays = previous
	})

	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
	player.cursor.IsReplay = true
	ruleset.PlayerStopped(player.cursor, 0)

	notified := 0
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 0 || subSet.failed {
		t.Fatal("IgnoreFailsInReplays allowed a completed replay to fail")
	}
}

func TestFailurePolicyEasyUsesExtraLifeBeforeFailing(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
	player.diff.SetMods(difficulty.Easy)
	subSet.recoveries = 1
	subSet.hp = &HealthProcessor{}

	notified := 0
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 0 || subSet.failed || subSet.recoveries != 0 {
		t.Fatalf("Easy recovery state = (notifications %d, failed %t, recoveries %d), want (0, false, 0)", notified, subSet.failed, subSet.recoveries)
	}

	ruleset.failInternal(player)
	if notified != 1 || !subSet.failed {
		t.Fatalf("post-recovery failure = (%d, %t), want (1, true)", notified, subSet.failed)
	}
}

func TestFailurePolicySuddenDeathFailureBypassesEasyRecovery(t *testing.T) {
	ruleset, player, subSet := newFailurePolicyTestRuleset(t)
	player.diff.SetMods(difficulty.Easy | difficulty.SuddenDeath)
	subSet.recoveries = 1
	subSet.sdpfFail = true

	notified := 0
	ruleset.SetFailListener(func(cursor *graphics.Cursor) {
		notified++
	})
	ruleset.failInternal(player)

	if notified != 1 || !subSet.failed || subSet.recoveries != 1 {
		t.Fatalf("Sudden Death failure state = (notifications %d, failed %t, recoveries %d), want (1, true, 1)", notified, subSet.failed, subSet.recoveries)
	}
}
