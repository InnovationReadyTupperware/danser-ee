package objects

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestAutomaticObjectPresentationUsesTheSharedOwner(t *testing.T) {
	previousPlay := settings.PLAY
	previousKnockout := settings.KNOCKOUT
	previousSoloKnockout := settings.SOLOKNOCKOUT
	previousPlayers := settings.PLAYERS
	t.Cleanup(func() {
		settings.PLAY = previousPlay
		settings.KNOCKOUT = previousKnockout
		settings.SOLOKNOCKOUT = previousSoloKnockout
		settings.PLAYERS = previousPlayers
	})

	tests := []struct {
		name         string
		play         bool
		knockout     bool
		soloKnockout bool
		players      int
		want         bool
	}{
		{name: "visual playback", players: 1, want: true},
		{name: "one-player gameplay", play: true, players: 1},
		{name: "one-player replay", knockout: true, players: 1},
		{name: "one-player solo knockout", knockout: true, soloKnockout: true, players: 1},
		{name: "multi-participant visual playback", players: 2, want: true},
		{name: "multi-participant replay", knockout: true, players: 2, want: true},
		{name: "multi-participant solo knockout", knockout: true, soloKnockout: true, players: 2, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings.PLAY = test.play
			settings.KNOCKOUT = test.knockout
			settings.SOLOKNOCKOUT = test.soloKnockout
			settings.PLAYERS = test.players

			if got := automaticObjectPresentationEnabled(); got != test.want {
				t.Fatalf("automaticObjectPresentationEnabled() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSpinnerSampleStateChangesOnlyOnEdges(t *testing.T) {
	state := spinnerSampleState{}

	tests := []struct {
		name        string
		active      bool
		wantChanged bool
		wantState   bool
	}{
		{name: "idle remains idle", active: false, wantState: false},
		{name: "starts", active: true, wantChanged: true, wantState: true},
		{name: "active remains active", active: true, wantState: true},
		{name: "pauses", active: false, wantChanged: true, wantState: false},
		{name: "paused remains paused", active: false, wantState: false},
		{name: "starts again", active: true, wantChanged: true, wantState: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := state.SetActive(test.active); got != test.wantChanged {
				t.Fatalf("SetActive(%t) changed = %t, want %t", test.active, got, test.wantChanged)
			}
			if state.active != test.wantState {
				t.Fatalf("sample active state = %t, want %t", state.active, test.wantState)
			}
		})
	}

	state.Reset()
	if state.active {
		t.Fatal("Reset left spinner sample active")
	}
}
