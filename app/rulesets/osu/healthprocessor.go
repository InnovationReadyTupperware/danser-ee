package osu

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	HpMu   = 6.0
	HpKatu = 10.0
	HpGeki = 14.0

	Hp50  = 0.4
	Hp100 = 2.2
	Hp300 = 6.0

	HpSliderTick   = 3.0
	HpSliderRepeat = 4.0

	HpSpinnerSpin  = 1.7
	HpSpinnerBonus = 2.0

	MaxHp = 200.0
)

type IHealthProcessor interface {
	CalculateRate()

	ResetHp()

	AddResult(result JudgementResult)

	Increase(amount float64, fromHitObject bool)

	IncreaseRelative(amount float64, fromHitObject bool)

	Update(time int64)

	AddFailListener(listener FailListener)

	GetHealth() float64

	GetDrainRate() float64
}

type FailListener func()

type drainPeriod struct {
	start, end int64
}

// drainedDuration counts only active drain periods, including periods crossed
// by one large update. This keeps seeks and delayed frames from losing drain
// while still excluding time before gameplay and during breaks.
func drainedDuration(periods []drainPeriod, from, to int64) int64 {
	if to <= from {
		return 0
	}

	var duration int64
	for _, period := range periods {
		start := max(from, period.start)
		end := min(to, period.end)
		if end > start {
			duration += end - start
		}
	}

	return duration
}

// healthObjectEndTime keeps the health timeline aligned with the participant's
// gameplay provenance. Lazer's non-classic health path uses the fractional
// slider duration, while ClassicHealth intentionally retains Stable timing.
func healthObjectEndTime(object objects.IHitObject, player *difficultyPlayer) float64 {
	if player != nil && player.diff != nil && player.diff.IsLazer() && !player.classicHealth {
		return objects.GetEndTimeForDiff(object, player.diff)
	}

	return object.GetEndTime()
}

// ignoresPathologicalSliderDetails applies the cursor-dance fallback to the
// CL health path. Generated cursors collapse pathological sliders to their
// head target, so their unreachable nested events must not drain health.
func (hp *HealthProcessor) ignoresPathologicalSliderDetails(slider *objects.Slider) bool {
	return hp.player != nil && hp.player.cursor != nil &&
		hp.player.cursor.IsCursorDance && hp.player.diff != nil &&
		hp.player.diff.IsLazer() && hp.player.classicHealth &&
		slider != nil && slider.IsPathological()
}

type HealthProcessor struct {
	beatMap *beatmap.BeatMap
	player  *difficultyPlayer

	PassiveDrain         float64
	HpMultiplierNormal   float64
	HpMultiplierComboEnd float64

	health         float64
	healthUncapped float64

	drains            []drainPeriod
	lastTime          int64
	lowerSpinnerDrain bool

	spinners      []*objects.Spinner
	spinnerActive bool

	playing bool

	failListeners []FailListener
}

func NewHealthProcessor(beatMap *beatmap.BeatMap, player *difficultyPlayer, lowerSpinnerDrain bool) *HealthProcessor {
	hp := &HealthProcessor{
		beatMap:           beatMap,
		player:            player,
		lowerSpinnerDrain: lowerSpinnerDrain,
	}

	for _, o := range beatMap.HitObjects {
		if s, ok := o.(*objects.Spinner); ok {
			hp.spinners = append(hp.spinners, s)
		}
	}

	hp.calculateDrainPeriods()

	return hp
}

func (hp *HealthProcessor) calculateDrainPeriods() {
	breakCount := len(hp.beatMap.Pauses)

	breakNumber := 0
	lastDrainStart := int64(hp.beatMap.HitObjects[0].GetStartTime()) - int64(hp.player.diff.Preempt)
	lastDrainEnd := int64(hp.beatMap.HitObjects[0].GetStartTime()) - int64(hp.player.diff.Preempt)

	for _, o := range hp.beatMap.HitObjects {
		if breakCount > 0 && breakNumber < breakCount {
			pause := hp.beatMap.Pauses[breakNumber]
			if pause.GetStartTime() >= float64(lastDrainEnd) && pause.GetEndTime() <= o.GetStartTime() {
				breakNumber++

				if hp.beatMap.Version < 8 {
					lastDrainEnd = int64(pause.GetStartTime())
				}

				hp.drains = append(hp.drains, drainPeriod{lastDrainStart, lastDrainEnd})

				lastDrainStart = int64(o.GetStartTime())
			}
		}

		lastDrainEnd = int64(healthObjectEndTime(o, hp.player))
	}

	hp.drains = append(hp.drains, drainPeriod{lastDrainStart, lastDrainEnd})
}

func (hp *HealthProcessor) CalculateRate() { //nolint:gocyclo
	lowestHpEver := difficulty.DifficultyRate(hp.player.diff.HPMod, 195, 160, 60)
	lowestHpComboEnd := difficulty.DifficultyRate(hp.player.diff.HPMod, 198, 170, 80)
	lowestHpEnd := difficulty.DifficultyRate(hp.player.diff.HPMod, 198, 180, 80)
	hpRecoveryAvailable := difficulty.DifficultyRate(hp.player.diff.HPMod, 8, 4, 0)

	hp.PassiveDrain = 0.05
	hp.HpMultiplierNormal = 1.0
	hp.HpMultiplierComboEnd = 1.0

	fail := true

	breakCount := len(hp.beatMap.Pauses)

	for fail {
		hp.ResetHp()

		lowestHp := hp.health

		lastTime := int64(hp.beatMap.HitObjects[0].GetStartTime()) - int64(hp.player.diff.Preempt)
		fail = false

		breakNumber := 0

		comboTooLowCount := 0

		for i, o := range hp.beatMap.HitObjects {
			localLastTime := lastTime

			breakTime := int64(0)

			if breakCount > 0 && breakNumber < breakCount {
				pause := hp.beatMap.Pauses[breakNumber]
				if pause.GetStartTime() >= float64(localLastTime) && pause.GetEndTime() <= o.GetStartTime() {
					if hp.beatMap.Version < 8 {
						breakTime = int64(pause.Length())
					} else {
						breakTime = int64(pause.GetEndTime()) - localLastTime
					}
					breakNumber++
				}
			}

			hp.Increase(-hp.PassiveDrain*(o.GetStartTime()-float64(lastTime+breakTime)), false)

			lastTime = int64(healthObjectEndTime(o, hp.player))

			lowestHp = min(lowestHp, hp.health)

			if hp.health <= lowestHpEver {
				fail = true
				hp.PassiveDrain *= 0.96

				break
			}

			decr := hp.PassiveDrain * (healthObjectEndTime(o, hp.player) - o.GetStartTime())
			hpUnder := min(0, hp.health-decr)

			hp.Increase(-decr, false)

			lazerSkipScore := false

			if s, ok := o.(*objects.Slider); ok {
				if hp.ignoresPathologicalSliderDetails(s) {
					// Cursor dance represents this authored slider with its head
					// only. Keep the parent result below, but do not model
					// unreachable nested events in the ideal health schedule.
				} else if hp.player.diff.IsLazer() && !hp.player.classicHealth {
					// Keep the fallback Lazer schedule for direct users of the
					// legacy processor. Production Lazer gameplay with Classic's
					// ClassicHealth setting uses the stable-compatible schedule below.
					lazerSkipScore = true

					headResult := Hit300
					if hp.player.classicNoSliderHeadAccuracy {
						headResult = LargeTickHit
					}
					hp.addResultWithPart(headResult, sliderPartHead)

					for _, event := range buildLazerSliderEvents(s, hp.player.classicNoSliderHeadAccuracy) {
						hp.addResultWithPart(event.maxResult, event.resultPart())
					}

					if hp.player.classicNoSliderHeadAccuracy {
						hp.addResultInternal(Hit300)
					}
				} else {
					repeats := len(s.TickReverse) + 1

					for j := 0; j < repeats; j++ {
						hp.addResultInternal(SliderRepeat)
					}

					for j := 0; j < len(s.TickPoints); j++ {
						hp.addResultInternal(SliderPoint)
					}
				}
			} else if s, ok := o.(*objects.Spinner); ok {
				requirement := calculateHealthSpinnerRequirement(hp.player.diff, s.GetStartTime(), s.GetEndTime())

				for j := int64(0); j < requirement; j++ {
					hp.addResultInternal(SpinnerSpin)
				}
			}

			//noinspection GoBoolExpressions - false positive
			if hpUnder < 0 && hp.health+hpUnder <= lowestHpEver {
				fail = true
				hp.PassiveDrain *= 0.96

				break
			}

			if !hp.player.diff.IsLazer() && (i == len(hp.beatMap.HitObjects)-1 || hp.beatMap.HitObjects[i+1].IsNewCombo()) {
				hp.addResultInternal(Hit300g)

				if hp.health < lowestHpComboEnd {
					comboTooLowCount++
					if comboTooLowCount > 2 {
						hp.HpMultiplierComboEnd *= 1.07
						hp.HpMultiplierNormal *= 1.03
						fail = true

						break
					}
				}
			} else if !lazerSkipScore {
				hp.addResultInternal(Hit300)
			}
		}

		if !fail && hp.health < lowestHpEnd {
			fail = true
			hp.PassiveDrain *= 0.94
			hp.HpMultiplierComboEnd *= 1.01
			hp.HpMultiplierNormal *= 1.01
		}

		recovery := (hp.healthUncapped - MaxHp) / float64(len(hp.beatMap.HitObjects))
		if !fail && recovery < hpRecoveryAvailable {
			fail = true
			hp.PassiveDrain *= 0.96
			hp.HpMultiplierComboEnd *= 1.02
			hp.HpMultiplierNormal *= 1.01
		}
	}

	hp.ResetHp()
	hp.playing = true
}

func (hp *HealthProcessor) ResetHp() {
	hp.health = MaxHp
	hp.healthUncapped = MaxHp
}

func (hp *HealthProcessor) AddResult(result JudgementResult) {
	if result.IsSliderNested() && result.object != nil {
		if slider, ok := result.object.GetObject().(*objects.Slider); ok && hp.ignoresPathologicalSliderDetails(slider) {
			return
		}
	}

	hp.addResultWithPart(result.HitResult, result.sliderPart)
}

func (hp *HealthProcessor) addResultInternal(result HitResult) {
	hp.addResultWithPart(result, sliderPartNone)
}

func (hp *HealthProcessor) addResultWithPart(result HitResult, part sliderJudgementPart) {
	normal := result & (^Additions)
	addition := result & Additions

	hpAdd := 0.0

	switch normal {
	case SliderMiss, SmallTickMiss, LargeTickMiss:
		hpAdd += difficulty.DifficultyRate(hp.player.diff.HPMod, -4.0, -15.0, -28.0)
	case IgnoreMiss:
		return
	case Miss:
		hpAdd += difficulty.DifficultyRate(hp.player.diff.HPMod, -6.0, -25.0, -40.0)
	case Hit50:
		hpAdd += hp.HpMultiplierNormal * difficulty.DifficultyRate(hp.player.diff.HPMod, 8*Hp50, Hp50, Hp50)
	case Hit100:
		hpAdd += hp.HpMultiplierNormal * difficulty.DifficultyRate(hp.player.diff.HPMod, 8*Hp100, Hp100, Hp100)
	case Hit300:
		hpAdd += hp.HpMultiplierNormal * Hp300
	case SliderPoint:
		hpAdd += hp.HpMultiplierNormal * HpSliderTick
	case SliderStart, SliderRepeat, LegacySliderEnd, SliderEnd:
		hpAdd += hp.HpMultiplierNormal * HpSliderRepeat
	case SmallTickHit, SliderTailHit:
		hpAdd += hp.HpMultiplierNormal * HpSliderRepeat
	case LargeTickHit:
		if part == sliderPartTick {
			hpAdd += hp.HpMultiplierNormal * HpSliderTick
		} else {
			hpAdd += hp.HpMultiplierNormal * HpSliderRepeat
		}
	case SpinnerSpin, SpinnerPoints:
		hpAdd += hp.HpMultiplierNormal * HpSpinnerSpin
	case SpinnerBonus:
		hpAdd += hp.HpMultiplierNormal * HpSpinnerBonus
	}

	if !hp.player.diff.IsLazer() {
		switch addition {
		case MuAddition:
			hpAdd += hp.HpMultiplierComboEnd * HpMu
		case KatuAddition:
			hpAdd += hp.HpMultiplierComboEnd * HpKatu
		case GekiAddition:
			hpAdd += hp.HpMultiplierComboEnd * HpGeki
		}
	}

	hp.Increase(hpAdd, true)
}

func (hp *HealthProcessor) Increase(amount float64, fromHitObject bool) {
	hp.healthUncapped = max(0.0, hp.healthUncapped+amount)
	hp.health = mutils.Clamp(hp.health+amount, 0.0, MaxHp)

	if hp.playing && hp.health <= 0 && fromHitObject {
		for _, f := range hp.failListeners {
			f()
		}
	}
}

func (hp *HealthProcessor) IncreaseRelative(amount float64, fromHitObject bool) {
	hp.Increase(amount*MaxHp, fromHitObject)
}

func (hp *HealthProcessor) reducePassive(amount int64) {
	scale := 1.0

	if !hp.player.diff.IsLazer() {
		if hp.spinnerActive && hp.lowerSpinnerDrain {
			scale = 0.25
		}

		if hp.player.diff.CheckModActive(difficulty.HalfTime) {
			scale *= 0.75
		}
	}

	hp.Increase(-hp.PassiveDrain*float64(amount)*scale, false)
}

func (hp *HealthProcessor) Update(time int64) {
	hp.spinnerActive = false
	for _, d := range hp.spinners {
		if d.GetStartTime() > float64(time) {
			break
		}

		if d.GetStartTime() <= float64(time) && float64(time) <= d.GetEndTime() {
			hp.spinnerActive = true
			break
		}
	}

	if time > hp.lastTime {
		hp.reducePassive(drainedDuration(hp.drains, hp.lastTime, time))
	}

	hp.lastTime = time
}

func (hp *HealthProcessor) AddFailListener(listener FailListener) {
	hp.failListeners = append(hp.failListeners, listener)
}

func (hp *HealthProcessor) GetHealth() float64 {
	return hp.health / MaxHp
}

func (hp *HealthProcessor) GetDrainRate() float64 {
	return hp.PassiveDrain
}
