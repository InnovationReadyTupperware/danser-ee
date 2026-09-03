package skills

import (
	"math"
	"slices"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260321/evaluators"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260321/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	aimSkillMultiplierSnap    = 71
	aimSkillMultiplierAgility = 2.5
	aimSkillMultiplierFlow    = 245.0
	aimSkillMultiplierTotal   = 1.1
	aimMeanExponent           = 1.2

	aimStrainDecayBase       = 0.15
	aimReducedSectionCount   = 10
	aimReducedStrainBaseline = 0.75
)

type AimSkill struct {
	*Skill
	withSliders   bool
	currentStrain float64

	diffSliders *putils.LogisticSum
	topSliders  *putils.LogisticSum

	peakWeights []float64
}

func NewAimSkill(d *difficulty.Difficulty, withSliders, stepCalc bool) *AimSkill {
	skill := &AimSkill{
		Skill:       NewSkill(d, stepCalc),
		withSliders: withSliders,
	}

	skill.StrainValueOf = skill.aimStrainValue
	skill.PostProcess = skill.postProcess
	skill.CalculateInitialStrain = skill.aimInitialStrain
	skill.CalculateDifficulty = skill.aimDifficulty

	skill.diffSliders = putils.NewLogisticSum(stepCalc, 6, 1, 1, func(previous, current float64) bool {
		return current > previous
	}, func(strains []float64) float64 {
		return slices.Max(strains) / 12
	})

	skill.topSliders = putils.NewLogisticSum(stepCalc, 0.88, 10, 1.1, func(previous, current float64) bool {
		return current != previous
	}, func(strains []float64) float64 {
		return skill.DifficultyValue() * (1 - skill.DecayWeight)
	})

	return skill
}

func (skill *AimSkill) strainDecay(ms float64) float64 {
	return math.Pow(aimStrainDecayBase, ms/1000)
}

func (skill *AimSkill) aimInitialStrain(time float64, current *preprocessing.DifficultyObject) float64 {
	return skill.currentStrain * skill.strainDecay(time-current.Previous(0).StartTime)
}

func (skill *AimSkill) aimStrainValue(current *preprocessing.DifficultyObject) float64 {
	decay := skill.strainDecay(current.AdjustedDeltaTime)

	snapDifficulty := evaluators.EvaluateSnapAim(current, skill.withSliders) * aimSkillMultiplierSnap
	agilityDifficulty := evaluators.EvaluateAgility(current) * aimSkillMultiplierAgility
	flowDifficulty := evaluators.EvaluateFlowAim(current, skill.withSliders) * aimSkillMultiplierFlow

	if skill.diff.CheckModActive(difficulty.TouchDevice) {
		snapDifficulty = math.Pow(snapDifficulty, 0.89)
		// we don't adjust agility here since agility represents TD difficulty in a decent enough way
		flowDifficulty = math.Pow(flowDifficulty, 1.1)
	}

	if skill.diff.CheckModActive(difficulty.Relax) {
		agilityDifficulty *= 0.3
	}

	totalDifficulty := calculateAimTotalValue(snapDifficulty, agilityDifficulty, flowDifficulty)

	skill.currentStrain *= decay
	skill.currentStrain += totalDifficulty * (1 - decay)

	if current.IsSlider {
		skill.diffSliders.AddStrain(skill.currentStrain)
		skill.topSliders.AddStrain(skill.currentStrain)
	}

	return skill.currentStrain
}

func calculateAimTotalValue(snapDifficulty, agilityDifficulty, flowDifficulty float64) float64 {
	// We compare flow to combined snap and agility because snap by itself doesn't have enough difficulty to be above flow on streams
	// Agility on the other hand is supposed to measure the rate of cursor velocity changes while snapping
	// So snapping every circle on a stream requires an enormous amount of agility at which point it's easier to flow
	combinedSnapDifficulty := putils.Norm(aimMeanExponent, snapDifficulty, agilityDifficulty)

	pSnap := calculateSnapFlowProbability(flowDifficulty / combinedSnapDifficulty)
	pFlow := 1 - pSnap

	totalDifficulty := combinedSnapDifficulty*pSnap + flowDifficulty*pFlow

	totalStrain := totalDifficulty * aimSkillMultiplierTotal

	return totalStrain
}

// A function that turns the ratio of snap : flow into the probability of snapping/flowing
// It has the constraints:
// P(snap) + P(flow) = 1 (the object is always either snapped or flowed)
// P(snap) = f(snap/flow), P(flow) = f(flow/snap) (ie snap and flow are symmetric and reversible)
// Therefore: f(x) + f(1/x) = 1
// 0 <= f(x) <= 1 (cannot have negative or greater than 100% probability of snapping or flowing)
// This logistic function is a solution, which fits nicely with the general idea of interpolation and provides a tuneable constant
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

func (skill *AimSkill) postProcess(current *preprocessing.DifficultyObject, strain float64, diffValue float64) {
	if current.IsSlider {
		skill.diffSliders.ProcessLastStrain(strain / 12)
	}

	skill.topSliders.ProcessLastStrain(diffValue * (1 - skill.DecayWeight))
}

func (skill *AimSkill) aimDifficulty() float64 {
	if skill.peakWeights == nil { //Precalculated peak weights
		skill.peakWeights = make([]float64, aimReducedSectionCount)
		for i := range aimReducedSectionCount {
			scale := math.Log10(mutils.Lerp(1.0, 10.0, mutils.Clamp(float64(i)/float64(aimReducedSectionCount), 0, 1)))
			skill.peakWeights[i] = mutils.Lerp(aimReducedStrainBaseline, 1.0, scale)
		}
	}

	diffValue := 0.0
	weight := 1.0

	strains := skill.getCurrentStrainPeaksSorted()

	lowest := strains[len(strains)-1]

	sectionsReduced := min(len(strains), aimReducedSectionCount)

	for i := range sectionsReduced {
		strains[len(strains)-1-i] *= skill.peakWeights[i]
		lowest = min(lowest, strains[len(strains)-1-i])
	}

	// Search for lowest strain that's higher or equal than lowest reduced strain to avoid unnecessary sorting
	idx, _ := slices.BinarySearch(strains[:len(strains)-sectionsReduced], lowest)
	slices.Sort(strains[idx:])

	lastDiff := -math.MaxFloat64

	for i := range len(strains) {
		diffValue += strains[len(strains)-1-i] * weight
		weight *= skill.DecayWeight

		if math.Abs(diffValue-lastDiff) < math.SmallestNonzeroFloat64 { // escape when strain * weight calculates to 0
			break
		}

		lastDiff = diffValue
	}

	return diffValue
}

func (skill *AimSkill) GetDifficultSliders() float64 {
	return skill.diffSliders.GetValue()
}

func (skill *AimSkill) CountTopWeightedSliders() float64 {
	return skill.topSliders.GetValue()
}
