//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (gles || arm || arm64) && !android && !ios && !mobile && !darwin && !wasm

package gl

import "github.com/belluikit/bellui/canvas"

// GLES handle types are integers, same shape as the core profile.

func textureToGL(t Texture) canvas.GLTexture             { return canvas.GLTexture(t) }
func glToTexture(g canvas.GLTexture) Texture             { return Texture(g) }
func bufferToGL(b Buffer) canvas.GLBuffer                { return canvas.GLBuffer(b) }
func glToBuffer(g canvas.GLBuffer) Buffer                { return Buffer(g) }
func programToGL(p Program) canvas.GLProgram             { return canvas.GLProgram(p) }
func glToProgram(g canvas.GLProgram) Program             { return Program(g) }
func shaderToGL(s Shader) canvas.GLShader                { return canvas.GLShader(s) }
func glToShader(g canvas.GLShader) Shader                { return Shader(g) }
func attributeToGL(a Attribute) canvas.GLAttribute       { return canvas.GLAttribute(a) }
func glToAttribute(g canvas.GLAttribute) Attribute       { return Attribute(g) }
func uniformToGL(u Uniform) canvas.GLUniform             { return canvas.GLUniform(u) }
func glToUniform(g canvas.GLUniform) Uniform             { return Uniform(g) }
func framebufferToGL(f Framebuffer) canvas.GLFramebuffer { return canvas.GLFramebuffer(f) }
func glToFramebuffer(g canvas.GLFramebuffer) Framebuffer { return Framebuffer(g) }
