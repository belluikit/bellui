//go:build !gl && android && !wasm

package mobile

import (
	"unsafe"

	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/internal/driver/mobile/app"
)

// renderReady reports whether the ANativeWindow handle is available (it arrives
// via paint.Event.Window) and the window is still alive, so the wgpu Vulkan
// surface can be created and presented to. The liveness check is load-bearing:
// during an in-process activity recreation the destroy→create gap overlaps
// queued paint ticks (the lifecycle re-cross refreshes the canvas), and a
// Vulkan present into the destroyed window wedges the emulator's GPU channel
// permanently — the #283 ANR. The handle alone cannot express this: it stays
// set until the recreated activity's paint event replaces it.
func (d *driver) renderReady(w *window) bool {
	return w != nil && w.handle != 0 && app.NativeWindowLive()
}

// nativeSurfacePresentable is the last-moment presentPainter re-check of the
// same liveness flag: a paint that began on a live window may outlive it (the
// destroy callback fires mid-frame), and the present — not the draw — is what
// poisons the dead window (#283). iOS has no equivalent window death.
func (d *driver) nativeSurfacePresentable() bool { return app.NativeWindowLive() }

// webgpuSurfaceDescriptor builds an ANativeWindow-backed surface descriptor for
// the Android window; wgpu-native renders into it via Vulkan. w.handle is the
// ANativeWindow* that gomobile delivered through paint.Event.Window.
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	return &wgpu.SurfaceDescriptor{
		AndroidNativeWindow: &wgpu.SurfaceSourceAndroidNativeWindow{
			Window: unsafe.Pointer(w.handle), //nolint:govet // ANativeWindow*, not a Go pointer
		},
	}
}

// syncNativeSurfaceSize is a no-op on Android: the ANativeWindow is sized by the
// system and the Vulkan surface follows it; only the iOS CAMetalLayer needs an
// explicit re-size on rotation (pre-rebrand tracker #243).
func syncNativeSurfaceSize(int, int) {}

// claimNativeSurface makes w the sole owner of the single ANativeWindow surface
// before it paints (pre-rebrand tracker #753). All Android windows share one native window, but each
// window's canvas builds its own wgpu surface + swapchain on it; two configured
// Vulkan swapchains on one native window can't both present, so opening a second
// window over a visible one left the newcomer's frames going nowhere (freeze).
//
// The driver only ever paints the current window, so on a current-window change
// (child Show, or child Close/Hide revealing the parent) release the previous
// owner's surface, then rebind w's — recreate if its device already exists (it
// was current before), otherwise leave the first-time build to initCanvasPainter
// in the caller. The previous owner keeps its device, so a later re-Show just
// recreates its surface.
func (d *driver) claimNativeSurface(c *canvas, w *window) {
	if d.surfaceOwner == w {
		return
	}
	if prev := d.surfaceOwner; prev != nil {
		if p, ok := prev.canvas.Painter().(interface{ ReleaseSurface() }); ok {
			p.ReleaseSurface()
		}
	}
	d.surfaceOwner = w
	if c.initialized {
		d.recreateSurface(c, w)
	}
}

// detachWindowSurface releases a closed/hidden window's surface so it stops
// holding the shared ANativeWindow; the caller (window close, shared path) then
// drops surface ownership so the revealed window reclaims it on its next paint
// (pre-rebrand tracker #753). The iOS backend detaches a per-window CAMetalLayer here instead (pre-rebrand tracker #645).
func (d *driver) detachWindowSurface(w *window) {
	if d.surfaceOwner != w {
		return
	}
	if p, ok := w.canvas.Painter().(interface{ ReleaseSurface() }); ok {
		p.ReleaseSurface()
	}
}

// setWindowSurfaceHidden is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer whose visibility must toggle on Hide/Show (pre-rebrand tracker #742).
func (d *driver) setWindowSurfaceHidden(*window, bool) {}
