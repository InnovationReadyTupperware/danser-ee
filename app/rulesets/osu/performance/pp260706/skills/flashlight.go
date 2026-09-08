package skills

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/evaluators"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
)

type Flashlight struct {
	*StrainSkill

	diff          *difficulty.Difficulty
	totalObjects  int
	currentStrain float64
}

func NewFlashlightSkill(d *difficulty.Difficulty, totalObjects ...int) *Flashlight {
	count := 0
	if len(totalObjects) > 0 {
		count = totalObjects[0]
	}
	skill := &Flashlight{
		StrainSkill:  NewStrainSkill(),
		diff:         d,
		totalObjects: count,
	}
	skill.StrainValueAt = skill.strainValueAt
	skill.CalculateInitialStrain = skill.calculateInitialStrain
	return skill
}

func (skill *Flashlight) strainDecay(ms float64) float64 {
	return math.Pow(0.15, ms/1000)
}

func (skill *Flashlight) calculateInitialStrain(time float64, current *preprocessing.DifficultyObject) float64 {
	previous := current.Previous(0)
	if previous == nil {
		return 0
	}
	return skill.currentStrain * skill.strainDecay(time-previous.StartTime)
}

func (skill *Flashlight) strainValueAt(current *preprocessing.DifficultyObject) float64 {
	if !skill.diff.CheckModActive(difficulty.Flashlight) {
		return 0
	}
	skill.currentStrain *= skill.strainDecay(current.DeltaTime)
	skill.currentStrain += skill.calculateAdjustedDifficulty(current) * 0.058
	return skill.currentStrain
}

func (skill *Flashlight) calculateAdjustedDifficulty(current *preprocessing.DifficultyObject) float64 {
	value := evaluators.EvaluateFlashlight(current)
	if skill.diff.CheckModActive(difficulty.TouchDevice) {
		value = math.Pow(value, 0.9)
	}
	if skill.diff.CheckModActive(difficulty.Relax) {
		value *= 0.7
	}
	if skill.diff.CheckModActive(difficulty.Autopilot) {
		value *= 0.4
	}
	value *= 0.985 + putils.PowInt(max(0, current.OverallDifficulty), 2)/4000
	return value
}

func (skill *Flashlight) DifficultyValue() float64 {
	var sum float64
	for _, peak := range skill.GetCurrentStrainPeaks() {
		sum += peak
	}
	count := skill.totalObjects
	if count <= 0 {
		count = len(skill.ObjectDifficulties) + 1
	}
	multiplier := 0.7 + 0.1*min(1.0, float64(count)/200)
	if count > 200 {
		multiplier += 0.2 * min(1.0, float64(count-200)/200)
	}
	return sum * multiplier
}

func FlashlightDifficultyToPerformance(difficulty float64) float64 {
	return 25 * putils.PowInt(difficulty, 2)
}
