package objects

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestAutomaticSliderEdgesUseTheSharedPresentationOwner(t *testing.T) {
	previousPlay := settings.PLAY
	previousKnockout := settings.KNOCKOUT
	previousPlayers := settings.PLAYERS
	t.Cleanup(func() {
		settings.PLAY = previousPlay
		settings.KNOCKOUT = previousKnockout
		settings.PLAYERS = previousPlayers
	})

	tests := []struct {
		name     string
		play     bool
		knockout bool
		players  int
		wantAuto bool
	}{
		{name: "visual cursor dance", players: 1, wantAuto: true},
		{name: "one-player gameplay", play: true, players: 1},
		{name: "one-player solo knockout", knockout: true, players: 1},
		{name: "multi-participant knockout", knockout: true, players: 2, wantAuto: true},
		{name: "multi-participant visual playback", players: 2, wantAuto: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings.PLAY = test.play
			settings.KNOCKOUT = test.knockout
			settings.PLAYERS = test.players

			if got := automaticSliderEdgesEnabled(); got != test.wantAuto {
				t.Fatalf("automaticSliderEdgesEnabled() = %t, want %t", got, test.wantAuto)
			}
		})
	}
}
