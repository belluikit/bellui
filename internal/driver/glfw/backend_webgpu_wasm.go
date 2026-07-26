//go:build !gl && wasm

package glfw

import (
	"github.com/belluikit/glfw-js"
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/painter/webgpu"
)

// WebGPU rendering backend on wasm — the default wasm build. The browser
// provides WebGPU directly (no wgpu-native), so the surface is just the canvas
// element of a context-less glfw-js window. The WebGL gl-js path is the
// fallback behind `-tags gl` (backend_default.go). (#203/#206.)

// needsGLContext is false: the glfw-js window is created with NoAPI and has no
// WebGL context, so RunWithContext must not call MakeContextCurrent.
const needsGLContext = false

// applyBackendWindowHints requests a context-less canvas (NoAPI) so the WebGPU
// backend can attach a "webgpu" context to it; a canvas holds one context type.
func applyBackendWindowHints() {
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
}

// newCanvasPainter creates the WebGPU painter and hands it the canvas-backed
// surface descriptor before Init brings the (browser) device up.
func newCanvasPainter(c core.Canvas, w *window) gpu.Painter {
	p := webgpu.NewPainter(c, w)
	if setter, ok := p.(interface {
		SetSurfaceDescriptor(*wgpu.SurfaceDescriptor)
	}); ok {
		setter.SetSurfaceDescriptor(&wgpu.SurfaceDescriptor{Canvas: w.viewport.GetCanvas()})
	}
	return p
}

// presentWindow submits the frame (the painter's EndFrame) and then pumps
// requestAnimationFrame. The browser presents the canvas's current texture
// automatically (wgpu Surface.Present is a no-op on js); SwapBuffers is the
// wasm frame heartbeat the run loop blocks on, so it is still required.
func presentWindow(w *window) {
	if pr, ok := w.canvas.Painter().(interface{ Present() }); ok {
		pr.Present()
	}
	w.viewport.SwapBuffers()
}
