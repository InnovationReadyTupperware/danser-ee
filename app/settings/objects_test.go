package settings

import "testing"

func TestDefaultSliderJudgmentMarkersAreEnabled(t *testing.T) {
	config := NewConfigFile()

	if !config.Objects.Sliders.ShowSliderJudgmentMarkers {
		t.Fatal("ShowSliderJudgmentMarkers default = false, want true")
	}
}
