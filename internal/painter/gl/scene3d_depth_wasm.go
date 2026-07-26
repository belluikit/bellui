//go:build (gl || (!darwin && !linux && !windows && !wasm)) && wasm

package gl

import "github.com/belluikit/gl-js"

// attachSceneDepth attaches a depth buffer to the currently-bound framebuffer
// for the Scene3D off-screen pass and returns a function that releases it.
//
// WebGL 1.0 has no core depth-texture support — a depth *texture* attachment
// needs the WEBGL_depth_texture extension, and without it the FBO is left
// incomplete so the Scene3D pass renders nothing (#136). A depth renderbuffer
// (DEPTH_COMPONENT16) is core in WebGL 1.0 and is all that is needed here: the
// depth buffer is only used for z-buffering this pass and is never sampled.
//
// The functions operate on the framebuffer already bound by drawScene3D, so
// this attaches to the right FBO; FramebufferRenderbuffer is issued directly
// against the gl-js context rather than through the painter's context
// abstraction (which exposes texture, not renderbuffer, attachments).
func (p *painter) attachSceneDepth(pixW, pixH int) func() {
	depthBuf := gl.CreateRenderbuffer()
	gl.BindRenderbuffer(gl.RENDERBUFFER, depthBuf)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, pixW, pixH)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depthBuf)

	return func() { gl.DeleteRenderbuffer(depthBuf) }
}
