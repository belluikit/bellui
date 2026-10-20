//go:build !gl && !android && !ios && !wasm

package mobile

import "github.com/belluikit/webgpu/wgpu"

// renderReady is always false: there is no native window on non-mobile builds
// of this package (e.g. desktop test compilation of the default webgpu build).
func (d *driver) renderReady(*window) bool { return false }

// nativeSurfacePresentable is always true: only Android's ANativeWindow dies
// under a live painter (#283), and renderReady above never lets a frame begin
// here anyway.
func (d *driver) nativeSurfacePresentable() bool { return true }

// webgpuSurfaceDescriptor has no native window on non-mobile builds of this
// package (e.g. desktop test compilation of the default webgpu build). iOS provides its
// own (CAMetalLayer) in backend_webgpu_ios.go.
func webgpuSurfaceDescriptor(*window) *wgpu.SurfaceDescriptor { return nil }

// syncNativeSurfaceSize is a no-op: there is no native surface on non-mobile
// builds of this package (only iOS re-sizes its CAMetalLayer, pre-rebrand tracker #243).
func syncNativeSurfaceSize(int, int) {}

// detachWindowSurface is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer that must be detached on Close (pre-rebrand tracker #645).
func (d *driver) detachWindowSurface(*window) {}

// claimNativeSurface is a no-op: there is no native window to contend for on
// non-mobile builds of this package (pre-rebrand tracker #753 is Android-specific).
func (d *driver) claimNativeSurface(*canvas, *window) {}

// setWindowSurfaceHidden is a no-op: only the iOS WebGPU backend stacks a
// per-window CAMetalLayer whose visibility must toggle on Hide/Show (pre-rebrand tracker #742).
func (d *driver) setWindowSurfaceHidden(*window, bool) {}
