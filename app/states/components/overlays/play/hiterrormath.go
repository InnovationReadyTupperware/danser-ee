package play

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
)

const (
	// legacyHitErrorBarHeight is the unscaled height used by osu!lazer's
	// LegacyBarHitErrorMeter before its stable-coordinate conversion.
	legacyHitErrorBarHeight = 3.0

	// legacyHitErrorScale converts the legacy x480 hit-error bar to danser's
	// x768 HUD coordinate system. One side of the bar is therefore 0.8 times
	// the active Meh window, while the complete bar is 1.6 times that window.
	legacyHitErrorScale = 1.6

	legacyHitErrorOffsetScale = legacyHitErrorScale / 2

	legacyHitErrorMaximumPosition = 0.5
)

type hitErrorWindows struct {
	great float64
	ok    float64
	meh   float64
}

// hitErrorLayout contains the timing-space geometry of the horizontal legacy
// bar. The layout deliberately keeps raw hit-window values separate from the
// user HUD scale: the former controls Lazer-compatible timing normalization,
// while the latter controls the visual size of the complete component.
type hitErrorLayout struct {
	windows       hitErrorWindows
	maxHitWindow  float64
	timingScale   float64
	barHalfWidth  float64
	bandHalfWidth [3]float64
}

func newHitErrorLayout(diff *difficulty.Difficulty, scaleWithSpeed bool) hitErrorLayout {
	windows := hitErrorWindowsFor(diff)
	maxHitWindow := windows.meh
	if maxHitWindow <= 0 || math.IsNaN(maxHitWindow) || math.IsInf(maxHitWindow, 0) {
		// Lazer starts maxHitWindow at one to keep malformed or incomplete
		// ruleset windows from producing a division by zero. Normal osu!
		// standard difficulties always replace this with their Meh window.
		maxHitWindow = 1
	}

	timingScale := hitErrorTimingScaleFor(diff, scaleWithSpeed)

	return hitErrorLayout{
		windows:      windows,
		maxHitWindow: maxHitWindow,
		timingScale:  timingScale,
		barHalfWidth: maxHitWindow * timingScale,
		bandHalfWidth: [3]float64{
			windows.great * timingScale,
			windows.ok * timingScale,
			windows.meh * timingScale,
		},
	}
}

func hitErrorWindowsFor(diff *difficulty.Difficulty) hitErrorWindows {
	if diff == nil {
		return hitErrorWindows{}
	}

	return hitErrorWindows{
		great: diff.Hit300U,
		ok:    diff.Hit100U,
		meh:   diff.Hit50U,
	}
}

func hitErrorTimingScaleFor(diff *difficulty.Difficulty, scaleWithSpeed bool) float64 {
	if !scaleWithSpeed || diff == nil || diff.Speed <= 0 || math.IsNaN(diff.Speed) || math.IsInf(diff.Speed, 0) {
		return legacyHitErrorOffsetScale
	}

	return legacyHitErrorOffsetScale / diff.Speed
}

func (layout hitErrorLayout) bandPositions() [3]float64 {
	return layout.bandHalfWidth
}

func (layout hitErrorLayout) displayOffset(offset float64) float64 {
	return clampHitErrorPosition(offset*layout.timingScale, layout.barHalfWidth)
}

func (layout hitErrorLayout) relativePosition(offset float64) float64 {
	if layout.maxHitWindow <= 0 {
		return 0
	}

	return max(-legacyHitErrorMaximumPosition, min(offset/layout.maxHitWindow/2, legacyHitErrorMaximumPosition))
}

func clampHitErrorPosition(position, halfWidth float64) float64 {
	if math.IsNaN(position) || math.IsInf(position, 0) || halfWidth < 0 || math.IsNaN(halfWidth) || math.IsInf(halfWidth, 0) {
		return 0
	}

	return max(-halfWidth, min(position, halfWidth))
}
