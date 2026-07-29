//go:build ios

package webview

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework UIKit -framework WebKit -framework QuartzCore
#include <stdlib.h>
#include "webview_ios.h"
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// Supported reports whether this platform has a web engine binding.
//
// Since: 2.19
const Supported = true

// webView holds the iOS platform state: an opaque handle to the clipping
// UIView that owns the WKWebView, plus the registry token every callback from
// the engine comes back with.
type webView struct {
	handle unsafe.Pointer
	token  uint64
	url    string
}

func (w *WebView) init() {
	w.token = registerView(w)
}

func lookup(token C.uint64_t) *WebView {
	return lookupView(uint64(token))
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

// attachRaw creates the WKWebView in the application's window.
//
// win is unused, unlike on macOS: iOS windows report only a
// [driver.UnknownContext] from RunNative, so there is no native handle to take
// from it. The window is reached through the driver's own belluiAppWindow
// accessor instead — and there is exactly one, since mobile shows a single
// window at a time (#499).
func (w *WebView) attachRaw(win bellui.Window) error {
	// The bridge is part of the configuration, so this is the one moment it
	// can be installed — and an empty script means WebKit is handed no script
	// message handler and no user script at all.
	script := w.bridgeScript("window.webkit.messageHandlers.bellui.postMessage(s);")
	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	w.handle = C.belluiWebViewCreate(C.uint64_t(w.token), cScript)
	if w.handle == nil {
		return errors.New("webview: no application window to host the view")
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

// SetGeometry places the clip view and the web view inside it.
//
// Bellui coordinates are device-independent points and UIKit view frames are
// in points too, with the same top-left origin, so the canvas scale is the
// only conversion needed — UIKit handles the denser backing store of a Retina
// display itself.
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

// TakeFocus gives the web engine keyboard focus, as a tap inside it would. It
// reports whether UIKit accepted the change.
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

// ResignFocus makes the web view give up the keyboard, so that keys and
// composition reach bellui again. It reports whether the web view let go; a
// view that does not hold the focus reports success and changes nothing.
//
// Nothing is made first responder in its place — bellui's own text input
// claims the responder only while an entry is focused, and claiming it here
// would raise the software keyboard with nothing being edited.
func (w *WebView) ResignFocus() bool {
	if w.handle == nil {
		return false
	}
	return bool(C.belluiWebViewResignFocus(w.handle))
}

// HasFocus reports whether the web engine currently holds the keyboard focus,
// meaning bellui is not receiving key events.
func (w *WebView) HasFocus() bool {
	if w.handle == nil {
		return false
	}
	return bool(C.belluiWebViewHasFocus(w.handle))
}

// destroyRaw removes the native view from the window and releases it.
func (w *WebView) destroyRaw() {
	forgetView(w.token)

	if w.handle == nil {
		return
	}
	C.belluiWebViewDestroy(w.handle)
	w.handle = nil
}
