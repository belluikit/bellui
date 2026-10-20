//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

// Framebuffer represents a GL framebuffer object for off-screen rendering. Its
// concrete type is platform-specific: an integer handle on desktop/mobile GL
// and a js.Value wrapper on the WebGL (wasm) target — see framebuffer.go and
// framebuffer_wasm.go. Bind defaultFramebuffer to target the screen.

type context interface {
	ActiveTexture(textureUnit uint32)
	AttachShader(program Program, shader Shader)
	BindBuffer(target uint32, buf Buffer)
	BindTexture(target uint32, texture Texture)
	BlendColor(r, g, b, a float32)
	BlendFunc(srcFactor, destFactor uint32)
	BlendFuncSeparate(srcRGB, destRGB, srcAlpha, destAlpha uint32)
	BufferData(target uint32, points []float32, usage uint32)
	BufferSubData(target uint32, points []float32)
	Clear(mask uint32)
	ClearColor(r, g, b, a float32)
	CompileShader(shader Shader)
	CreateBuffer() Buffer
	CreateProgram() Program
	CreateShader(typ uint32) Shader
	CreateTexture() Texture
	DeleteBuffer(buffer Buffer)
	DeleteTexture(texture Texture)
	Disable(capability uint32)
	DrawArrays(mode uint32, first, count int)
	Enable(capability uint32)
	EnableVertexAttribArray(attribute Attribute)
	GetAttribLocation(program Program, name string) Attribute
	GetError() uint32
	GetProgrami(program Program, param uint32) int
	GetProgramInfoLog(program Program) string
	GetShaderi(shader Shader, param uint32) int
	GetShaderInfoLog(shader Shader) string
	GetUniformLocation(program Program, name string) Uniform
	LinkProgram(program Program)
	ReadBuffer(src uint32)
	ReadPixels(x, y, width, height int, colorFormat, typ uint32, pixels []uint8)
	Scissor(x, y, w, h int32)
	ShaderSource(shader Shader, source string)
	TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8)
	TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8)
	TexParameteri(target, param uint32, value int32)
	Uniform1f(uniform Uniform, v float32)
	Uniform2f(uniform Uniform, v0, v1 float32)
	Uniform3f(uniform Uniform, v0, v1, v2 float32)
	Uniform4f(uniform Uniform, v0, v1, v2, v3 float32)
	UniformMatrix3fv(uniform Uniform, value [9]float32)
	UniformMatrix4fv(uniform Uniform, value [16]float32)
	UseProgram(program Program)
	VertexAttribPointerWithOffset(attribute Attribute, size int, typ uint32, normalized bool, stride, offset int)
	Viewport(x, y, width, height int)

	// Flush ensures all previously issued GL commands are submitted to the GPU.
	Flush()

	// CopyTexSubImage2D copies a rectangle of pixels from the current read
	// framebuffer directly into a bound texture, avoiding a GPU→CPU roundtrip.
	CopyTexSubImage2D(target uint32, level, xoffset, yoffset, x, y, width, height int)

	// Framebuffer methods for off-screen rendering.
	CreateFramebuffer() Framebuffer
	BindFramebuffer(target uint32, fb Framebuffer)
	FramebufferTexture2D(target, attachment uint32, texture Texture, level int32)
	CheckFramebufferStatus(target uint32) uint32
	DeleteFramebuffer(fb Framebuffer)

	// Color mask for stencil write pass.
	ColorMask(r, g, b, a bool)

	// Stencil buffer methods.
	ClearStencil(s int32)
	StencilFunc(fn uint32, ref int32, mask uint32)
	StencilOp(sfail, dpfail, dppass uint32)
	StencilOpSeparate(face, sfail, dpfail, dppass uint32)
	StencilMask(mask uint32)
}
