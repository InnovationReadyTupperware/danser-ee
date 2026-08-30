package ffmpeg

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
)

var easings = []easing.Easing{
	flat,
	easing.Linear,
	easing.InQuad,
	easing.OutQuad,
	easing.InOutQuad,
	easing.InCubic,
	easing.OutCubic,
	easing.InOutCubic,
	easing.InQuart,
	easing.OutQuart,
	easing.InOutQuart,
	easing.InQuint,
	easing.OutQuint,
	easing.InOutQuint,
	easing.InSine,
	easing.OutSine,
	easing.InOutSine,
	easing.InExpo,
	easing.OutExpo,
	easing.InOutExpo,
	easing.InCirc,
	easing.OutCirc,
	easing.InOutCirc,
	inBack,
	easing.OutBack,
	inOutBack,
	flat,
	flat,
	pyramidSymmetric,
	semiCircle,
}

func calculateWeights(bFrames, id int, gaussMultiplier float64) []float32 {
	if bFrames == 1 {
		return []float32{1}
	}
	weights := make([]float32, 0, bFrames)

	if id < 0 || id >= len(easings) {
		id = 0
	}

	easeFunc := easings[id]
	for i := range bFrames {
		t := float64(i) / float64(bFrames-1)
		value := easeFunc(t)
		if id == 26 {
			value = math.Exp(-math.Pow(gaussMultiplier*(t-1), 2))
		} else if id == 27 {
			value = math.Exp(-math.Pow(gaussMultiplier*(t*2-1), 2))
		}
		w := 1.0 + value*100
		weights = append(weights, float32(w))
	}

	return weights
}

func flat(_ float64) float64 {
	return 1.0
}

func inBack(t float64) float64 {
	return easing.InBack(t) + 0.100004
}

func inOutBack(t float64) float64 {
	return easing.InOutBack(t) + 0.100004
}

func pyramidSymmetric(t float64) float64 {
	return 1.0 - math.Abs(t*2-1)
}

func semiCircle(t float64) float64 {
	return math.Sqrt(1 - math.Pow(0.5-t, 2))
}
