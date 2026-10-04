package skin

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/gl/v4.5-core/gl"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/files"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
)

// Run this opt-in test from the repository root with the test binary beside
// SDL3.dll: DANSER_GL_TESTS=1 ./danser-skin.test.exe -test.timeout=30s
func TestRecordingTexturesReadyOnWorkerAndRenderThreads(t *testing.T) {
	if os.Getenv("DANSER_GL_TESTS") != "1" {
		t.Skip("requires a real OpenGL context; set DANSER_GL_TESTS=1")
	}
	dir := t.TempDir()
	for _, name := range []string{"worker.png", "render@2x.png"} {
		file, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		pixels := image.NewRGBA(image.Rect(0, 0, 32, 16))
		for y := range 16 {
			for x := range 32 {
				pixels.SetRGBA(x, y, color.RGBA{R: 51, G: 102, B: 153, A: 255})
			}
		}
		err = png.Encode(file, pixels)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("write texture: %v; close: %v", err, closeErr)
		}
	}
	fileMap, err := files.NewFileMap(dir)
	if err != nil {
		t.Fatal(err)
	}
	oldRecord, oldInfo := settings.RECORD, info
	oldCurrent, oldFallback, oldPath := CurrentSkin, FallbackSkin, skinPathCache
	oldSkinCache, oldSourceCache, oldAtlas := skinCache, sourceCache, atlas
	t.Cleanup(func() {
		settings.RECORD, info = oldRecord, oldInfo
		CurrentSkin, FallbackSkin, skinPathCache = oldCurrent, oldFallback, oldPath
		skinCache, sourceCache, atlas = oldSkinCache, oldSourceCache, oldAtlas
	})
	settings.RECORD = true
	info = newDefaultInfo()
	CurrentSkin, FallbackSkin, skinPathCache = "test", defaultName, fileMap
	skinCache = make(map[string]*texture.TextureRegion)
	sourceCache = make(map[*texture.TextureRegion]Source)
	atlas = nil
	env.Init("danser-ee")
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var setupErr, workerErr, renderErr error
	var workerIsMain, renderIsMain bool
	goroutines.RunMain(func() {
		goroutines.CallMain(func() {
			setupErr = gcontext.Initialize(true)
			if setupErr == nil {
				setupErr = gcontext.SDLCreateWindow(64, 64, "texture-test", gcontext.OptionalProps{Hidden: true})
			}
			if setupErr == nil {
				setupErr = gcontext.GLInit(false)
			}
		})
		if setupErr != nil {
			return
		}
		workerIsMain = gcontext.IsMainThread()
		worker := GetTextureSource("worker", SKIN)
		// Inspect immediately on return, before queuing another main-thread
		// call that could otherwise hide an asynchronous upload regression
		workerErr = readyTextureRegion(worker, 32, 16)
		goroutines.CallMain(func() {
			if workerErr == nil {
				workerErr = uploadedTexturePixel(worker)
			}
			renderIsMain = gcontext.IsMainThread()
			render := GetTextureSource("render", SKIN)
			renderErr = readyTextureRegion(render, 16, 8)
			if renderErr == nil {
				renderErr = uploadedTexturePixel(render)
			}
		})
		if atlas != nil {
			atlas.Dispose()
		}
		// Dispose queues GL deletion; FIFO dispatch completes it before Quit
		goroutines.CallMain(func() { sdl.Quit() })
	})
	if setupErr != nil || workerErr != nil || renderErr != nil {
		t.Fatalf("setup=%v worker=%v render=%v", setupErr, workerErr, renderErr)
	}
	if workerIsMain || !renderIsMain {
		t.Fatal("SDL thread identity did not match the CallMain dispatcher")
	}
}

func readyTextureRegion(region *texture.TextureRegion, width, height float32) error {
	if region == nil || region.Texture == nil || region.U2 <= region.U1 || region.V2 <= region.V1 {
		return fmt.Errorf("texture upload incomplete when lookup returned: %#v", region)
	}
	if region.Width != width || region.Height != height {
		return fmt.Errorf("texture dimensions = %gx%g, want %gx%g", region.Width, region.Height, width, height)
	}
	return nil
}

func uploadedTexturePixel(region *texture.TextureRegion) error {
	x := int32((region.U1 + region.U2) * 0.5 * float32(region.Texture.GetWidth()))
	y := int32((region.V1 + region.V2) * 0.5 * float32(region.Texture.GetHeight()))
	var pixel [4]uint8
	gl.GetTextureSubImage(region.Texture.GetID(), 0, x, y, region.Layer, 1, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, 4, gl.Ptr(&pixel[0]))
	if pixel != [4]uint8{51, 102, 153, 255} {
		return fmt.Errorf("uploaded texture pixel = %v", pixel)
	}
	return nil
}
