package schedulers

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/dance/spinners"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
)

type Scheduler interface {
	Init(objects []objects.IHitObject, diff *difficulty.Difficulty, cursor *graphics.Cursor, spinnerMoverCtor func() spinners.SpinnerMover, initKeys bool)
	Update(time float64)
	Seek(time float64)
}
