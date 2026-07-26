//go:build (gl || (!darwin && !linux && !windows && !wasm)) && wasm

package gl

// framebufferSupported reports whether the GL context can allocate and render
// to off-screen framebuffer objects. The WebGL backend drives the real
// framebuffer API (see gl_wasm.go) and the blur shader programs are
// initialised in Init, so FBO-dependent effects (Gaussian-blur drop shadows,
// backdrop blur) work on the web target — see issue #118 / #7.
const framebufferSupported = true
