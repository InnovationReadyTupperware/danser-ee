package utils

import (
	"strconv"
	"strings"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	dancesliders "github.com/innovationreadytupperware/danser-ee/app/dance/sliders"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestApplySliderDanceKeepsOneSemanticTargetPerSlider(t *testing.T) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	t.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	slider := newNormalTestSlider(t, 1000)
	queue := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(10, 10), 0),
		slider,
		objects.DummyCircle(vector.NewVec2f(20, 20), 100000),
	}

	processed := ApplySliderDance(queue)
	if len(processed) != len(queue) {
		t.Fatalf("processed queue length = %d, want %d", len(processed), len(queue))
	}

	target, ok := processed[1].(*dancesliders.Target)
	if !ok {
		t.Fatalf("processed slider has type %T, want *sliders.Target", processed[1])
	}
	if target.GetDuration() <= 0 {
		t.Fatalf("ordinary slider target duration = %g, want positive", target.GetDuration())
	}
}

func TestApplySliderDanceUsesLazerDuration(t *testing.T) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	t.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	slider := newNormalTestSlider(t, 1000)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)

	processed := ApplySliderDanceForDiff([]objects.IHitObject{slider}, lazer)
	target, ok := processed[0].(*dancesliders.Target)
	if !ok {
		t.Fatalf("processed slider has type %T, want *sliders.Target", processed[0])
	}
	if got := target.GetEndTime(); got != slider.EndTimeLazer {
		t.Fatalf("Lazer target end time = %g, want %g", got, slider.EndTimeLazer)
	}
}

func TestPathologicalSliderDanceQueueUsesOnlyHeadPoint(t *testing.T) {
	slider := newBoundedDenseTestSlider(t, 1000)
	queue := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(10, 10), 0),
		slider,
		objects.DummyCircle(vector.NewVec2f(20, 20), 100000),
	}

	processed := ApplySliderDance(queue)
	if len(processed) != len(queue) {
		t.Fatalf("pathological queue length = %d, want %d", len(processed), len(queue))
	}
	target, ok := processed[1].(*objects.Circle)
	if !ok {
		t.Fatalf("pathological slider has type %T, want *objects.Circle", processed[1])
	}
	if target.GetDuration() != 0 {
		t.Fatalf("pathological target duration = %g, want zero", target.GetDuration())
	}
	if target.SliderPoint {
		t.Fatal("pathological slider target retained slider-point behavior")
	}
	if _, ok := processed[1].(objects.ILongObject); ok {
		t.Fatal("pathological slider target retained long-object behavior")
	}

	withoutSliderDance := PreprocessQueue(1, queue, false)
	if _, ok := withoutSliderDance[1].(*objects.Slider); !ok {
		t.Fatal("pathological slider was collapsed when slider dance was disabled")
	}

	sliderDanceProcessed := PreprocessQueue(1, queue, true)
	if _, ok := sliderDanceProcessed[1].(*objects.Slider); ok {
		t.Fatal("pathological slider remained when slider dance was enabled")
	}
}

func TestExtremeTraversalPreprocessingDoesNotRequireSliderDance(t *testing.T) {
	slider := newDenseTestSlider(t, 1000)
	if !slider.NeedsGeneratedMovementFallback() {
		t.Fatal("extreme slider did not request the generated-movement fallback")
	}

	queue := []objects.IHitObject{slider}
	processed := PreprocessQueue(0, queue, false)
	if _, ok := processed[0].(*objects.Slider); ok {
		t.Fatal("extreme slider remained when slider dance was disabled")
	}
}

func TestSingularSliderPreprocessingDoesNotRequireSliderDance(t *testing.T) {
	slider := objects.NewSlider([]string{
		"256", "192", "1000", "2", "0",
		"L", "1", "0", "0", "0:0",
	})
	if slider == nil || !slider.IsSingular() {
		t.Fatal("test slider was not classified as singular")
	}

	queue := []objects.IHitObject{slider}
	processed := PreprocessQueue(0, queue, false)
	if _, ok := processed[0].(*objects.Slider); ok {
		t.Fatal("singular slider remained when slider dance was disabled")
	}
	if _, ok := processed[0].(objects.ILongObject); ok {
		t.Fatal("singular slider target retained long-object behavior")
	}
}

func BenchmarkApplySliderDancePathological(b *testing.B) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	b.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	queue := make([]objects.IHitObject, 0, 64)
	for i := range 64 {
		queue = append(queue, newDenseTestSlider(b, float64(1000+i*1000)))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		processed := ApplySliderDance(queue)
		if len(processed) == 0 {
			b.Fatal("dense processing returned an empty queue")
		}
	}
}

type testHelper interface {
	Helper()
	Fatalf(format string, args ...any)
}

func newDenseTestSlider(t testHelper, startTime float64) *objects.Slider {
	t.Helper()

	slider := objects.NewSlider([]string{
		"256", "192", formatTestTime(startTime), "2", "0",
		"B|255:1768|255:1768|255:-29156", "1", "32496.5", "0", "0:0",
	})
	if slider == nil {
		t.Fatalf("dense test slider was rejected")
	}

	timings := objects.NewTimings()
	timings.SliderMult = 2.06
	timings.TickRate = 0.5
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	return slider
}

func newBoundedDenseTestSlider(t testHelper, startTime float64) *objects.Slider {
	t.Helper()

	var curve strings.Builder
	curve.WriteString("L")
	for i := range 514 {
		if i%2 == 0 {
			curve.WriteString("|257:192")
		} else {
			curve.WriteString("|256:192")
		}
	}

	slider := objects.NewSlider([]string{
		"256", "192", formatTestTime(startTime), "2", "0",
		curve.String(), "1", "514", "0", "0:0",
	})
	if slider == nil {
		t.Fatalf("bounded dense test slider was rejected")
	}

	timings := objects.NewTimings()
	timings.SliderMult = 2.06
	timings.TickRate = 0.5
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)

	if !slider.IsPathological() {
		t.Fatalf("bounded dense test slider was not classified as pathological")
	}
	if slider.NeedsGeneratedMovementFallback() {
		t.Fatalf("bounded dense test slider requested the generated-movement fallback")
	}

	return slider
}

func newNormalTestSlider(t testHelper, startTime float64) *objects.Slider {
	t.Helper()

	slider := objects.NewSlider([]string{
		"256", "192", formatTestTime(startTime), "2", "0",
		"B|320:240|380:192", "1", "160", "0", "0:0",
	})
	if slider == nil {
		t.Fatalf("normal test slider was rejected")
	}

	timings := objects.NewTimings()
	timings.SliderMult = 1.4
	timings.TickRate = 1
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)
	return slider
}

func formatTestTime(time float64) string {
	return strconv.FormatFloat(time, 'f', -1, 64)
}
