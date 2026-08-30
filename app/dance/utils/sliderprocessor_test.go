package utils

import (
	"strconv"
	"testing"

	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/vector"
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

func TestPathologicalSliderDanceQueueUsesOnlyHeadPoint(t *testing.T) {
	slider := newDenseTestSlider(t, 1000)
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
	if _, ok := processed[1].(*objects.Slider); ok {
		t.Fatal("pathological slider remained when slider dance was disabled")
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
