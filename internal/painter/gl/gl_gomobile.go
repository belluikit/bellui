// This gomobile GL painter converts to/from the gomobile binding's struct
// Texture type, which is cache.TextureType only under `(android || ios ||
// mobile) && gl` (texture_gomobile.go) — unlike the desktop GL painter it is
// NOT uint32-compatible, so it must never compile against the WebGPU `uint32`
// texture type. It carries the same package GL-selection gate as every other
// file (kept uniform so the common files always have exactly one platform impl
// in every active build view); an explicit `gl` term is not needed because the
// no-`gl` fallback branch `(!darwin && !linux && !windows && !wasm)` is already
// false on every real mobile target — `GOOS=android` sets the `linux` build
// tag and `GOOS=ios` sets `darwin`. The only way to reach this file without
// `gl` is `-tags mobile` on a non-mobile GOOS, a
// nonsensical combo no build uses. (pre-rebrand tracker #251)
//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (android || ios || mobile)

package gl

import (
	"math"

	"github.com/belluikit/bellui/internal/driver/mobile/gl"
)

const (
	arrayBuffer           = gl.ArrayBuffer
	bitColorBuffer        = gl.ColorBufferBit
	bitDepthBuffer        = gl.DepthBufferBit
	clampToEdge           = gl.ClampToEdge
	colorFormatRGBA       = gl.RGBA
	compileStatus         = gl.CompileStatus
	constantAlpha         = gl.ConstantAlpha
	float                 = gl.Float
	fragmentShader        = gl.FragmentShader
	back                  = gl.Back
	front                 = gl.Front
	glFalse               = gl.False
	linkStatus            = gl.LinkStatus
	one                   = gl.One
	oneMinusConstantAlpha = gl.OneMinusConstantAlpha
	oneMinusSrcAlpha      = gl.OneMinusSrcAlpha
	scissorTest           = gl.ScissorTest
	srcAlpha              = gl.SrcAlpha
	staticDraw            = gl.StaticDraw
	texture0              = gl.Texture0
	texture2D             = gl.Texture2D
	textureMinFilter      = gl.TextureMinFilter
	textureMagFilter      = gl.TextureMagFilter
	textureWrapS          = gl.TextureWrapS
	textureWrapT          = gl.TextureWrapT
	triangles             = gl.Triangles
	triangleStrip         = gl.TriangleStrip
	unsignedByte          = gl.UnsignedByte
	vertexShader          = gl.VertexShader

	// FBO constants
	glFramebuffer       = gl.FramebufferTarget
	colorAttachment0    = gl.ColorAttachment0
	framebufferComplete = gl.FramebufferComplete

	// Depth constants
	depthTest       = gl.DepthTest
	depthAttachment = gl.DepthAttachment
	depthComponent  = gl.DepthComponent
	unsignedShort   = gl.UnsignedShort

	// Stencil constants
	stencilTest      = gl.StencilTest
	bitStencilBuffer = gl.StencilBufferBit
	glAlways         = gl.Always
	glEqual          = gl.Equal
	glNotEqual       = gl.NotEqual
	glKeep           = gl.Keep
	glReplace        = gl.Replace
	glIncrWrap       = gl.IncrWrap
	glDecrWrap       = gl.DecrWrap
	glInvert         = gl.Invert
	glFront          = gl.Front
	glBack           = gl.Back
)

type (
	// Attribute represents a GL attribute
	Attribute gl.Attrib
	// Buffer represents a GL buffer
	Buffer gl.Buffer
	// Program represents a compiled GL program
	Program gl.Program
	// Shader represents a GL shader
	Shader gl.Shader
	// Uniform represents a GL uniform
	Uniform gl.Uniform
)

var (
	compiled          []ProgramState // avoid multiple compilations with the re-used mobile GUI context
	noBuffer          = Buffer{}
	noShader          = Shader{}
	textureFilterToGL = [...]int32{gl.Linear, gl.Nearest, gl.Linear}
)

func (p *painter) glctx() gl.Context {
	return p.contextProvider.Context().(gl.Context)
}

func (p *painter) Init() {
	p.ctx = &mobileContext{glContext: p.contextProvider.Context().(gl.Context)}
	p.glctx().Disable(gl.DepthTest)
	p.glctx().Enable(gl.Blend)
	if compiled == nil {
		p.program = ProgramState{
			ref:        p.createProgram("simple_es"),
			buff:       p.createBuffer(20),
			uniforms:   make(map[string]*UniformState),
			attributes: make(map[string]Attribute),
		}
		p.getUniformLocations(p.program, "text", "alpha", "cornerRadius", "size", "inset")
		p.enableAttribArrays(p.program, "vert", "vertTexCoord")

		p.lineProgram = ProgramState{
			ref:        p.createProgram("line_es"),
			buff:       p.createBuffer(24),
			uniforms:   make(map[string]*UniformState),
			attributes: make(map[string]Attribute),
		}
		p.getUniformLocations(p.lineProgram, "color", "feather", "lineWidth")
		p.enableAttribArrays(p.lineProgram, "vert", "normal")

		p.rectangleProgram = ProgramState{
			ref:            p.createProgram("rectangle_es"),
			buff:           p.createBuffer(16),
			uniforms:       make(map[string]*UniformState),
			matrixUniforms: make(map[string]*MatrixUniformState),
			attributes:     make(map[string]Attribute),
		}
		p.getUniformLocations(
			p.rectangleProgram,
			"frame_size", "rect_coords", "stroke_width", "fill_color", "stroke_color",
		)
		p.getMatrixUniformLocations(p.rectangleProgram, "inv_transform")
		p.enableAttribArrays(p.rectangleProgram, "vert", "normal")

		p.roundRectangleProgram = ProgramState{
			ref:            p.createProgram("round_rectangle_es"),
			buff:           p.createBuffer(16),
			uniforms:       make(map[string]*UniformState),
			matrixUniforms: make(map[string]*MatrixUniformState),
			attributes:     make(map[string]Attribute),
		}
		p.getUniformLocations(
			p.roundRectangleProgram,
			"frame_size", "rect_coords",
			"stroke_width_half", "rect_size_half",
			"radius", "edge_softness",
			"fill_color", "stroke_color",
		)
		p.getMatrixUniformLocations(p.roundRectangleProgram, "inv_transform")
		p.enableAttribArrays(p.roundRectangleProgram, "vert", "normal")

		p.polygonProgram = ProgramState{
			ref:            p.createProgram("polygon_es"),
			buff:           p.createBuffer(16),
			uniforms:       make(map[string]*UniformState),
			matrixUniforms: make(map[string]*MatrixUniformState),
			attributes:     make(map[string]Attribute),
		}
		p.getUniformLocations(
			p.polygonProgram,
			"frame_size", "rect_coords", "edge_softness",
			"outer_radius", "angle", "sides",
			"fill_color", "corner_radius",
			"stroke_width", "stroke_color",
		)
		p.getMatrixUniformLocations(p.polygonProgram, "inv_transform")
		p.enableAttribArrays(p.polygonProgram, "vert", "normal")

		p.arcProgram = ProgramState{
			ref:            p.createProgram("arc_es"),
			buff:           p.createBuffer(16),
			uniforms:       make(map[string]*UniformState),
			matrixUniforms: make(map[string]*MatrixUniformState),
			attributes:     make(map[string]Attribute),
		}
		p.getUniformLocations(
			p.arcProgram,
			"frame_size", "rect_coords",
			"inner_radius", "outer_radius",
			"start_angle", "end_angle",
			"edge_softness", "corner_radius",
			"stroke_width", "stroke_color",
			"fill_color",
		)
		p.getMatrixUniformLocations(p.arcProgram, "inv_transform")
		p.enableAttribArrays(p.arcProgram, "vert", "normal")

		p.pathProgram = ProgramState{
			ref:        p.createProgram("path_es"),
			buff:       p.createBuffer(64),
			uniforms:   make(map[string]*UniformState),
			attributes: make(map[string]Attribute),
		}
		p.getUniformLocations(p.pathProgram, "fill_color")
		p.enableAttribArrays(p.pathProgram, "vert", "alpha")

		p.scene3dProgram = ProgramState{
			ref:          p.createProgram("scene3d_es"),
			buff:         p.createBuffer(1024),
			uniforms:     make(map[string]*UniformState),
			mat4Uniforms: make(map[string]*Mat4UniformState),
			attributes:   make(map[string]Attribute),
		}
		p.getUniformLocations(p.scene3dProgram, "ambient", "light_dir", "light_color")
		p.getMat4UniformLocations(p.scene3dProgram, "mvp")
		p.enableAttribArrays(p.scene3dProgram, "vert", "normal", "color")

		// Gaussian-blur shadow programs (two-pass separable blur). These were
		// missing from the gomobile painter, so any object with a BoxShadow —
		// notably every overlay/popup/dialog drawn via ShadowingRenderer —
		// reached a nil uniform map in SetUniform2f and crashed during paint.
		// gl_es.go and gl_core.go create these; mirror them here.
		p.blurHProgram = ProgramState{
			ref:        p.createProgram("blur_h_es"),
			buff:       p.createBuffer(20),
			uniforms:   make(map[string]*UniformState),
			attributes: make(map[string]Attribute),
		}
		p.getUniformLocations(p.blurHProgram, "tex", "resolution", "blur_radius")
		p.enableAttribArrays(p.blurHProgram, "vert", "vertTexCoord")

		p.blurVProgram = ProgramState{
			ref:        p.createProgram("blur_v_es"),
			buff:       p.createBuffer(20),
			uniforms:   make(map[string]*UniformState),
			attributes: make(map[string]Attribute),
		}
		p.getUniformLocations(p.blurVProgram, "tex", "resolution", "blur_radius")
		p.enableAttribArrays(p.blurVProgram, "vert", "vertTexCoord")

		// Shader-based radial ripple program. Like the blur programs above this
		// was missing from the gomobile painter, so the MD3 ripple drawn on
		// every Button/IconButton/Chip tap reached a nil uniform map in
		// SetUniform2f and crashed during paint. gl_es.go creates this; mirror
		// it here.
		p.rippleProgram = ProgramState{
			ref:            p.createProgram("ripple_es"),
			buff:           p.createBuffer(16),
			uniforms:       make(map[string]*UniformState),
			matrixUniforms: make(map[string]*MatrixUniformState),
			attributes:     make(map[string]Attribute),
		}
		p.getUniformLocations(
			p.rippleProgram,
			"frame_size", "rect_coords",
			"rect_size_half", "radius", "edge_softness",
			"ripple_center", "ripple_radius", "ripple_color",
		)
		p.getMatrixUniformLocations(p.rippleProgram, "inv_transform")
		p.enableAttribArrays(p.rippleProgram, "vert", "normal")

		compiled = []ProgramState{
			p.program,
			p.lineProgram,
			p.rectangleProgram,
			p.roundRectangleProgram,
			p.polygonProgram,
			p.arcProgram,
			p.pathProgram,
			p.scene3dProgram,
			p.blurHProgram,
			p.blurVProgram,
			p.rippleProgram,
		}
	}

	p.program = compiled[0]
	p.lineProgram = compiled[1]
	p.rectangleProgram = compiled[2]
	p.roundRectangleProgram = compiled[3]
	p.polygonProgram = compiled[4]
	p.arcProgram = compiled[5]
	p.pathProgram = compiled[6]
	p.scene3dProgram = compiled[7]
	p.blurHProgram = compiled[8]
	p.blurVProgram = compiled[9]
	p.rippleProgram = compiled[10]
}

func (p *painter) getUniformLocations(pState ProgramState, names ...string) {
	for _, name := range names {
		u := p.ctx.GetUniformLocation(pState.ref, name)
		pState.uniforms[name] = &UniformState{ref: u}
	}
}

func (p *painter) getMatrixUniformLocations(pState ProgramState, names ...string) {
	for _, name := range names {
		u := p.ctx.GetUniformLocation(pState.ref, name)
		pState.matrixUniforms[name] = &MatrixUniformState{ref: u}
	}
}

func (p *painter) getMat4UniformLocations(pState ProgramState, names ...string) {
	for _, name := range names {
		u := p.ctx.GetUniformLocation(pState.ref, name)
		pState.mat4Uniforms[name] = &Mat4UniformState{ref: u}
	}
}

func (p *painter) enableAttribArrays(pState ProgramState, names ...string) {
	for _, name := range names {
		a := p.ctx.GetAttribLocation(pState.ref, name)
		p.ctx.EnableVertexAttribArray(a)
		pState.attributes[name] = a
	}
}

type mobileContext struct {
	glContext gl.Context
}

var _ context = (*mobileContext)(nil)

func (c *mobileContext) ActiveTexture(textureUnit uint32) {
	c.glContext.ActiveTexture(gl.Enum(textureUnit))
}

func (c *mobileContext) AttachShader(program Program, shader Shader) {
	c.glContext.AttachShader(gl.Program(program), gl.Shader(shader))
}

func (c *mobileContext) BindBuffer(target uint32, buf Buffer) {
	c.glContext.BindBuffer(gl.Enum(target), gl.Buffer(buf))
}

func (c *mobileContext) BindTexture(target uint32, texture Texture) {
	c.glContext.BindTexture(gl.Enum(target), gl.Texture(texture))
}

func (c *mobileContext) BlendColor(r, g, b, a float32) {
	c.glContext.BlendColor(r, g, b, a)
}

func (c *mobileContext) BlendFunc(srcFactor, destFactor uint32) {
	c.glContext.BlendFunc(gl.Enum(srcFactor), gl.Enum(destFactor))
}

func (c *mobileContext) BufferData(target uint32, points []float32, usage uint32) {
	data := toLEByteOrder(points...)
	c.glContext.BufferData(gl.Enum(target), data, gl.Enum(usage))
}

func (c *mobileContext) BufferSubData(target uint32, points []float32) {
	data := toLEByteOrder(points...)
	c.glContext.BufferSubData(gl.Enum(target), data)
}

func (c *mobileContext) Clear(mask uint32) {
	c.glContext.Clear(gl.Enum(mask))
}

func (c *mobileContext) ClearColor(r, g, b, a float32) {
	c.glContext.ClearColor(r, g, b, a)
}

func (c *mobileContext) CompileShader(shader Shader) {
	c.glContext.CompileShader(gl.Shader(shader))
}

func (c *mobileContext) CreateBuffer() Buffer {
	return Buffer(c.glContext.CreateBuffer())
}

func (c *mobileContext) CreateProgram() Program {
	return Program(c.glContext.CreateProgram())
}

func (c *mobileContext) CreateShader(typ uint32) Shader {
	return Shader(c.glContext.CreateShader(gl.Enum(typ)))
}

func (c *mobileContext) CreateTexture() (texture Texture) {
	return Texture(c.glContext.CreateTexture())
}

func (c *mobileContext) DeleteBuffer(buffer Buffer) {
	c.glContext.DeleteBuffer(gl.Buffer(buffer))
}

func (c *mobileContext) DeleteTexture(texture Texture) {
	c.glContext.DeleteTexture(gl.Texture(texture))
}

func (c *mobileContext) Disable(capability uint32) {
	c.glContext.Disable(gl.Enum(capability))
}

func (c *mobileContext) DrawArrays(mode uint32, first, count int) {
	c.glContext.DrawArrays(gl.Enum(mode), first, count)
}

func (c *mobileContext) Enable(capability uint32) {
	c.glContext.Enable(gl.Enum(capability))
}

func (c *mobileContext) EnableVertexAttribArray(attribute Attribute) {
	c.glContext.EnableVertexAttribArray(gl.Attrib(attribute))
}

func (c *mobileContext) GetAttribLocation(program Program, name string) Attribute {
	return Attribute(c.glContext.GetAttribLocation(gl.Program(program), name))
}

func (c *mobileContext) GetError() uint32 {
	return uint32(c.glContext.GetError())
}

func (c *mobileContext) GetProgrami(program Program, param uint32) int {
	return c.glContext.GetProgrami(gl.Program(program), gl.Enum(param))
}

func (c *mobileContext) GetProgramInfoLog(program Program) string {
	return c.glContext.GetProgramInfoLog(gl.Program(program))
}

func (c *mobileContext) GetShaderi(shader Shader, param uint32) int {
	return c.glContext.GetShaderi(gl.Shader(shader), gl.Enum(param))
}

func (c *mobileContext) GetShaderInfoLog(shader Shader) string {
	return c.glContext.GetShaderInfoLog(gl.Shader(shader))
}

func (c *mobileContext) GetUniformLocation(program Program, name string) Uniform {
	return Uniform(c.glContext.GetUniformLocation(gl.Program(program), name))
}

func (c *mobileContext) LinkProgram(program Program) {
	c.glContext.LinkProgram(gl.Program(program))
}

func (c *mobileContext) ReadBuffer(_ uint32) {
}

func (c *mobileContext) ReadPixels(x, y, width, height int, colorFormat, typ uint32, pixels []uint8) {
	c.glContext.ReadPixels(pixels, x, y, width, height, gl.Enum(colorFormat), gl.Enum(typ))
}

func (c *mobileContext) Scissor(x, y, w, h int32) {
	c.glContext.Scissor(x, y, w, h)
}

func (c *mobileContext) ShaderSource(shader Shader, source string) {
	c.glContext.ShaderSource(gl.Shader(shader), source)
}

func (c *mobileContext) TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8) {
	c.glContext.TexImage2D(
		gl.Enum(target),
		level,
		int(colorFormat),
		width,
		height,
		gl.Enum(colorFormat),
		gl.Enum(typ),
		data,
	)
}

func (c *mobileContext) TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8) {
	c.glContext.TexSubImage2D(
		gl.Enum(target),
		level,
		x,
		y,
		width,
		height,
		gl.Enum(colorFormat),
		gl.Enum(typ),
		data,
	)
}

func (c *mobileContext) TexParameteri(target, param uint32, value int32) {
	c.glContext.TexParameteri(gl.Enum(target), gl.Enum(param), int(value))
}

func (c *mobileContext) Uniform1f(uniform Uniform, v float32) {
	c.glContext.Uniform1f(gl.Uniform(uniform), v)
}

func (c *mobileContext) Uniform2f(uniform Uniform, v0, v1 float32) {
	c.glContext.Uniform2f(gl.Uniform(uniform), v0, v1)
}

func (c *mobileContext) Uniform3f(uniform Uniform, v0, v1, v2 float32) {
	c.glContext.Uniform3f(gl.Uniform(uniform), v0, v1, v2)
}

func (c *mobileContext) Uniform4f(uniform Uniform, v0, v1, v2, v3 float32) {
	c.glContext.Uniform4f(gl.Uniform(uniform), v0, v1, v2, v3)
}

func (c *mobileContext) UniformMatrix3fv(uniform Uniform, value [9]float32) {
	c.glContext.UniformMatrix3fv(gl.Uniform(uniform), value[:])
}

func (c *mobileContext) UniformMatrix4fv(uniform Uniform, value [16]float32) {
	c.glContext.UniformMatrix4fv(gl.Uniform(uniform), value[:])
}

func (c *mobileContext) UseProgram(program Program) {
	c.glContext.UseProgram(gl.Program(program))
}

func (c *mobileContext) VertexAttribPointerWithOffset(attribute Attribute, size int, typ uint32, normalized bool, stride, offset int) {
	c.glContext.VertexAttribPointer(gl.Attrib(attribute), size, gl.Enum(typ), normalized, stride, offset)
}

func (c *mobileContext) Viewport(x, y, width, height int) {
	c.glContext.Viewport(x, y, width, height)
}

// --- Framebuffer methods (mobile GL context) ---

func (c *mobileContext) CreateFramebuffer() Framebuffer {
	return Framebuffer(c.glContext.CreateFramebuffer().Value)
}

func (c *mobileContext) BindFramebuffer(target uint32, fb Framebuffer) {
	c.glContext.BindFramebuffer(gl.Enum(target), gl.Framebuffer{Value: uint32(fb)})
}

func (c *mobileContext) FramebufferTexture2D(target, attachment uint32, texture Texture, level int32) {
	// Texture is an alias of gl.Texture on mobile builds.
	c.glContext.FramebufferTexture2D(gl.Enum(target), gl.Enum(attachment), gl.Enum(texture2D), gl.Texture(texture), int(level))
}

func (c *mobileContext) CheckFramebufferStatus(target uint32) uint32 {
	return uint32(c.glContext.CheckFramebufferStatus(gl.Enum(target)))
}

func (c *mobileContext) DeleteFramebuffer(fb Framebuffer) {
	c.glContext.DeleteFramebuffer(gl.Framebuffer{Value: uint32(fb)})
}

func (c *mobileContext) Flush() {
	c.glContext.Flush()
}

func (c *mobileContext) CopyTexSubImage2D(target uint32, level, xoffset, yoffset, x, y, width, height int) {
	c.glContext.CopyTexSubImage2D(gl.Enum(target), level, xoffset, yoffset, x, y, width, height)
}

// --- Color mask ---

func (c *mobileContext) ColorMask(r, g, b, a bool) {
	c.glContext.ColorMask(r, g, b, a)
}

// --- Stencil methods ---

func (c *mobileContext) ClearStencil(s int32) {
	c.glContext.ClearStencil(int(s))
}

func (c *mobileContext) StencilFunc(fn uint32, ref int32, mask uint32) {
	c.glContext.StencilFunc(gl.Enum(fn), int(ref), uint32(mask))
}

func (c *mobileContext) StencilOp(sfail, dpfail, dppass uint32) {
	c.glContext.StencilOp(gl.Enum(sfail), gl.Enum(dpfail), gl.Enum(dppass))
}

func (c *mobileContext) StencilOpSeparate(face, sfail, dpfail, dppass uint32) {
	c.glContext.StencilOpSeparate(gl.Enum(face), gl.Enum(sfail), gl.Enum(dpfail), gl.Enum(dppass))
}

func (c *mobileContext) StencilMask(mask uint32) {
	c.glContext.StencilMask(uint32(mask))
}

// toLEByteOrder returns the byte representation of float32 values in little endian byte order.
func toLEByteOrder(values ...float32) []byte {
	b := make([]byte, 4*len(values))
	for i, v := range values {
		u := math.Float32bits(v)
		b[4*i+0] = byte(u >> 0)
		b[4*i+1] = byte(u >> 8)
		b[4*i+2] = byte(u >> 16)
		b[4*i+3] = byte(u >> 24)
	}
	return b
}
