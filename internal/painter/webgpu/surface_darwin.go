//go:build !gl && darwin && !ios && !wasm

package webgpu

import (
	"unsafe"

	"github.com/belluikit/glfw"
	"github.com/belluikit/webgpu/wgpu"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <QuartzCore/CAMetalLayer.h>

// metalLayerFromNSWindow attaches a CAMetalLayer to the window's content view
// and returns it, so wgpu can create a Metal-backed surface. This is the same
// bridge wgpuglfw uses, reproduced here so it works against the belluikit/glfw
// fork (whose GetCocoaWindow returns the NSWindow) instead of the upstream
// go-gl/glfw that wgpuglfw is hard-wired to. (WebGPU Phase 0 spike, pre-rebrand tracker #199.)
static void* metalLayerFromNSWindow(void* nsWindowPtr) {
	NSWindow* nsWindow = (__bridge NSWindow*)nsWindowPtr;
	[nsWindow.contentView setWantsLayer:YES];
	CAMetalLayer* layer = [CAMetalLayer layer];
	[nsWindow.contentView setLayer:layer];
	return (__bridge void*)layer;
}
*/
import "C"

// SurfaceDescriptorForWindow builds a WebGPU surface descriptor for a GLFW
// window on macOS by attaching a CAMetalLayer to its content view. The window
// must have been created with glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI) so it
// has no GL context. The driver passes the result to painter.SetSurfaceDescriptor.
func SurfaceDescriptorForWindow(w *glfw.Window) *wgpu.SurfaceDescriptor {
	nsWindow := w.GetCocoaWindow() // unsafe.Pointer to NSWindow
	layer := C.metalLayerFromNSWindow(unsafe.Pointer(nsWindow))
	return &wgpu.SurfaceDescriptor{
		MetalLayer: &wgpu.SurfaceSourceMetalLayer{Layer: unsafe.Pointer(layer)},
	}
}
