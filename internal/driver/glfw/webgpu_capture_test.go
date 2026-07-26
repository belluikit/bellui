//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// ensureCanvasSizeOrSkip waits for the window's canvas to reach size, SKIPPING
// the test if it never does within the retry budget — unlike the shared
// ensureCanvasSize, which fails. A headless or degraded surface cannot configure
// the window to the requested size (e.g. a self-hosted CI runner whose xvfb lacks
// DRI3 — "libEGL DRI3 error: Could not get DRI3 device" — where the WebGPU
// swapchain lands at a garbage size). These present→capture tests exercise the
// GPU round-trip, not window sizing, so an environment that can't size the
// surface is a gap to skip, matching the tests' existing no-window / no-capture
// skips — not a regression to fail on. A real sizing bug is caught by the
// dedicated resize tests on a healthy runner.
func ensureCanvasSizeOrSkip(t *testing.T, w *safeWindow, size geo.Size) {
	const maxAttempts = 100 // ~1s at 10ms, matching waitForCanvasSize
	for attempts := 0; ; attempts++ {
		if w.Canvas().Size() == size {
			return
		}
		if attempts >= maxAttempts {
			t.Skipf("canvas did not reach %v (got %v) — headless/degraded surface (e.g. no DRI3); "+
				"cannot exercise present/capture", size, w.Canvas().Size())
		}
		if attempts%20 == 0 {
			w.Resize(size) // the resize sometimes does not reach the window
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestWindowedSurfaceCapture exercises the real-window present + Canvas().Capture()
// path under the default backend (WebGPU unless built -tags gl). Every WebGPU GPU
// test that runs in CI is offscreen texture readback — none creates a real
// surface, calls presentWindow, or goes through Canvas().Capture() on a live
// window, so the surface-configure / present / capture-fallback path has zero
// automated coverage (#329). This drives that path on a real window (Metal on
// macOS, Vulkan/lavapipe on Linux, D3D12 on Windows) and asserts a known solid
// color survives a present→capture round-trip within tolerance.
//
// Run it with NO ci/gl/no_glfw tags so it builds the WebGPU backend on a real
// window; it self-skips when no window/GPU surface is available so it can't break
// a box without a GUI session.
func TestWindowedSurfaceCapture(t *testing.T) {
	want := color.NRGBA{R: 0, G: 180, B: 80, A: 255}

	w := createWindow("WebGPU surface capture")
	if w == nil || w.window == nil {
		t.Skip("no window available (headless / no GPU surface)")
	}
	defer w.Close()
	w.SetPadded(false)

	rect := canvas.NewRectangle(want)
	// The label exercises the glyph/text present path too; it is centered, so we
	// sample a quadrant point that stays on the solid rectangle, not on a glyph.
	w.SetContent(container.NewStack(rect, widget.NewLabel("webgpu surface capture")))
	runOnMain(func() { w.window.Show() })
	w.Resize(geo.NewSize(200, 150))
	ensureCanvasSizeOrSkip(t, w, geo.NewSize(200, 150))

	repaintWindow(w)
	repaintWindow(w)

	// safeWindow.Canvas() and safeCanvas.Capture() each marshal onto the main
	// thread themselves, so call them directly — wrapping in another runOnMain
	// would deadlock (nested main-thread dispatch).
	img := w.Canvas().Capture()
	if img == nil || img.Bounds().Empty() {
		t.Skip("no capture available (headless / no GPU surface)")
	}

	// Sample the top-left quadrant: solid rectangle, away from the centered label.
	b := img.Bounds()
	sx, sy := b.Min.X+b.Dx()/4, b.Min.Y+b.Dy()/4
	r, g, bl, _ := img.At(sx, sy).RGBA()
	gr, gg, gb := int(r>>8), int(g>>8), int(bl>>8)

	const tol = 16 // WebGPU writes straight sRGB into a non-sRGB surface (#322), so near-exact.
	if absInt(gr-int(want.R)) > tol || absInt(gg-int(want.G)) > tol || absInt(gb-int(want.B)) > tol {
		t.Fatalf("captured pixel (%d,%d) = R%d G%d B%d, want ~R%d G%d B%d — real-surface present/capture regression (#329)",
			sx, sy, gr, gg, gb, want.R, want.G, want.B)
	}
}

// TestWindowedSurfaceCaptureResizeStorm drives a burst of resizes interleaved with
// repaints before capturing, to provoke the surface-acquire recovery path (#385):
// on software Vulkan (lavapipe) a resize the swapchain hasn't reconfigured for makes
// the next GetCurrentTexture return Outdated. Before #384/#385 that aborted; now
// BeginFrame reconfigures and re-acquires, so frames must keep landing. This asserts
// a known solid color still survives present→capture after the storm — the
// integration coverage the static TestWindowedSurfaceCapture doesn't provide, since a
// single resize never races present against reconfigure (and Metal/D3D12 reconfigure
// transparently, so there the storm is simply exercised without ever hitting Outdated).
func TestWindowedSurfaceCaptureResizeStorm(t *testing.T) {
	want := color.NRGBA{R: 0, G: 180, B: 80, A: 255}

	w := createWindow("WebGPU resize storm")
	if w == nil || w.window == nil {
		t.Skip("no window available (headless / no GPU surface)")
	}
	defer w.Close()
	w.SetPadded(false)

	rect := canvas.NewRectangle(want)
	w.SetContent(container.NewStack(rect, widget.NewLabel("webgpu resize storm")))
	runOnMain(func() { w.window.Show() })

	// Alternate sizes and repaint at each: every Resize reconfigures the surface, so
	// presenting/acquiring across the burst exercises the reconfigure-and-retry path.
	sizes := []geo.Size{
		geo.NewSize(200, 150), geo.NewSize(260, 180), geo.NewSize(200, 150),
		geo.NewSize(320, 140), geo.NewSize(180, 220), geo.NewSize(200, 150),
	}
	for _, s := range sizes {
		w.Resize(s)
		ensureCanvasSizeOrSkip(t, w, s)
		repaintWindow(w)
		repaintWindow(w)
	}

	// Settle at a known size and repaint so the captured frame is deterministic.
	final := geo.NewSize(200, 150)
	w.Resize(final)
	ensureCanvasSizeOrSkip(t, w, final)
	repaintWindow(w)
	repaintWindow(w)

	img := w.Canvas().Capture()
	if img == nil || img.Bounds().Empty() {
		t.Skip("no capture available (headless / no GPU surface)")
	}

	// Sample the top-left quadrant: solid rectangle, away from the centered label.
	b := img.Bounds()
	sx, sy := b.Min.X+b.Dx()/4, b.Min.Y+b.Dy()/4
	r, g, bl, _ := img.At(sx, sy).RGBA()
	gr, gg, gb := int(r>>8), int(g>>8), int(bl>>8)

	const tol = 16
	if absInt(gr-int(want.R)) > tol || absInt(gg-int(want.G)) > tol || absInt(gb-int(want.B)) > tol {
		t.Fatalf("after resize storm, captured pixel (%d,%d) = R%d G%d B%d, want ~R%d G%d B%d — surface recovery regression (#385)",
			sx, sy, gr, gg, gb, want.R, want.G, want.B)
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestWindowedSurfaceCaptureOverlays pins the #473 fix: Canvas().Capture()
// must include the overlay pass — a shown PopUp was previously invisible to
// programmatic screenshots (the WebGPU per-frame copy never refreshed on a
// static scene and the fallback read the overlay-less canvas FBO).
func TestWindowedSurfaceCaptureOverlays(t *testing.T) {
	marker := color.NRGBA{R: 220, G: 30, B: 30, A: 255}

	w := createWindow("capture overlays")
	if w == nil || w.window == nil {
		t.Skip("no window available (headless / no GPU surface)")
	}
	defer w.Close()
	w.SetPadded(false)

	w.SetContent(widget.NewLabel("content beneath the popup"))
	runOnMain(func() { w.window.Show() })
	w.Resize(geo.NewSize(220, 160))
	ensureCanvasSizeOrSkip(t, w, geo.NewSize(220, 160))
	repaintWindow(w)

	// Show a popup overlay carrying a solid marker rectangle, then stop
	// producing frames: the capture itself must render the fresh frame.
	// Resolve the canvas outside runOnMain — safeWindow.Canvas() marshals to
	// the main thread itself, and nesting that inside runOnMain deadlocks.
	c := w.Canvas()
	runOnMain(func() {
		rect := canvas.NewRectangle(marker)
		rect.SetMinSize(geo.NewSize(80, 50))
		pop := widget.NewPopUp(rect, c)
		pop.ShowAtPosition(geo.NewPos(40, 40))
	})

	img := w.Canvas().Capture()
	if img == nil || img.Bounds().Empty() {
		t.Skip("no capture available (headless / no GPU surface)")
	}

	found := false
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y && !found; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if int(r>>8) > 180 && int(g>>8) < 90 && int(bl>>8) < 90 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("popup overlay marker not present in Capture() output (#473)")
	}
}
