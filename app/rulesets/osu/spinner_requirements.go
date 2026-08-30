package osu

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
)

type spinnerRequirements struct {
	required     int64
	maximumBonus int64
}

// calculateSpinnerRequirements is the runtime requirement source. Its
// corrected Lazer result is reused by legacy health precomputation, while
// Stable intentionally retains its historical integer-millisecond calculation
// for runtime judgment.
func calculateSpinnerRequirements(diff *difficulty.Difficulty, startTime, endTime float64) spinnerRequirements {
	if diff == nil {
		return spinnerRequirements{}
	}
	if math.IsNaN(startTime) || math.IsInf(startTime, 0) || math.IsNaN(endTime) || math.IsInf(endTime, 0) {
		return spinnerRequirements{}
	}

	if diff.IsLazer() {
		duration := endTime - startTime
		if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
			return spinnerRequirements{}
		}

		required := truncateNonnegative(diff.LazerSpinnerMinRPS * duration / 1000)
		maximum := truncateNonnegative(diff.LazerSpinnerMaxRPS * duration / 1000)

		return spinnerRequirements{
			required:     required,
			maximumBonus: max(0, maximum-required-difficulty.LazerSpinBonusGap),
		}
	}

	duration := int64(endTime) - int64(startTime)
	if duration <= 0 {
		return spinnerRequirements{}
	}

	required := float64(duration) / 1000 * diff.SpinnerRatio
	if required <= 0 || math.IsNaN(required) || math.IsInf(required, 0) {
		return spinnerRequirements{}
	}

	return spinnerRequirements{required: int64(required)}
}

// calculateHealthSpinnerRequirement keeps the health precomputation's
// historical Stable duration arithmetic while sharing the corrected Lazer
// calculation with runtime judgment. Stable health used the exact fractional
// duration before truncation; changing that detail would alter old replays
// whose object timestamps are not whole milliseconds.
func calculateHealthSpinnerRequirement(diff *difficulty.Difficulty, startTime, endTime float64) int64 {
	if diff == nil {
		return 0
	}

	if diff.IsLazer() {
		return calculateSpinnerRequirements(diff, startTime, endTime).required
	}

	durationSeconds := (endTime - startTime) / 1000
	required := durationSeconds * diff.SpinnerRatio
	if required <= 0 || math.IsNaN(required) || math.IsInf(required, 0) {
		return 0
	}

	return int64(required)
}

func truncateNonnegative(value float64) int64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	return int64(value + 0.0001)
}
