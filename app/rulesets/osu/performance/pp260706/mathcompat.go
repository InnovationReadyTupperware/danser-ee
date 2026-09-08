package pp260706

import "math"

// osuErf and osuErfInv reproduce the fixed approximations from
// osu.Game/Rulesets/Difficulty/Utils/DiffUtils.cs in the reference source
// snapshot ppy/osu@48c4800e3ae4ee752452cdff83bd3787ccf3105f. That hash pins the
// source tree used for this port; it is not the commit that introduced the July
// 2026 PP rework. Do not replace these with math.Erf/math.Erfinv: the released
// performance model's numeric output depends on these approximations.
func osuErf(x float64) float64 {
	if x == 0 {
		return 0
	}
	if math.IsInf(x, 1) {
		return 1
	}
	if math.IsInf(x, -1) {
		return -1
	}
	if math.IsNaN(x) {
		return math.NaN()
	}

	t := 1 / (1 + 0.3275911*math.Abs(x))
	tau := t * (0.254829592 + t*(-0.284496736+t*(1.421413741+t*(-1.453152027+t*1.061405429))))
	erf := 1 - tau*math.Exp(-x*x)
	if x >= 0 {
		return erf
	}
	return -erf
}

func osuErfInv(x float64) float64 {
	if x <= -1 {
		return math.Inf(-1)
	}
	if x >= 1 {
		return math.Inf(1)
	}
	if x == 0 {
		return 0
	}

	sign := 1.0
	if x < 0 {
		sign = -1
	}
	x = math.Abs(x)
	const a = 0.147
	ln := math.Log(1 - x*x)
	t1 := 2/(math.Pi*a) + ln/2
	t2 := ln / a
	baseApprox := math.Sqrt(t1*t1-t2) - t1
	correction := 0.0
	if x >= 0.85 {
		correction = math.Pow((x-0.85)/0.293, 8)
	}
	return sign * (math.Sqrt(baseApprox) + correction)
}
