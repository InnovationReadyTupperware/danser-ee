package difficulty

import "math"

const (
	// Lazer's spinner RPM display is calculated over this trailing time window.
	// Keep the value aligned with SpinnerSpmCalculator.cs in the local osu!
	// lazer source snapshot. The unusual 595 ms value is intentional: lazer
	// avoids a round 600 ms window because frame rounding can otherwise affect
	// the displayed result.
	lazerSpinnerRPMWindowMilliseconds = 595.0

	// This is the nominal frame interval used by danser's existing Lazer
	// display ramp. Normalising the decay by elapsed time keeps the entrance
	// animation stable when rendering runs above or below 60 FPS.
	nominalSpinnerFrameMilliseconds = 1000.0 / 60
	spinnerRPMRampDecayPerFrame     = 0.95
	spinnerRPMSettledTolerance      = 0.5
)

type spinnerRPMRecord struct {
	time     float64
	rotation float64
}

// SpinnerRPMMeter reproduces osu!lazer's display-only spinner rate
// calculation. It measures the change in total spinner rotation over a
// trailing window rather than exposing a noisy per-frame angular velocity.
//
// The meter does not participate in spinner judgment. Callers must continue
// to use their raw rotation state for completion and scoring decisions.
type SpinnerRPMMeter struct {
	records []spinnerRPMRecord
	head    int

	value    float64
	lastTime float64
	hasTime  bool
}

// Reset discards display history while retaining the small backing buffer for
// reuse by the same spinner. A reset is required when playback seeks
// backwards because old samples belong to a different timeline.
func (meter *SpinnerRPMMeter) Reset() {
	meter.records = meter.records[:0]
	meter.head = 0
	meter.value = 0
	meter.lastTime = 0
	meter.hasTime = false
}

// Update records a total-rotation sample and returns the display RPM.
// Duplicate timestamps are ignored, the first sample returns zero, and a
// backwards timestamp starts a fresh history. These details mirror Lazer's
// SpinnerSpmCalculator and avoid division by zero or stale seek-time values.
func (meter *SpinnerRPMMeter) Update(currentTime, currentRotation float64) float64 {
	if !isFiniteSpinnerRateValue(currentTime) || !isFiniteSpinnerRateValue(currentRotation) {
		meter.Reset()
		return 0
	}

	if meter.hasTime {
		if currentTime < meter.lastTime {
			meter.Reset()
		} else if currentTime == meter.lastTime {
			return meter.value
		}
	}

	hadRecord := meter.head < len(meter.records)
	if hadRecord {
		record := meter.records[meter.head]

		for meter.head < len(meter.records) && currentTime-meter.records[meter.head].time > lazerSpinnerRPMWindowMilliseconds {
			record = meter.records[meter.head]
			meter.head++
		}

		elapsed := currentTime - record.time
		if elapsed > 0 {
			meter.value = (currentRotation - record.rotation) / elapsed * 1000 * 60 / 360
			if meter.value < 0 || !isFiniteSpinnerRateValue(meter.value) {
				meter.value = 0
			}
		} else {
			meter.value = 0
		}
	}

	meter.records = append(meter.records, spinnerRPMRecord{time: currentTime, rotation: currentRotation})
	meter.lastTime = currentTime
	meter.hasTime = true
	meter.compact()

	return meter.value
}

func (meter *SpinnerRPMMeter) compact() {
	if meter.head == len(meter.records) {
		meter.records = meter.records[:0]
		meter.head = 0
		return
	}

	// A spinner normally retains only about 36 samples. Compact after a larger
	// seek/update burst so consumed records do not keep growing the backing
	// slice over a long render session.
	if meter.head >= 64 && meter.head*2 >= len(meter.records) {
		copy(meter.records, meter.records[meter.head:])
		meter.records = meter.records[:len(meter.records)-meter.head]
		meter.head = 0
	}
}

// SpinnerRPMRamp supplies the cursor-dance-only entrance animation. The
// target is known exactly for generated movement, but the value still eases
// in so the HUD does not jump from zero to the target on the first spinner
// frame. Once the value is close enough to affect no displayed integer, it is
// snapped to the exact target so float32 cursor geometry cannot reintroduce
// a 249/250-style boundary oscillation.
type SpinnerRPMRamp struct {
	value    float64
	lastTime float64
	hasTime  bool
}

// Reset returns the ramp to its zero-rate startup state.
func (ramp *SpinnerRPMRamp) Reset() {
	ramp.value = 0
	ramp.lastTime = 0
	ramp.hasTime = false
}

// Update advances the ramp toward targetRPM and returns its display value.
// The first sample establishes the timeline and returns zero. A backwards
// timestamp also resets the ramp, matching the seek behavior of
// SpinnerRPMMeter.
func (ramp *SpinnerRPMRamp) Update(currentTime, targetRPM float64) float64 {
	if !isFiniteSpinnerRateValue(currentTime) || targetRPM <= 0 || !isFiniteSpinnerRateValue(targetRPM) {
		ramp.Reset()
		return 0
	}

	if ramp.hasTime {
		if currentTime < ramp.lastTime {
			ramp.Reset()
		} else if currentTime == ramp.lastTime {
			return ramp.value
		}
	}

	if !ramp.hasTime {
		ramp.lastTime = currentTime
		ramp.hasTime = true
		return 0
	}

	elapsed := currentTime - ramp.lastTime
	if elapsed > 0 {
		decay := math.Pow(spinnerRPMRampDecayPerFrame, elapsed/nominalSpinnerFrameMilliseconds)
		if !isFiniteSpinnerRateValue(decay) || decay < 0 || decay > 1 {
			decay = 0
		}

		ramp.value = targetRPM - (targetRPM-ramp.value)*decay
		if math.Abs(targetRPM-ramp.value) <= spinnerRPMSettledTolerance {
			ramp.value = targetRPM
		}
	}

	ramp.lastTime = currentTime
	return ramp.value
}

func isFiniteSpinnerRateValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
