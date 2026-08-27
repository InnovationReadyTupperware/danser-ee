package play

import (
	"math"
	"testing"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/framework/math/vector"
)

func TestScalarStatisticsMatchesPopulationStandardDeviation(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
	}{
		{name: "empty"},
		{name: "single", values: []float64{12}},
		{name: "signed values", values: []float64{-12, -4, 8, 20}},
		{name: "repeated values", values: []float64{7, 7, 7, 7}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var statistics scalarStatistics
			for _, value := range test.values {
				statistics.Add(value)
			}

			want := populationStandardDeviation(test.values)
			if math.Abs(statistics.standardDeviation()-want) > 1e-12 {
				t.Fatalf("standardDeviation() = %.15g, want %.15g", statistics.standardDeviation(), want)
			}
		})
	}
}

func TestVectorStatisticsMatchesPopulationStandardDeviation(t *testing.T) {
	values := []vector.Vector2d{
		vector.NewVec2d(1, 2),
		vector.NewVec2d(3, 4),
		vector.NewVec2d(-1, 5),
	}

	var statistics vectorStatistics
	for _, value := range values {
		statistics.Add(value)
	}

	mean := vector.NewVec2d(0, 0)
	for _, value := range values {
		mean = mean.Add(value)
	}
	mean = mean.Scl(1 / float64(len(values)))

	variance := 0.0
	for _, value := range values {
		variance += value.DstSq(mean)
	}
	variance /= float64(len(values))

	want := math.Sqrt(variance)
	if math.Abs(statistics.standardDeviation()-want) > 1e-12 {
		t.Fatalf("standardDeviation() = %.15g, want %.15g", statistics.standardDeviation(), want)
	}
}

func TestMedianReturnsSignedMiddleValue(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{name: "empty", want: 0},
		{name: "odd", values: []float64{4, -2, 10}, want: 4},
		{name: "even", values: []float64{10, -4, 2, 6}, want: 4},
		{name: "zero", values: []float64{-2, 2}, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := median(test.values); got != test.want {
				t.Fatalf("median(%v) = %g, want %g", test.values, got, test.want)
			}
		})
	}
}

func TestHitErrorBandPositionsUseFixedPhysicalSpan(t *testing.T) {
	stableOD0 := difficulty.NewDifficulty(5, 5, 0, 5)
	stableOD0.SetGameplayMode(difficulty.GameplayStable)
	stableOD10 := difficulty.NewDifficulty(5, 5, 10, 5)
	stableOD10.SetGameplayMode(difficulty.GameplayStable)
	lazerOD5 := difficulty.NewDifficulty(5, 5, 5, 5)
	lazerOD5.SetGameplayMode(difficulty.GameplayLazer)

	const halfWidth = 160.0
	od0 := hitErrorBandPositions(stableOD0, halfWidth)
	od10 := hitErrorBandPositions(stableOD10, halfWidth)
	lazer := hitErrorBandPositions(lazerOD5, halfWidth)

	if od0[2] != halfWidth || od10[2] != halfWidth || lazer[2] != halfWidth {
		t.Fatalf("maximum band positions = %v, %v, %v; want fixed half-width %g", od0[2], od10[2], lazer[2], halfWidth)
	}

	if math.Abs(od0[0]/od0[2]-0.4) > 1e-12 || math.Abs(od0[1]/od0[2]-0.7) > 1e-12 {
		t.Fatalf("OD0 band ratios = %v, want 0.4 and 0.7", od0)
	}
	if math.Abs(od10[0]/od10[2]-0.2) > 1e-12 || math.Abs(od10[1]/od10[2]-0.6) > 1e-12 {
		t.Fatalf("OD10 band ratios = %v, want 0.2 and 0.6", od10)
	}

	windows := hitErrorWindowsFor(lazerOD5)
	if math.Abs(lazer[0]/lazer[2]-windows.great/windows.meh) > 1e-12 ||
		math.Abs(lazer[1]/lazer[2]-windows.good/windows.meh) > 1e-12 {
		t.Fatalf("Lazer band ratios = %v, want ratios from %v", lazer, windows)
	}
}

func TestHitErrorDisplayOffsetIsClampedToMeter(t *testing.T) {
	if got := clampHitErrorPosition(-200, 160); got != -160 {
		t.Fatalf("negative display offset = %g, want -160", got)
	}
	if got := clampHitErrorPosition(200, 160); got != 160 {
		t.Fatalf("positive display offset = %g, want 160", got)
	}
	if got := clampHitErrorPosition(12, 160); got != 12 {
		t.Fatalf("in-range display offset = %g, want 12", got)
	}
}

func TestHitErrorMeterMedianConversion(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayLazer)
	diff.SetMods(difficulty.DoubleTime)

	meter := &HitErrorMeter{
		diff:         diff,
		timingErrors: []float64{-20, 10, 30},
	}

	if got := meter.GetMedian(); got != 10 {
		t.Fatalf("GetMedian() = %g, want 10", got)
	}
	if got := meter.GetMedianConverted(); math.Abs(got-10/1.5) > 1e-12 {
		t.Fatalf("GetMedianConverted() = %g, want %g", got, 10/1.5)
	}
}

func populationStandardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))

	variance := 0.0
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}

	return math.Sqrt(variance / float64(len(values)))
}
