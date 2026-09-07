package objects

import "math"

// SliderWorkloadClass is a set of presentation flags for a valid slider. The
// flags describe both unusually expensive work and exceptional traversal
// behavior; they are optimization and routing hints, not gameplay or parsing
// decisions.
type SliderWorkloadClass uint8

const (
	// SliderWorkloadNormal uses the ordinary per-slider presentation paths.
	SliderWorkloadNormal SliderWorkloadClass = 0
	// SliderWorkloadPathological routes slider-dance expansion and slider-detail
	// audio through the normal hit-note policy. The authored slider remains
	// available to gameplay and rendering, and to direct cursor tracking when
	// its traversal stays within the bounded movement regime.
	SliderWorkloadPathological SliderWorkloadClass = 1 << 0
	// SliderWorkloadSingular describes a slider whose effective traversal has
	// no usable path or no positive time span. It is handled as one effective
	// point where a presentation path needs a traversable slider.
	SliderWorkloadSingular SliderWorkloadClass = 1 << 1
)

const (
	// Keep authored out-of-playfield movement available for Aspire-style
	// sliders. The coordinate limit is only an emergency guard for trajectories
	// that would carry a generated cursor several playfields out of view.
	pathologicalSliderCoordinateMargin = 2048.0
	pathologicalSliderScorePoints      = 256
	pathologicalSliderRenderSegments   = 512
	pathologicalSliderRepeats          = 256
)

// WorkloadClass returns the cached presentation classification for the slider.
// The result can contain both Pathological and Singular flags. A pathological
// slider is still valid and retains its complete authored geometry and score
// data; the flags only select bounded presentation behavior.
func (slider *Slider) WorkloadClass() SliderWorkloadClass {
	if slider == nil {
		return SliderWorkloadNormal
	}

	if slider.workloadClassComputed {
		return slider.workloadClass
	}

	slider.updateWorkloadClass()
	return slider.workloadClass
}

// Has reports whether all bits in flag are present in the workload class.
func (class SliderWorkloadClass) Has(flag SliderWorkloadClass) bool {
	return flag != SliderWorkloadNormal && class&flag == flag
}

// String returns the stable diagnostic name for a workload class.
func (class SliderWorkloadClass) String() string {
	switch class {
	case SliderWorkloadNormal:
		return "normal"
	case SliderWorkloadPathological:
		return "pathological"
	case SliderWorkloadSingular:
		return "singular"
	case SliderWorkloadPathological | SliderWorkloadSingular:
		return "pathological+singular"
	default:
		return "unknown"
	}
}

// IsPathological reports whether slider-dance expansion and slider-detail
// audio should use the normal hit-note policy for this slider. It must not be
// used to reject the map or discard the authored gameplay geometry.
func (slider *Slider) IsPathological() bool {
	return slider.WorkloadClass().Has(SliderWorkloadPathological)
}

// NeedsGeneratedMovementFallback reports whether direct traversal leaves the
// bounded playfield/path regime used by generated cursor movement. Dense but
// bounded geometry can still be tracked directly when slider dance is off.
func (slider *Slider) NeedsGeneratedMovementFallback() bool {
	if slider == nil {
		return false
	}

	slider.WorkloadClass()
	return slider.movementFallback
}

// IsSingular reports whether the slider's effective traversal has no usable
// stable path or no positive stable time span. Cursor-dance expansion treats a
// singular slider as one effective point, while gameplay keeps the authored
// slider object and its head timing.
func (slider *Slider) IsSingular() bool {
	return slider.WorkloadClass().Has(SliderWorkloadSingular)
}

func (slider *Slider) updateWorkloadClass() {
	if slider == nil {
		return
	}

	slider.movementFallback = false

	var class SliderWorkloadClass
	if slider.isSingularTraversal() {
		class |= SliderWorkloadSingular
	}

	if slider.multiCurve != nil {
		if len(slider.multiCurve.GetLines()) > pathologicalSliderRenderSegments {
			class |= SliderWorkloadPathological
		}

		for _, line := range slider.multiCurve.GetLines() {
			if !finiteSliderCoordinate(line.Point1.X) || !finiteSliderCoordinate(line.Point1.Y) ||
				!finiteSliderCoordinate(line.Point2.X) || !finiteSliderCoordinate(line.Point2.Y) ||
				outsideSliderWorkloadBounds(line.Point1.X, line.Point1.Y) ||
				outsideSliderWorkloadBounds(line.Point2.X, line.Point2.Y) {
				class |= SliderWorkloadPathological
				slider.movementFallback = true
				break
			}
		}
	}

	if len(slider.ScorePoints) > pathologicalSliderScorePoints ||
		len(slider.ScorePointsLazer) > pathologicalSliderScorePoints ||
		slider.RepeatCount > pathologicalSliderRepeats {
		class |= SliderWorkloadPathological
	}

	slider.workloadClass = class
	slider.workloadClassComputed = true
}

func (slider *Slider) isSingularTraversal() bool {
	if slider == nil || slider.multiCurve == nil || len(slider.multiCurve.GetLines()) == 0 {
		return true
	}

	curveLength := slider.multiCurve.GetLengthLazer()
	if !finiteSliderValue(curveLength) || curveLength <= 0 {
		return true
	}

	if slider.stablePathComputed {
		return !slider.hasUsableStableTraversal()
	}

	if slider.timingComputed {
		return !slider.hasUsableLazerTraversal()
	}

	return false
}

func (slider *Slider) hasUsableStableTraversal() bool {
	return slider != nil && len(slider.scorePath) > 0 &&
		finiteSliderValue(slider.StartTime) && finiteSliderValue(slider.EndTime) &&
		slider.EndTime > slider.StartTime
}

func (slider *Slider) hasUsableLazerTraversal() bool {
	return slider != nil && finiteSliderValue(slider.StartTime) &&
		finiteSliderValue(slider.EndTimeLazer) && slider.EndTimeLazer > slider.StartTime &&
		finiteSliderValue(slider.spanDuration) && slider.spanDuration > 0
}

func outsideSliderWorkloadBounds(x, y float32) bool {
	return float64(x) < -pathologicalSliderCoordinateMargin ||
		float64(x) > 512+pathologicalSliderCoordinateMargin ||
		float64(y) < -pathologicalSliderCoordinateMargin ||
		float64(y) > 384+pathologicalSliderCoordinateMargin
}

func finiteSliderCoordinate(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func finiteSliderValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
