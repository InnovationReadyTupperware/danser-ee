package buffer

import (
	"fmt"
	"runtime"

	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/history"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/profiler"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
)

// Framebuffer is a fixed resolution texture that you can draw on.
type Framebuffer struct {
	handle uint32

	width  int
	height int

	tex             *texture.TextureSingle
	texs            []texture.Texture
	multisampled    bool
	helperHandle    uint32
	helperTexture   uint32
	depth           uint32
	texRenderbuffer uint32

	disposed bool
}

// NewFrame creates a new fully transparent Framebuffer with given dimensions in pixels.
func NewFrame(width, height int, smooth, depth bool) *Framebuffer {
	f := new(Framebuffer)
	f.width = width
	f.height = height

	f.tex = texture.NewTextureSingle(width, height, 0)

	gl.CreateFramebuffers(1, &f.handle)

	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT0, f.tex.GetID(), 0, 0)

	if depth {
		gl.CreateRenderbuffers(1, &f.depth)

		gl.NamedRenderbufferStorage(f.depth, gl.DEPTH_COMPONENT, int32(width), int32(height))
		gl.NamedFramebufferRenderbuffer(f.handle, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, f.depth)
	}

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f
}

func NewFrameF(width, height int) *Framebuffer {
	f := new(Framebuffer)
	f.width = width
	f.height = height

	f.tex = texture.NewTextureSingleFormat(width, height, texture.RGBA32F, 0)
	f.tex.SetFiltering(texture.Filtering.Nearest, texture.Filtering.Nearest)

	gl.CreateFramebuffers(1, &f.handle)

	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT0, f.tex.GetID(), 0, 0)

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f
}

func NewFrameLayer(texture texture.Texture, layer int) *Framebuffer {
	f := new(Framebuffer)
	f.width = int(texture.GetWidth())
	f.height = int(texture.GetHeight())

	gl.CreateFramebuffers(1, &f.handle)

	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT0, texture.GetID(), 0, int32(layer))

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f
}

func NewFrameDepth(width, height int, smooth bool) *Framebuffer {
	f := new(Framebuffer)
	f.width = width
	f.height = height

	f.tex = texture.NewTextureSingleFormat(width, height, texture.Depth, 0)

	gl.CreateFramebuffers(1, &f.handle)

	gl.NamedFramebufferTextureLayer(f.handle, gl.DEPTH_ATTACHMENT, f.tex.GetID(), 0, 0)

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f
}

// NewFrameMultisample creates a multisample color framebuffer and its
// single-sample resolve target. A zero sample count creates an ordinary
// texture-backed framebuffer; unsupported positive counts are returned as
// errors before the framebuffer is used.
func NewFrameMultisample(width, height int, samples int) (*Framebuffer, error) {
	if samples < 0 {
		return nil, fmt.Errorf("MSAA sample count cannot be negative: %d", samples)
	}

	if samples == 0 {
		return NewFrame(width, height, false, false), nil
	}

	if err := validateMultisampleSamples(samples); err != nil {
		return nil, err
	}

	f := new(Framebuffer)
	f.width = width
	f.height = height
	f.multisampled = true

	gl.CreateFramebuffers(1, &f.handle)
	if err := checkOpenGLError("create multisample framebuffer"); err != nil {
		f.Dispose()
		return nil, err
	}

	gl.CreateRenderbuffers(1, &f.texRenderbuffer)
	gl.NamedRenderbufferStorageMultisample(f.texRenderbuffer, int32(samples), texture.RGBA.InternalFormat(), int32(width), int32(height))
	gl.NamedFramebufferRenderbuffer(f.handle, gl.COLOR_ATTACHMENT0, gl.RENDERBUFFER, f.texRenderbuffer)
	if err := validateMultisampleFramebuffer(f.handle, f.texRenderbuffer, samples, "multisample framebuffer"); err != nil {
		f.Dispose()
		return nil, err
	}

	f.tex = texture.NewTextureSingle(width, height, 0)

	gl.CreateFramebuffers(1, &f.helperHandle)
	gl.NamedFramebufferTextureLayer(f.helperHandle, gl.COLOR_ATTACHMENT0, f.tex.GetID(), 0, 0)
	if err := checkFramebufferComplete(f.helperHandle, "multisample resolve framebuffer"); err != nil {
		f.Dispose()
		return nil, err
	}

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f, nil
}

// NewFrameMultisampleScreen creates a multisample framebuffer intended for
// the screen or an external resolve target. A zero sample count creates an
// ordinary framebuffer without a multisample resolve step.
func NewFrameMultisampleScreen(width, height int, depth bool, samples int) (*Framebuffer, error) {
	if samples < 0 {
		return nil, fmt.Errorf("MSAA sample count cannot be negative: %d", samples)
	}

	if samples == 0 {
		return NewFrame(width, height, false, depth), nil
	}

	if err := validateMultisampleSamples(samples); err != nil {
		return nil, err
	}

	f := new(Framebuffer)
	f.width = width
	f.height = height
	f.multisampled = true

	gl.CreateFramebuffers(1, &f.handle)
	if err := checkOpenGLError("create multisample screen framebuffer"); err != nil {
		f.Dispose()
		return nil, err
	}

	gl.CreateRenderbuffers(1, &f.texRenderbuffer)
	gl.NamedRenderbufferStorageMultisample(f.texRenderbuffer, int32(samples), texture.RGBA.InternalFormat(), int32(width), int32(height))
	gl.NamedFramebufferRenderbuffer(f.handle, gl.COLOR_ATTACHMENT0, gl.RENDERBUFFER, f.texRenderbuffer)

	if depth {
		gl.CreateRenderbuffers(1, &f.depth)
		gl.NamedRenderbufferStorageMultisample(f.depth, int32(samples), gl.DEPTH_COMPONENT, int32(width), int32(height))
		gl.NamedFramebufferRenderbuffer(f.handle, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, f.depth)
	}

	if err := validateMultisampleFramebuffer(f.handle, f.texRenderbuffer, samples, "multisample screen framebuffer"); err != nil {
		f.Dispose()
		return nil, err
	}

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f, nil
}

func validateMultisampleSamples(samples int) error {
	var maxSamples int32
	gl.GetIntegerv(gl.MAX_SAMPLES, &maxSamples)
	if err := checkOpenGLError("query GL_MAX_SAMPLES"); err != nil {
		return err
	}

	return validateRequestedMultisampleSamples(samples, int(maxSamples))
}

func validateRequestedMultisampleSamples(samples, maxSamples int) error {
	if samples < 0 {
		return fmt.Errorf("MSAA sample count cannot be negative: %d", samples)
	}

	if samples > maxSamples {
		return fmt.Errorf("requested %dx MSAA, but this OpenGL context supports at most %dx", samples, maxSamples)
	}

	return nil
}

func validateMultisampleFramebuffer(handle, colorRenderbuffer uint32, samples int, operation string) error {
	if err := checkOpenGLError(operation + " allocation"); err != nil {
		return err
	}

	var actualSamples int32
	gl.GetNamedRenderbufferParameteriv(colorRenderbuffer, gl.RENDERBUFFER_SAMPLES, &actualSamples)
	if err := checkOpenGLError(operation + " sample query"); err != nil {
		return err
	}

	if int(actualSamples) < samples {
		return fmt.Errorf("%s allocated %dx samples, requested %dx", operation, actualSamples, samples)
	}

	return checkFramebufferComplete(handle, operation)
}

func checkFramebufferComplete(handle uint32, operation string) error {
	status := gl.CheckNamedFramebufferStatus(handle, gl.FRAMEBUFFER)
	if err := checkOpenGLError(operation + " status check"); err != nil {
		return err
	}

	if status != gl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("%s is incomplete: status 0x%X", operation, status)
	}

	return nil
}

func checkOpenGLError(operation string) error {
	if code := gl.GetError(); code != gl.NO_ERROR {
		return fmt.Errorf("%s failed with OpenGL error 0x%X", operation, code)
	}

	return nil
}

func NewFrameYUV(width, height int) *Framebuffer {
	f := new(Framebuffer)
	f.width = width
	f.height = height

	f.texs = append(f.texs, texture.NewTextureSingleFormat(width, height, texture.Red, 0))
	f.texs = append(f.texs, texture.NewTextureSingleFormat(width, height, texture.Red, 0))
	f.texs = append(f.texs, texture.NewTextureSingleFormat(width, height, texture.Red, 0))

	gl.CreateFramebuffers(1, &f.handle)

	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT0, f.texs[0].GetID(), 0, 0)
	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT1, f.texs[1].GetID(), 0, 0)
	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT2, f.texs[2].GetID(), 0, 0)

	attchs := []uint32{gl.COLOR_ATTACHMENT0, gl.COLOR_ATTACHMENT1, gl.COLOR_ATTACHMENT2}

	gl.NamedFramebufferDrawBuffers(f.handle, 3, &attchs[0])

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f
}

func NewFrameYUVSmall(width, height int) *Framebuffer {
	f := new(Framebuffer)
	f.width = width
	f.height = height

	f.texs = append(f.texs, texture.NewTextureSingleFormat(width, height, texture.RG, 0))
	f.texs = append(f.texs, texture.NewTextureSingleFormat(width, height, texture.Red, 0))

	gl.CreateFramebuffers(1, &f.handle)

	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT0, f.texs[0].GetID(), 0, 0)
	gl.NamedFramebufferTextureLayer(f.handle, gl.COLOR_ATTACHMENT1, f.texs[1].GetID(), 0, 0)

	attchs := []uint32{gl.COLOR_ATTACHMENT0, gl.COLOR_ATTACHMENT1}

	gl.NamedFramebufferDrawBuffers(f.handle, 2, &attchs[0])

	runtime.SetFinalizer(f, (*Framebuffer).Dispose)

	return f
}

func (f *Framebuffer) Dispose() {
	if !f.disposed {
		goroutines.CallNonBlockMain(func() {
			if f.tex != nil {
				f.tex.Dispose()
			}

			if f.texs != nil && len(f.texs) > 0 {
				for _, tex := range f.texs {
					tex.Dispose()
				}
			}

			if f.depth > 0 {
				gl.DeleteRenderbuffers(1, &f.depth)
			}

			if f.texRenderbuffer > 0 {
				gl.DeleteRenderbuffers(1, &f.texRenderbuffer)
			}

			if f.helperHandle > 0 {
				gl.DeleteFramebuffers(1, &f.helperHandle)
			}

			if f.helperTexture > 0 {
				gl.DeleteTextures(1, &f.helperTexture)
			}

			gl.DeleteFramebuffers(1, &f.handle)
		})
	}

	f.disposed = true
}

// GetID returns the OpenGL framebuffer ID of this Framebuffer.
func (f *Framebuffer) GetID() uint32 {
	return f.handle
}

// Bind binds the Framebuffer. All draw operations will target this Framebuffer until Unbind is called.
func (f *Framebuffer) Bind() {
	history.Push(gl.FRAMEBUFFER_BINDING, f.handle)
	gl.BindFramebuffer(gl.FRAMEBUFFER, f.handle)
	profiler.IncrementStat(profiler.FBOBinds)
}

// Unbind unbinds the Framebuffer. All draw operations will go to whatever was bound before this Framebuffer.
func (f *Framebuffer) Unbind() {
	handle := history.Pop(gl.FRAMEBUFFER_BINDING)

	if f.multisampled {
		hHandle := f.helperHandle
		if hHandle == 0 {
			hHandle = handle
		}

		gl.NamedFramebufferReadBuffer(f.handle, gl.COLOR_ATTACHMENT0)

		if hHandle > 0 {
			gl.NamedFramebufferDrawBuffer(hHandle, gl.COLOR_ATTACHMENT0)
		}

		gl.BlitNamedFramebuffer(f.handle, hHandle, 0, 0, int32(f.width), int32(f.height), 0, 0, int32(f.width), int32(f.height), gl.COLOR_BUFFER_BIT, gl.LINEAR)
	}

	if handle != 0 {
		profiler.IncrementStat(profiler.FBOBinds)
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, handle)
}

// Texture returns the Framebuffer's underlying Texture that the Framebuffer draws on.
func (f *Framebuffer) Texture() texture.Texture {
	return f.tex
}

func (f *Framebuffer) Textures() []texture.Texture {
	return f.texs
}

func (f *Framebuffer) GetWidth() int {
	return f.width
}

func (f *Framebuffer) GetHeight() int {
	return f.height
}

func (f *Framebuffer) ClearColor(r, g, b, a float32) {
	col := []float32{r, g, b, a}
	gl.ClearNamedFramebufferfv(f.handle, gl.COLOR, 0, &col[0])
}

func (f *Framebuffer) ClearColorI(index int, r, g, b, a float32) {
	col := []float32{r, g, b, a}
	gl.ClearNamedFramebufferfv(f.handle, gl.COLOR, int32(index), &col[0])
}

func (f *Framebuffer) ClearColorM(color color2.Color) {
	gl.ClearNamedFramebufferfv(f.handle, gl.COLOR, 0, &color.ToArray()[0])
}

func (f *Framebuffer) ClearColorIM(index int, color color2.Color) {
	gl.ClearNamedFramebufferfv(f.handle, gl.COLOR, int32(index), &color.ToArray()[0])
}

func (f *Framebuffer) ClearDepthV(v float32) {
	gl.ClearNamedFramebufferfv(f.handle, gl.DEPTH, 0, &v)
}

func (f *Framebuffer) ClearDepth() {
	f.ClearDepthV(1)
}
