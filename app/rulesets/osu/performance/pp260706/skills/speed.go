package skills

import (
	"math"
	"slices"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/evaluators"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
)

type SpeedSkill struct {
	*Harmonic

	diff          *difficulty.Difficulty
	currentStrain float64
	sliderStrains []float64
}

func NewSpeedSkill(d *difficulty.Difficulty, _ ...bool) *SpeedSkill {
	skill := &SpeedSkill{Harmonic: NewHarmonic(), diff: d}
	skill.HarmonicScale = 20
	skill.DecayExponent = 0.9
	skill.DifficultyOf = skill.speedDifficulty
	return skill
}

func (skill *SpeedSkill) strainDecay(ms float64) float64 {
	return math.Pow(0.3, ms/1000)
}

func (skill *SpeedSkill) speedDifficulty(current *preprocessing.DifficultyObject) float64 {
	if skill.diff.CheckModActive(difficulty.Relax) {
		return 0
	}

	decay := skill.strainDecay(current.AdjustedDeltaTime)
	skill.currentStrain *= decay

	adjustedDifficulty := evaluators.EvaluateSpeed(current)
	if skill.diff.CheckModActive(difficulty.Autopilot) {
		adjustedDifficulty *= 0.5
	}

	skill.currentStrain += adjustedDifficulty * (1 - decay) * 1.16
	totalStrain := skill.currentStrain * evaluators.EvaluateRhythm(current)
	if current.IsSlider {
		skill.sliderStrains = append(skill.sliderStrains, totalStrain)
	}
	return totalStrain
}

func (skill *SpeedSkill) RelevantNoteCount() float64 {
	if len(skill.ObjectDifficulties) == 0 {
		return 0
	}
	maxStrain := slices.Max(skill.ObjectDifficulties)
	if maxStrain == 0 {
		return 0
	}
	var count float64
	for _, strain := range skill.ObjectDifficulties {
		count += putils.Logistic(strain/maxStrain, 0.5, 12, 1)
	}
	return count
}

func (skill *SpeedSkill) CountTopWeightedSliders(difficultyValue float64) float64 {
	if len(skill.sliderStrains) == 0 || skill.ObjectWeightSum == 0 {
		return 0
	}
	consistentTop := difficultyValue / skill.ObjectWeightSum
	if consistentTop == 0 {
		return 0
	}
	var count float64
	for _, strain := range skill.sliderStrains {
		count += putils.Logistic(strain/consistentTop, 0.88, 10, 1.1)
	}
	return count
}
