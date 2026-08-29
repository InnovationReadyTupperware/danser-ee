package skin

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/assets"
	"github.com/wieku/danser-go/framework/bass"
	"github.com/wieku/danser-go/framework/files"
	"github.com/wieku/danser-go/framework/goroutines"
	"github.com/wieku/danser-go/framework/graphics/font"
	"github.com/wieku/danser-go/framework/graphics/texture"
	"github.com/wieku/danser-go/framework/math/color"
)

type Source int

type Color int

const (
	UNKNOWN = Source(0)
	LOCAL   = Source(1 << iota)
	FALLBACK
	SKIN
	BEATMAP
	ALL = LOCAL | FALLBACK | SKIN | BEATMAP
)

const (
	InputOverlayText = Color(iota)
	SliderBorder
	SliderTrackOverride
	SliderBall
)

const defaultName = "default"

var fontLock = &sync.Mutex{}
var soundLock = &sync.Mutex{}
var textureLock = &sync.Mutex{}

var atlas *texture.TextureAtlas

var animationCache = make(map[string][]*texture.TextureRegion)

var skinCache = make(map[string]*texture.TextureRegion)
var fallbackCache = make(map[string]*texture.TextureRegion)
var defaultCache = make(map[string]*texture.TextureRegion)

var sourceCache = make(map[*texture.TextureRegion]Source)

var fontCache = make(map[string]*font.Font)

var sampleCache = make(map[string]*bass.Sample)

var skinPathCache *files.FileMap
var fallbackPathCache *files.FileMap

var CurrentSkin = defaultName
var FallbackSkin = defaultName

var info *SkinInfo

func loadDefault() {
	CurrentSkin = defaultName
	FallbackSkin = defaultName

	var err error
	info, err = LoadInfo(filepath.Join("assets", "default-skin", "skin.ini"), true)

	if err != nil {
		log.Println("SkinManager: Default skin is corrupted! Please don't manipulate game's assets!")
		panic(err)
	}
}

func checkInit() {
	if info != nil {
		return
	}

	tryLoadSkin(settings.Skin.CurrentSkin, settings.Skin.FallbackSkin)

	log.Println(fmt.Sprintf("SkinManager: Skin \"%s\" loaded.", CurrentSkin))

	if FallbackSkin != CurrentSkin && FallbackSkin != defaultName {
		log.Println("SkinManager: Loading fallback skin:", FallbackSkin)

		var err error
		fallbackPathCache, err = files.NewFileMap(filepath.Join(settings.General.GetSkinsDir(), FallbackSkin))

		if err != nil {
			log.Println("SkinManager:", FallbackSkin, "does not exist, falling back to default...")
			FallbackSkin = defaultName
		} else {
			log.Println(fmt.Sprintf("SkinManager: Fallback skin \"%s\" loaded.", FallbackSkin))
		}
	}
}

func tryLoadSkin(name, fallbackName string) {
	CurrentSkin = name
	FallbackSkin = fallbackName

	if name == defaultName {
		loadDefault()
		return
	}

	log.Println("SkinManager: Loading skin:", name)

	var err error
	skinPathCache, err = files.NewFileMap(filepath.Join(settings.General.GetSkinsDir(), name))

	if err != nil {
		log.Println(fmt.Sprintf("SkinManager: %s does not exist, falling back to %s...", name, fallbackName))
		tryLoadSkin(fallbackName, defaultName)
	} else {
		path, err := skinPathCache.GetFile("skin.ini")
		if err != nil {
			info = newDefaultInfo()
		} else if info, err = LoadInfo(path, false); err != nil {
			log.Println(fmt.Sprintf("SkinManager: %s is corrupted, falling back to %s...", name, fallbackName))
			tryLoadSkin(fallbackName, defaultName)
		}
	}
}

func GetInfo() *SkinInfo {
	checkInit()
	return info
}

func GetFont(name string) *font.Font {
	checkInit()

	fontLock.Lock()
	defer fontLock.Unlock()

	if fnt, exists := fontCache[name]; exists {
		return fnt
	}

	overlap := 0.0

	prefix := name

	switch name {
	case defaultName:
		prefix = info.HitCirclePrefix
		overlap = info.HitCircleOverlap
	case "score":
		prefix = info.ScorePrefix
		overlap = info.ScoreOverlap
	case "combo":
		prefix = info.ComboPrefix
		overlap = info.ComboOverlap
	}

	if name == "scoreentry" && GetTexture(prefix+"-0") == nil {
		return nil
	}

	chars := make(map[rune]*texture.TextureRegion)

	for i := '0'; i <= '9'; i++ {
		chars[i] = GetTexture(prefix + "-" + string(i))
	}

	chars[','] = GetTexture(prefix + "-comma")
	chars['.'] = GetTexture(prefix + "-dot")
	chars['%'] = GetTexture(prefix + "-percent")
	chars['x'] = GetTexture(prefix + "-x")

	fnt := font.LoadTextureFontMap2(chars, overlap)

	fontCache[name] = fnt

	return fnt
}

func GetTexture(name string) *texture.TextureRegion {
	return GetTextureSource(name, ALL)
}

// GetTextureWithSource resolves a texture through the active skin hierarchy
// and returns the provider which supplied it. Callers that need a related
// legacy asset must use this provider instead of resolving the related name
// through ALL, otherwise a fallback texture can silently mix with a custom
// skin's assets.
func GetTextureWithSource(name string) (*texture.TextureRegion, Source) {
	checkInit()

	if CurrentSkin == defaultName {
		if texture := GetTextureSource(name, LOCAL); texture != nil {
			return texture, LOCAL
		}

		return nil, UNKNOWN
	}

	for _, source := range []Source{SKIN, FALLBACK, LOCAL} {
		if texture := GetTextureSource(name, source); texture != nil {
			return texture, source
		}
	}

	return nil, UNKNOWN
}

// GetLegacySliderEndTextures resolves the optional legacy slider tail
// component. The component is available only when the active skin hierarchy
// provides a real legacy hitcircle. A custom skin which relies on the local
// default hitcircle must not accidentally acquire the default skin's tail
// animation; this is the distinction between Lazer's legacy component lookup
// and an unconditional texture fallback.
func GetLegacySliderEndTextures() (circle, overlay *texture.TextureRegion, ok bool) {
	hitCircle, source := GetTextureWithSource("hitcircle")
	if hitCircle == nil || source == UNKNOWN || (source == LOCAL && CurrentSkin != defaultName) {
		return nil, nil, false
	}

	lookupPrefix := "hitcircle"
	if GetTextureSource("sliderendcircle", source) != nil {
		lookupPrefix = "sliderendcircle"
	}

	// The provider chooses the prefix, while the final lookup follows the
	// normal hierarchy. This matches Lazer's handling of a fallback skin while
	// preventing a custom slider-end circle from inheriting hitcircleoverlay.
	circle = GetTexture(lookupPrefix)
	if circle == nil {
		circle = hitCircle
	}
	overlay = GetTexture(lookupPrefix + "overlay")

	return circle, overlay, true
}

func GetTextureSource(name string, source Source) *texture.TextureRegion {
	checkInit()

	textureLock.Lock()
	defer textureLock.Unlock()

	source = source & (^BEATMAP)

	if CurrentSkin == defaultName {
		source = source & (^(SKIN | FALLBACK))
	}

	if CurrentSkin == FallbackSkin || FallbackSkin == defaultName {
		source = source & (^FALLBACK)
	}

	if source&SKIN > 0 {
		if rg, exists := skinCache[name]; exists {
			if rg != nil {
				return rg
			}
		} else {
			rg := loadTexture(name+".png", SKIN)
			skinCache[name] = rg

			if rg != nil {
				sourceCache[rg] = SKIN
				return rg
			}
		}
	}

	if source&FALLBACK > 0 {
		if rg, exists := fallbackCache[name]; exists {
			if rg != nil {
				return rg
			}
		} else {
			rg := loadTexture(name+".png", FALLBACK)
			fallbackCache[name] = rg

			if rg != nil {
				sourceCache[rg] = FALLBACK
				return rg
			}
		}
	}

	if source&LOCAL > 0 {
		if rg, exists := defaultCache[name]; exists {
			return rg
		}

		rg := loadTexture(name+".png", LOCAL)
		defaultCache[name] = rg

		if rg != nil {
			sourceCache[rg] = LOCAL
		}

		return rg
	}

	return nil
}

func GetFrames(name string, useDash bool) []*texture.TextureRegion {
	if rg, exists := animationCache[name]; exists {
		return rg
	}

	dash := ""
	if useDash {
		dash = "-"
	}

	textures := make([]*texture.TextureRegion, 0)

	spTexture := GetTexture(name)
	frame := GetTexture(name + dash + "0")

	if frame != nil && frame == GetMostSpecific(frame, spTexture) {
		source := sourceCache[frame]

		for i := 1; frame != nil; i++ {
			textures = append(textures, frame)
			frame = GetTextureSource(name+dash+strconv.Itoa(i), source)
		}
	} else if spTexture != nil {
		textures = append(textures, spTexture)
	}

	animationCache[name] = textures

	return textures
}

func GetMostSpecific(rg1, rg2 *texture.TextureRegion) *texture.TextureRegion {
	if rg1 == nil {
		return rg2
	}

	if rg2 == nil {
		return rg1
	}

	rg1S := sourceCache[rg1]
	rg2S := sourceCache[rg2]

	if rg1S == BEATMAP ||
		rg1S == SKIN && rg2S != BEATMAP ||
		rg1S == FALLBACK && rg2S != SKIN && rg2S != BEATMAP ||
		rg1S == LOCAL && rg2S != FALLBACK && rg2S != SKIN && rg2S != BEATMAP {
		return rg1
	}

	return rg2
}

func GetSource(name string) Source {
	tx := GetTexture(name)
	if tx == nil {
		return UNKNOWN
	}

	return sourceCache[tx]
}

func GetSourceFromTexture(rg *texture.TextureRegion) Source {
	if rg == nil {
		return UNKNOWN
	}

	return sourceCache[rg]
}

func checkAtlas() {
	if atlas == nil {
		mipmaps := 0
		if settings.RECORD {
			mipmaps = 4
		}

		atlas = texture.NewTextureAtlas(2048, mipmaps)
		atlas.Bind(27)
	}
}

func getPixmap(name string, source Source) (*texture.Pixmap, error) {
	if source == LOCAL {
		return assets.GetPixmap(filepath.Join("assets", "default-skin", name))
	}

	var path string
	var err error

	if source == SKIN {
		path, err = skinPathCache.GetFile(name)
	} else if source == FALLBACK {
		path, err = fallbackPathCache.GetFile(name)
	}

	if err != nil {
		return nil, err
	}

	return texture.NewPixmapFileString(path)
}

func loadTexture(name string, source Source) *texture.TextureRegion {
	ext := filepath.Ext(name)

	x2Name := strings.TrimSuffix(name, ext) + "@2x" + ext

	var region *texture.TextureRegion

	image, err := getPixmap(x2Name, source)
	if err != nil {
		image, err = getPixmap(name, source)
		if err == nil {
			region = &texture.TextureRegion{}
			region.Width = float32(image.Width)
			region.Height = float32(image.Height)
		}
	} else {
		region = &texture.TextureRegion{}
		region.Width = float32(image.Width / 2)
		region.Height = float32(image.Height / 2)
	}

	if region != nil {
		// Upload this texture in GL thread
		goroutines.CallNonBlockMain(func() {
			checkAtlas()

			var rg *texture.TextureRegion

			if image.Width <= 1000 && image.Height <= 1000 {
				rg = atlas.AddTexture(name, image.Width, image.Height, image.Data)
			}

			// If texture is too big load it separately
			if rg == nil {
				mipmaps := 0
				if settings.RECORD {
					mipmaps = 4
				}

				tx := texture.NewTextureSingle(image.Width, image.Height, mipmaps)
				tx.SetData(0, 0, image.Width, image.Height, image.Data)

				reg := tx.GetRegion()
				rg = &reg

				log.Println("SkinManager: Texture uploaded as single texture:", name)
			}

			image.Dispose()

			region.Texture = rg.Texture
			region.Layer = rg.Layer
			region.U1 = rg.U1
			region.U2 = rg.U2
			region.V1 = rg.V1
			region.V2 = rg.V2
		})
	}

	return region
}

func GetSample(name string) *bass.Sample {
	checkInit()

	soundLock.Lock()
	defer soundLock.Unlock()

	if sample, exists := sampleCache[name]; exists {
		return sample
	}

	var sample *bass.Sample

	if CurrentSkin != defaultName {
		sample = tryLoad(name, SKIN)

		if sample == nil && FallbackSkin != CurrentSkin && FallbackSkin != defaultName {
			sample = tryLoad(name, FALLBACK)
		}
	}

	if sample == nil {
		sample = tryLoad(name, LOCAL)
	}

	sampleCache[name] = sample

	return sample
}

func getSample(name string, source Source) *bass.Sample {
	if source == LOCAL {
		data, err := assets.GetBytes(filepath.Join("assets", "default-skin", name))
		if err != nil {
			return nil
		}

		return bass.NewSampleData(data)
	}

	var path string
	var err error

	if source == SKIN {
		path, err = skinPathCache.GetFile(name)
	} else if source == FALLBACK {
		path, err = fallbackPathCache.GetFile(name)
	}

	if err != nil {
		return nil
	}

	return bass.NewSample(path)
}

func tryLoad(basePath string, source Source) *bass.Sample {
	if sam := getSample(basePath+".wav", source); sam != nil {
		return sam
	}

	if sam := getSample(basePath+".ogg", source); sam != nil {
		return sam
	}

	if sam := getSample(basePath+".mp3", source); sam != nil {
		return sam
	}

	return nil
}

var beatmapColorsI []colorI
var beatmapColors []color.Color

var beatmapCustomColors = make(map[Color]color.Color)

func AddBeatmapColor(data []string) {
	switch data[0] {
	case "InputOverlayText":
		beatmapCustomColors[InputOverlayText] = ParseColor(data[1], data[0])
	case "SliderBorder":
		beatmapCustomColors[SliderBorder] = ParseColor(data[1], data[0])
	case "SliderTrackOverride":
		beatmapCustomColors[SliderTrackOverride] = ParseColor(data[1], data[0])
	case "SliderBall":
		beatmapCustomColors[SliderBall] = ParseColor(data[1], data[0])
	case "Combo1", "Combo2", "Combo3", "Combo4", "Combo5", "Combo6", "Combo7", "Combo8":
		index, _ := strconv.ParseInt(strings.TrimPrefix(data[0], "Combo"), 10, 64)
		beatmapColorsI = append(beatmapColorsI, colorI{
			index: int(index),
			color: ParseColor(data[1], data[0]),
		})
	}
}

func FinishBeatmapColors() {
	if len(beatmapColorsI) > 0 {
		sort.SliceStable(beatmapColorsI, func(i, j int) bool {
			return beatmapColorsI[i].index <= beatmapColorsI[j].index
		})

		beatmapColors = make([]color.Color, 0)

		for _, c := range beatmapColorsI {
			beatmapColors = append(beatmapColors, c.color)
		}
	}
}

func GetColors() []color.Color {
	if settings.Skin.UseBeatmapColors && len(beatmapColors) > 0 {
		return beatmapColors
	}

	return info.ComboColors
}

func TryGetColor(col Color) (color.Color, bool) {
	if settings.Skin.UseBeatmapColors {
		if c, ok := beatmapCustomColors[col]; ok {
			return c, true
		}
	}

	c, ok := info.Colors[col]
	return c, ok
}

func GetColor(col Color) color.Color {
	c, ok := TryGetColor(col)

	if !ok {
		panic("missing color in skin info")
	}

	return c
}

// GetObjectColor resolves the active combo palette for one hit object and
// applies the configured perceived-brightness normalization to the resolved
// combo color. Danser's independent rainbow/base-color mode remains untouched
// because it is not a combo palette.
func GetObjectColor(comboSet, comboSetHax int, base color.Color) color.Color {
	baseColor := color.NewRGB(base.R, base.G, base.B)

	comboColor, ok := resolveComboColor(comboSet, comboSetHax)
	if !ok {
		return baseColor
	}

	return color.NormalizePerceivedBrightness(
		comboColor,
		float32(settings.Objects.Colors.ComboColorNormalization),
	)
}

// resolveComboColor preserves the existing source precedence while making it
// explicit whether a combo color was actually selected. That distinction is
// required because Lazer normalizes combo accent colors, not Danser's separate
// generated object-color fallback or explicit slider override colors.
func resolveComboColor(comboSet, comboSetHax int) (color.Color, bool) {
	if settings.Skin.UseColorsFromSkin {
		colors := GetColors()
		if len(colors) > 0 {
			colorSet := comboSet
			if settings.Skin.UseBeatmapColors {
				colorSet = comboSetHax
			}

			return colors[colorSet%len(colors)], true
		}
	}

	if !settings.Objects.Colors.UseComboColors &&
		!settings.Objects.Colors.UseSkinComboColors &&
		!settings.Objects.Colors.UseBeatmapComboColors {
		return color.Color{}, false
	}

	colorSet := comboSet
	if settings.Objects.Colors.UseBeatmapComboColors {
		colorSet = comboSetHax
	}

	if settings.Objects.Colors.UseBeatmapComboColors && len(beatmapColors) > 0 {
		return beatmapColors[colorSet%len(beatmapColors)], true
	}

	if settings.Objects.Colors.UseSkinComboColors && len(info.ComboColors) > 0 {
		return info.ComboColors[colorSet%len(info.ComboColors)], true
	}

	if settings.Objects.Colors.UseComboColors && len(settings.Objects.Colors.ComboColors) > 0 {
		comboHSV := settings.Objects.Colors.ComboColors[colorSet%len(settings.Objects.Colors.ComboColors)]
		r, g, b := color.HSVToRGB(float32(comboHSV.Hue), float32(comboHSV.Saturation), float32(comboHSV.Value))
		return color.NewRGB(r, g, b), true
	}

	return color.Color{}, false
}
