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

func TestDefaultPPVersionIsOctober2025Model(t *testing.T) {
	if got := NewConfigFile().Gameplay.PPVersion; got != "251020" {
		t.Fatalf("PPVersion default = %q, want 251020", got)
	}
}

func TestCanonicalPPVersionMapsLegacyAliases(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "latest", want: "251020"},
		{in: "26xxxx", want: "260321"},
	}

	for _, test := range tests {
		if got := CanonicalPPVersion(test.in); got != test.want {
			t.Errorf("CanonicalPPVersion(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestMigratePPVersionMapsLegacyAliases(t *testing.T) {
	tests := []struct {
		legacy    string
		canonical string
	}{
		{legacy: "latest", canonical: "251020"},
		{legacy: "26xxxx", canonical: "260321"},
	}

	for _, test := range tests {
		t.Run(test.legacy, func(t *testing.T) {
			config := NewConfigFile()
			config.Gameplay.PPVersion = test.legacy
			config.migratePPVersion()

			if got := config.Gameplay.PPVersion; got != test.canonical {
				t.Fatalf("migrated PPVersion = %q, want %q", got, test.canonical)
			}
		})
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
