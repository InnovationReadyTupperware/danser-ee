package font

import (
	"math"
	"sync"
	"unicode"

	fontpkg "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

var fonts map[string]*Font

func init() {
	fonts = make(map[string]*Font)
}

func GetFont(name string) *Font {
	return fonts[name]
}

func AddAlias(font *Font, name string) {
	fonts[name] = font
}

type glyphData struct {
	region  *texture.TextureRegion
	advance float64
	offsetX float64
	offsetY float64
}

type Font struct {
	atlas       *texture.TextureAtlas
	glyphs      map[rune]*glyphData
	initialSize float64
	sfntFont    *sfnt.Font
	glyphIndex  map[rune]sfnt.GlyphIndex
	kernMu      sync.RWMutex
	kernCache   map[[2]rune]float64
	biggest     float64
	Overlap     float64
	ascent      float64
	flip        bool
	pixel       *texture.TextureRegion

	drawBg   bool
	bgBorder float64
	bgColor  color2.Color
}

// kerning caches only the pairs text actually uses. The parsed font and glyph
// indices are immutable after loading; each miss gets its own sfnt buffer.
func (font *Font) kerning(previous, current rune) float64 {
	if font.sfntFont == nil {
		return 0 // Texture fonts have no kerning data
	}

	pair := [2]rune{previous, current}
	font.kernMu.RLock()
	kern, cached := font.kernCache[pair]
	font.kernMu.RUnlock()
	if cached {
		return kern
	}

	font.kernMu.Lock()
	defer font.kernMu.Unlock()
	if kern, cached = font.kernCache[pair]; cached {
		return kern
	}

	previousIndex, previousOK := font.glyphIndex[previous]
	currentIndex, currentOK := font.glyphIndex[current]
	if !previousOK || !currentOK {
		font.kernCache[pair] = 0
		return 0
	}

	var buffer sfnt.Buffer
	// Match opentype.Face.Kern: use UnitsPerEm, not the rasterized font size
	if amount, err := font.sfntFont.Kern(&buffer, previousIndex, currentIndex, fixed.Int26_6(font.sfntFont.UnitsPerEm()), fontpkg.HintingFull); err == nil {
		kern = float64(amount) / 64
	}
	font.kernCache[pair] = kern // Zero-valued pairs must also be cached
	return kern
}

func (font *Font) drawInternal(renderer *batch.QuadBatch, x, y, width, size, rotation float64, text string, monospaced bool, color color2.Color) {
	rotation += renderer.GetRotation()

	if font.drawBg && font.pixel != nil {
		pos2 := vector.NewVec2d(-font.bgBorder, -font.bgBorder).Rotate(rotation).AddS(x, y)
		vSize := vector.NewVec2d(width+2*font.bgBorder, size+2*font.bgBorder)

		renderer.DrawStObject(pos2, vector.TopLeft, vSize, false, false, rotation, font.bgColor, false, *font.pixel)
	}

	scale := size / font.initialSize

	scBase := scale * renderer.GetScale().Y * renderer.GetSubScale().Y

	scl := vector.NewVec2d(scale, scale)

	advance := 0.0

	cos := math.Cos(rotation)
	sin := math.Sin(rotation)

	var previous rune
	var hasPrevious bool
	for _, c := range text {
		char := font.glyphs[c]
		if char == nil {
			previous, hasPrevious = c, true
			continue
		}

		if hasPrevious && !(monospaced && (unicode.IsDigit(c) || unicode.IsDigit(previous))) {
			advance += font.kerning(previous, c)
		}

		if hasPrevious {
			advance -= font.Overlap
		}

		bearX := char.offsetX

		if unicode.IsDigit(c) && monospaced {
			bearX = (font.biggest - float64(char.region.Width)) / 2
		}

		pX := (advance + bearX) * scBase
		pY := char.offsetY * scBase

		pos := vector.NewVec2d(pX*cos-pY*sin+x, pX*sin+pY*cos+y)

		renderer.DrawStObject(pos, vector.TopLeft, scl, false, font.flip, rotation-renderer.GetRotation(), color, false, *char.region)

		if monospaced && (unicode.IsDigit(c) || unicode.IsSpace(c)) {
			advance += font.biggest
		} else {
			advance += char.advance
		}
		previous, hasPrevious = c, true
	}
}

func (font *Font) Draw(renderer *batch.QuadBatch, x, y, size float64, text string) {
	font.DrawOrigin(renderer, x, y, vector.BottomLeft, size, false, text)
}

func (font *Font) DrawMonospaced(renderer *batch.QuadBatch, x, y, size float64, text string) {
	font.DrawOrigin(renderer, x, y, vector.BottomLeft, size, true, text)
}

func (font *Font) getWidthInternal(size float64, text string, monospaced bool) float64 {
	advance := 0.0

	scale := size / font.initialSize

	var previous rune
	var hasPrevious bool
	for _, c := range text {
		char := font.glyphs[c]
		if char == nil {
			previous, hasPrevious = c, true
			continue
		}

		if hasPrevious && !(monospaced && (unicode.IsDigit(c) || unicode.IsDigit(previous))) {
			advance += font.kerning(previous, c)
		}

		if hasPrevious {
			advance -= font.Overlap
		}

		if monospaced && (unicode.IsDigit(c) || unicode.IsSpace(c)) {
			advance += font.biggest
		} else {
			advance += char.advance
		}
		previous, hasPrevious = c, true
	}

	return advance * scale
}

func (font *Font) GetWidth(size float64, text string) float64 {
	return font.getWidthInternal(size, text, false)
}

func (font *Font) GetWidthMonospaced(size float64, text string) float64 {
	return font.getWidthInternal(size, text, true)
}

func (font *Font) GetSize() float64 {
	return font.initialSize
}

func (font *Font) GetAscent() float64 {
	return font.ascent
}

func (font *Font) DrawBg(v bool) {
	font.drawBg = v
}

func (font *Font) SetBgBorderSize(size float64) {
	font.bgBorder = size
}

func (font *Font) SetBgColor(color color2.Color) {
	font.bgColor = color
}

func (font *Font) DrawOrigin(renderer *batch.QuadBatch, x, y float64, origin vector.Vector2d, size float64, monospaced bool, text string) {
	font.DrawOriginRotation(renderer, x, y, origin, size, 0, monospaced, text)
}

func (font *Font) DrawOriginRotation(renderer *batch.QuadBatch, x, y float64, origin vector.Vector2d, size, rotation float64, monospaced bool, text string) {
	font.DrawOriginRotationColor(renderer, x, y, origin, size, rotation, monospaced, color2.NewL(1), text)
}

func (font *Font) DrawOriginRotationColor(renderer *batch.QuadBatch, x, y float64, origin vector.Vector2d, size, rotation float64, monospaced bool, color color2.Color, text string) {
	width := font.getWidthInternal(size, text, monospaced)
	align := origin.AddS(1, 1).Mult(vector.NewVec2d(-width/2, -(size/font.initialSize*font.ascent)/2)).Mult(renderer.GetScale()).Mult(renderer.GetSubScale()).Rotate(rotation + renderer.GetRotation())

	font.drawInternal(renderer, x+align.X, y+align.Y, width, size, rotation, text, monospaced, color)
}

func (font *Font) DrawOriginV(renderer *batch.QuadBatch, position vector.Vector2d, origin vector.Vector2d, size float64, monospaced bool, text string) {
	font.DrawOrigin(renderer, position.X, position.Y, origin, size, monospaced, text)
}

func (font *Font) DrawOriginRotationV(renderer *batch.QuadBatch, position vector.Vector2d, origin vector.Vector2d, size, rotation float64, monospaced bool, text string) {
	font.DrawOriginRotation(renderer, position.X, position.Y, origin, size, rotation, monospaced, text)
}

func (font *Font) DrawOriginRotationColorV(renderer *batch.QuadBatch, position vector.Vector2d, origin vector.Vector2d, size, rotation float64, monospaced bool, color color2.Color, text string) {
	font.DrawOriginRotationColor(renderer, position.X, position.Y, origin, size, rotation, monospaced, color, text)
}
