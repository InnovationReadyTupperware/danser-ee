package frame

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestFrameStats_ReturnsZeroUntilAValidIntervalIsAdded(t *testing.T) {
	var stats FrameStats

	if got := stats.FPS(); got != 0 {
		t.Fatalf("FPS before a sample = %v, want 0", got)
	}
	if got := stats.AverageFrameTime(); got != 0 {
		t.Fatalf("average before a sample = %v, want 0", got)
	}

	for _, interval := range []time.Duration{0, -time.Nanosecond} {
		if stats.Add(interval) {
			t.Fatalf("Add(%v) accepted an invalid interval", interval)
		}
	}

	if got := stats.FPS(); got != 0 {
		t.Fatalf("FPS after invalid samples = %v, want 0", got)
	}
}

func TestFrameStats_FirstSampleDefinesFiniteRate(t *testing.T) {
	stats := NewFrameStats()

	if !stats.Add(20 * time.Millisecond) {
		t.Fatal("Add rejected a positive frame interval")
	}

	if got, want := stats.AverageFrameTime(), 20*time.Millisecond; got != want {
		t.Fatalf("average frame time = %v, want %v", got, want)
	}
	if got, want := stats.FPS(), 50.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("FPS = %v, want %v", got, want)
	}
}

func TestFrameStats_UsesElapsedTimeForSmoothing(t *testing.T) {
	stats := NewFrameStats()
	for range 100 {
		stats.Add(10 * time.Millisecond)
		stats.Add(100 * time.Millisecond)
	}

	// The estimate should remain a plausible recent throughput for an
	// alternating workload. The old value-dependent coefficient converged near
	// 11 FPS because every slow sample received disproportionate weight.
	if got := stats.FPS(); got < 14 || got > 20 {
		t.Fatalf("alternating-frame FPS = %v, want a recent rate between 14 and 20", got)
	}

	average := stats.AverageFrameTime()
	if average <= 0 {
		t.Fatal("alternating-frame average was not initialized")
	}
	if got, want := stats.FPS(), float64(time.Second)/float64(average); math.Abs(got-want) > 0.02 {
		t.Fatalf("FPS = %v, reciprocal average = %v", got, want)
	}
}

func TestExponentialMovingAverage_UsesElapsedTimeAndAcceptsSignedValues(t *testing.T) {
	fast := NewExponentialMovingAverage(100 * time.Millisecond)
	slow := NewExponentialMovingAverage(100 * time.Millisecond)

	if !fast.Add(0, time.Millisecond) || !slow.Add(0, time.Millisecond) {
		t.Fatal("EMA rejected the initial zero sample")
	}
	if !fast.Add(100, 10*time.Millisecond) || !slow.Add(100, 100*time.Millisecond) {
		t.Fatal("EMA rejected a finite sample")
	}

	if got := fast.Average(); got >= slow.Average() {
		t.Fatalf("short elapsed interval produced average %v, want less than %v", got, slow.Average())
	}
	if got, want := slow.Average(), 50.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("half-life sample average = %v, want %v", got, want)
	}

	signed := NewExponentialMovingAverage(time.Second)
	if !signed.Add(-10, time.Millisecond) {
		t.Fatal("EMA rejected a finite negative sample")
	}
	if got := signed.Average(); got != -10 {
		t.Fatalf("negative initial average = %v, want -10", got)
	}

	var zeroValue ExponentialMovingAverage
	if !zeroValue.Add(7, time.Millisecond) {
		t.Fatal("zero-value EMA rejected a valid sample")
	}
	if got := zeroValue.Average(); got != 7 {
		t.Fatalf("zero-value EMA average = %v, want 7", got)
	}
}

func TestExponentialMovingAverage_IgnoresInvalidSamples(t *testing.T) {
	ema := NewExponentialMovingAverage(time.Second)
	if !ema.Add(10, time.Millisecond) {
		t.Fatal("EMA rejected a valid initial sample")
	}

	want := ema.Average()
	invalid := []struct {
		name    string
		value   float64
		elapsed time.Duration
	}{
		{name: "nan", value: math.NaN(), elapsed: time.Millisecond},
		{name: "positive infinity", value: math.Inf(1), elapsed: time.Millisecond},
		{name: "negative infinity", value: math.Inf(-1), elapsed: time.Millisecond},
		{name: "zero elapsed", value: 20, elapsed: 0},
		{name: "negative elapsed", value: 20, elapsed: -time.Millisecond},
	}

	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if ema.Add(test.value, test.elapsed) {
				t.Fatal("EMA accepted an invalid sample")
			}
			if got := ema.Average(); got != want {
				t.Fatalf("average after invalid sample = %v, want %v", got, want)
			}
		})
	}
}

func TestExponentialMovingAverage_RejectsInvalidHalfLife(t *testing.T) {
	for _, halfLife := range []time.Duration{0, -time.Millisecond} {
		t.Run(halfLife.String(), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("constructor did not panic")
				}
			}()

			NewExponentialMovingAverage(halfLife)
		})
	}
}

func TestStatistics_AreSafeForConcurrentReadersAndWriters(t *testing.T) {
	stats := NewFrameStats()
	ema := NewExponentialMovingAverage(300 * time.Millisecond)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 1; i <= 1000; i++ {
				stats.Add(time.Duration(i) * time.Microsecond)
				ema.Add(float64(i), time.Microsecond)
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				fps := stats.FPS()
				if math.IsNaN(fps) || math.IsInf(fps, 0) {
					t.Errorf("FPS became non-finite: %v", fps)
				}
				average := ema.Average()
				if math.IsNaN(average) || math.IsInf(average, 0) {
					t.Errorf("EMA became non-finite: %v", average)
				}
			}
		}()
	}

	wg.Wait()
}

var (
	benchmarkFrameStatsFPS float64
	benchmarkEMAValue      float64
)

func BenchmarkFrameStatsAdd(b *testing.B) {
	stats := NewFrameStats()
	b.ReportAllocs()

	for b.Loop() {
		stats.Add(16 * time.Millisecond)
		benchmarkFrameStatsFPS = stats.FPS()
	}
}

func BenchmarkExponentialMovingAverageAdd(b *testing.B) {
	ema := NewExponentialMovingAverage(300 * time.Millisecond)
	b.ReportAllocs()

	for b.Loop() {
		ema.Add(16.667, 16*time.Millisecond)
		benchmarkEMAValue = ema.Average()
	}
}
