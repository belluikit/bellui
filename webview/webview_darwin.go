//go:build darwin && !ios && !wasm

package webview

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
#include "webview_darwin.h"
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
)

// Supported reports whether this platform has a web engine binding.
//
// Since: 2.19
const Supported = true

// webView holds the macOS platform state: an opaque handle to the flipped,
// layer-backed clip view that owns the WKWebView, plus the registry token the
// page's script messages come back with.
type webView struct {
	handle unsafe.Pointer
	token  uint64
	url    string
}

// Script messages arrive from ObjC carrying a token rather than a Go pointer,
// which cgo would not allow us to hand out. The registry resolves it back.
// Messages are delivered on the main thread — the same thread the bellui main
// goroutine sits on inside the GLFW event loop — but the registry is still
// mutex-guarded because Attach and Destroy may race a late message.
var (
	registryMu   sync.Mutex
	registry     = map[uint64]*WebView{}
	registryNext uint64
)

func (w *WebView) init() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registryNext++
	w.token = registryNext
	registry[w.token] = w
}

//export belluiWebViewMessage
func belluiWebViewMessage(token C.uint64_t, body *C.char) {
	registryMu.Lock()
	w := registry[uint64(token)]
	registryMu.Unlock()
	if w == nil || w.OnMessage == nil {
		return
	}
	w.OnMessage(C.GoString(body))
}

// Attach creates the WKWebView inside the window's native content view.
func (w *WebView) Attach(win bellui.Window) error {
	native, ok := win.(driver.NativeWindow)
	if !ok {
		return errors.New("webview: window does not expose a native handle")
	}

	var nsWindow uintptr
	native.RunNative(func(ctx any) {
		if mac, ok := ctx.(driver.MacWindowContext); ok {
			nsWindow = mac.NSWindow
		}
	})
	if nsWindow == 0 {
		return errors.New("webview: no NSWindow for this bellui window")
	}

	w.handle = C.belluiWebViewCreate(C.uintptr_t(nsWindow), C.uint64_t(w.token))
	if w.handle == nil {
		return errors.New("webview: NSWindow has no content view")
	}
	if w.url != "" {
		w.LoadURL(w.url)
	}
	return nil
}

// LoadURL navigates the view. It is a no-op until the view is attached, in
// which case the URL is remembered and loaded on attach.
func (w *WebView) LoadURL(url string) {
	w.url = url
	if w.handle == nil {
		return
	}
	c := C.CString(url)
	defer C.free(unsafe.Pointer(c))
	C.belluiWebViewLoadURL(w.handle, c)
}

// SetGeometry places the clip view and the web view inside it. Bellui
// coordinates are device-independent points and AppKit view frames are in
// points too, so the canvas scale is the only conversion needed (on a Retina
// display the backing store is denser, but AppKit handles that itself).
func (w *WebView) SetGeometry(rect, clip geo.Rectangle, scale float32) {
	if w.handle == nil {
		return
	}
	s := float64(scale)
	C.belluiWebViewSetGeometry(w.handle,
		C.double(float64(clip.Pos.X)*s), C.double(float64(clip.Pos.Y)*s),
		C.double(float64(clip.Size.Width)*s), C.double(float64(clip.Size.Height)*s),
		C.double(float64(rect.Pos.X)*s), C.double(float64(rect.Pos.Y)*s),
		C.double(float64(rect.Size.Width)*s), C.double(float64(rect.Size.Height)*s))
}

// SetVisible shows or hides the native view.
func (w *WebView) SetVisible(visible bool) {
	if w.handle == nil {
		return
	}
	C.belluiWebViewSetVisible(w.handle, C.bool(visible))
}

// TakeFocus gives the web engine keyboard focus, as a click inside it would.
// It reports whether the window accepted the change.
//
// Keyboard focus is arbitrated by the OS, not by bellui: while the web view
// holds it, keystrokes and input-method composition go to the web engine and
// bellui's canvas receives nothing. A hosted web view is kept in step with
// bellui's own focus by [widget.NativeViewHost], which drives these three
// methods; call them directly only on an unhosted view.
func (w *WebView) TakeFocus() bool {
	if w.handle == nil {
		return false
	}
	return bool(C.belluiWebViewTakeFocus(w.handle))
}

// ResignFocus hands the keyboard focus back to bellui's own surface, so that
// keys and input-method composition reach the canvas again. It reports
// whether the window accepted the change; a view that does not hold the focus
// reports success and changes nothing.
func (w *WebView) ResignFocus() bool {
	if w.handle == nil {
		return false
	}
	return bool(C.belluiWebViewResignFocus(w.handle))
}

// HasFocus reports whether the web engine currently holds the window's
// keyboard focus, meaning bellui is not receiving key events.
func (w *WebView) HasFocus() bool {
	if w.handle == nil {
		return false
	}
	return bool(C.belluiWebViewHasFocus(w.handle))
}

// Eval runs JavaScript inside the page, discarding its result. Together with
// [WebView.OnMessage] it is the two-way channel between bellui logic and page
// logic: the hole punch limits what can be drawn over the view, not what can
// be exchanged with it.
func (w *WebView) Eval(js string) {
	if w.handle == nil {
		return
	}
	c := C.CString(js)
	defer C.free(unsafe.Pointer(c))
	C.belluiWebViewEval(w.handle, c)
}

// Destroy removes the native view from the window and releases it.
func (w *WebView) Destroy() {
	registryMu.Lock()
	delete(registry, w.token)
	registryMu.Unlock()

	if w.handle == nil {
		return
	}
	C.belluiWebViewDestroy(w.handle)
	w.handle = nil
}
