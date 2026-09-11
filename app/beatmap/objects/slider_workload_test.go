package objects

import (
	"math"
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

func TestSliderContinuousSamplesUseStartControlPoint(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "1",
		"L|556:192", "1", "300", "0", "0:0", "0:0:9:20",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 2, 3, 0.4, 4, false, false, false)
	timings.AddPoint(1200, 600, 3, 8, 0.9, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	slider.SetID(0x51a1d3)

	type sampleEvent struct {
		sampleSet     int
		hitsoundIndex int
		index         int
		volume        float64
	}

	var events []sampleEvent
	audio.AddListener(func(sampleSet, hitsoundIndex, index int, volume float64, objectID int64) {
		if objectID != slider.GetID() {
			return
		}

		events = append(events, sampleEvent{
			sampleSet:     sampleSet,
			hitsoundIndex: hitsoundIndex,
			index:         index,
			volume:        volume,
		})
	})

	slider.PlayTickAt(1500)
	slider.PlaySlideSamples(1500)

	if len(events) != 2 {
		t.Fatalf("continuous sample event count = %d, want 2", len(events))
	}

	for _, event := range events {
		if event.hitsoundIndex != 4 && event.hitsoundIndex != 5 {
			t.Fatalf("continuous sample type = %d, want slidertick or sliderslide", event.hitsoundIndex)
		}
		if event.sampleSet != 2 || event.index != 3 || event.volume != 0.4 {
			t.Fatalf("continuous sample metadata = set %d index %d volume %g, want set 2 index 3 volume 0.4",
				event.sampleSet, event.index, event.volume)
		}
	}
}

func TestSliderParentAndNodeSamplesUseLazerControlPointLeniency(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "1",
		"L|356:192", "1", "100", "0", "0:0", "0:0:9:20",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 1, 2, 0.2, 4, false, false, false)
	timings.AddPoint(1005, 600, 2, 5, 0.5, 4, false, false, false)
	timings.AddPoint(1006, 600, 3, 6, 0.6, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	slider.SetID(0x51a1d7)

	type sampleEvent struct {
		sampleSet     int
		hitsoundIndex int
		index         int
		volume        float64
	}

	var events []sampleEvent
	audio.AddListener(func(sampleSet, hitsoundIndex, index int, volume float64, objectID int64) {
		if objectID != slider.GetID() {
			return
		}

		events = append(events, sampleEvent{
			sampleSet:     sampleSet,
			hitsoundIndex: hitsoundIndex,
			index:         index,
			volume:        volume,
		})
	})

	slider.PlayEdgeSample(0)
	slider.PlayTickAt(1100)

	if len(events) != 2 {
		t.Fatalf("sample event count = %d, want 2", len(events))
	}

	head, tick := events[0], events[1]
	if head.sampleSet != 2 || head.index != 5 || head.volume != 0.5 {
		t.Fatalf("head sample metadata = set %d index %d volume %g, want +5 ms node metadata 2/5/0.5",
			head.sampleSet, head.index, head.volume)
	}
	if tick.sampleSet != 3 || tick.hitsoundIndex != 4 || tick.index != 6 || tick.volume != 0.6 {
		t.Fatalf("tick sample metadata = set %d type %d index %d volume %g, want +6 ms parent metadata 3/4/6/0.6",
			tick.sampleSet, tick.hitsoundIndex, tick.index, tick.volume)
	}
}

func TestSliderTrailingHitSampleOnlyProvidesBanks(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "1",
		"L|356:192", "1", "100", "0", "0:0", "3:2:9:20:ignored.wav",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	if slider.BasicHitSound.SampleSet != 3 || slider.BasicHitSound.AdditionSet != 2 {
		t.Fatalf("slider sample banks = %d:%d, want 3:2", slider.BasicHitSound.SampleSet, slider.BasicHitSound.AdditionSet)
	}
	if slider.BasicHitSound.CustomIndex != 0 || slider.BasicHitSound.CustomVolume != 0 {
		t.Fatalf("slider trailing custom index/volume = %d/%g, want ignored", slider.BasicHitSound.CustomIndex, slider.BasicHitSound.CustomVolume)
	}
	if slider.BasicHitSound.Filename != "" {
		t.Fatalf("slider trailing filename = %q, want ignored", slider.BasicHitSound.Filename)
	}
}

func TestSliderAudioNodeTimeUsesFractionalLazerSpan(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L|457:192", "2", "201", "0|0|0", "0:0|0:0|0:0", "0:0",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 537.5, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)

	want := slider.StartTime + slider.visualSpanDuration()
	if got := slider.audioNodeTime(1); got != want {
		t.Fatalf("audio node time = %.12f, want lazer span boundary %.12f", got, want)
	}
	stableBoundary := slider.StartTime + math.Floor(slider.partLen)
	if want == stableBoundary {
		t.Fatalf("test fixture did not produce distinct stable/lazer node times: %g", want)
	}
}

func TestSliderHeadSampleUsesNodeControlPointInsteadOfObjectVolume(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "1",
		"L|356:192", "1", "100", "0", "0:0", "3:3:9:20",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 2, 4, 0.6, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	slider.SetID(0x51a1d4)

	var gotSet, gotIndex int
	var gotVolume float64
	audio.AddListener(func(sampleSet, hitsoundIndex, index int, volume float64, objectID int64) {
		if objectID != slider.GetID() || hitsoundIndex != 0 {
			return
		}

		gotSet = sampleSet
		gotIndex = index
		gotVolume = volume
	})

	slider.PlayEdgeSample(0)

	if gotSet != 2 || gotIndex != 4 || gotVolume != 0.6 {
		t.Fatalf("head sample metadata = set %d index %d volume %g, want set 2 index 4 volume 0.6", gotSet, gotIndex, gotVolume)
	}
}

func TestSliderNodeSampleOverridesControlPointIndexAndVolume(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "1",
		"L|356:192", "1", "100", "0", "0:0:7:60", "0:0",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	timings := NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 2, 4, 0.3, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	slider.SetID(0x51a1d5)

	var gotIndex int
	var gotVolume float64
	audio.AddListener(func(_ int, hitsoundIndex, index int, volume float64, objectID int64) {
		if objectID != slider.GetID() || hitsoundIndex != 0 {
			return
		}

		gotIndex = index
		gotVolume = volume
	})

	slider.PlayEdgeSample(0)

	if gotIndex != 7 || gotVolume != 0.6 {
		t.Fatalf("node sample metadata = index %d volume %g, want index 7 volume 0.6", gotIndex, gotVolume)
	}
}

func TestSliderNodeSampleKeepsCustomFilename(t *testing.T) {
	slider := NewSlider([]string{
		"256", "192", "1000", "2", "1",
		"L|356:192", "1", "100", "0", "0:0:7:60:custom-edge.wav", "0:0",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	if got := slider.sampleFiles[0]; got != "custom-edge.wav" {
		t.Fatalf("node custom filename = %q, want custom-edge.wav", got)
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
