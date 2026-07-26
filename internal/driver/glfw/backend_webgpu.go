//go:build !gl && (darwin || linux || windows) && !wasm

package glfw

import (
	"github.com/belluikit/glfw"
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/painter/webgpu"
)

// WebGPU rendering backend (desktop) — the default. GL falls back behind
// `-tags gl` (backend_default.go). The per-OS surface descriptor is built by
// webgpuSurfaceDescriptor (backend_webgpu_{darwin,x11,wayland,windows}.go).
// Verified live on Metal (macOS), Vulkan (Linux X11/Wayland) and D3D12 (Windows).

// needsGLContext is false: a WebGPU window is created with glfw.NoAPI and has no
// GL context, so RunWithContext must not call MakeContextCurrent.
const needsGLContext = false

// applyBackendWindowHints requests a context-less window; wgpu owns the surface.
func applyBackendWindowHints() {
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
}

// newCanvasPainter creates the WebGPU painter and hands it the native surface
// descriptor for this platform before Init brings the device up.
func newCanvasPainter(c core.Canvas, w *window) gpu.Painter {
	p := webgpu.NewPainter(c, w)
	if setter, ok := p.(interface {
		SetSurfaceDescriptor(*wgpu.SurfaceDescriptor)
	}); ok {
		setter.SetSurfaceDescriptor(webgpuSurfaceDescriptor(w))
	}
	return p
}

// presentWindow presents the WebGPU surface (the analogue of SwapBuffers).
func presentWindow(w *window) {
	if pr, ok := w.canvas.Painter().(interface{ Present() }); ok {
		pr.Present()
	}
}
