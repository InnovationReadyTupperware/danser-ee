package osu

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
)

type scoreProcessor interface {
	Init(beatMap *beatmap.BeatMap, player *difficultyPlayer)
	AddResult(result JudgementResult)
	ModifyResult(result HitResult, src HitObject) HitResult
	GetScore() int64
	GetCombo() int64
	GetAccuracy() float64
}

type Score struct {
	Score        int64
	Accuracy     float64
	Grade        Grade
	CurrentCombo uint
	Combo        uint
	PerfectCombo bool
	Count300     uint
	CountGeki    uint
	Count100     uint
	CountKatu    uint
	Count50      uint
	CountMiss    uint
	CountSB      uint
	MaxTicks     uint
	// SliderEnd and MaxSliderEnd are historical aggregate counters used by
	// older PP versions, accuracy reconstruction, and custom statistics. The
	// exact current-model tail/tick counters below intentionally overlap them.
	SliderEnd        uint
	MaxSliderEnd     uint
	SliderTickMisses uint
	SliderTailHits   uint
	PP               api.PPv2Results

	scoredObjects uint
}

func (s *Score) ToPerfScore(diff *difficulty.Difficulty) api.PerfScore {
	perfScore := api.PerfScore{
		Score:            int(s.Score),
		MaxCombo:         int(s.Combo),
		CountGreat:       int(s.Count300),
		CountOk:          int(s.Count100),
		CountMeh:         int(s.Count50),
		CountMiss:        int(s.CountMiss),
		SliderBreaks:     int(s.CountSB),
		SliderEnd:        int(s.SliderEnd),
		SliderTickMisses: int(s.SliderTickMisses),
		SliderTailHits:   int(s.SliderTailHits),
		Accuracy:         s.Accuracy,
	}

	if diff != nil && !diff.IsLazer() && !diff.Mods.Active(difficulty.ScoreV2) {
		legacyScore := s.Score
		perfScore.LegacyTotalScore = &legacyScore
	}

	return perfScore
}

func (s *Score) AddResult(result JudgementResult) {
	bResult := result.HitResult & BaseHitsM

	if bResult > 0 {
		switch bResult {
		case Hit300:
			s.Count300++
		case Hit100:
			s.Count100++
		case Hit50:
			s.Count50++
		case Miss:
			s.CountMiss++
		}

		s.scoredObjects++
	}

	if (result.HitResult & (SliderEnd | LegacySliderEnd | SmallTickHit | SliderTailHit)) > 0 {
		s.SliderEnd++
	}

	if result.HitResult&LargeTickMiss != 0 {
		s.SliderTickMisses++
	}

	if result.HitResult&SliderTailHit != 0 {
		s.SliderTailHits++
	}

	if result.ComboResult == Reset && result.HitResult != Miss { // skips missed slider "ends" as they don't reset combo
		s.CountSB++
	}

	if result.MaxResult&(SliderStart|SliderPoint|SliderRepeat|LargeTickHit) > 0 {
		s.MaxTicks++
	}

	if result.MaxResult&(LegacySliderEnd|SliderEnd|SmallTickHit|SliderTailHit) > 0 {
		s.MaxSliderEnd++
	}
}

func (s *Score) CalculateGrade(mode difficulty.GameplayMode, mods difficulty.Modifier) {
	// F is a terminal grade, not a terminal score state. AddResult and PP
	// calculation continue through SendResult after a generated failure.
	if s.Grade == F {
		return
	}

	var baseGrade Grade

	if mode.IsLazer() {
		baseGrade = s.gradeV2()
	} else {
		baseGrade = s.gradeV1()
	}

	if mods&(difficulty.Hidden|difficulty.Flashlight) > 0 {
		switch baseGrade {
		case S:
			baseGrade = SH
		case SS:
			baseGrade = SSH
		}
	}

	s.Grade = baseGrade
}

func (s *Score) gradeV1() Grade {
	ratio := 1.0
	if s.scoredObjects > 0 {
		ratio = float64(s.Count300) / float64(s.scoredObjects)
	}

	if s.Count300 == s.scoredObjects {
		return SS
	} else if ratio > 0.9 && float64(s.Count50)/float64(s.scoredObjects) < 0.01 && s.CountMiss == 0 {
		return S
	} else if ratio > 0.8 && s.CountMiss == 0 || ratio > 0.9 {
		return A
	} else if ratio > 0.7 && s.CountMiss == 0 || ratio > 0.8 {
		return B
	} else if ratio > 0.6 {
		return C
	}

	return D
}

func (s *Score) gradeV2() Grade {
	if s.Accuracy == 1 && s.CountMiss == 0 {
		return SS
	} else if s.Accuracy >= 0.95 && s.CountMiss == 0 {
		return S
	} else if s.Accuracy >= 0.9 {
		return A
	} else if s.Accuracy >= 0.8 {
		return B
	} else if s.Accuracy >= 0.7 {
		return C
	}

	return D
}
