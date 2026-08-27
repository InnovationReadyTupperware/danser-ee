package difficulty

import (
	"math"
	"testing"
)

func TestSpinnerRPMMeterUsesLazerTrailingWindow(t *testing.T) {
	var meter SpinnerRPMMeter

	if got := meter.Update(0, 0); got != 0 {
		t.Fatalf("first spinner RPM sample = %v, want 0", got)
	}

	if got := meter.Update(100, rotationAtRPM(250, 100)); math.Abs(got-250) > 1e-9 {
		t.Fatalf("steady spinner RPM = %v, want 250", got)
	}

	// The old samples have expired, so a stopped spinner must report zero
	// rather than retain the last non-zero rate.
	if got := meter.Update(800, rotationAtRPM(250, 100)); got != 0 {
		t.Fatalf("stopped spinner RPM = %v, want 0 after the display window", got)
	}
}

func TestSpinnerRPMMeterIgnoresDuplicateTimestamps(t *testing.T) {
	var meter SpinnerRPMMeter

	meter.Update(0, 0)
	meter.Update(100, rotationAtRPM(250, 100))

	if got := meter.Update(100, 100000); math.Abs(got-250) > 1e-9 {
		t.Fatalf("duplicate-timestamp spinner RPM = %v, want the previous 250 RPM", got)
	}

	if got := meter.Update(200, rotationAtRPM(250, 200)); math.Abs(got-250) > 1e-9 {
		t.Fatalf("post-duplicate spinner RPM = %v, want 250", got)
	}
}

func TestSpinnerRPMMeterResetsOnRewind(t *testing.T) {
	var meter SpinnerRPMMeter

	meter.Update(0, 0)
	meter.Update(100, rotationAtRPM(250, 100))

	if got := meter.Update(50, rotationAtRPM(250, 50)); got != 0 {
		t.Fatalf("rewound spinner RPM = %v, want 0 for the new first sample", got)
	}

	if got := meter.Update(150, rotationAtRPM(250, 150)); math.Abs(got-250) > 1e-9 {
		t.Fatalf("post-rewind spinner RPM = %v, want 250", got)
	}
}

func TestSpinnerRPMMeterRejectsNonFiniteSamples(t *testing.T) {
	var meter SpinnerRPMMeter

	meter.Update(0, 0)
	meter.Update(100, rotationAtRPM(250, 100))

	if got := meter.Update(math.NaN(), 0); got != 0 {
		t.Fatalf("non-finite-time spinner RPM = %v, want 0", got)
	}

	if got := meter.Update(100, math.Inf(1)); got != 0 {
		t.Fatalf("non-finite-rotation spinner RPM = %v, want 0", got)
	}
}

func TestSpinnerRPMRampSmoothlySettlesOnTarget(t *testing.T) {
	var ramp SpinnerRPMRamp

	if got := ramp.Update(0, 250); got != 0 {
		t.Fatalf("first spinner RPM ramp value = %v, want 0", got)
	}

	previous := 0.0
	for frame := 1; frame <= 180; frame++ {
		got := ramp.Update(float64(frame)*nominalSpinnerFrameMilliseconds, 250)
		if got < previous || got > 250 {
			t.Fatalf("spinner RPM ramp value at frame %d = %v, previous %v", frame, got, previous)
		}

		previous = got
	}

	if previous != 250 {
		t.Fatalf("settled spinner RPM ramp = %v, want exact target 250", previous)
	}
}

func TestSpinnerRPMRampIsFrameRateIndependent(t *testing.T) {
	var sixtyFPS SpinnerRPMRamp
	var oneTwentyFPS SpinnerRPMRamp

	sixtyFPS.Update(0, 250)
	oneTwentyFPS.Update(0, 250)

	for frame := 1; frame <= 60; frame++ {
		sixtyFPS.Update(float64(frame)*nominalSpinnerFrameMilliseconds, 250)
	}

	for frame := 1; frame <= 120; frame++ {
		oneTwentyFPS.Update(float64(frame)*nominalSpinnerFrameMilliseconds/2, 250)
	}

	if math.Abs(sixtyFPS.value-oneTwentyFPS.value) > 1e-9 {
		t.Fatalf("frame-rate-dependent ramp values = %v and %v", sixtyFPS.value, oneTwentyFPS.value)
	}
}

func TestSpinnerRPMRampResetsOnRewind(t *testing.T) {
	var ramp SpinnerRPMRamp

	ramp.Update(0, 250)
	ramp.Update(100, 250)

	if got := ramp.Update(50, 250); got != 0 {
		t.Fatalf("rewound spinner RPM ramp = %v, want 0", got)
	}
}

func rotationAtRPM(rpm, elapsedMilliseconds float64) float64 {
	return rpm * elapsedMilliseconds / 60000 * 360
}
