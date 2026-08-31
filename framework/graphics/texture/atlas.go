package texture

import (
	"log"
	"runtime"

	"github.com/go-gl/gl/v4.5-core/gl"

	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
)

type rectangle struct {
	x, y, width, height int
}

func (rect rectangle) area() int {
	return rect.width * rect.height
}

type TextureAtlas struct {
	*TextureMultiLayer
	allocator   *atlasAllocator
	subTextures map[string]*TextureRegion
}

type atlasAllocator struct {
	size        int
	padding     int
	maxLayers   int
	emptySpaces [][]rectangle
}

func NewTextureAtlas(size, mipmaps int) *TextureAtlas {
	return NewTextureAtlasFormat(size, RGBA, mipmaps, 1)
}

func NewTextureAtlasCC(size, mipmaps int, clearColor color2.Color) *TextureAtlas {
	return NewTextureAtlasFormatCC(size, RGBA, mipmaps, 1, clearColor)
}

func NewTextureAtlasFormat(size int, format Format, mipmaps int, layers int) *TextureAtlas {
	return NewTextureAtlasFormatCC(size, format, mipmaps, layers, color2.NewLA(0, 0))
}

func NewTextureAtlasFormatCC(size int, format Format, mipmaps int, layers int, clearColor color2.Color) *TextureAtlas {
	if size <= 0 {
		panic("Texture atlas size must be positive")
	}

	if mipmaps < 0 || mipmaps > 30 {
		panic("Texture atlas mipmap count must be between 0 and 30")
	}

	if layers <= 0 {
		panic("Texture atlas layer count must be positive")
	}

	var maxTextureSize int32
	gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxTextureSize)
	if maxTextureSize <= 0 {
		panic("OpenGL reported an invalid maximum texture size")
	}

	if int(maxTextureSize) < size {
		log.Printf("Graphics: Warning: GPU supports only %dx%d textures; clamping requested %dx%d atlas", maxTextureSize, maxTextureSize, size, size)
		size = int(maxTextureSize)
	}

	var maxArrayLayers int32
	gl.GetIntegerv(gl.MAX_ARRAY_TEXTURE_LAYERS, &maxArrayLayers)
	if maxArrayLayers <= 0 {
		panic("OpenGL reported an invalid maximum texture-array layer count")
	}

	if int(maxArrayLayers) < layers {
		log.Printf("Graphics: Warning: GPU supports only %d texture-array layers; clamping requested %d-layer atlas", maxArrayLayers, layers)
		layers = int(maxArrayLayers)
	}

	texture := new(TextureAtlas)
	texture.subTextures = make(map[string]*TextureRegion)
	texture.allocator = newAtlasAllocator(size, 1<<uint(mipmaps), layers, int(maxArrayLayers))
	texture.TextureMultiLayer = NewTextureMultiLayerFormatCC(size, size, format, mipmaps, layers, clearColor)

	texture.defRegion = TextureRegion{texture, 0, 1, 0, 1, float32(size), float32(size), 0}

	runtime.SetFinalizer(texture, (*TextureAtlas).Dispose)

	return texture
}

func (texture *TextureAtlas) AddTexture(name string, width, height int, data []uint8) *TextureRegion {
	expectedBytes, valid := textureByteSize(width, height, texture.store.format.Size())
	if !valid || len(data) != expectedBytes {
		panic("Wrong number of pixels given!")
	}

	layer, x, y, ok := texture.allocator.allocate(width, height)
	if !ok {
		log.Printf("Texture is too big! Atlas size: %dx%d, texture size: %dx%d", texture.GetWidth(), texture.GetHeight(), width, height)
		return nil
	}

	for int(texture.store.layers) <= layer {
		texture.TextureMultiLayer.NewLayer()
	}

	texture.SetData(x, y, width, height, layer, data)

	region := TextureRegion{Texture: texture, Width: float32(width), Height: float32(height), Layer: int32(layer)}
	region.U1 = (float32(x) + 0.5) / float32(texture.store.width)
	region.V1 = (float32(y) + 0.5) / float32(texture.store.height)
	region.U2 = region.U1 + float32(width-1)/float32(texture.store.width)
	region.V2 = region.V1 + float32(height-1)/float32(texture.store.height)

	texture.subTextures[name] = &region
	return &region
}

func (texture *TextureAtlas) GetTexture(name string) *TextureRegion {
	return texture.subTextures[name]
}

func (texture *TextureAtlas) NewLayer() {
	if !texture.allocator.addLayer() {
		log.Printf("Graphics: Warning: texture atlas reached the GPU layer limit (%d)", texture.allocator.maxLayers)
		return
	}

	texture.TextureMultiLayer.NewLayer()
}

func newAtlasAllocator(size, padding, layers, maxLayers int) *atlasAllocator {
	allocator := &atlasAllocator{
		size:      size,
		padding:   padding,
		maxLayers: maxLayers,
	}

	for range min(layers, maxLayers) {
		allocator.addLayer()
	}

	return allocator
}

func (allocator *atlasAllocator) addLayer() bool {
	if len(allocator.emptySpaces) >= allocator.maxLayers {
		return false
	}

	allocator.emptySpaces = append(
		allocator.emptySpaces,
		[]rectangle{{x: 0, y: 0, width: allocator.size, height: allocator.size}},
	)

	return true
}

func (allocator *atlasAllocator) allocate(width, height int) (layer, x, y int, ok bool) {
	if width <= 0 || height <= 0 || width > allocator.size-allocator.padding ||
		height > allocator.size-allocator.padding {
		return 0, 0, 0, false
	}

	bounds := rectangle{width: width + allocator.padding, height: height + allocator.padding}
	for {
		for layer, spaces := range allocator.emptySpaces {
			spaceIndex, placement, found := findAtlasSpace(spaces, bounds)
			if !found {
				continue
			}

			allocator.emptySpaces[layer] = splitAtlasSpace(spaces, spaceIndex, placement, bounds)
			return layer, placement.x, placement.y, true
		}

		if !allocator.addLayer() {
			return 0, 0, 0, false
		}
	}
}

func findAtlasSpace(spaces []rectangle, bounds rectangle) (int, rectangle, bool) {
	spaceIndex := -1
	var smallest rectangle

	for i, space := range spaces {
		if bounds.width > space.width || bounds.height > space.height {
			continue
		}

		if spaceIndex < 0 || space.area() <= smallest.area() {
			spaceIndex = i
			smallest = space
		}
	}

	return spaceIndex, smallest, spaceIndex >= 0
}

func splitAtlasSpace(spaces []rectangle, index int, space, bounds rectangle) []rectangle {
	dw := space.width - bounds.width
	dh := space.height - bounds.height

	var first, second rectangle
	if dh > dw {
		first = rectangle{x: space.x + bounds.width, y: space.y, width: dw, height: bounds.height}
		second = rectangle{x: space.x, y: space.y + bounds.height, width: space.width, height: dh}
	} else {
		first = rectangle{x: space.x + bounds.width, y: space.y, width: dw, height: space.height}
		second = rectangle{x: space.x, y: space.y + bounds.height, width: bounds.width, height: dh}
	}

	updated := make([]rectangle, 0, len(spaces)+1)
	updated = append(updated, spaces[:index]...)
	if first.width > 0 && first.height > 0 {
		updated = append(updated, first)
	}
	if second.width > 0 && second.height > 0 {
		updated = append(updated, second)
	}
	updated = append(updated, spaces[index+1:]...)

	return updated
}

func textureByteSize(width, height, bytesPerPixel int) (int, bool) {
	if width <= 0 || height <= 0 || bytesPerPixel <= 0 {
		return 0, false
	}

	maxInt := int(^uint(0) >> 1)
	if width > maxInt/height || width*height > maxInt/bytesPerPixel {
		return 0, false
	}

	return width * height * bytesPerPixel, true
}
