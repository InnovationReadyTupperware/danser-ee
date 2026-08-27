package settings

import "testing"

func TestDefaultCursorDanceUsesMaximumSpinnerRate(t *testing.T) {
	config := NewConfigFile()

	if config.CursorDance.SpinnerBehavior == nil {
		t.Fatal("CursorDance.SpinnerBehavior is nil")
	}

	if config.CursorDance.SpinnerBehavior.SpinAtLowestRPM {
		t.Fatal("SpinAtLowestRPM default = true, want false")
	}
}
