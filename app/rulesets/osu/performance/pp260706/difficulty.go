package pp260706

import (
	"log"
	"math"
	"time"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/skills"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const CurrentVersion = 20260706

const performanceBaseMultiplier = 1.12
const performanceNormExponent = 1.1

type DifficultyCalculator struct{}

func NewDifficultyCalculator() api.IDifficultyCalculator {
	return &DifficultyCalculator{}
}

type skillSet struct {
	aim               *skills.AimSkill
	aimWithoutSliders *skills.AimSkill
	speed             *skills.SpeedSkill
	reading           *skills.ReadingSkill
	flashlight        *skills.Flashlight
}

func newSkillSet(diff *difficulty.Difficulty, objectCount int) *skillSet {
	return &skillSet{
		aim:               skills.NewAimSkill(diff, true),
		aimWithoutSliders: skills.NewAimSkill(diff, false),
		speed:             skills.NewSpeedSkill(diff),
		reading:           skills.NewReadingSkill(diff),
		flashlight:        skills.NewFlashlightSkill(diff, objectCount),
	}
}

func (set *skillSet) process(object *preprocessing.DifficultyObject) {
	set.aim.Process(object)
	set.aimWithoutSliders.Process(object)
	set.speed.Process(object)
	set.reading.Process(object)
	set.flashlight.Process(object)
}

func (calc *DifficultyCalculator) buildAttributes(set *skillSet, sim *scoreSimulator, diff *difficulty.Difficulty) api.Attributes {
	aimDifficultyValue := set.aim.DifficultyValue()
	aimNoSlidersDifficultyValue := set.aimWithoutSliders.DifficultyValue()
	speedDifficultyValue := set.speed.DifficultyValue()
	readingDifficultyValue := set.reading.DifficultyValue()
	flashlightDifficultyValue := set.flashlight.DifficultyValue()

	aimDifficultStrainCount := set.aim.CountTopWeightedStrains(aimDifficultyValue)
	speedDifficultStrainCount := set.speed.CountTopWeightedObjectDifficulties(speedDifficultyValue)
	readingDifficultNoteCount := set.reading.CountTopWeightedObjectDifficulties(readingDifficultyValue)

	aimNoSlidersTopWeightedSliderCount := set.aimWithoutSliders.CountTopWeightedSliders(aimNoSlidersDifficultyValue)
	aimNoSlidersDifficultStrainCount := set.aimWithoutSliders.CountTopWeightedStrains(aimNoSlidersDifficultyValue)
	aimTopWeightedSliderFactor := aimNoSlidersTopWeightedSliderCount / max(1, aimNoSlidersDifficultStrainCount-aimNoSlidersTopWeightedSliderCount)

	speedTopWeightedSliderCount := set.speed.CountTopWeightedSliders(speedDifficultyValue)
	speedTopWeightedSliderFactor := speedTopWeightedSliderCount / max(1, speedDifficultStrainCount-speedTopWeightedSliderCount)

	aimRating := calculateAimDifficultyRating(aimDifficultyValue)
	aimNoSlidersRating := calculateAimDifficultyRating(aimNoSlidersDifficultyValue)
	sliderFactor := 1.0
	if aimDifficultyValue > 0 && aimRating > 0 {
		sliderFactor = aimNoSlidersRating / aimRating
	}
	speedRating := calculateDifficultyRating(speedDifficultyValue)
	readingRating := calculateDifficultyRating(readingDifficultyValue)
	flashlightRating := 0.0
	if diff.CheckModActive(difficulty.Flashlight) {
		flashlightRating = calculateDifficultyRating(flashlightDifficultyValue)
	}

	baseAimPerformance := skills.HarmonicDifficultyToPerformance(aimRating)
	baseSpeedPerformance := skills.HarmonicDifficultyToPerformance(speedRating)
	baseReadingPerformance := skills.HarmonicDifficultyToPerformance(readingRating)
	baseFlashlightPerformance := skills.FlashlightDifficultyToPerformance(flashlightRating)
	baseCognitionPerformance := sumCognitionDifficulty(baseReadingPerformance, baseFlashlightPerformance)
	basePerformance := putils.Norm(performanceNormExponent, baseAimPerformance, baseSpeedPerformance, baseCognitionPerformance)

	return api.Attributes{
		Total:                        calculateStarRating(basePerformance),
		Aim:                          aimRating,
		AimNoSliders:                 aimNoSlidersRating,
		Speed:                        speedRating,
		Flashlight:                   flashlightRating,
		Reading:                      readingRating,
		SliderFactor:                 sliderFactor,
		SpeedNoteCount:               set.speed.RelevantNoteCount(),
		AimDifficultStrainCount:      aimDifficultStrainCount,
		AimDifficultSliderCount:      set.aim.GetDifficultSliders(),
		SpeedDifficultStrainCount:    speedDifficultStrainCount,
		ReadingDifficultNoteCount:    readingDifficultNoteCount,
		AimTopWeightedSliderFactor:   aimTopWeightedSliderFactor,
		SpeedTopWeightedSliderFactor: speedTopWeightedSliderFactor,
		ObjectCount:                  sim.hitObjects,
		Circles:                      sim.circles,
		Sliders:                      sim.sliders,
		Spinners:                     sim.spinners,
		MaxCombo:                     int(sim.combo),
		NestedScorePerObject:         sim.NestedScorePerObject,
		LegacyScoreBaseMultiplier:    sim.ScoreMultiplier,
		MaximumLegacyComboScore:      sim.ComboScore,
	}
}

func (calc *DifficultyCalculator) CalculateSingle(bMap *beatmap.BeatMap, diff *difficulty.Difficulty) api.Attributes {
	if bMap == nil || diff == nil || len(bMap.HitObjects) == 0 {
		return api.Attributes{}
	}

	diffObjects := preprocessing.CreateDifficultyObjects(bMap.HitObjects, diff)
	set := newSkillSet(diff, len(bMap.HitObjects))
	sim := newScoreSim(bMap, diff)
	sim.add(bMap.HitObjects[0])

	for _, object := range diffObjects {
		sim.Add(object)
		set.process(object)
	}

	return calc.buildAttributes(set, sim, diff)
}

// CalculateStep mirrors osu!lazer's timed difficulty calculation: hitobjects
// are added to the progressive score state in beatmap order, while difficulty
// objects are processed once their base object's end time is reached.
func (calc *DifficultyCalculator) CalculateStep(bMap *beatmap.BeatMap, diff *difficulty.Difficulty) []api.Attributes {
	if bMap == nil || diff == nil || len(bMap.HitObjects) == 0 {
		return nil
	}

	modString := difficulty.GetDiffMaskedMods(diff.Mods).String()
	if modString == "" {
		modString = "NM"
	}
	log.Println("Calculating step SR for mods:", modString)
	started := time.Now()

	diffObjects := preprocessing.CreateDifficultyObjects(bMap.HitObjects, diff)
	set := newSkillSet(diff, len(bMap.HitObjects))
	sim := newScoreSim(bMap, diff)
	attributes := make([]api.Attributes, 0, len(bMap.HitObjects))
	currentIndex := 0

	for _, object := range bMap.HitObjects {
		sim.add(object)
		objectEnd := difficultyCalculationEndTime(object) / diff.Speed
		for currentIndex < len(diffObjects) && diffObjects[currentIndex].EndTime <= objectEnd {
			set.process(diffObjects[currentIndex])
			currentIndex++
		}
		attributes = append(attributes, calc.buildAttributes(set, sim, diff))
	}

	log.Println("Calculations finished! Took ", time.Since(started).Truncate(time.Millisecond).String())
	return attributes
}

func difficultyCalculationEndTime(object objects.IHitObject) float64 {
	if slider, ok := object.(*objects.Slider); ok {
		return slider.EndTimeLazer
	}
	return object.GetEndTime()
}

// CalculateStrainPeaks is a danser presentation adapter. The authoritative
// 20260706 calculation is CalculateSingle/CalculateStep; this method samples
// local skill strain into 400ms buckets for the existing graph UI.
func (calc *DifficultyCalculator) CalculateStrainPeaks(bMap *beatmap.BeatMap, diff *difficulty.Difficulty) api.StrainPeaks {
	peaks := api.StrainPeaks{Baseline: 0}
	if bMap == nil || diff == nil || len(bMap.HitObjects) < 2 {
		return peaks
	}

	diffObjects := preprocessing.CreateDifficultyObjects(bMap.HitObjects, diff)
	if len(diffObjects) == 0 {
		return peaks
	}

	set := newSkillSet(diff, len(bMap.HitObjects))
	startTime := diffObjects[0].StartTime
	endTime := diffObjects[len(diffObjects)-1].StartTime
	sectionCount := max(1, int(math.Ceil((endTime-startTime)/400))+1)

	peaks.Aim = make([]float64, sectionCount)
	peaks.Speed = make([]float64, sectionCount)
	peaks.Flashlight = make([]float64, sectionCount)
	reading := make([]float64, sectionCount)

	for _, object := range diffObjects {
		section := int((object.StartTime - startTime) / 400)
		section = mutils.Clamp(section, 0, sectionCount-1)

		aimStrain := set.aim.Process(object)
		set.aimWithoutSliders.Process(object)
		speedStrain := set.speed.Process(object)
		readingStrain := set.reading.Process(object)
		flashlightStrain := set.flashlight.Process(object)

		peaks.Aim[section] = max(peaks.Aim[section], aimStrain)
		peaks.Speed[section] = max(peaks.Speed[section], speedStrain)
		peaks.Flashlight[section] = max(peaks.Flashlight[section], flashlightStrain)
		reading[section] = max(reading[section], readingStrain)
	}

	peaks.Total = make([]float64, sectionCount)
	for i := range sectionCount {
		aimRating := calculateAimDifficultyRating(peaks.Aim[i])
		speedRating := calculateDifficultyRating(peaks.Speed[i])
		readingRating := calculateDifficultyRating(reading[i])
		flashlightRating := 0.0
		if diff.CheckModActive(difficulty.Flashlight) {
			flashlightRating = calculateDifficultyRating(peaks.Flashlight[i])
		}

		baseCognition := sumCognitionDifficulty(
			skills.HarmonicDifficultyToPerformance(readingRating),
			skills.FlashlightDifficultyToPerformance(flashlightRating),
		)
		basePerformance := putils.Norm(
			performanceNormExponent,
			skills.HarmonicDifficultyToPerformance(aimRating),
			skills.HarmonicDifficultyToPerformance(speedRating),
			baseCognition,
		)
		peaks.Total[i] = calculateStarRating(basePerformance)
	}

	return peaks
}

func (calc *DifficultyCalculator) GetVersion() int {
	return CurrentVersion
}

func (calc *DifficultyCalculator) GetVersionMessage() string {
	return "2026-07-06 deployed SR/PP rework (reference source snapshot: ppy/osu@48c4800e3ae4ee752452cdff83bd3787ccf3105f)"
}

func calculateAimDifficultyRating(difficultyValue float64) float64 {
	return math.Pow(difficultyValue, 0.63) * 0.02275
}

func calculateDifficultyRating(difficultyValue float64) float64 {
	return math.Sqrt(difficultyValue) * 0.0675
}

// sumCognitionDifficulty matches osu!lazer's internal cognition combination.
// Cognition is not an exported PP component; Reading and Flashlight remain
// separate in api.PPv2Results.
func sumCognitionDifficulty(reading, flashlight float64) float64 {
	if reading <= 0 {
		return flashlight
	}
	if flashlight <= 0 {
		return reading
	}
	return putils.Norm(performanceNormExponent, reading, flashlight*mutils.Clamp(flashlight/reading, 0.25, 1))
}

func calculateStarRating(basePerformance float64) float64 {
	return math.Cbrt(basePerformance * performanceBaseMultiplier)
}
