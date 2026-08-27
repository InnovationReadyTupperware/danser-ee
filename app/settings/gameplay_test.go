package settings

import "testing"

func TestDefaultGameplayHidesPositionalMisses(t *testing.T) {
	if Gameplay.HitErrorMeter.ShowPositionalMisses {
		t.Fatal("ShowPositionalMisses default = true, want false")
	}
}
