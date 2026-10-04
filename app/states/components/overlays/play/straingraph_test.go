package play

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestStrainGraphUsesBucketTimesWhenMerging(t *testing.T) {
	graph := StrainGraph{
		trueStartTime: 1000, strainStartTime: 1250, strainEndTime: 2600, strainLength: 1350,
		strains: api.StrainPeaks{Total: []float64{1, 3, 2, 4}, SampleTimes: []float64{1600, 2000, 2400, 2800}},
	}
	for _, test := range []struct {
		merge int
		times []float32
		peaks []float32
	}{
		{1, []float32{1600, 2000, 2400, 2600}, []float32{1, 3, 2, 4}},
		{2, []float32{2000, 2600}, []float32{3, 4}},
	} {
		points := graph.curvePoints(test.merge)
		if len(points) != len(test.times)+1 {
			t.Fatalf("merged points = %v", points)
		}
		for i := range test.times {
			if points[i+1].X != test.times[i] || points[i+1].Y != test.peaks[i] {
				t.Fatalf("point %d = %v, want %g/%g", i, points[i+1], test.times[i], test.peaks[i])
			}
		}
	}
	graph.strains.SampleTimes = nil
	points := graph.curvePoints(2)
	if points[1].X != 1250 || points[2].X != 2600 {
		t.Fatalf("historical layout changed: %v", points)
	}
}

func TestStrainGraphEmptyAndSingleObjectCurvesRemainFinite(t *testing.T) {
	for _, beatMap := range []*beatmap.BeatMap{nil, beatmap.NewBeatMap(), {HitObjects: []objects.IHitObject{objects.NewDummySpinner(1000, 2000)}}} {
		graph := StrainGraph{}
		graph.setMapTimes(beatMap)
		curve := graph.generateCurve()
		for _, progress := range []float32{0, 0.5, 1} {
			point := curve.PointAt(progress)
			if math.IsNaN(float64(point.Y)) || math.IsInf(float64(point.Y), 0) {
				t.Fatalf("empty graph point = %v", point)
			}
		}
	}
	graph := StrainGraph{
		trueStartTime: 1000, strainStartTime: 1000, strainEndTime: 1000,
		strains: api.StrainPeaks{Total: []float64{0}, SampleTimes: []float64{1200}},
		size:    vector.NewVec2d(200, 40),
	}
	curve := graph.generateCurve()
	if got := curve.PointAt(0.5).Y; math.IsNaN(float64(got)) || graph.maxStrain <= 0 {
		t.Fatalf("zero curve = %g, max strain %g", got, graph.maxStrain)
	}
}
