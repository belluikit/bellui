//go:build !gl && !android && !ios

package mobile

import "github.com/belluikit/webgpu/wgpu"

// renderReady is always false: there is no native window on non-mobile builds
// of this package (e.g. desktop test compilation of the default webgpu build).
func (d *driver) renderReady(*window) bool { return false }

// webgpuSurfaceDescriptor has no native window on non-mobile builds of this
// package (e.g. desktop test compilation of the default webgpu build). iOS provides its
// own (CAMetalLayer) in backend_webgpu_ios.go.
func webgpuSurfaceDescriptor(*window) *wgpu.SurfaceDescriptor { return nil }

// syncNativeSurfaceSize is a no-op: there is no native surface on non-mobile
// builds of this package (only iOS re-sizes its CAMetalLayer, #243).
func syncNativeSurfaceSize(int, int) {}

// detachWindowSurface is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer that must be detached on Close (#645).
func (d *driver) detachWindowSurface(*window) {}

// claimNativeSurface is a no-op: there is no native window to contend for on
// non-mobile builds of this package (#753 is Android-specific).
func (d *driver) claimNativeSurface(*canvas, *window) {}

// setWindowSurfaceHidden is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer whose visibility must toggle on Hide/Show (#742).
func (d *driver) setWindowSurfaceHidden(*window, bool) {}
