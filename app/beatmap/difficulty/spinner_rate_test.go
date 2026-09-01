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

func TestSpinnerRPMRampWithoutTimelinePreservesNominalSmoothing(t *testing.T) {
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

func TestSpinnerRPMRampAdaptsBriefSpinners(t *testing.T) {
	const (
		targetRPM       = 250.0
		spinnerDuration = 100.0
	)

	var ramp SpinnerRPMRamp
	ramp.Configure(0, spinnerDuration)

	if got := ramp.Update(0, targetRPM); got != 0 {
		t.Fatalf("spinner RPM ramp at its start = %v, want 0", got)
	}

	if got := ramp.Update(nominalSpinnerFrameMilliseconds, targetRPM); got <= 53 {
		t.Fatalf("brief spinner RPM ramp after one frame = %v, want more than 53", got)
	}

	got := ramp.Update(spinnerDuration, targetRPM)
	if got >= targetRPM {
		t.Fatalf("brief spinner RPM ramp at its end = %v, want a soft cap below %v", got, targetRPM)
	}

	if gap := targetRPM - got; gap > spinnerRPMSettledTolerance {
		t.Fatalf("brief spinner RPM ramp end gap = %v, want at most %v", gap, spinnerRPMSettledTolerance)
	}
}

func TestSpinnerRPMRampIsMonotoneAndBoundedAcrossDurations(t *testing.T) {
	testCases := []struct {
		name     string
		duration float64
		target   float64
	}{
		{name: "brief low target", duration: 50, target: 250},
		{name: "brief high target", duration: 100, target: 440},
		{name: "medium", duration: 500, target: 250},
		{name: "long", duration: 10000, target: 440},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var ramp SpinnerRPMRamp
			ramp.Configure(0, test.duration)

			previous := 0.0
			for frame := 0; frame <= 120; frame++ {
				currentTime := min(float64(frame)*nominalSpinnerFrameMilliseconds, test.duration)
				got := ramp.Update(currentTime, test.target)
				if got < previous || got < 0 || got > test.target || !isFiniteSpinnerRateValue(got) {
					t.Fatalf("ramp value at %v ms = %v, previous %v, target %v", currentTime, got, previous, test.target)
				}

				previous = got
			}
		})
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

func TestConfiguredSpinnerRPMRampIsFrameRateIndependent(t *testing.T) {
	var sixtyFPS SpinnerRPMRamp
	var oneTwentyFPS SpinnerRPMRamp

	sixtyFPS.Configure(0, 1000)
	oneTwentyFPS.Configure(0, 1000)

	for frame := 1; frame <= 60; frame++ {
		sixtyFPS.Update(float64(frame)*nominalSpinnerFrameMilliseconds, 250)
	}

	for frame := 1; frame <= 120; frame++ {
		oneTwentyFPS.Update(float64(frame)*nominalSpinnerFrameMilliseconds/2, 250)
	}

	if got, want := sixtyFPS.Update(1000, 250), oneTwentyFPS.Update(1000, 250); math.Abs(got-want) > 1e-9 {
		t.Fatalf("configured frame-rate-dependent ramp values = %v and %v", got, want)
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

func TestConfiguredSpinnerRPMRampRecomputesAfterRewind(t *testing.T) {
	var ramp SpinnerRPMRamp
	ramp.Configure(100, 1100)
	ramp.Update(700, 250)

	got := ramp.Update(300, 250)

	var fresh SpinnerRPMRamp
	fresh.Configure(100, 1100)
	want := fresh.Update(300, 250)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("rewound configured spinner RPM ramp = %v, want %v", got, want)
	}

	if duplicate := ramp.Update(300, 250); duplicate != got {
		t.Fatalf("duplicate configured spinner RPM ramp value = %v, want %v", duplicate, got)
	}
}

func TestConfiguredSpinnerRPMRampRejectsInvalidTimelines(t *testing.T) {
	testCases := []struct {
		name      string
		startTime float64
		endTime   float64
	}{
		{name: "zero duration", startTime: 100, endTime: 100},
		{name: "negative duration", startTime: 100, endTime: 99},
		{name: "non-finite start", startTime: math.NaN(), endTime: 100},
		{name: "non-finite end", startTime: 0, endTime: math.Inf(1)},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var ramp SpinnerRPMRamp
			ramp.Configure(test.startTime, test.endTime)

			if got := ramp.Update(100, 250); got != 0 {
				t.Fatalf("invalid-timeline spinner RPM ramp = %v, want 0", got)
			}
		})
	}
}

func TestSpinnerRPMRampRejectsNonFiniteInputs(t *testing.T) {
	testCases := []struct {
		name        string
		currentTime float64
		targetRPM   float64
	}{
		{name: "non-finite time", currentTime: math.NaN(), targetRPM: 250},
		{name: "positive infinity time", currentTime: math.Inf(1), targetRPM: 250},
		{name: "non-finite target", currentTime: 100, targetRPM: math.NaN()},
		{name: "positive infinity target", currentTime: 100, targetRPM: math.Inf(1)},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var ramp SpinnerRPMRamp
			ramp.Configure(0, 1000)

			if got := ramp.Update(test.currentTime, test.targetRPM); got != 0 {
				t.Fatalf("invalid-input spinner RPM ramp = %v, want 0", got)
			}
		})
	}
}

func TestConfiguredSpinnerRPMRampResetRetainsTimeline(t *testing.T) {
	var ramp SpinnerRPMRamp
	ramp.Configure(0, 1000)
	ramp.Update(500, 250)
	ramp.Reset()

	got := ramp.Update(500, 250)

	var fresh SpinnerRPMRamp
	fresh.Configure(0, 1000)
	want := fresh.Update(500, 250)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("reset configured spinner RPM ramp = %v, want %v", got, want)
	}
}

func rotationAtRPM(rpm, elapsedMilliseconds float64) float64 {
	return rpm * elapsedMilliseconds / 60000 * 360
}
