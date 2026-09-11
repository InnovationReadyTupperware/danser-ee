package play

import (
	"fmt"
	"math"
	"strconv"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/font"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const (
	legacyHitErrorHeight      = legacyHitErrorBarHeight * 4 * legacyHitErrorScale
	legacyHitErrorColorHeight = legacyHitErrorBarHeight * legacyHitErrorScale
	legacyHitErrorCenterWidth = 1.5 * legacyHitErrorScale
	legacyHitErrorArrowWidth  = 17 * 0.6
	legacyHitErrorArrowHeight = 8 * 0.6
	legacyHitErrorNumericGap  = 2.0

	maxHitErrorJudgmentLines = 50
)

// HitErrorSample is one accepted timing event supplied to HitErrorMeter.
// Positional misses are represented by Result == osu.PositionalMiss and are
// rendered as diagnostics without entering the meter's timing statistics.
type HitErrorSample struct {
	// Time is the gameplay-clock timestamp at which the judgment was shown.
	Time float64
	// Offset is the signed timing error in milliseconds, after feed-level
	// clamping to the object's maximum judgment window.
	Offset float64
	// GameplayRate is the mod-derived gameplay rate recorded for the judgment.
	// It excludes seek and playback adjustments, matching osu!lazer's true
	// gameplay-rate contract. osu!lazer normalizes each timing offset by this
	// value before calculating unstable rate.
	GameplayRate float64
	// Result identifies the osu!lazer-compatible timing result used to color
	// the judgment line. Stable slider heads are translated to Classic's
	// LargeTickHit before reaching the meter.
	Result osu.HitResult
}

// hitErrorJudgmentLine is a reusable legacy-bar line. Keeping the fixed
// number of slots locally mirrors osu!lazer's DrawablePool<JudgementLine>(50)
// without making the generic sprite manager responsible for this HUD limit.
type hitErrorJudgmentLine struct {
	drawable         *sprite.Sprite
	displayOffset    float64
	thickness        float64
	heightMultiplier float64
	endTime          float64
	active           bool
}

type hitErrorJudgmentLinePool struct {
	lines [maxHitErrorJudgmentLines]hitErrorJudgmentLine
	next  int
}

func newHitErrorJudgmentLinePool(pixel *texture.TextureRegion) *hitErrorJudgmentLinePool {
	pool := &hitErrorJudgmentLinePool{}
	for i := range pool.lines {
		pool.lines[i].drawable = sprite.NewSpriteSingle(pixel, 3.0, vector.NewVec2d(0, 0), vector.Centre)
		pool.lines[i].drawable.ShowForever(false)
	}

	return pool
}

func (pool *hitErrorJudgmentLinePool) add(
	time, duration, displayOffset float64,
	position vector.Vector2d,
	scale, thickness, heightMultiplier, alpha float64,
	color color2.Color,
	additive bool,
) {
	line := &pool.lines[pool.next]
	pool.next = (pool.next + 1) % len(pool.lines)

	line.active = false
	line.displayOffset = displayOffset
	line.thickness = thickness
	line.heightMultiplier = heightMultiplier
	line.endTime = time
	line.drawable.ClearTransformations()
	line.drawable.SetStartTime(time)
	line.drawable.SetEndTime(time)
	line.drawable.SetAlpha(0)

	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return
	}

	line.endTime = time + duration
	line.active = true
	line.drawable.SetPosition(position)
	line.drawable.SetScaleV(vector.NewVec2d(
		max(0, thickness*scale),
		max(0, legacyHitErrorHeight*heightMultiplier*scale),
	))
	line.drawable.SetColor(color)
	line.drawable.SetAlpha(float32(alpha))
	line.drawable.SetAdditive(additive)
	line.drawable.SetStartTime(time)
	line.drawable.SetEndTime(line.endTime)
	line.drawable.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, time, line.endTime, alpha, 0))
	line.drawable.AdjustTimesToTransformations()
}

func (pool *hitErrorJudgmentLinePool) relayout(centerX, barY, scale float64) {
	for i := range pool.lines {
		line := &pool.lines[i]
		if !line.active {
			continue
		}

		line.drawable.SetPosition(vector.NewVec2d(centerX+line.displayOffset*scale, barY))
		line.drawable.SetScaleV(vector.NewVec2d(
			max(0, line.thickness*scale),
			max(0, legacyHitErrorHeight*line.heightMultiplier*scale),
		))
	}
}

func (pool *hitErrorJudgmentLinePool) update(time float64) {
	for i := range pool.lines {
		line := &pool.lines[i]
		if !line.active {
			continue
		}

		if time >= line.endTime {
			line.active = false
			continue
		}

		line.drawable.Update(time)
	}
}

func (pool *hitErrorJudgmentLinePool) draw(time float64, batch *batch.QuadBatch) {
	for i := range pool.lines {
		line := &pool.lines[i]
		if line.active && time >= line.drawable.GetStartTime() && time < line.endTime {
			line.drawable.Draw(time, batch)
		}
	}
}

func (pool *hitErrorJudgmentLinePool) clear() {
	for i := range pool.lines {
		line := &pool.lines[i]
		line.active = false
		line.endTime = 0
		line.drawable.ClearTransformations()
		line.drawable.SetAlpha(0)
	}
}

// HitErrorMeter renders osu!lazer's horizontal legacy hit-error bar and keeps
// the timing statistics used by the results panel. The numeric unstable-rate
// text is intentionally kept as a separate companion display because Lazer's
// LegacyBarHitErrorMeter does not own a numeric counter.
type HitErrorMeter struct {
	diff *difficulty.Difficulty

	Width  float64
	Height float64

	layout   hitErrorLayout
	barScale float64
	barY     float64

	barBackground *sprite.Sprite
	colorBands    [3]*sprite.Sprite
	centerLine    *sprite.Sprite
	triangle      *sprite.Sprite

	judgmentLines *hitErrorJudgmentLinePool
	lastTime      float64
	errorCurrent  float64

	timingErrors []float64
	statistics   unstableRateStatistics
	avgPos       float64
	avgNeg       float64

	urText        string
	urGlider      *animation.TargetGlider
	urDisplayFade *animation.Glider

	averageN float64
	averageP float64
	countN   int
	countP   int
}

// NewHitErrorMeter creates a legacy-profile hit-error meter in the 768-high
// logical HUD coordinate system used by ScoreOverlay. Width and height are
// logical dimensions, so the component scales consistently across window
// resolutions and monitor DPI settings.
func NewHitErrorMeter(width, height float64, diff *difficulty.Difficulty) *HitErrorMeter {
	meter := &HitErrorMeter{
		diff:          diff,
		Width:         width,
		Height:        height,
		layout:        newHitErrorLayout(diff, settings.Gameplay.HitErrorMeter.ScaleTimingWithSpeed),
		barScale:      hitErrorVisualScale(settings.Gameplay.HitErrorMeter.Scale),
		urText:        "0UR",
		urGlider:      animation.NewTargetGlider(0, 0),
		urDisplayFade: animation.NewGlider(0),
	}

	meter.barY = meter.Height - legacyHitErrorHeight/2*meter.barScale

	pixel := graphics.Pixel.GetRegion()
	newPixel := func(depth float64) *sprite.Sprite {
		return sprite.NewSpriteSingle(&pixel, depth, vector.NewVec2d(meter.Width/2, meter.barY), vector.Centre)
	}

	meter.barBackground = newPixel(0)
	meter.barBackground.SetScaleV(vector.NewVec2d(
		meter.layout.barHalfWidth*2*meter.barScale,
		legacyHitErrorHeight*meter.barScale,
	))
	meter.barBackground.SetColor(color2.NewL(0))
	meter.barBackground.SetAlpha(0.6)

	bandColors := [...]color2.Color{
		legacyHitErrorGreatColor,
		legacyHitErrorOkColor,
		legacyHitErrorMehColor,
	}
	for i, halfWidth := range meter.layout.bandPositions() {
		band := newPixel(1)
		band.SetScaleV(vector.NewVec2d(
			halfWidth*2*meter.barScale,
			legacyHitErrorColorHeight*meter.barScale,
		))
		band.SetColor(bandColors[i])
		band.SetAlpha(1)
		meter.colorBands[i] = band
	}

	meter.centerLine = newPixel(2)
	meter.centerLine.SetScaleV(vector.NewVec2d(
		legacyHitErrorCenterWidth*meter.barScale,
		legacyHitErrorHeight*meter.barScale,
	))
	meter.centerLine.SetColor(color2.NewL(1))
	meter.centerLine.SetAlpha(1)

	meter.triangle = sprite.NewSpriteSingle(
		graphics.TriangleSmall,
		2.0,
		vector.NewVec2d(meter.Width/2, meter.barTopY()),
		vector.BottomCentre,
	)
	meter.triangle.SetScaleV(meter.arrowScale())
	meter.triangle.SetAlpha(1)

	meter.judgmentLines = newHitErrorJudgmentLinePool(&pixel)

	return meter
}

func hitErrorVisualScale(scale float64) float64 {
	if scale < 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0
	}

	return scale
}

func (meter *HitErrorMeter) barTopY() float64 {
	return meter.Height - legacyHitErrorHeight*meter.barScale
}

func (meter *HitErrorMeter) arrowScale() vector.Vector2d {
	if graphics.TriangleSmall == nil || graphics.TriangleSmall.Width <= 0 || graphics.TriangleSmall.Height <= 0 {
		return vector.NewVec2d(meter.barScale, meter.barScale)
	}

	return vector.NewVec2d(
		legacyHitErrorArrowWidth/float64(graphics.TriangleSmall.Width)*meter.barScale,
		legacyHitErrorArrowHeight/float64(graphics.TriangleSmall.Height)*meter.barScale,
	)
}

func (meter *HitErrorMeter) numericURScale() float64 {
	// UnstableRateScale remains a user-controlled relative multiplier, while
	// barScale keeps the numeric companion attached to the same HUD geometry.
	return meter.barScale * hitErrorVisualScale(settings.Gameplay.HitErrorMeter.UnstableRateScale)
}

// Add records one hit-error event and presents its short-lived HUD feedback.
func (meter *HitErrorMeter) Add(sample HitErrorSample) {
	meter.add(sample, true)
}

// Record adds one hit-error event to persistent timing statistics without
// replaying its short-lived HUD feedback. Initial catch-up uses this path.
func (meter *HitErrorMeter) Record(sample HitErrorSample) {
	meter.add(sample, false)
}

func (meter *HitErrorMeter) add(sample HitErrorSample, present bool) {
	if math.IsNaN(sample.Time) || math.IsInf(sample.Time, 0) || math.IsNaN(sample.Offset) || math.IsInf(sample.Offset, 0) {
		return
	}

	positionalMiss := sample.Result == osu.PositionalMiss
	if present {
		meter.presentSample(sample, positionalMiss)
	}

	if positionalMiss {
		return
	}

	if sample.GameplayRate <= 0 || math.IsNaN(sample.GameplayRate) || math.IsInf(sample.GameplayRate, 0) {
		return
	}

	if sample.Offset >= 0 {
		meter.averageP += sample.Offset
		meter.countP++
	} else {
		meter.averageN += sample.Offset
		meter.countN++
	}

	meter.timingErrors = append(meter.timingErrors, sample.Offset)
	meter.statistics.Add(sample.Offset, sample.GameplayRate)

	meter.avgNeg = meter.averageN / max(float64(meter.countN), 1)
	meter.avgPos = meter.averageP / max(float64(meter.countP), 1)

	unstableRate, _ := meter.statistics.value()
	meter.urGlider.SetValue(unstableRate, settings.Gameplay.HitErrorMeter.StaticUnstableRate)
}

func (meter *HitErrorMeter) presentSample(sample HitErrorSample, positionalMiss bool) {
	meter.syncGeometry()

	if positionalMiss && !settings.Gameplay.HitErrorMeter.ShowPositionalMisses {
		return
	}

	displayOffset := meter.layout.displayOffset(sample.Offset)
	baseAlpha := 0.4
	heightMultiplier := 1.0
	additive := true
	if positionalMiss {
		baseAlpha = 0.8
		heightMultiplier = max(0, settings.Gameplay.HitErrorMeter.PositionalMissScale)
		additive = false
	}

	lineDuration := hitErrorFadeDuration()
	lineThickness := max(1, settings.Gameplay.HitErrorMeter.JudgmentLineThickness)
	meter.judgmentLines.add(
		sample.Time,
		lineDuration,
		displayOffset,
		vector.NewVec2d(meter.Width/2+displayOffset*meter.barScale, meter.barY),
		meter.barScale,
		lineThickness,
		heightMultiplier,
		baseAlpha,
		hitErrorColorFor(sample.Result),
		additive,
	)

	if positionalMiss {
		return
	}

	meter.errorCurrent = meter.errorCurrent*0.8 + displayOffset*0.2
	meter.triangle.ClearTransformations()
	meter.triangle.AddTransform(animation.NewSingleTransform(
		animation.MoveX,
		easing.OutQuad,
		sample.Time,
		sample.Time+800,
		meter.triangle.GetPosition().X,
		meter.Width/2+meter.errorCurrent*meter.barScale,
	))

	// The bar stays visible at rest, but retain the existing companion UR
	// readout fade so changing the bar's lifecycle does not unexpectedly alter
	// the numeric display's established behavior.
	meter.urDisplayFade.Reset()
	meter.urDisplayFade.SetValue(1)
	meter.urDisplayFade.AddEventSEase(sample.Time+4000, sample.Time+5000, 1, 0, easing.InQuad)
}

func hitErrorFadeDuration() float64 {
	duration := settings.Gameplay.HitErrorMeter.JudgmentLineFadeOutTime * 1000
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0
	}

	return duration
}

func (meter *HitErrorMeter) syncGeometry() {
	scale := hitErrorVisualScale(settings.Gameplay.HitErrorMeter.Scale)
	layout := newHitErrorLayout(meter.diff, settings.Gameplay.HitErrorMeter.ScaleTimingWithSpeed)
	if scale == meter.barScale && layout == meter.layout {
		return
	}

	meter.barScale = scale
	meter.layout = layout
	meter.barY = meter.Height - legacyHitErrorHeight/2*meter.barScale

	meter.barBackground.SetPosition(vector.NewVec2d(meter.Width/2, meter.barY))
	meter.barBackground.SetScaleV(vector.NewVec2d(
		meter.layout.barHalfWidth*2*meter.barScale,
		legacyHitErrorHeight*meter.barScale,
	))

	for i, halfWidth := range meter.layout.bandPositions() {
		meter.colorBands[i].SetPosition(vector.NewVec2d(meter.Width/2, meter.barY))
		meter.colorBands[i].SetScaleV(vector.NewVec2d(
			halfWidth*2*meter.barScale,
			legacyHitErrorColorHeight*meter.barScale,
		))
	}

	meter.centerLine.SetPosition(vector.NewVec2d(meter.Width/2, meter.barY))
	meter.centerLine.SetScaleV(vector.NewVec2d(
		legacyHitErrorCenterWidth*meter.barScale,
		legacyHitErrorHeight*meter.barScale,
	))

	meter.triangle.ClearTransformations()
	meter.triangle.SetPosition(vector.NewVec2d(meter.Width/2+meter.errorCurrent*meter.barScale, meter.barTopY()))
	meter.triangle.SetScaleV(meter.arrowScale())
	meter.judgmentLines.relayout(meter.Width/2, meter.barY, meter.barScale)
}

// Update advances the arrow, judgment-line fades, and numeric UR animation.
func (meter *HitErrorMeter) Update(time float64) {
	meter.syncGeometry()
	meter.triangle.Update(time)
	meter.judgmentLines.update(time)
	meter.urDisplayFade.Update(time)
	meter.lastTime = time

	meter.urGlider.SetDecimals(settings.Gameplay.HitErrorMeter.UnstableRateDecimals)
	meter.urGlider.Update(time)
	meter.urText = fmt.Sprintf("%."+strconv.Itoa(settings.Gameplay.HitErrorMeter.UnstableRateDecimals)+"fUR", meter.urGlider.GetValue())
}

// Draw renders the legacy bar in logical HUD coordinates. Static bar elements
// are drawn independently from judgment lines so only the latter can fade.
func (meter *HitErrorMeter) Draw(batch *batch.QuadBatch, alpha float64) {
	batch.ResetTransform()
	meter.syncGeometry()

	meterAlpha := settings.Gameplay.HitErrorMeter.Opacity * alpha
	if meterAlpha > 0.001 && settings.Gameplay.HitErrorMeter.Show {
		batch.SetColor(1, 1, 1, meterAlpha)
		batch.SetTranslation(vector.NewVec2d(settings.Gameplay.HitErrorMeter.XOffset, settings.Gameplay.HitErrorMeter.YOffset))

		meter.barBackground.Draw(meter.lastTime, batch)
		if settings.Gameplay.HitErrorMeter.ShowColorBar {
			// Lazer adds the widest Meh box first and overlays Ok and Great on
			// top of it. Drawing in reverse index order preserves those exact
			// boundaries with the existing pixel-sprite renderer.
			for i := len(meter.colorBands) - 1; i >= 0; i-- {
				meter.colorBands[i].Draw(meter.lastTime, batch)
			}
		}

		meter.centerLine.Draw(meter.lastTime, batch)
		meter.judgmentLines.draw(meter.lastTime, batch)
		if settings.Gameplay.HitErrorMeter.ShowMovingAverage {
			// LegacyBarHitErrorMeter adds the arrow after its judgment
			// container, so it remains visually above the fading lines.
			meter.triangle.Draw(meter.lastTime, batch)
		}

		if settings.Gameplay.HitErrorMeter.ShowUnstableRate {
			urAlpha := meterAlpha * meter.urDisplayFade.GetValue()
			if urAlpha > 0.001 {
				batch.SetColor(1, 1, 1, urAlpha)
				// The triangle uses BottomCentre, so its top is one arrow height
				// above barTopY. Anchor the text above that top edge instead of
				// using a fixed bar-relative baseline that can overlap the arrow
				// when the HUD scale is increased.
				pY := meter.barTopY() - (legacyHitErrorArrowHeight+legacyHitErrorNumericGap)*meter.barScale
				scale := meter.numericURScale()

				fnt := font.GetFont("HUDFont")
				fnt.DrawOrigin(batch, meter.Width/2, pY, vector.BottomCentre, 15*scale, true, meter.urText)
			}
		}
	}

	batch.ResetTransform()
}

// GetAvgNeg returns the average signed offset of early accepted hits.
func (meter *HitErrorMeter) GetAvgNeg() float64 {
	return meter.avgNeg
}

// GetAvgNegConverted returns the early-hit average in replay-speed-adjusted
// milliseconds.
func (meter *HitErrorMeter) GetAvgNegConverted() float64 {
	return meter.avgNeg / meter.speed()
}

// GetAvgPos returns the average signed offset of late accepted hits.
func (meter *HitErrorMeter) GetAvgPos() float64 {
	return meter.avgPos
}

// GetAvgPosConverted returns the late-hit average in replay-speed-adjusted
// milliseconds.
func (meter *HitErrorMeter) GetAvgPosConverted() float64 {
	return meter.avgPos / meter.speed()
}

// GetMedian returns the median signed timing offset of accepted hit-error
// samples. Negative values indicate early hits and positive values indicate
// late hits. Positional misses and non-timing slider events are never stored.
func (meter *HitErrorMeter) GetMedian() float64 {
	return median(meter.timingErrors)
}

// GetMedianConverted returns the median timing offset after the existing
// gameplay-speed conversion used by the ranking panel.
func (meter *HitErrorMeter) GetMedianConverted() float64 {
	return meter.GetMedian() / meter.speed()
}

// GetUnstableRate returns osu!lazer-compatible unstable rate, with each
// accepted timing offset normalized by its gameplay rate.
func (meter *HitErrorMeter) GetUnstableRate() float64 {
	unstableRate, _ := meter.statistics.value()
	return unstableRate
}

// HasUnstableRate reports whether at least one timing hit contributes to UR.
func (meter *HitErrorMeter) HasUnstableRate() bool {
	return meter.statistics.count > 0
}

// GetUnstableRateConverted returns the same gameplay-rate-normalized value as
// GetUnstableRate.
//
// Deprecated: unstable rate is normalized per event; use GetUnstableRate.
func (meter *HitErrorMeter) GetUnstableRateConverted() float64 {
	return meter.GetUnstableRate()
}

func (meter *HitErrorMeter) speed() float64 {
	if meter.diff == nil || meter.diff.Speed <= 0 || math.IsNaN(meter.diff.Speed) || math.IsInf(meter.diff.Speed, 0) {
		return 1
	}

	return meter.diff.Speed
}
