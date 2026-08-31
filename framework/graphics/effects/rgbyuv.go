package effects

import (
	"fmt"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/innovationreadytupperware/danser-ee/framework/assets"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/attribute"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/blend"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/buffer"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/shader"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/viewport"
)

// bt709LimitedMatrix converts full-range nonlinear RGB into limited-range
// BT.709 YCbCr. FFmpeg receives matching bt709/tv metadata for this stream.
var bt709LimitedMatrix = mgl32.Mat4x3{
	0.183302, -0.101039, 0.440938,
	0.616640, -0.339900, -0.400505,
	0.062250, 0.440938, -0.040431,
	0.062745, 0.501961, 0.501961,
}

func convertBT709Limited(rgb mgl32.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{
		bt709LimitedMatrix[0]*rgb[0] + bt709LimitedMatrix[3]*rgb[1] + bt709LimitedMatrix[6]*rgb[2] + bt709LimitedMatrix[9],
		bt709LimitedMatrix[1]*rgb[0] + bt709LimitedMatrix[4]*rgb[1] + bt709LimitedMatrix[7]*rgb[2] + bt709LimitedMatrix[10],
		bt709LimitedMatrix[2]*rgb[0] + bt709LimitedMatrix[5]*rgb[1] + bt709LimitedMatrix[8]*rgb[2] + bt709LimitedMatrix[11],
	}
}

type RGBYUV struct {
	width  int
	height int

	fbo          *buffer.Framebuffer
	yuvFBO       *buffer.Framebuffer
	subsampleFBO *buffer.Framebuffer

	yuvShader       *shader.RShader
	subsampleShader *shader.RShader

	vao *buffer.VertexArrayObject

	subsample bool
}

func NewRGBYUV(width, height int, subsample bool) (*RGBYUV, error) {
	effect := new(RGBYUV)
	var err error
	effect.fbo, err = buffer.NewFrame(width, height, false, false)
	if err != nil {
		return nil, fmt.Errorf("create RGB recording target: %w", err)
	}
	effect.yuvFBO, err = buffer.NewFrameYUV(width, height)
	if err != nil {
		effect.fbo.Dispose()
		return nil, fmt.Errorf("create YUV recording target: %w", err)
	}
	if subsample {
		effect.subsampleFBO, err = buffer.NewFrameYUVSmall((width+1)/2, (height+1)/2)
		if err != nil {
			effect.fbo.Dispose()
			effect.yuvFBO.Dispose()
			return nil, fmt.Errorf("create subsampled YUV recording target: %w", err)
		}
	}

	effect.width = width
	effect.height = height
	effect.subsample = subsample

	vert, err := assets.GetString("assets/shaders/fbopass.vsh")
	if err != nil {
		panic(err)
	}

	frag, err := assets.GetString("assets/shaders/rgbyuv.fsh")
	if err != nil {
		panic(err)
	}

	fpass, err := assets.GetString("assets/shaders/rgbyuv_scale.fsh")
	if err != nil {
		panic(err)
	}

	effect.yuvShader = shader.NewRShader(shader.NewSource(vert, shader.Vertex), shader.NewSource(frag, shader.Fragment))
	effect.yuvShader.SetUniform("rgbToYuv", bt709LimitedMatrix)

	effect.subsampleShader = shader.NewRShader(shader.NewSource(vert, shader.Vertex), shader.NewSource(fpass, shader.Fragment))

	effect.vao = buffer.NewVertexArrayObject()

	effect.vao.AddVBO("default", 6, 0, attribute.Format{
		{Name: "in_position", Type: attribute.Vec3},
		{Name: "in_tex_coord", Type: attribute.Vec2},
	})

	effect.vao.SetData("default", 0, []float32{
		-1, -1, 0, 0, 0,
		1, -1, 0, 1, 0,
		-1, 1, 0, 0, 1,
		1, -1, 0, 1, 0,
		1, 1, 0, 1, 1,
		-1, 1, 0, 0, 1,
	})

	effect.vao.Attach(effect.yuvShader)

	return effect, nil
}

func (effect *RGBYUV) Begin() {
	effect.fbo.Bind()
	viewport.Push(effect.width, effect.height)
}

func (effect *RGBYUV) End() {
	viewport.Pop()
	effect.fbo.Unbind()
}

func (effect *RGBYUV) Draw() (yuv, uv []texture.Texture) {
	blend.Push()
	blend.Disable()

	effect.fbo.Texture().Bind(5)

	effect.yuvShader.SetUniform("tex", 5)

	effect.vao.Bind()

	effect.yuvFBO.Bind()

	viewport.Push(effect.width, effect.height)

	effect.yuvShader.Bind()

	effect.vao.Draw()

	effect.yuvShader.Unbind()

	viewport.Pop()

	effect.yuvFBO.Unbind()

	if effect.subsample {
		effect.yuvFBO.Textures()[1].Bind(6)
		effect.yuvFBO.Textures()[2].Bind(7)

		effect.subsampleShader.SetUniform("texU", 6)
		effect.subsampleShader.SetUniform("texV", 7)

		effect.subsampleFBO.Bind()

		viewport.Push(effect.subsampleFBO.GetWidth(), effect.subsampleFBO.GetHeight())

		effect.subsampleShader.Bind()

		effect.vao.Draw()

		effect.subsampleShader.Unbind()

		viewport.Pop()

		effect.subsampleFBO.Unbind()
	}

	effect.vao.Unbind()

	blend.Pop()

	if effect.subsampleFBO == nil {
		return effect.yuvFBO.Textures(), nil
	}

	return effect.yuvFBO.Textures(), effect.subsampleFBO.Textures()
}
