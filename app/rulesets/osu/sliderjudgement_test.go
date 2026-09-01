package osu

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
)

func TestHitResultLazerTaxonomy(t *testing.T) {
	tests := []struct {
		name     string
		result   HitResult
		score    int64
		accuracy bool
		combo    bool
		isTick   bool
		isHit    bool
		isMiss   bool
		scorable bool
	}{
		{name: "hit 300", result: Hit300, score: 300, accuracy: true, combo: true, isHit: true, scorable: true},
		{name: "hit 100", result: Hit100, score: 100, accuracy: true, combo: true, isHit: true, scorable: true},
		{name: "hit 50", result: Hit50, score: 50, accuracy: true, combo: true, isHit: true, scorable: true},
		{name: "large tick hit", result: LargeTickHit, score: 30, accuracy: true, combo: true, isTick: true, isHit: true, scorable: true},
		{name: "large tick miss", result: LargeTickMiss, accuracy: true, combo: true, isTick: true, isMiss: true, scorable: true},
		{name: "small tick hit", result: SmallTickHit, score: 10, accuracy: true, isTick: true, isHit: true, scorable: true},
		{name: "small tick miss", result: SmallTickMiss, accuracy: true, isTick: true, isMiss: true, scorable: true},
		{name: "slider tail hit", result: SliderTailHit, score: 150, accuracy: true, combo: true, isTick: true, isHit: true, scorable: true},
		{name: "ignored slider tail", result: IgnoreMiss, isMiss: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.ScoreValueLazer(); got != tt.score {
				t.Fatalf("ScoreValueLazer() = %d, want %d", got, tt.score)
			}
			if got := tt.result.AffectsAccLazer(); got != tt.accuracy {
				t.Fatalf("AffectsAccLazer() = %t, want %t", got, tt.accuracy)
			}
			if got := tt.result.AffectsCombo(); got != tt.combo {
				t.Fatalf("AffectsCombo() = %t, want %t", got, tt.combo)
			}
			if got := tt.result.IsTick(); got != tt.isTick {
				t.Fatalf("IsTick() = %t, want %t", got, tt.isTick)
			}
			if got := tt.result.IsHit(); got != tt.isHit {
				t.Fatalf("IsHit() = %t, want %t", got, tt.isHit)
			}
			if got := tt.result.IsMiss(); got != tt.isMiss {
				t.Fatalf("IsMiss() = %t, want %t", got, tt.isMiss)
			}
			if got := tt.result.IsScorable(); got != tt.scorable {
				t.Fatalf("IsScorable() = %t, want %t", got, tt.scorable)
			}
		})
	}
}

func TestHitWindowModeBoundaries(t *testing.T) {
	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	player := &difficultyPlayer{diff: diff}
	ruleset := &OsuRuleSet{}

	lazerTests := []struct {
		name  string
		delta float64
		want  HitResult
	}{
		{name: "great boundary", delta: diff.Hit300U, want: Hit300},
		{name: "ok boundary", delta: diff.Hit100U, want: Hit100},
		{name: "meh boundary", delta: diff.Hit50U, want: Hit50},
		{name: "past meh boundary", delta: diff.Hit50U + 0.001, want: Miss},
	}

	for _, tt := range lazerTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleset.GetResultForDelta(player, tt.delta); got != tt.want {
				t.Fatalf("Lazer GetResultForDelta(%g) = %v, want %v", tt.delta, got, tt.want)
			}
		})
	}

	diff.SetGameplayMode(difficulty.GameplayStable)
	stableTests := []struct {
		name  string
		delta float64
		want  HitResult
	}{
		{name: "just inside great", delta: float64(diff.Hit300) - 0.001, want: Hit300},
		{name: "great boundary", delta: float64(diff.Hit300), want: Hit100},
		{name: "ok boundary", delta: float64(diff.Hit100), want: Hit50},
		{name: "meh boundary", delta: float64(diff.Hit50), want: Miss},
	}

	for _, tt := range stableTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleset.GetResultForDelta(player, tt.delta); got != tt.want {
				t.Fatalf("Stable GetResultForDelta(%g) = %v, want %v", tt.delta, got, tt.want)
			}
		})
	}
}

func TestHitErrorResultTaxonomy(t *testing.T) {
	tests := []struct {
		name   string
		result JudgementResult
		want   bool
	}{
		{name: "circle hit", result: JudgementResult{HitResult: Hit300}, want: true},
		{name: "stable slider head", result: JudgementResult{HitResult: SliderStart}, want: true},
		{name: "lazer slider head", result: JudgementResult{HitResult: Hit100, sliderPart: sliderPartHead}, want: true},
		{name: "classic slider head", result: JudgementResult{HitResult: LargeTickHit, sliderPart: sliderPartHead}, want: true},
		{name: "ordinary miss", result: JudgementResult{HitResult: Miss}, want: false},
		{name: "stable slider miss", result: JudgementResult{HitResult: SliderMiss, sliderPart: sliderPartHead}, want: false},
		{name: "nested tick", result: JudgementResult{HitResult: LargeTickHit, sliderPart: sliderPartTick}, want: false},
		{name: "slider tail", result: JudgementResult{HitResult: SliderTailHit, sliderPart: sliderPartTail}, want: false},
		{name: "positional miss", result: JudgementResult{HitResult: PositionalMiss}, want: false},
		{name: "ignored miss", result: JudgementResult{HitResult: IgnoreMiss}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.AffectsHitError(); got != tt.want {
				t.Fatalf("AffectsHitError() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSliderJudgementResultParts(t *testing.T) {
	tests := []struct {
		name    string
		part    sliderJudgementPart
		isHead  bool
		nested  bool
		tick    bool
		repeat  bool
		tail    bool
		summary bool
	}{
		{name: "head", part: sliderPartHead, isHead: true},
		{name: "tick", part: sliderPartTick, nested: true, tick: true},
		{name: "repeat", part: sliderPartRepeat, nested: true, repeat: true},
		{name: "tail", part: sliderPartTail, nested: true, tail: true},
		{name: "summary", part: sliderPartSummary, summary: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JudgementResult{sliderPart: tt.part}
			if result.IsSliderHead() != tt.isHead {
				t.Fatalf("IsSliderHead() = %t, want %t", result.IsSliderHead(), tt.isHead)
			}
			if result.IsSliderNested() != tt.nested {
				t.Fatalf("IsSliderNested() = %t, want %t", result.IsSliderNested(), tt.nested)
			}
			if result.IsSliderTick() != tt.tick {
				t.Fatalf("IsSliderTick() = %t, want %t", result.IsSliderTick(), tt.tick)
			}
			if result.IsSliderRepeat() != tt.repeat {
				t.Fatalf("IsSliderRepeat() = %t, want %t", result.IsSliderRepeat(), tt.repeat)
			}
			if result.IsSliderTail() != tt.tail {
				t.Fatalf("IsSliderTail() = %t, want %t", result.IsSliderTail(), tt.tail)
			}
			if result.IsSliderSummary() != tt.summary {
				t.Fatalf("IsSliderSummary() = %t, want %t", result.IsSliderSummary(), tt.summary)
			}
		})
	}
}

func TestBuildSliderEvents(t *testing.T) {
	slider := &objects.Slider{
		HitObject: &objects.HitObject{StartTime: 100, EndTime: 500},
		ScorePoints: []objects.TickPoint{
			{Time: 200.75},
			{Time: 300, IsReverse: true, EdgeIndex: 1},
			{Time: 500, IsReverse: true, LastPoint: true, EdgeIndex: 2},
		},
		ScorePointsLazer: []objects.TickPoint{
			{Time: 200},
			{Time: 300, IsReverse: true},
			{Time: 500, LastPoint: true},
		},
	}

	stable := buildStableSliderEvents(slider)
	if len(stable) != 3 {
		t.Fatalf("stable event count = %d, want 3", len(stable))
	}
	if stable[0].kind != sliderPointTick || stable[1].kind != sliderPointRepeat || stable[2].kind != sliderPointTail {
		t.Fatalf("stable event kinds = %#v, want tick/repeat/tail", []sliderPointKind{stable[0].kind, stable[1].kind, stable[2].kind})
	}
	if stable[2].time != 464 {
		t.Fatalf("stable tail time = %g, want 464", stable[2].time)
	}
	if stable[2].maxResult != SliderEnd {
		t.Fatalf("stable tail max result = %v, want SliderEnd", stable[2].maxResult)
	}
	if stable[0].time != 200 {
		t.Fatalf("stable fractional tick time = %g, want 200", stable[0].time)
	}

	lazer := buildLazerSliderEvents(slider, false)
	if lazer[0].maxResult != LargeTickHit || lazer[1].maxResult != LargeTickHit || lazer[2].maxResult != SliderTailHit {
		t.Fatalf("Lazer max results = %#v, want large/large/tail", []HitResult{lazer[0].maxResult, lazer[1].maxResult, lazer[2].maxResult})
	}
	if lazer[2].time != 500 {
		t.Fatalf("Lazer tail time = %g, want 500", lazer[2].time)
	}

	classic := buildLazerSliderEvents(slider, true)
	if classic[2].maxResult != SmallTickHit {
		t.Fatalf("Classic tail max result = %v, want SmallTickHit", classic[2].maxResult)
	}
}

func TestLazerSliderUsesUnflooredEndTime(t *testing.T) {
	slider := &objects.Slider{
		HitObject:    &objects.HitObject{StartTime: 100, EndTime: 500},
		EndTimeLazer: 500.75,
	}

	if got := lazerSliderEndTime(slider); got != 500.75 {
		t.Fatalf("Lazer slider end time = %g, want 500.75", got)
	}

	slider.EndTimeLazer = 0
	if got := lazerSliderEndTime(slider); got != 500 {
		t.Fatalf("fallback Lazer slider end time = %g, want 500", got)
	}
}

func TestLazerSliderTailTimingAndOrdering(t *testing.T) {
	tail := sliderEvent{time: 500, kind: sliderPointTail, maxResult: SliderTailHit}

	if lazerSliderEventDue(tail, 463.9) {
		t.Fatal("tail became due before the -36 ms leniency boundary")
	}
	if !lazerSliderEventDue(tail, 464) {
		t.Fatal("tail did not become due at the -36 ms leniency boundary")
	}
	if lazerSliderTailMayBeJudged(tail, 480, false) {
		t.Fatal("tail was allowed before earlier nested events were judged")
	}
	if !lazerSliderTailMayBeJudged(tail, 480, true) {
		t.Fatal("tail was not allowed inside its leniency window after prior events")
	}
	if lazerSliderEventResult(tail, false) != IgnoreMiss {
		t.Fatal("dropped non-classic tail did not produce IgnoreMiss")
	}

	classicTail := tail
	classicTail.maxResult = SmallTickHit
	if lazerSliderEventResult(classicTail, false) != SmallTickMiss {
		t.Fatal("dropped classic tail did not produce SmallTickMiss")
	}
	if lazerSliderEventResult(tail, true) != SliderTailHit {
		t.Fatal("tracked tail did not produce SliderTailHit")
	}
}

func TestClassicSliderCollapse(t *testing.T) {
	tests := []struct {
		hitCount   int
		totalCount int
		want       HitResult
	}{
		{hitCount: 0, totalCount: 4, want: Miss},
		{hitCount: 1, totalCount: 4, want: Hit50},
		{hitCount: 2, totalCount: 4, want: Hit100},
		{hitCount: 3, totalCount: 4, want: Hit100},
		{hitCount: 4, totalCount: 4, want: Hit300},
	}

	for _, tt := range tests {
		if got := classicSliderCollapse(tt.hitCount, tt.totalCount); got != tt.want {
			t.Errorf("classicSliderCollapse(%d, %d) = %v, want %v", tt.hitCount, tt.totalCount, got, tt.want)
		}
	}
}

func TestSliderHeadResultMapping(t *testing.T) {
	if got, maxResult := stableSliderHeadResults(Hit100); got != SliderStart || maxResult != SliderStart {
		t.Fatalf("stable hit mapping = %v/%v, want SliderStart/SliderStart", got, maxResult)
	}
	if got, maxResult := stableSliderHeadResults(Miss); got != SliderMiss || maxResult != SliderStart {
		t.Fatalf("stable miss mapping = %v/%v, want SliderMiss/SliderStart", got, maxResult)
	}
	if got, maxResult := lazerSliderHeadResults(false, Hit100); got != Hit100 || maxResult != Hit300 {
		t.Fatalf("Lazer accurate head mapping = %v/%v, want Hit100/Hit300", got, maxResult)
	}
	if got, maxResult := lazerSliderHeadResults(false, Miss); got != Miss || maxResult != Hit300 {
		t.Fatalf("Lazer accurate miss mapping = %v/%v, want Miss/Hit300", got, maxResult)
	}
	if got, maxResult := lazerSliderHeadResults(true, Hit100); got != LargeTickHit || maxResult != LargeTickHit {
		t.Fatalf("Classic head mapping = %v/%v, want LargeTickHit/LargeTickHit", got, maxResult)
	}
	if got, maxResult := lazerSliderHeadResults(true, Miss); got != LargeTickMiss || maxResult != LargeTickHit {
		t.Fatalf("Classic head miss mapping = %v/%v, want LargeTickMiss/LargeTickHit", got, maxResult)
	}
}

func TestScoreV3InitializesLazerSliderMaximums(t *testing.T) {
	slider := &objects.Slider{
		HitObject: &objects.HitObject{StartTime: 100, EndTime: 500},
		ScorePointsLazer: []objects.TickPoint{
			{Time: 200},
			{Time: 300, IsReverse: true},
			{Time: 500, LastPoint: true},
		},
	}
	beatMap := &beatmap.BeatMap{HitObjects: []objects.IHitObject{slider}}

	accuratePlayer := &difficultyPlayer{diff: difficulty.NewDifficulty(5, 5, 5, 5)}
	accurate := newScoreV3Processor(false)
	accurate.Init(beatMap, accuratePlayer)

	if accurate.maxHits != 4 {
		t.Fatalf("accurate max hits = %d, want 4", accurate.maxHits)
	}

	classicPlayer := &difficultyPlayer{
		diff:                        difficulty.NewDifficulty(5, 5, 5, 5),
		classicNoSliderHeadAccuracy: true,
	}
	classic := newScoreV3Processor(false)
	classic.Init(beatMap, classicPlayer)

	if classic.maxHits != 5 {
		t.Fatalf("classic max hits = %d, want 5", classic.maxHits)
	}
}

func TestScoreV3CountsIgnoredTailInAccuracyMaximum(t *testing.T) {
	processor := &scoreV3Processor{
		maxHits:       1,
		comboPartMax:  1,
		modMultiplier: 1,
	}

	processor.AddResult(JudgementResult{HitResult: IgnoreMiss, MaxResult: SliderTailHit})

	if processor.accPartMax != 150 {
		t.Fatalf("ignored tail maximum = %d, want 150", processor.accPartMax)
	}
	if processor.hits != 1 {
		t.Fatalf("ignored tail hit count = %d, want 1", processor.hits)
	}
	if processor.GetAccuracy() != 0 {
		t.Fatalf("ignored tail accuracy = %g, want 0", processor.GetAccuracy())
	}
}

func TestScoreTracksLazerSliderSummaryFields(t *testing.T) {
	score := Score{}
	score.AddResult(JudgementResult{HitResult: LargeTickHit, MaxResult: LargeTickHit})
	score.AddResult(JudgementResult{HitResult: SliderTailHit, MaxResult: SliderTailHit})
	score.AddResult(JudgementResult{HitResult: SmallTickMiss, MaxResult: SmallTickHit})

	if score.MaxTicks != 1 {
		t.Fatalf("max ticks = %d, want 1", score.MaxTicks)
	}
	if score.SliderEnd != 1 {
		t.Fatalf("successful slider ends = %d, want 1", score.SliderEnd)
	}
	if score.MaxSliderEnd != 2 {
		t.Fatalf("maximum slider ends = %d, want 2", score.MaxSliderEnd)
	}
}

func TestHealthProcessorV2SliderResultValues(t *testing.T) {
	processor := &HealthProcessorV2{player: &difficultyPlayer{diff: difficulty.NewDifficulty(5, 5, 5, 5)}}

	tests := []struct {
		name   string
		result HitResult
		part   sliderJudgementPart
		want   float64
	}{
		{name: "large tick miss", result: LargeTickMiss, want: -0.075},
		{name: "small tick miss", result: SmallTickMiss, want: -0.075},
		{name: "ignored tail", result: IgnoreMiss, want: 0},
		{name: "large tick", result: LargeTickHit, part: sliderPartTick, want: 0.015},
		{name: "repeat", result: LargeTickHit, part: sliderPartRepeat, want: 0.02},
		{name: "classic tail", result: SmallTickHit, part: sliderPartTail, want: 0.02},
		{name: "Lazer tail", result: SliderTailHit, part: sliderPartTail, want: 0.02},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processor.getHPResultForPart(tt.result, tt.part); got != tt.want {
				t.Fatalf("getHPResultForPart() = %g, want %g", got, tt.want)
			}
		})
	}
}

func TestHealthProcessorV2CarriesSliderMissQualityToTail(t *testing.T) {
	beatmapSlider := &objects.Slider{
		HitObject: &objects.HitObject{
			NewCombo:    true,
			LastInCombo: true,
		},
	}
	runtimeSlider := &Slider{hitSlider: beatmapSlider}
	processor := &HealthProcessorV2{
		player: &difficultyPlayer{diff: difficulty.NewDifficulty(5, 5, 5, 5)},
		health: 0.5,
	}

	processor.AddResult(JudgementResult{
		HitResult:   LargeTickMiss,
		ComboResult: Reset,
		object:      runtimeSlider,
		sliderPart:  sliderPartTick,
	})
	processor.AddResult(JudgementResult{
		HitResult:   SliderTailHit,
		ComboResult: Increase,
		object:      runtimeSlider,
		sliderPart:  sliderPartTail,
	})

	// The large tick miss is a Good-quality combo result (-0.075). The
	// successful tail receives its normal +0.02 and combo-end +0.05 bonus,
	// for a final health of 0.495.
	if math.Abs(processor.GetHealth()-0.495) > 1e-9 {
		t.Fatalf("health after slider miss and tail = %g, want 0.495", processor.GetHealth())
	}
}

func TestHealthProcessorV2RetainsCircleComboEndBonus(t *testing.T) {
	beatmapCircle := &objects.Circle{
		HitObject: &objects.HitObject{
			NewCombo:    true,
			LastInCombo: true,
		},
	}
	runtimeCircle := &Circle{hitCircle: beatmapCircle}
	processor := &HealthProcessorV2{
		player: &difficultyPlayer{diff: difficulty.NewDifficulty(5, 5, 5, 5)},
		health: 0.5,
	}

	processor.AddResult(JudgementResult{
		HitResult:   Hit300,
		ComboResult: Increase,
		object:      runtimeCircle,
	})

	// A clean last-combo circle receives the normal +0.03 and combo-end
	// recovery +0.07. This guards the non-slider branch while the slider
	// taxonomy adds its own summary and tail end conditions.
	if math.Abs(processor.GetHealth()-0.6) > 1e-9 {
		t.Fatalf("health after last-combo circle = %g, want 0.6", processor.GetHealth())
	}
}
