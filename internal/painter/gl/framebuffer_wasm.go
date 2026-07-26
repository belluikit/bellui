//go:build (gl || (!darwin && !linux && !windows && !wasm)) && wasm

package gl

import "github.com/belluikit/gl-js"

// Framebuffer represents a WebGL framebuffer object for off-screen rendering.
// WebGL object handles are js.Value wrappers, not integers, so the web target
// mirrors gl.Framebuffer rather than the desktop uint32 name.
type Framebuffer gl.Framebuffer

// defaultFramebuffer is the canvas back buffer. WebGL identifies it with the
// null framebuffer; binding it directs subsequent draws to the screen. Its
// zero value is not usable (a zero js.Value is not WebGL null), so it must be
// initialised from gl.NoFramebuffer.
var defaultFramebuffer = Framebuffer(gl.NoFramebuffer)
