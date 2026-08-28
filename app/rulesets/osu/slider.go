package osu

import (
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/framework/math/math87"
	"github.com/wieku/danser-go/framework/math/vector"
	"math"
)

type Buttons int64

const Left = Buttons(1)
const Right = Buttons(2)

type sliderstate struct {
	downButton       Buttons
	isStartHit       bool
	isHit            bool
	points           []sliderEvent
	scored           int
	missed           int
	slideStart       int64
	sliding          bool
	startResult      HitResult
	endScored        bool
	tailSamplePlayed bool
}

type Slider struct {
	ruleSet           *OsuRuleSet
	hitSlider         *objects.Slider
	players           []*difficultyPlayer
	state             map[*difficultyPlayer]*sliderstate
	fadeStartRelative float64

	lastSliderTime      int64
	sliderPosition      vector.Vector2f
	sliderPositionLazer vector.Vector2f
}

func (slider *Slider) GetNumber() int64 {
	return slider.hitSlider.GetID()
}

func (slider *Slider) IsSliding(player *difficultyPlayer) bool {
	return slider.state[player].sliding
}

func (slider *Slider) Init(ruleSet *OsuRuleSet, object objects.IHitObject, players []*difficultyPlayer) {
	slider.ruleSet = ruleSet
	slider.hitSlider = object.(*objects.Slider)
	slider.players = players
	slider.state = make(map[*difficultyPlayer]*sliderstate)

	slider.lastSliderTime = math.MinInt64
	slider.fadeStartRelative = 100000

	for _, player := range slider.players {
		slider.fadeStartRelative = min(slider.fadeStartRelative, player.diff.Preempt)
		slider.state[player] = new(sliderstate)
		slider.state[player].startResult = Miss

		if player.diff.IsLazer() {
			slider.state[player].points = buildLazerSliderEvents(slider.hitSlider, player.classicNoSliderHeadAccuracy)
		} else {
			slider.state[player].points = buildStableSliderEvents(slider.hitSlider)
		}
	}
}

func (slider *Slider) UpdateClickFor(player *difficultyPlayer, time int64) bool {
	state := slider.state[player]

	position := slider.hitSlider.GetStackedStartPositionMod(player.diff)

	clicked := player.leftCondE || player.rightCondE

	inRadius := player.cursor.RawPosition.Dst(position) <= player.diff.GetRadius()

	if clicked && !state.isStartHit && (!state.isHit || player.diff.IsLazer()) {
		action := slider.ruleSet.CanBeHit(time, slider, player)

		if inRadius {
			if action == Click {
				if player.leftCondE {
					player.leftCondE = false
				} else if player.rightCondE {
					player.rightCondE = false
				}

				if player.leftCond {
					state.downButton = Left
				} else if player.rightCond {
					state.downButton = Right
				} else {
					state.downButton = player.mouseDownButton
				}

				state.startResult = slider.ruleSet.GetResultForDelta(player, math.Abs(float64(time)-slider.hitSlider.GetStartTime()))

				hit, maxResult := stableSliderHeadResults(state.startResult)
				part := sliderPartHead
				if player.diff.IsLazer() {
					hit, maxResult = lazerSliderHeadResults(player.classicNoSliderHeadAccuracy, state.startResult)
				}

				combo := Increase
				if !hit.IsHit() {
					combo = Reset
				}

				if hit != Ignore {
					if len(slider.players) == 1 {
						slider.hitSlider.HitEdge(0, float64(time), hit.IsHit())
					}

					state.isStartHit = true

					slider.ruleSet.PostHit(time, slider, player)

					slider.ruleSet.SendResult(player.cursor, createSliderJudgementResult(hit, maxResult, combo, time, position, slider, part))

					if state.startResult != Miss && player.diff.IsLazer() {
						slider.lazerPostHeadProcess(player, state, time)
					}
				}
			} else {
				player.leftCondE = false
				player.rightCondE = false
			}
		} else if action == Click {
			slider.ruleSet.SendResult(player.cursor, createJudgementResult(PositionalMiss, SliderStart, Hold, time, position, slider))
		}
	}

	return state.isStartHit
}

func (slider *Slider) lazerPostHeadProcess(player *difficultyPlayer, state *sliderstate, time int64) {
	sliderPosition := slider.hitSlider.GetStackedPositionAtModLazer(float64(time), player.diff)

	followRadiusFull := player.diff.GetRadius() * 2.4

	if player.cursor.RawPosition.Dst(sliderPosition) > followRadiusFull {
		return
	}

	allTicksInRange := true

	for _, point := range state.points {
		if point.judged {
			continue
		}

		// The tail's gameplay start time is the true end of the slider. Its
		// -36 ms leniency is handled by normal frame processing, not by the
		// late-head catch-up pass, which mirrors lazer's nested object timing.
		if point.time > float64(time) {
			break
		}

		currPos := slider.hitSlider.GetStackedPositionAtModLazer(float64(point.time), player.diff)

		if player.cursor.RawPosition.Dst(currPos) > followRadiusFull {
			allTicksInRange = false
			break
		}
	}

	slider.processTicksLazer(player, state, time, allTicksInRange)

	if allTicksInRange || player.cursor.RawPosition.Dst(sliderPosition) <= player.diff.GetRadius() {
		state.sliding = true
		state.slideStart = time

		if len(slider.players) == 1 {
			slider.hitSlider.InitSlide(float64(time))
		}
	}
}

func (slider *Slider) UpdateFor(player *difficultyPlayer, time int64, processSliderEndsAhead bool) bool {
	lazerMode := player.diff.IsLazer()

	if lazerMode {
		slider.processHeadMiss(player, time)
	}

	state := slider.state[player]

	if time != slider.lastSliderTime {
		slider.sliderPosition = slider.hitSlider.GetPositionAt(float64(time))
		slider.sliderPositionLazer = slider.hitSlider.PositionAtLazer(float64(time))
		slider.lastSliderTime = time
	}

	sliderPosition := slider.sliderPosition
	if lazerMode {
		sliderPosition = slider.sliderPositionLazer
	}

	sliderPosition = objects.ModifyPosition(slider.hitSlider.HitObject, sliderPosition, player.diff) // Calculate stacked position

	if time >= int64(slider.hitSlider.GetStartTime()) && ((!state.isHit && !lazerMode) || (lazerMode && state.isStartHit)) {
		mouseDownAcceptable := false
		mouseDownAcceptableSwap := player.gameDownState &&
			!(player.lastButton == (Left|Right) &&
				player.lastButton2 == player.mouseDownButton)

		if player.gameDownState {
			if state.downButton == Buttons(0) || (player.mouseDownButton != (Left|Right) && mouseDownAcceptableSwap) {
				state.downButton = Buttons(0)
				if player.leftCond {
					state.downButton = Left
				} else if player.rightCond {
					state.downButton = Right
				} else {
					state.downButton = player.mouseDownButton
				}

				mouseDownAcceptable = true
			} else if (player.mouseDownButton & state.downButton) > 0 {
				mouseDownAcceptable = true
			}
		} else {
			state.downButton = Buttons(0)
		}

		mouseDownAcceptable = mouseDownAcceptable || mouseDownAcceptableSwap || player.diff.CheckModActive(difficulty.Relax)

		allowable := mouseDownAcceptable
		radiusNeeded := player.diff.GetRadius()

		if lazerMode {
			if state.sliding {
				radiusNeeded *= 2.4
			}

			allowable = allowable && player.cursor.RawPosition.DstSq(sliderPosition) <= radiusNeeded*radiusNeeded
		} else {
			if state.sliding {
				radiusNeeded = math87.Mul87(radiusNeeded, 2.4)
			}

			allowable = allowable && player.cursor.RawPosition.DstSq87(sliderPosition) < math87.Mul87(radiusNeeded, radiusNeeded)
		}

		if allowable && !state.sliding {
			state.sliding = true
			state.slideStart = time

			if len(slider.players) == 1 {
				slider.hitSlider.InitSlide(float64(time))
			}
		}

		if lazerMode {
			slider.processTicksLazer(player, state, time, allowable)
		} else {
			slider.processTicksStable(player, state, time, allowable, sliderPosition, processSliderEndsAhead)
		}

		if !allowable && state.sliding && hasUnjudgedSliderEvents(state.points) {
			if len(slider.players) == 1 {
				slider.hitSlider.StopSlide()
			}

			state.sliding = false
		}
	}

	return true
}

// animateSliderEvent keeps rendering side effects separate from judgement and
// sample playback. Lazer may resolve several nested events in one frame, and
// the renderer must receive each event exactly once without using animation
// as a proxy for audio policy.
func (slider *Slider) animateSliderEvent(point sliderEvent, time int64) {
	if len(slider.players) != 1 {
		return
	}

	if point.hitResult.IsHit() {
		if point.kind == sliderPointTick {
			slider.hitSlider.AnimateSliderTick(float64(time))
		} else {
			slider.hitSlider.AnimateSliderPoint(point.edgeIndex, float64(time), true)
		}
		return
	}

	if point.kind != sliderPointTick {
		slider.hitSlider.AnimateSliderPoint(point.edgeIndex, float64(time), false)
	}

	// IgnoreMiss still represents a dropped tail in the visual state. Lazer
	// suppresses its score result, but the follow circle must not remain in its
	// pressed state after the slider has broken.
	slider.hitSlider.AnimateSliderBreak(float64(time))
}

func (slider *Slider) processTicksStable(player *difficultyPlayer, state *sliderstate, time int64, allowable bool, sliderPosition vector.Vector2f, processSliderEndsAhead bool) {
	// Stable replays intentionally retain the historical one-event-per-call
	// cadence and integer timestamps. The explicit event kind removes the old
	// repeat-index arithmetic without letting the Lazer batch-processing path
	// change Stable replay results.
	for index := range state.points {
		point := &state.points[index]

		if point.judged {
			continue
		}

		passed := point.time <= float64(time)
		if !passed && processSliderEndsAhead && index == len(state.points)-1 && point.time-float64(time) == 1 {
			passed = true
		}

		if !passed {
			break
		}
		if !allowable && float64(time) < point.time {
			// processSliderEndsAhead only makes the final point eligible for
			// the next frame's Stable processing. The historical path did not
			// turn that early, untracked point into a miss yet.
			return
		}

		point.judged = true
		point.hitResult = SliderMiss
		combo := Reset

		if allowable && float64(state.slideStart) <= point.time {
			point.hitResult = point.maxResult
			state.scored++
			combo = Increase
		} else {
			state.missed++
			if point.isTail() {
				combo = Hold
			}
		}

		slider.animateSliderEvent(*point, time)

		slider.ruleSet.SendResult(player.cursor, createSliderJudgementResult(point.hitResult, point.maxResult, combo, time, sliderPosition, slider, point.resultPart()))
		break
	}
}

func (slider *Slider) processTicksLazer(player *difficultyPlayer, state *sliderstate, time int64, allowable bool) {
	// Lazer resolves every event that is due in the current frame. This matters
	// for late frames: a cursor sample can retroactively resolve several ticks,
	// but a tail remains ordered behind all preceding events and can use its
	// separate early-release window.
	if !state.isStartHit {
		return
	}

	for index := range state.points {
		point := &state.points[index]

		if point.judged {
			continue
		}

		if !lazerSliderEventDue(*point, float64(time)) {
			break
		}

		previousEventsJudged := true
		for previousIndex := 0; previousIndex < index; previousIndex++ {
			if !state.points[previousIndex].judged {
				previousEventsJudged = false
				break
			}
		}

		if point.isTail() && !lazerSliderTailMayBeJudged(*point, float64(time), previousEventsJudged) {
			break
		}

		// Lazer does not judge a tail as a miss during its early leniency
		// window. It waits until the true end so a player can still regain
		// tracking and receive the tail hit.
		if point.isTail() && float64(time) < point.time && !allowable {
			break
		}

		point.judged = true
		point.hitResult = lazerSliderEventResult(*point, allowable)
		combo := Hold

		if point.hitResult.IsHit() {
			state.scored++
			combo = Increase

			if point.isTail() {
				state.endScored = true
				if player.classicNoSliderHeadAccuracy {
					combo = Hold
				}
			}
		} else if point.hitResult != IgnoreMiss {
			state.missed++
			if !point.isTail() {
				combo = Reset
			}
		}

		slider.animateSliderEvent(*point, time)

		if point.isTail() && point.hitResult.IsHit() && !player.classicAlwaysPlayTailSample && len(slider.players) == 1 {
			slider.hitSlider.PlayEdgeSample(point.edgeIndex)
			state.tailSamplePlayed = true
		}

		// A late frame can resolve several events at once. Keep the result
		// timestamp at the actual processing time, but place the result at the
		// event's path position so judgement markers do not bunch at the
		// cursor's current position.
		eventPosition := slider.hitSlider.GetStackedPositionAtModLazer(point.time, player.diff)
		slider.ruleSet.SendResult(player.cursor, createSliderJudgementResult(point.hitResult, point.maxResult, combo, time, eventPosition, slider, point.resultPart()))
	}
}

func (slider *Slider) UpdatePostFor(player *difficultyPlayer, time int64, processSliderEndsAhead bool) bool {
	state := slider.state[player]

	if !player.diff.IsLazer() {
		slider.processHeadMiss(player, time)
	}

	atEnd := float64(time) >= lazerSliderEndTime(slider.hitSlider)
	if !player.diff.IsLazer() {
		atEnd = time >= int64(slider.hitSlider.GetEndTime())
		atEnd = atEnd || (processSliderEndsAhead && int64(slider.hitSlider.GetEndTime())-time == 1)
	}

	if atEnd && !state.isHit {
		if len(slider.players) == 1 && !state.isStartHit && !player.diff.IsLazer() {
			slider.hitSlider.ArmStart(false, float64(time))
		}

		hitCount := state.scored
		if state.startResult.IsHit() {
			hitCount++
		}

		totalCount := len(state.points) + 1
		sliderResult := classicSliderCollapse(hitCount, totalCount)

		if len(slider.players) == 1 {
			lazerMode := player.diff.IsLazer()

			if !lazerMode {
				if sliderResult != Miss {
					slider.hitSlider.PlayEdgeSample(len(slider.hitSlider.TickReverse))
				}
			} else if player.classicAlwaysPlayTailSample && sliderResult != Miss {
				slider.hitSlider.PlayEdgeSample(len(slider.hitSlider.TickReverse))
			} else if state.endScored && !state.tailSamplePlayed && !player.classicAlwaysPlayTailSample {
				slider.hitSlider.PlayEdgeSample(len(slider.hitSlider.TickReverse))
			}
		}

		position := slider.hitSlider.GetStackedEndPositionMod(player.diff)

		if !player.diff.IsLazer() || player.classicNoSliderHeadAccuracy {
			combo := Reset
			if sliderResult != Miss {
				combo = Hold
				if player.diff.IsLazer() {
					// Classic Lazer sliders score their proportional parent result
					// in addition to the nested head and tick results. The parent
					// result is therefore a real combo increase, while Stable keeps
					// its historical summary-only behavior.
					combo = Increase
				}
			}

			slider.ruleSet.SendResult(player.cursor, createSliderJudgementResult(sliderResult, Hit300, combo, time, position, slider, sliderPartSummary))
		}

		state.isHit = true
	}

	return state.isHit
}

func (slider *Slider) processHeadMiss(player *difficultyPlayer, time int64) {
	state := slider.state[player]

	missDeadline := float64(int64(slider.hitSlider.GetStartTime()) + player.diff.Hit50)
	if player.diff.IsLazer() {
		missDeadline = slider.hitSlider.GetStartTime() + player.diff.Hit50U
	}

	if float64(time) > missDeadline && !state.isStartHit {
		if len(slider.players) == 1 && !state.isHit { //don't fade if slider already ended (and armed the start)
			slider.hitSlider.ArmStart(false, float64(time))
		}

		position := slider.hitSlider.GetStackedStartPositionMod(player.diff)

		hit, maxResult := stableSliderHeadResults(Miss)
		if player.diff.IsLazer() {
			hit, maxResult = lazerSliderHeadResults(player.classicNoSliderHeadAccuracy, Miss)
		}

		slider.ruleSet.SendResult(player.cursor, createSliderJudgementResult(hit, maxResult, Reset, time, position, slider, sliderPartHead))

		if player.leftCond {
			state.downButton = Left
		} else if player.rightCond {
			state.downButton = Right
		} else {
			state.downButton = player.mouseDownButton
		}

		state.isStartHit = true
		state.startResult = Miss
	}
}

func (slider *Slider) UpdatePost(_ int64) bool {
	numFinishedTotal := 0

	for _, player := range slider.players {
		state := slider.state[player]

		if !state.isHit || !state.isStartHit {
			numFinishedTotal++
		}
	}

	return numFinishedTotal == 0
}

func (slider *Slider) MissForcefully(player *difficultyPlayer, time int64) {
	state := slider.state[player]

	if !state.isStartHit {
		position := slider.hitSlider.GetStackedStartPositionMod(player.diff)

		if len(slider.players) == 1 {
			slider.hitSlider.HitEdge(0, float64(time), false)
		}

		hit, maxResult := stableSliderHeadResults(Miss)
		if player.diff.IsLazer() {
			hit, maxResult = lazerSliderHeadResults(player.classicNoSliderHeadAccuracy, Miss)
		}

		slider.ruleSet.SendResult(player.cursor, createSliderJudgementResult(hit, maxResult, Reset, time, position, slider, sliderPartHead))

		state.isStartHit = true
		state.startResult = Miss
	}
}

func (slider *Slider) IsHit(pl *difficultyPlayer) bool {
	return slider.state[pl].isHit
}

func (slider *Slider) IsStartHit(pl *difficultyPlayer) bool {
	return slider.state[pl].isStartHit
}

func (slider *Slider) GetStartResult(pl *difficultyPlayer) HitResult {
	return slider.state[pl].startResult
}

func (slider *Slider) GetFadeTime() int64 {
	return int64(slider.hitSlider.GetStartTime() - slider.fadeStartRelative)
}

func (slider *Slider) GetObject() objects.IHitObject {
	return slider.hitSlider
}
