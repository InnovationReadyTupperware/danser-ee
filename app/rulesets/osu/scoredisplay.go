package osu

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
)

// ClassicDisplayScore converts a Lazer standardised score to its osu!standard
// classic display value using the full beatmap object count.
func ClassicDisplayScore(standardisedScore int64, objectCount int) int64 {
	maximumClassicScore := math.Pow(float64(objectCount), 2)*32.57 + 100000
	return int64(math.RoundToEven(maximumClassicScore * float64(standardisedScore) / 1000000))
}

// RecordedStableScore contains the result stored in a Stable replay header.
// Those results can differ from a later judgement of the saved input frames.
type RecordedStableScore struct {
	TotalScore int64
	MaxCombo   int
	Count300   int
	Count100   int
	Count50    int
	CountMiss  int
}

type legacyMigrationAttributes struct {
	accuracyScore   int64
	comboScore      int64
	bonusScore      int64
	bonusScoreRatio float64
	maxCombo        int
}

// standardisedStableScore applies osu!lazer's score V1 migration to the
// recorded result. The score multiplier for the new scale includes Classic;
// the legacy multiplier excludes the Classic mod injected for Stable playback.
func standardisedStableScore(replay RecordedStableScore, beatMap *beatmap.BeatMap, diff *difficulty.Difficulty, legacyDifficultyMultiplier float64) int64 {
	if beatMap == nil || diff == nil || len(beatMap.HitObjects) == 0 || replay.TotalScore < 0 {
		return 0
	}

	attrs := simulateLegacyMigrationAttributes(beatMap, legacyDifficultyMultiplier)
	legacy := diff.Clone()
	legacy.SetGameplayMode(difficulty.GameplayStable)
	legacyModMultiplier := legacyReplayScoreMultiplier(legacy)
	modern := diff.Clone()
	// Stable score migration targets current Lazer scoring, not the replay era
	modern.ScoreVersion = 0
	modern.SetGameplayMode(difficulty.GameplayLazer)
	return migrateLegacyScoreV1(replay, attrs, legacyModMultiplier, modern.GetScoreMultiplier())
}

func migrateLegacyScoreV1(replay RecordedStableScore, attrs legacyMigrationAttributes, legacyModMultiplier, modernModMultiplier float64) int64 {
	objectCount := replay.Count300 + replay.Count100 + replay.Count50 + replay.CountMiss
	if objectCount <= 0 {
		return 0
	}

	accuracy := float64(replay.Count300*300+replay.Count100*100+replay.Count50*50) / float64(objectCount*300)
	maximumLegacyComboScore := math.RoundToEven(float64(attrs.comboScore) * legacyModMultiplier)
	maximumLegacyBaseScore := float64(attrs.accuracyScore) + maximumLegacyComboScore
	legacyAccScore := float64(attrs.accuracyScore) * accuracy
	comboProportion := 0.0
	if maximumLegacyComboScore+float64(attrs.bonusScore) > 0 {
		comboProportion = math.Max(float64(replay.TotalScore)-legacyAccScore, 0) / (maximumLegacyComboScore + float64(attrs.bonusScore))
	} else if legacyModMultiplier != 0 {
		comboProportion = 1
	}
	bonusProportion := math.Max(0, (float64(replay.TotalScore)-maximumLegacyBaseScore)*attrs.bonusScoreRatio)

	convertedWithoutMods := 0.0
	if replay.MaxCombo == 0 || accuracy == 0 {
		convertedWithoutMods = 500000*math.Pow(accuracy, 5) + bonusProportion
	} else if maximumLegacyComboScore+float64(attrs.bonusScore) == 0 {
		convertedWithoutMods = 500000*comboProportion + 500000*math.Pow(accuracy, 5) + bonusProportion
	} else {
		maximumV1ComboPortion := math.Pow(float64(attrs.maxCombo), 2)
		maximumStandardisedComboPortion := math.Pow(float64(attrs.maxCombo), 1.5)
		longestV1ComboPortion := math.Pow(float64(replay.MaxCombo), 2)
		longestStandardisedComboPortion := math.Pow(float64(replay.MaxCombo), 1.5)
		v1ComboPortion := math.Max(maximumV1ComboPortion*comboProportion/accuracy, longestV1ComboPortion)

		longestComboOccurrences := math.Floor(v1ComboPortion / longestV1ComboPortion)
		remainingCombo := math.Sqrt(v1ComboPortion - longestComboOccurrences*longestV1ComboPortion)
		scoreEstimate := longestComboOccurrences*longestStandardisedComboPortion + math.Pow(remainingCombo, 1.5)

		remainingObjectCount := float64(attrs.maxCombo - replay.MaxCombo - replay.CountMiss)
		remainingComboLength := 0.0
		if remainingObjectCount > 0 {
			remainingComboLength = (v1ComboPortion - longestV1ComboPortion) / remainingObjectCount
		}
		objectEstimate := longestStandardisedComboPortion + remainingObjectCount*math.Sqrt(math.Max(remainingComboLength, 0))
		scoreEstimate = min(max(scoreEstimate, 0), maximumStandardisedComboPortion)
		objectEstimate = min(max(objectEstimate, 0), maximumStandardisedComboPortion)
		lower, upper := math.Min(scoreEstimate, objectEstimate), math.Max(scoreEstimate, objectEstimate)
		estimate := math.Min(0.3*lower+0.7*upper, 1.2*(lower+upper)/2)

		convertedWithoutMods = 500000*estimate/maximumStandardisedComboPortion*accuracy +
			500000*math.Pow(accuracy, 5) + bonusProportion
	}

	withoutMods := math.RoundToEven(convertedWithoutMods)
	return int64(math.RoundToEven(withoutMods * modernModMultiplier))
}

func simulateLegacyMigrationAttributes(beatMap *beatmap.BeatMap, difficultyMultiplier float64) legacyMigrationAttributes {
	var attrs legacyMigrationAttributes
	var standardisedBonusScore int64
	combo := 0
	for _, object := range beatMap.HitObjects {
		switch hitObject := object.(type) {
		case *objects.Slider:
			attrs.accuracyScore += 330 // Slider summary and head
			combo++
			for _, point := range hitObject.ScorePointsLazer {
				if point.IsReverse || point.LastPoint {
					attrs.accuracyScore += 30
				} else {
					attrs.accuracyScore += 10
				}
				combo++
			}
		case *objects.Spinner:
			attrs.accuracyScore += 300
			const maxRotationsPerSecond = 477.0 / 60
			seconds := hitObject.GetDuration() / 1000
			totalHalfSpins := int(seconds * maxRotationsPerSecond * 2)
			beforeBonus := int(seconds*3) + 3
			for i := range totalHalfSpins + 1 {
				if i > beforeBonus && (i-beforeBonus)%2 == 0 {
					attrs.bonusScore += 1100
					standardisedBonusScore += 50
				} else if i > 1 && i%2 == 0 {
					attrs.bonusScore += 100
					standardisedBonusScore += 10
				}
			}
		default:
			attrs.accuracyScore += 300
		}
		attrs.comboScore += int64(float64(max(0, combo-1)) * (12 * difficultyMultiplier))
		if _, isSlider := object.(*objects.Slider); !isSlider {
			combo++
		}
	}
	attrs.maxCombo = combo
	if attrs.bonusScore > 0 {
		attrs.bonusScoreRatio = float64(standardisedBonusScore) / float64(attrs.bonusScore)
	}
	return attrs
}
