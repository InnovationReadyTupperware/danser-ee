package osu

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

type HitResult int64

const (
	Ignore          = HitResult(0)
	SliderMiss      = HitResult(1 << 0)
	Miss            = HitResult(1 << 1)
	Hit50           = HitResult(1 << 2)
	Hit100          = HitResult(1 << 3)
	Hit300          = HitResult(1 << 4)
	SliderStart     = HitResult(1 << 5)
	SliderPoint     = HitResult(1 << 6)
	SliderRepeat    = HitResult(1 << 7)
	LegacySliderEnd = HitResult(1 << 8)
	SliderEnd       = HitResult(1 << 9)
	// Bit 10 was the former SliderFinish sentinel. It remains unused so the
	// historical positions of spinner and addition flags are not shifted.
	SpinnerSpin    = HitResult(1 << 11)
	SpinnerPoints  = HitResult(1 << 12)
	SpinnerBonus   = HitResult(1 << 13)
	MuAddition     = HitResult(1 << 14)
	KatuAddition   = HitResult(1 << 15)
	GekiAddition   = HitResult(1 << 16)
	PositionalMiss = HitResult(1 << 17)
	// These result values intentionally come after the historical values above.
	// HitResult is internal, but keeping the old bit positions stable makes the
	// change safer for callers which retain a result mask between frames.
	IgnoreMiss         = HitResult(1 << 18)
	SmallTickHit       = HitResult(1 << 19)
	SmallTickMiss      = HitResult(1 << 20)
	LargeTickHit       = HitResult(1 << 21)
	LargeTickMiss      = HitResult(1 << 22)
	SliderTailHit      = HitResult(1 << 23)
	Additions          = MuAddition | KatuAddition | GekiAddition
	Hit50m             = Hit50 | MuAddition
	Hit100m            = Hit100 | MuAddition
	Hit300m            = Hit300 | MuAddition
	Hit100k            = Hit100 | KatuAddition
	Hit300k            = Hit300 | KatuAddition
	Hit300g            = Hit300 | GekiAddition
	BaseHits           = Hit50 | Hit100 | Hit300
	BaseHitsM          = BaseHits | Miss
	HitValues          = Hit50 | Hit100 | Hit300 | GekiAddition | KatuAddition
	SliderHits         = SliderStart | SliderPoint | SliderRepeat | LegacySliderEnd | SliderEnd
	LazerSliderResults = SmallTickHit | SmallTickMiss | LargeTickHit | LargeTickMiss | SliderTailHit | IgnoreMiss
	SliderResults      = SliderMiss | SliderHits | LazerSliderResults
	SpinnerHits        = SpinnerSpin | SpinnerPoints | SpinnerBonus
	RawHits            = SliderHits | SpinnerHits
)

func (r HitResult) IsBonus() bool {
	v := r & (^Additions)

	return v&(SpinnerPoints|SpinnerBonus) != 0
}

func (r HitResult) AffectsAccV1() bool {
	v := r & (^Additions)

	return v&(BaseHitsM) != 0
}

func (r HitResult) AffectsAccLazer() bool {
	v := r & (^Additions)

	return v&(BaseHitsM|SmallTickHit|SmallTickMiss|LargeTickHit|LargeTickMiss|SliderTailHit) != 0
}

// IsHit reports whether r is a successful judgement result. IgnoreMiss is
// deliberately not a hit: it records a processed but unscorable miss.
func (r HitResult) IsHit() bool {
	v := r & (^Additions)

	switch v {
	case Ignore, IgnoreMiss, PositionalMiss, SliderMiss, Miss, SmallTickMiss, LargeTickMiss:
		return false
	default:
		return true
	}
}

// IsMiss reports whether r represents a miss which can be relevant to
// gameplay. PositionalMiss is a diagnostic marker and is handled separately.
func (r HitResult) IsMiss() bool {
	v := r & (^Additions)

	switch v {
	case SliderMiss, Miss, SmallTickMiss, LargeTickMiss, IgnoreMiss:
		return true
	default:
		return false
	}
}

// IsScorable reports whether r contributes a score or accuracy denominator.
// IgnoreMiss is kept as an explicit result because osu!lazer emits it for a
// dropped non-classic slider tail, but it must not change score or accuracy.
func (r HitResult) IsScorable() bool {
	v := r & (^Additions)

	switch v {
	case Miss, Hit50, Hit100, Hit300,
		SmallTickHit, SmallTickMiss, LargeTickHit, LargeTickMiss, SliderTailHit:
		return true
	default:
		return false
	}
}

// AffectsHitError reports whether a judgement is a successful real-object
// timing result that can feed the hit-error meter. Slider heads are allowed
// through their explicit slider part so Lazer Classic's LargeTickHit head is
// retained, while nested slider results remain excluded. SliderStart is also
// accepted without a part for the historical Stable result representation.
func (result JudgementResult) AffectsHitError() bool {
	v := result.HitResult & (^Additions)

	if result.IsSliderHead() || v == SliderStart {
		return result.HitResult.IsHit()
	}

	return v&BaseHits != 0
}

// IsTick reports whether r is one of osu!lazer's slider tick result kinds.
// SliderTailHit is tick-like for accuracy and health even though it has its
// own score weight.
func (r HitResult) IsTick() bool {
	v := r & (^Additions)

	switch v {
	case SmallTickHit, SmallTickMiss, LargeTickHit, LargeTickMiss, SliderTailHit:
		return true
	default:
		return false
	}
}

// AffectsCombo reports whether a Lazer result changes combo. Small tick
// results intentionally do not affect combo, matching osu!lazer's classic
// slider tail behavior.
func (r HitResult) AffectsCombo() bool {
	v := r & (^Additions)

	switch v {
	case Miss, Hit50, Hit100, Hit300, LargeTickHit, LargeTickMiss, SliderTailHit:
		return true
	default:
		return false
	}
}

func (r HitResult) ScoreValue() int64 {
	v := r & (^Additions)
	switch v {
	case Hit50:
		return 50
	case Hit100, SpinnerPoints:
		return 100
	case Hit300:
		return 300
	case SliderStart, SliderRepeat, SliderEnd:
		return 30
	case SliderPoint:
		return 10
	case SpinnerBonus:
		return 1100
	}

	return 0
}

func (r HitResult) ScoreValueV2() int64 {
	if r&SpinnerBonus > 0 {
		return 500
	}

	return r.ScoreValue()
}

func (r HitResult) ScoreValueLazer() int64 {
	v := r & (^Additions)
	switch v {
	case Hit50:
		return 50
	case Hit100:
		return 100
	case SliderEnd:
		return 150
	case SliderTailHit:
		return 150
	case Hit300:
		return 300
	case SliderStart, SliderPoint, SliderRepeat:
		return 30
	case LargeTickHit:
		return 30
	case SmallTickHit:
		return 10
	case SpinnerPoints, LegacySliderEnd:
		return 10
	case SpinnerBonus:
		return 50
	}

	return 0
}

func (r HitResult) ScoreValueFor(mode difficulty.GameplayMode, mod difficulty.Modifier) int64 {
	if mode.IsLazer() {
		return r.ScoreValueLazer()
	} else if mod.Active(difficulty.ScoreV2) {
		return r.ScoreValueV2()
	}

	return r.ScoreValue()
}

type ComboResult uint8

const (
	Reset = ComboResult(iota)
	Hold
	Increase
)

type JudgementResult struct {
	HitResult HitResult
	MaxResult HitResult

	ComboResult ComboResult

	Time     int64
	Position vector.Vector2f

	Number int64
	object HitObject

	sliderPart sliderJudgementPart
	catchUp    bool
}

type sliderJudgementPart uint8

const (
	sliderPartNone sliderJudgementPart = iota
	sliderPartHead
	sliderPartTick
	sliderPartRepeat
	sliderPartTail
	sliderPartSummary
)

// IsSliderHead reports whether this result belongs to a slider head. It is
// used by HUD consumers to include classic binary heads in timing displays
// without displaying nested tick results as ordinary hit objects.
func (result JudgementResult) IsSliderHead() bool {
	return result.sliderPart == sliderPartHead
}

// IsSliderNested reports whether this result belongs to a slider tick,
// repeat, or tail rather than the parent slider judgement.
func (result JudgementResult) IsSliderNested() bool {
	return result.sliderPart >= sliderPartTick && result.sliderPart <= sliderPartTail
}

// IsSliderTick reports whether this result belongs to a slider tick.
func (result JudgementResult) IsSliderTick() bool {
	return result.sliderPart == sliderPartTick
}

// IsSliderRepeat reports whether this result belongs to a slider repeat.
func (result JudgementResult) IsSliderRepeat() bool {
	return result.sliderPart == sliderPartRepeat
}

// IsSliderTail reports whether this result belongs to a slider tail.
func (result JudgementResult) IsSliderTail() bool {
	return result.sliderPart == sliderPartTail
}

// IsSliderSummary reports whether this is the final Classic slider collapse
// result, rather than one of the nested slider events.
func (result JudgementResult) IsSliderSummary() bool {
	return result.sliderPart == sliderPartSummary
}

// IsCatchUp reports whether the result was produced while reconstructing
// ruleset state after an initial timeline seek. Catch-up results still own
// score, health, and statistics, but presentation consumers must not replay
// their short-lived hit animations at the seek target.
func (result JudgementResult) IsCatchUp() bool {
	return result.catchUp
}

func createJudgementResult(result HitResult, maxResult HitResult, comboResult ComboResult, time int64, position vector.Vector2f, obj HitObject) JudgementResult {
	nm := int64(-1)
	if obj != nil {
		nm = obj.GetNumber()
	}

	return JudgementResult{
		HitResult:   result,
		MaxResult:   maxResult,
		ComboResult: comboResult,
		Time:        time,
		Position:    position,
		Number:      nm,
		object:      obj,
	}
}

func createSliderJudgementResult(result HitResult, maxResult HitResult, comboResult ComboResult, time int64, position vector.Vector2f, obj HitObject, part sliderJudgementPart) JudgementResult {
	jResult := createJudgementResult(result, maxResult, comboResult, time, position, obj)
	jResult.sliderPart = part

	return jResult
}
