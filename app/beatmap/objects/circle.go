package objects

import (
	"math"
	"strconv"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/skin"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const (
	defaultCircleName    = "hit"
	defaultReverseBounce = 300
)

type Circle struct {
	*HitObject

	sample  int
	Timings *Timings

	hitCircleTexture *texture.TextureRegion
	fullTexture      *texture.TextureRegion
	hitCircle        *sprite.Sprite
	hitCircleOverlay *sprite.Sprite
	approachCircle   *sprite.Sprite
	reverseArrow     *sprite.Sprite
	comboText        *sprite.TextSprite

	sprites         []sprite.ISprite
	diff            *difficulty.Difficulty
	lastTime        float64
	silent          bool
	firstEndCircle  bool
	textureName     string
	appearTime      float64
	bounceStartTime float64
	ArrowRotation   float64

	SliderPoint      bool
	SliderPointStart bool
	SliderPointEnd   bool

	sliderPointLatched bool

	// DoubleClick is used in cursordances when 2 nearby circles are merged to one
	DoubleClick bool
}

func NewCircle(data []string) *Circle {
	circle := &Circle{
		HitObject: commonParse(data, 5),
	}

	f, _ := strconv.ParseInt(data[4], 10, 64)
	circle.sample = int(f)

	circle.textureName = defaultCircleName

	return circle
}

func DummyCircle(pos vector.Vector2f, time float64) *Circle {
	return DummyCircleInherit(pos, time, false, false, false)
}

func DummyCircleInherit(pos vector.Vector2f, time float64, inherit bool, inheritStart bool, inheritEnd bool) *Circle {
	circle := &Circle{HitObject: &HitObject{}}
	circle.StackIndexMap = make(map[int64]int64)
	circle.StartPosRaw = pos
	circle.EndPosRaw = pos
	circle.StartTime = time
	circle.EndTime = time
	circle.SliderPoint = inherit
	circle.SliderPointStart = inheritStart
	circle.SliderPointEnd = inheritEnd
	circle.silent = true
	circle.textureName = "sliderstart"

	return circle
}

func NewSliderEndCircle(pos vector.Vector2f, appearTime, bounceStartTime, time float64, first, last bool) *Circle {
	circle := &Circle{HitObject: &HitObject{}}
	circle.StartPosRaw = pos
	circle.EndPosRaw = pos
	circle.StartTime = time
	circle.EndTime = time
	circle.SliderPoint = true
	circle.SliderPointEnd = last
	circle.firstEndCircle = first
	circle.silent = true
	circle.textureName = "sliderend"
	circle.appearTime = appearTime
	circle.bounceStartTime = bounceStartTime

	return circle
}

func (circle *Circle) Update(time float64) bool {
	if !circle.silent && ((!settings.PLAY && (!settings.KNOCKOUT || settings.SOLOKNOCKOUT)) || settings.PLAYERS > 1) && (circle.lastTime < circle.StartTime && time >= circle.StartTime) {
		circle.Arm(true, circle.StartTime)
		circle.PlaySound(circle.StartTime)
	}

	for _, s := range circle.sprites {
		s.Update(time)
	}

	circle.lastTime = time

	return true
}

// PlaySound submits the circle's hit sound at eventTime. The update that
// notices a hit may be late, especially when a replay frame advances over
// several milliseconds, so using the nominal or judged event timestamp keeps
// the sample aligned with the music.
func (circle *Circle) PlaySound(eventTime float64) {
	if circle.audioSubmissionDisabled {
		return
	}

	point := circle.Timings.GetPointAt(circle.StartTime)

	index := circle.BasicHitSound.CustomIndex
	sampleSet := circle.BasicHitSound.SampleSet

	if index == 0 {
		index = point.SampleIndex
	}

	if sampleSet == 0 {
		sampleSet = point.SampleSet
	}

	audio.PlaySampleAt(eventTime, sampleSet, circle.BasicHitSound.AdditionSet, circle.sample, index,
		point.SampleVolume, circle.BasicHitSound.CustomVolume, circle.HitObjectID,
		circle.GetStackedStartPositionMod(circle.diff).X64())
}

func (circle *Circle) SetTiming(timings *Timings, _ int, _ bool) {
	circle.Timings = timings
}

func (circle *Circle) SetDifficulty(diff *difficulty.Difficulty) {
	circle.diff = diff
	circle.sliderPointLatched = false

	startTime := circle.StartTime - diff.Preempt

	if circle.SliderPoint {
		startTime = circle.appearTime
	}
	fadeInStartTime := startTime
	fadeInDuration := float64(diff.TimeFadeIn)
	if circle.SliderPoint && !circle.SliderPointStart {
		if !circle.firstEndCircle {
			fadeInDuration = 0
		} else if settings.Objects.Sliders.Snaking.In {
			fadeInStartTime += diff.Preempt / 3
		}
	}

	endTime := circle.StartTime

	name := circle.textureName + "circle"
	var overlayTexture *texture.TextureRegion

	if circle.SliderPoint && !circle.SliderPointStart {
		// Lazer only creates the legacy slider-tail component when a real
		// legacy hitcircle provider exists. Do not let a custom modern skin
		// inherit the local default tail just because DrawEndCircles is on.
		circle.hitCircleTexture, overlayTexture, _ = skin.GetLegacySliderEndTextures()
	} else {
		base := skin.GetTexture(defaultCircleName + "circle")
		named := skin.GetTexture(circle.textureName + "circle")

		if named == nil || skin.GetMostSpecific(named, base) == base {
			name = defaultCircleName + "circle"
		}

		circle.hitCircleTexture = skin.GetTexture(name)
		overlayTexture = skin.GetTexture(name + "overlay")
	}

	circle.fullTexture = skin.GetTexture("hitcircle-full")

	circle.hitCircle = sprite.NewSpriteSingle(circle.hitCircleTexture, 0, vector.NewVec2d(0, 0), vector.Centre)
	circle.hitCircleOverlay = sprite.NewSpriteSingle(overlayTexture, 0, vector.NewVec2d(0, 0), vector.Centre)

	circle.comboText = sprite.NewTextSpriteSize(strconv.Itoa(int(circle.ComboNumber)), skin.GetFont("default"), skin.GetFont("default").GetSize()*0.8, 0, vector.NewVec2d(0, 0), vector.Centre)

	circle.sprites = append(circle.sprites, circle.hitCircle, circle.hitCircleOverlay, circle.comboText)

	circle.hitCircle.SetAlpha(0)
	circle.hitCircleOverlay.SetAlpha(0)

	circle.comboText.SetAlpha(0)

	circles := []sprite.ISprite{circle.hitCircle, circle.hitCircleOverlay, circle.comboText}

	// Stable always fades hit circles before the miss deadline. Lazer keeps
	// them visible until judgement unless Classic explicitly restores the
	// legacy presentation setting carried in the replay.
	fadeHitCircleEarly := !diff.IsLazer()
	if diff.IsLazer() && diff.CheckModActive(difficulty.Classic) {
		if classicSettings, ok := difficulty.GetModConfig[difficulty.ClassicSettings](diff); ok {
			fadeHitCircleEarly = classicSettings.FadeHitCircleEarly
		}
	}

	for _, t := range circles {
		if diff.CheckModActive(difficulty.Hidden) {
			if !circle.SliderPoint || circle.SliderPointStart || circle.firstEndCircle {
				t.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, fadeInStartTime, fadeInStartTime+diff.Preempt*0.4, 0.0, 1.0))
				t.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, fadeInStartTime+diff.Preempt*0.4, fadeInStartTime+diff.Preempt*0.7, 1.0, 0.0))
			}
		} else if !diff.CheckModActive(difficulty.Traceable) || circle.HitObjectID == 0 {
			t.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, fadeInStartTime, fadeInStartTime+fadeInDuration, 0.0, 1.0))
			if fadeHitCircleEarly && (!circle.SliderPoint || circle.SliderPointStart) {
				t.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, endTime+float64(diff.Hit100), endTime+float64(diff.Hit50), 1.0, 0.0))
			} else if circle.SliderPoint {
				t.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, endTime, endTime, 1.0, 0.0))
			}
		}
	}

	if circle.SliderPointEnd {
		return
	}

	if circle.SliderPoint && !circle.SliderPointStart {
		circle.reverseArrow = sprite.NewSpriteSingle(skin.GetTexture("reversearrow"), 0, vector.NewVec2d(0, 0), vector.Centre)
		circle.reverseArrow.SetAlpha(0)

		circle.reverseArrow.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, fadeInStartTime, min(endTime, fadeInStartTime+150), 0.0, 1.0))
		circle.reverseArrow.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, endTime, endTime, 1.0, 0.0))

		circle.sprites = append(circle.sprites, circle.reverseArrow)

		rCount := int((endTime - circle.bounceStartTime) / defaultReverseBounce)

		if rCount > 0 {
			setReverse(circle.reverseArrow, circle.bounceStartTime, defaultReverseBounce, rCount)
		}

		rStart := circle.bounceStartTime + float64(rCount)*defaultReverseBounce
		rTime := endTime - rStart

		if rCount == 0 || rTime > 5 {
			setReverse(circle.reverseArrow, rStart, rTime, 1)
		}
	} else {
		circle.approachCircle = sprite.NewSpriteSingle(skin.GetTexture("approachcircle"), 0, vector.NewVec2d(0, 0), vector.Centre)
		circle.approachCircle.SetAlpha(0)

		circle.sprites = append(circle.sprites, circle.approachCircle)

		if !diff.CheckModActive(difficulty.Hidden) || circle.HitObjectID == 0 {
			circle.approachCircle.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, startTime, min(endTime, endTime-diff.Preempt+diff.TimeFadeIn*2), 0.0, 0.9))
			circle.approachCircle.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, endTime, endTime, 0.0, 0.0))

			circle.approachCircle.AddTransform(animation.NewSingleTransform(animation.Scale, easing.Linear, startTime, endTime, 4.0, 1.0))
		}
	}
}

func setReverse(arrow *sprite.Sprite, start float64, length float64, loops int) {
	scale := animation.NewSingleTransform(animation.Scale, easing.Linear, start, start+length, 1.3, 1.0)
	scale.SetLoop(loops, length)

	arrow.AddTransform(scale)

	if skin.GetInfo().Version < 2 {
		rotate := animation.NewSingleTransform(animation.Rotate, easing.Linear, start, start+length, 6*math.Pi/180, -6*math.Pi/180)
		rotate.SetLoop(loops, length)

		arrow.AddTransform(rotate)
	}
}

func (circle *Circle) Arm(clicked bool, time float64) {
	circle.clearHitTransforms(time)

	if circle.shouldAnimateHit(clicked) {
		circle.addLegacyHitAnimation(time, time+difficulty.HitFadeOut)
		return
	}

	duration := 60.0
	if !clicked {
		duration = 100
	}
	circle.addHitFadeOut(time, time+duration, easing.OutQuad)
}

// ArmSliderPoint applies the Lazer legacy endpoint lifecycle. Repeats use a
// span-length fade capped at 300 ms, while tails use the shorter 100 ms miss
// fade (or the 60 ms no-hit-animation path). A successful repeat latches its
// position before the slider body retracts past it.
func (circle *Circle) ArmSliderPoint(clicked bool, time, spanDuration float64) {
	circle.clearHitTransforms(time)

	if clicked && circle.shouldAnimateHit(true) {
		circle.addLegacyHitAnimation(time, time+difficulty.HitFadeOut)
		if circle.reverseArrow != nil {
			circle.addReverseArrowAnimation(time, sliderPointFadeDuration(false, true, spanDuration))
		}
	} else {
		duration := sliderPointFadeDuration(circle.SliderPointEnd, clicked, spanDuration)
		circle.addHitFadeOut(time, time+duration, easing.Linear)
	}

	if clicked && !circle.SliderPointEnd {
		circle.sliderPointLatched = true
	}
}

func (circle *Circle) clearHitTransforms(time float64) {
	if circle.hitCircle != nil {
		circle.hitCircle.ClearTransformations()
	}
	if circle.hitCircleOverlay != nil {
		circle.hitCircleOverlay.ClearTransformations()
	}
	if circle.reverseArrow != nil {
		circle.reverseArrow.ClearTransformations()
	}
	if circle.comboText != nil {
		circle.comboText.ClearTransformations()
	}

	if circle.approachCircle != nil {
		circle.approachCircle.ClearTransformations()
		circle.approachCircle.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, time, time, 0.0, 0.0))
	}
}

func (circle *Circle) addLegacyHitAnimation(startTime, endTime float64) {
	endScale := 1.4
	if skin.GetInfo().Version < 2 {
		endScale = 1.8
	}

	for _, hit := range []*sprite.Sprite{circle.hitCircle, circle.hitCircleOverlay} {
		if hit == nil {
			continue
		}

		hit.AddTransform(animation.NewSingleTransform(animation.Scale, easing.OutQuad, startTime, endTime, 1.0, endScale))
		hit.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, startTime, endTime, 1.0, 0.0))
	}

	if skin.GetInfo().Version < 2 && circle.comboText != nil {
		circle.comboText.AddTransform(animation.NewSingleTransform(animation.Scale, easing.OutQuad, startTime, endTime, 1.0, endScale))
		circle.comboText.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, startTime, endTime, 1.0, 0.0))
	} else if circle.comboText != nil {
		circle.comboText.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, startTime, startTime+60, 1.0, 0.0))
	}
}

func (circle *Circle) addReverseArrowAnimation(startTime, duration float64) {
	if circle.reverseArrow == nil {
		return
	}

	endTime := startTime + duration
	circle.reverseArrow.AddTransform(animation.NewSingleTransform(animation.Scale, easing.OutQuad, startTime, endTime, 1.0, 1.4))
	circle.reverseArrow.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, startTime, endTime, 1.0, 0.0))
}

func (circle *Circle) addHitFadeOut(startTime, endTime float64, ease func(float64) float64) {
	for _, hit := range []*sprite.Sprite{circle.hitCircle, circle.hitCircleOverlay, circle.reverseArrow} {
		if hit == nil {
			continue
		}

		hit.AddTransform(animation.NewSingleTransform(animation.Fade, ease, startTime, endTime, hit.GetAlpha(), 0.0))
	}

	if circle.comboText != nil {
		circle.comboText.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, startTime, endTime, circle.comboText.GetAlpha(), 0.0))
	}
}

// setSliderPosition updates an unjudged endpoint's raw path position. Once a
// repeat has been judged successfully its position is deliberately frozen,
// matching Lazer's repeat drawable instead of following the retracting body.
func (circle *Circle) setSliderPosition(position vector.Vector2f, rotation float64) {
	if circle.sliderPointLatched {
		return
	}

	circle.StartPosRaw = position
	circle.ArrowRotation = rotation
}

// sliderPointFadeDuration returns the parent lifecycle used when an endpoint
// has no legacy hit-circle animation to play.
func sliderPointFadeDuration(isEnd, clicked bool, spanDuration float64) float64 {
	if !isEnd {
		return min(300.0, max(0.0, spanDuration))
	}

	if clicked {
		return 60
	}

	return 100
}

// shouldAnimateHit keeps the Lazer hit-animation switch separate from the
// slider-specific follow-circle and endpoint animation switch. A slider head
// or legacy endpoint must satisfy both settings, while ordinary hit circles
// only depend on the global setting.
func (circle *Circle) shouldAnimateHit(clicked bool) bool {
	if !clicked || !settings.Objects.HitAnimations || circle.diff == nil {
		return false
	}

	if circle.diff.CheckModActive(difficulty.Hidden) || circle.diff.CheckModActive(difficulty.Traceable) {
		return false
	}

	if !circle.SliderPoint {
		return true
	}

	return circle.hitCircleTexture != nil && settings.Objects.Sliders.HitAnimations
}

func (circle *Circle) Shake(time float64) {
	for _, s := range circle.sprites {
		s.ClearTransformationsOfType(animation.MoveX)
		s.AddTransform(animation.NewSingleTransform(animation.MoveX, easing.Linear, time, time+20, 0, 8))
		s.AddTransform(animation.NewSingleTransform(animation.MoveX, easing.Linear, time+20, time+40, 8, -8))
		s.AddTransform(animation.NewSingleTransform(animation.MoveX, easing.Linear, time+40, time+60, -8, 8))
		s.AddTransform(animation.NewSingleTransform(animation.MoveX, easing.Linear, time+60, time+80, 8, -8))
		s.AddTransform(animation.NewSingleTransform(animation.MoveX, easing.Linear, time+80, time+100, -8, 8))
		s.AddTransform(animation.NewSingleTransform(animation.MoveX, easing.Linear, time+100, time+120, 8, 0))
	}
}

func (circle *Circle) Draw(time float64, color color2.Color, batch *batch.QuadBatch) bool {
	position := circle.GetStackedPositionAtMod(time, circle.diff)

	batch.SetSubScale(1, 1)
	batch.SetTranslation(position.Copy64())

	alpha := float64(color.A)

	if settings.DIVIDES >= settings.Objects.Colors.MandalaTexturesTrigger {
		alpha *= settings.Objects.Colors.MandalaTexturesAlpha
		circle.hitCircle.Texture = circle.fullTexture
	} else {
		circle.hitCircle.Texture = circle.hitCircleTexture
	}

	batch.SetColor(1, 1, 1, alpha)

	circle.hitCircle.SetColor(skin.GetObjectColor(int(circle.ComboSet), int(circle.ComboSetHax), color))

	drawCircle := circle.hitCircleTexture != nil && (!circle.SliderPoint || circle.SliderPointStart || settings.Objects.Sliders.DrawEndCircles)

	if drawCircle {
		circle.hitCircle.Draw(time, batch)
	}

	if settings.DIVIDES < settings.Objects.Colors.MandalaTexturesTrigger {
		if !skin.GetInfo().HitCircleOverlayAboveNumber && drawCircle {
			circle.hitCircleOverlay.Draw(time, batch)
		}

		if !circle.SliderPoint || circle.SliderPointStart {
			if settings.DIVIDES < 2 && settings.Objects.DrawComboNumbers && drawCircle {
				circle.comboText.Draw(0, batch)
			}
		} else if !circle.SliderPointEnd && settings.Objects.Sliders.DrawReverseArrows {
			prevRotation := batch.GetRotation()
			batch.SetRotation(circle.ArrowRotation)
			//circle.reverseArrow.SetRotation(circle.ArrowRotation)
			circle.reverseArrow.Draw(time, batch)
			batch.SetRotation(prevRotation)
		}

		batch.SetSubScale(1, 1)
		batch.SetTranslation(position.Copy64())
		batch.SetColor(1, 1, 1, alpha)

		if skin.GetInfo().HitCircleOverlayAboveNumber && drawCircle {
			circle.hitCircleOverlay.Draw(time, batch)
		}
	}

	batch.SetSubScale(1, 1)
	batch.SetTranslation(vector.NewVec2d(0, 0))

	if time >= circle.StartTime && circle.hitCircle.GetAlpha() <= 0.001 {
		return true
	}

	return false
}

func (circle *Circle) DrawApproach(time float64, color color2.Color, batch *batch.QuadBatch) {
	if circle.approachCircle == nil || circle.diff.Preempt > 15000 {
		return
	}

	position := circle.GetStackedPositionAtMod(time, circle.diff)

	batch.SetSubScale(1, 1)
	batch.SetTranslation(position.Copy64())
	batch.SetColor(1, 1, 1, float64(color.A))

	circle.approachCircle.SetColor(skin.GetObjectColor(int(circle.ComboSet), int(circle.ComboSetHax), color))

	circle.approachCircle.Draw(time, batch)
}

func (circle *Circle) GetType() Type {
	return CIRCLE
}
