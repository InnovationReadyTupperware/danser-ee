package play

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestUnstableRateStatisticsMatchesLazerRateNormalization(t *testing.T) {
	type sample struct {
		offset float64
		rate   float64
	}

	tests := []struct {
		name    string
		samples []sample
		want    float64
		valid   bool
	}{
		{name: "empty"},
		{name: "single", samples: []sample{{offset: 12, rate: 1}}, valid: true},
		{
			name: "distributed hits",
			samples: []sample{
				{offset: -10, rate: 1},
				{offset: -9, rate: 1},
				{offset: -8, rate: 1},
				{offset: -7, rate: 1},
				{offset: -6, rate: 1},
				{offset: -5, rate: 1},
				{offset: -4, rate: 1},
				{offset: -3, rate: 1},
				{offset: -2, rate: 1},
				{offset: -1, rate: 1},
				{offset: 0, rate: 1},
			},
			want:  10 * math.Sqrt(10),
			valid: true,
		},
		{
			name: "static rate change",
			samples: []sample{
				{offset: -150, rate: 1.5},
				{offset: -150, rate: 1.5},
				{offset: 150, rate: 1.5},
				{offset: 150, rate: 1.5},
			},
			want:  1000,
			valid: true,
		},
		{
			name: "dynamic rate change",
			samples: []sample{
				{offset: -50, rate: 0.5},
				{offset: 75, rate: 0.75},
				{offset: -100, rate: 1},
				{offset: 125, rate: 1.25},
			},
			want:  1000,
			valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var statistics unstableRateStatistics
			for _, sample := range test.samples {
				statistics.Add(sample.offset, sample.rate)
			}

			got, valid := statistics.value()
			if valid != test.valid {
				t.Fatalf("value() valid = %t, want %t", valid, test.valid)
			}
			if math.Abs(got-test.want) > 1e-12 {
				t.Fatalf("value() = %.15g, want %.15g", got, test.want)
			}
		})
	}
}

func TestHitErrorMeterKeepsRawOffsetsWhileNormalizingURPerEvent(t *testing.T) {
	meter := &HitErrorMeter{urGlider: animation.NewTargetGlider(0, 0)}
	samples := []HitErrorSample{
		{Offset: -50, GameplayRate: 0.5, Result: osu.Hit300},
		{Offset: 75, GameplayRate: 0.75, Result: osu.Hit300},
		{Offset: -100, GameplayRate: 1, Result: osu.Hit300},
		{Offset: 125, GameplayRate: 1.25, Result: osu.Hit300},
	}

	for _, sample := range samples {
		meter.Record(sample)
	}

	if !meter.HasUnstableRate() {
		t.Fatal("HasUnstableRate() = false, want true after timing hits")
	}
	if got := meter.GetUnstableRate(); math.Abs(got-1000) > 1e-12 {
		t.Fatalf("GetUnstableRate() = %g, want 1000", got)
	}
	if got := meter.GetMedian(); got != 12.5 {
		t.Fatalf("GetMedian() = %g, want raw-offset median 12.5", got)
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

func TestHitErrorLayoutUsesActiveMaximumWindow(t *testing.T) {
	tests := []struct {
		name string
		diff *difficulty.Difficulty
	}{
		{name: "stable OD0", diff: stableDifficulty(0)},
		{name: "stable OD10", diff: stableDifficulty(10)},
		{name: "lazer OD5", diff: lazerDifficulty(5)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := newHitErrorLayout(test.diff, false)
			windows := hitErrorWindowsFor(test.diff)

			if got, want := layout.barHalfWidth, windows.meh*legacyHitErrorOffsetScale; math.Abs(got-want) > 1e-12 {
				t.Fatalf("bar half-width = %g, want active Meh half-width %g", got, want)
			}

			positions := layout.bandPositions()
			want := [3]float64{
				windows.great * legacyHitErrorOffsetScale,
				windows.ok * legacyHitErrorOffsetScale,
				windows.meh * legacyHitErrorOffsetScale,
			}
			for i := range positions {
				if math.Abs(positions[i]-want[i]) > 1e-12 {
					t.Fatalf("band position %d = %g, want %g", i, positions[i], want[i])
				}
			}
		})
	}

	if got, want := newHitErrorLayout(stableDifficulty(0), false).barHalfWidth, 159.6; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Stable OD0 half-width = %g, want %g", got, want)
	}
	if got, want := newHitErrorLayout(stableDifficulty(10), false).barHalfWidth, 79.6; math.Abs(got-want) > 1e-12 {
		t.Fatalf("Stable OD10 half-width = %g, want %g", got, want)
	}
}

func TestHitErrorLayoutUsesLazerWindowsIndependentOfGameplayMode(t *testing.T) {
	stable := stableDifficulty(5)
	lazer := lazerDifficulty(5)

	if got, want := hitErrorWindowsFor(stable), hitErrorWindowsFor(lazer); got != want {
		t.Fatalf("Stable replay windows = %#v, want osu!lazer Classic windows %#v", got, want)
	}
}

func TestHitErrorLayoutNormalizesAndClampsOffsets(t *testing.T) {
	layout := newHitErrorLayout(stableDifficulty(5), false)

	if got, want := layout.displayOffset(layout.maxHitWindow), layout.barHalfWidth; got != want {
		t.Fatalf("Meh offset = %g, want %g", got, want)
	}
	if got, want := layout.displayOffset(layout.maxHitWindow*2), layout.barHalfWidth; got != want {
		t.Fatalf("out-of-range offset = %g, want %g", got, want)
	}
	if got, want := layout.relativePosition(layout.maxHitWindow), 0.5; got != want {
		t.Fatalf("relative Meh position = %g, want %g", got, want)
	}
	if got, want := layout.relativePosition(-layout.maxHitWindow*2), -0.5; got != want {
		t.Fatalf("relative negative position = %g, want %g", got, want)
	}
}

func TestHitErrorLayoutSpeedScalingIsOptIn(t *testing.T) {
	diff := lazerDifficulty(5)
	diff.SetMods(difficulty.DoubleTime)

	withoutSpeed := newHitErrorLayout(diff, false)
	withSpeed := newHitErrorLayout(diff, true)
	want := withoutSpeed.barHalfWidth / diff.Speed

	if math.Abs(withSpeed.barHalfWidth-want) > 1e-12 {
		t.Fatalf("speed-scaled half-width = %g, want %g", withSpeed.barHalfWidth, want)
	}
	if withSpeed.barHalfWidth == withoutSpeed.barHalfWidth {
		t.Fatal("speed scaling did not change the layout")
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

func stableDifficulty(overallDifficulty float64) *difficulty.Difficulty {
	diff := difficulty.NewDifficulty(5, 5, overallDifficulty, 5)
	diff.SetGameplayMode(difficulty.GameplayStable)
	return diff
}

func lazerDifficulty(overallDifficulty float64) *difficulty.Difficulty {
	diff := difficulty.NewDifficulty(5, 5, overallDifficulty, 5)
	diff.SetGameplayMode(difficulty.GameplayLazer)
	return diff
}
