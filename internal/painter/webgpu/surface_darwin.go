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
#include <stdlib.h>
#include <string.h>

// tagSRGB declares the layer's pixels sRGB. Bellui draws sRGB values into a
// non-sRGB (unorm) surface, and without a colour space the window server does
// no colour matching, so on a wide-gamut display sRGB colours were shown
// oversaturated (#373).
static void tagSRGB(CAMetalLayer* layer) {
	CGColorSpaceRef cs = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
	layer.colorspace = cs;
	CGColorSpaceRelease(cs);
}

// layerColorSpaceName is the name of the layer's colour space, or "" when it
// has none; the caller frees it.
static char* layerColorSpaceName(void* layerPtr) {
	CAMetalLayer* layer = (__bridge CAMetalLayer*)layerPtr;
	CGColorSpaceRef cs = layer.colorspace;
	if (cs == NULL) {
		return strdup("");
	}
	CFStringRef name = CGColorSpaceCopyName(cs);
	if (name == NULL) {
		return strdup("(unnamed)");
	}
	char buf[256];
	Boolean ok = CFStringGetCString(name, buf, sizeof buf, kCFStringEncodingUTF8);
	CFRelease(name);
	return strdup(ok ? buf : "(unnamed)");
}

// metalLayerFromNSWindow attaches a CAMetalLayer to the window's content view
// and returns it, so wgpu can create a Metal-backed surface. This is the same
// bridge wgpuglfw uses, reproduced here so it works against the belluikit/glfw
// fork (whose GetCocoaWindow returns the NSWindow) instead of the upstream
// go-gl/glfw that wgpuglfw is hard-wired to. (WebGPU Phase 0 spike, pre-rebrand tracker #199.)
static void* metalLayerFromNSWindow(void* nsWindowPtr) {
	NSWindow* nsWindow = (__bridge NSWindow*)nsWindowPtr;
	[nsWindow.contentView setWantsLayer:YES];
	CAMetalLayer* layer = [CAMetalLayer layer];
	tagSRGB(layer);
	[nsWindow.contentView setLayer:layer];
	return (__bridge void*)layer;
}

// newBareMetalLayer returns a CAMetalLayer attached to no window, retained for
// the life of the process (owned via alloc/init; this file is not built with ARC).
static void* newBareMetalLayer(int srgb) {
	CAMetalLayer* layer = [[CAMetalLayer alloc] init];
	if (srgb) {
		tagSRGB(layer);
	}
	return (void*)layer;
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

// bareMetalLayerSurfaceDescriptor builds a surface descriptor over a CAMetalLayer
// attached to no window, so a test can configure and present a real Metal
// surface without opening one (#437).
func bareMetalLayerSurfaceDescriptor() *wgpu.SurfaceDescriptor {
	return bareMetalLayer(true)
}

// bareMetalLayer is bareMetalLayerSurfaceDescriptor with the sRGB tag the
// window's layer gets (srgb), or without it, for a test to compare.
func bareMetalLayer(srgb bool) *wgpu.SurfaceDescriptor {
	tag := C.int(0)
	if srgb {
		tag = 1
	}
	return &wgpu.SurfaceDescriptor{
		MetalLayer: &wgpu.SurfaceSourceMetalLayer{Layer: unsafe.Pointer(C.newBareMetalLayer(tag))},
	}
}

// metalLayerColorSpace is the name of a Metal layer's colour space, "" for
// none.
func metalLayerColorSpace(layer unsafe.Pointer) string {
	name := C.layerColorSpaceName(layer)
	defer C.free(unsafe.Pointer(name))
	return C.GoString(name)
}
