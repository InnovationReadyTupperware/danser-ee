package dance

import (
	"fmt"
	"strings"
	"time"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/dance/movers"
	"github.com/innovationreadytupperware/danser-ee/app/dance/schedulers"
	"github.com/innovationreadytupperware/danser-ee/app/dance/spinners"
	"github.com/innovationreadytupperware/danser-ee/app/dance/utils"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

type Controller interface {
	SetBeatMap(beatMap *beatmap.BeatMap)
	InitCursors()
	Update(time float64, delta float64)
	GetCursors() []*graphics.Cursor
}

// RulesetController exposes scoring for controllers that own an osu! ruleset.
// Visual cursor-dance controllers may intentionally leave this unavailable.
type RulesetController interface {
	Controller
	GetRuleset() *osu.OsuRuleSet
}

// GeneratedCursorController identifies cursors whose input was produced by
// Danser rather than read from a replay or from the user.
type GeneratedCursorController interface {
	IsGeneratedCursor(cursor *graphics.Cursor) bool
}

// KnockoutController is the common data and scoring surface consumed by the
// knockout overlay. Replay-backed and generated participants deliberately
// implement the same interface so the overlay does not need a second copy of
// its ranking, failure, and animation logic.
type KnockoutController interface {
	RulesetController
	GetBeatMap() *beatmap.BeatMap
	GetClick(player, key int) bool
	GetReplays() []RpData
}

// InitialSeeker can initialize a controller at a later timeline position
// without replaying every millisecond before it. It is deliberately optional:
// generated cursor-dance movement can be reconstructed from the target time,
// while real replay and player controllers must retain their event-by-event
// processing for scoring and input correctness.
type InitialSeeker interface {
	Seek(time float64) bool
}

type GenericController struct {
	bMap             *beatmap.BeatMap
	cursors          []*graphics.Cursor
	schedulers       []schedulers.Scheduler
	generatedCursors map[*graphics.Cursor]struct{}
	ruleset          *osu.OsuRuleSet
	replays          []RpData
	knockout         bool
	lastTime         float64
}

func NewGenericController() Controller {
	return &GenericController{}
}

// NewSoloKnockoutController creates generated Danser participants that are
// scored and managed by the knockout overlay. Movement, object distribution,
// spinner behavior, and seeking remain owned by GenericController so the
// solo mode follows cursor-dance configuration without duplicating it.
func NewSoloKnockoutController() *GenericController {
	return &GenericController{
		knockout: true,
		lastTime: -200,
	}
}

func (controller *GenericController) SetBeatMap(beatMap *beatmap.BeatMap) {
	controller.bMap = beatMap
}

func (controller *GenericController) InitCursors() {
	settings.NormalizeCursorDance()
	participantCount := max(1, settings.TAG)
	settings.TAG = participantCount

	controller.ruleset = nil
	controller.replays = nil
	controller.generatedCursors = nil
	controller.lastTime = -200
	if controller.knockout {
		controller.generatedCursors = make(map[*graphics.Cursor]struct{}, participantCount)
		settings.PLAYERS = participantCount
	}

	controller.cursors = make([]*graphics.Cursor, participantCount)
	controller.schedulers = make([]schedulers.Scheduler, participantCount)

	counter := make(map[string]int)

	// Mover initialization
	for i := range controller.cursors {
		controller.cursors[i] = graphics.NewCursor()
		controller.cursors[i].IsCursorDance = true
		if controller.knockout {
			controller.cursors[i].IsPlayer = true
			controller.cursors[i].IsAutoplay = true
			controller.cursors[i].Name = generatedParticipantName(i)
			controller.cursors[i].ScoreTime = time.Now()
			controller.cursors[i].ScoreID = -1
			controller.generatedCursors[controller.cursors[i]] = struct{}{}
		}

		mover := "flower"
		if len(settings.CursorDance.Movers) > 0 {
			mover = strings.ToLower(settings.CursorDance.Movers[i%len(settings.CursorDance.Movers)].Mover)
		}

		moverCtor, mName := movers.GetMoverCtorByName(mover)

		controller.schedulers[i] = schedulers.NewGenericScheduler(moverCtor, i, counter[mName])

		counter[mName]++
	}

	type Queue struct {
		hitObjects []objects.IHitObject
	}

	queues := make([]Queue, participantCount)

	queue := controller.bMap.GetObjectsCopy()

	// Treat pathological and singular sliders as one normal hit-note target.
	// Their authored slider data remains owned by the gameplay object, but
	// generated cursors must not traverse an unstable or disproportionate path.
	for i := range queue {
		if s, ok := queue[i].(*objects.Slider); ok && (s.IsPathological() || s.IsSingular()) {
			queue = utils.PreprocessQueueForDiff(i, queue, true, controller.bMap.Diff)
		}
	}

	// Convert sliders to pseudo-circles for tag cursors
	if !settings.CursorDance.ComboTag && !settings.CursorDance.Battle &&
		settings.CursorDance.TAGSliderDance && participantCount > 1 {
		queue = utils.ExpandSliderDanceQueueForDiff(queue, controller.bMap.Diff)
	}

	if !settings.CursorDance.Resolve2BAfterTAG {
		queue = utils.Solve2BForDiff(queue, controller.bMap.Diff)
	}

	// Solo knockout scores every generated cursor as a complete participant.
	// Visual TAG playback can partition objects because it has no per-cursor
	// score, but a knockout participant must see the whole map or it would be
	// judged for objects that its movement scheduler never visits. The shared
	// cursor-dance mover, slider, and spinner processing remains unchanged.
	for j, o := range queue {
		_, isSpinner := o.(*objects.Spinner)

		if controller.knockout || (isSpinner && settings.CursorDance.DoSpinnersTogether) || settings.CursorDance.Battle {
			for i := range queues {
				queues[i].hitObjects = append(queues[i].hitObjects, o)
			}
		} else if settings.CursorDance.ComboTag {
			i := int(o.GetComboSet()) % participantCount
			queues[i].hitObjects = append(queues[i].hitObjects, o)
		} else {
			i := j % participantCount
			queues[i].hitObjects = append(queues[i].hitObjects, o)
		}
	}

	// Initialize spinner movers after the profile resolver has captured each
	// cursor's independent shape and center configuration.
	for i := range controller.cursors {
		spinnerProfile := spinners.ResolveSpinnerProfile(i)
		controller.schedulers[i].Init(queues[i].hitObjects, controller.bMap.Diff, controller.cursors[i], spinners.GetMoverCtor(spinnerProfile), true)
	}

	if controller.knockout {
		diffs := make([]*difficulty.Difficulty, participantCount)
		controller.replays = make([]RpData, participantCount)

		for i := range controller.cursors {
			diff := controller.bMap.Diff.Clone()
			diffs[i] = diff
			controller.replays[i] = RpData{
				RawName:   controller.cursors[i].Name,
				Name:      controller.cursors[i].Name,
				Mods:      diff.GetModString(),
				ModsV:     diff.Mods,
				Accuracy:  1,
				Grade:     osu.NONE,
				scoreID:   -1,
				ScoreTime: controller.cursors[i].ScoreTime,
			}
		}

		controller.ruleset = osu.NewOsuRuleset(controller.bMap, controller.cursors, diffs)
	}
}

func (controller *GenericController) Update(time float64, delta float64) {
	if controller.ruleset != nil {
		numSkipped := int(time) - int(controller.lastTime) - 1
		if controller.lastTime >= 0 && numSkipped >= 1 {
			for nTime := numSkipped; nTime >= 1; nTime-- {
				controller.updateAt(time-float64(nTime), 1)
			}
		}

		controller.updateAt(time, delta)
		controller.updateScores()

		return
	}

	controller.updateCursors(time, delta)
}

func (controller *GenericController) updateAt(time float64, delta float64) {
	controller.updateCursors(time, delta)

	if controller.ruleset == nil {
		return
	}

	for _, cursor := range controller.cursors {
		if int64(time)%17 == 0 {
			cursor.LastFrameTime = int64(time) - 17
			cursor.CurrentFrameTime = int64(time)
			cursor.IsInputFrame = true
			cursor.IsReplayFrame = true
		} else {
			cursor.IsInputFrame = false
			cursor.IsReplayFrame = false
		}

		if int64(time) != int64(controller.lastTime) {
			controller.ruleset.UpdateClickFor(cursor, int64(time))
			controller.ruleset.UpdateNormalFor(cursor, int64(time), false)
			controller.ruleset.UpdatePostFor(cursor, int64(time), false)
		}
	}

	if int64(time) != int64(controller.lastTime) {
		controller.ruleset.Update(int64(time))
	}

	controller.lastTime = time
}

func (controller *GenericController) updateCursors(time float64, delta float64) {
	for i := range controller.cursors {
		controller.schedulers[i].Update(time)
		controller.cursors[i].Update(delta)

		controller.cursors[i].LeftButton = controller.cursors[i].LeftKey || controller.cursors[i].LeftMouse
		controller.cursors[i].RightButton = controller.cursors[i].RightKey || controller.cursors[i].RightMouse
	}
}

func (controller *GenericController) updateScores() {
	if controller.ruleset == nil {
		return
	}

	for i, cursor := range controller.cursors {
		score := controller.ruleset.GetScore(cursor)
		controller.replays[i].Accuracy = score.Accuracy
		controller.replays[i].Combo = int64(score.Combo)
		controller.replays[i].Grade = score.Grade
	}
}

// Seek advances generated cursor-dance schedulers directly to time. Their
// movement is a pure function of the current object window, so reconstructing
// every skipped millisecond would only add startup latency without improving
// the rendered result. Scored solo-knockout controllers additionally rebuild
// the ruleset's timeline state under catch-up semantics.
func (controller *GenericController) Seek(time float64) bool {
	for i := range controller.cursors {
		controller.schedulers[i].Seek(time)
		controller.cursors[i].Update(0)

		controller.cursors[i].LeftButton = controller.cursors[i].LeftKey || controller.cursors[i].LeftMouse
		controller.cursors[i].RightButton = controller.cursors[i].RightKey || controller.cursors[i].RightMouse
	}

	if controller.ruleset != nil {
		previousCatchUp := controller.ruleset.SetCatchUp(true)
		defer controller.ruleset.SetCatchUp(previousCatchUp)

		controller.bMap.Update(time)
		controller.ruleset.Update(int64(time))

		for _, cursor := range controller.cursors {
			cursor.IsInputFrame = false
			cursor.IsReplayFrame = false
			controller.ruleset.UpdateClickFor(cursor, int64(time))
			controller.ruleset.UpdateNormalFor(cursor, int64(time), false)
			controller.ruleset.UpdatePostFor(cursor, int64(time), false)
		}

		controller.ruleset.Update(int64(time))
		controller.updateScores()
	}

	controller.lastTime = time

	return true
}

func (controller *GenericController) GetCursors() []*graphics.Cursor {
	return controller.cursors
}

func generatedParticipantName(index int) string {
	base := strings.TrimSpace(settings.Knockout.DanserName)
	if base == "" {
		base = "danser"
	}

	if index == 0 {
		return base
	}

	return fmt.Sprintf("%s %d", base, index+1)
}

// GetBeatMap implements KnockoutController.
func (controller *GenericController) GetBeatMap() *beatmap.BeatMap {
	return controller.bMap
}

// GetReplays implements KnockoutController. The name is retained for the
// overlay's existing ranking model; generated participants expose the same
// score metadata without pretending that an .osr file exists.
func (controller *GenericController) GetReplays() []RpData {
	return controller.replays
}

// GetRuleset implements KnockoutController.
func (controller *GenericController) GetRuleset() *osu.OsuRuleSet {
	return controller.ruleset
}

// GetClick implements KnockoutController.
func (controller *GenericController) GetClick(player, key int) bool {
	if player < 0 || player >= len(controller.cursors) {
		return false
	}

	cursor := controller.cursors[player]
	switch key {
	case 0:
		return cursor.LeftKey
	case 1:
		return cursor.RightKey
	case 2:
		return cursor.LeftMouse
	case 3:
		return cursor.RightMouse
	default:
		return false
	}
}

// IsGeneratedCursor reports ownership for failure routing. Ordinary visual
// cursor dance has no ruleset and therefore never exposes generated knockout
// participants.
func (controller *GenericController) IsGeneratedCursor(cursor *graphics.Cursor) bool {
	_, ok := controller.generatedCursors[cursor]
	return ok
}
