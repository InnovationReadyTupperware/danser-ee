package graphics

import (
	"strconv"

	"github.com/innovationreadytupperware/danser-ee/app/utils"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
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

	CursorTrail, _ = utils.LoadTexture("assets/textures/cursortrail.png")
	Pixel = texture.NewTextureSingle(1, 1, 0)
	Pixel.SetData(0, 0, 1, 1, []byte{0xFF, 0xFF, 0xFF, 0xFF})
}

// LoadLauncherTextures loads only the textures used by the launcher. Gameplay
// assets remain in LoadTextures so the launcher does not allocate or upload
// cursor and hit-result resources that can never be drawn in that process.
func LoadLauncherTextures() {
	Atlas = texture.NewTextureAtlas(1024, 4)
	Atlas.Bind(16)

	Triangle, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/triangle.png")
	TriangleShadowed, _ = utils.LoadTextureToAtlas(Atlas, "assets/textures/triangle-shadow.png")
	TriangleSmall = nil
	Cross = nil
	Hit50 = nil
	Hit100 = nil
	CursorTex = nil
	CursorTop = nil
	CursorTrail = nil

	Pixel = texture.NewTextureSingle(1, 1, 0)
	Pixel.SetData(0, 0, 1, 1, []byte{0xFF, 0xFF, 0xFF, 0xFF})

	Snowflakes = nil
	Snow = nil
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
