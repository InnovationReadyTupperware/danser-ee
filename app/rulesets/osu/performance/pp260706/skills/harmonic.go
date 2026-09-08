package skills

import (
	"math"
	"slices"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
)

type Harmonic struct {
	HarmonicScale float64
	DecayExponent float64

	DifficultyOf                  func(*preprocessing.DifficultyObject) float64
	ApplyDifficultyTransformation func([]float64) []float64

	ObjectDifficulties []float64
	ObjectWeightSum    float64
}

func NewHarmonic() *Harmonic {
	return &Harmonic{HarmonicScale: 1, DecayExponent: 0.9}
}

func (skill *Harmonic) Process(current *preprocessing.DifficultyObject) float64 {
	value := skill.DifficultyOf(current)
	skill.ObjectDifficulties = append(skill.ObjectDifficulties, value)
	return value
}

func (skill *Harmonic) DifficultyValue() float64 {
	skill.ObjectWeightSum = 0
	if len(skill.ObjectDifficulties) == 0 {
		return 0
	}

	difficulties := slices.Clone(skill.ObjectDifficulties)
	if skill.ApplyDifficultyTransformation != nil {
		difficulties = skill.ApplyDifficultyTransformation(difficulties)
	}
	slices.Sort(difficulties)

	var difficultyValue float64
	index := 0
	for i := len(difficulties) - 1; i >= 0; i-- {
		value := difficulties[i]
		if value <= 0 {
			continue
		}
		fi := float64(index)
		weight := (1 + skill.HarmonicScale/(1+fi)) / (math.Pow(fi, skill.DecayExponent) + 1 + skill.HarmonicScale/(1+fi))
		skill.ObjectWeightSum += weight
		difficultyValue += value * weight
		index++
	}
	return difficultyValue
}

func (skill *Harmonic) CountTopWeightedObjectDifficulties(difficultyValue float64) float64 {
	if len(skill.ObjectDifficulties) == 0 || skill.ObjectWeightSum == 0 {
		return 0
	}
	consistentTop := difficultyValue / skill.ObjectWeightSum
	if consistentTop == 0 {
		return 0
	}
	var count float64
	for _, difficulty := range skill.ObjectDifficulties {
		count += putils.Logistic(difficulty/consistentTop, 0.88, 10, 1.1)
	}
	return count
}

func HarmonicDifficultyToPerformance(difficulty float64) float64 {
	return 4 * putils.PowInt(difficulty, 3)
}
