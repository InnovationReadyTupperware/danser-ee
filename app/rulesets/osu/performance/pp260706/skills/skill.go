package skills

import (
	"math"
	"sort"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
)

type StrainPeak struct {
	Value         float64
	SectionLength float64
}

type queuedStrain struct {
	value     float64
	startTime float64
}

type VariableLengthSkill struct {
	DecayWeight      float64
	MaxSectionLength float64

	StrainValueAt          func(*preprocessing.DifficultyObject) float64
	CalculateInitialStrain func(float64, *preprocessing.DifficultyObject) float64

	currentSectionPeak  float64
	currentSectionBegin float64
	currentSectionEnd   float64
	maxStoredLength     float64
	totalLength         float64

	strainPeaks   []StrainPeak
	queuedStrains []queuedStrain

	ObjectDifficulties []float64
}

func NewVariableLengthSkill() *VariableLengthSkill {
	const decayWeight = 0.9
	return &VariableLengthSkill{
		DecayWeight:      decayWeight,
		MaxSectionLength: 400,
		maxStoredLength:  11 / (1 - decayWeight),
	}
}

func (skill *VariableLengthSkill) Process(current *preprocessing.DifficultyObject) float64 {
	if current.Index == 0 {
		skill.currentSectionBegin = current.StartTime
		skill.currentSectionEnd = skill.currentSectionBegin + skill.MaxSectionLength
		skill.currentSectionPeak = skill.StrainValueAt(current)
		skill.ObjectDifficulties = append(skill.ObjectDifficulties, skill.currentSectionPeak)
		return skill.currentSectionPeak
	}

	skill.backfillPeaks(current)
	currentStrain := skill.StrainValueAt(current)
	skill.ObjectDifficulties = append(skill.ObjectDifficulties, currentStrain)

	if currentStrain > skill.currentSectionPeak {
		skill.queuedStrains = skill.queuedStrains[:0]
		skill.saveCurrentPeak(current.StartTime - skill.currentSectionBegin)
		skill.currentSectionBegin = current.StartTime
		skill.currentSectionEnd = skill.currentSectionBegin + skill.MaxSectionLength
		skill.currentSectionPeak = currentStrain
	} else {
		for len(skill.queuedStrains) > 0 && skill.queuedStrains[len(skill.queuedStrains)-1].value < currentStrain {
			skill.queuedStrains = skill.queuedStrains[:len(skill.queuedStrains)-1]
		}
		skill.queuedStrains = append(skill.queuedStrains, queuedStrain{value: currentStrain, startTime: current.StartTime})
	}

	return currentStrain
}

func (skill *VariableLengthSkill) backfillPeaks(current *preprocessing.DifficultyObject) {
	for current.StartTime > skill.currentSectionEnd {
		skill.saveCurrentPeak(skill.currentSectionEnd - skill.currentSectionBegin)
		skill.currentSectionBegin = skill.currentSectionEnd

		if len(skill.queuedStrains) > 0 {
			queued := skill.queuedStrains[0]
			skill.queuedStrains = skill.queuedStrains[1:]
			skill.currentSectionEnd = queued.startTime + skill.MaxSectionLength
			skill.currentSectionPeak = skill.CalculateInitialStrain(skill.currentSectionBegin, current)
			skill.currentSectionPeak = max(skill.currentSectionPeak, queued.value)
		} else {
			skill.currentSectionEnd = skill.currentSectionBegin + skill.MaxSectionLength
			skill.currentSectionPeak = skill.CalculateInitialStrain(skill.currentSectionBegin, current)
		}
	}
}

func (skill *VariableLengthSkill) saveCurrentPeak(sectionLength float64) {
	peak := StrainPeak{Value: skill.currentSectionPeak, SectionLength: math.Round(sectionLength)}
	index := sort.Search(len(skill.strainPeaks), func(i int) bool { return skill.strainPeaks[i].Value <= peak.Value })
	skill.strainPeaks = append(skill.strainPeaks, StrainPeak{})
	copy(skill.strainPeaks[index+1:], skill.strainPeaks[index:])
	skill.strainPeaks[index] = peak
	skill.totalLength += peak.SectionLength

	for skill.totalLength > skill.maxStoredLength*skill.MaxSectionLength && len(skill.strainPeaks) > 0 {
		last := len(skill.strainPeaks) - 1
		skill.totalLength -= skill.strainPeaks[last].SectionLength
		skill.strainPeaks = skill.strainPeaks[:last]
	}
}

func (skill *VariableLengthSkill) GetCurrentStrainPeaks() []StrainPeak {
	peaks := append([]StrainPeak(nil), skill.strainPeaks...)
	finalPeak := StrainPeak{Value: skill.currentSectionPeak, SectionLength: math.Round(skill.currentSectionEnd - skill.currentSectionBegin)}
	index := sort.Search(len(peaks), func(i int) bool { return peaks[i].Value <= finalPeak.Value })
	peaks = append(peaks, StrainPeak{})
	copy(peaks[index+1:], peaks[index:])
	peaks[index] = finalPeak
	return peaks
}

func (skill *VariableLengthSkill) CountTopWeightedStrains(difficultyValue float64) float64 {
	if len(skill.ObjectDifficulties) == 0 {
		return 0
	}
	consistentTopStrain := difficultyValue * (1 - skill.DecayWeight)
	if consistentTopStrain == 0 {
		return float64(len(skill.ObjectDifficulties))
	}
	var count float64
	for _, strain := range skill.ObjectDifficulties {
		count += putils.Logistic(strain/consistentTopStrain, 0.88, 10, 1.1)
	}
	return count
}

type StrainSkill struct {
	SectionLength float64
	DecayWeight   float64

	StrainValueAt          func(*preprocessing.DifficultyObject) float64
	CalculateInitialStrain func(float64, *preprocessing.DifficultyObject) float64

	currentSectionPeak float64
	currentSectionEnd  float64
	strainPeaks        []float64

	ObjectDifficulties []float64
}

func NewStrainSkill() *StrainSkill {
	return &StrainSkill{SectionLength: 400, DecayWeight: 0.9}
}

func (skill *StrainSkill) Process(current *preprocessing.DifficultyObject) float64 {
	if current.Index == 0 {
		skill.currentSectionEnd = math.Ceil(current.StartTime/skill.SectionLength) * skill.SectionLength
	}

	for current.StartTime > skill.currentSectionEnd {
		skill.strainPeaks = append(skill.strainPeaks, skill.currentSectionPeak)
		skill.currentSectionPeak = skill.CalculateInitialStrain(skill.currentSectionEnd, current)
		skill.currentSectionEnd += skill.SectionLength
	}

	strain := skill.StrainValueAt(current)
	skill.ObjectDifficulties = append(skill.ObjectDifficulties, strain)
	skill.currentSectionPeak = max(skill.currentSectionPeak, strain)
	return strain
}

func (skill *StrainSkill) GetCurrentStrainPeaks() []float64 {
	peaks := append([]float64(nil), skill.strainPeaks...)
	return append(peaks, skill.currentSectionPeak)
}
