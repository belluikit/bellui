//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import "github.com/belluikit/bellui/canvas"

// glContextAdapter wraps the internal context interface to implement
// the public canvas.GLContext interface.
type glContextAdapter struct {
	ctx context
}

var _ canvas.GLContext = (*glContextAdapter)(nil)

// Texture operations

func (a *glContextAdapter) CreateTexture() canvas.GLTexture {
	return textureToGL(a.ctx.CreateTexture())
}

func (a *glContextAdapter) DeleteTexture(texture canvas.GLTexture) {
	a.ctx.DeleteTexture(glToTexture(texture))
}

func (a *glContextAdapter) ActiveTexture(textureUnit uint32) {
	a.ctx.ActiveTexture(textureUnit)
}

func (a *glContextAdapter) BindTexture(target uint32, texture canvas.GLTexture) {
	a.ctx.BindTexture(target, glToTexture(texture))
}

func (a *glContextAdapter) TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8) {
	a.ctx.TexImage2D(target, level, width, height, colorFormat, typ, data)
}

func (a *glContextAdapter) TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8) {
	a.ctx.TexSubImage2D(target, level, x, y, width, height, colorFormat, typ, data)
}

func (a *glContextAdapter) TexParameteri(target, param uint32, value int32) {
	a.ctx.TexParameteri(target, param, value)
}

// Buffer operations

func (a *glContextAdapter) CreateBuffer() canvas.GLBuffer {
	return bufferToGL(a.ctx.CreateBuffer())
}

func (a *glContextAdapter) DeleteBuffer(buffer canvas.GLBuffer) {
	a.ctx.DeleteBuffer(glToBuffer(buffer))
}

func (a *glContextAdapter) BindBuffer(target uint32, buf canvas.GLBuffer) {
	a.ctx.BindBuffer(target, glToBuffer(buf))
}

func (a *glContextAdapter) BufferData(target uint32, points []float32, usage uint32) {
	a.ctx.BufferData(target, points, usage)
}

func (a *glContextAdapter) BufferSubData(target uint32, points []float32) {
	a.ctx.BufferSubData(target, points)
}

// Shader operations

func (a *glContextAdapter) CreateShader(typ uint32) canvas.GLShader {
	return shaderToGL(a.ctx.CreateShader(typ))
}

func (a *glContextAdapter) CompileShader(shader canvas.GLShader) {
	a.ctx.CompileShader(glToShader(shader))
}

func (a *glContextAdapter) ShaderSource(shader canvas.GLShader, source string) {
	a.ctx.ShaderSource(glToShader(shader), source)
}

func (a *glContextAdapter) GetShaderi(shader canvas.GLShader, param uint32) int {
	return a.ctx.GetShaderi(glToShader(shader), param)
}

func (a *glContextAdapter) GetShaderInfoLog(shader canvas.GLShader) string {
	return a.ctx.GetShaderInfoLog(glToShader(shader))
}

// Program operations

func (a *glContextAdapter) CreateProgram() canvas.GLProgram {
	return programToGL(a.ctx.CreateProgram())
}

func (a *glContextAdapter) AttachShader(program canvas.GLProgram, shader canvas.GLShader) {
	a.ctx.AttachShader(glToProgram(program), glToShader(shader))
}

func (a *glContextAdapter) LinkProgram(program canvas.GLProgram) {
	a.ctx.LinkProgram(glToProgram(program))
}

func (a *glContextAdapter) GetProgrami(program canvas.GLProgram, param uint32) int {
	return a.ctx.GetProgrami(glToProgram(program), param)
}

func (a *glContextAdapter) GetProgramInfoLog(program canvas.GLProgram) string {
	return a.ctx.GetProgramInfoLog(glToProgram(program))
}

func (a *glContextAdapter) UseProgram(program canvas.GLProgram) {
	a.ctx.UseProgram(glToProgram(program))
}

// Vertex attributes

func (a *glContextAdapter) GetAttribLocation(program canvas.GLProgram, name string) canvas.GLAttribute {
	return attributeToGL(a.ctx.GetAttribLocation(glToProgram(program), name))
}

func (a *glContextAdapter) EnableVertexAttribArray(attribute canvas.GLAttribute) {
	a.ctx.EnableVertexAttribArray(glToAttribute(attribute))
}

func (a *glContextAdapter) VertexAttribPointerWithOffset(attribute canvas.GLAttribute, size int, typ uint32, normalized bool, stride, offset int) {
	a.ctx.VertexAttribPointerWithOffset(glToAttribute(attribute), size, typ, normalized, stride, offset)
}

// Uniforms

func (a *glContextAdapter) GetUniformLocation(program canvas.GLProgram, name string) canvas.GLUniform {
	return uniformToGL(a.ctx.GetUniformLocation(glToProgram(program), name))
}

func (a *glContextAdapter) Uniform1f(uniform canvas.GLUniform, v float32) {
	a.ctx.Uniform1f(glToUniform(uniform), v)
}

func (a *glContextAdapter) Uniform2f(uniform canvas.GLUniform, v0, v1 float32) {
	a.ctx.Uniform2f(glToUniform(uniform), v0, v1)
}

func (a *glContextAdapter) Uniform4f(uniform canvas.GLUniform, v0, v1, v2, v3 float32) {
	a.ctx.Uniform4f(glToUniform(uniform), v0, v1, v2, v3)
}

func (a *glContextAdapter) Uniform3f(uniform canvas.GLUniform, v0, v1, v2 float32) {
	a.ctx.Uniform3f(glToUniform(uniform), v0, v1, v2)
}

func (a *glContextAdapter) UniformMatrix3fv(uniform canvas.GLUniform, value [9]float32) {
	a.ctx.UniformMatrix3fv(glToUniform(uniform), value)
}

func (a *glContextAdapter) UniformMatrix4fv(uniform canvas.GLUniform, value [16]float32) {
	a.ctx.UniformMatrix4fv(glToUniform(uniform), value)
}

// Drawing

func (a *glContextAdapter) DrawArrays(mode uint32, first, count int) {
	a.ctx.DrawArrays(mode, first, count)
}

func (a *glContextAdapter) Viewport(x, y, width, height int) {
	a.ctx.Viewport(x, y, width, height)
}

func (a *glContextAdapter) Clear(mask uint32) {
	a.ctx.Clear(mask)
}

func (a *glContextAdapter) ClearColor(r, g, b, alpha float32) {
	a.ctx.ClearColor(r, g, b, alpha)
}

// Blending

func (a *glContextAdapter) BlendFunc(srcFactor, destFactor uint32) {
	a.ctx.BlendFunc(srcFactor, destFactor)
}

func (a *glContextAdapter) BlendColor(r, g, b, alpha float32) {
	a.ctx.BlendColor(r, g, b, alpha)
}

// Clipping

func (a *glContextAdapter) Scissor(x, y, w, h int32) {
	a.ctx.Scissor(x, y, w, h)
}

func (a *glContextAdapter) Enable(capability uint32) {
	a.ctx.Enable(capability)
}

func (a *glContextAdapter) Disable(capability uint32) {
	a.ctx.Disable(capability)
}

// Framebuffer objects

func (a *glContextAdapter) CreateFramebuffer() canvas.GLFramebuffer {
	return framebufferToGL(a.ctx.CreateFramebuffer())
}

func (a *glContextAdapter) DeleteFramebuffer(fb canvas.GLFramebuffer) {
	a.ctx.DeleteFramebuffer(glToFramebuffer(fb))
}

func (a *glContextAdapter) BindFramebuffer(target uint32, fb canvas.GLFramebuffer) {
	a.ctx.BindFramebuffer(target, glToFramebuffer(fb))
}

func (a *glContextAdapter) FramebufferTexture2D(target, attachment uint32, texture canvas.GLTexture, level int32) {
	a.ctx.FramebufferTexture2D(target, attachment, glToTexture(texture), level)
}

func (a *glContextAdapter) CheckFramebufferStatus(target uint32) uint32 {
	return a.ctx.CheckFramebufferStatus(target)
}

// Stencil

func (a *glContextAdapter) ClearStencil(s int32) {
	a.ctx.ClearStencil(s)
}

func (a *glContextAdapter) StencilFunc(fn uint32, ref int32, mask uint32) {
	a.ctx.StencilFunc(fn, ref, mask)
}

func (a *glContextAdapter) StencilOp(sfail, dpfail, dppass uint32) {
	a.ctx.StencilOp(sfail, dpfail, dppass)
}

func (a *glContextAdapter) StencilOpSeparate(face, sfail, dpfail, dppass uint32) {
	a.ctx.StencilOpSeparate(face, sfail, dpfail, dppass)
}

func (a *glContextAdapter) StencilMask(mask uint32) {
	a.ctx.StencilMask(mask)
}

func (a *glContextAdapter) ColorMask(r, g, b, alpha bool) {
	a.ctx.ColorMask(r, g, b, alpha)
}

// Pixel readback

func (a *glContextAdapter) ReadBuffer(src uint32) {
	a.ctx.ReadBuffer(src)
}

func (a *glContextAdapter) ReadPixels(x, y, width, height int, colorFormat, typ uint32, pixels []uint8) {
	a.ctx.ReadPixels(x, y, width, height, colorFormat, typ, pixels)
}

// Error

func (a *glContextAdapter) GetError() uint32 {
	return a.ctx.GetError()
}
