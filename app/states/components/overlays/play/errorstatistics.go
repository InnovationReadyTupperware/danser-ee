package play

import (
	"math"
	"slices"

	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

// unstableRateStatistics stores osu!lazer-compatible unstable-rate state using
// Welford's online population variance. Each hit error is normalized by the
// mod-derived gameplay rate recorded for that event before it enters the
// accumulator.
type unstableRateStatistics struct {
	count        int64
	mean         float64
	sumOfSquares float64
}

func (statistics *unstableRateStatistics) Add(timeOffset, gameplayRate float64) {
	value := timeOffset / gameplayRate
	statistics.count++

	delta := value - statistics.mean
	nextMean := statistics.mean + delta/float64(statistics.count)
	statistics.sumOfSquares += delta * (value - nextMean)
	statistics.mean = nextMean
}

func (statistics unstableRateStatistics) value() (float64, bool) {
	if statistics.count == 0 {
		return 0, false
	}

	// Round-off can leave a mathematically zero SSE very slightly negative.
	variance := max(statistics.sumOfSquares/float64(statistics.count), 0)
	return 10 * math.Sqrt(variance), true
}

// vectorStatistics stores the aim-error meter's two-dimensional population
// variance. The accumulated SSE is the sum of squared Euclidean distances from
// the running mean, matching the previous full-history pass.
type vectorStatistics struct {
	count        int64
	mean         vector.Vector2d
	sumOfSquares float64
}

func (statistics *vectorStatistics) Add(value vector.Vector2d) {
	statistics.count++

	delta := value.Sub(statistics.mean)
	nextMean := statistics.mean.Add(delta.Scl(1 / float64(statistics.count)))
	statistics.sumOfSquares += delta.Dot(value.Sub(nextMean))
	statistics.mean = nextMean
}

func (statistics vectorStatistics) standardDeviation() float64 {
	if statistics.count == 0 {
		return 0
	}

	return math.Sqrt(max(statistics.sumOfSquares/float64(statistics.count), 0))
}

// median returns the signed middle value. A copy is sorted so callers can
// retain the original chronological sample order for any future diagnostics.
// The hit-error ranking screen calls this once after gameplay, rather than on
// every hit, so the sort does not reintroduce the old per-hit hot-path cost.
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := slices.Clone(values)
	slices.Sort(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}

	return (sorted[middle-1] + sorted[middle]) / 2
}
