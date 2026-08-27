package play

import (
	"math"
	"sort"

	"github.com/wieku/danser-go/framework/math/vector"
)

// scalarStatistics stores population variance using Welford's online
// algorithm. UR uses sum-of-squares divided by the number of accepted hits,
// so this intentionally does not use the sample-variance n-1 denominator.
// Keeping the accumulator incremental avoids rescanning the entire replay
// history every time a new hit arrives.
type scalarStatistics struct {
	count        int64
	mean         float64
	sumOfSquares float64
}

func (statistics *scalarStatistics) Add(value float64) {
	statistics.count++

	delta := value - statistics.mean
	nextMean := statistics.mean + delta/float64(statistics.count)
	statistics.sumOfSquares += delta * (value - nextMean)
	statistics.mean = nextMean
}

func (statistics scalarStatistics) standardDeviation() float64 {
	if statistics.count == 0 {
		return 0
	}

	// Round-off can leave a mathematically zero SSE very slightly negative.
	return math.Sqrt(max(statistics.sumOfSquares/float64(statistics.count), 0))
}

// vectorStatistics is the two-dimensional form of scalarStatistics used by
// the aim-error meter. The accumulated SSE is the sum of squared Euclidean
// distances from the running mean, matching the previous full-history pass.
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

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}

	return (sorted[middle-1] + sorted[middle]) / 2
}
