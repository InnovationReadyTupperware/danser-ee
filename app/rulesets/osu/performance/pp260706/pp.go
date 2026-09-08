package pp260706

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/skills"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

type PPv2 struct {
	attribs api.Attributes
	score   api.PerfScore
	diff    *difficulty.Difficulty

	usingClassicSliderAccuracy bool
	usingScoreV2               bool

	accuracy      float64
	scoreMaxCombo int
	countGreat    int
	countOk       int
	countMeh      int
	countMiss     int

	countSliderTickMiss    int
	countSliderEndsDropped int
	effectiveMissCount     float64

	totalHits           int
	totalSuccessfulHits int
	totalImperfectHits  int

	greatHitWindow float64
	okHitWindow    float64
	mehHitWindow   float64

	overallDifficulty float64
	approachRate      float64

	speedDeviation             *float64
	aimEstimatedSliderBreaks   float64
	speedEstimatedSliderBreaks float64
}

func NewPPCalculator() api.IPerformanceCalculator {
	return &PPv2{}
}

func usesClassicSliderAccuracy(diff *difficulty.Difficulty) bool {
	if diff == nil {
		return false
	}

	if !diff.IsLazer() {
		// osu!lazer appends Classic with its default settings when decoding a
		// pre-lazer replay. Danser keeps replay provenance separate from mods,
		// so Stable replay playback maps to the same default slider semantics here
		return true
	}

	if !diff.CheckModActive(difficulty.Classic) {
		return false
	}

	conf, ok := difficulty.GetModConfig[difficulty.ClassicSettings](diff)
	return ok && conf.NoSliderHeadAccuracy
}

// Calculate evaluates performance for attribs and score. diff must be non-nil
// because the released model reads mods, gameplay provenance, and adjusted hit
// windows from it.
func (pp *PPv2) Calculate(attribs api.Attributes, score api.PerfScore, diff *difficulty.Difficulty) api.PPv2Results {
	if score.MaxCombo < 0 {
		score.MaxCombo = attribs.MaxCombo
	}
	if score.CountGreat < 0 {
		score.CountGreat = attribs.ObjectCount - score.CountOk - score.CountMeh - score.CountMiss
	}
	if score.SliderTailHits < 0 {
		score.SliderTailHits = attribs.Sliders
	}

	pp.attribs = attribs
	pp.score = score
	pp.diff = diff
	pp.usingScoreV2 = diff.Mods.Active(difficulty.ScoreV2)
	pp.usingClassicSliderAccuracy = usesClassicSliderAccuracy(diff)

	pp.accuracy = mutils.Clamp(score.Accuracy, 0, 1)
	pp.scoreMaxCombo = mutils.Clamp(score.MaxCombo, 0, attribs.MaxCombo)
	pp.countGreat = score.CountGreat
	pp.countOk = score.CountOk
	pp.countMeh = score.CountMeh
	pp.countMiss = score.CountMiss
	pp.countSliderEndsDropped = max(0, attribs.Sliders-score.SliderTailHits)
	pp.countSliderTickMiss = max(0, score.SliderTickMisses)
	pp.totalHits = pp.countGreat + pp.countOk + pp.countMeh + pp.countMiss
	pp.totalSuccessfulHits = pp.countGreat + pp.countOk + pp.countMeh
	pp.totalImperfectHits = pp.countOk + pp.countMeh + pp.countMiss
	pp.effectiveMissCount = float64(pp.countMiss)

	pp.greatHitWindow = diff.Hit300U / diff.GetSpeed()
	pp.okHitWindow = diff.Hit100U / diff.GetSpeed()
	pp.mehHitWindow = diff.Hit50U / diff.GetSpeed()
	pp.overallDifficulty = diff.ODReal
	pp.approachRate = diff.ARReal

	comboBasedEstimatedMissCount := pp.calculateComboBasedEstimatedMissCount(attribs)
	if pp.usingClassicSliderAccuracy && !pp.usingScoreV2 && score.LegacyTotalScore != nil {
		pp.effectiveMissCount = CalculateMissCount(score, attribs, diff)
	} else {
		pp.effectiveMissCount = comboBasedEstimatedMissCount
	}

	pp.effectiveMissCount = max(float64(pp.countMiss), pp.effectiveMissCount)
	pp.effectiveMissCount = min(float64(pp.totalHits), pp.effectiveMissCount)
	pp.effectiveMissCount = max(0, pp.effectiveMissCount)

	pp.aimEstimatedSliderBreaks = 0
	pp.speedEstimatedSliderBreaks = 0
	if pp.effectiveMissCount > 0 {
		pp.aimEstimatedSliderBreaks = pp.calculateEstimatedSliderBreaks(attribs.AimTopWeightedSliderFactor, attribs)
		pp.speedEstimatedSliderBreaks = pp.calculateEstimatedSliderBreaks(attribs.SpeedTopWeightedSliderFactor, attribs)
	}

	multiplier := performanceBaseMultiplier
	if diff.Mods.Active(difficulty.NoFail) {
		multiplier *= max(0.90, 1-0.02*pp.effectiveMissCount)
	}
	if diff.Mods.Active(difficulty.SpunOut) && pp.totalHits > 0 {
		multiplier *= 1 - math.Pow(float64(attribs.Spinners)/float64(pp.totalHits), 0.85)
	}
	if diff.Mods.Active(difficulty.Relax) {
		okMultiplier := 0.75
		mehMultiplier := 1.0
		if pp.overallDifficulty > 0 {
			okMultiplier *= max(0, 1-pp.overallDifficulty/13.33)
			mehMultiplier *= max(0, 1-putils.PowInt(pp.overallDifficulty/13.33, 5))
		}
		pp.effectiveMissCount = min(pp.effectiveMissCount+float64(pp.countOk)*okMultiplier+float64(pp.countMeh)*mehMultiplier, float64(pp.totalHits))
	}

	pp.speedDeviation = pp.calculateSpeedDeviation(attribs)
	aimValue := pp.computeAimValue()
	speedValue := pp.computeSpeedValue()
	accuracyValue := pp.computeAccuracyValue()
	readingValue := pp.computeReadingValue()
	flashlightValue := pp.computeFlashlightValue()
	cognitionValue := sumCognitionDifficulty(readingValue, flashlightValue)

	results := api.PPv2Results{
		Aim:        aimValue,
		Speed:      speedValue,
		Acc:        accuracyValue,
		Flashlight: flashlightValue,
		Reading:    readingValue,
	}
	results.Total = putils.Norm(performanceNormExponent, aimValue, speedValue, accuracyValue, cognitionValue) * multiplier
	return results
}

func (pp *PPv2) computeAimValue() float64 {
	if pp.diff.CheckModActive(difficulty.Autopilot) {
		return 0
	}

	aimDifficulty := pp.attribs.Aim
	if pp.attribs.Sliders > 0 && pp.attribs.AimDifficultSliderCount > 0 {
		var improperlyFollowed float64
		if pp.usingClassicSliderAccuracy {
			maximumPossibleDroppedSliders := pp.totalImperfectHits
			improperlyFollowed = mutils.Clamp(float64(min(maximumPossibleDroppedSliders, pp.attribs.MaxCombo-pp.scoreMaxCombo)), 0, pp.attribs.AimDifficultSliderCount)
		} else {
			improperlyFollowed = mutils.Clamp(float64(pp.countSliderEndsDropped+pp.countSliderTickMiss), 0, pp.attribs.AimDifficultSliderCount)
		}
		sliderNerfFactor := (1-pp.attribs.SliderFactor)*putils.PowInt(1-improperlyFollowed/pp.attribs.AimDifficultSliderCount, 3) + pp.attribs.SliderFactor
		aimDifficulty *= sliderNerfFactor
	}

	aimValue := skills.HarmonicDifficultyToPerformance(aimDifficulty)
	lengthBonus := 0.95 + 0.35*min(1.0, float64(pp.totalHits)/2000)
	if pp.totalHits > 2000 {
		lengthBonus += math.Log10(float64(pp.totalHits)/2000) * 0.5
	}
	aimValue *= lengthBonus

	if pp.effectiveMissCount > 0 {
		relevantMissCount := min(pp.effectiveMissCount+pp.aimEstimatedSliderBreaks, float64(pp.totalImperfectHits+pp.countSliderTickMiss))
		aimValue *= pp.calculateMissPenalty(relevantMissCount, pp.attribs.AimDifficultStrainCount)
	}
	if pp.diff.Mods.Active(difficulty.Traceable) {
		aimValue *= 1 + pp.calculateTraceableBonus(pp.attribs.SliderFactor)
	}
	aimValue *= pp.accuracy
	return aimValue
}

func (pp *PPv2) computeSpeedValue() float64 {
	if pp.diff.Mods.Active(difficulty.Relax) || pp.speedDeviation == nil {
		return 0
	}

	speedValue := skills.HarmonicDifficultyToPerformance(pp.attribs.Speed)
	if pp.effectiveMissCount > 0 {
		relevantMissCount := min(pp.effectiveMissCount+pp.speedEstimatedSliderBreaks, float64(pp.totalImperfectHits+pp.countSliderTickMiss))
		speedValue *= pp.calculateMissPenalty(relevantMissCount, pp.attribs.SpeedDifficultStrainCount)
	}
	speedValue *= pp.calculateSpeedHighDeviationNerf(pp.attribs)

	effectiveHitWindow := 20 * math.Pow(4/pp.attribs.Speed, 0.35)
	effectiveAccuracy := osuErf(effectiveHitWindow / *pp.speedDeviation)
	speedValue *= putils.PowInt(effectiveAccuracy, 2)
	return speedValue
}

func (pp *PPv2) computeAccuracyValue() float64 {
	if pp.diff.Mods.Active(difficulty.Relax) {
		return 0
	}

	amountHitObjectsWithAccuracy := pp.attribs.Circles
	if !pp.usingClassicSliderAccuracy || pp.usingScoreV2 {
		amountHitObjectsWithAccuracy += pp.attribs.Sliders
	}

	betterAccuracyPercentage := 0.0
	if amountHitObjectsWithAccuracy > 0 {
		betterAccuracyPercentage = float64((pp.countGreat-max(pp.totalHits-amountHitObjectsWithAccuracy, 0))*6+pp.countOk*2+pp.countMeh) / float64(amountHitObjectsWithAccuracy*6)
	}
	betterAccuracyPercentage = max(0, betterAccuracyPercentage)

	accuracyValue := math.Pow(1.52163, pp.overallDifficulty) * math.Pow(betterAccuracyPercentage, 24) * 2.83
	if amountHitObjectsWithAccuracy < 1000 {
		accuracyValue *= math.Pow(float64(amountHitObjectsWithAccuracy)/1000, 0.3)
	} else {
		accuracyValue *= math.Pow(float64(amountHitObjectsWithAccuracy)/1000, 0.1)
	}
	if pp.diff.Mods.Active(difficulty.Traceable) {
		accuracyValue *= 1 + 0.08*putils.ReverseLerp(pp.approachRate, 11.5, 10)
	}
	return accuracyValue
}

func (pp *PPv2) computeFlashlightValue() float64 {
	if !pp.diff.CheckModActive(difficulty.Flashlight) {
		return 0
	}

	flashlightValue := skills.FlashlightDifficultyToPerformance(pp.attribs.Flashlight)
	if pp.effectiveMissCount > 0 && pp.totalHits > 0 {
		flashlightValue *= 0.97 * math.Pow(1-math.Pow(pp.effectiveMissCount/float64(pp.totalHits), 0.775), math.Pow(pp.effectiveMissCount, 0.875))
	}
	flashlightValue *= pp.getComboScalingFactor()
	flashlightValue *= 0.5 + pp.accuracy/2
	return flashlightValue
}

func (pp *PPv2) computeReadingValue() float64 {
	readingValue := skills.HarmonicDifficultyToPerformance(pp.attribs.Reading)
	if pp.effectiveMissCount > 0 {
		readingValue *= pp.calculateMissPenalty(pp.effectiveMissCount+pp.aimEstimatedSliderBreaks, pp.attribs.ReadingDifficultNoteCount)
	}
	readingValue *= putils.PowInt(pp.accuracy, 3)
	return readingValue
}

func (pp *PPv2) calculateComboBasedEstimatedMissCount(attributes api.Attributes) float64 {
	if attributes.Sliders <= 0 {
		return float64(pp.countMiss)
	}

	missCount := float64(pp.countMiss)
	if pp.usingClassicSliderAccuracy {
		likelyMissedSliderendPortion := 0.04 + 0.06*putils.PowInt(min(attributes.AimTopWeightedSliderFactor, 1), 2)
		fullComboThreshold := float64(attributes.MaxCombo) - min(4+likelyMissedSliderendPortion*float64(attributes.Sliders), float64(attributes.Sliders))
		if float64(pp.scoreMaxCombo) < fullComboThreshold {
			missCount = fullComboThreshold / max(1, float64(pp.scoreMaxCombo))
		}
		missCount = min(missCount, float64(pp.totalImperfectHits))
		maxPossibleSliderBreaks := min(attributes.Sliders, (attributes.MaxCombo-pp.scoreMaxCombo)/2)
		sliderBreaks := missCount - float64(pp.countMiss)
		if sliderBreaks > float64(maxPossibleSliderBreaks) {
			missCount = float64(pp.countMiss + maxPossibleSliderBreaks)
		}
	} else {
		fullComboThreshold := float64(attributes.MaxCombo - pp.countSliderEndsDropped)
		if float64(pp.scoreMaxCombo) < fullComboThreshold {
			missCount = fullComboThreshold / max(1, float64(pp.scoreMaxCombo))
		}
		missCount = min(missCount, float64(pp.countSliderTickMiss+pp.countMiss))
	}
	return missCount
}

func (pp *PPv2) calculateEstimatedSliderBreaks(topWeightedSliderFactor float64, attributes api.Attributes) float64 {
	nonMissMistakes := pp.countOk + pp.countMeh
	if !pp.usingClassicSliderAccuracy || nonMissMistakes == 0 || attributes.MaxCombo <= 0 {
		return 0
	}

	missedComboPercent := 1 - float64(pp.scoreMaxCombo)/float64(attributes.MaxCombo)
	estimatedSliderBreaks := min(float64(nonMissMistakes), pp.effectiveMissCount*topWeightedSliderFactor)
	nonMissMistakeAdjustment := (float64(nonMissMistakes) - estimatedSliderBreaks + 4.5) / (float64(nonMissMistakes) + 4)
	estimatedSliderBreaks *= putils.Smoothstep(pp.effectiveMissCount, 1, 2)
	return estimatedSliderBreaks * nonMissMistakeAdjustment * putils.Logistic(missedComboPercent, 0.33, 15, 1)
}

func (pp *PPv2) calculateSpeedDeviation(attributes api.Attributes) *float64 {
	if pp.totalSuccessfulHits == 0 {
		return nil
	}

	speedNoteCount := attributes.SpeedNoteCount + (float64(pp.totalHits)-attributes.SpeedNoteCount)*0.1
	relevantCountMiss := min(float64(pp.countMiss), speedNoteCount)
	relevantCountMeh := min(float64(pp.countMeh), speedNoteCount-relevantCountMiss)
	relevantCountOk := min(float64(pp.countOk), speedNoteCount-relevantCountMiss-relevantCountMeh)
	relevantCountGreat := max(0, speedNoteCount-relevantCountMiss-relevantCountMeh-relevantCountOk)
	return pp.calculateDeviation(relevantCountGreat, relevantCountOk, relevantCountMeh)
}

func (pp *PPv2) calculateDeviation(relevantCountGreat, relevantCountOk, relevantCountMeh float64) *float64 {
	if relevantCountGreat+relevantCountOk+relevantCountMeh <= 0 {
		return nil
	}

	n := max(1, relevantCountGreat+relevantCountOk)
	p := relevantCountGreat / n
	const z = 2.32634787404
	pLowerBound := min(p, (n*p+z*z/2)/(n+z*z)-z/(n+z*z)*math.Sqrt(n*p*(1-p)+z*z/4))

	var deviation float64
	if pLowerBound > 0.01 {
		deviation = pp.greatHitWindow / (math.Sqrt2 * osuErfInv(pLowerBound))
		okHitWindowTailAmount := math.Sqrt(2/math.Pi) * pp.okHitWindow * math.Exp(-0.5*putils.PowInt(pp.okHitWindow/deviation, 2)) /
			(deviation * osuErf(pp.okHitWindow/(math.Sqrt2*deviation)))
		deviation *= math.Sqrt(1 - okHitWindowTailAmount)
	} else {
		deviation = pp.okHitWindow / math.Sqrt(3)
	}

	mehVariance := (pp.mehHitWindow*pp.mehHitWindow + pp.okHitWindow*pp.mehHitWindow + pp.okHitWindow*pp.okHitWindow) / 3
	deviation = math.Sqrt(((relevantCountGreat+relevantCountOk)*putils.PowInt(deviation, 2) + relevantCountMeh*mehVariance) /
		(relevantCountGreat + relevantCountOk + relevantCountMeh))
	return &deviation
}

func (pp *PPv2) calculateSpeedHighDeviationNerf(attributes api.Attributes) float64 {
	if pp.speedDeviation == nil {
		return 0
	}
	speedValue := skills.HarmonicDifficultyToPerformance(attributes.Speed)
	excessSpeedDifficultyCutoff := 100 + 220*math.Pow(22/(*pp.speedDeviation), 6.5)
	if speedValue <= excessSpeedDifficultyCutoff {
		return 1
	}
	const scale = 50.0
	adjustedSpeedValue := scale * (math.Log((speedValue-excessSpeedDifficultyCutoff)/scale+1) + excessSpeedDifficultyCutoff/scale)
	lerp := 1 - putils.ReverseLerp(*pp.speedDeviation, 22, 27)
	adjustedSpeedValue = mutils.Lerp(adjustedSpeedValue, speedValue, lerp)
	return adjustedSpeedValue / speedValue
}

func (pp *PPv2) calculateTraceableBonus(sliderFactor float64) float64 {
	highARSliderVisibilityFactor := 0.5 + math.Pow(sliderFactor, 6)/2
	lowARSliderVisibilityFactor := math.Pow(sliderFactor, 6)
	traceableBonus := 0.0275
	traceableBonus += 0.025 * (12 - max(pp.approachRate, 7)) * highARSliderVisibilityFactor
	if pp.approachRate < 7 {
		traceableBonus += 0.025 * (7 - max(pp.approachRate, 0)) * lowARSliderVisibilityFactor
	}
	if pp.approachRate < 0 {
		traceableBonus += 0.025 * (1 - math.Pow(1.5, pp.approachRate)) * lowARSliderVisibilityFactor
	}
	return traceableBonus
}

func (pp *PPv2) calculateMissPenalty(missCount, difficultStrainCount float64) float64 {
	return 0.93 / (missCount/(4*math.Log(max(1, difficultStrainCount))) + 1)
}

func (pp *PPv2) getComboScalingFactor() float64 {
	if pp.attribs.MaxCombo <= 0 {
		return 1
	}
	return min(math.Pow(float64(pp.scoreMaxCombo), 0.8)/math.Pow(float64(pp.attribs.MaxCombo), 0.8), 1)
}
