package pp260706

import "math"

const graphSectionLength = 400.0

// graphSampler follows upstream's chronological reconstruction independently
// of the sorted peaks and harmonic sums used for authoritative difficulty
type graphSampler struct {
	decay        func(float64) float64
	sectionEnd   float64
	peak         float64
	previousTime float64
	previous     float64
	peaks        []float64
	initialized  bool
}

func (sampler *graphSampler) add(time, strain float64) {
	if !sampler.initialized {
		sampler.sectionEnd = math.Ceil(time/graphSectionLength) * graphSectionLength
		sampler.previousTime = time
		sampler.initialized = true
	}
	for time > sampler.sectionEnd {
		sampler.peaks = append(sampler.peaks, sampler.peak)
		sampler.peak = sampler.previous * sampler.decay(sampler.sectionEnd-sampler.previousTime)
		sampler.sectionEnd += graphSectionLength
	}
	sampler.peak = max(sampler.peak, strain)
	sampler.previousTime = time
	sampler.previous = strain
}

func (sampler *graphSampler) finish() []float64 {
	return append(sampler.peaks, sampler.peak)
}
