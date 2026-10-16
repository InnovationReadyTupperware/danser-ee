package overlays

import "testing"

func TestKnockoutActiveCursorCountTracksEliminationsAndRevival(t *testing.T) {
	first := &knockoutPlayer{}
	second := &knockoutPlayer{}
	third := &knockoutPlayer{}
	overlay := &KnockoutOverlay{playersArray: []*knockoutPlayer{first, second, third}}

	if got := overlay.ActiveCursorCount(); got != 3 {
		t.Fatalf("initial active count = %d, want 3", got)
	}

	second.hasBroken = true
	third.hasBroken = true
	if got := overlay.ActiveCursorCount(); got != 1 {
		t.Fatalf("after elimination = %d, want 1", got)
	}

	first.hasBroken = true
	if got := overlay.ActiveCursorCount(); got != 0 {
		t.Fatalf("all eliminated = %d, want 0", got)
	}

	second.hasBroken = false
	third.hasBroken = false
	if got := overlay.ActiveCursorCount(); got != 2 {
		t.Fatalf("after revival = %d, want 2", got)
	}
}
