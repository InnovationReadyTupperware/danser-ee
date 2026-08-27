package play

import "github.com/wieku/danser-go/app/beatmap/difficulty"

const (
	// hitErrorReferenceWindow is the OD0 osu!standard Meh window. The meter
	// uses it as a fixed physical reference so changing OD changes band
	// proportions without changing the outer HUD span.
	hitErrorReferenceWindow = 200.0
	hitErrorOffsetScale     = 0.8
)

type hitErrorWindows struct {
	great float64
	good  float64
	meh   float64
}

func hitErrorWindowsFor(diff *difficulty.Difficulty) hitErrorWindows {
	if diff.IsLazer() {
		return hitErrorWindows{
			great: diff.Hit300U,
			good:  diff.Hit100U,
			meh:   diff.Hit50U,
		}
	}

	return hitErrorWindows{
		great: float64(diff.Hit300),
		good:  float64(diff.Hit100),
		meh:   float64(diff.Hit50),
	}
}

func hitErrorOffsetScaleFor(diff *difficulty.Difficulty, scaleWithSpeed bool) float64 {
	scale := hitErrorOffsetScale
	if scaleWithSpeed {
		scale /= diff.Speed
	}

	return scale
}

func hitErrorBandPositions(diff *difficulty.Difficulty, halfWidth float64) [3]float64 {
	windows := hitErrorWindowsFor(diff)
	if windows.meh <= 0 {
		return [3]float64{}
	}

	return [3]float64{
		halfWidth * windows.great / windows.meh,
		halfWidth * windows.good / windows.meh,
		halfWidth,
	}
}

func clampHitErrorPosition(position, halfWidth float64) float64 {
	return max(-halfWidth, min(position, halfWidth))
}
