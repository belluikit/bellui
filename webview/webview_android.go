//go:build android

package webview

/*
#cgo LDFLAGS: -landroid -llog

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

bool belluiWebViewInit(uintptr_t jniEnv, uintptr_t ctx);
int belluiWebViewCreate(uintptr_t jniEnv, uint64_t token, const char *script);
void belluiWebViewDestroy(uintptr_t jniEnv, int id);
void belluiWebViewSetGeometry(uintptr_t jniEnv, int id, int clipX, int clipY, int clipW, int clipH,
                              int viewX, int viewY, int viewW, int viewH);
void belluiWebViewSetVisible(uintptr_t jniEnv, int id, bool visible);
void belluiWebViewLoadURL(uintptr_t jniEnv, int id, const char *url);
void belluiWebViewLoadHTML(uintptr_t jniEnv, int id, const char *html, const char *baseURL);
void belluiWebViewEval(uintptr_t jniEnv, int id, const char *js, uint64_t callID, bool wantResult);
bool belluiWebViewNav(uintptr_t jniEnv, int id, int op);
bool belluiWebViewHasFocus(uintptr_t jniEnv, int id);
bool belluiWebViewSetFocus(uintptr_t jniEnv, int id, bool take);
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/mobileinit"
)

// Supported reports whether this platform has a web engine binding.
//
// Since: 1.0
const Supported = true

// webView holds the Android platform state: the Java-side id of the hosted
// WebView, plus the registry token every callback comes back with.
//
// Unlike the darwin backends there is no native pointer here. The view itself
// is a Java object — a WebView needs real Java subclasses and a
// @JavascriptInterface object, which cannot be built from JNI — so Go holds
// only the id that names it.
type webView struct {
	id    int
	token uint64
	url   string
}

// Callbacks arrive from Java carrying a token rather than a Go pointer. The
// registry resolves it back, mutex-guarded because Java delivers on the UI
// thread while Attach and Destroy run on the main goroutine.
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

func lookup(token uint64) *WebView {
	registryMu.Lock()
	defer registryMu.Unlock()

	return registry[token]
}

// onMain runs fn on the bellui main goroutine, which is where every callback
// this package makes is contracted to run. Java delivers on the Android UI
// thread, so the hop is always needed.
func onMain(fn func()) {
	bellui.Do(fn)
}

//export belluiWebViewMessageGo
func belluiWebViewMessageGo(token C.uint64_t, origin, body *C.char) {
	o, b := C.GoString(origin), C.GoString(body)
	tok := uint64(token)
	onMain(func() {
		if w := lookup(tok); w != nil {
			w.deliverMessage(o, b)
		}
	})
}

//export belluiWebViewEvalDoneGo
func belluiWebViewEvalDoneGo(token C.uint64_t, callID C.uint64_t, result, errMsg *C.char) {
	r, e := C.GoString(result), C.GoString(errMsg)
	tok, id := uint64(token), uint64(callID)
	onMain(func() {
		if w := lookup(tok); w != nil {
			w.deliverEvalResult(id, r, e)
		}
	})
}

//export belluiWebViewLoadGo
func belluiWebViewLoadGo(token C.uint64_t, state C.int, url, errMsg *C.char) {
	u, e := C.GoString(url), C.GoString(errMsg)
	tok, st := uint64(token), LoadState(state)
	onMain(func() {
		if w := lookup(tok); w != nil {
			w.deliverLoad(st, u, e)
		}
	})
}

// belluiWebViewNavigateGo answers the navigation policy hook. WebViewClient is
// blocked on the result, so unlike the other callbacks this one cannot simply
// post to the main goroutine — it waits for the answer there. OnNavigate is
// documented as having to return promptly for exactly this reason.
//
//export belluiWebViewNavigateGo
func belluiWebViewNavigateGo(token C.uint64_t, url *C.char, kind C.int, mainFrame, newWindow C.bool) C.char {
	req := NavigationRequest{
		URL:       C.GoString(url),
		Kind:      NavigationKind(kind),
		MainFrame: bool(mainFrame),
		NewWindow: bool(newWindow),
	}
	tok := uint64(token)

	allow := true
	bellui.DoAndWait(func() {
		if w := lookup(tok); w != nil {
			allow = w.decideNavigation(req)
		}
	})
	if allow {
		return 1
	}
	return 0
}

// withJNI runs fn with a JNIEnv for the calling goroutine. Every down-call
// needs one, and the Java side posts its own work to the UI thread, so nothing
// here blocks on it.
func withJNI(fn func(env uintptr)) {
	err := mobileinit.RunOnJVM(func(_, jniEnv, _ uintptr) error {
		fn(jniEnv)
		return nil
	})
	if err != nil {
		bellui.LogError("webview: JVM call failed", err)
	}
}

// attachRaw creates the WebView through the activity's Java entry points.
func (w *WebView) attachRaw(win bellui.Window) error {
	native, ok := win.(driver.NativeWindow)
	if !ok {
		return errors.New("webview: window does not expose a native handle")
	}

	// The bridge is installed as part of creating the view, so this is the one
	// moment it can be enabled — an empty script means the page is handed no
	// JavaScript interface and no shim at all.
	script := w.bridgeScript("belluiHost.post(s, window.location.origin);")
	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	var id int
	var ready bool
	native.RunNative(func(ctx any) {
		android, ok := ctx.(*driver.AndroidWindowContext)
		if !ok {
			return
		}
		if !bool(C.belluiWebViewInit(C.uintptr_t(android.Env), C.uintptr_t(android.Ctx))) {
			return
		}
		ready = true
		id = int(C.belluiWebViewCreate(C.uintptr_t(android.Env), C.uint64_t(w.token), cScript))
	})

	if !ready {
		// The activity class carries no webview entry points, which is what an
		// app built against an older embedded dex looks like. The host stays
		// inert rather than crashing.
		return errors.New("webview: this app's embedded Android support predates web view hosting")
	}
	if id == 0 {
		return errors.New("webview: no activity to host the view")
	}
	w.id = id
	return nil
}

func (w *WebView) attachedRaw() bool { return w.id != 0 }

func (w *WebView) destroyRaw() {
	registryMu.Lock()
	delete(registry, w.token)
	registryMu.Unlock()

	if w.id == 0 {
		return
	}
	id := w.id
	w.id = 0
	withJNI(func(env uintptr) {
		C.belluiWebViewDestroy(C.uintptr_t(env), C.int(id))
	})
}

func (w *WebView) loadURLRaw(url string) {
	if w.id == 0 {
		return
	}
	// android.webkit consults shouldOverrideUrlLoading only for navigations
	// the PAGE starts; a programmatic loadUrl bypasses it entirely, which
	// would make LoadURL the one navigation OnNavigate never sees on this
	// platform (#76 measured exactly that). The policy hook is applied here
	// instead, with the same refusal semantics the engine path has: a denied
	// main-frame load reports LoadStateFailed with ErrNavigationCancelled.
	if !w.decideNavigation(NavigationRequest{URL: url, Kind: NavigationOther, MainFrame: true}) {
		return
	}
	c := C.CString(url)
	defer C.free(unsafe.Pointer(c))
	withJNI(func(env uintptr) {
		C.belluiWebViewLoadURL(C.uintptr_t(env), C.int(w.id), c)
	})
}

func (w *WebView) loadHTMLRaw(html, baseURL string) {
	if w.id == 0 {
		return
	}
	cHTML, cBase := C.CString(html), C.CString(baseURL)
	defer C.free(unsafe.Pointer(cHTML))
	defer C.free(unsafe.Pointer(cBase))
	withJNI(func(env uintptr) {
		C.belluiWebViewLoadHTML(C.uintptr_t(env), C.int(w.id), cHTML, cBase)
	})
}

func (w *WebView) navRaw(op navOp) bool {
	if w.id == 0 {
		return false
	}
	var ok bool
	withJNI(func(env uintptr) {
		ok = bool(C.belluiWebViewNav(C.uintptr_t(env), C.int(w.id), C.int(op)))
	})
	return ok
}

func (w *WebView) evalRaw(js string) {
	w.evalAndroid(js, 0, false)
}

func (w *WebView) evalResultRaw(js string, callID uint64) {
	w.evalAndroid(js, callID, true)
}

func (w *WebView) evalAndroid(js string, callID uint64, wantResult bool) {
	if w.id == 0 {
		return
	}
	c := C.CString(js)
	defer C.free(unsafe.Pointer(c))
	withJNI(func(env uintptr) {
		C.belluiWebViewEval(C.uintptr_t(env), C.int(w.id), c, C.uint64_t(callID), C.bool(wantResult))
	})
}

// SetGeometry places the clipping container and the web view inside it.
//
// Android view geometry is in device pixels, and bellui works in
// device-independent points, so the canvas scale is the conversion — unlike the
// darwin backends, where view frames are in points and the toolkit handles the
// backing store itself.
func (w *WebView) SetGeometry(rect, clip geo.Rectangle, scale float32) {
	if w.id == 0 {
		return
	}
	px := func(v float32) C.int { return C.int(v * scale) }
	withJNI(func(env uintptr) {
		C.belluiWebViewSetGeometry(C.uintptr_t(env), C.int(w.id),
			px(clip.Pos.X), px(clip.Pos.Y), px(clip.Size.Width), px(clip.Size.Height),
			px(rect.Pos.X), px(rect.Pos.Y), px(rect.Size.Width), px(rect.Size.Height))
	})
}

// SetVisible shows or hides the native view.
func (w *WebView) SetVisible(visible bool) {
	if w.id == 0 {
		return
	}
	withJNI(func(env uintptr) {
		C.belluiWebViewSetVisible(C.uintptr_t(env), C.int(w.id), C.bool(visible))
	})
}

// TakeFocus gives the web engine keyboard focus, as a tap inside it would.
//
// While the web view holds it, keystrokes and input-method composition go to
// the web engine and bellui's canvas receives nothing. A hosted web view is
// kept in step with bellui's own focus by [widget.NativeViewHost], which drives
// these three methods; call them directly only on an unhosted view.
func (w *WebView) TakeFocus() bool {
	return w.setFocus(true)
}

// ResignFocus makes the web view give up the keyboard, so that keys and
// composition reach bellui again. Nothing is focused in its place: bellui's own
// text input claims focus only while an entry is focused, so claiming it here
// would raise the software keyboard with nothing being edited.
func (w *WebView) ResignFocus() bool {
	return w.setFocus(false)
}

func (w *WebView) setFocus(take bool) bool {
	if w.id == 0 {
		return false
	}
	var ok bool
	withJNI(func(env uintptr) {
		ok = bool(C.belluiWebViewSetFocus(C.uintptr_t(env), C.int(w.id), C.bool(take)))
	})
	return ok
}

// HasFocus reports whether the web engine currently holds the keyboard focus,
// meaning bellui is not receiving key events.
//
// It reads a shadow flag the Java side keeps up to date on the UI thread rather
// than asking the view, so the per-frame focus poll never blocks a Go goroutine
// on the Android UI thread.
func (w *WebView) HasFocus() bool {
	if w.id == 0 {
		return false
	}
	var focused bool
	withJNI(func(env uintptr) {
		focused = bool(C.belluiWebViewHasFocus(C.uintptr_t(env), C.int(w.id)))
	})
	return focused
}
