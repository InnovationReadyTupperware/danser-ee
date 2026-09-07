package utils

import (
	"strconv"
	"strings"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestExpandSliderDanceQueueRetainsCompleteSliderSequenceAndTimeOrder(t *testing.T) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	t.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	slider := newNormalTestSlider(t, 1000)
	queue := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(10, 10), 0),
		slider,
		objects.DummyCircle(vector.NewVec2f(20, 20), 100000),
	}

	expanded := ExpandSliderDanceQueue(queue)
	if len(expanded) != len(queue)-1+len(slider.ScorePoints)+1 {
		t.Fatalf("expanded queue length = %d, want %d", len(expanded), len(queue)-1+len(slider.ScorePoints)+1)
	}

	for i := 1; i < len(expanded); i++ {
		if expanded[i].GetStartTime() < expanded[i-1].GetStartTime() {
			t.Fatalf("expanded queue is not sorted at %d: %g before %g", i, expanded[i-1].GetStartTime(), expanded[i].GetStartTime())
		}
	}

	for _, hitObject := range expanded {
		if _, isSlider := hitObject.(*objects.Slider); isSlider {
			t.Fatal("expanded queue still contains the original slider")
		}
	}
}

func TestExpandSliderDanceQueueUsesLazerScorePointsForLazerDifficulty(t *testing.T) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	t.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	slider := newNormalTestSlider(t, 1000)
	lazer := difficulty.NewDifficulty(5, 5, 5, 5)

	expanded := ExpandSliderDanceQueueForDiff([]objects.IHitObject{slider}, lazer)
	if got, want := len(expanded), len(slider.ScorePointsLazer)+1; got != want {
		t.Fatalf("Lazer expanded queue length = %d, want %d", got, want)
	}

	for i, point := range slider.ScorePointsLazer {
		circle, ok := expanded[i+1].(*objects.Circle)
		if !ok {
			t.Fatalf("expanded Lazer point %d has type %T, want *objects.Circle", i, expanded[i+1])
		}
		if circle.GetStartTime() != point.Time {
			t.Fatalf("expanded Lazer point %d time = %g, want %g", i, circle.GetStartTime(), point.Time)
		}
		if circle.GetStartPosition() != slider.PositionAtLazer(point.Time) {
			t.Fatalf("expanded Lazer point %d position = %v, want %v", i, circle.GetStartPosition(), slider.PositionAtLazer(point.Time))
		}
	}
}

func TestPathologicalSliderDanceQueueUsesOnlyHeadPoint(t *testing.T) {
	slider := newBoundedDenseTestSlider(t, 1000)
	queue := []objects.IHitObject{
		objects.DummyCircle(vector.NewVec2f(10, 10), 0),
		slider,
		objects.DummyCircle(vector.NewVec2f(20, 20), 100000),
	}

	expanded := ExpandSliderDanceQueue(queue)
	if len(expanded) != len(queue) {
		t.Fatalf("pathological expansion length = %d, want %d", len(expanded), len(queue))
	}
	if _, ok := expanded[1].(*objects.Slider); ok {
		t.Fatal("pathological slider remained in cursor-dance queue")
	}
	if got := expanded[1].GetStartPosition(); got != slider.StartPosRaw {
		t.Fatalf("pathological cursor point = %v, want slider head %v", got, slider.StartPosRaw)
	}

	processed := PreprocessQueue(1, queue, false)
	if _, ok := processed[1].(*objects.Slider); !ok {
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

func TestLongBoundedSliderPreprocessingFollowsBody(t *testing.T) {
	// Zigzag slider
	slider := objects.NewSlider([]string{
		"56", "49", "61388", "6", "0",
		"B|455:49|455:49|56:74|56:74|455:74|455:74|56:99|56:99|455:99|455:99|56:124|56:124|455:121|455:121|56:149|56:149|455:149|455:149|56:174|56:174|455:174|455:174|56:199|56:199|455:202|455:202|56:224|56:224|455:224|455:224|56:249|56:249|455:249|455:249|56:274|56:274|455:274|455:274|56:299|56:299|455:299|455:299|56:324",
		"1", "8800", "0", "0:0",
	})
	if slider == nil {
		t.Fatal("long bounded zigzag slider was rejected")
	}

	timings := objects.NewTimings()
	timings.SliderMult = 2
	timings.TickRate = 1
	timings.AddPoint(0, 600, 1, 1, 1, 4, false, false, false)
	timings.FinalizePoints()
	slider.SetTiming(timings, 14, false)

	if slider.WorkloadClass() != objects.SliderWorkloadNormal {
		t.Fatalf("long bounded slider workload class = %v, want normal", slider.WorkloadClass())
	}
	if slider.NeedsGeneratedMovementFallback() {
		t.Fatal("long bounded slider requested movement fallback, want direct body tracking when slider dance is off")
	}

	queue := []objects.IHitObject{slider}
	if processed := PreprocessQueue(0, queue, false); len(processed) != 1 {
		t.Fatalf("dance-off queue length = %d, want the retained slider", len(processed))
	} else if _, ok := processed[0].(*objects.Slider); !ok {
		t.Fatalf("dance-off queue holds %T, want the retained *objects.Slider", processed[0])
	}

	lazer := difficulty.NewDifficulty(5, 5, 5, 5)
	expanded := ExpandSliderDanceQueueForDiff(queue, lazer)
	if got, want := len(expanded), len(slider.ScorePointsLazer)+1; got != want || got <= 1 {
		t.Fatalf("dance-on expansion holds %d points, want the %d-point Lazer slider sequence", got, want)
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
}

func BenchmarkExpandSliderDanceQueuePathological(b *testing.B) {
	previousKnockout := settings.KNOCKOUT
	settings.KNOCKOUT = false
	b.Cleanup(func() { settings.KNOCKOUT = previousKnockout })

	queue := make([]objects.IHitObject, 0, 64)
	for i := range 64 {
		queue = append(queue, newDenseTestSlider(b, float64(1000+i*1000)))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		expanded := ExpandSliderDanceQueue(queue)
		if len(expanded) == 0 {
			b.Fatal("dense expansion returned an empty queue")
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
