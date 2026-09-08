package performance

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestCurrentModelIsJuly2026Calculator(t *testing.T) {
	model := modelForVersion("260706")
	if model.id != "260706" || !model.hasReading {
		t.Fatalf("260706 model = %#v, want current model with Reading", model)
	}
	if got := model.newDifficulty().GetVersion(); got != 20260706 {
		t.Fatalf("260706 difficulty version = %d, want 20260706", got)
	}
}

func TestUnknownModelFallsBackToCurrent(t *testing.T) {
	model := modelForVersion("unknown")
	if model.id != currentModel.id {
		t.Fatalf("unknown model id = %q, want %q", model.id, currentModel.id)
	}
}

func TestRemovedDevSnapshotAliasResolvesToCurrentModel(t *testing.T) {
	version := settings.CanonicalPPVersion("260321")
	model := modelForVersion(version)
	if model.id != "260706" || model.newDifficulty().GetVersion() != 20260706 {
		t.Fatalf("260321 migration resolved to model %#v, want 260706", model)
	}
}

func TestCurrentCatalogCalculatorIgnoresHistoricalSelection(t *testing.T) {
	previousVersion := settings.Gameplay.PPVersion
	settings.Gameplay.PPVersion = "251020"
	t.Cleanup(func() {
		settings.Gameplay.PPVersion = previousVersion
	})

	if got := CreateCurrentDifficultyCalculator().GetVersion(); got != 20260706 {
		t.Fatalf("current catalog calculator version = %d, want 20260706", got)
	}
}

func TestSelectedModelCacheFollowsConfiguredVersion(t *testing.T) {
	previousVersion := settings.Gameplay.PPVersion
	previousSelectedVersion := selectedVersion
	previousSelectedModel := selectedModel
	previousDiffCalc := diffCalc
	t.Cleanup(func() {
		settings.Gameplay.PPVersion = previousVersion
		selectedVersion = previousSelectedVersion
		selectedModel = previousSelectedModel
		diffCalc = previousDiffCalc
	})

	settings.Gameplay.PPVersion = "251020"
	selectedVersion = ""
	selectedModel = model{}
	diffCalc = nil
	if got := GetDifficultyCalculator().GetVersion(); got != 20251020 {
		t.Fatalf("historical selected difficulty version = %d, want 20251020", got)
	}

	settings.Gameplay.PPVersion = "260706"
	if got := GetDifficultyCalculator().GetVersion(); got != 20260706 {
		t.Fatalf("refreshed selected difficulty version = %d, want 20260706", got)
	}
}
