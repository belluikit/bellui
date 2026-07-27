//go:build !gl && linux && wayland && !x11 && !wasm

package glfw

import (
	"unsafe"

	"github.com/belluikit/glfw"
	"github.com/belluikit/webgpu/wgpu"
)

// webgpuSurfaceDescriptor builds a Wayland surface descriptor. The conversions
// mirror window_wayland.go's RunNative. NOTE: authored for #215 but not built on
// this (macOS) host — verify on Linux/Wayland.
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	return &wgpu.SurfaceDescriptor{
		WaylandSurface: &wgpu.SurfaceSourceWaylandSurface{
			Display: unsafe.Pointer(glfw.GetWaylandDisplay()),
			Surface: unsafe.Pointer(w.viewport.GetWaylandWindow()),
		},
	}
}
