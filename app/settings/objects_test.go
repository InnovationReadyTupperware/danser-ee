package settings

import (
	"encoding/json"
	"testing"
)

func TestDefaultSliderJudgmentMarkersAreEnabled(t *testing.T) {
	config := NewConfigFile()

	if !config.Objects.Sliders.ShowSliderJudgmentMarkers {
		t.Fatal("ShowSliderJudgmentMarkers default = false, want true")
	}
}

func TestDefaultComboColorNormalizationMatchesLazer(t *testing.T) {
	config := NewConfigFile()

	if got := config.Objects.Colors.ComboColorNormalization; got != 0.2 {
		t.Fatalf("ComboColorNormalization default = %v, want 0.2", got)
	}
}

func TestLegacyConfigKeepsComboColorNormalizationDefault(t *testing.T) {
	config := NewConfigFile()
	data := []byte(`{"Objects":{"Colors":{"UseComboColors":true}}}`)

	if err := json.Unmarshal(data, config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got := config.Objects.Colors.ComboColorNormalization; got != 0.2 {
		t.Fatalf("ComboColorNormalization after legacy config load = %v, want 0.2", got)
	}
}
