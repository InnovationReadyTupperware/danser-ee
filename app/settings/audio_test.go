package settings

import "testing"

func TestDefaultHitsoundStereoSeparationMatchesLazer(t *testing.T) {
	if got := NewConfigFile().Audio.HitsoundStereoSeparation; got != 0.2 {
		t.Fatalf("HitsoundStereoSeparation default = %g, want 0.2", got)
	}
}
