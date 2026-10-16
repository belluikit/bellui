//go:build !gl

package mobile

import (
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/core"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/painter/webgpu"
)

// WebGPU rendering backend on mobile — the default. wgpu-native renders via
// Vulkan (Android) / Metal (iOS) directly onto the native window; gomobile keeps
// driving lifecycle and input but its EGL/GL setup is bypassed (see the app
// package's webgpu build). GLES is the fallback behind `-tags gl`. The per-OS
// surface descriptor is built by webgpuSurfaceDescriptor.

// newCanvasPainter creates the WebGPU painter for a window's canvas. The surface
// is attached later (initCanvasPainter) once the native window handle arrives.
func newCanvasPainter(c core.Canvas, w intdriver.WithContext) gpu.Painter {
	return webgpu.NewPainter(c, w)
}

// renderReady (per-OS: backend_webgpu_{android,ios,other}.go) reports whether a
// native surface can be created so we can paint. Android keys it on the
// ANativeWindow handle; iOS builds the CAMetalLayer from the key window, so it is
// ready as soon as the window exists.

// initCanvasPainter hands the painter the native-window surface descriptor, then
// brings the WebGPU device up. It reports whether the native surface was ready;
// the caller retries on a later frame if not (the iOS CAMetalLayer is built from
// the key window, which may not exist on the very first paint).
func (d *driver) initCanvasPainter(c *canvas, w *window) bool {
	sd := webgpuSurfaceDescriptor(w)
	if sd == nil {
		return false
	}
	if setter, ok := c.Painter().(interface {
		SetSurfaceDescriptor(*wgpu.SurfaceDescriptor)
	}); ok {
		setter.SetSurfaceDescriptor(sd)
	}
	c.Painter().Init() // may no-op until the output size is known (set in paintWindow)
	return true
}

// recreateSurface rebuilds the WebGPU surface after the native window changed —
// Android delivers a fresh ANativeWindow when the app returns from the
// background, and the painter must rebind or it presents to the destroyed
// surface (black canvas + ANR, pre-rebrand tracker #310). iOS builds the CAMetalLayer from the key
// window and does not deliver per-handle paint events, so this is effectively
// Android-only (the handle-change guard in handlePaint never fires on iOS).
func (d *driver) recreateSurface(c *canvas, w *window) {
	sd := webgpuSurfaceDescriptor(w)
	if sd == nil {
		return
	}
	if r, ok := c.Painter().(interface {
		RecreateSurface(*wgpu.SurfaceDescriptor)
	}); ok {
		r.RecreateSurface(sd)
	}
}

// presentPainter submits and presents the WebGPU frame. gomobile's Publish()
// does not present under -tags webgpu (EGL is bypassed), so the painter must.
//
// If the native window died while the frame was being recorded (Android
// destroys it mid-recreation; nativeSurfacePresentable is the per-OS liveness
// check), the frame is abandoned instead: presenting into a destroyed
// ANativeWindow is what wedges the GPU channel behind the #283 ANR. The
// abandoned frame's content is not lost for long — the recreated window's
// lifecycle re-cross refreshes the canvas, so the next paint redraws it.
func (d *driver) presentPainter(c *canvas) {
	if !d.nativeSurfacePresentable() {
		if ab, ok := c.Painter().(interface{ AbandonFrame() }); ok {
			ab.AbandonFrame()
		}
		return
	}
	if pr, ok := c.Painter().(interface{ Present() }); ok {
		pr.Present()
	}
}

// setPainterOutputSize tells the WebGPU painter its surface pixel size, which it
// needs to configure the surface and bring the device up on first paint. It also
// re-syncs the native surface to the new size where required (the iOS
// CAMetalLayer on rotation/resize, pre-rebrand tracker #243); a no-op on Android/other.
func (d *driver) setPainterOutputSize(c *canvas, w, h int) {
	syncNativeSurfaceSize(w, h)
	c.Painter().SetOutputSize(w, h)
}
