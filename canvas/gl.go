package canvas

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// GL handle types — thin wrappers over the underlying integer types used by OpenGL.
type (
	GLTexture     uint32
	GLBuffer      uint32
	GLProgram     uint32
	GLShader      uint32
	GLFramebuffer uint32
	GLAttribute   int32
	GLUniform     int32
)

// Standard OpenGL constants. These match the OpenGL/ES specification values
// and can be passed directly to GLContext methods.
const (
	// Texture targets
	GL_TEXTURE_2D = 0x0DE1
	GL_TEXTURE0   = 0x84C0

	// Pixel formats
	GL_RGBA          = 0x1908
	GL_RGB           = 0x1907
	GL_UNSIGNED_BYTE = 0x1401

	// Texture parameters
	GL_TEXTURE_MIN_FILTER = 0x2801
	GL_TEXTURE_MAG_FILTER = 0x2800
	GL_TEXTURE_WRAP_S     = 0x2802
	GL_TEXTURE_WRAP_T     = 0x2803
	GL_LINEAR             = 0x2601
	GL_NEAREST            = 0x2600
	GL_CLAMP_TO_EDGE      = 0x812F
	GL_REPEAT             = 0x2901

	// Buffer targets
	GL_ARRAY_BUFFER         = 0x8892
	GL_ELEMENT_ARRAY_BUFFER = 0x8893

	// Buffer usage
	GL_STATIC_DRAW  = 0x88E4
	GL_DYNAMIC_DRAW = 0x88E8

	// Data types
	GL_FLOAT          = 0x1406
	GL_UNSIGNED_SHORT = 0x1403

	// Shader types
	GL_VERTEX_SHADER   = 0x8B31
	GL_FRAGMENT_SHADER = 0x8B30
	GL_COMPILE_STATUS  = 0x8B81
	GL_LINK_STATUS     = 0x8B82

	// Draw modes
	GL_POINTS         = 0x0000
	GL_LINES          = 0x0001
	GL_LINE_STRIP     = 0x0003
	GL_TRIANGLES      = 0x0004
	GL_TRIANGLE_STRIP = 0x0005
	GL_TRIANGLE_FAN   = 0x0006

	// Blend factors
	GL_ZERO                = 0
	GL_ONE                 = 1
	GL_SRC_ALPHA           = 0x0302
	GL_ONE_MINUS_SRC_ALPHA = 0x0303

	// Clear bits
	GL_COLOR_BUFFER_BIT   = 0x00004000
	GL_DEPTH_BUFFER_BIT   = 0x00000100
	GL_STENCIL_BUFFER_BIT = 0x00000400

	// Capabilities
	GL_BLEND        = 0x0BE2
	GL_DEPTH_TEST   = 0x0B71
	GL_SCISSOR_TEST = 0x0C11
	GL_STENCIL_TEST = 0x0B90

	// Depth
	GL_DEPTH_ATTACHMENT = 0x8D00
	GL_DEPTH_COMPONENT  = 0x1902

	// Framebuffer
	GL_FRAMEBUFFER          = 0x8D40
	GL_COLOR_ATTACHMENT0    = 0x8CE0
	GL_FRAMEBUFFER_COMPLETE = 0x8CD5

	// Stencil ops
	GL_KEEP    = 0x1E00
	GL_REPLACE = 0x1E01
	GL_ALWAYS  = 0x0207
	GL_EQUAL   = 0x0202

	// Boolean
	GL_TRUE  = 1
	GL_FALSE = 0
)

// GLContext provides access to OpenGL operations during a GLView render callback.
// It is only valid for the duration of the callback — do not store references.
type GLContext interface {
	// Texture operations
	CreateTexture() GLTexture
	DeleteTexture(texture GLTexture)
	ActiveTexture(textureUnit uint32)
	BindTexture(target uint32, texture GLTexture)
	TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8)
	TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8)
	TexParameteri(target, param uint32, value int32)

	// Buffer operations
	CreateBuffer() GLBuffer
	DeleteBuffer(buffer GLBuffer)
	BindBuffer(target uint32, buf GLBuffer)
	BufferData(target uint32, points []float32, usage uint32)
	BufferSubData(target uint32, points []float32)

	// Shader operations
	CreateShader(typ uint32) GLShader
	CompileShader(shader GLShader)
	ShaderSource(shader GLShader, source string)
	GetShaderi(shader GLShader, param uint32) int
	GetShaderInfoLog(shader GLShader) string

	// Program operations
	CreateProgram() GLProgram
	AttachShader(program GLProgram, shader GLShader)
	LinkProgram(program GLProgram)
	GetProgrami(program GLProgram, param uint32) int
	GetProgramInfoLog(program GLProgram) string
	UseProgram(program GLProgram)

	// Vertex attributes
	GetAttribLocation(program GLProgram, name string) GLAttribute
	EnableVertexAttribArray(attribute GLAttribute)
	VertexAttribPointerWithOffset(attribute GLAttribute, size int, typ uint32, normalized bool, stride, offset int)

	// Uniforms
	GetUniformLocation(program GLProgram, name string) GLUniform
	Uniform1f(uniform GLUniform, v float32)
	Uniform2f(uniform GLUniform, v0, v1 float32)
	Uniform3f(uniform GLUniform, v0, v1, v2 float32)
	Uniform4f(uniform GLUniform, v0, v1, v2, v3 float32)
	UniformMatrix3fv(uniform GLUniform, value [9]float32)
	UniformMatrix4fv(uniform GLUniform, value [16]float32)

	// Drawing
	DrawArrays(mode uint32, first, count int)
	Viewport(x, y, width, height int)
	Clear(mask uint32)
	ClearColor(r, g, b, a float32)

	// Blending
	BlendFunc(srcFactor, destFactor uint32)
	BlendColor(r, g, b, a float32)

	// Clipping
	Scissor(x, y, w, h int32)
	Enable(capability uint32)
	Disable(capability uint32)

	// Framebuffer objects
	CreateFramebuffer() GLFramebuffer
	DeleteFramebuffer(fb GLFramebuffer)
	BindFramebuffer(target uint32, fb GLFramebuffer)
	FramebufferTexture2D(target, attachment uint32, texture GLTexture, level int32)
	CheckFramebufferStatus(target uint32) uint32

	// Stencil
	ClearStencil(s int32)
	StencilFunc(fn uint32, ref int32, mask uint32)
	StencilOp(sfail, dpfail, dppass uint32)
	StencilMask(mask uint32)
	ColorMask(r, g, b, a bool)

	// Pixel readback
	ReadBuffer(src uint32)
	ReadPixels(x, y, width, height int, colorFormat, typ uint32, pixels []uint8)

	// Error
	GetError() uint32
}

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*GLView)(nil)

// GLView is a canvas object that renders custom OpenGL content.
// The Render callback is invoked on the GL thread during the paint phase
// with the viewport set to the view's pixel bounds. The callback receives
// a GLContext for issuing GL commands, the pixel dimensions, and the
// display scale factor.
//
// # GLView requires the GL painter
//
// It hands the application a live OpenGL context and lets it draw into the
// window's GL framebuffer, so it only works under the GL painter — built with
// -tags gl, and the default on targets that have no WebGPU backend. The
// WebGPU painter, which is the default on desktop, has no OpenGL context to
// hand out: a GLView placed there draws nothing and reports why, once, to the
// error log (#213).
//
// For GPU-drawn content that works under every painter, use [Scene3D]: it
// describes meshes, a camera and lights declaratively rather than issuing GL
// calls, and every painter can draw it.
type GLView struct {
	baseObject

	// Render is called on the GL thread during each paint. The viewport
	// is pre-configured to this view's screen area. The context is only
	// valid for the duration of the callback.
	Render func(ctx GLContext, width, height int, pixelScale float32)
}

// NewGLView creates a new GLView with the given render callback.
func NewGLView(render func(ctx GLContext, width, height int, pixelScale float32)) *GLView {
	return &GLView{Render: render}
}

// Hide will set this view to not be visible.
func (v *GLView) Hide() {
	v.baseObject.Hide()
	repaint(v)
}

// Move the view to a new position, relative to its parent / canvas.
func (v *GLView) Move(pos geo.Position) {
	if v.Position() == pos {
		return
	}
	v.baseObject.Move(pos)
	repaint(v)
}

// Resize sets the view's size and triggers a refresh.
func (v *GLView) Resize(s geo.Size) {
	if s == v.Size() {
		return
	}
	v.baseObject.Resize(s)
	Refresh(v)
}

// Refresh causes this view to be redrawn.
func (v *GLView) Refresh() {
	Refresh(v)
}
