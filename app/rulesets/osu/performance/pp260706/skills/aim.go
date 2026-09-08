package skills

import (
	"math"
	"slices"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/evaluators"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

type AimSkill struct {
	*VariableLengthSkill

	diff          *difficulty.Difficulty
	withSliders   bool
	currentStrain float64
	sliderStrains []float64
}

func NewAimSkill(d *difficulty.Difficulty, withSliders bool, _ ...bool) *AimSkill {
	skill := &AimSkill{
		VariableLengthSkill: NewVariableLengthSkill(),
		diff:                d,
		withSliders:         withSliders,
	}
	skill.StrainValueAt = skill.strainValueAt
	skill.CalculateInitialStrain = skill.calculateInitialStrain
	return skill
}

func (skill *AimSkill) strainDecay(ms float64) float64 {
	return math.Pow(0.2, ms/1000)
}

func (skill *AimSkill) calculateInitialStrain(time float64, current *preprocessing.DifficultyObject) float64 {
	previous := current.Previous(0)
	if previous == nil {
		return 0
	}
	return skill.currentStrain * skill.strainDecay(time-previous.StartTime)
}

func (skill *AimSkill) strainValueAt(current *preprocessing.DifficultyObject) float64 {
	if skill.diff.CheckModActive(difficulty.Autopilot) {
		return 0
	}

	decay := skill.strainDecay(current.AdjustedDeltaTime)
	skill.currentStrain *= decay
	skill.currentStrain += skill.calculateAdjustedDifficulty(current) * (1 - decay)

	if current.IsSlider {
		skill.sliderStrains = append(skill.sliderStrains, skill.currentStrain)
	}

	return skill.currentStrain
}

func (skill *AimSkill) calculateAdjustedDifficulty(current *preprocessing.DifficultyObject) float64 {
	const (
		snapMultiplier    = 70.9
		agilityMultiplier = 2.35
		flowMultiplier    = 242.0
	)

	snapDifficulty := evaluators.EvaluateSnapAim(current, skill.withSliders) * snapMultiplier
	agilityDifficulty := evaluators.EvaluateAgility(current) * agilityMultiplier
	flowDifficulty := evaluators.EvaluateFlowAim(current, skill.withSliders) * flowMultiplier

	totalDifficulty := skill.calculateTotalValue(snapDifficulty, agilityDifficulty, flowDifficulty)
	totalDifficulty *= 0.985 + putils.PowInt(max(0, current.OverallDifficulty), 2)/4000
	return totalDifficulty
}

func (skill *AimSkill) calculateTotalValue(snapDifficulty, agilityDifficulty, flowDifficulty float64) float64 {
	const (
		totalMultiplier      = 1.12
		combinedSnapExponent = 1.2
	)

	combinedSnapDifficulty := putils.Norm(combinedSnapExponent, snapDifficulty, agilityDifficulty)
	pSnap := calculateSnapFlowProbability(flowDifficulty / combinedSnapDifficulty)
	pFlow := 1 - pSnap

	if skill.diff.CheckModActive(difficulty.TouchDevice) {
		snapDifficulty = math.Pow(snapDifficulty, 0.89)
		combinedSnapDifficulty = putils.Norm(combinedSnapExponent, snapDifficulty, agilityDifficulty)
	}

	if skill.diff.CheckModActive(difficulty.Relax) {
		combinedSnapDifficulty *= 0.75
		flowDifficulty *= 0.6
	}

	return (combinedSnapDifficulty*pSnap + flowDifficulty*pFlow) * totalMultiplier
}

func calculateSnapFlowProbability(ratio float64) float64 {
	const k = 7.27
	if ratio == 0 {
		return 0
	}
	if math.IsNaN(ratio) {
		return 1
	}
	return putils.LogisticE(-k*math.Log(ratio), 1)
}

func (skill *AimSkill) GetDifficultSliders() float64 {
	if len(skill.sliderStrains) == 0 {
		return 0
	}
	maxSliderStrain := slices.Max(skill.sliderStrains)
	if maxSliderStrain == 0 {
		return 0
	}
	var count float64
	for _, strain := range skill.sliderStrains {
		count += putils.Logistic(strain/maxSliderStrain, 0.5, 12, 1)
	}
	return count
}

func (skill *AimSkill) CountTopWeightedSliders(difficultyValue float64) float64 {
	if len(skill.sliderStrains) == 0 {
		return 0
	}
	consistentTopStrain := difficultyValue * (1 - skill.DecayWeight)
	if consistentTopStrain == 0 {
		return 0
	}
	var count float64
	for _, strain := range skill.sliderStrains {
		count += putils.Logistic(strain/consistentTopStrain, 0.88, 10, 1.1)
	}
	return count
}

func (skill *AimSkill) DifficultyValue() float64 {
	var difficultyValue float64
	var time float64
	for _, strain := range skill.getReducedStrainPeaks() {
		startTime := time
		endTime := time + strain.SectionLength/skill.MaxSectionLength
		weight := math.Pow(skill.DecayWeight, startTime) - math.Pow(skill.DecayWeight, endTime)
		difficultyValue += strain.Value * weight
		time = endTime
	}
	return difficultyValue / (1 - skill.DecayWeight)
}

func (skill *AimSkill) getReducedStrainPeaks() []StrainPeak {
	const (
		reducedSectionTime    = 4000.0
		reducedStrainBaseline = 0.727
		chunkSize             = 20.0
	)

	allPeaks := skill.GetCurrentStrainPeaks()
	strains := make([]StrainPeak, 0, len(allPeaks))
	for _, peak := range allPeaks {
		if peak.Value > 0 {
			strains = append(strains, peak)
		}
	}

	var time float64
	skipCount := 0
	for len(strains) > skipCount && time < reducedSectionTime {
		strain := strains[skipCount]
		for addedTime := 0.0; addedTime < strain.SectionLength; addedTime += chunkSize {
			scale := math.Log10(mutils.Lerp(1.0, 10.0, mutils.Clamp((time+addedTime)/reducedSectionTime, 0, 1)))
			strains = append(strains, StrainPeak{
				Value:         strain.Value * mutils.Lerp(reducedStrainBaseline, 1, scale),
				SectionLength: min(chunkSize, strain.SectionLength-addedTime),
			})
		}
		time += strain.SectionLength
		skipCount++
	}

	strains = strains[skipCount:]
	slices.SortFunc(strains, func(a, b StrainPeak) int {
		switch {
		case a.Value > b.Value:
			return -1
		case a.Value < b.Value:
			return 1
		default:
			return 0
		}
	})
	return strains
}
