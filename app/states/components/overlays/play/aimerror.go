package play

import (
	"fmt"
	"math"
	"strconv"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/skin"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/font"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/shape"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const baseSpaceSize = 64.0

var aimErrorColors = []color2.Color{
	color2.NewRGBA(0.2, 0.8, 1, 1),
	color2.NewRGBA(0.44, 0.98, 0.18, 1),
	color2.NewRGBA(0.85, 0.68, 0.27, 1),
	color2.NewRGBA(0.98, 0.11, 0.011, 1),
}

type AimErrorMeter struct {
	diff             *difficulty.Difficulty
	errorDisplay     *sprite.Manager
	errorCurrent     vector.Vector2d
	errorDot         *sprite.Sprite
	errorDisplayFade *animation.Glider

	lastTime float64

	hitCircle        *texture.TextureRegion
	hitCircleOverlay *texture.TextureRegion

	statistics   vectorStatistics
	unstableRate float64
	urText       string
	urGlider     *animation.TargetGlider

	normalized    bool
	shapeRenderer *shape.Renderer
}

func NewAimErrorMeter(diff *difficulty.Difficulty) *AimErrorMeter {
	meter := new(AimErrorMeter)

	meter.diff = diff
	meter.errorDisplay = sprite.NewManager()
	meter.errorDisplayFade = animation.NewGlider(0)
	meter.urText = "0UR"
	meter.urGlider = animation.NewTargetGlider(0, 0)

	meter.errorDot = sprite.NewSpriteSingle(graphics.Cross, 3.0, vector.NewVec2d(0, 0), vector.Centre)

	dotSize := settings.Gameplay.AimErrorMeter.DotScale / float64(graphics.Cross.Height) / 4
	meter.errorDot.SetScaleV(vector.NewVec2d(dotSize, dotSize))

	meter.errorDot.SetAlpha(1)

	meter.errorDisplay.Add(meter.errorDot)

	src := skin.ALL
	if settings.Gameplay.AimErrorMeter.UseFallbackSkin {
		src = skin.FALLBACK | skin.LOCAL
	}

	meter.hitCircle = skin.GetTextureSource("hitcircle", src)
	meter.hitCircleOverlay = skin.GetTextureSource("hitcircleoverlay", src)

	meter.normalized = settings.Gameplay.AimErrorMeter.AngleNormalized

	if meter.normalized {
		meter.shapeRenderer = shape.NewRenderer()
	}

	return meter
}

func (meter *AimErrorMeter) Add(time float64, hitPosition vector.Vector2f, startPos, endPos *vector.Vector2f) {
	err := hitPosition.Sub(*endPos)

	if meter.normalized {
		if startPos == nil {
			return
		}

		var angle float32
		if startPos.Dst(*endPos) > 0.01 {
			angle = startPos.AngleRV(*endPos)
		}

		err = err.Rotate(-angle - math.Pi/4).Scl(-1)
	}

	scl := baseSpaceSize * settings.Gameplay.AimErrorMeter.Scale

	errorS := err.Scl(float32(1 / meter.diff.CircleRadius))

	errorA := errorS.Len()

	if errorA > 1.2 && settings.Gameplay.AimErrorMeter.CapPositionalMisses {
		errorS = errorS.Nor().Scl(1.2)
	}

	middle := sprite.NewSpriteSingle(graphics.Cross, 2.0, errorS.Copy64().Scl(scl), vector.Centre)
	middle.ShowForever(false)
	middle.SetAdditive(true)

	switch {
	case errorA < 0.33:
		middle.SetColor(aimErrorColors[0])
	case errorA < 0.66:
		middle.SetColor(aimErrorColors[1])
	case errorA <= 1:
		middle.SetColor(aimErrorColors[2])
	case errorA > 1:
		middle.SetColor(aimErrorColors[3])
	}

	dotSize := settings.Gameplay.AimErrorMeter.DotScale / (float64(graphics.Cross.Height) / math.Sqrt(2)) / 8

	middle.SetScaleV(vector.NewVec2d(dotSize, dotSize))
	middle.SetRotation(math.Pi / 4)

	middle.AddTransform(animation.NewSingleTransform(animation.Fade, easing.InQuad, time, time+max(0, settings.Gameplay.AimErrorMeter.PointFadeOutTime*1000), 0.7, 0.0))
	middle.AdjustTimesToTransformations()

	meter.errorDisplay.Add(middle)

	if errorA > 1 {
		return
	}

	meter.errorCurrent = meter.errorCurrent.Scl(0.8).Add(errorS.Copy64().Scl(0.2))

	meter.errorDot.ClearTransformations()
	meter.errorDot.AddTransform(animation.NewVectorTransformV(animation.Move, easing.OutQuad, time, time+800, meter.errorDot.GetPosition(), meter.errorCurrent.Scl(scl)))

	meter.errorDisplayFade.Reset()
	meter.errorDisplayFade.SetValue(1.0)
	meter.errorDisplayFade.AddEventSEase(time+4000, time+5000, 1.0, 0.0, easing.InQuad)

	meter.statistics.Add(err.Copy64())
	meter.unstableRate = meter.statistics.standardDeviation() * 10

	meter.urGlider.SetValue(meter.unstableRate, settings.Gameplay.AimErrorMeter.StaticUnstableRate)
}

func (meter *AimErrorMeter) Update(time float64) {
	meter.errorDisplayFade.Update(time)
	meter.errorDisplay.Update(time)

	meter.lastTime = time

	meter.urGlider.SetDecimals(settings.Gameplay.AimErrorMeter.UnstableRateDecimals)
	meter.urGlider.Update(time)
	meter.urText = fmt.Sprintf("%."+strconv.Itoa(settings.Gameplay.AimErrorMeter.UnstableRateDecimals)+"fUR", meter.urGlider.GetValue())
}

func (meter *AimErrorMeter) Draw(batch *batch.QuadBatch, alpha float64) {
	batch.ResetTransform()

	meterAlpha := settings.Gameplay.AimErrorMeter.Opacity * meter.errorDisplayFade.GetValue() * alpha
	if meterAlpha > 0.001 && settings.Gameplay.AimErrorMeter.Show {
		basePos := vector.NewVec2d(settings.Gameplay.AimErrorMeter.XPosition, settings.Gameplay.AimErrorMeter.YPosition)
		origin := vector.ParseOrigin(settings.Gameplay.AimErrorMeter.Align)

		scl := baseSpaceSize * settings.Gameplay.AimErrorMeter.Scale

		pos := basePos.Sub(origin.Scl(scl))

		if meter.normalized {
			batch.Flush()

			meter.shapeRenderer.Begin()
			meter.shapeRenderer.SetCamera(batch.Projection)
			meter.shapeRenderer.SetColor(1, 1, 1, meterAlpha)

			p32 := pos.Copy32()

			direction := vector.NewVec2fRad(-math.Pi/4, float32(scl*1.4))

			lWidth := float32(scl / 32)
			lLength := float32(scl) / 6

			ePos := p32.Add(direction)

			meter.shapeRenderer.DrawLineV(p32.Sub(direction), ePos, float32(scl/32))
			meter.shapeRenderer.DrawLineV(ePos.SubS(lLength, 0), ePos.SubS(lWidth/2, 0), lWidth)
			meter.shapeRenderer.DrawLineV(ePos.SubS(0, lWidth/2), ePos.AddS(0, lLength), lWidth)

			direction.X *= -1

			meter.shapeRenderer.SetColor(0.8, 0.8, 0.8, meterAlpha*0.7)

			meter.shapeRenderer.DrawLineV(p32.Sub(direction), p32.Add(direction), float32(scl/32))

			meter.shapeRenderer.End()
		}

		batch.SetTranslation(pos)

		batch.SetScale(scl/64, scl/64)

		batch.SetColor(0.2, 0.2, 0.2, meterAlpha*0.8)
		batch.DrawTexture(*meter.hitCircle)

		batch.SetColor(1, 1, 1, meterAlpha*0.8)
		batch.DrawTexture(*meter.hitCircleOverlay)

		batch.SetScale(scl, scl)

		batch.SetColor(1, 1, 1, meterAlpha)

		meter.errorDisplay.Draw(meter.lastTime, batch)

		batch.SetScale(1, 1)

		if settings.Gameplay.AimErrorMeter.ShowUnstableRate {
			scale := settings.Gameplay.AimErrorMeter.UnstableRateScale

			fnt := font.GetFont("HUDFont")
			fnt.DrawOrigin(batch, 0, scl+4, vector.TopCentre, 15*scale, true, meter.urText)
		}
	}

	batch.ResetTransform()
}
