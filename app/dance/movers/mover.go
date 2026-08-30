package movers

import (
	"math"
	"strings"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/framework/math/vector"
)

const sixtyTime = 1000.0 / 60

type MultiPointMover interface {
	Reset(diff *difficulty.Difficulty, id int)
	SetObjects(objs []objects.IHitObject) int
	Update(time float64) vector.Vector2f
	GetObjectsStartTime(object objects.IHitObject) float64
	GetObjectsEndTime(object objects.IHitObject) float64
	GetObjectsStartPosition(object objects.IHitObject) vector.Vector2f
	GetObjectsEndPosition(object objects.IHitObject) vector.Vector2f
	GetObjectsPosition(time float64, object objects.IHitObject) vector.Vector2f
	GetStartTime() float64
	GetEndTime() float64
}

type basicMover struct {
	startTime float64
	endTime   float64

	id int

	diff *difficulty.Difficulty
}

func (mover *basicMover) Reset(diff *difficulty.Difficulty, id int) {
	mover.diff = diff
	mover.id = id
}

func (mover *basicMover) GetObjectsStartTime(object objects.IHitObject) float64 {
	return object.GetStartTime()
}

func (mover *basicMover) GetObjectsEndTime(object objects.IHitObject) float64 {
	return object.GetEndTime()
}

func (mover *basicMover) GetObjectsStartPosition(object objects.IHitObject) vector.Vector2f {
	return object.GetStackedStartPositionMod(mover.diff)
}

func (mover *basicMover) GetObjectsEndPosition(object objects.IHitObject) vector.Vector2f {
	return object.GetStackedEndPositionMod(mover.diff)
}

func (mover *basicMover) GetObjectsPosition(time float64, object objects.IHitObject) vector.Vector2f {
	return object.GetStackedPositionAtMod(time, mover.diff)
}

func (mover *basicMover) GetStartTime() float64 {
	return mover.startTime
}

func (mover *basicMover) GetEndTime() float64 {
	return mover.endTime
}

// movementProgress keeps generated movers finite when a malformed or dense
// object queue produces a zero-length movement window. Valid positive windows
// use the same clamp behavior as the original movers.
func movementProgress(time, start, end float64) float32 {
	if math.IsNaN(time) || math.IsInf(time, 0) || math.IsNaN(start) || math.IsInf(start, 0) || math.IsNaN(end) || math.IsInf(end, 0) {
		return 0
	}
	if end <= start {
		if time >= end {
			return 1
		}
		return 0
	}

	progress := (time - start) / (end - start)
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}
	return float32(progress)
}

func GetMoverByName(name string) MultiPointMover {
	ctor, _ := GetMoverCtorByName(name)

	return ctor()
}

func GetMoverCtorByName(name string) (moverCtor func() MultiPointMover, finalName string) {
	finalName = strings.ToLower(name)

	switch finalName {
	case "spline":
		moverCtor = NewSplineMover
	case "bezier":
		moverCtor = NewBezierMover
	case "circular":
		moverCtor = NewHalfCircleMover
	case "linear":
		moverCtor = NewLinearMover
	case "axis":
		moverCtor = NewAxisMover
	case "exgon":
		moverCtor = NewExGonMover
	case "aggressive":
		moverCtor = NewAggressiveMover
	case "momentum":
		moverCtor = NewMomentumMover
	case "pippi":
		moverCtor = NewPippiMover
	default:
		moverCtor = NewAngleOffsetMover
		finalName = "flower"
	}

	return
}
