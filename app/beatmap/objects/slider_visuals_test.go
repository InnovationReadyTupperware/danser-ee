package objects

import (
	"math"
	"testing"
)

func TestSliderSpanProgressMatchesLazerBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		completion float64
		spanCount  int
		wantSpan   int
		wantPath   float64
	}{
		{name: "start of first span", completion: 0, spanCount: 3, wantSpan: 0, wantPath: 0},
		{name: "middle of first span", completion: 0.25, spanCount: 2, wantSpan: 0, wantPath: 0.5},
		{name: "repeat boundary", completion: 0.5, spanCount: 2, wantSpan: 1, wantPath: 1},
		{name: "middle of reverse span", completion: 0.75, spanCount: 2, wantSpan: 1, wantPath: 0.5},
		{name: "one-span end", completion: 1, spanCount: 1, wantSpan: 1, wantPath: 1},
		{name: "two-span end", completion: 1, spanCount: 2, wantSpan: 2, wantPath: 0},
		{name: "three-span end", completion: 1, spanCount: 3, wantSpan: 3, wantPath: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpan, gotPath := sliderSpanProgress(tt.completion, tt.spanCount)
			if gotSpan != tt.wantSpan || gotPath != tt.wantPath {
				t.Fatalf("sliderSpanProgress(%g, %d) = (%d, %g), want (%d, %g)", tt.completion, tt.spanCount, gotSpan, gotPath, tt.wantSpan, tt.wantPath)
			}
		})
	}
}

func TestSliderBodyRangeAtUsesFinalSpanParity(t *testing.T) {
	settings := sliderSnakeSettings{in: false, out: true}

	tests := []struct {
		name       string
		completion float64
		spanCount  int
		wantHead   float64
		wantTail   float64
	}{
		{name: "final odd span retracts from tail", completion: 0.75, spanCount: 2, wantHead: 0, wantTail: 0.5},
		{name: "even final span retracts from head", completion: 0.8, spanCount: 3, wantHead: 0.4, wantTail: 1},
		{name: "even final span collapses at end", completion: 1, spanCount: 2, wantHead: 0, wantTail: 0},
		{name: "odd final span remains full at end", completion: 1, spanCount: 3, wantHead: 1, wantTail: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliderBodyRangeAt(1000+tt.completion*1000, 1000, 2000, 300, 333.3333333333333, tt.spanCount, settings)
			if !closeEnough(got.head, tt.wantHead) || !closeEnough(got.tail, tt.wantTail) {
				t.Fatalf("sliderBodyRangeAt() = (%g, %g), want (%g, %g)", got.head, got.tail, tt.wantHead, tt.wantTail)
			}
		})
	}
}

func TestSliderBodyRangeAtAppliesVisualizerSnakeInOverrides(t *testing.T) {
	base := sliderBodyRangeAt(900, 1000, 2000, 300, 1000, 1, sliderSnakeSettings{in: true})
	custom := sliderBodyRangeAt(900, 1000, 2000, 300, 1000, 1, sliderSnakeSettings{
		in:                 true,
		durationMultiplier: 1,
		fadeMultiplier:     1,
	})

	if base.tail != 1 {
		t.Fatalf("Lazer snake-in tail at 900 = %g, want 1", base.tail)
	}
	if !closeEnough(custom.tail, 2.0/13.0) {
		t.Fatalf("extended snake-in tail at 900 = %g, want %g", custom.tail, 2.0/13.0)
	}
}

func closeEnough(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func TestSliderBodyRangeAtHandlesInvalidSpanCount(t *testing.T) {
	got := sliderBodyRangeAt(1000, 1000, 2000, 300, 1000, 0, sliderSnakeSettings{in: true, out: true})
	if got.head != 0 || got.tail != 0 {
		t.Fatalf("invalid span count range = (%g, %g), want (0, 0)", got.head, got.tail)
	}
}

func TestSliderBodyRangeAtHandlesInstantaneousSlider(t *testing.T) {
	settings := sliderSnakeSettings{out: true}

	beforeStart := sliderBodyRangeAt(900, 1000, 1000, 300, 0, 1, settings)
	if !closeEnough(beforeStart.head, 0) || !closeEnough(beforeStart.tail, 1) {
		t.Fatalf("instantaneous slider before start = (%g, %g), want (0, 1)", beforeStart.head, beforeStart.tail)
	}

	atStart := sliderBodyRangeAt(1000, 1000, 1000, 300, 0, 1, settings)
	if !closeEnough(atStart.head, 1) || !closeEnough(atStart.tail, 1) {
		t.Fatalf("instantaneous slider at start = (%g, %g), want (1, 1)", atStart.head, atStart.tail)
	}
}
