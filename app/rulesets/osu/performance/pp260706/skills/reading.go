package skills

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/evaluators"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

type ReadingSkill struct {
	*Harmonic

	diff             *difficulty.Difficulty
	hasHidden        bool
	currentStrain    float64
	reducedNoteCount float64
	reducedDuration  *float64
}

func NewReadingSkill(d *difficulty.Difficulty, _ ...bool) *ReadingSkill {
	skill := &ReadingSkill{
		Harmonic:  NewHarmonic(),
		diff:      d,
		hasHidden: d.HasHiddenObjectFading(),
	}
	skill.DifficultyOf = skill.readingDifficulty
	skill.ApplyDifficultyTransformation = skill.transformDifficulties
	return skill
}

func (skill *ReadingSkill) strainDecay(ms float64) float64 {
	return math.Pow(0.8, ms/1000)
}

func (skill *ReadingSkill) readingDifficulty(current *preprocessing.DifficultyObject) float64 {
	const (
		skillMultiplier           = 2.5
		reducedDifficultyDuration = 60 * 1000.0
	)

	decay := skill.strainDecay(current.DeltaTime)
	skill.currentStrain *= decay
	skill.currentStrain += skill.calculateAdjustedDifficulty(current) * (1 - decay) * skillMultiplier

	if skill.reducedDuration == nil {
		duration := current.StartTime + reducedDifficultyDuration
		skill.reducedDuration = &duration
	}
	if current.StartTime <= *skill.reducedDuration {
		skill.reducedNoteCount++
	}

	return skill.currentStrain
}

func (skill *ReadingSkill) calculateAdjustedDifficulty(current *preprocessing.DifficultyObject) float64 {
	value := evaluators.EvaluateReading(current, skill.hasHidden)
	if skill.diff.CheckModActive(difficulty.TouchDevice) {
		value = math.Pow(value, 0.89)
	}
	if skill.diff.CheckModActive(difficulty.Relax) {
		value *= 0.4
	}
	if skill.diff.CheckModActive(difficulty.Autopilot) {
		value *= 0.1
	}
	value *= 0.825 + math.Pow(max(0, current.OverallDifficulty), 2.2)/1125
	return value
}

func (skill *ReadingSkill) transformDifficulties(difficulties []float64) []float64 {
	filtered := difficulties[:0]
	for _, value := range difficulties {
		if value > 0 {
			filtered = append(filtered, value)
		}
	}
	difficulties = filtered

	if skill.reducedNoteCount <= 0 {
		return difficulties
	}
	limit := min(len(difficulties), int(skill.reducedNoteCount))
	for i := 0; i < limit; i++ {
		scale := math.Log10(mutils.Lerp(1.0, 10.0, mutils.Clamp(float64(i)/skill.reducedNoteCount, 0, 1)))
		difficulties[i] *= scale
	}
	return difficulties
}

func (skill *ReadingSkill) CountTopWeightedObjectDifficulties(difficultyValue float64) float64 {
	if len(skill.ObjectDifficulties) == 0 || skill.ObjectWeightSum == 0 {
		return 0
	}
	consistentTop := difficultyValue / skill.ObjectWeightSum
	if consistentTop == 0 {
		return 0
	}
	var count float64
	for _, value := range skill.ObjectDifficulties {
		count += putils.Logistic(value/consistentTop, 1.15, 5, 1.1)
	}
	return count
}
