//go:build gl || (!darwin && !linux && !windows && !wasm)

package glfw

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/painter/gl"
)

// Legacy GL fallback backend: OpenGL/GLES (and the WebGL gl-js path on wasm),
// selected with `-tags gl`. WebGPU (backend_webgpu*.go) is now the default on
// every desktop/wasm build; this GL path is also used on platforms WebGPU does
// not target.

// needsGLContext reports that the window owns a GL context that must be made
// current around paint work (RunWithContext).
const needsGLContext = true

// applyBackendWindowHints adds no extra hints; initWindowHints already sets the
// GL context hints for the platform.
func applyBackendWindowHints() {}

// newCanvasPainter creates the GL painter for a window's canvas.
func newCanvasPainter(c core.Canvas, w *window) gpu.Painter {
	return gl.NewPainter(c, w)
}

// tagSRGBSurface declares the window's pixels sRGB where the platform would
// otherwise show the GL surface unmatched (macOS, #373).
func tagSRGBSurface(w *window) { tagSRGBWindow(w) }

// presentWindow swaps the GL double buffer to present the frame.
func presentWindow(w *window) {
	w.viewport.SwapBuffers()
}
