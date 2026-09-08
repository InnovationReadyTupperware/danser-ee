package performance

import (
	"log"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp211112"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp220930"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp241007"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp250306"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp251020"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

type model struct {
	id             string
	hasReading     bool
	newDifficulty  func() api.IDifficultyCalculator
	newPerformance func() api.IPerformanceCalculator
}

var currentModel = model{
	id:             "260706",
	hasReading:     true,
	newDifficulty:  pp260706.NewDifficultyCalculator,
	newPerformance: pp260706.NewPPCalculator,
}

func modelForVersion(version string) model {
	switch version {
	case "211112":
		return model{id: version, newDifficulty: pp211112.NewDifficultyCalculator, newPerformance: pp211112.NewPPCalculator}
	case "220930":
		return model{id: version, newDifficulty: pp220930.NewDifficultyCalculator, newPerformance: pp220930.NewPPCalculator}
	case "241007":
		return model{id: version, newDifficulty: pp241007.NewDifficultyCalculator, newPerformance: pp241007.NewPPCalculator}
	case "250306":
		return model{id: version, newDifficulty: pp250306.NewDifficultyCalculator, newPerformance: pp250306.NewPPCalculator}
	case "251020":
		return model{id: version, newDifficulty: pp251020.NewDifficultyCalculator, newPerformance: pp251020.NewPPCalculator}
	case "260706":
		return currentModel
	default:
		return currentModel
	}
}

var selectedModel model
var diffCalc api.IDifficultyCalculator
var selectedVersion string

func initSelectedModel() {
	version := settings.CanonicalPPVersion(settings.Gameplay.PPVersion)
	if selectedModel.newDifficulty != nil && selectedVersion == version {
		return
	}

	selectedModel = modelForVersion(version)
	selectedVersion = version
	diffCalc = nil

	if selectedModel.id != version {
		log.Printf("Performance: Warning: unknown PP version %q; using %s", version, currentModel.id)
	}
}

func GetDifficultyCalculator() api.IDifficultyCalculator {
	initSelectedModel()

	if diffCalc == nil {
		diffCalc = selectedModel.newDifficulty()
	}

	return diffCalc
}

func CreatePPCalculator() api.IPerformanceCalculator {
	initSelectedModel()
	return selectedModel.newPerformance()
}

// CreateCurrentDifficultyCalculator returns the canonical current public osu!
// difficulty model used for persisted catalog star ratings. It intentionally
// ignores the user's historical gameplay PP selection.
func CreateCurrentDifficultyCalculator() api.IDifficultyCalculator {
	return currentModel.newDifficulty()
}

// SelectedModelHasReading reports whether the selected gameplay model exposes
// a Reading difficulty/performance component.
func SelectedModelHasReading() bool {
	initSelectedModel()
	return selectedModel.hasReading
}
