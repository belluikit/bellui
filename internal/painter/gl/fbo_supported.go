//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !wasm

package gl

// framebufferSupported reports whether the GL context can allocate and render
// to off-screen framebuffer objects. FBO-dependent effects (Gaussian-blur drop
// shadows, backdrop blur) require it; desktop and mobile GL provide it.
const framebufferSupported = true
