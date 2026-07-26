//go:build !gl && linux && ((!x11 && !wayland) || (x11 && wayland)) && !wasm

package glfw

import (
	"unsafe"

	"github.com/belluikit/bellui/internal/build"

	"github.com/belluikit/glfw"
	"github.com/belluikit/webgpu/wgpu"
)

// webgpuSurfaceDescriptor builds the WebGPU surface descriptor for whichever
// backend GLFW selected at runtime. The default linux build compiles both the
// X11 and Wayland native accessors; build.IsWayland (set after glfw.Init) picks
// the live one. The conversions mirror window_x11wayland.go's RunNative.
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	if build.IsWayland {
		return &wgpu.SurfaceDescriptor{
			WaylandSurface: &wgpu.SurfaceDescriptorFromWaylandSurface{
				Display: unsafe.Pointer(glfw.GetWaylandDisplay()),
				Surface: unsafe.Pointer(w.viewport.GetWaylandWindow()),
			},
		}
	}

	return &wgpu.SurfaceDescriptor{
		XlibWindow: &wgpu.SurfaceDescriptorFromXlibWindow{
			Display: unsafe.Pointer(glfw.GetX11Display()),
			Window:  uint32(w.viewport.GetX11Window()),
		},
	}
}
