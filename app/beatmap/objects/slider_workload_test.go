package objects

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestPathologicalSliderClassificationUsesOptimizationMetadata(t *testing.T) {
	normal := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L|300:192", "1", "44", "0", "0:0",
	})
	if normal == nil {
		t.Fatal("normal slider was rejected")
	}
	if normal.WorkloadClass() != SliderWorkloadNormal {
		t.Fatalf("normal slider workload class = %v, want normal", normal.WorkloadClass())
	}

	aesthetic := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L|256:1768", "1", "1576", "0", "0:0",
	})
	if aesthetic == nil {
		t.Fatal("out-of-playfield slider was rejected")
	}
	if aesthetic.WorkloadClass() != SliderWorkloadNormal || aesthetic.NeedsGeneratedMovementFallback() {
		t.Fatalf("out-of-playfield slider workload = %v, fallback = %t; want normal trackable movement",
			aesthetic.WorkloadClass(), aesthetic.NeedsGeneratedMovementFallback())
	}

	pathological := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|255:1768|255:1768|255:-29156", "1", "32496.5", "0", "0:0",
	})
	if pathological == nil {
		t.Fatal("pathological slider was rejected")
	}
	if !pathological.IsPathological() {
		t.Fatal("out-of-playfield slider was not classified as pathological")
	}
	if pathological.IsSingular() {
		t.Fatal("pathological slider was incorrectly classified as singular")
	}
}

func TestPathologicalSliderSuppressesSliderDetailAudio(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|255:1768|255:1768|255:-29156", "1", "32496.5", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("pathological slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 2.06
	timings.TickRate = 0.5
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	slider.SetID(987654321)

	var detailEvents int
	audio.AddListener(func(_, _, _ int, _ float64, objectID int64) {
		if objectID == slider.GetID() {
			detailEvents++
		}
	})

	slider.PlayTickAt(slider.StartTime + 10)
	slider.PlaySlideSamples(slider.StartTime + 10)

	if detailEvents != 0 {
		t.Fatalf("pathological slider emitted %d slider-detail audio events, want none", detailEvents)
	}
}

func TestLongBoundedSliderKeepsNormalPresentation(t *testing.T) {
	// Zigzag slider
	slider := NewSlider([]string{
		"56", "49", "61388", "6", "0",
		"B|455:49|455:49|56:74|56:74|455:74|455:74|56:99|56:99|455:99|455:99|56:124|56:124|455:121|455:121|56:149|56:149|455:149|455:149|56:174|56:174|455:174|455:174|56:199|56:199|455:202|455:202|56:224|56:224|455:224|455:224|56:249|56:249|455:249|455:249|56:274|56:274|455:274|455:274|56:299|56:299|455:299|455:299|56:324",
		"1", "8800", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("long zigzag slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 2
	timings.TickRate = 1
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)

	if slider.WorkloadClass() != SliderWorkloadNormal {
		t.Fatalf("long bounded slider workload class = %v, want normal", slider.WorkloadClass())
	}
	if slider.NeedsGeneratedMovementFallback() {
		t.Fatal("long bounded slider requested generated-movement fallback")
	}

	lazer := difficulty.NewDifficulty(5, 5, 5, 5)
	stable := difficulty.NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(difficulty.GameplayStable)

	slider.diff = lazer
	lazerDots := slider.visualTickPoints()
	if len(lazerDots) == 0 {
		t.Fatal("long slider has no Lazer tick dots, want the full tick timeline")
	}

	slider.diff = stable
	if got := len(slider.visualTickPoints()); got != len(slider.TickPoints) || got == 0 {
		t.Fatalf("long Stable slider tick dots = %d, want the %d generated ticks", got, len(slider.TickPoints))
	}

	slider.diff = lazer
	slider.initScorePointAnimations()
	for i, p := range slider.visualTickPoints() {
		if p.fade == nil || p.scale == nil {
			t.Fatalf("Lazer tick dot %d at %g has no animation transforms", i, p.Time)
		}
		if want := slider.PositionAtLazer(p.Time); p.Pos != want {
			t.Fatalf("Lazer tick dot %d position = %v, want the Lazer path position %v", i, p.Pos, want)
		}
	}
}

func TestSingularSliderUsesOneEffectivePoint(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L", "1", "0", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("zero-path slider was rejected")
	}

	if !slider.IsSingular() {
		t.Fatalf("zero-path workload class = %v, want singular", slider.WorkloadClass())
	}
	if slider.IsPathological() {
		t.Fatal("zero-path slider was incorrectly classified as pathological")
	}

	if got := len(slider.GetAsDummyCircles()); got != 1 {
		t.Fatalf("zero-path slider dance points = %d, want one", got)
	}
	if got := slider.PositionAt(slider.StartTime + 100); got != slider.StartPosRaw {
		t.Fatalf("zero-path PositionAt = %v, want start position %v", got, slider.StartPosRaw)
	}
}

func TestSingularSliderRecognizesSubMillisecondStableDuration(t *testing.T) {
	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()

	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L|256.1:192", "1", "0.1", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("sub-millisecond slider was rejected")
	}
	slider.SetTiming(timings, 14, false)

	if len(slider.scorePath) == 0 {
		t.Fatal("sub-millisecond slider did not retain its authored stable path")
	}
	if slider.EndTime <= slider.StartTime {
		if !slider.IsSingular() {
			t.Fatalf("zero-span workload class = %v, want singular", slider.WorkloadClass())
		}
	} else {
		t.Fatalf("sub-millisecond stable duration = %g, want a non-positive floored span", slider.EndTime-slider.StartTime)
	}
	if got := len(slider.GetAsDummyCircles()); got != 1 {
		t.Fatalf("zero-span slider dance points = %d, want one", got)
	}
}

func TestSliderTimingRebuildDoesNotDuplicateWorkloadInputs(t *testing.T) {
	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()

	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|256:192|300:192|300:240", "1", "120", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}
	slider.SetTiming(timings, 14, false)

	stablePathCount := len(slider.scorePath)
	stableScorePointCount := len(slider.ScorePoints)
	lazerTickCount := len(slider.TickPointsLazer)
	lazerScorePointCount := len(slider.ScorePointsLazer)

	slider.SetTiming(timings, 14, false)

	if len(slider.scorePath) != stablePathCount || len(slider.ScorePoints) != stableScorePointCount || len(slider.TickPointsLazer) != lazerTickCount || len(slider.ScorePointsLazer) != lazerScorePointCount {
		t.Fatalf("repeated SetTiming duplicated derived work: path %d/%d, stable points %d/%d, Lazer ticks %d/%d, Lazer points %d/%d",
			len(slider.scorePath), stablePathCount, len(slider.ScorePoints), stableScorePointCount, len(slider.TickPointsLazer), lazerTickCount, len(slider.ScorePointsLazer), lazerScorePointCount)
	}
}

func TestDifficultyOnlyTimingKeepsTraversableSliderOutOfSingularClass(t *testing.T) {
	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 1000, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()

	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L|356:192", "1", "100", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}
	slider.SetTiming(timings, 14, true)

	if slider.IsSingular() {
		t.Fatalf("difficulty-only workload class = %v, want a traversable Lazer slider", slider.WorkloadClass())
	}
	if slider.EndTimeLazer <= slider.StartTime {
		t.Fatalf("difficulty-only Lazer duration = %g, want positive", slider.EndTimeLazer-slider.StartTime)
	}
}

func TestSliderParserRejectsNonFiniteFieldsWithoutApplyingWorkloadLimits(t *testing.T) {
	tests := []struct {
		name string
		data []string
	}{
		{
			name: "non-finite start coordinate",
			data: []string{"NaN", "192", "1000", "2", "0", "L|300:192", "1", "44"},
		},
		{
			name: "non-finite control coordinate",
			data: []string{"256", "192", "1000", "2", "0", "L|Inf:192", "1", "44"},
		},
		{
			name: "invalid pixel length",
			data: []string{"256", "192", "1000", "2", "0", "L|300:192", "1", "not-a-number"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if slider := NewSlider(test.data); slider != nil {
				t.Fatal("malformed slider was accepted")
			}
		})
	}
}

func TestSliderDanceRetainsEveryStableScorePointForNormalSlider(t *testing.T) {
	timings := NewTimings()
	timings.SliderMult = 2.06
	timings.TickRate = 0.5
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()

	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"B|320:240|380:192", "1", "160", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}
	slider.SetTiming(timings, 14, false)

	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	t.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	dummyCircles := slider.GetAsDummyCircles()
	want := len(slider.ScorePoints) + 1
	if len(dummyCircles) != want {
		t.Fatalf("slider dance points = %d, want %d", len(dummyCircles), want)
	}
	if len(dummyCircles) < 2 {
		t.Fatalf("slider dance generated %d points, want the authored score-point sequence", len(dummyCircles))
	}
}
