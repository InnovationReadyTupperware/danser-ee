package frame

import (
	"math"
	"sync"
	"time"
)

const defaultHalfLife = 300 * time.Millisecond

// FrameStats tracks a time-weighted estimate of frame throughput.
//
// Add accepts positive frame intervals. The zero value is ready for use,
// Add rejects invalid intervals without changing the estimate, and all
// methods are safe to call concurrently. FPS returns zero until the first
// valid interval has been added.
type FrameStats struct {
	mu sync.RWMutex

	frameWeight   float64
	elapsedWeight float64
	initialized   bool
}

// NewFrameStats creates an empty frame-throughput tracker.
func NewFrameStats() *FrameStats {
	return &FrameStats{}
}

// Add records a frame interval and reports whether it was accepted.
//
// The interval also determines how quickly old observations decay. A fixed
// half-life in wall-clock time keeps a slow frame from receiving extra weight
// merely because its numeric value is large, while still making the estimate
// responsive when the application changes performance regimes.
func (stats *FrameStats) Add(frameTime time.Duration) bool {
	if frameTime <= 0 {
		return false
	}

	elapsed := float64(frameTime)
	decay := math.Exp2(-elapsed / float64(defaultHalfLife))

	stats.mu.Lock()
	defer stats.mu.Unlock()

	frameWeight := stats.frameWeight*decay + 1
	elapsedWeight := stats.elapsedWeight*decay + elapsed
	if !finite(frameWeight) || !finite(elapsedWeight) {
		return false
	}

	stats.frameWeight = frameWeight
	stats.elapsedWeight = elapsedWeight
	stats.initialized = true
	return true
}

// AverageFrameTime returns the weighted average frame interval.
//
// A zero duration means that no valid frame interval has been recorded yet.
func (stats *FrameStats) AverageFrameTime() time.Duration {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	if !stats.initialized || stats.frameWeight <= 0 || stats.elapsedWeight <= 0 ||
		!finite(stats.frameWeight) || !finite(stats.elapsedWeight) {
		return 0
	}

	return durationFromNanoseconds(stats.elapsedWeight / stats.frameWeight)
}

// FPS returns a time-weighted estimate of frames per second.
//
// The estimate is derived from a decayed frame count and elapsed-time total,
// so alternating fast and slow frames are represented by their time
// contribution rather than by a value-dependent smoothing coefficient. It is
// the reciprocal of AverageFrameTime up to duration rounding.
// FPS returns zero until a valid frame interval has been recorded.
func (stats *FrameStats) FPS() float64 {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	if !stats.initialized || stats.frameWeight <= 0 || stats.elapsedWeight <= 0 ||
		!finite(stats.frameWeight) || !finite(stats.elapsedWeight) {
		return 0
	}

	fps := float64(time.Second) * stats.frameWeight / stats.elapsedWeight
	if !finite(fps) {
		return 0
	}

	return fps
}

// ExponentialMovingAverage tracks a finite value whose smoothing is driven by
// elapsed wall-clock time rather than by the value being measured.
//
// The zero value is ready for use and uses a 300 millisecond half-life. Add
// accepts finite values, including zero and negative values, and rejects
// invalid values or non-positive elapsed intervals without changing the
// estimate. All methods are safe to call concurrently.
type ExponentialMovingAverage struct {
	mu sync.RWMutex

	halfLife    time.Duration
	average     float64
	initialized bool
}

// NewExponentialMovingAverage creates an EMA with the supplied positive
// half-life. It panics when halfLife is not positive because that is a
// programmer configuration error, not a recoverable measurement failure.
func NewExponentialMovingAverage(halfLife time.Duration) *ExponentialMovingAverage {
	if halfLife <= 0 {
		panic("frame: EMA half-life must be positive")
	}

	return &ExponentialMovingAverage{halfLife: halfLife}
}

// Add records value after elapsed time has passed and reports whether the
// sample was accepted. The value is not required to be positive because the
// EMA is also used for signed measurements.
func (ema *ExponentialMovingAverage) Add(value float64, elapsed time.Duration) bool {
	if !finite(value) || elapsed <= 0 {
		return false
	}

	ema.mu.Lock()
	defer ema.mu.Unlock()

	if !ema.initialized {
		ema.average = value
		ema.initialized = true
		return true
	}

	halfLife := ema.halfLife
	if halfLife <= 0 {
		halfLife = defaultHalfLife
	}

	alpha := -math.Expm1(-math.Ln2 * float64(elapsed) / float64(halfLife))
	if alpha > 1 {
		alpha = 1
	}

	// This form avoids subtracting two extreme, opposite-signed values before
	// applying alpha. The finite check is still required because a malformed
	// internal state must never be propagated to future samples.
	candidate := ema.average*(1-alpha) + value*alpha
	if !finite(candidate) {
		return false
	}

	ema.average = candidate
	return true
}

// Average returns the current EMA value. It returns zero before the first
// accepted sample, including when the first accepted sample itself is zero.
func (ema *ExponentialMovingAverage) Average() float64 {
	ema.mu.RLock()
	defer ema.mu.RUnlock()

	if !ema.initialized || !finite(ema.average) {
		return 0
	}

	return ema.average
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func durationFromNanoseconds(value float64) time.Duration {
	if !finite(value) || value <= 0 {
		return 0
	}

	const maxDuration = time.Duration(1<<63 - 1)
	if value >= float64(maxDuration) {
		return maxDuration
	}

	return time.Duration(value)
}
