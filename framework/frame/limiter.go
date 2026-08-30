package frame

import (
	"runtime"
	"sync/atomic"
	"time"

	"github.com/innovationreadytupperware/danser-ee/framework/qpc"
)

const (
	nanosPerSecond = int64(time.Second)
	maximumFPS     = nanosPerSecond
	yieldTail      = 100 * time.Microsecond
	maxInt64       = int64(1<<63 - 1)
)

type limiterClock struct {
	now   func() int64
	sleep func(time.Duration)
	yield func()
}

// Limiter spaces calls to Sync so a loop does not run faster than the
// configured frame rate.
//
// SetFPS and FPS are safe to use while another goroutine is reading the
// configuration. Sync itself owns the deadline state and must not be called
// concurrently. A positive FPS update is observed by the next Sync call; a
// Sync call already sleeping cannot be cancelled by a configuration update.
// The zero value is disabled until a positive rate is supplied with SetFPS.
type Limiter struct {
	fps atomic.Int64

	// nextFrame and initialized belong to the goroutine that calls Sync. The
	// public FPS configuration is separate so it can be changed safely by a
	// settings or UI goroutine.
	nextFrame   int64
	initialized bool
	clock       limiterClock
}

// NewLimiter creates a limiter configured for fps frames per second. Zero or
// negative values disable limiting, and positive values above one billion are
// clamped to the smallest representable one-nanosecond frame period.
func NewLimiter(fps int) *Limiter {
	return newLimiter(fps, limiterClock{
		now:   qpc.GetNanoTime,
		sleep: time.Sleep,
		yield: runtime.Gosched,
	})
}

// SetFPS changes the target frame rate. Values at or below zero disable the
// limiter. The method is safe to call concurrently with FPS and Sync.
func (limiter *Limiter) SetFPS(fps int) {
	if fps <= 0 {
		limiter.fps.Store(0)
		return
	}

	rate := int64(fps)
	if rate > maximumFPS {
		rate = maximumFPS
	}

	limiter.fps.Store(rate)
}

// FPS returns the normalized target frame rate. It returns zero when limiting
// is disabled.
func (limiter *Limiter) FPS() int {
	return int(limiter.fps.Load())
}

// Sync waits until the next frame deadline and then advances that deadline.
// Most of the wait uses one coarse sleep; only the final 100 microseconds use
// scheduler yielding. This avoids the repeated one-millisecond polling loop
// that can consume a full CPU core at high frame rates while retaining a
// short, responsive timing tail.
func (limiter *Limiter) Sync() {
	fps := limiter.FPS()
	if fps <= 0 {
		return
	}

	clock := limiter.clock
	if clock.now == nil {
		clock = defaultLimiterClock()
	}

	period := nanosPerSecond / int64(fps)
	if period < 1 {
		period = 1
	}

	now := clock.now()
	if !limiter.initialized {
		// The first Sync establishes the phase without delaying the work that
		// caused the limiter to be created. The next call observes one full
		// frame period from this point.
		limiter.initialized = true
		limiter.nextFrame = now
	} else {
		now = waitForDeadline(limiter.nextFrame, clock)
	}

	limiter.nextFrame = nextDeadline(limiter.nextFrame, period, now)
}

func newLimiter(fps int, clock limiterClock) *Limiter {
	if clock.now == nil {
		clock = defaultLimiterClock()
	}

	limiter := &Limiter{clock: clock}
	limiter.SetFPS(fps)
	return limiter
}

func defaultLimiterClock() limiterClock {
	return limiterClock{
		now:   qpc.GetNanoTime,
		sleep: time.Sleep,
		yield: runtime.Gosched,
	}
}

func waitForDeadline(deadline int64, clock limiterClock) int64 {
	now := clock.now()
	if deadline <= now {
		return now
	}

	remaining := deadline - now
	if remaining > int64(yieldTail) && clock.sleep != nil {
		clock.sleep(time.Duration(remaining - int64(yieldTail)))
	}

	for {
		now = clock.now()
		if deadline <= now {
			return now
		}

		if clock.yield == nil {
			return now
		}
		clock.yield()
	}
}

func nextDeadline(previous, period, now int64) int64 {
	var deadline int64
	if previous > maxInt64-period {
		deadline = maxInt64
	} else {
		deadline = previous + period
	}

	if deadline < now {
		return now
	}

	return deadline
}
