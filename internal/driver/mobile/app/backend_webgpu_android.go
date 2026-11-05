//go:build android && !gl

package app

// webgpuBackend is true by default (false under the -tags gl fallback): the EGL surface and eglSwapBuffers
// are skipped so wgpu-native can own the ANativeWindow with a Vulkan surface (a
// native window can only host one rendering surface). gomobile still drives
// lifecycle and input; the bellui painter presents.
const webgpuBackend = true

// The painter has presented by the time it publishes, so Publish only has to
// release a pending onNativeWindowRedrawNeeded, which it can do itself (#967).
func init() {
	publishDirect = func() PublishResult {
		signalRedrawDone()
		return PublishResult{}
	}
}
