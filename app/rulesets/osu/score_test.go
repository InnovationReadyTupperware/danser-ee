package osu

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/api"
)

func TestScoreContinuesAggregatingAfterFailedGrade(t *testing.T) {
	score := Score{Grade: F, Accuracy: 1}
	score.AddResult(JudgementResult{HitResult: Miss, MaxResult: Hit300})
	score.AddResult(JudgementResult{HitResult: Hit300, MaxResult: Hit300})
	score.CalculateGrade(difficulty.GameplayLazer, 0)

	if score.Grade != F {
		t.Fatalf("grade after post-failure results = %v, want F", score.Grade)
	}
	if score.CountMiss != 1 || score.Count300 != 1 || score.scoredObjects != 2 {
		t.Fatalf("post-failure score counts = (300 %d, miss %d, objects %d), want (1, 1, 2)", score.Count300, score.CountMiss, score.scoredObjects)
	}

	perfScore := score.ToPerfScore()
	if perfScore.CountGreat != 1 || perfScore.CountMiss != 1 {
		t.Fatalf("post-failure performance counts = (great %d, miss %d), want (1, 1)", perfScore.CountGreat, perfScore.CountMiss)
	}
}

func TestFailurePolicyContinueKeepsScoreAndPerformanceStateUpdating(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayLazer)
	cursor := &graphics.Cursor{}
	player := &difficultyPlayer{
		cursor:             cursor,
		diff:               diff,
		difficultyCacheKey: difficultyCacheKey{mode: diff.GetGameplayMode(), mods: diff.GetModStringMasked()},
		failurePolicy:      FailurePolicyContinue,
	}
	performanceCalculator := &recordingPerformanceCalculator{}
	scoreProcessor := &recordingScoreProcessor{}
	ruleset := &OsuRuleSet{
		beatMap: &beatmap.BeatMap{},
		cursors: map[*graphics.Cursor]*subSet{},
		oppDiffs: map[difficultyCacheKey][]api.Attributes{
			player.difficultyCacheKey: {{ObjectCount: 1}},
		},
	}
	ruleset.cursors[cursor] = &subSet{
		player:         player,
		score:          &Score{Accuracy: 1},
		hp:             &HealthProcessorV2{player: player, health: 1},
		ppv2:           performanceCalculator,
		scoreProcessor: scoreProcessor,
	}

	ruleset.failInternal(player)
	ruleset.SendResult(cursor, JudgementResult{
		HitResult:   Hit300,
		MaxResult:   Hit300,
		ComboResult: Increase,
	})

	score := ruleset.GetScore(cursor)
	if score.Grade != F {
		t.Fatalf("continued score grade = %v, want F", score.Grade)
	}
	if score.Count300 != 1 || score.scoredObjects != 1 {
		t.Fatalf("continued score counts = (300 %d, objects %d), want (1, 1)", score.Count300, score.scoredObjects)
	}
	if scoreProcessor.addResults != 1 {
		t.Fatalf("score processor results = %d, want 1", scoreProcessor.addResults)
	}
	if performanceCalculator.calls != 1 || performanceCalculator.lastScore.CountGreat != 1 {
		t.Fatalf("performance updates = (%d calls, %d great), want (1, 1)", performanceCalculator.calls, performanceCalculator.lastScore.CountGreat)
	}
	if score.PP.Total != 1 {
		t.Fatalf("continued score PP = %g, want 1", score.PP.Total)
	}
}

type recordingScoreProcessor struct {
	addResults int
	score      int64
	combo      int64
	accuracy   float64
}

func (processor *recordingScoreProcessor) Init(*beatmap.BeatMap, *difficultyPlayer) {}

func (processor *recordingScoreProcessor) AddResult(result JudgementResult) {
	processor.addResults++
	if result.HitResult == Hit300 {
		processor.combo++
		processor.score += 300
	}
	processor.accuracy = 1
}

func (processor *recordingScoreProcessor) ModifyResult(result HitResult, _ HitObject) HitResult {
	return result
}

func (processor *recordingScoreProcessor) GetScore() int64 {
	return processor.score
}

func (processor *recordingScoreProcessor) GetCombo() int64 {
	return processor.combo
}

func (processor *recordingScoreProcessor) GetAccuracy() float64 {
	return processor.accuracy
}

type recordingPerformanceCalculator struct {
	calls     int
	lastScore api.PerfScore
}

func (calculator *recordingPerformanceCalculator) Calculate(_ api.Attributes, score api.PerfScore, _ *difficulty.Difficulty) api.PPv2Results {
	calculator.calls++
	calculator.lastScore = score
	return api.PPv2Results{Total: float64(score.CountGreat)}
}
