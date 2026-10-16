//go:build !gl && linux && x11 && !wayland && !wasm

package glfw

import (
	"unsafe"

	"github.com/belluikit/glfw"
	"github.com/belluikit/webgpu/wgpu"
)

// webgpuSurfaceDescriptor builds an Xlib surface descriptor. The uintptr/
// unsafe.Pointer conversions of the glfw native handles mirror window_x11.go's
// RunNative. NOTE: authored for pre-rebrand tracker #215 but not built on this (macOS) host —
// verify on Linux/X11.
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	return &wgpu.SurfaceDescriptor{
		XlibWindow: &wgpu.SurfaceSourceXlibWindow{
			Display: unsafe.Pointer(glfw.GetX11Display()),
			Window:  uint32(w.viewport.GetX11Window()),
		},
	}
}
