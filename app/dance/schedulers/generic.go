package schedulers

import (
	"math/rand"
	"slices"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/dance/input"
	"github.com/wieku/danser-go/app/dance/movers"
	"github.com/wieku/danser-go/app/dance/spinners"
	"github.com/wieku/danser-go/app/dance/utils"
	"github.com/wieku/danser-go/app/graphics"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/vector"
)

type GenericScheduler struct {
	cursor   *graphics.Cursor
	queue    []objects.IHitObject
	mover    movers.MultiPointMover
	lastTime float64
	input    *input.NaturalInputProcessor
	diff     *difficulty.Difficulty
	index    int
	id       int
}

func NewGenericScheduler(mover func() movers.MultiPointMover, index, id int) Scheduler {
	return &GenericScheduler{mover: mover(), index: index, id: id}
}

func (scheduler *GenericScheduler) Init(objs []objects.IHitObject, diff *difficulty.Difficulty, cursor *graphics.Cursor, spinnerMoverCtor func() spinners.SpinnerMover, initKeys bool) {
	// GenericController normalizes this section before constructing its
	// schedulers, but PlayerController and ReplayController also use this
	// scheduler directly. Keep the scheduler safe for those call sites and for
	// programmatic settings replacement.
	settings.NormalizeCursorDance()

	scheduler.diff = diff
	scheduler.cursor = cursor
	scheduler.queue = slices.Clone(objs)
	scheduler.input = nil
	scheduler.lastTime = 0

	scheduler.mover.Reset(diff, scheduler.id)

	config := settings.CursorDance.Movers[scheduler.index%len(settings.CursorDance.Movers)]

	if settings.CursorDance.Resolve2BAfterTAG {
		scheduler.queue = utils.Solve2B(scheduler.queue)
	}

	// Slider dance / random slider dance resolving
	for i := range len(scheduler.queue) {
		scheduler.queue = utils.PreprocessQueue(i, scheduler.queue, (config.SliderDance && !config.RandomSliderDance) || (config.RandomSliderDance && rand.Intn(2) == 0))
	}

	// Convert spinners to pseudo spinners with captured beginning and ending
	// angles. This keeps the regular mover code independent from settings.
	for i := range len(scheduler.queue) {
		if s, ok := scheduler.queue[i].(*objects.Spinner); ok {
			scheduler.queue[i] = spinners.NewSpinner(s, diff, spinnerMoverCtor)
		}
	}

	// Convert two overlapping circles (slider starts too if slider danced) to one double-tap circle.
	// The queue shrinks when a pair is collapsed, so this condition must be
	// evaluated on every iteration instead of capturing the original length.
	for i := 0; i+1 < len(scheduler.queue); i++ {
		current, pOk := scheduler.queue[i].(*objects.Circle)
		next, cOk := scheduler.queue[i+1].(*objects.Circle)

		if pOk && cOk && (!current.SliderPoint || current.SliderPointStart || (current.SliderPointEnd && diff.IsLazer())) && (!next.SliderPoint || next.SliderPointStart || (next.SliderPointEnd && diff.IsLazer())) {
			dst := current.GetStackedEndPositionMod(diff).Dst(next.GetStackedStartPositionMod(diff))

			if dst <= float32(diff.CircleRadius*1.995) && next.GetStartTime()-current.GetEndTime() <= 3 { // Sacrificing a bit of UR for better looks
				sTime := (next.GetStartTime() + current.GetEndTime()) / 2

				if current.SliderPointEnd && diff.IsLazer() { // Prioritize slider end timing
					sTime = current.GetEndTime()
				}

				dC := objects.DummyCircle(current.GetStackedEndPositionMod(diff).Add(next.GetStackedStartPositionMod(diff)).Scl(0.5), sTime)

				if !diff.IsLazer() || (!current.SliderPointEnd && !next.SliderPointEnd) { // Don't double-click if any of them is a slider end
					dC.DoubleClick = true
				}

				scheduler.queue[i] = dC

				scheduler.queue = append(scheduler.queue[:i+1], scheduler.queue[i+2:]...)
			}
		}
	}

	// Spread overlapping circles timing-wise
	for i := range len(scheduler.queue) - 1 {
		current := scheduler.queue[i]
		dtP := 0.0
		if c, cOk := current.(*objects.Circle); cOk && c.DoubleClick {
			dtP = 1
		}

		for j := i + 1; j < len(scheduler.queue); j++ {
			o := scheduler.queue[j]

			if current.GetEndTime() < o.GetStartTime() {
				break
			}

			if c, cOk := o.(*objects.Circle); cOk && (!c.SliderPoint || c.SliderPointStart) {
				scheduler.queue[j] = objects.DummyCircle(c.GetStackedStartPositionMod(diff), c.GetStartTime()+float64(j-i)+dtP)
			}
		}
	}

	if len(scheduler.queue) == 0 {
		// TAG distribution can legitimately leave a cursor without objects. This
		// is especially common for spinner-only maps when spinners are not shared
		// between TAG cursors. Movers require a two-object window, so do not feed
		// the synthetic dummy circle to SetObjects by itself.
		scheduler.initializeCursor()
		return
	}

	if initKeys {
		if cursor != nil && diff != nil {
			scheduler.input = input.NewNaturalInputProcessor(scheduler.queue, cursor, scheduler.mover, diff.GetSpeed())
		}
	}

	scheduler.queue = append([]objects.IHitObject{objects.DummyCircle(vector.NewVec2f(100, 100), -500)}, scheduler.queue...)

	scheduler.initializeCursor()

	toRemove := scheduler.mover.SetObjects(scheduler.queue) - 1
	scheduler.queue = scheduler.queue[toRemove:]
}

// initializeCursor preserves the historical starting position for real
// cursors while allowing an object-free scheduler to be tested and advanced
// without constructing the OpenGL-backed cursor renderer.
func (scheduler *GenericScheduler) initializeCursor() {
	if scheduler.cursor == nil {
		return
	}

	scheduler.cursor.SetPos(vector.NewVec2f(100, 100))
	scheduler.cursor.Update(0)
}

func (scheduler *GenericScheduler) Update(time float64) {
	if len(scheduler.queue) > 0 {
		useMover := true
		lastEndTime := 0.0

		for i := 0; i < len(scheduler.queue); i++ {
			g := scheduler.queue[i]

			gStartTime := scheduler.mover.GetObjectsStartTime(g)
			gEndTime := scheduler.mover.GetObjectsEndTime(g)

			if gStartTime > time {
				break
			}

			lastEndTime = max(lastEndTime, gEndTime)

			if scheduler.lastTime <= gStartTime || time <= gEndTime {
				if scheduler.lastTime <= gStartTime { // brief movement lock for ExGon mover
					useMover = false
					scheduler.cursor.SetPos(scheduler.mover.GetObjectsStartPosition(g))
				} else {
					scheduler.cursor.SetPos(scheduler.mover.GetObjectsPosition(time, g))
				}
			}

			if time > gEndTime {
				upperLimit := len(scheduler.queue)

				for j := i; j < len(scheduler.queue); j++ {
					if scheduler.mover.GetObjectsEndTime(scheduler.queue[j]) >= lastEndTime {
						break
					}

					upperLimit = j + 1
				}

				toRemove := 1

				if upperLimit-i > 1 {
					toRemove = scheduler.mover.SetObjects(scheduler.queue[i:upperLimit]) - 1
				}

				scheduler.queue = append(scheduler.queue[:i], scheduler.queue[i+toRemove:]...)
				i--
			}
		}

		if useMover && scheduler.mover.GetEndTime() >= time {
			scheduler.cursor.SetPos(scheduler.mover.Update(time))
		}
	}

	if scheduler.input != nil {
		scheduler.input.Update(time)
	}

	scheduler.lastTime = time
}
