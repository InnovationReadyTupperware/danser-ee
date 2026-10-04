package pp260706

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
)

func TestGraphSamplerCarriesDecayAcrossEmptyBuckets(t *testing.T) {
	sampler := graphSampler{decay: func(ms float64) float64 { return math.Pow(0.5, ms/1000) }}
	sampler.add(450, 10)
	sampler.add(1800, 2)
	got := sampler.finish()
	want := []float64{10, 10 * math.Pow(0.5, 0.35), 10 * math.Pow(0.5, 0.75), 10 * math.Pow(0.5, 1.15)}
	if len(got) != len(want) {
		t.Fatalf("peaks = %v, want %v", got, want)
	}
	for i := range want {
		assertNear(t, "decayed graph peak", got[i], want[i])
	}
}

func TestGraphSamplerIncludesExactSectionBoundary(t *testing.T) {
	sampler := graphSampler{decay: func(ms float64) float64 { return math.Pow(0.5, ms/1000) }}
	sampler.add(400, 10)
	sampler.add(800, 5)
	got := sampler.finish()
	if len(got) != 2 || got[0] != 10 || got[1] != 10 {
		t.Fatalf("boundary peaks = %v, want [10 10]", got)
	}
}

func TestGraphSampleTimesFollowPlaybackRate(t *testing.T) {
	for _, rate := range []float64{0.75, 1, 1.2, 1.5} {
		beatMap := loadRegressionBeatmap(t, difficulty.None, difficulty.GameplayLazer)
		beatMap.Diff.SetMods(difficulty.DoubleTime)
		conf := difficulty.NewSpeedSettings(rate, false)
		difficulty.SetModConfig(beatMap.Diff, conf)
		beatMap.Diff.AddMod(difficulty.None)
		calculator := NewDifficultyCalculator()
		before := calculator.CalculateSingle(beatMap, beatMap.Diff)
		peaks := calculator.CalculateStrainPeaks(beatMap, beatMap.Diff)
		after := calculator.CalculateSingle(beatMap, beatMap.Diff)
		if after != before {
			t.Fatalf("graph changed difficulty attributes at rate %g", rate)
		}
		steps := calculator.CalculateStep(beatMap, beatMap.Diff)
		last := steps[len(steps)-1]
		assertNear(t, "custom-rate timed final SR", last.Total, before.Total)
		pp := NewPPCalculator()
		score := perfectPerfScore(before)
		assertNear(t, "custom-rate timed final PP", pp.Calculate(last, score, beatMap.Diff).Total, pp.Calculate(before, score, beatMap.Diff).Total)
		if len(peaks.Total) != len(peaks.SampleTimes) || len(peaks.Aim) != len(peaks.Total) || len(peaks.Speed) != len(peaks.Total) || len(peaks.Flashlight) != len(peaks.Total) {
			t.Fatalf("unaligned graph arrays at rate %g", rate)
		}
		firstEnd := math.Ceil(beatMap.HitObjects[1].GetStartTime()/rate/400) * 400
		for i, time := range peaks.SampleTimes {
			assertNear(t, "map sample time", time, (firstEnd+float64(i)*400)*rate)
			if math.IsNaN(peaks.Total[i]) || math.IsInf(peaks.Total[i], 0) {
				t.Fatalf("graph total[%d] = %g", i, peaks.Total[i])
			}
		}
	}
}
