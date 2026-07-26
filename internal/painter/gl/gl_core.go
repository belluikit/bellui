//go:build (gl || (!darwin && !linux && !windows && !wasm)) && ((!gles && !arm && !arm64 && !android && !ios && !mobile && !wasm) || (darwin && !mobile && !ios && !wasm))

package gl

import (
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v2.1/gl"

	"github.com/belluikit/bellui"
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

const (
	noBuffer = Buffer(0)
	noShader = Shader(0)
)

type (
	// Attribute represents a GL attribute
	Attribute int32
	// Buffer represents a GL buffer
	Buffer uint32
	// Program represents a compiled GL program
	Program uint32
	// Shader represents a GL shader
	Shader uint32
	// Uniform represents a GL uniform
	Uniform int32
)

var textureFilterToGL = [...]int32{gl.LINEAR, gl.NEAREST, gl.LINEAR}

func (p *painter) Init() {
	p.ctx = &coreContext{}
	err := gl.Init()
	if err != nil {
		bellui.LogError("failed to initialise OpenGL", err)
		return
	}

	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	p.logError()
	p.program = ProgramState{
		ref:        p.createProgram("simple"),
		buff:       p.createBuffer(20),
		uniforms:   make(map[string]*UniformState),
		attributes: make(map[string]Attribute),
	}
	p.getUniformLocations(p.program, "text", "alpha", "cornerRadius", "size", "inset")
	p.enableAttribArrays(p.program, "vert", "vertTexCoord")

	p.lineProgram = ProgramState{
		ref:        p.createProgram("line"),
		buff:       p.createBuffer(24),
		uniforms:   make(map[string]*UniformState),
		attributes: make(map[string]Attribute),
	}
	p.getUniformLocations(p.lineProgram, "feather", "color", "lineWidth")
	p.enableAttribArrays(p.lineProgram, "vert", "normal")

	p.rectangleProgram = ProgramState{
		ref:            p.createProgram("rectangle"),
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
		ref:            p.createProgram("round_rectangle"),
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
		ref:            p.createProgram("polygon"),
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
		ref:            p.createProgram("arc"),
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
		ref:            p.createProgram("ripple"),
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

	p.blurHProgram = ProgramState{
		ref:        p.createProgram("blur_h"),
		buff:       p.createBuffer(20),
		uniforms:   make(map[string]*UniformState),
		attributes: make(map[string]Attribute),
	}
	p.getUniformLocations(p.blurHProgram, "tex", "resolution", "blur_radius")
	p.enableAttribArrays(p.blurHProgram, "vert", "vertTexCoord")

	p.blurVProgram = ProgramState{
		ref:        p.createProgram("blur_v"),
		buff:       p.createBuffer(20),
		uniforms:   make(map[string]*UniformState),
		attributes: make(map[string]Attribute),
	}
	p.getUniformLocations(p.blurVProgram, "tex", "resolution", "blur_radius")
	p.enableAttribArrays(p.blurVProgram, "vert", "vertTexCoord")

	p.pathProgram = ProgramState{
		ref:        p.createProgram("path"),
		buff:       p.createBuffer(64),
		uniforms:   make(map[string]*UniformState),
		attributes: make(map[string]Attribute),
	}
	p.getUniformLocations(p.pathProgram, "fill_color")
	p.enableAttribArrays(p.pathProgram, "vert", "alpha")

	p.scene3dProgram = ProgramState{
		ref:          p.createProgram("scene3d"),
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

type coreContext struct{}

var _ context = (*coreContext)(nil)

func (c *coreContext) ActiveTexture(textureUnit uint32) {
	gl.ActiveTexture(textureUnit)
}

func (c *coreContext) AttachShader(program Program, shader Shader) {
	gl.AttachShader(uint32(program), uint32(shader))
}

func (c *coreContext) BindBuffer(target uint32, buf Buffer) {
	gl.BindBuffer(target, uint32(buf))
}

func (c *coreContext) BindTexture(target uint32, texture Texture) {
	gl.BindTexture(target, uint32(texture))
}

func (c *coreContext) BlendColor(r, g, b, a float32) {
	gl.BlendColor(r, g, b, a)
}

func (c *coreContext) BlendFunc(srcFactor, destFactor uint32) {
	gl.BlendFunc(srcFactor, destFactor)
}

func (c *coreContext) BufferData(target uint32, points []float32, usage uint32) {
	gl.BufferData(target, 4*len(points), gl.Ptr(points), usage)
}

func (c *coreContext) BufferSubData(target uint32, points []float32) {
	gl.BufferSubData(target, 0, 4*len(points), gl.Ptr(points))
}

func (c *coreContext) Clear(mask uint32) {
	gl.Clear(mask)
}

func (c *coreContext) ClearColor(r, g, b, a float32) {
	gl.ClearColor(r, g, b, a)
}

func (c *coreContext) CompileShader(shader Shader) {
	gl.CompileShader(uint32(shader))
}

func (c *coreContext) CreateBuffer() Buffer {
	var vbo uint32
	gl.GenBuffers(1, &vbo)
	return Buffer(vbo)
}

func (c *coreContext) CreateProgram() Program {
	return Program(gl.CreateProgram())
}

func (c *coreContext) CreateShader(typ uint32) Shader {
	return Shader(gl.CreateShader(typ))
}

func (c *coreContext) CreateTexture() (texture Texture) {
	var tex uint32
	gl.GenTextures(1, &tex)
	return Texture(tex)
}

func (c *coreContext) DeleteBuffer(buffer Buffer) {
	gl.DeleteBuffers(1, (*uint32)(&buffer))
}

func (c *coreContext) DeleteTexture(texture Texture) {
	tex := uint32(texture)
	gl.DeleteTextures(1, &tex)
}

func (c *coreContext) Disable(capability uint32) {
	gl.Disable(capability)
}

func (c *coreContext) DrawArrays(mode uint32, first, count int) {
	gl.DrawArrays(mode, int32(first), int32(count))
}

func (c *coreContext) Enable(capability uint32) {
	gl.Enable(capability)
}

func (c *coreContext) EnableVertexAttribArray(attribute Attribute) {
	gl.EnableVertexAttribArray(uint32(attribute))
}

func (c *coreContext) GetAttribLocation(program Program, name string) Attribute {
	return Attribute(gl.GetAttribLocation(uint32(program), gl.Str(name+"\x00")))
}

func (c *coreContext) GetError() uint32 {
	return gl.GetError()
}

func (c *coreContext) GetProgrami(program Program, param uint32) int {
	var value int32
	gl.GetProgramiv(uint32(program), param, &value)
	return int(value)
}

func (c *coreContext) GetProgramInfoLog(program Program) string {
	var logLength int32
	gl.GetProgramiv(uint32(program), gl.INFO_LOG_LENGTH, &logLength)
	info := strings.Repeat("\x00", int(logLength+1))
	gl.GetProgramInfoLog(uint32(program), logLength, nil, gl.Str(info))
	return info
}

func (c *coreContext) GetShaderi(shader Shader, param uint32) int {
	var value int32
	gl.GetShaderiv(uint32(shader), param, &value)
	return int(value)
}

func (c *coreContext) GetShaderInfoLog(shader Shader) string {
	var logLength int32
	gl.GetShaderiv(uint32(shader), gl.INFO_LOG_LENGTH, &logLength)
	info := strings.Repeat("\x00", int(logLength+1))
	gl.GetShaderInfoLog(uint32(shader), logLength, nil, gl.Str(info))
	return info
}

func (c *coreContext) GetUniformLocation(program Program, name string) Uniform {
	return Uniform(gl.GetUniformLocation(uint32(program), gl.Str(name+"\x00")))
}

func (c *coreContext) LinkProgram(program Program) {
	gl.LinkProgram(uint32(program))
}

func (c *coreContext) ReadBuffer(src uint32) {
	gl.ReadBuffer(src)
}

func (c *coreContext) ReadPixels(x, y, width, height int, colorFormat, typ uint32, pixels []uint8) {
	gl.ReadPixels(int32(x), int32(y), int32(width), int32(height), colorFormat, typ, gl.Ptr(pixels))
}

func (c *coreContext) Scissor(x, y, w, h int32) {
	gl.Scissor(x, y, w, h)
}

func (c *coreContext) ShaderSource(shader Shader, source string) {
	csources, free := gl.Strs(source + "\x00")
	defer free()
	gl.ShaderSource(uint32(shader), 1, csources, nil)
}

func (c *coreContext) TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8) {
	var ptr unsafe.Pointer
	if len(data) > 0 {
		ptr = gl.Ptr(data)
	}
	gl.TexImage2D(
		target,
		int32(level),
		int32(colorFormat),
		int32(width),
		int32(height),
		0,
		colorFormat,
		typ,
		ptr,
	)
}

func (c *coreContext) TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8) {
	gl.TexSubImage2D(
		target,
		int32(level),
		int32(x),
		int32(y),
		int32(width),
		int32(height),
		colorFormat,
		typ,
		gl.Ptr(data),
	)
}

func (c *coreContext) TexParameteri(target, param uint32, value int32) {
	gl.TexParameteri(target, param, value)
}

func (c *coreContext) Uniform1f(uniform Uniform, v float32) {
	gl.Uniform1f(int32(uniform), v)
}

func (c *coreContext) Uniform2f(uniform Uniform, v0, v1 float32) {
	gl.Uniform2f(int32(uniform), v0, v1)
}

func (c *coreContext) Uniform3f(uniform Uniform, v0, v1, v2 float32) {
	gl.Uniform3f(int32(uniform), v0, v1, v2)
}

func (c *coreContext) Uniform4f(uniform Uniform, v0, v1, v2, v3 float32) {
	gl.Uniform4f(int32(uniform), v0, v1, v2, v3)
}

func (c *coreContext) UniformMatrix3fv(uniform Uniform, value [9]float32) {
	gl.UniformMatrix3fv(int32(uniform), 1, false, &value[0])
}

func (c *coreContext) UniformMatrix4fv(uniform Uniform, value [16]float32) {
	gl.UniformMatrix4fv(int32(uniform), 1, false, &value[0])
}

func (c *coreContext) UseProgram(program Program) {
	gl.UseProgram(uint32(program))
}

func (c *coreContext) VertexAttribPointerWithOffset(attribute Attribute, size int, typ uint32, normalized bool, stride, offset int) {
	gl.VertexAttribPointerWithOffset(uint32(attribute), int32(size), typ, normalized, int32(stride), uintptr(offset))
}

func (c *coreContext) Viewport(x, y, width, height int) {
	gl.Viewport(int32(x), int32(y), int32(width), int32(height))
}

// --- Framebuffer methods ---

func (c *coreContext) CreateFramebuffer() Framebuffer {
	var fbo uint32
	gl.GenFramebuffers(1, &fbo)
	return Framebuffer(fbo)
}

func (c *coreContext) BindFramebuffer(target uint32, fb Framebuffer) {
	gl.BindFramebuffer(target, uint32(fb))
}

func (c *coreContext) FramebufferTexture2D(target, attachment uint32, texture Texture, level int32) {
	gl.FramebufferTexture2D(target, attachment, gl.TEXTURE_2D, uint32(texture), level)
}

func (c *coreContext) CheckFramebufferStatus(target uint32) uint32 {
	return gl.CheckFramebufferStatus(target)
}

func (c *coreContext) DeleteFramebuffer(fb Framebuffer) {
	f := uint32(fb)
	gl.DeleteFramebuffers(1, &f)
}

func (c *coreContext) Flush() {
	gl.Flush()
}

func (c *coreContext) CopyTexSubImage2D(target uint32, level, xoffset, yoffset, x, y, width, height int) {
	gl.CopyTexSubImage2D(target, int32(level), int32(xoffset), int32(yoffset), int32(x), int32(y), int32(width), int32(height))
}

func (c *coreContext) ColorMask(r, g, b, a bool) {
	gl.ColorMask(r, g, b, a)
}

// --- Stencil methods ---

func (c *coreContext) ClearStencil(s int32) {
	gl.ClearStencil(s)
}

func (c *coreContext) StencilFunc(fn uint32, ref int32, mask uint32) {
	gl.StencilFunc(fn, ref, mask)
}

func (c *coreContext) StencilOp(sfail, dpfail, dppass uint32) {
	gl.StencilOp(sfail, dpfail, dppass)
}

func (c *coreContext) StencilOpSeparate(face, sfail, dpfail, dppass uint32) {
	gl.StencilOpSeparate(face, sfail, dpfail, dppass)
}

func (c *coreContext) StencilMask(mask uint32) {
	gl.StencilMask(mask)
}
