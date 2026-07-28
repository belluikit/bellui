//go:build darwin && !ios && !wasm

package webview

/*
#cgo CFLAGS: -x objective-c
// QuartzCore is needed for CALayer, which the clip view uses for its native
// masksToBounds clipping. It happened to be linked in by another package
// until this one was built on its own.
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework QuartzCore
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
// layer-backed clip view that owns the WKWebView, plus the registry token
// every callback from the engine comes back with.
type webView struct {
	handle unsafe.Pointer
	token  uint64
	url    string
}

// Callbacks arrive from ObjC carrying a token rather than a Go pointer, which
// cgo would not allow us to hand out. The registry resolves it back.
// Everything is delivered on the main thread — the same thread the bellui main
// goroutine sits on inside the GLFW event loop — but the registry is still
// mutex-guarded because Attach and Destroy may race a late callback.
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

func lookup(token C.uint64_t) *WebView {
	registryMu.Lock()
	defer registryMu.Unlock()
	return registry[uint64(token)]
}

//export belluiWebViewMessage
func belluiWebViewMessage(token C.uint64_t, origin, body *C.char) {
	if w := lookup(token); w != nil {
		w.deliverMessage(C.GoString(origin), C.GoString(body))
	}
}

//export belluiWebViewEvalDone
func belluiWebViewEvalDone(token C.uint64_t, callID C.uint64_t, result, errMsg *C.char) {
	if w := lookup(token); w != nil {
		w.deliverEvalResult(uint64(callID), C.GoString(result), C.GoString(errMsg))
	}
}

//export belluiWebViewLoad
func belluiWebViewLoad(token C.uint64_t, state C.int, url, errMsg *C.char) {
	if w := lookup(token); w != nil {
		w.deliverLoad(LoadState(state), C.GoString(url), C.GoString(errMsg))
	}
}

//export belluiWebViewNavigate
func belluiWebViewNavigate(token C.uint64_t, url *C.char, kind C.int, mainFrame, newWindow C.bool) C.bool {
	w := lookup(token)
	if w == nil {
		return C.bool(true)
	}
	return C.bool(w.decideNavigation(NavigationRequest{
		URL:       C.GoString(url),
		Kind:      NavigationKind(kind),
		MainFrame: bool(mainFrame),
		NewWindow: bool(newWindow),
	}))
}

// attachRaw creates the WKWebView inside the window's native content view.
func (w *WebView) attachRaw(win bellui.Window) error {
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

	// The bridge is part of the configuration, so this is the one moment it
	// can be installed — and an empty script means WebKit is handed no script
	// message handler and no user script at all.
	script := w.bridgeScript("window.webkit.messageHandlers.bellui.postMessage(s);")
	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	w.handle = C.belluiWebViewCreate(C.uintptr_t(nsWindow), C.uint64_t(w.token), cScript)
	if w.handle == nil {
		return errors.New("webview: NSWindow has no content view")
	}
	return nil
}

func (w *WebView) attachedRaw() bool { return w.handle != nil }

func (w *WebView) loadURLRaw(url string) {
	c := C.CString(url)
	defer C.free(unsafe.Pointer(c))
	C.belluiWebViewLoadURL(w.handle, c)
}

func (w *WebView) loadHTMLRaw(html, baseURL string) {
	cHTML, cBase := C.CString(html), C.CString(baseURL)
	defer C.free(unsafe.Pointer(cHTML))
	defer C.free(unsafe.Pointer(cBase))
	C.belluiWebViewLoadHTML(w.handle, cHTML, cBase)
}

func (w *WebView) navRaw(op navOp) bool {
	if w.handle == nil {
		return false
	}
	return bool(C.belluiWebViewNav(w.handle, C.int(op)))
}

func (w *WebView) evalRaw(js string) {
	c := C.CString(js)
	defer C.free(unsafe.Pointer(c))
	C.belluiWebViewEval(w.handle, c, 0, C.bool(false))
}

func (w *WebView) evalResultRaw(js string, callID uint64) {
	c := C.CString(js)
	defer C.free(unsafe.Pointer(c))
	C.belluiWebViewEval(w.handle, c, C.uint64_t(callID), C.bool(true))
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

// destroyRaw removes the native view from the window and releases it.
func (w *WebView) destroyRaw() {
	registryMu.Lock()
	delete(registry, w.token)
	registryMu.Unlock()

	if w.handle == nil {
		return
	}
	C.belluiWebViewDestroy(w.handle)
	w.handle = nil
}
