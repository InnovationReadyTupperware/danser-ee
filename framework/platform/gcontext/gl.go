package gcontext

import "C"
import (
	"fmt"
	"log"
	"os"
	"strings"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/gl/v4.5-core/gl"

	"github.com/innovationreadytupperware/danser-ee/framework/graphics/hacks"
)

// GLInit initializes OpenGL, checks for needed extensions, eventually sets up GPU debug logs
func GLInit(debugLogs bool, additionalExtensions ...string) error {
	log.Println("Initializing OpenGL...")

	err := gl.Init()
	if err != nil {
		return err
	}

	err = extensionCheck(additionalExtensions)
	if err != nil {
		return err
	}

	var major, minor int32
	gl.GetIntegerv(gl.MAJOR_VERSION, &major)
	gl.GetIntegerv(gl.MINOR_VERSION, &minor)
	if !supportsOpenGL45(major, minor) {
		return fmt.Errorf("OpenGL 4.5 core is required, but the initialized context reports %d.%d", major, minor)
	}

	var maxSamples, maxTextureSize, maxArrayLayers, maxRenderbufferSize int32
	gl.GetIntegerv(gl.MAX_SAMPLES, &maxSamples)
	gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxTextureSize)
	gl.GetIntegerv(gl.MAX_ARRAY_TEXTURE_LAYERS, &maxArrayLayers)
	gl.GetIntegerv(gl.MAX_RENDERBUFFER_SIZE, &maxRenderbufferSize)
	if code := gl.GetError(); code != gl.NO_ERROR {
		return fmt.Errorf("query OpenGL capabilities: OpenGL error 0x%X", code)
	}

	glVendor := C.GoString((*C.char)(unsafe.Pointer(gl.GetString(gl.VENDOR))))
	glRenderer := C.GoString((*C.char)(unsafe.Pointer(gl.GetString(gl.RENDERER))))
	glVersion := C.GoString((*C.char)(unsafe.Pointer(gl.GetString(gl.VERSION))))
	glslVersion := C.GoString((*C.char)(unsafe.Pointer(gl.GetString(gl.SHADING_LANGUAGE_VERSION))))

	lVendor := strings.ToLower(glVendor)

	// HACK HACK HACK: please see github.com/innovationreadytupperware/danser-ee/framework/graphics/hacks.IsIntel for more info
	if strings.Contains(lVendor, "intel") {
		hacks.IsIntel = true
	}

	forceAMDHack := false

	if enF, ok := os.LookupEnv("DANSER_FLAGS"); ok && strings.Contains(enF, "AMDHACK") {
		forceAMDHack = true
	}

	// HACK HACK HACK: please see github.com/innovationreadytupperware/danser-ee/framework/graphics/hacks.IsOldAMD for more info
	if forceAMDHack || (strings.Contains(lVendor, "amd") || strings.Contains(lVendor, "ati")) &&
		(strings.Contains(glVersion, "15.201.") || strings.Contains(glVersion, "15.200.")) {
		hacks.IsOldAMD = true
	}

	log.Printf("OpenGL: vendor=%q renderer=%q version=%q GLSL=%q", glVendor, glRenderer, glVersion, glslVersion)
	log.Printf(
		"OpenGL: limits texture=%d array-layers=%d renderbuffer=%d samples=%d",
		maxTextureSize,
		maxArrayLayers,
		maxRenderbufferSize,
		maxSamples,
	)
	log.Println("OpenGL: initialized")

	if debugLogs {
		gl.Enable(gl.DEBUG_OUTPUT)
		gl.DebugMessageCallback(func(
			source uint32,
			glType uint32,
			id uint32,
			severity uint32,
			length int32,
			message string,
			userParam unsafe.Pointer) {
			log.Println("GL:", message)
		}, gl.Ptr(nil))

		gl.DebugMessageControl(gl.DONT_CARE, gl.DONT_CARE, gl.DONT_CARE, 0, nil, true)
	}

	return nil
}

func extensionCheck(additionalExtensions []string) (ret error) {
	var notSupported []string

	for _, ext := range additionalExtensions {
		if !sdl.GL_ExtensionSupported(ext) {
			notSupported = append(notSupported, ext)
		}
	}

	if len(notSupported) > 0 {
		return fmt.Errorf("your GPU does not support one or more required OpenGL extensions: %s. Please update your graphics drivers or upgrade your GPU", notSupported)
	}

	return nil
}

func supportsOpenGL45(major, minor int32) bool {
	return major > 4 || major == 4 && minor >= 5
}
