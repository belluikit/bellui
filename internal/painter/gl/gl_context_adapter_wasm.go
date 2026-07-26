//go:build (gl || (!darwin && !linux && !windows && !wasm)) && wasm

package gl

import "github.com/belluikit/bellui/canvas"

// WebGL object handles are js.Value wrappers and cannot be represented by the
// integer-based public canvas GL handle types. The public canvas GL API
// (canvas.GL* handles) is therefore not usable on the web target, so these
// conversions return zero values to keep the package compiling; they must not
// be relied on for real GL work. Internal off-screen rendering does work — the
// painter drives WebGL framebuffers directly through the context (see
// gl_wasm.go), bypassing these public-handle conversions.

func textureToGL(Texture) canvas.GLTexture             { return 0 }
func glToTexture(canvas.GLTexture) Texture             { return Texture{} }
func bufferToGL(Buffer) canvas.GLBuffer                { return 0 }
func glToBuffer(canvas.GLBuffer) Buffer                { return Buffer{} }
func programToGL(Program) canvas.GLProgram             { return 0 }
func glToProgram(canvas.GLProgram) Program             { return Program{} }
func shaderToGL(Shader) canvas.GLShader                { return 0 }
func glToShader(canvas.GLShader) Shader                { return Shader{} }
func attributeToGL(a Attribute) canvas.GLAttribute     { return canvas.GLAttribute(a.Value) }
func glToAttribute(g canvas.GLAttribute) Attribute     { return Attribute{Value: int(g)} }
func uniformToGL(Uniform) canvas.GLUniform             { return 0 }
func glToUniform(canvas.GLUniform) Uniform             { return Uniform{} }
func framebufferToGL(Framebuffer) canvas.GLFramebuffer { return 0 }
func glToFramebuffer(canvas.GLFramebuffer) Framebuffer { return Framebuffer{} }
