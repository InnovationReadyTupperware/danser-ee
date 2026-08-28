package launcher

import (
	"math"
	"math/rand"
	"testing"
)

func TestVariableHeightLayoutPrefixAndUpdates(t *testing.T) {
	var layout variableHeightLayout
	heights := []float32{10, 20, 30}
	layout.rebuild(len(heights), func(index int) float32 {
		return heights[index]
	})

	if got := layout.len(); got != len(heights) {
		t.Fatalf("layout length = %d, want %d", got, len(heights))
	}
	if got := layout.top(0); got != 0 {
		t.Fatalf("top(0) = %v, want 0", got)
	}
	if got := layout.top(2); got != 30 {
		t.Fatalf("top(2) = %v, want 30", got)
	}
	if got := layout.total(); got != 60 {
		t.Fatalf("total = %v, want 60", got)
	}

	if got := layout.update(1, 25); got != 5 {
		t.Fatalf("height delta = %v, want 5", got)
	}
	if got := layout.top(2); got != 35 {
		t.Fatalf("top(2) after update = %v, want 35", got)
	}
	if got := layout.total(); got != 65 {
		t.Fatalf("total after update = %v, want 65", got)
	}
	if got := layout.update(-1, 100); got != 0 {
		t.Fatalf("invalid update delta = %v, want 0", got)
	}
}

func TestVariableHeightLayoutVisibleRangeMatchesLinearReference(t *testing.T) {
	const count = 257
	heights := make([]float32, count)
	random := rand.New(rand.NewSource(42))
	var total float32

	var layout variableHeightLayout
	layout.rebuild(count, func(index int) float32 {
		heights[index] = float32(8 + random.Intn(40))
		return heights[index]
	})
	for _, height := range heights {
		total += height
	}

	queries := []struct {
		scrollY        float32
		viewportHeight float32
	}{
		{scrollY: 0, viewportHeight: 1},
		{scrollY: 0, viewportHeight: 720},
		{scrollY: 10, viewportHeight: 100},
		{scrollY: total / 2, viewportHeight: 480},
		{scrollY: total - 1, viewportHeight: 720},
		{scrollY: total + 100, viewportHeight: 720},
	}
	for i := 0; i < 1000; i++ {
		queries = append(queries, struct {
			scrollY        float32
			viewportHeight float32
		}{
			scrollY:        random.Float32() * (total + 100),
			viewportHeight: random.Float32() * 800,
		})
	}

	for _, query := range queries {
		gotStart, gotEnd := layout.visibleRange(query.scrollY, query.viewportHeight)
		wantStart, wantEnd := linearVisibleRange(heights, query.scrollY, query.viewportHeight)
		if gotStart != wantStart || gotEnd != wantEnd {
			t.Fatalf("visibleRange(%v, %v) = [%d, %d], want [%d, %d]", query.scrollY, query.viewportHeight, gotStart, gotEnd, wantStart, wantEnd)
		}
	}
}

func TestVariableHeightLayoutHandlesEmptyAndInvalidHeights(t *testing.T) {
	var layout variableHeightLayout
	layout.rebuild(0, nil)
	if start, end := layout.visibleRange(0, 720); start != 0 || end != 0 {
		t.Fatalf("empty visible range = [%d, %d], want [0, 0]", start, end)
	}

	layout.rebuild(2, func(index int) float32 {
		if index == 0 {
			return 0
		}
		return float32(math.Inf(1))
	})
	if layout.height(0) != 1 || layout.height(1) != 1 {
		t.Fatalf("invalid heights = [%v, %v], want both rows clamped to 1", layout.height(0), layout.height(1))
	}
}

func linearVisibleRange(heights []float32, scrollY, viewportHeight float32) (int, int) {
	if len(heights) == 0 {
		return 0, 0
	}
	if scrollY < 0 {
		scrollY = 0
	}
	if viewportHeight < 0 {
		viewportHeight = 0
	}

	first := firstLinearRowAtOrAfter(heights, scrollY)
	end := firstLinearRowAtOrAfter(heights, scrollY+viewportHeight)
	if first > 0 {
		first--
	}
	if end < len(heights) {
		end++
	}
	if end <= first {
		end = min(len(heights), first+1)
	}

	return first, end
}

func firstLinearRowAtOrAfter(heights []float32, offset float32) int {
	if offset <= 0 {
		return 0
	}

	total := float32(0)
	for index, height := range heights {
		total += height
		if total > offset {
			return index
		}
	}

	return len(heights)
}

var (
	variableLayoutBenchmarkIndex int
	variableLayoutBenchmarkRange int
	variableLayoutBenchmarkDelta float32
)

func BenchmarkVariableHeightLayoutRangeAndUpdate140K(b *testing.B) {
	const setCount = 35_000
	var layout variableHeightLayout
	layout.rebuild(setCount, func(index int) float32 {
		return float32(72 + index%9)
	})

	b.ReportAllocs()
	variableLayoutBenchmarkIndex = 0
	b.ResetTimer()
	for b.Loop() {
		index := variableLayoutBenchmarkIndex % setCount
		start, end := layout.visibleRange(float32((variableLayoutBenchmarkIndex*7919)%2_800_000), 720)
		variableLayoutBenchmarkRange = start + end
		variableLayoutBenchmarkDelta = layout.update(index, float32(72+(index+1)%9))
		variableLayoutBenchmarkIndex++
	}
}
