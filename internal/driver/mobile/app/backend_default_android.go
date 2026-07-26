//go:build android && gl

package app

// webgpuBackend is false for the default GLES build: gomobile owns an EGL
// surface on the ANativeWindow and presents with eglSwapBuffers.
const webgpuBackend = false
