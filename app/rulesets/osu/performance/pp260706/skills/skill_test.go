package skills

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
)

// Oracle areas were generated from the unmodified VariableLengthStrainSkill.cs
// and Skill.cs at ppy/osu@48c4800e3ae4ee752452cdff83bd3787ccf3105f in .NET 10.
// The subclass returns Index+1 as strain, zero initial decay, and sums peak
// Value*SectionLength. Input starts at 1000ms with 250.5ms gaps divided by rate
func TestFractionalSectionsMatchCSharpOracleAtEveryStep(t *testing.T) {
	cases := []struct {
		rate  float64
		areas []float64
	}{
		{1, []float64{400, 1050, 1950, 3100}},
		{1.2, []float64{400, 1009, 1827, 2854}},
		{1.5, []float64{400, 967, 1701, 2602}},
		{0.75, []float64{400, 1134, 2202, 3604}},
	}
	for _, test := range cases {
		skill := oracleStrainSkill()
		for i, want := range test.areas {
			skill.Process(&preprocessing.DifficultyObject{Index: i, StartTime: (1000 + float64(i)*250.5) / test.rate})
			for range 2 {
				if got := peakArea(skill.GetCurrentStrainPeaks()); got != want {
					t.Fatalf("rate %g step %d peak area = %g, C# oracle %g", test.rate, i, got, want)
				}
			}
		}
	}
}

// The same C# subclass processes 300 objects separated by 200.25ms. These
// snapshots straddle the 44,000ms retention limit and include repeated reads
func TestPeakRetentionMatchesCSharpFractionalDurationOracle(t *testing.T) {
	wantAreas := map[int]float64{219: 4906000, 220: 4950200, 299: 8442000}
	skill := oracleStrainSkill()
	for i := range 300 {
		skill.Process(&preprocessing.DifficultyObject{Index: i, StartTime: 1000 + float64(i)*200.25})
		if want, ok := wantAreas[i]; ok {
			peaks := skill.GetCurrentStrainPeaks()
			if got := peakArea(peaks); got != want || len(peaks) != 220 {
				t.Fatalf("step %d area = %g count = %d, C# oracle %g/220", i, got, len(peaks), want)
			}
			// A caller's presentation changes cannot mutate calculation state
			peaks[0].Value = -100
			if got := peakArea(skill.GetCurrentStrainPeaks()); got != want {
				t.Fatalf("mutated snapshot changed area to %g", got)
			}
		}
	}
}

func oracleStrainSkill() *VariableLengthSkill {
	skill := NewVariableLengthSkill()
	skill.StrainValueAt = func(object *preprocessing.DifficultyObject) float64 { return float64(object.Index + 1) }
	skill.CalculateInitialStrain = func(float64, *preprocessing.DifficultyObject) float64 { return 0 }
	return skill
}

func peakArea(peaks []StrainPeak) float64 {
	var area float64
	for _, peak := range peaks {
		area += peak.Value * peak.SectionLength
	}
	return area
}
