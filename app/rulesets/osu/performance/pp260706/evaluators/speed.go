package evaluators

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp260706/preprocessing"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/putils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	speedMinSpeedBonus   float64 = 200 // 200 BPM 1/4th
	speedBalancingFactor float64 = 40
)

func EvaluateSpeed(current *preprocessing.DifficultyObject) float64 {
	if current.IsSpinner {
		return 0
	}

	osuCurrObj := current

	strainTime := osuCurrObj.AdjustedDeltaTime
	doubletapness := 1.0 - osuCurrObj.GetDoubletapness(current.Next(0))

	// Cap deltatime to the OD 300 hitwindow.
	// 0.93 is derived from making sure 260bpm OD8 streams aren't nerfed harshly, whilst 0.92 limits the effect of the cap.
	strainTime /= mutils.Clamp((strainTime/osuCurrObj.GreatWindow)/0.93, 0.92, 1)

	// speedBonus will be 0.0 for BPM < 200
	speedBonus := 0.0

	// Add additional scaling bonus for streams/bursts higher than 200bpm
	if putils.MillisecondsToBPMD(strainTime) > speedMinSpeedBonus {
		speedBonus = 0.75 * putils.PowInt((putils.BPMToMillisecondsD(speedMinSpeedBonus)-strainTime)/speedBalancingFactor, 2)
	}

	// Base difficulty with all bonuses
	difficulty := (1.0 + speedBonus) * 1000 / strainTime

	difficulty *= speedHighBpmBonus(osuCurrObj.AdjustedDeltaTime)

	// Apply penalty if there's doubletappable doubles
	return difficulty * doubletapness
}

func speedHighBpmBonus(ms float64) float64 {
	return 1 / (1 - math.Pow(0.3, ms/1000))
}
