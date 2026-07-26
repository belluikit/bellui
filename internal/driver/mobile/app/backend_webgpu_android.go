//go:build android && !gl

package app

// webgpuBackend is true by default (false under the -tags gl fallback): the EGL surface and eglSwapBuffers
// are skipped so wgpu-native can own the ANativeWindow with a Vulkan surface (a
// native window can only host one rendering surface). gomobile still drives
// lifecycle, input and the publish handshake; the bellui painter presents.
const webgpuBackend = true
