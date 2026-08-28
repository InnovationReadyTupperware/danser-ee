package settings

import (
	"encoding/json"
	"testing"
)

func TestDefaultGameplayHidesPositionalMisses(t *testing.T) {
	if Gameplay.HitErrorMeter.ShowPositionalMisses {
		t.Fatal("ShowPositionalMisses default = true, want false")
	}
}

func TestDefaultHitErrorMeterUsesLegacyBarDefaults(t *testing.T) {
	meter := NewConfigFile().Gameplay.HitErrorMeter

	if !meter.ShowColorBar {
		t.Fatal("ShowColorBar default = false, want true")
	}
	if !meter.ShowMovingAverage {
		t.Fatal("ShowMovingAverage default = false, want true")
	}
	if meter.JudgmentLineThickness != 3 {
		t.Fatalf("JudgmentLineThickness default = %g, want 3", meter.JudgmentLineThickness)
	}
	if meter.JudgmentLineFadeOutTime != 10 {
		t.Fatalf("JudgmentLineFadeOutTime default = %g, want 10", meter.JudgmentLineFadeOutTime)
	}
	if meter.ScaleTimingWithSpeed {
		t.Fatal("ScaleTimingWithSpeed default = true, want false")
	}
}

func TestHitErrorMeterSettingsAcceptLegacyJSONNames(t *testing.T) {
	config := NewConfigFile()
	data := []byte(`{"Gameplay":{"HitErrorMeter":{"PointFadeOutTime":4.5,"ScaleWithSpeed":true}}}`)

	if err := json.Unmarshal(data, config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	meter := config.Gameplay.HitErrorMeter
	if meter.JudgmentLineFadeOutTime != 4.5 {
		t.Fatalf("JudgmentLineFadeOutTime = %g, want 4.5", meter.JudgmentLineFadeOutTime)
	}
	if !meter.ScaleTimingWithSpeed {
		t.Fatal("ScaleTimingWithSpeed = false, want true")
	}
}
