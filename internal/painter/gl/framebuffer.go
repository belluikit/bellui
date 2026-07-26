//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !wasm

package gl

// Framebuffer represents an OpenGL framebuffer object for off-screen rendering.
// Desktop and mobile GL expose framebuffers as integer names.
type Framebuffer uint32

// defaultFramebuffer is the window-provided framebuffer (name 0). Binding it
// directs subsequent draws to the on-screen back buffer.
var defaultFramebuffer Framebuffer
