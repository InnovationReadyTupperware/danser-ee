package skills

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260321/evaluators"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260321/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	readingSkillMultiplier float64 = 2.5
	readingStrainDecayBase float64 = 0.8
)

type ReadingSkill struct {
	*Harmonic

	objects []*preprocessing.DifficultyObject

	currentDiff float64
}

func NewReadingSkill(d *difficulty.Difficulty, stepCalc bool) *ReadingSkill {
	skill := &ReadingSkill{
		Harmonic: NewHarmonic(d, stepCalc),
	}

	skill.DifficultyOf = skill.readingDifficulty
	skill.ApplyDifficultyTransformation = skill.applyDifficultyTransformation

	skill.diffStrains = putils.NewLogisticSum(stepCalc,
		1.15,
		5,
		1.1,
		func(previous, current float64) bool {
			return previous != current
		},
		func(strains []float64) float64 {
			if skill.NoteWeightSum == 0 {
				return 0
			}

			return skill.DifficultyValue() / skill.NoteWeightSum
		},
	)

	return skill
}

func (s *ReadingSkill) applyDifficultyTransformation(difficulties []float64) {
	const reducedDifficultyBaseLine = 0.0 // Assume the first seconds are completely memorised

	reducedNoteCount := s.calculateReducedNoteCount()

	for i := 0; i < min(len(difficulties), reducedNoteCount); i++ {
		scale := math.Log10(mutils.Lerp(1.0, 10.0, mutils.Clamp(float64(i)/float64(reducedNoteCount), 0.0, 1.0)))
		difficulties[i] *= mutils.Lerp(reducedDifficultyBaseLine, 1.0, scale)
	}
}

func (s *ReadingSkill) calculateReducedNoteCount() int {
	const reducedDifficultyDuration = 60 * 1000

	if len(s.objects) == 0 {
		return 0
	}

	reducedDuration := s.objects[0].StartTime + reducedDifficultyDuration

	reducedNoteCount := 0

	for _, hitObject := range s.objects {
		if hitObject.StartTime > reducedDuration {
			break
		}

		reducedNoteCount++
	}

	return reducedNoteCount
}

func (s *ReadingSkill) strainDecay(ms float64) float64 {
	return math.Pow(readingStrainDecayBase, ms/1000)
}

func (s *ReadingSkill) readingDifficulty(current *preprocessing.DifficultyObject) float64 {
	s.objects = append(s.objects, current)

	s.currentDiff *= s.strainDecay(current.AdjustedDeltaTime)
	s.currentDiff += evaluators.EvaluateReading(current, s.diff.CheckModActive(difficulty.Hidden)) * readingSkillMultiplier

	return s.currentDiff
}
