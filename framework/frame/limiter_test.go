package frame

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeLimiterClock struct {
	now         int64
	sleeps      []time.Duration
	yieldCalls  int
	nowReadings int
}

func (clock *fakeLimiterClock) nowValue() int64 {
	clock.nowReadings++
	return clock.now
}

func (clock *fakeLimiterClock) sleep(duration time.Duration) {
	clock.sleeps = append(clock.sleeps, duration)
	clock.now += int64(duration)
}

func (clock *fakeLimiterClock) yield() {
	clock.yieldCalls++
	clock.now += 25 * int64(time.Microsecond)
}

func newFakeLimiter(fps int, clock *fakeLimiterClock) *Limiter {
	return newLimiter(fps, limiterClock{
		now:   clock.nowValue,
		sleep: clock.sleep,
		yield: clock.yield,
	})
}

func TestLimiter_DisabledSyncDoesNotReadTheClock(t *testing.T) {
	clock := &fakeLimiterClock{}
	limiter := newFakeLimiter(0, clock)

	limiter.Sync()

	if clock.nowReadings != 0 {
		t.Fatalf("disabled limiter read the clock %d times", clock.nowReadings)
	}
}

func TestLimiter_NormalizesConfiguredFPS(t *testing.T) {
	tests := []struct {
		name string
		fps  int
		want int
	}{
		{name: "negative disables", fps: -1, want: 0},
		{name: "zero disables", fps: 0, want: 0},
		{name: "positive is retained", fps: 60, want: 60},
		{name: "above representable period is clamped", fps: int(maximumFPS) + 1, want: int(maximumFPS)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter := NewLimiter(test.fps)
			if got := limiter.FPS(); got != test.want {
				t.Fatalf("FPS = %d, want %d", got, test.want)
			}
		})
	}
}

func TestLimiter_UsesOneCoarseSleepBeforeTheYieldTail(t *testing.T) {
	clock := &fakeLimiterClock{}
	limiter := newFakeLimiter(100, clock)

	limiter.Sync()
	if len(clock.sleeps) != 0 {
		t.Fatalf("first Sync slept %v, want no sleep", clock.sleeps)
	}

	limiter.Sync()

	if got, want := len(clock.sleeps), 1; got != want {
		t.Fatalf("sleep calls = %d, want %d", got, want)
	}
	if got, want := clock.sleeps[0], 10*time.Millisecond-yieldTail; got != want {
		t.Fatalf("coarse sleep = %v, want %v", got, want)
	}
	if clock.yieldCalls == 0 {
		t.Fatal("deadline tail did not yield")
	}
	if got, want := clock.now, int64(10*time.Millisecond); got != want {
		t.Fatalf("clock after second Sync = %d, want %d", got, want)
	}
	if got, want := limiter.nextFrame, int64(20*time.Millisecond); got != want {
		t.Fatalf("next deadline after second Sync = %d, want %d", got, want)
	}
}

func TestLimiter_OverrunResetsDeadlineWithoutPolling(t *testing.T) {
	clock := &fakeLimiterClock{}
	limiter := newFakeLimiter(100, clock)
	limiter.Sync()

	clock.now = 25 * int64(time.Millisecond)
	limiter.Sync()

	if len(clock.sleeps) != 0 || clock.yieldCalls != 0 {
		t.Fatalf("overrun performed waits: sleeps=%v yields=%d", clock.sleeps, clock.yieldCalls)
	}
	if got, want := limiter.nextFrame, int64(25*time.Millisecond); got != want {
		t.Fatalf("next deadline = %d, want %d", got, want)
	}
}

func TestLimiter_HighFPSUsesOnlyTheYieldTail(t *testing.T) {
	clock := &fakeLimiterClock{}
	limiter := newFakeLimiter(int(maximumFPS), clock)

	limiter.Sync()
	limiter.Sync()

	if len(clock.sleeps) != 0 {
		t.Fatalf("high-FPS limiter used coarse sleep: %v", clock.sleeps)
	}
	if clock.yieldCalls == 0 {
		t.Fatal("high-FPS limiter did not use the yield tail")
	}
}

func TestLimiter_SetFPSIsSafeWhileSyncRuns(t *testing.T) {
	var now atomic.Int64
	clock := limiterClock{
		now: func() int64 {
			return now.Load()
		},
		sleep: func(duration time.Duration) {
			now.Add(int64(duration))
		},
		yield: func() {
			now.Add(int64(yieldTail))
		},
	}
	limiter := newLimiter(60, clock)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			limiter.SetFPS(i % 10001)
		}
	}()
	go func() {
		defer wg.Done()
		for range 2000 {
			limiter.Sync()
		}
	}()

	wg.Wait()
	if got := limiter.FPS(); got < 0 || got > int(maximumFPS) {
		t.Fatalf("normalized FPS after concurrent updates = %d", got)
	}
}

func TestNextDeadlineHandlesOverflowAndOverruns(t *testing.T) {
	tests := []struct {
		name     string
		previous int64
		period   int64
		now      int64
		want     int64
	}{
		{name: "normal cadence", previous: 100, period: 10, now: 105, want: 110},
		{name: "overrun catches up", previous: 100, period: 10, now: 120, want: 120},
		{name: "overflow saturates", previous: maxInt64, period: 10, now: maxInt64 - 5, want: maxInt64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nextDeadline(test.previous, test.period, test.now); got != test.want {
				t.Fatalf("next deadline = %d, want %d", got, test.want)
			}
		})
	}
}
