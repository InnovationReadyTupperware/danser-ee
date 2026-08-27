package difficulty

const (
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
