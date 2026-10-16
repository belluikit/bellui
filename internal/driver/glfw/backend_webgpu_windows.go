//go:build !gl && windows && !wasm

package glfw

import (
	"unsafe"

	"github.com/belluikit/webgpu/wgpu"
)

// webgpuSurfaceDescriptor builds a Win32 HWND surface descriptor. Hinstance is
// left nil (wgpu-native resolves the module handle); set it via the win32 API if
// a backend requires it. NOTE: authored for pre-rebrand tracker #215 but not built on this (macOS)
// host — verify on Windows.
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	return &wgpu.SurfaceDescriptor{
		WindowsHWND: &wgpu.SurfaceSourceWindowsHWND{
			Hinstance: nil,
			Hwnd:      unsafe.Pointer(w.viewport.GetWin32Window()),
		},
	}
}
