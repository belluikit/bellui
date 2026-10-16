// Speaks the gomobile binding's struct Texture/Program types, so it is gl-only
// in practice for the same reason as gl_gomobile.go — see the build-tag note
// there. Carries the uniform package gate; no explicit `gl` term needed. (pre-rebrand tracker #251)
//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (android || ios || mobile)

package gl

import "github.com/belluikit/bellui/canvas"

func textureToGL(t Texture) canvas.GLTexture             { return canvas.GLTexture(t.Value) }
func glToTexture(g canvas.GLTexture) Texture             { return Texture{Value: uint32(g)} }
func bufferToGL(b Buffer) canvas.GLBuffer                { return canvas.GLBuffer(b.Value) }
func glToBuffer(g canvas.GLBuffer) Buffer                { return Buffer{Value: uint32(g)} }
func programToGL(p Program) canvas.GLProgram             { return canvas.GLProgram(p.Value) }
func glToProgram(g canvas.GLProgram) Program             { return Program{Init: true, Value: uint32(g)} }
func shaderToGL(s Shader) canvas.GLShader                { return canvas.GLShader(s.Value) }
func glToShader(g canvas.GLShader) Shader                { return Shader{Value: uint32(g)} }
func attributeToGL(a Attribute) canvas.GLAttribute       { return canvas.GLAttribute(a.Value) }
func glToAttribute(g canvas.GLAttribute) Attribute       { return Attribute{Value: uint(g)} }
func uniformToGL(u Uniform) canvas.GLUniform             { return canvas.GLUniform(u.Value) }
func glToUniform(g canvas.GLUniform) Uniform             { return Uniform{Value: int32(g)} }
func framebufferToGL(f Framebuffer) canvas.GLFramebuffer { return canvas.GLFramebuffer(f) }
func glToFramebuffer(g canvas.GLFramebuffer) Framebuffer { return Framebuffer(g) }
