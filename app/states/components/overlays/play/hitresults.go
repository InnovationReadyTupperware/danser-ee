package play

import (
	"math"
	"math/rand"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/graphics"
	"github.com/wieku/danser-go/app/rulesets/osu"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/app/skin"
	"github.com/wieku/danser-go/framework/graphics/batch"
	"github.com/wieku/danser-go/framework/graphics/sprite"
	"github.com/wieku/danser-go/framework/math/animation"
	"github.com/wieku/danser-go/framework/math/animation/easing"
	color2 "github.com/wieku/danser-go/framework/math/color"
	"github.com/wieku/danser-go/framework/math/vector"
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
	sliderDefaultJudgmentMarkerSize      = 16.0
	sliderDefaultJudgmentMarkerScaleTime = 150.0
	sliderDefaultJudgmentMarkerFadeOut   = 600.0

	sliderLegacyJudgmentMarkerFadeIn       = 120.0
	sliderLegacyJudgmentMarkerFadeOutDelay = 250.0
	sliderLegacyJudgmentMarkerFadeOut      = 600.0
	sliderLegacyJudgmentMarkerScaleTime    = 100.0
)

var (
	sliderTickMissMarkerColor = color2.NewIRGB(237, 17, 33)
	sliderEndMissMarkerColor  = color2.NewIRGB(128, 128, 128)
)

type sliderJudgmentMarker struct {
	textureName   string
	fallbackColor color2.Color
}

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
// judgments, while Stable keeps its historical slider visuals.
func (results *HitResults) AddJudgmentResult(judgement osu.JudgementResult, object objects.IHitObject) {
	results.addHitResult(judgement.Time, judgement.HitResult, judgement.Position.Copy64(), object)
	results.addSliderJudgmentMarker(judgement)
}

// AddResult preserves the original hit-result API for callers that do not
// have the complete judgement metadata required by Lazer slider markers.
func (results *HitResults) AddResult(time int64, result osu.HitResult, position vector.Vector2d, object objects.IHitObject) {
	results.addHitResult(time, result, position, object)
}

func (results *HitResults) addHitResult(time int64, result osu.HitResult, position vector.Vector2d, object objects.IHitObject) {
	var tex string
	var particle string

	switch result & osu.BaseHitsM {
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
		} else {
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, float64(time), float64(time+difficulty.ResultFadeIn*0.8), 0.6, 1.1))
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, fadeIn, float64(time+difficulty.ResultFadeIn*1.2), 1.1, 0.9))
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.Linear, float64(time+difficulty.ResultFadeIn*1.2), float64(time+difficulty.ResultFadeIn*1.4), 0.9, 1.0))
		}

		if result == osu.Miss {
			rotation := rand.Float64()*0.3 - 0.15

			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Rotate, easing.Linear, float64(time), fadeIn, 0.0, rotation))
			hit.AddTransformUnordered(animation.NewSingleTransform(animation.Rotate, easing.Linear, fadeIn, fadeOut, rotation, rotation*2))

			hit.AddTransformUnordered(animation.NewSingleTransform(animation.MoveY, easing.Linear, float64(time), fadeOut, position.Y-5, position.Y+40))
		}
	}

	hit.SortTransformations()
	hit.AdjustTimesToTransformations()
	hit.ResetValuesToTransforms()

	results.top.Add(hit)

	if !settings.Gameplay.ShowHitLighting || result&osu.BaseHitsM < osu.Hit50 {
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

func (results *HitResults) addSliderJudgmentMarker(judgement osu.JudgementResult) {
	if results.diff == nil || !results.diff.IsLazer() || !settings.Objects.Sliders.ShowSliderJudgmentMarkers {
		return
	}

	definition, ok := sliderJudgmentMarkerFor(judgement.HitResult, judgement.IsSliderNested(), judgement.IsSliderHead())
	if !ok {
		return
	}

	startTime := float64(judgement.Time)
	position := judgement.Position.Copy64()
	frames := skin.GetFrames(definition.textureName, true)

	var marker sprite.ISprite
	if len(frames) > 0 {
		marker = sprite.NewAnimation(
			frames,
			skin.GetInfo().GetFrameTime(max(1, len(frames))),
			false,
			startTime+1,
			position,
			vector.Centre,
		)
		addLegacySliderJudgmentMarkerTransforms(marker, startTime, len(frames))
	} else {
		// Lazer's ruleset fallback is a 16-unit additive circle. It is not the
		// cross used by legacy miss textures, and it inherits the judged
		// object's circle-size scale from DrawTop. A non-empty custom animation
		// takes precedence, even when its pixels are intentionally transparent.
		if graphics.SliderJudgmentMarker == nil || graphics.SliderJudgmentMarker.Width <= 0 {
			return
		}

		marker = sprite.NewSpriteSingle(graphics.SliderJudgmentMarker, startTime+1, position, vector.Centre)
		marker.SetColor(definition.fallbackColor)
		marker.SetAdditive(true)
		marker.SetScale(sliderDefaultJudgmentMarkerSize / float64(graphics.SliderJudgmentMarker.Width))
		marker.AddTransformUnordered(animation.NewSingleTransform(
			animation.Scale,
			easing.OutQuad,
			startTime,
			startTime+sliderDefaultJudgmentMarkerScaleTime,
			1.4*marker.GetScale().X,
			marker.GetScale().X,
		))
		marker.AddTransformUnordered(animation.NewSingleTransform(
			animation.Fade,
			easing.Linear,
			startTime,
			startTime+sliderDefaultJudgmentMarkerFadeOut,
			1,
			0,
		))
	}

	marker.ShowForever(false)
	marker.SortTransformations()
	marker.AdjustTimesToTransformations()
	marker.ResetValuesToTransforms()

	results.top.Add(marker)
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
			textureName:   "slidertickmiss",
			fallbackColor: sliderTickMissMarkerColor,
		}, true
	case osu.IgnoreMiss:
		if !nested {
			return sliderJudgmentMarker{}, false
		}

		return sliderJudgmentMarker{
			textureName:   "sliderendmiss",
			fallbackColor: sliderEndMissMarkerColor,
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
	results.color = c[0]
	results.alpha = alpha

	batch.ResetTransform()
	batch.SetColor(1, 1, 1, alpha)

	scale := results.diff.CircleRadius / 64
	batch.SetScale(scale, scale)

	results.bottom.Draw(results.lastTime, batch)

	batch.ResetTransform()
}

func (results *HitResults) DrawTop(batch *batch.QuadBatch, _ float64) {
	batch.ResetTransform()
	batch.SetColor(1, 1, 1, results.alpha)

	scale := results.diff.CircleRadius / 64
	batch.SetScale(scale, scale)

	results.top.Draw(results.lastTime, batch)

	batch.ResetTransform()
	batch.SetColor(1, 1, 1, 1)
}
