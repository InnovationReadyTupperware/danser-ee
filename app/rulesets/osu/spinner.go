package osu

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/math32"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const FrameTime = 1000.0 / 60

type spinnerState struct {
	// Shared completion and lifecycle state.
	requirement       int64
	rotationCount     int64
	lastRotationCount int64
	rotationCountF    float32
	finished          bool
	updatedBefore     bool

	// Stable's angle/velocity tracker and legacy scoring state.
	lastAngle            float64
	scoringRotationCount int64
	rotationCountFD      float64
	frameVariance        float64
	theoreticalVelocity  float64
	currentVelocity      float64
	zeroCount            int64
	displayRPM           float64

	// Lazer's direction-aware spin history and visual rotation state.
	lastAngle32                              float32
	rotationCountFPrev                       float32
	totalAccumulatedRotation                 float32
	currentSpinMaxRotation                   float32
	totalAccumulatedRotationAtLastCompletion float32
	maximumBonusSpins                        int64
	lazerRPMMeter                            difficulty.SpinnerRPMMeter
	cursorDanceRPMRamp                       difficulty.SpinnerRPMRamp
	lastTime                                 int64
	hasLastTime                              bool
}

func (s *spinnerState) currentSpinRotation() float32 {
	return s.totalAccumulatedRotation - s.totalAccumulatedRotationAtLastCompletion
}

func (s *spinnerState) totalRotation() float32 {
	return 360*float32(s.rotationCount) + s.currentSpinMaxRotation
}

func (s *spinnerState) getCompletion() float32 {
	if s.requirement <= 0 {
		// osu!lazer treats a spinner with no required rotations as complete.
		// Returning one also keeps the HUD and post-judgement paths finite.
		return 1
	}

	completion := s.totalRotation() / 360 / float32(s.requirement)
	if math.IsNaN(float64(completion)) {
		return 0
	}
	if math.IsInf(float64(completion), 1) {
		return 1
	}

	return completion
}

type Spinner struct {
	ruleSet           *OsuRuleSet
	hitSpinner        *objects.Spinner
	players           []*difficultyPlayer
	state             map[*difficultyPlayer]*spinnerState
	fadeStartRelative float64
	maxAcceleration   float64
}

func (spinner *Spinner) GetNumber() int64 {
	return spinner.hitSpinner.GetID()
}

func (spinner *Spinner) Init(ruleSet *OsuRuleSet, object objects.IHitObject, players []*difficultyPlayer) {
	spinner.ruleSet = ruleSet
	spinner.hitSpinner = object.(*objects.Spinner)
	spinner.players = players
	spinner.state = make(map[*difficultyPlayer]*spinnerState)

	rSpinner := spinner.hitSpinner

	spinnerTime := int64(rSpinner.GetEndTime()) - int64(rSpinner.GetStartTime())

	spinner.fadeStartRelative = 100000

	for _, player := range spinner.players {
		spinner.state[player] = new(spinnerState)
		spinner.fadeStartRelative = min(spinner.fadeStartRelative, player.diff.Preempt)
		spinner.state[player].frameVariance = FrameTime

		requirements := calculateSpinnerRequirements(player.diff, rSpinner.GetStartTime(), rSpinner.GetEndTime())
		spinner.state[player].requirement = requirements.required
		spinner.state[player].maximumBonusSpins = requirements.maximumBonus
	}

	spinner.maxAcceleration = 0.00008 + max(0, (5000-float64(spinnerTime))/1000/2000)
}

func (spinner *Spinner) UpdateClickFor(*difficultyPlayer, int64) bool {
	return true
}

func (spinner *Spinner) UpdateFor(player *difficultyPlayer, time int64, _ bool) bool {
	state := spinner.state[player]

	if !state.finished {
		if player.diff.IsLazer() {
			spinner.processLazer(player, time)
		} else {
			spinner.processStable(player, time)
		}
	}

	return state.finished
}

func (spinner *Spinner) processStable(player *difficultyPlayer, time int64) {
	spinnerPosition := spinner.hitSpinner.GetStartPosition()

	state := spinner.state[player]

	timeDiff := float64(time - player.cursor.LastFrameTime)
	if player.cursor.LastFrameTime == 0 {
		timeDiff = FrameTime
	}

	if player.cursor.IsReplayFrame && time > int64(spinner.hitSpinner.GetStartTime()) && time < int64(spinner.hitSpinner.GetEndTime()) {
		maxAccelThisFrame := player.diff.GetModifiedTime(spinner.maxAcceleration * timeDiff)

		if player.diff.CheckModActive(difficulty.SpunOut) || player.diff.CheckModActive(difficulty.Relax2) {
			state.currentVelocity = 0.03
		} else if state.theoreticalVelocity > state.currentVelocity {
			accel := maxAccelThisFrame
			if state.currentVelocity < 0 && player.diff.CheckModActive(difficulty.Relax) {
				accel /= 4
			}

			state.currentVelocity += min(state.theoreticalVelocity-state.currentVelocity, accel)
		} else {
			accel := -maxAccelThisFrame
			if state.currentVelocity > 0 && player.diff.CheckModActive(difficulty.Relax) {
				accel /= 4
			}

			state.currentVelocity += max(state.theoreticalVelocity-state.currentVelocity, accel)
		}

		state.currentVelocity = max(-0.05, min(state.currentVelocity, 0.05))

		if len(spinner.players) == 1 {
			if state.currentVelocity == 0 {
				spinner.hitSpinner.PauseSpinSample()
			} else {
				spinner.hitSpinner.StartSpinSample(float64(time))
			}
		}

		decay1 := math.Pow(0.9, timeDiff/FrameTime)
		state.displayRPM = state.displayRPM*decay1 + (1.0-decay1)*(math.Abs(state.currentVelocity)*1000)/(math.Pi*2)*60

		mouseAngle := float64(player.cursor.RawPosition.Sub(spinnerPosition).AngleR())

		if !player.cursor.OldSpinnerScoring && !state.updatedBefore {
			state.lastAngle = mouseAngle
			state.updatedBefore = true
		}

		angleDiff := mouseAngle - state.lastAngle

		if mouseAngle-state.lastAngle < -math.Pi {
			angleDiff = (2 * math.Pi) + mouseAngle - state.lastAngle
		} else if state.lastAngle-mouseAngle < -math.Pi {
			angleDiff = (-2 * math.Pi) - state.lastAngle + mouseAngle
		}

		decay := math.Pow(0.999, timeDiff)
		state.frameVariance = decay*state.frameVariance + (1-decay)*timeDiff

		if angleDiff == 0 {
			state.zeroCount += 1

			if state.zeroCount < 2 {
				state.theoreticalVelocity /= 3
			} else {
				state.theoreticalVelocity = 0
			}
		} else {
			state.zeroCount = 0

			if (!player.gameDownState && !player.diff.CheckModActive(difficulty.Relax)) || time < int64(spinner.hitSpinner.GetStartTime()) || time > int64(spinner.hitSpinner.GetEndTime()) {
				angleDiff = 0
			}

			if math.Abs(angleDiff) < math.Pi {
				if player.diff.GetModifiedTime(state.frameVariance) > FrameTime*1.04 {
					if timeDiff > 0 {
						state.theoreticalVelocity = angleDiff / player.diff.GetModifiedTime(timeDiff)
					} else {
						state.theoreticalVelocity = 0
					}
				} else {
					state.theoreticalVelocity = angleDiff / FrameTime
				}
			} else {
				state.theoreticalVelocity = 0
			}
		}

		state.lastAngle = mouseAngle

		rotationAddition := state.currentVelocity * timeDiff

		state.rotationCountFD += rotationAddition
		state.rotationCountF += float32(math.Abs(float64(float32(rotationAddition)) / math.Pi))

		if len(spinner.players) == 1 {
			spinner.hitSpinner.SetRotation(player.diff.GetModifiedTime(state.rotationCountFD))
			spinner.hitSpinner.SetRPM(state.displayRPM)
			completion := float64(0)
			if state.requirement > 0 {
				completion = float64(state.rotationCountF) / float64(state.requirement)
			}
			spinner.hitSpinner.UpdateCompletion(completion)
		}

		state.rotationCount = int64(state.rotationCountF)

		if state.rotationCount != state.lastRotationCount {
			state.scoringRotationCount++

			if state.scoringRotationCount == spinner.getRequirementClear(player) && len(spinner.players) == 1 {
				spinner.hitSpinner.Clear()
			}

			if state.scoringRotationCount > state.requirement+3 && (state.scoringRotationCount-(state.requirement+3))%2 == 0 {
				if len(spinner.players) == 1 {
					spinner.hitSpinner.Bonus(1000, time)
				}

				spinner.ruleSet.SendResult(player.cursor, createJudgementResult(SpinnerBonus, SpinnerBonus, Hold, time, spinnerPosition, spinner))
			} else if state.scoringRotationCount > 1 && state.scoringRotationCount%2 == 0 {
				spinner.ruleSet.SendResult(player.cursor, createJudgementResult(SpinnerPoints, SpinnerPoints, Hold, time, spinnerPosition, spinner))
			} else if state.scoringRotationCount > 1 {
				spinner.ruleSet.SendResult(player.cursor, createJudgementResult(SpinnerSpin, SpinnerSpin, Hold, time, spinnerPosition, spinner))
			}

			state.lastRotationCount = state.rotationCount
		}
	}
}

func (spinner *Spinner) processLazer(player *difficultyPlayer, time int64) {
	spinnerPosition := spinner.hitSpinner.GetStartPosition()

	state := spinner.state[player]

	rewound := state.hasLastTime && time < state.lastTime
	timeDiff := 0.0
	if state.hasLastTime && !rewound {
		timeDiff = float64(time - state.lastTime)
	}

	if rewound {
		// The display meters are timeline-local. Reset them and discard the
		// previous angle so seeking cannot turn a cursor jump into a fake spin
		// or retain a rate from the abandoned future.
		state.lazerRPMMeter.Reset()
		state.cursorDanceRPMRamp.Reset()
		state.displayRPM = 0
		state.updatedBefore = false
	}

	state.lastTime = time
	state.hasLastTime = true

	if time >= int64(spinner.hitSpinner.GetStartTime()) && time <= int64(spinner.hitSpinner.GetEndTime()) {
		var delta float32 = 0.0

		thisAngle := player.cursor.RawPosition.Sub(spinnerPosition).Angle()

		if state.updatedBefore && !rewound {
			delta = thisAngle - state.lastAngle32
		}

		state.lastAngle32 = thisAngle

		if delta > 180 {
			delta -= 360
		}

		if delta < -180 {
			delta += 360
		}

		if player.diff.CheckModActive(difficulty.SpunOut) {
			rotationSpeed := float32(0)
			duration := spinner.hitSpinner.GetEndTime() - spinner.hitSpinner.GetStartTime()
			if duration > 0 && state.requirement > 0 {
				rotationSpeed = float32(1.01 * float64(state.requirement) / duration)
			}

			delta = float32(timeDiff) * rotationSpeed * 360

			reportLazerRotationDelta(state, delta)
		} else if !rewound && (player.gameDownState || player.diff.CheckModActive(difficulty.Relax)) {
			delta *= float32(player.diff.GetSpeed())

			reportLazerRotationDelta(state, delta)
		}

		state.updatedBefore = true

		spinning := mutils.Abs(state.rotationCountF-state.rotationCountFPrev) > 10

		state.rotationCountFPrev = mutils.Lerp(state.rotationCountFPrev, state.rotationCountF, 1-math32.Pow(0.99, float32(player.diff.GetModifiedTime(timeDiff))))

		if len(spinner.players) == 1 {
			if spinning {
				spinner.hitSpinner.StartSpinSample(float64(time))
			} else {
				spinner.hitSpinner.PauseSpinSample()
			}
		}

		if len(spinner.players) == 1 {
			state.displayRPM = state.lazerRPMMeter.Update(float64(time), float64(state.totalRotation()))

			if player.cursor.IsCursorDance {
				targetRPM := spinner.hitSpinner.GetAutoplayRPM()
				if targetRPM > 0 && !math.IsNaN(targetRPM) && !math.IsInf(targetRPM, 0) {
					state.displayRPM = state.cursorDanceRPMRamp.Update(float64(time), targetRPM)
				}
			}

			spinner.hitSpinner.SetRotation(float64(state.rotationCountFPrev * math32.Pi / 180))
			spinner.hitSpinner.SetRPM(state.displayRPM)
			spinner.hitSpinner.UpdateCompletion(float64(state.getCompletion()))
		}

		totalSpins := state.maximumBonusSpins + state.requirement + difficulty.LazerSpinBonusGap

		for i := state.lastRotationCount; i < state.rotationCount; i++ {
			if i == state.requirement && len(spinner.players) == 1 {
				spinner.hitSpinner.Clear()
			}

			if i < totalSpins {
				if i < state.requirement+difficulty.LazerSpinBonusGap {
					spinner.ruleSet.SendResult(player.cursor, createJudgementResult(SpinnerPoints, SpinnerPoints, Hold, time, spinnerPosition, spinner))
				} else {
					if len(spinner.players) == 1 {
						spinner.hitSpinner.Bonus(int(SpinnerBonus.ScoreValueFor(player.diff.GetGameplayMode(), player.diff.Mods)), time)
					}

					spinner.ruleSet.SendResult(player.cursor, createJudgementResult(SpinnerBonus, SpinnerBonus, Hold, time, spinnerPosition, spinner))
				}
			} else {
				if len(spinner.players) == 1 {
					spinner.hitSpinner.Bonus(0, time)
				}
			}
		}

		state.lastRotationCount = state.rotationCount
	}
}

// reportLazerRotationDelta mirrors osu!lazer's direction-aware spin history:
// a reversal can reduce the current spin without erasing completed spins.
func reportLazerRotationDelta(state *spinnerState, delta float32) {
	if math.IsNaN(float64(delta)) || math.IsInf(float64(delta), 0) {
		return
	}

	if delta != 0 {
		state.totalAccumulatedRotation += delta

		state.currentSpinMaxRotation = max(state.currentSpinMaxRotation, mutils.Abs(state.currentSpinRotation()))

		// Handle the case where the user has completed another spin.
		// This could be an if rather than a while if one frame could never cross
		// more than one spin. Keep the loop because replay stepping and tests can
		// legitimately provide a larger delta.
		for state.currentSpinMaxRotation >= 360 {
			direction := mutils.Signum(state.currentSpinRotation())

			state.rotationCount++

			// Incrementing the last completion point will cause `currentSpinRotation` to
			// hold the remaining spin that needs to be considered.
			state.totalAccumulatedRotationAtLastCompletion += float32(direction) * 360

			// Reset the current max as we are entering a new spin.
			// Importantly, carry over the remainder (which is now stored in `currentSpinRotation`).
			state.currentSpinMaxRotation = mutils.Abs(state.currentSpinRotation())
		}
	}

	state.rotationCountF += delta
}

func (spinner *Spinner) UpdatePostFor(player *difficultyPlayer, time int64, _ bool) bool {
	state := spinner.state[player]

	if time >= int64(spinner.hitSpinner.GetEndTime()) && !state.finished {
		hit := Miss
		combo := Reset

		if player.diff.IsLazer() {
			completion := state.getCompletion()
			if state.requirement == 0 || completion >= 1.0 {
				hit = Hit300
			} else if completion >= 0.9 {
				hit = Hit100
			} else if completion >= 0.75 {
				hit = Hit50
			}
		} else {
			if (!player.cursor.OldSpinnerScoring && spinner.state[player].requirement == 0) || state.scoringRotationCount >= spinner.getRequirementGreat(player) {
				hit = Hit300
			} else if state.scoringRotationCount >= spinner.getRequirementOk(player) {
				hit = Hit100
			} else if state.scoringRotationCount >= spinner.getRequirementMeh(player) {
				hit = Hit50
			}
		}

		if hit != Miss {
			combo = Increase
		}

		if len(spinner.players) == 1 {
			spinner.hitSpinner.StopSpinSample()
			spinner.hitSpinner.Hit(float64(time), hit != Miss)
		}

		spinner.ruleSet.SendResult(player.cursor, createJudgementResult(hit, Hit300, combo, time, spinner.hitSpinner.GetPosition(), spinner))

		state.finished = true
	}

	return state.finished
}

func (spinner *Spinner) UpdatePost(_ int64) bool {
	numFinishedTotal := 0

	for _, player := range spinner.players {
		state := spinner.state[player]

		if !state.finished {
			numFinishedTotal++
		}
	}

	return numFinishedTotal == 0
}

func (spinner *Spinner) MissForcefully(_ *difficultyPlayer, _ int64) {
}

func (spinner *Spinner) IsHit(pl *difficultyPlayer) bool {
	return spinner.state[pl].finished
}

func (spinner *Spinner) GetFadeTime() int64 {
	return int64(spinner.hitSpinner.GetStartTime() - spinner.fadeStartRelative)
}

func (spinner *Spinner) GetObject() objects.IHitObject {
	return spinner.hitSpinner
}

// new vs old spinner handling helpers
func (spinner *Spinner) getRequirementMeh(player *difficultyPlayer) int64 {
	if player.cursor.OldSpinnerScoring {
		return spinner.state[player].requirement
	}

	return spinner.state[player].requirement / 4
}

func (spinner *Spinner) getRequirementOk(player *difficultyPlayer) int64 {
	if player.cursor.OldSpinnerScoring {
		return spinner.state[player].requirement + 1
	}

	return spinner.state[player].requirement - 1
}

func (spinner *Spinner) getRequirementGreat(player *difficultyPlayer) int64 {
	if player.cursor.OldSpinnerScoring {
		return spinner.state[player].requirement + 2
	}

	return spinner.state[player].requirement + 1
}

func (spinner *Spinner) getRequirementClear(player *difficultyPlayer) int64 {
	if player.cursor.OldSpinnerScoring {
		return spinner.state[player].requirement + 1
	}

	return spinner.state[player].requirement
}
