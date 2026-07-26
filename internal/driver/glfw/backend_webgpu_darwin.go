//go:build !gl && darwin && !wasm

package glfw

import (
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/internal/painter/webgpu"
)

// webgpuSurfaceDescriptor builds a CAMetalLayer-backed surface descriptor for
// the macOS window (GetCocoaWindow returns an unsafe.Pointer NSWindow, usable
// cross-package, so the bridge lives in the webgpu package).
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	return webgpu.SurfaceDescriptorForWindow(w.viewport)
}
