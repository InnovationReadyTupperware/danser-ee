package difficulty

import "math"

const (
	// LegacySpinnerRotationsPerMillisecond is the historical osu!stable
	// autoplay spinner rate used by danser's Stable-compatible paths.
	LegacySpinnerRotationsPerMillisecond = 0.00795

	// LegacySpinnerRPM is the historical spinner rate expressed in revolutions
	// per minute. Keep the conversion next to the source constant so callers do
	// not maintain a second, potentially drifting rate literal.
	LegacySpinnerRPM = LegacySpinnerRotationsPerMillisecond * 60000

	// LazerSpinnerMaximumOD is the highest OD covered by the fixed spinner
	// completion rate used when the per-map setting is disabled.
	LazerSpinnerMaximumOD = 11

	lazerSpinnerCompletionRPMAtOD0  = 250.0
	lazerSpinnerCompletionRPMAtOD5  = 380.0
	lazerSpinnerCompletionRPMAtOD10 = 430.0

	// LazerSpinnerMaximumCompletionRPM is derived from osu!lazer's
	// completion-rate endpoints rather than being a separately maintained RPM
	// literal. Go evaluates this constant expression at compile time, so the
	// fixed rate remains synchronized with the documented OD 11 ceiling.
	LazerSpinnerMaximumCompletionRPM = lazerSpinnerCompletionRPMAtOD5 +
		(lazerSpinnerCompletionRPMAtOD10-lazerSpinnerCompletionRPMAtOD5)*
			(LazerSpinnerMaximumOD-5)/(10-5)
)

// LazerSpinnerCompletionRPM returns the average RPM required to complete an
// osu!lazer spinner and receive all normal and bonus ticks at the supplied OD.
// The rate follows osu!lazer's linear difficulty range and intentionally keeps
// extrapolating for the custom OD values danser permits.
func LazerSpinnerCompletionRPM(od float64) float64 {
	return DifficultyRate(od, lazerSpinnerCompletionRPMAtOD0, lazerSpinnerCompletionRPMAtOD5, lazerSpinnerCompletionRPMAtOD10)
}

// SpinnerAutoplayRPM returns the rate used for generated spinner movement.
// Stable gameplay keeps danser's historical rate, while Lazer gameplay uses
// either the map's completion endpoint or the fixed OD 11 endpoint selected
// by the cursor-dance setting. The boolean is passed in by the settings owner
// so this package remains independent from UI/configuration globals.
func SpinnerAutoplayRPM(diff *Difficulty, spinAtLowestRPM bool) float64 {
	if diff == nil || !diff.IsLazer() {
		return LegacySpinnerRPM
	}

	if spinAtLowestRPM {
		rpm := diff.LazerSpinnerMaxRPS * 60
		if rpm > 0 && !math.IsNaN(rpm) && !math.IsInf(rpm, 0) {
			return rpm
		}
	}

	return LazerSpinnerMaximumCompletionRPM
}
