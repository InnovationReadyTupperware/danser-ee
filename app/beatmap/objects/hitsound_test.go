package objects

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
)

func TestParseExtrasKeepsCustomHitsoundFilename(t *testing.T) {
	info := parseExtras([]string{"3:2:7:60:custom-hit.wav"}, 0)

	if info.SampleSet != 3 || info.AdditionSet != 2 || info.CustomIndex != 7 || info.CustomVolume != 0.6 {
		t.Fatalf("sample metadata = %d:%d:%d:%g, want 3:2:7:0.6",
			info.SampleSet, info.AdditionSet, info.CustomIndex, info.CustomVolume)
	}
	if info.Filename != "custom-hit.wav" {
		t.Fatalf("Filename = %q, want custom-hit.wav", info.Filename)
	}
}

func TestCircleSampleUsesLazerControlPointLeniency(t *testing.T) {
	circle := NewCircle([]string{"256", "192", "1000", "1", "1", "0:0"})
	circle.SetID(0x51a1d8)
	circle.diff = difficulty.NewDifficulty(5, 5, 5, 5)

	timings := NewTimings()
	timings.AddPoint(0, 600, 1, 2, 0.2, 4, false, false, false)
	timings.AddPoint(1005, 600, 2, 5, 0.5, 4, false, false, false)
	timings.FinalizePoints()
	circle.SetTiming(timings, 14, false)

	var gotSet, gotIndex int
	var gotVolume float64
	audio.AddListener(func(sampleSet, hitsoundIndex, index int, volume float64, objectID int64) {
		if objectID != circle.GetID() || hitsoundIndex != 0 {
			return
		}

		gotSet = sampleSet
		gotIndex = index
		gotVolume = volume
	})

	circle.PlaySound(circle.StartTime)

	if gotSet != 2 || gotIndex != 5 || gotVolume != 0.5 {
		t.Fatalf("circle sample metadata = set %d index %d volume %g, want +5 ms metadata 2/5/0.5",
			gotSet, gotIndex, gotVolume)
	}
}

func TestSpinnerSampleUsesLazerControlPointLeniency(t *testing.T) {
	spinner := NewSpinner([]string{"256", "192", "1000", "8", "1", "1500", "0:0"})
	spinner.SetID(0x51a1d9)

	timings := NewTimings()
	timings.AddPoint(0, 600, 1, 2, 0.2, 4, false, false, false)
	timings.AddPoint(1505, 600, 3, 6, 0.6, 4, false, false, false)
	timings.FinalizePoints()
	spinner.SetTiming(timings, 14, false)

	var gotSet, gotIndex int
	var gotVolume float64
	audio.AddListener(func(sampleSet, hitsoundIndex, index int, volume float64, objectID int64) {
		if objectID != spinner.GetID() || hitsoundIndex != 0 {
			return
		}

		gotSet = sampleSet
		gotIndex = index
		gotVolume = volume
	})

	spinner.Hit(spinner.EndTime, true)

	if gotSet != 3 || gotIndex != 6 || gotVolume != 0.6 {
		t.Fatalf("spinner sample metadata = set %d index %d volume %g, want +5 ms metadata 3/6/0.6",
			gotSet, gotIndex, gotVolume)
	}
}
