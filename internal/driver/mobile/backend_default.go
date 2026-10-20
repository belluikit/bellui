//go:build gl && !wasm

package mobile

import (
	"github.com/belluikit/bellui/core"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/gpu"
	pgl "github.com/belluikit/bellui/internal/painter/gl"
)

// Legacy GL fallback mobile backend: OpenGL ES via gomobile's EGL context,
// selected with `-tags gl`. WebGPU (backend_webgpu.go) is now the default
// mobile backend.

// newCanvasPainter creates the GL painter for a window's canvas.
func newCanvasPainter(c core.Canvas, w intdriver.WithContext) gpu.Painter {
	return pgl.NewPainter(c, w)
}

// renderReady reports whether the GL draw context is current and we can paint.
func (d *driver) renderReady(*window) bool { return d.glctx != nil }

// initCanvasPainter initialises the painter once the GL context is current. It
// always reports ready (the GL context is current by the time renderReady is
// true).
func (d *driver) initCanvasPainter(c *canvas, _ *window) bool { c.Painter().Init(); return true }

// presentPainter is a no-op for GL: gomobile's Publish() (eglSwapBuffers)
// presents the frame.
func (d *driver) presentPainter(*canvas) {}

// setPainterOutputSize is a no-op for GL: the painter derives its size from the
// per-paint frame dimensions.
func (d *driver) setPainterOutputSize(*canvas, int, int) {}

// recreateSurface is a no-op for GL: gomobile owns the EGL surface lifecycle and
// re-creates the GL context across background/resume itself (pre-rebrand tracker #310 is WebGPU-only).
func (d *driver) recreateSurface(*canvas, *window) {}

// detachWindowSurface is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer that must be detached on Close (pre-rebrand tracker #645).
func (d *driver) detachWindowSurface(*window) {}

// claimNativeSurface is a no-op for GL: gomobile owns the single EGL surface and
// every window's canvas paints through the shared GL context, so there is no
// per-window swapchain contention on the native window (pre-rebrand tracker #753 is WebGPU-only).
func (d *driver) claimNativeSurface(*canvas, *window) {}

// setWindowSurfaceHidden is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer whose visibility must toggle on Hide/Show (pre-rebrand tracker #742).
func (d *driver) setWindowSurfaceHidden(*window, bool) {}
