//go:build ios && gl

package app

// webgpuBackend is false for the default GLES build: gomobile renders through
// the GLKView's EAGL context and presents with swapBuffers.
const webgpuBackend = false
