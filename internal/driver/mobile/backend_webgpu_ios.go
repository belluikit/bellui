//go:build !gl && ios && !wasm

package mobile

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework UIKit -framework QuartzCore

#import <UIKit/UIKit.h>
#import <QuartzCore/CAMetalLayer.h>

// metalLayerForKeyWindow attaches a CAMetalLayer to the key window's root view
// and returns it, so wgpu can build a Metal-backed surface. This mirrors the
// desktop NSWindow bridge (painter/webgpu/surface_darwin.go) for iOS/UIView.
// Under -tags webgpu the gomobile GLKView/GLES draw path is bypassed (see the
// app package's BELLUI_WEBGPU branch), so the bare Metal sublayer is what shows —
// which also dodges the iOS-Simulator's broken GLES path (pre-rebrand tracker #195).
static void *metalLayerForKeyWindowMain() {
	UIWindow *keyWin = nil;
	for (UIWindow *win in [UIApplication sharedApplication].windows) {
		if (win.isKeyWindow) { keyWin = win; break; }
	}
	if (keyWin == nil) {
		keyWin = [UIApplication sharedApplication].windows.firstObject;
	}
	if (keyWin == nil) { return NULL; }
	UIView *v = keyWin.rootViewController.view;
	if (v == nil) { return NULL; }
	if (v.bounds.size.width <= 0 || v.bounds.size.height <= 0) { return NULL; } // not laid out yet

	CGFloat scale = [UIScreen mainScreen].scale;
	CAMetalLayer *layer = [CAMetalLayer layer];
	layer.frame = v.bounds;
	layer.contentsScale = scale;
	layer.drawableSize = CGSizeMake(v.bounds.size.width * scale, v.bounds.size.height * scale);
	layer.opaque = YES;
	[v.layer addSublayer:layer];
	return (__bridge void *)layer;
}

// metalLayerForKeyWindow runs the layer setup on the main thread: UIKit /
// CoreAnimation views must be read and mutated there, but the bellui painter runs
// on a background goroutine. Returns NULL until the view exists and is laid out.
static void *metalLayerForKeyWindow() {
	if ([NSThread isMainThread]) {
		return metalLayerForKeyWindowMain();
	}
	__block void *result = NULL;
	dispatch_sync(dispatch_get_main_queue(), ^{
		result = metalLayerForKeyWindowMain();
	});
	return result;
}

// resizeMetalLayerMain re-sizes the layer to wPx×hPx device pixels. frame is in
// the superlayer's point space (pixels / contentsScale); drawableSize is in
// pixels and is set authoritatively to the painter's surface size so wgpu never
// needs to derive it from the view (it matches the SurfaceConfiguration size).
static void resizeMetalLayerMain(void *layerPtr, int wPx, int hPx) {
	if (layerPtr == NULL) { return; }
	CAMetalLayer *layer = (__bridge CAMetalLayer *)layerPtr;
	CGFloat scale = layer.contentsScale;
	if (scale <= 0) { scale = [UIScreen mainScreen].scale; }
	// CATransaction without implicit animations: a plain frame/drawableSize set
	// would otherwise animate the resize on rotation.
	[CATransaction begin];
	[CATransaction setDisableActions:YES];
	layer.frame = CGRectMake(0, 0, wPx / scale, hPx / scale);
	layer.drawableSize = CGSizeMake(wPx, hPx);
	[CATransaction commit];
}

// resizeMetalLayer marshals the resize onto the main thread (CoreAnimation views
// must be mutated there; the bellui painter runs on a background goroutine).
static void resizeMetalLayer(void *layerPtr, int wPx, int hPx) {
	if ([NSThread isMainThread]) {
		resizeMetalLayerMain(layerPtr, wPx, hPx);
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), ^{
		resizeMetalLayerMain(layerPtr, wPx, hPx);
	});
}

// removeMetalLayerMain detaches a CAMetalLayer sublayer from its superlayer, so a
// closed window's leftover layer stops covering the window revealed beneath it.
static void removeMetalLayerMain(void *layerPtr) {
	if (layerPtr == NULL) { return; }
	CAMetalLayer *layer = (__bridge CAMetalLayer *)layerPtr;
	[layer removeFromSuperlayer];
}

static void removeMetalLayer(void *layerPtr) {
	if ([NSThread isMainThread]) {
		removeMetalLayerMain(layerPtr);
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), ^{
		removeMetalLayerMain(layerPtr);
	});
}

// setMetalLayerHiddenMain toggles a layer's visibility without detaching it, so a
// Hidden (but re-Showable) window's layer stops covering the window beneath while
// staying available for a later Show (pre-rebrand tracker #742). Unlike removeMetalLayer, the layer
// keeps its place in the superlayer and its wgpu surface stays valid.
static void setMetalLayerHiddenMain(void *layerPtr, int hidden) {
	if (layerPtr == NULL) { return; }
	CAMetalLayer *layer = (__bridge CAMetalLayer *)layerPtr;
	layer.hidden = hidden ? YES : NO;
}

static void setMetalLayerHidden(void *layerPtr, int hidden) {
	if ([NSThread isMainThread]) {
		setMetalLayerHiddenMain(layerPtr, hidden);
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), ^{
		setMetalLayerHiddenMain(layerPtr, hidden);
	});
}

// bringMetalLayerToFrontMain re-orders a layer to the top of its superlayer's
// sublayer stack, so a re-Shown window covers the one revealed while it was
// hidden (pre-rebrand tracker #742). Re-adding an already-attached sublayer moves it to the front.
static void bringMetalLayerToFrontMain(void *layerPtr) {
	if (layerPtr == NULL) { return; }
	CAMetalLayer *layer = (__bridge CAMetalLayer *)layerPtr;
	CALayer *super = layer.superlayer;
	if (super == nil) { return; }
	[layer removeFromSuperlayer];
	[super addSublayer:layer];
}

static void bringMetalLayerToFront(void *layerPtr) {
	if ([NSThread isMainThread]) {
		bringMetalLayerToFrontMain(layerPtr);
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), ^{
		bringMetalLayerToFrontMain(layerPtr);
	});
}
*/
import "C"

import (
	"unsafe"

	"github.com/belluikit/webgpu/wgpu"
)

// iosMetalLayer is the CAMetalLayer created for the key window, retained so the
// layer can be re-sized on rotation/resize (pre-rebrand tracker #243). Only touched from the mobile
// paint goroutine (webgpuSurfaceDescriptor and syncNativeSurfaceSize both run
// there, in handlePaint), so no synchronisation is needed.
var (
	iosMetalLayer        unsafe.Pointer
	iosLayerW, iosLayerH int
	// iosWindowLayers maps each window to the CAMetalLayer created for it, so
	// Close can detach exactly that window's layer (pre-rebrand tracker #645). Only touched from the
	// mobile paint/run goroutine (webgpuSurfaceDescriptor in handlePaint, Close
	// via bellui.Do), so no synchronisation is needed — like iosMetalLayer.
	iosWindowLayers = map[*window]unsafe.Pointer{}
)

// renderReady is true once the window exists; the CAMetalLayer surface is built
// from the key window (no per-event handle plumbing, unlike Android).
func (d *driver) renderReady(w *window) bool { return w != nil }

// nativeSurfacePresentable is always true on iOS: the CAMetalLayer lives with
// the view hierarchy and is never destroyed under a live painter the way
// Android's ANativeWindow is (#283).
func (d *driver) nativeSurfacePresentable() bool { return true }

// webgpuSurfaceDescriptor builds a CAMetalLayer-backed surface descriptor for the
// iOS view; wgpu-native renders into it via Metal. Returns nil until the window
// and its view exist (the painter's Init then retries on the next frame).
func webgpuSurfaceDescriptor(w *window) *wgpu.SurfaceDescriptor {
	layer := C.metalLayerForKeyWindow()
	if layer == nil {
		return nil
	}
	// Retain the layer for later re-sizing; reset the cached size so the first
	// syncNativeSurfaceSize re-asserts the authoritative surface pixel size. Also
	// record it on the window so Close can detach exactly this window's layer.
	iosMetalLayer = unsafe.Pointer(layer)
	iosLayerW, iosLayerH = 0, 0
	if w != nil {
		iosWindowLayers[w] = unsafe.Pointer(layer)
	}
	return &wgpu.SurfaceDescriptor{
		MetalLayer: &wgpu.SurfaceSourceMetalLayer{Layer: unsafe.Pointer(layer)},
	}
}

// claimNativeSurface is a no-op on iOS: each window gets its own CAMetalLayer
// sublayer (stacked/detached/hidden per window), so there is no shared-surface
// contention to hand over — that is the Android-only pre-rebrand tracker #753 problem.
func (d *driver) claimNativeSurface(*canvas, *window) {}

// detachWindowSurface removes a closed window's CAMetalLayer from the view so it
// stops covering the window revealed beneath it (pre-rebrand tracker #645). If the removed layer was
// the one tracked for resizing, re-point the resize state at the now-current
// window's layer.
func (d *driver) detachWindowSurface(w *window) {
	layer := iosWindowLayers[w]
	if layer == nil {
		return
	}
	C.removeMetalLayer(layer)
	delete(iosWindowLayers, w)
	if iosMetalLayer == layer {
		iosMetalLayer = nil
		iosLayerW, iosLayerH = 0, 0
		if cur := d.currentWindow(); cur != nil {
			iosMetalLayer = iosWindowLayers[cur]
		}
	}
}

// setWindowSurfaceHidden toggles a window's CAMetalLayer visibility for
// Hide/Show pop-back (pre-rebrand tracker #742). Hiding keeps the layer attached and its wgpu
// surface intact (a hidden window may be re-Shown), unlike Close's
// detachWindowSurface which removes it. Hiding also re-points the resize state at
// the now-revealed window; un-hiding brings the layer back to the front and
// makes it the resize target again (resetting the size cache so the next
// syncNativeSurfaceSize re-asserts its pixel size).
func (d *driver) setWindowSurfaceHidden(w *window, hidden bool) {
	layer := iosWindowLayers[w]
	if layer == nil {
		return
	}
	C.setMetalLayerHidden(layer, cInt(hidden))
	if hidden {
		if iosMetalLayer == layer {
			iosMetalLayer = nil
			iosLayerW, iosLayerH = 0, 0
			if cur := d.currentWindow(); cur != nil {
				iosMetalLayer = iosWindowLayers[cur]
			}
		}
		return
	}
	C.bringMetalLayerToFront(layer)
	iosMetalLayer = layer
	iosLayerW, iosLayerH = 0, 0
}

func cInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// syncNativeSurfaceSize re-syncs the CAMetalLayer to the new pixel size when it
// changes — on rotation or any resize the sublayer must follow the view, else
// the render is mis-sized/clipped (pre-rebrand tracker #243). The CAMetalLayer bridge sets the size
// once at creation, so without this the layer would stay at its initial bounds.
// Guarded on an actual change to avoid a per-frame main-thread round-trip (the
// paint loop calls this every frame, like the pre-rebrand tracker #262 Device.Resize guard).
func syncNativeSurfaceSize(w, h int) {
	if iosMetalLayer == nil || w <= 0 || h <= 0 {
		return
	}
	if w == iosLayerW && h == iosLayerH {
		return
	}
	iosLayerW, iosLayerH = w, h
	C.resizeMetalLayer(iosMetalLayer, C.int(w), C.int(h))
}
