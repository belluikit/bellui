//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !wasm

package gl

// attachSceneDepth attaches a depth buffer to the currently-bound framebuffer
// for the Scene3D off-screen pass and returns a function that releases it.
//
// Desktop and mobile GL use a depth *texture*: core in desktop GL and widely
// available on GLES, and the original cross-platform path. WebGL 1.0 is the
// exception and uses a renderbuffer instead (see scene3d_depth_wasm.go, #136).
// The depth buffer is only used for z-buffering and is never sampled.
func (p *painter) attachSceneDepth(pixW, pixH int) func() {
	depthTex := p.ctx.CreateTexture()
	p.ctx.BindTexture(texture2D, depthTex)
	p.ctx.TexImage2D(texture2D, 0, pixW, pixH, depthComponent, unsignedShort, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[1]) // nearest
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[1])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.FramebufferTexture2D(glFramebuffer, depthAttachment, depthTex, 0)

	return func() { p.ctx.DeleteTexture(depthTex) }
}
