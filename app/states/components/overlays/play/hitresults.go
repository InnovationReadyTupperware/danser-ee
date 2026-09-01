package play

import (
	"math"
	"math/rand"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/skin"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

type HitResults struct {
	bottom   *sprite.Manager
	top      *sprite.Manager
	lastTime float64
	diff     *difficulty.Difficulty
	color    color2.Color
	alpha    float64
}

const (
	sliderLegacyJudgmentMarkerFadeIn       = 120.0
	sliderLegacyJudgmentMarkerFadeOutDelay = 250.0
	sliderLegacyJudgmentMarkerFadeOut      = 600.0
	sliderLegacyJudgmentMarkerScaleTime    = 100.0
)

type sliderJudgmentMarker struct {
	textureName string
}

type judgmentPresentationKind uint8

const (
	judgmentPresentationNone judgmentPresentationKind = iota
	judgmentPresentationHit
	judgmentPresentationSliderMarker
)

func NewHitResults(diff *difficulty.Difficulty) *HitResults {
	// Preload all frames to avoid stalling during gameplay
	skin.GetFrames("hit0", true)
	skin.GetFrames("hit50", true)
	skin.GetFrames("hit100", true)
	skin.GetFrames("hit100k", true)
	skin.GetFrames("hit300", true)
	skin.GetFrames("hit300k", true)
	skin.GetFrames("hit300g", true)
	skin.GetFrames("slidertickmiss", true)
	skin.GetFrames("sliderendmiss", true)

	return &HitResults{
		bottom: sprite.NewManager(),
		top:    sprite.NewManager(),
		diff:   diff,
	}
}

// AddJudgmentResult adds the visual effects associated with a gameplay
// judgment. Slider-part results are deliberately accepted here as well as
// ordinary hit results: Lazer exposes missed slider parts as independent
// judgments, and Stable slider misses are mapped to the same legacy markers.
func (results *HitResults) AddJudgmentResult(judgement osu.JudgementResult, object objects.IHitObject) {
	if judgement.IsCatchUp() {
		return
	}

	switch classifyJudgment(judgement) {
	case judgmentPresentationHit:
		results.addHitResult(judgement.Time, judgement.HitResult, judgement.Position.Copy64(), object)
	case judgmentPresentationSliderMarker:
		results.addSliderJudgmentMarker(judgement)
	}
}

// AddResult preserves the original hit-result API for callers that do not
// have the complete judgement metadata required by Lazer slider markers.
func (results *HitResults) AddResult(time int64, result osu.HitResult, position vector.Vector2d, object objects.IHitObject) {
	results.addHitResult(time, result, position, object)
}

func (results *HitResults) addHitResult(time int64, result osu.HitResult, position vector.Vector2d, object objects.IHitObject) {
	var tex string
	var particle string
	baseResult := result &^ osu.Additions

	switch baseResult {
	case osu.Hit300:
		tex = "hit300"
		particle = "particle300"
	case osu.Hit100:
		tex = "hit100"
		particle = "particle100"
	case osu.Hit50:
		tex = "hit50"
		particle = "particle50"
	case osu.Miss:
		tex = "hit0"
	case osu.SliderMiss:
		tex = "hit0"
	}

	switch result & osu.Additions {
	case osu.KatuAddition:
		tex += "k"
	case osu.GekiAddition:
		tex += "g"
	}

	if tex == "" {
		return
	}

	frames := skin.GetFrames(tex, true)

	particles := false

	if particle != "" && len(frames) > 0 {
		particleTex := skin.GetTextureSource(particle, skin.GetSourceFromTexture(frames[0]))

		if particleTex != nil {
			particles = true

			for range 150 {
				fadeOut := 500 + 700*rand.Float64()
				direction := vector.NewVec2dRad(rand.Float64()*2*math.Pi, rand.Float64()*35)

				sp := sprite.NewSpriteSingle(particleTex, float64(time)+0.5, position, vector.Centre)
				sp.SetAdditive(true)
				sp.AddTransform(animation.NewSingleTransform(animation.Fade, easing.OutQuad, float64(time), float64(time)+fadeOut, 1.0, 0.0))
				sp.AddTransform(animation.NewVectorTransformV(animation.Move, easing.OutQuad, float64(time), float64(time)+fadeOut, position, position.Add(direction)))
				sp.ResetValuesToTransforms()
				sp.AdjustTimesToTransformations()
				sp.ShowForever(false)

				results.bottom.Add(sp)
			}
		}
	}

	hit := sprite.NewAnimation(frames, 1000.0/60, false, float64(time)+1, position, vector.Centre)
	hit.ShowForever(false)

	fadeIn := float64(time + difficulty.ResultFadeIn)
	if particles {
		fadeIn = float64(time + 80)
	}

	postEmpt := float64(time + difficulty.PostEmpt)
	fadeOut := postEmpt + float64(difficulty.ResultFadeOut)

	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Fade, easing.Linear, float64(time), fadeIn, 0.0, 1.0))
	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Fade, easing.Linear, postEmpt, fadeOut, 1.0, 0.0))

	if len(frames) == 1 {
		if particles {
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, float64(time), fadeOut, 0.9, 1.05))
		} else if baseResult == osu.Miss || baseResult == osu.SliderMiss {
			addLegacyMissTransforms(hit, float64(time), fadeIn, fadeOut, position)
		} else {
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, float64(time), float64(time+difficulty.ResultFadeIn*0.8), 0.6, 1.1))
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, fadeIn, float64(time+difficulty.ResultFadeIn*1.2), 1.1, 0.9))
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, float64(time+difficulty.ResultFadeIn*1.2), float64(time+difficulty.ResultFadeIn*1.4), 0.9, 1.0))
		}
	}

	hit.SortTransformations()
	hit.AdjustTimesToTransformations()
	hit.ResetValuesToTransforms()

	results.top.Add(hit)

	if !settings.Gameplay.ShowHitLighting || object == nil || baseResult&osu.BaseHits == 0 {
		return
	}

	lighting := sprite.NewSpriteSingle(skin.GetTexture("lighting"), float64(time), position, vector.Centre)
	lighting.SetColor(skin.GetObjectColor(int(object.GetComboSet()), int(object.GetComboSetHax()), results.color))
	lighting.SetAdditive(true)
	lighting.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.OutQuad, float64(time), float64(time+600), 0.8, 1.2))
	lighting.AddTransformUnordered(animation.NewSingleTransform(animation.Fade, easing.Linear, float64(time), float64(time+200), 0, 1))
	lighting.AddTransformUnordered(animation.NewSingleTransform(animation.Fade, easing.Linear, float64(time+400), float64(time+1400), 1, 0))

	results.bottom.Add(lighting)
}

// addLegacyMissTransforms matches the single-frame miss branch in Lazer's
// LegacyJudgementPieceOld, including the version-gated drop and rotation.
func addLegacyMissTransforms(hit *sprite.Animation, startTime, fadeIn, fadeOut float64, position vector.Vector2d) {
	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, startTime, startTime, 1.6, 1.6))
	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.InQuad, startTime, startTime+100, 1.6, 1.0))

	if skin.GetInfo().Version > 1 {
		hit.AddTransformUnordered(animation.NewSingleTransform(animation.MoveY, easing.Linear, startTime, startTime, position.Y-5, position.Y-5))
		hit.AddTransformUnordered(animation.NewSingleTransform(animation.MoveY, easing.InQuad, startTime, fadeOut, position.Y-5, position.Y+75))
	}

	rotation := rand.Float64()*0.3 - 0.15
	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Rotate, easing.Linear, startTime, startTime, 0, 0))
	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Rotate, easing.Linear, startTime, fadeIn, 0, rotation))
	hit.AddTransformUnordered(animation.NewSingleTransform(animation.Rotate, easing.InQuad, fadeIn, fadeOut, rotation, rotation*2))
}

func (results *HitResults) addSliderJudgmentMarker(judgement osu.JudgementResult) {
	if results.diff == nil || !settings.Objects.Sliders.ShowSliderJudgmentMarkers {
		return
	}

	definition, ok := sliderJudgmentMarkerForPart(
		judgement.HitResult,
		judgement.IsSliderNested(),
		judgement.IsSliderHead(),
		judgement.IsSliderTail(),
	)
	if !ok {
		return
	}

	startTime := float64(judgement.Time)
	position := judgement.Position.Copy64()
	frames := skin.GetFrames(definition.textureName, true)

	if len(frames) == 0 {
		return
	}

	// Lazer resolves a legacy skin through its embedded DefaultLegacySkin
	// before it reaches the later Triangles/Argon ruleset fallback. Danser's
	// default skin carries the same slider miss assets, so a missing custom
	// animation reaches the correct X here through the normal skin hierarchy.
	marker := sprite.NewAnimation(
		frames,
		1000.0/60,
		false,
		startTime+1,
		position,
		vector.Centre,
	)
	addLegacySliderJudgmentMarkerTransforms(marker, startTime, len(frames))

	marker.ShowForever(false)
	marker.SortTransformations()
	marker.AdjustTimesToTransformations()
	marker.ResetValuesToTransforms()

	results.top.Add(marker)
}

// classifyJudgment keeps ordinary result popups separate from slider-part
// markers. Nested slider hits and markerless miss results must not become
// duplicate ordinary hit popups.
func classifyJudgment(judgement osu.JudgementResult) judgmentPresentationKind {
	if _, ok := sliderJudgmentMarkerForPart(
		judgement.HitResult,
		judgement.IsSliderNested(),
		judgement.IsSliderHead(),
		judgement.IsSliderTail(),
	); ok {
		return judgmentPresentationSliderMarker
	}

	baseResult := judgement.HitResult &^ osu.Additions
	switch baseResult {
	case osu.Hit50, osu.Hit100, osu.Hit300, osu.Miss:
		return judgmentPresentationHit
	case osu.SliderMiss:
		if !judgement.IsSliderNested() || judgement.IsSliderHead() {
			return judgmentPresentationHit
		}
	}

	return judgmentPresentationNone
}

func addLegacySliderJudgmentMarkerTransforms(marker sprite.ISprite, startTime float64, frameCount int) {
	marker.AddTransformUnordered(animation.NewSingleTransform(
		animation.Fade,
		easing.Linear,
		startTime,
		startTime+sliderLegacyJudgmentMarkerFadeIn,
		0,
		1,
	))

	if frameCount <= 1 {
		marker.AddTransformUnordered(animation.NewSingleTransform(
			animation.Scale,
			easing.InQuad,
			startTime,
			startTime+sliderLegacyJudgmentMarkerScaleTime,
			1.2,
			1,
		))
		marker.AddTransformUnordered(animation.NewSingleTransform(
			animation.Fade,
			easing.Linear,
			startTime+sliderLegacyJudgmentMarkerFadeOutDelay,
			startTime+sliderLegacyJudgmentMarkerFadeOutDelay+sliderLegacyJudgmentMarkerFadeOut,
			1,
			0,
		))
	} else {
		// LegacyJudgementPieceOld intentionally skips the miss-specific scale
		// and shortened fade for multi-frame animations unless transforms are
		// forced. Preserve that skin-specific behavior instead of flattening
		// every marker into the single-frame path.
		marker.AddTransformUnordered(animation.NewSingleTransform(
			animation.Fade,
			easing.Linear,
			startTime+500,
			startTime+500+sliderLegacyJudgmentMarkerFadeOut,
			1,
			0,
		))
	}
}

func sliderJudgmentMarkerFor(result osu.HitResult, nested, head bool) (sliderJudgmentMarker, bool) {
	return sliderJudgmentMarkerForPart(result, nested, head, false)
}

func sliderJudgmentMarkerForPart(result osu.HitResult, nested, head, tail bool) (sliderJudgmentMarker, bool) {
	if !nested && !head {
		return sliderJudgmentMarker{}, false
	}

	switch result &^ osu.Additions {
	case osu.LargeTickMiss:
		// Classic Lazer can use LargeTickMiss for a missed slider head when
		// classicNoSliderHeadAccuracy is enabled. LegacySkin uses the same
		// slidertickmiss component for that result, so heads are included for
		// this result only.
		return sliderJudgmentMarker{
			textureName: "slidertickmiss",
		}, true
	case osu.IgnoreMiss:
		if !nested {
			return sliderJudgmentMarker{}, false
		}

		return sliderJudgmentMarker{
			textureName: "sliderendmiss",
		}, true
	case osu.SliderMiss:
		if !nested {
			return sliderJudgmentMarker{}, false
		}

		if tail {
			return sliderJudgmentMarker{
				textureName: "sliderendmiss",
			}, true
		}

		return sliderJudgmentMarker{
			textureName: "slidertickmiss",
		}, true
	default:
		return sliderJudgmentMarker{}, false
	}
}

func (results *HitResults) Update(time float64) {
	results.bottom.Update(time)
	results.top.Update(time)
	results.lastTime = time
}

func (results *HitResults) DrawBottom(batch *batch.QuadBatch, c []color2.Color, alpha float64) {
	results.alpha = alpha
	if len(c) == 0 {
		return
	}

	results.DrawBottomWithColor(batch, c[0], alpha)
}

// DrawBottomWithColor draws the lower layer for one participant. Knockout
// uses one presenter per replay, so each participant must retain its own hit
// lighting color instead of sharing the first overlay color.
func (results *HitResults) DrawBottomWithColor(batch *batch.QuadBatch, c color2.Color, alpha float64) {
	if results.diff == nil {
		return
	}

	results.color = c
	results.alpha = alpha

	batch.ResetTransform()
	batch.SetColor(1, 1, 1, alpha)

	scale := results.diff.CircleRadius / 64
	batch.SetScale(scale, scale)

	results.bottom.Draw(results.lastTime, batch)

	batch.ResetTransform()
}

func (results *HitResults) DrawTop(batch *batch.QuadBatch, _ float64) {
	if results.diff == nil {
		return
	}

	batch.ResetTransform()
	batch.SetColor(1, 1, 1, results.alpha)

	scale := results.diff.CircleRadius / 64
	batch.SetScale(scale, scale)

	results.top.Draw(results.lastTime, batch)

	batch.ResetTransform()
	batch.SetColor(1, 1, 1, 1)
}
