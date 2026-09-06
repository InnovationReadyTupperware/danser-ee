package gcontext

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/innovationreadytupperware/danser-ee/framework/assets"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

type OptionalProps struct {
	IconName       string
	BuiltinMSAA    bool
	Resizable      bool
	ScaleToMonitor bool
	Hidden         bool
	Fullscreen     bool
}

var (
	sdlWindow      *sdl.Window
	sdlContext     sdl.GLContext
	sdlShouldClose bool
	offscreenCtx   bool
	hovered        bool
	keyMap         = make(map[sdl.Keycode]bool)
	kMutex         sync.RWMutex
	fullscreen     bool
)

// SDLWindow exposes the raw SDL window handle backing the shared GL context,
// or nil when no window has been created yet (for example in offscreen mode).
// Consumers like native file dialogs use it as a parent handle so popups layer
// and focus correctly; passing nil is always safe - the platform then creates
// an unparented window.
func SDLWindow() *sdl.Window {
	return sdlWindow
}

func Initialize(offscreen bool) error {
	libPath := filepath.Join(env.LibDir(), "SDL3.dll")
	if runtime.GOOS != "windows" {
		libPath = filepath.Join(env.LibDir(), "libSDL3.so")
	}

	if err := sdl.LoadLibrary(libPath); err != nil {
		return fmt.Errorf("sdl: couldn't load library: %w", err)
	}

	_ = sdl.SetHint(sdl.HINT_MOUSE_FOCUS_CLICKTHROUGH, "1")

	if err := sdl.SetHint("SDL_WINDOWS_DPI_AWARENESS", "derptest"); err != nil {
		return fmt.Errorf(`sdl: couldn't set hint "SDL_WINDOWS_DPI_AWARENESS": %w`, err)
	} // we set garbage value here so we can set proper one just before creating the window

	if offscreen && runtime.GOOS != "windows" {
		if err := sdl.SetHint(sdl.HINT_VIDEO_DRIVER, "offscreen"); err != nil {
			return fmt.Errorf(`sdl: couldn't set hint "%s": %w`, sdl.HINT_VIDEO_DRIVER, err)
		}

		offscreenCtx = true
	}

	return sdl.Init(sdl.INIT_VIDEO) // preinitialize to get access to display info - it will be reinitialized during window creation
}

func SDLCreateWindow(width, height int, title string, props OptionalProps) error {
	sdl.QuitSubSystem(sdl.INIT_VIDEO)

	if props.ScaleToMonitor {
		// We want the window to be rescaled
		if err := sdl.SetHint("SDL_WINDOWS_DPI_AWARENESS", "unaware"); err != nil {
			return fmt.Errorf("sdl: couldn't enable monitor scaling: %w", err)
		}
	} else {
		if err := sdl.SetHint("SDL_WINDOWS_DPI_AWARENESS", ""); err != nil {
			return fmt.Errorf("sdl: couldn't restore native monitor scaling: %w", err)
		}
	}

	if err := sdl.InitSubSystem(sdl.INIT_VIDEO); err != nil {
		return fmt.Errorf("sdl: couldn't reinitialize video subsystem: %w", err)
	}

	flags := sdl.WINDOW_OPENGL

	if props.Resizable {
		flags |= sdl.WINDOW_RESIZABLE
	}

	if props.Hidden {
		flags |= sdl.WINDOW_HIDDEN
	}

	// Weak drivers can refuse the requested framebuffer (GLXBadFBConfig).
	// Step down through degraded configurations instead of failing outright.
	var err error
	usedMSAA := props.BuiltinMSAA
	for i, config := range glConfigs(props.BuiltinMSAA) {
		if i > 0 {
			log.Printf("OpenGL: Warning: %v; retrying with multisampling=%t sRGB=%t", err, config.msaa, config.srgb)
		}
		if createErr := createGLWindow(width, height, title, flags, config); createErr != nil {
			err = createErr
			continue
		}
		err = nil
		usedMSAA = config.msaa
		break
	}
	if err != nil {
		return fmt.Errorf("sdl: couldn't create OpenGL context in any framebuffer configuration (%v)\ndanser needs OpenGL 4.5 core: update the graphics driver or use a supported GPU", err)
	}

	if err = validateGLContext(usedMSAA); err != nil {
		return err
	}

	if props.Fullscreen {
		// The context already exists here. SDL maps visible windows at
		// creation, so enter fullscreen from a mapped window rather than
		// showing a window whose transition is still pending.
		display := sdl.GetPrimaryDisplay()
		currentMode, err := display.CurrentDisplayMode()
		if err != nil {
			return fmt.Errorf("sdl: couldn't get current display mode: %w", err)
		}

		// CurrentDisplayMode returns SDL-owned const storage. Ask SDL for a real
		// supported mode instead of modifying that shared object in place.
		fullscreenMode, err := display.ClosestFullscreenDisplayMode(
			int32(width),
			int32(height),
			currentMode.RefreshRate,
			false,
		)
		if err != nil {
			return fmt.Errorf("sdl: couldn't find fullscreen display mode: %w", err)
		}

		if err = sdlWindow.SetFullscreenMode(fullscreenMode); err != nil {
			return fmt.Errorf("sdl: couldn't set fullscreen display mode: %w", err)
		}

		if err = sdlWindow.SetFullscreen(true); err != nil {
			return fmt.Errorf("sdl: couldn't enter fullscreen: %w", err)
		}

		// SDL3 fullscreen changes can be asynchronous. Rendering starts as soon as
		// this function returns, so wait until the native window state is settled.
		if err = sdlWindow.Sync(); err != nil {
			return fmt.Errorf("sdl: couldn't synchronize fullscreen window: %w", err)
		}

		fullscreen = true
	}

	if !offscreenCtx {
		if err = sdlWindow.StartTextInput(); err != nil {
			return fmt.Errorf("sdl: couldn't start text input: %w", err)
		}
	}

	if props.IconName != "" && !offscreenCtx {
		loadIconsSDL(props.IconName)
	}

	return nil
}

// glConfig is one framebuffer configuration to attempt, most demanding first.
type glConfig struct {
	msaa bool
	srgb bool
}

// glConfigs orders the framebuffer configurations for a window request:
// the requested one first, then degraded ones weaker drivers may accept.
func glConfigs(requestMSAA bool) []glConfig {
	configs := []glConfig{{msaa: requestMSAA, srgb: true}}
	if requestMSAA {
		configs = append(configs, glConfig{srgb: true})
	}
	return append(configs, glConfig{})
}

// createGLWindow requests the framebuffer in config, creates the window
// and its OpenGL context, and publishes both on success. A window left
// behind by a failed context creation is destroyed before returning.
func createGLWindow(width, height int, title string, flags sdl.WindowFlags, config glConfig) error {
	var srgb, buffers, samples int32
	if config.srgb {
		srgb = 1
	}
	if config.msaa {
		buffers, samples = 1, 4
	}

	if err := setGLAttribute("sRGB-capable framebuffer", sdl.GL_FRAMEBUFFER_SRGB_CAPABLE, srgb); err != nil {
		return err
	}

	if err := setGLAttribute("context major version", sdl.GL_CONTEXT_MAJOR_VERSION, 4); err != nil {
		return err
	}

	if err := setGLAttribute("context minor version", sdl.GL_CONTEXT_MINOR_VERSION, 5); err != nil {
		return err
	}

	if err := setGLAttribute("core profile", sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_CORE); err != nil {
		return err
	}

	if err := setGLAttribute("multisample buffers", sdl.GL_MULTISAMPLEBUFFERS, buffers); err != nil {
		return err
	}

	if err := setGLAttribute("multisample samples", sdl.GL_MULTISAMPLESAMPLES, samples); err != nil {
		return err
	}

	window, err := sdl.CreateWindow(title, width, height, flags)
	if err != nil {
		return fmt.Errorf("sdl: couldn't create window: %w", err)
	}

	context, err := sdl.GL_CreateContext(window)
	if err != nil {
		window.Destroy()
		return fmt.Errorf("sdl: couldn't create OpenGL context: %w", err)
	}

	sdlWindow = window
	sdlContext = context
	return nil
}

func setGLAttribute(name string, attribute sdl.GLAttr, value int32) error {
	if err := sdl.GL_SetAttribute(attribute, value); err != nil {
		return fmt.Errorf("sdl: couldn't request OpenGL %s=%d: %w", name, value, err)
	}

	return nil
}

func validateGLContext(builtinMSAA bool) error {
	major, err := getGLAttribute("context major version", sdl.GL_CONTEXT_MAJOR_VERSION)
	if err != nil {
		return err
	}

	minor, err := getGLAttribute("context minor version", sdl.GL_CONTEXT_MINOR_VERSION)
	if err != nil {
		return err
	}

	profile, err := getGLAttribute("context profile", sdl.GL_CONTEXT_PROFILE_MASK)
	if err != nil {
		return err
	}

	srgbCapable, err := getGLAttribute("sRGB-capable framebuffer", sdl.GL_FRAMEBUFFER_SRGB_CAPABLE)
	if err != nil {
		return err
	}

	if major < 4 || major == 4 && minor < 5 || profile&sdl.GL_CONTEXT_PROFILE_CORE == 0 {
		return fmt.Errorf(
			"sdl: OpenGL 4.5 core is required, but SDL created version %d.%d with profile mask %#x; update the graphics driver or use a supported GPU",
			major,
			minor,
			profile,
		)
	}

	log.Printf(
		"OpenGL: SDL created %d.%d core context (profile mask %#x, sRGB-capable framebuffer=%t)",
		major,
		minor,
		profile,
		srgbCapable != 0,
	)

	if !builtinMSAA {
		return nil
	}

	buffers, err := getGLAttribute("multisample buffers", sdl.GL_MULTISAMPLEBUFFERS)
	if err != nil {
		return err
	}

	samples, err := getGLAttribute("multisample samples", sdl.GL_MULTISAMPLESAMPLES)
	if err != nil {
		return err
	}

	if buffers < 1 || samples < 4 {
		return fmt.Errorf(
			"sdl: requested a 4x multisampled default framebuffer, but SDL created buffers=%d samples=%d",
			buffers,
			samples,
		)
	}

	log.Printf("OpenGL: SDL default framebuffer uses %dx MSAA", samples)

	return nil
}

func getGLAttribute(name string, attribute sdl.GLAttr) (int32, error) {
	value, err := sdl.GL_GetAttribute(attribute)
	if err != nil {
		return 0, fmt.Errorf("sdl: couldn't query actual OpenGL %s: %w", name, err)
	}

	return value, nil
}

func GetFramebufferSize() (int, int) {
	w, h, err := sdlWindow.Size()
	if err != nil {
		panic(err)
	}

	return int(w), int(h)
}

func IsHovered() bool {
	return hovered
}

func IsFocused() bool {
	return sdlWindow.Flags()&sdl.WINDOW_INPUT_FOCUS > 0
}

func Focus() {
	if err := sdlWindow.Raise(); err != nil {
		panic(err)
	}
}

func IsMinimized() bool {
	return sdlWindow.Flags()&sdl.WINDOW_MINIMIZED > 0
}

func GetCursorPosition() (float32, float32) {
	if sdlWindow.RelativeMouseMode() {
		_, x, y := sdl.GetMouseState()

		return x, y
	}

	xW, yW, _ := sdlWindow.Position()
	_, xG, yG := sdl.GetGlobalMouseState()

	return xG - float32(xW), yG - float32(yW)
}

func GetRelativePosition() (float32, float32) {
	_, xG, yG := sdl.GetRelativeMouseState()

	return xG, yG
}

func getWindowBounds() (tl, br vector.Vector2f) {
	xW, yW, _ := sdlWindow.Position()
	w, h, _ := sdlWindow.SizeInPixels()

	return vector.NewVec2f(float32(xW), float32(yW)), vector.NewVec2f(float32(xW)+float32(w), float32(yW)+float32(h))
}

func SetCursorPosition(x, y float32) {
	tl, br := getWindowBounds()

	if x < 0 || x > br.X-tl.X || y < 0 || y > br.Y-tl.Y {
		hovered = false
	} else {
		hovered = true
	}

	err := sdl.WarpMouseGlobal(x+tl.X, y+tl.Y)
	if err != nil {
		panic(err)
	}
}

func SetWindowCursorPosition(x, y float32) {
	sdlWindow.WarpMouseIn(x, y)
}

func SetRawInput(on bool) {
	if err := sdlWindow.SetRelativeMouseMode(on); err != nil {
		panic(err)
	}
}

func SetCursorVisible(visible bool) {
	if visible {
		if err := sdl.ShowCursor(); err != nil {
			return
		}
	} else {
		if err := sdl.HideCursor(); err != nil {
			return
		}
	}
}

func GetLeftClick() bool {
	flags, _, _ := sdl.GetMouseState()

	return flags&(1<<(sdl.BUTTON_LEFT-1)) != 0
}

func GetRightClick() bool {
	flags, _, _ := sdl.GetMouseState()

	return flags&(1<<(sdl.BUTTON_RIGHT-1)) != 0
}

func GetKeyState(key sdl.Keycode) Action {
	kMutex.RLock()
	defer kMutex.RUnlock()

	if keyMap[key] {
		return Press
	}

	return Release
}

func Minimize() {
	if err := sdlWindow.Minimize(); err != nil {
		panic(err)
	}
}

func Restore() {
	if err := sdlWindow.Restore(); err != nil {
		panic(err)
	}
}

func ShouldClose() bool {
	return sdlShouldClose
}

func SetShouldClose(shouldClose bool) {
	sdlShouldClose = shouldClose
}

func StartProgress() {
	if err := sdlWindow.SetProgressState(sdl.PROGRESS_STATE_NORMAL); err != nil {
		panic(err)
	}
}

func StopProgress() {
	if err := sdlWindow.SetProgressState(sdl.PROGRESS_STATE_NONE); err != nil {
		panic(err)
	}
}

func ErrorProgress() {
	if err := sdlWindow.SetProgressState(sdl.PROGRESS_STATE_ERROR); err != nil {
		panic(err)
	}
}

func SetProgress(value float32) {
	if err := sdlWindow.SetProgressValue(value); err != nil {
		panic(err)
	}
}

func loadIconsSDL(name string) {
	var iconSizes = []int{128, 64, 48, 32, 24, 16}
	if runtime.GOOS == "windows" { // windows looks broken with higher res icons, so 32px one is the first
		iconSizes = []int{32, 128, 64, 48, 24, 16}
	}

	var mainIcon *sdl.Surface

	var toDispose []*texture.Pixmap

	for i, size := range iconSizes {
		pxMap, _ := assets.GetPixmap("assets/textures/" + strings.Replace(name, "*", strconv.Itoa(size), 1) + ".png")

		icon, err := sdl.CreateSurfaceFrom(size, size, sdl.PIXELFORMAT_RGBA32, pxMap.Data, size*4)

		if err != nil {
			panic(err)
		}

		if i == 0 {
			mainIcon = icon
		} else if err = mainIcon.AddAlternateImage(icon); err != nil {
			panic(err)
		}

		toDispose = append(toDispose, pxMap)
	}

	err := sdlWindow.SetIcon(mainIcon)
	if err != nil {
		panic(err)
	}

	for _, pxMap := range toDispose {
		pxMap.Dispose()
	}
}

func GetPrimaryRefreshRate() float32 {
	if offscreenCtx {
		return 60
	}

	data, err := sdl.GetPrimaryDisplay().CurrentDisplayMode()
	if err != nil {
		panic(err)
	}

	return data.RefreshRate
}

func GetPrimaryVideoMode() sdl.DisplayMode {
	if offscreenCtx {
		return sdl.DisplayMode{
			W:           3840,
			H:           2160,
			RefreshRate: 60,
		}
	}

	data, err := sdl.GetPrimaryDisplay().CurrentDisplayMode()
	if err != nil {
		panic(err)
	}

	return *data
}

func AddToClipboard(text string) {
	if err := sdl.SetClipboardText(text); err != nil {
		panic(err)
	}
}
