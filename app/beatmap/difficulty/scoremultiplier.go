package difficulty

import "math"

// LazerScoreV2Version is the first replay version using Lazer's V2 multipliers
// This score revision is separate from the legacy ScoreV2 modifier
const LazerScoreV2Version = 30000017

func (diff *Difficulty) lazerScoreMultiplier() float64 {
	v2 := diff.ScoreVersion == 0 || diff.ScoreVersion >= LazerScoreV2Version
	multiplier := 1.0
	if diff.Mods.Active(Easy) {
		value := 0.5
		if v2 {
			retries := NewEasySettings().Retries
			if easy, ok := GetModConfig[EasySettings](diff); ok {
				retries = easy.Retries
			}
			value = max(0.4, 0.8-max(0, 0.1*float64(retries-2)))
		}
		multiplier *= value
	}
	if diff.Mods.Active(NoFail) {
		multiplier *= 0.5
	}
	if diff.Mods.Active(HardRock) {
		if v2 {
			multiplier *= 1.09
		} else {
			multiplier *= 1.06
		}
	}
	if diff.Mods.Active(Hidden) {
		hidden, _ := GetModConfig[HiddenSettings](diff)
		if v2 {
			if hidden.OnlyFadeApproachCircles {
				multiplier *= 1.02
			} else {
				multiplier *= 1.04
			}
		} else if !hidden.OnlyFadeApproachCircles {
			multiplier *= 1.06
		}
	}
	if v2 && diff.Mods.Active(Traceable) {
		multiplier *= 1.02
	}
	if diff.Mods.Active(Flashlight) {
		flashlight := NewFlashlightSettings()
		if config, ok := GetModConfig[FlashlightSettings](diff); ok {
			flashlight = config
		}
		value := 1.0
		if v2 {
			value = max(1.02, min(1.2, 1.2-0.2*(flashlight.SizeMultiplier-1)))
			if !flashlight.ComboBasedSize {
				value = 1 + (value-1)/5
			}
		} else if flashlight == NewFlashlightSettings() {
			value = 1.12
		}
		multiplier *= value
	}
	if diff.Mods.Active(Classic) {
		classic := NewClassicSettings()
		if config, ok := GetModConfig[ClassicSettings](diff); ok {
			classic = config
		}
		if v2 && classic.ClassicNoteLock {
			multiplier *= 0.985
		} else {
			multiplier *= 0.96
		}
	}
	if diff.Mods.Active(DifficultyAdjust) {
		value := 0.5
		if v2 {
			value = 1
			// Compare explicit selections to unmodified map values, before HR/EZ
			if config, ok := GetModConfig[DiffAdjustSettings](diff); ok {
				for _, delta := range []float64{
					config.CircleSize - diff.baseCS,
					config.DrainRate - diff.baseHP,
					config.OverallDifficulty - diff.baseOD,
					config.ApproachRate - diff.baseAR,
				} {
					value *= max(0.1, 1-math.Abs(delta)*0.5)
				}
			}
			value = max(0.1, value)
		}
		multiplier *= value
	}
	if diff.Mods.Active(Relax | Autopilot) {
		multiplier *= 0.1
	}
	if diff.Mods.Active(SpunOut) {
		if v2 {
			multiplier *= 0.95
		} else {
			multiplier *= 0.9
		}
	}
	// Rate multipliers belong to speed mods; an independent playback speed
	// override does not introduce a scored mod
	if diff.Mods.Active(HalfTime | Daycore | DoubleTime | Nightcore) {
		if !v2 {
			value := math.Trunc(diff.Speed*10)/10 - 1
			if diff.Speed >= 1 {
				multiplier *= 1 + value/5
			} else {
				multiplier *= 0.6 + value
			}
		} else if diff.Mods.Active(HalfTime | Daycore) {
			multiplier *= math.Trunc(diff.Speed*20)/20*1.4 - 0.5
		} else {
			value := math.Trunc(diff.Speed*10) / 10
			penalty := 0.0
			if value != 1 && value != 1.5 {
				penalty = 0.01
			}
			multiplier *= 1 + (value-1)*0.46 - penalty
		}
	}
	return multiplier
}
