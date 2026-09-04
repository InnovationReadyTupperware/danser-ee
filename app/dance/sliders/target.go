package sliders

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

// Target is the movement representation of one authored slider.
type Target struct {
	*objects.Slider

	plan    *motionPlan
	diff    *difficulty.Difficulty
	endTime float64
}

// NewTarget plans movement for one traversable slider.
func NewTarget(slider *objects.Slider, diff *difficulty.Difficulty) *Target {
	return &Target{
		Slider:  slider,
		plan:    newMotionPlan(slider, diff),
		diff:    diff,
		endTime: objects.GetEndTimeForDiff(slider, diff),
	}
}

// NewHeadTarget returns one hit-note target at the slider head.
func NewHeadTarget(slider *objects.Slider, diff *difficulty.Difficulty) *objects.Circle {
	target := objects.DummyCircle(slider.PositionAtForDiff(slider.GetStartTime(), diff), slider.GetStartTime())
	target.StackLeniency = slider.StackLeniency
	target.StackIndexMap = slider.StackIndexMap
	target.SetComboSet(slider.GetComboSet())
	target.SetComboSetHax(slider.GetComboSetHax())
	target.SetComboNumber(slider.ComboNumber)
	target.SetNewCombo(slider.IsNewCombo())
	target.SetLastInCombo(slider.IsLastCombo())
	return target
}

func (target *Target) GetEndTime() float64 {
	return target.endTime
}

func (target *Target) GetDuration() float64 {
	return target.endTime - target.GetStartTime()
}

func (target *Target) GetPositionAt(time float64) vector.Vector2f {
	return target.plan.positionAt(time)
}

func (target *Target) GetStackedPositionAtMod(time float64, _ *difficulty.Difficulty) vector.Vector2f {
	return target.GetPositionAt(time)
}

func (target *Target) GetStackedStartPositionMod(_ *difficulty.Difficulty) vector.Vector2f {
	if target.diff == nil {
		return target.Slider.GetStartPosition()
	}

	return target.Slider.GetStackedStartPositionMod(target.diff)
}

func (target *Target) GetStackedEndPositionMod(_ *difficulty.Difficulty) vector.Vector2f {
	return target.GetPositionAt(target.endTime)
}

func (target *Target) GetStartAngleMod(_ *difficulty.Difficulty) float32 {
	start := target.GetPositionAt(target.GetStartTime())
	next := target.GetPositionAt(min(target.endTime, target.GetStartTime()+1))
	return start.AngleRV(next)
}

func (target *Target) GetEndAngleMod(_ *difficulty.Difficulty) float32 {
	end := target.GetPositionAt(target.endTime)
	previous := target.GetPositionAt(max(target.GetStartTime(), target.endTime-1))
	return end.AngleRV(previous)
}
