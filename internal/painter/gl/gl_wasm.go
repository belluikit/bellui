//go:build (gl || (!darwin && !linux && !windows && !wasm)) && wasm

package gl

import (
	"math"

	"github.com/belluikit/gl-js"
)

const (
	arrayBuffer           = gl.ARRAY_BUFFER
	bitColorBuffer        = gl.COLOR_BUFFER_BIT
	bitDepthBuffer        = gl.DEPTH_BUFFER_BIT
	clampToEdge           = gl.CLAMP_TO_EDGE
	colorFormatRGBA       = gl.RGBA
	compileStatus         = gl.COMPILE_STATUS
	constantAlpha         = gl.CONSTANT_ALPHA
	float                 = gl.FLOAT
	fragmentShader        = gl.FRAGMENT_SHADER
	back                  = gl.BACK
	front                 = gl.FRONT
	glFalse               = gl.FALSE
	linkStatus            = gl.LINK_STATUS
	one                   = gl.ONE
	oneMinusConstantAlpha = gl.ONE_MINUS_CONSTANT_ALPHA
	oneMinusSrcAlpha      = gl.ONE_MINUS_SRC_ALPHA
	scissorTest           = gl.SCISSOR_TEST
	srcAlpha              = gl.SRC_ALPHA
	staticDraw            = gl.STATIC_DRAW
	texture0              = gl.TEXTURE0
	texture2D             = gl.TEXTURE_2D
	textureMinFilter      = gl.TEXTURE_MIN_FILTER
	textureMagFilter      = gl.TEXTURE_MAG_FILTER
	textureWrapS          = gl.TEXTURE_WRAP_S
	textureWrapT          = gl.TEXTURE_WRAP_T
	triangles             = gl.TRIANGLES
	triangleStrip         = gl.TRIANGLE_STRIP
	unsignedByte          = gl.UNSIGNED_BYTE
	vertexShader          = gl.VERTEX_SHADER

	// FBO constants
	glFramebuffer       = gl.FRAMEBUFFER
	colorAttachment0    = gl.COLOR_ATTACHMENT0
	framebufferComplete = gl.FRAMEBUFFER_COMPLETE

	// Depth constants
	depthTest       = gl.DEPTH_TEST
	depthAttachment = gl.DEPTH_ATTACHMENT
	depthComponent  = gl.DEPTH_COMPONENT
	unsignedShort   = gl.UNSIGNED_SHORT

	// Stencil constants
	stencilTest      = gl.STENCIL_TEST
	bitStencilBuffer = gl.STENCIL_BUFFER_BIT
	glAlways         = gl.ALWAYS
	glEqual          = gl.EQUAL
	glNotEqual       = gl.NOTEQUAL
	glKeep           = gl.KEEP
	glReplace        = gl.REPLACE
	glIncrWrap       = gl.INCR_WRAP
	glDecrWrap       = gl.DECR_WRAP
	glInvert         = gl.INVERT
	glFront          = gl.FRONT
	glBack           = gl.BACK
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
	noBuffer          = Buffer(gl.NoBuffer)
	noShader          = Shader(gl.NoShader)
	textureFilterToGL = [...]int32{gl.LINEAR, gl.NEAREST, gl.LINEAR}
)

func (p *painter) Init() {
	p.ctx = &xjsContext{
		enabledAttribs: map[Attribute]bool{},
		fedAttribs:     map[Attribute]bool{},
	}
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	p.logError()
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

	p.pathProgram = ProgramState{
		ref:        p.createProgram("path_es"),
		buff:       p.createBuffer(64),
		uniforms:   make(map[string]*UniformState),
		attributes: make(map[string]Attribute),
	}
	p.getUniformLocations(p.pathProgram, "fill_color")
	p.enableAttribArrays(p.pathProgram, "vert", "alpha")

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

// xjsContext drives the WebGL backend. Unlike desktop GL, WebGL validates
// vertex-attribute state on every draw: if any *enabled* attribute array has no
// buffer bound, the draw fails with INVALID_OPERATION ("no buffer is bound to
// enabled attribute"). The painter enables an attribute array per program but
// only binds a buffer (VertexAttribPointer) for the attributes the current
// program supplies, so after switching from a program with more attributes a
// later draw leaves an enabled attribute unfed. We track which attribute
// locations are enabled and which were fed since the last UseProgram, and
// disable any enabled-but-unfed array right before each draw — giving WebGL the
// per-draw vertex-attribute state it requires. See pre-rebrand tracker #102.
type xjsContext struct {
	enabledAttribs map[Attribute]bool // attribute arrays currently enabled in GL
	fedAttribs     map[Attribute]bool // attributes given a buffer since the last UseProgram
}

var _ context = (*xjsContext)(nil)

func (c *xjsContext) ActiveTexture(textureUnit uint32) {
	gl.ActiveTexture(gl.Enum(textureUnit))
}

func (c *xjsContext) AttachShader(program Program, shader Shader) {
	gl.AttachShader(gl.Program(program), gl.Shader(shader))
}

func (c *xjsContext) BindBuffer(target uint32, buf Buffer) {
	gl.BindBuffer(gl.Enum(target), gl.Buffer(buf))
}

func (c *xjsContext) BindTexture(target uint32, texture Texture) {
	gl.BindTexture(gl.Enum(target), gl.Texture(texture))
}

func (c *xjsContext) BlendColor(r, g, b, a float32) {
	gl.BlendColor(r, g, b, a)
}

func (c *xjsContext) BlendFunc(srcFactor, destFactor uint32) {
	gl.BlendFunc(gl.Enum(srcFactor), gl.Enum(destFactor))
}

func (c *xjsContext) BlendFuncSeparate(srcRGB, destRGB, srcAlpha, destAlpha uint32) {
	gl.BlendFuncSeparate(gl.Enum(srcRGB), gl.Enum(destRGB), gl.Enum(srcAlpha), gl.Enum(destAlpha))
}

func (c *xjsContext) BufferData(target uint32, points []float32, usage uint32) {
	gl.BufferData(gl.Enum(target), toLEByteOrder(points...), gl.Enum(usage))
}

func (c *xjsContext) BufferSubData(target uint32, points []float32) {
	data := toLEByteOrder(points...)
	gl.BufferSubData(gl.Enum(target), 0, data)
}

func (c *xjsContext) Clear(mask uint32) {
	gl.Clear(gl.Enum(mask))
}

func (c *xjsContext) ClearColor(r, g, b, a float32) {
	gl.ClearColor(r, g, b, a)
}

func (c *xjsContext) CompileShader(shader Shader) {
	gl.CompileShader(gl.Shader(shader))
}

func (c *xjsContext) CreateBuffer() Buffer {
	return Buffer(gl.CreateBuffer())
}

func (c *xjsContext) CreateProgram() Program {
	return Program(gl.CreateProgram())
}

func (c *xjsContext) CreateShader(typ uint32) Shader {
	return Shader(gl.CreateShader(gl.Enum(typ)))
}

func (c *xjsContext) CreateTexture() (texture Texture) {
	return Texture(gl.CreateTexture())
}

func (c *xjsContext) DeleteBuffer(buffer Buffer) {
	gl.DeleteBuffer(gl.Buffer(buffer))
}

func (c *xjsContext) DeleteTexture(texture Texture) {
	gl.DeleteTexture(gl.Texture(texture))
}

func (c *xjsContext) Disable(capability uint32) {
	gl.Disable(gl.Enum(capability))
}

func (c *xjsContext) DrawArrays(mode uint32, first, count int) {
	// Disable any attribute array enabled but not fed a buffer by the current
	// program, so WebGL doesn't reject the draw (see xjsContext doc comment).
	for a := range c.enabledAttribs {
		if !c.fedAttribs[a] {
			gl.DisableVertexAttribArray(gl.Attrib(a))
			delete(c.enabledAttribs, a)
		}
	}
	gl.DrawArrays(gl.Enum(mode), first, count)
}

func (c *xjsContext) Enable(capability uint32) {
	gl.Enable(gl.Enum(capability))
}

func (c *xjsContext) EnableVertexAttribArray(attribute Attribute) {
	gl.EnableVertexAttribArray(gl.Attrib(attribute))
	c.enabledAttribs[attribute] = true
}

func (c *xjsContext) GetAttribLocation(program Program, name string) Attribute {
	return Attribute(gl.GetAttribLocation(gl.Program(program), name))
}

func (c *xjsContext) GetError() uint32 {
	return uint32(gl.GetError())
}

func (c *xjsContext) GetProgrami(program Program, param uint32) int {
	return gl.GetProgrami(gl.Program(program), gl.Enum(param))
}

func (c *xjsContext) GetProgramInfoLog(program Program) string {
	return gl.GetProgramInfoLog(gl.Program(program))
}

func (c *xjsContext) GetShaderi(shader Shader, param uint32) int {
	return gl.GetShaderi(gl.Shader(shader), gl.Enum(param))
}

func (c *xjsContext) GetShaderInfoLog(shader Shader) string {
	return gl.GetShaderInfoLog(gl.Shader(shader))
}

func (c *xjsContext) GetUniformLocation(program Program, name string) Uniform {
	return Uniform(gl.GetUniformLocation(gl.Program(program), name))
}

func (c *xjsContext) LinkProgram(program Program) {
	gl.LinkProgram(gl.Program(program))
}

func (c *xjsContext) ReadBuffer(_ uint32) {
}

func (c *xjsContext) ReadPixels(x, y, width, height int, colorFormat, typ uint32, pixels []uint8) {
	gl.ReadPixels(pixels, x, y, width, height, gl.Enum(colorFormat), gl.Enum(typ))
}

func (c *xjsContext) ShaderSource(shader Shader, source string) {
	gl.ShaderSource(gl.Shader(shader), source)
}

func (c *xjsContext) Scissor(x, y, w, h int32) {
	gl.Scissor(x, y, w, h)
}

func (c *xjsContext) TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8) {
	gl.TexImage2D(
		gl.Enum(target),
		level,
		width,
		height,
		gl.Enum(colorFormat),
		gl.Enum(typ),
		data,
	)
}

func (c *xjsContext) TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8) {
	gl.TexSubImage2D(
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

func (c *xjsContext) TexParameteri(target, param uint32, value int32) {
	gl.TexParameteri(gl.Enum(target), gl.Enum(param), int(value))
}

func (c *xjsContext) Uniform1f(uniform Uniform, v float32) {
	gl.Uniform1f(gl.Uniform(uniform), v)
}

func (c *xjsContext) Uniform2f(uniform Uniform, v0, v1 float32) {
	gl.Uniform2f(gl.Uniform(uniform), v0, v1)
}

func (c *xjsContext) Uniform3f(uniform Uniform, v0, v1, v2 float32) {
	gl.Uniform3f(gl.Uniform(uniform), v0, v1, v2)
}

func (c *xjsContext) Uniform4f(uniform Uniform, v0, v1, v2, v3 float32) {
	gl.Uniform4f(gl.Uniform(uniform), v0, v1, v2, v3)
}

func (c *xjsContext) UniformMatrix3fv(uniform Uniform, value [9]float32) {
	gl.UniformMatrix3fv(gl.Uniform(uniform), value[:])
}

func (c *xjsContext) UniformMatrix4fv(uniform Uniform, value [16]float32) {
	gl.UniformMatrix4fv(gl.Uniform(uniform), value[:])
}

func (c *xjsContext) UseProgram(program Program) {
	gl.UseProgram(gl.Program(program))
	// A new program supplies its own attribute set; forget the previous one's
	// fed attributes so the next draw reconciles against this program's.
	clear(c.fedAttribs)
}

func (c *xjsContext) VertexAttribPointerWithOffset(attribute Attribute, size int, typ uint32, normalized bool, stride, offset int) {
	// Binding a buffer to the attribute re-enables it if a prior draw disabled
	// it, so a program's attributes are always live for its own draws.
	if !c.enabledAttribs[attribute] {
		gl.EnableVertexAttribArray(gl.Attrib(attribute))
		c.enabledAttribs[attribute] = true
	}
	gl.VertexAttribPointer(gl.Attrib(attribute), size, gl.Enum(typ), normalized, stride, offset)
	c.fedAttribs[attribute] = true
}

func (c *xjsContext) Viewport(x, y, width, height int) {
	gl.Viewport(x, y, width, height)
}

// --- Framebuffer objects ---
// The wasm Framebuffer type wraps gl.Framebuffer (a js.Value handle), so these
// drive the real WebGL framebuffer API — enabling off-screen rendering for
// drop shadows, backdrop blur, RepaintBoundary caching and dirty-region culling
// on the web target. Bind defaultFramebuffer (the null framebuffer) to return
// to the canvas back buffer.

func (c *xjsContext) CreateFramebuffer() Framebuffer {
	return Framebuffer(gl.CreateFramebuffer())
}

func (c *xjsContext) BindFramebuffer(target uint32, fb Framebuffer) {
	gl.BindFramebuffer(gl.Enum(target), gl.Framebuffer(fb))
}

func (c *xjsContext) FramebufferTexture2D(target, attachment uint32, texture Texture, level int32) {
	gl.FramebufferTexture2D(gl.Enum(target), gl.Enum(attachment), gl.TEXTURE_2D, gl.Texture(texture), int(level))
}

func (c *xjsContext) CheckFramebufferStatus(target uint32) uint32 {
	return uint32(gl.CheckFramebufferStatus(gl.Enum(target)))
}

func (c *xjsContext) DeleteFramebuffer(fb Framebuffer) {
	gl.DeleteFramebuffer(gl.Framebuffer(fb))
}

// --- Methods with working wasm implementations ---

func (c *xjsContext) Flush()                    { gl.Flush() }
func (c *xjsContext) ColorMask(r, g, b, a bool) { gl.ColorMask(r, g, b, a) }

func (c *xjsContext) CopyTexSubImage2D(target uint32, level, xoffset, yoffset, x, y, width, height int) {
	gl.CopyTexSubImage2D(gl.Enum(target), level, xoffset, yoffset, x, y, width, height)
}

func (c *xjsContext) ClearStencil(s int32) { gl.ClearStencil(int(s)) }
func (c *xjsContext) StencilFunc(fn uint32, ref int32, mask uint32) {
	gl.StencilFunc(gl.Enum(fn), int(ref), mask)
}

func (c *xjsContext) StencilOp(sfail, dpfail, dppass uint32) {
	gl.StencilOp(gl.Enum(sfail), gl.Enum(dpfail), gl.Enum(dppass))
}

func (c *xjsContext) StencilOpSeparate(face, sfail, dpfail, dppass uint32) {
	gl.StencilOpSeparate(gl.Enum(face), gl.Enum(sfail), gl.Enum(dpfail), gl.Enum(dppass))
}
func (c *xjsContext) StencilMask(mask uint32) { gl.StencilMask(mask) }

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
