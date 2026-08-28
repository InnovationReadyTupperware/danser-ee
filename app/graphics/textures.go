package graphics

import (
	"math"
	"strconv"

	"github.com/wieku/danser-go/app/utils"
	"github.com/wieku/danser-go/framework/graphics/texture"
)

//TODO: Refactor this

var Atlas *texture.TextureAtlas

var CursorTex *texture.TextureRegion
var CursorTop *texture.TextureRegion
var CursorTrail *texture.TextureSingle

var Pixel *texture.TextureSingle
var Triangle *texture.TextureRegion
var TriangleShadowed *texture.TextureRegion

var Snowflakes []*texture.TextureRegion
var Snow []*texture.TextureRegion

var TriangleSmall *texture.TextureRegion
var Cross *texture.TextureRegion
var SliderJudgmentMarker *texture.TextureRegion

var Hit50 *texture.TextureRegion
var Hit100 *texture.TextureRegion

func LoadTextures() {
	Atlas = texture.NewTextureAtlas(2048, 4)
	Atlas.Bind(16)

	CursorTex, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/cursor.png")
	CursorTop, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/cursor-top.png")

	Hit50, _ = utils.LoadTextureToAtlas(Atlas, "assets/default-skin/hit50.png")
	Hit100, _ = utils.LoadTextureToAtlas(Atlas, "assets/default-skin/hit100.png")

	Triangle, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/triangle.png")
	TriangleShadowed, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/triangle-shadow.png")
	TriangleSmall, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/triangle-small.png")

	Cross, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/cross.png")
	// Lazer's default missed slider tick and tail judgments use an unpadded
	// additive circle rather than a legacy cross texture. Generate that simple
	// primitive independently from skin assets: Danser's built-in hitcircle
	// images include stable-compatible transparent padding and are therefore
	// the wrong geometry even when scaled to the same nominal size.
	SliderJudgmentMarker = newCircleTextureRegion(32)

	CursorTrail, _ = utils.LoadTexture("assets/textures/cursortrail.png")
	Pixel = texture.NewTextureSingle(1, 1, 0)
	Pixel.SetData(0, 0, 1, 1, []byte{0xFF, 0xFF, 0xFF, 0xFF})
}

func newCircleTextureRegion(size int) *texture.TextureRegion {
	pixels := make([]byte, size*size*4)
	centre := float64(size) / 2
	radius := centre - 0.5

	for y := range size {
		for x := range size {
			distance := math.Hypot(float64(x)+0.5-centre, float64(y)+0.5-centre)
			// One source pixel of coverage gives the scaled marker a smooth edge
			// without expanding the requested diameter with opaque padding.
			coverage := min(1.0, max(0.0, radius+0.5-distance))
			offset := (y*size + x) * 4
			pixels[offset] = 0xFF
			pixels[offset+1] = 0xFF
			pixels[offset+2] = 0xFF
			pixels[offset+3] = byte(math.Round(coverage * 0xFF))
		}
	}

	circle := texture.NewTextureSingle(size, size, 0)
	circle.SetData(0, 0, size, size, pixels)
	region := circle.GetRegion()
	return &region
}

func LoadWinterTextures() {
	for i := 1; i <= 5; i++ {
		tex1, _ := utils.LoadTextureToAtlas(Atlas, "assets/textures/snowflake"+strconv.Itoa(i)+".png")

		Snowflakes = append(Snowflakes, tex1)
	}

	for i := 1; i <= 6; i++ {
		tex1, _ := utils.LoadTextureToAtlas(Atlas, "assets/textures/snow"+strconv.Itoa(i)+".png")

		Snow = append(Snow, tex1)
	}
}
