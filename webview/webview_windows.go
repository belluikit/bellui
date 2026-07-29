//go:build windows

package webview

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
)

// Supported reports whether this platform has a web engine binding.
//
// It is true on Windows because the binding exists, not because the machine
// necessarily has a runtime: WebView2 is a separate component and [WebView.Attach]
// reports [ErrRuntimeMissing] when it is absent, leaving the host inert.
//
// Since: 2.19
const Supported = true

// winState is where a view is in the WebView2 creation handshake. Creating an
// environment and then a controller are both asynchronous — their results
// arrive on the message loop — so a view is attached before it can be talked
// to, and operations issued in between are held until it can.
type winState int

const (
	winIdle winState = iota
	winCreating
	winReady
	winDead
)

// winRect mirrors the Win32 RECT the controller's Bounds property takes.
type winRect struct {
	left, top, right, bottom int32
}

// webView holds the Windows platform state.
//
// The clip window is what makes an enclosing [container.Scroll] work: WebView2
// has no clip of its own, only Bounds, and shrinking Bounds would re-lay-out
// the page rather than reveal less of it. So the controller is parented to a
// child window sized to the clip, and the web view is placed inside it at the
// host's full rectangle — the same shape as the macOS clip view, expressed in
// HWNDs.
type webView struct {
	url   string
	token uint64

	parent uintptr // the glfw window's HWND
	clip   uintptr // our child HWND, the native clip container

	env  comObject // ICoreWebView2Environment
	ctrl comObject // ICoreWebView2Controller
	core comObject // ICoreWebView2

	state    winState
	script   string // the page-side bridge, empty when the bridge is off
	deferred []func()

	// focus shadows the engine's focus state, fed by the controller's GotFocus
	// and LostFocus events. WebView2 has no "do you have the focus" getter, and
	// the host polls HasFocus every frame, so the answer has to be local.
	focus bool

	bounds  winRect
	visible bool

	// cancelled records navigations refused by OnNavigate, so the completion
	// the engine reports for them afterwards is not delivered a second time
	// with a network-shaped error over the cancellation already reported.
	cancelled map[uint64]struct{}
	stopping  bool
}

// Callbacks arrive from the engine carrying a token rather than a Go pointer,
// so a callback that outlives its view resolves to nothing instead of into
// freed memory.
var (
	viewsMu   sync.Mutex
	views     = map[uint64]*WebView{}
	viewsNext uint64
)

func (w *WebView) init() {
	w.token = registerView(w)
}

func registerView(w *WebView) uint64 {
	viewsMu.Lock()
	defer viewsMu.Unlock()

	viewsNext++
	views[viewsNext] = w
	return viewsNext
}

func lookupView(token uint64) *WebView {
	viewsMu.Lock()
	defer viewsMu.Unlock()

	return views[token]
}

func forgetView(token uint64) {
	viewsMu.Lock()
	defer viewsMu.Unlock()

	delete(views, token)
}

// ---- attach ----

// attachRaw creates the clip window and starts the WebView2 handshake. It
// returns as soon as the request is accepted: the environment and controller
// arrive later, on the message loop, and everything asked of the view in the
// meantime is queued and replayed in order.
//
// Blocking here instead — pumping messages until the controller exists — would
// re-enter the driver's own window procedure from inside a widget lifecycle
// call, which is the deadlock shape #973 cost Android; the queue is the same
// answer the other backends reached.
func (w *WebView) attachRaw(win bellui.Window) error {
	native, ok := win.(driver.NativeWindow)
	if !ok {
		return errors.New("webview: window does not expose a native handle")
	}

	var hwnd uintptr
	native.RunNative(func(ctx any) {
		if w, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = w.HWND
		}
	})
	if hwnd == 0 {
		return errors.New("webview: no HWND for this bellui window")
	}

	if _, err := webview2Runtime(); err != nil {
		return err
	}
	folder, err := webview2UserDataFolder()
	if err != nil {
		return fmt.Errorf("webview: no writable user data folder: %w", err)
	}
	initCOM()

	clip, err := createClipWindow(hwnd)
	if err != nil {
		return err
	}

	if lookupView(w.token) == nil { // re-attaching a destroyed view
		w.token = registerView(w)
	}
	w.parent, w.clip = hwnd, clip
	w.state = winCreating

	// The bridge is part of the view's configuration, so this is the one moment
	// it can be decided. An empty script means no document script and no
	// WebMessageReceived subscription, and web messages are switched off
	// outright below, so the page has no channel to bellui at all.
	w.script = w.bridgeScript("window.chrome.webview.postMessage(s);")

	h := newHandler(handlerEnvCompleted, &iidEnvironmentCompleted, w.token, 0)
	hr, err := createWebView2Environment(folder, h)
	h.release()
	if err != nil {
		w.teardown()
		return err
	}
	if hr < 0 {
		w.teardown()
		return fmt.Errorf("webview: CreateCoreWebView2Environment failed (0x%08x)", uint32(hr))
	}
	return nil
}

func (w *WebView) attachedRaw() bool {
	return w.state == winCreating || w.state == winReady
}

// onEnvironmentCreated continues the handshake with the controller, which is
// the object that owns the browser's window and geometry.
func (w *WebView) onEnvironmentCreated(hr int32, env comObject) {
	if w.state != winCreating {
		return // destroyed while the environment was being created
	}
	if hr < 0 || env == 0 {
		w.attachFailed(fmt.Errorf("webview: WebView2 environment unavailable (0x%08x)", uint32(hr)))
		return
	}

	env.addRef()
	w.env = env

	h := newHandler(handlerControllerCompleted, &iidControllerCompleted, w.token, 0)
	res := env.call(envCreateController, w.clip, h.ptr())
	h.release()
	if res < 0 {
		w.attachFailed(fmt.Errorf("webview: CreateCoreWebView2Controller failed (0x%08x)", uint32(res)))
	}
}

// onControllerCreated finishes the handshake: it wires the events, installs the
// bridge script, applies the geometry gathered while the view was still being
// created, and replays everything that was asked of it in the meantime.
func (w *WebView) onControllerCreated(hr int32, ctrl comObject) {
	if w.state != winCreating {
		if ctrl != 0 { // destroyed meanwhile; do not leave a browser running
			ctrl.call(ctrlClose)
		}
		return
	}
	if hr < 0 || ctrl == 0 {
		w.attachFailed(fmt.Errorf("webview: WebView2 controller unavailable (0x%08x)", uint32(hr)))
		return
	}

	ctrl.addRef()
	w.ctrl = ctrl
	w.core = ctrl.getObject(ctrlGetCoreWebView2)
	if w.core == 0 {
		w.attachFailed(errors.New("webview: WebView2 controller has no core object"))
		return
	}

	w.applySettings()
	w.wireEvents()
	w.state = winReady

	w.ctrl.putBounds(w.bounds)
	w.ctrl.putBool(ctrlPutIsVisible, w.visible)

	deferred := w.deferred
	w.deferred = nil
	for _, fn := range deferred {
		fn()
	}
}

// applySettings narrows the engine to what this package actually uses. Host
// objects are never wanted — the bridge is the supported channel — and web
// messages exist only when the bridge does, so a page whose view was attached
// with the bridge off has no window.chrome.webview.postMessage to reach for.
func (w *WebView) applySettings() {
	settings := w.core.getObject(coreGetSettings)
	if settings == 0 {
		return
	}
	defer settings.release()

	settings.putBool(settingsPutAreHostObjectsAllowed, false)
	if w.script == "" {
		settings.putBool(settingsPutIsWebMessageEnabled, false)
	}
}

// wireEvents subscribes to everything the portable half reports on. The bridge
// subscription and the document script are installed together or not at all.
func (w *WebView) wireEvents() {
	w.subscribe(w.core, coreAddNavigationStarting, handlerNavStarting, &iidNavigationStarting)
	w.subscribe(w.core, coreAddFrameNavigationStarting, handlerFrameNavStarting, &iidNavigationStarting)
	w.subscribe(w.core, coreAddNavigationCompleted, handlerNavCompleted, &iidNavigationCompleted)
	w.subscribe(w.core, coreAddNewWindowRequested, handlerNewWindow, &iidNewWindowRequested)
	w.subscribe(w.ctrl, ctrlAddGotFocus, handlerGotFocus, &iidFocusChanged)
	w.subscribe(w.ctrl, ctrlAddLostFocus, handlerLostFocus, &iidFocusChanged)

	if w.script == "" {
		return
	}
	w.subscribe(w.core, coreAddWebMessageReceived, handlerWebMessage, &iidWebMessageReceived)

	script, err := windows.UTF16PtrFromString(w.script)
	if err != nil {
		bellui.LogError("webview: bridge script cannot be encoded", err)
		return
	}
	done := newHandler(handlerAddScriptDone, &iidAddScriptDone, w.token, 0)
	w.core.call(coreAddScriptToExecuteOnDocumentCreated, uintptr(unsafe.Pointer(script)), done.ptr())
	done.release()
	runtime.KeepAlive(script)
}

// subscribe adds an event handler through one of the add_* methods, whose
// shape is always (handler, out token).
func (w *WebView) subscribe(obj comObject, slot int, kind handlerKind, iid *windows.GUID) {
	h := newHandler(kind, iid, w.token, 0)
	defer h.release()

	var token int64
	obj.call(slot, h.ptr(), uintptr(unsafe.Pointer(&token)))
}

// attachFailed reports a handshake that could not be completed and leaves the
// view in the same state an unsupported platform would: inert, with every
// pending callback failed rather than left waiting.
func (w *WebView) attachFailed(err error) {
	bellui.LogError("Failed to create WebView2", err)
	w.teardown()
	w.failPending(ErrNotAttached)
}

// whenReady runs fn now if the engine exists, or queues it in order until it
// does. A view that never becomes ready drops the queue in teardown, and the
// callbacks behind those operations are failed there.
func (w *WebView) whenReady(fn func()) {
	switch w.state {
	case winReady:
		fn()
	case winCreating:
		w.deferred = append(w.deferred, fn)
	}
}

// ---- teardown ----

func (w *WebView) destroyRaw() {
	w.teardown()
	forgetView(w.token)
}

// teardown releases the engine and the clip window. It is idempotent, and safe
// at any point in the handshake: a controller that has not arrived yet is
// refused by the state check in its own completion handler.
func (w *WebView) teardown() {
	w.state = winDead
	w.deferred = nil

	if w.core != 0 {
		w.core.release()
		w.core = 0
	}
	if w.ctrl != 0 {
		w.ctrl.call(ctrlClose)
		w.ctrl.release()
		w.ctrl = 0
	}
	if w.env != 0 {
		w.env.release()
		w.env = 0
	}
	if w.clip != 0 {
		procDestroyWindow.Call(w.clip)
		w.clip = 0
	}
	w.parent = 0
	w.focus = false
	w.visible = false
	w.cancelled = nil
}

// ---- navigation and scripting ----

func (w *WebView) loadURLRaw(url string) {
	w.whenReady(func() {
		p, err := windows.UTF16PtrFromString(url)
		if err != nil {
			bellui.LogError("webview: cannot encode URL", err)
			return
		}
		w.core.call(coreNavigate, uintptr(unsafe.Pointer(p)))
		runtime.KeepAlive(p)
	})
}

// loadHTMLRaw shows literal content.
//
// WebView2 has no base URL for literal content — NavigateToString is the whole
// API — so baseURL is ignored here, and the content is given the opaque origin
// the bridge reports as [OriginOpaque] whatever was passed. Relative references
// in such a page do not resolve; serve the content, or use
// [WebView.LoadURL] with a file: URL, when it has any.
func (w *WebView) loadHTMLRaw(html, _ string) {
	w.whenReady(func() {
		p, err := windows.UTF16PtrFromString(html)
		if err != nil {
			bellui.LogError("webview: cannot encode HTML", err)
			return
		}
		w.core.call(coreNavigateToString, uintptr(unsafe.Pointer(p)))
		runtime.KeepAlive(p)
	})
}

// navRaw answers the history and load-control operations. The queries read the
// engine's own cached state, which is a local property read rather than a round
// trip to the browser process, so the per-frame paths that reach them do not
// wait on anything.
func (w *WebView) navRaw(op navOp) bool {
	if w.state != winReady {
		return false
	}

	switch op {
	case navCanGoBack:
		return w.core.getBool(coreGetCanGoBack)
	case navGoBack:
		if !w.core.getBool(coreGetCanGoBack) {
			return false
		}
		return w.core.call(coreGoBack) >= 0
	case navCanGoForward:
		return w.core.getBool(coreGetCanGoForward)
	case navGoForward:
		if !w.core.getBool(coreGetCanGoForward) {
			return false
		}
		return w.core.call(coreGoForward) >= 0
	case navReload:
		return w.core.call(coreReload) >= 0
	case navStopLoading:
		// Remember only a stop that interrupts something: the engine reports
		// the interrupted navigation as cancelled afterwards, and the portable
		// half has already reported that cancellation itself.
		w.stopping = w.load == LoadStateLoading
		return w.core.call(coreStop) >= 0
	}
	return false
}

func (w *WebView) evalRaw(js string) {
	w.execute(js, 0)
}

func (w *WebView) evalResultRaw(js string, callID uint64) {
	w.execute(js, callID)
}

// execute runs a script in the page, optionally reporting its value.
//
// A script that throws is reported as the JSON value null rather than as an
// error: ExecuteScript reports an unhandled exception by producing a null
// result with a successful HRESULT, and gives no channel for the exception
// itself. Android's evaluateJavascript has the same shape and the same caveat.
func (w *WebView) execute(js string, callID uint64) {
	w.whenReady(func() {
		p, err := windows.UTF16PtrFromString(js)
		if err != nil {
			bellui.LogError("webview: cannot encode script", err)
			return
		}

		// A completion handler is passed even when the result is unwanted: the
		// API documents no null argument, and an id nothing is waiting on is
		// dropped by deliverEvalResult anyway.
		h := newHandler(handlerExecuteScriptDone, &iidExecuteScriptDone, w.token, callID)
		defer h.release()

		w.core.call(coreExecuteScript, uintptr(unsafe.Pointer(p)), h.ptr())
		runtime.KeepAlive(p)
	})
}

// ---- events ----

func (w *WebView) onWebMessage(args comObject) {
	if args == 0 {
		return
	}
	body := args.getString(msgArgsTryGetWebMessageAsString)
	if body == "" {
		return // not a string message: nothing the shim posted
	}
	// The origin is the engine's own attribution of the sending frame, derived
	// from the document URI it reports, never anything the message claims —
	// which is what makes the OriginPolicy a boundary rather than a suggestion.
	w.deliverMessage(serializeOrigin(args.getString(msgArgsGetSource)), body)
}

func (w *WebView) onNavigationStarting(args comObject, mainFrame bool) {
	if args == 0 {
		return
	}

	uri := args.getString(navStartGetURI)
	kind := NavigationOther
	if args.getBool(navStartGetIsUserInitiated) {
		// WebView2 reports whether the user started the navigation, not what
		// they did to start it, so a form submission is reported as a link.
		kind = NavigationLinkClicked
	}

	if !w.decideNavigation(NavigationRequest{URL: uri, Kind: kind, MainFrame: mainFrame}) {
		args.putBool(navStartPutCancel, true)
		if mainFrame {
			// Remembered only for the main frame, which is the only completion
			// this backend subscribes to and therefore the only id that will be
			// taken back out again.
			if w.cancelled == nil {
				w.cancelled = map[uint64]struct{}{}
			}
			w.cancelled[args.getUint64(navStartGetNavigationID)] = struct{}{}
		}
		return
	}
	if mainFrame {
		w.reportLoad(LoadStateLoading, uri, nil)
	}
}

func (w *WebView) onNavigationCompleted(args comObject) {
	if args == 0 {
		return
	}

	id := args.getUint64(navDoneGetNavigationID)
	if _, refused := w.cancelled[id]; refused {
		delete(w.cancelled, id)
		return // already reported as cancelled by decideNavigation
	}

	url := w.core.getString(coreGetSource)
	if args.getBool(navDoneGetIsSuccess) {
		w.deliverLoad(LoadStateComplete, url, "")
		return
	}

	status := args.getInt32(navDoneGetErrorStatus)
	if status == webErrorOperationCanceled && w.stopping {
		w.stopping = false
		return // StopLoading already reported this as cancelled
	}
	w.deliverLoad(LoadStateFailed, url, webErrorMessage(status))
}

// onNewWindowRequested answers a target="_blank" or window.open navigation.
// There is no second window to open it in, so an allowed one replaces the
// current page and the engine is told the request was handled either way —
// otherwise WebView2 opens a bare popup window of its own, outside the layout
// and outside the app's control.
func (w *WebView) onNewWindowRequested(args comObject) {
	if args == 0 {
		return
	}

	uri := args.getString(newWinGetURI)
	kind := NavigationOther
	if args.getBool(newWinGetIsUserInitiated) {
		kind = NavigationLinkClicked
	}

	allowed := w.decideNavigation(NavigationRequest{URL: uri, Kind: kind, MainFrame: true, NewWindow: true})
	args.putBool(newWinPutHandled, true)
	if allowed && uri != "" {
		w.loadURLRaw(uri)
	}
}

func (w *WebView) onFocusChanged(has bool) {
	w.focus = has
}

func (w *WebView) onExecuteScriptDone(callID uint64, hr int32, resultJSON uintptr) {
	if hr < 0 {
		w.deliverEvalResult(callID, "", fmt.Sprintf("webview: ExecuteScript failed (0x%08x)", uint32(hr)))
		return
	}
	// The string belongs to the engine for the duration of the call, so it is
	// read but never freed.
	w.deliverEvalResult(callID, utf16AtToString(resultJSON), "")
}

// webErrorMessage names a COREWEBVIEW2_WEB_ERROR_STATUS. Only the statuses an
// app can act on are named; the rest report their number, which is enough to
// look up and better than inventing a description.
func webErrorMessage(status int32) string {
	switch status {
	case 1, 2, 3, 4, 5:
		return "webview: the server's certificate was rejected"
	case 6, 12:
		return "webview: cannot connect to the server"
	case 7:
		return "webview: the connection timed out"
	case 9, 10, 11:
		return "webview: the connection was lost"
	case 13:
		return "webview: the host name could not be resolved"
	case 14:
		return "webview: the navigation was cancelled"
	case 15:
		return "webview: a redirect failed"
	case 17, 18:
		return "webview: authentication is required"
	}
	return fmt.Sprintf("webview: navigation failed (status %d)", status)
}

// ---- geometry, visibility and focus ----

// SetGeometry places the clip window and the web view inside it.
//
// Win32 window geometry is in physical pixels and bellui works in
// device-independent points, so the canvas scale is the conversion — as on
// Android, and unlike the darwin backends where view frames are in points and
// the toolkit handles the backing store itself.
func (w *WebView) SetGeometry(rect, clip geo.Rectangle, scale float32) {
	if w.clip == 0 {
		return
	}

	cx, cy := px(clip.Pos.X, scale), px(clip.Pos.Y, scale)
	cw, ch := px(clip.Size.Width, scale), px(clip.Size.Height, scale)
	procSetWindowPos.Call(w.clip, 0, uintptr(cx), uintptr(cy), uintptr(cw), uintptr(ch),
		swpNoZOrder|swpNoActivate)

	// The web view keeps its full size and is offset inside the clip, so a
	// partly scrolled-out host reveals less of the page rather than squeezing
	// it — the clip window is what hides the rest.
	x, y := px(rect.Pos.X, scale)-cx, px(rect.Pos.Y, scale)-cy
	w.bounds = winRect{left: x, top: y, right: x + px(rect.Size.Width, scale), bottom: y + px(rect.Size.Height, scale)}
	if w.state == winReady {
		w.ctrl.putBounds(w.bounds)
		// The browser maps its own popups and candidate windows through the
		// ancestor chain, which put_Bounds does not tell it about.
		w.ctrl.call(ctrlNotifyParentWindowPositionChanged)
	}
}

// px converts a bellui coordinate to physical pixels.
func px(v float32, scale float32) int32 {
	return int32(math.Round(float64(v) * float64(scale)))
}

// SetVisible shows or hides the native view. The controller is told as well as
// the window, so a hidden view stops rendering rather than merely being covered.
func (w *WebView) SetVisible(visible bool) {
	if w.clip == 0 {
		return
	}
	w.visible = visible

	show := uintptr(swHide)
	if visible {
		show = swShowNoActivate
	}
	procShowWindow.Call(w.clip, show)

	if w.state == winReady {
		w.ctrl.putBool(ctrlPutIsVisible, visible)
	}
}

// TakeFocus gives the web engine keyboard focus, as a click inside it would.
//
// Keyboard focus is arbitrated by the OS, not by bellui: while the web view
// holds it, keystrokes and input-method composition go to the web engine and
// bellui's canvas receives nothing. A hosted web view is kept in step with
// bellui's own focus by [widget.NativeViewHost], which drives these three
// methods; call them directly only on an unhosted view.
func (w *WebView) TakeFocus() bool {
	if w.state != winReady {
		return false
	}
	return w.ctrl.call(ctrlMoveFocus, moveFocusProgrammatic) >= 0
}

// ResignFocus hands the keyboard focus back to bellui's own surface, so that
// keys and input-method composition reach the canvas again. It reports whether
// the platform accepted; a view that does not hold the focus reports success
// and changes nothing.
//
// The focus is given to the window bellui actually listens on. The driver
// installs its key handling on the glfw window's own HWND, so handing the focus
// anywhere else — the clip window, say — would route keystrokes to nothing at
// all, which is the same trap the macOS backend has to avoid with the content
// view rather than the window.
func (w *WebView) ResignFocus() bool {
	if !w.focus {
		return true
	}
	if w.parent == 0 {
		return false
	}
	prev, _, _ := procSetFocus.Call(w.parent)
	if prev == 0 {
		return false
	}
	w.focus = false
	return true
}

// HasFocus reports whether the web engine currently holds the window's keyboard
// focus, meaning bellui is not receiving key events.
//
// WebView2 has no getter for this, so the answer is a shadow of the
// controller's GotFocus and LostFocus events rather than a query — which is
// also what keeps the host's per-frame poll free of a round trip to the browser
// process.
func (w *WebView) HasFocus() bool {
	return w.focus
}

// initCOM puts the calling thread — the main goroutine's, where the driver
// pumps the message loop — into a single-threaded apartment, which is what
// WebView2 needs to deliver its callbacks there.
//
// It is deliberately never balanced with CoUninitialize. The driver may have
// initialized the apartment already (RPC_E_CHANGED_MODE or S_FALSE simply say
// so), and the browser has to outlive this call in any case, so the reference
// this takes is meant to last as long as the process. Once is enough: repeating
// it per view would stack references for no purpose.
var initCOM = sync.OnceFunc(func() {
	procCoInitializeEx.Call(0, coinitApartmentThreaded)
})

// ---- the clip window ----

var (
	moduser32   = windows.NewLazySystemDLL("user32.dll")
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modole32    = windows.NewLazySystemDLL("ole32.dll")

	procCoInitializeEx = modole32.NewProc("CoInitializeEx")

	procRegisterClassEx = moduser32.NewProc("RegisterClassExW")
	procCreateWindowEx  = moduser32.NewProc("CreateWindowExW")
	procDestroyWindow   = moduser32.NewProc("DestroyWindow")
	procDefWindowProc   = moduser32.NewProc("DefWindowProcW")
	procSetWindowPos    = moduser32.NewProc("SetWindowPos")
	procShowWindow      = moduser32.NewProc("ShowWindow")
	procSetFocus        = moduser32.NewProc("SetFocus")
	procGetWindowLong   = moduser32.NewProc("GetWindowLongW")
	procSetWindowLong   = moduser32.NewProc("SetWindowLongW")

	procGetModuleHandle = modkernel32.NewProc("GetModuleHandleW")
)

const (
	wsChild        = 0x40000000
	wsClipChildren = 0x02000000
	wsClipSiblings = 0x04000000

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	swHide           = 0
	swShowNoActivate = 4

	coinitApartmentThreaded = 0x2

	// GWL_STYLE is -16; only the low 32 bits of the index reach the API, so it
	// travels as the unsigned spelling of that value.
	gwlStyle = uintptr(0xFFFFFFF0)
)

// wndClassEx mirrors the Win32 WNDCLASSEXW.
type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

var (
	clipClassOnce sync.Once
	clipClassName *uint16
	clipClassErr  error
)

// registerClipClass registers the window class of the clip container, once per
// process. The class paints nothing — no background brush — because everything
// inside it is covered by the browser's own window, and erasing first would
// flash under it.
func registerClipClass() {
	name, err := windows.UTF16PtrFromString("BelluiWebViewClip")
	if err != nil {
		clipClassErr = err
		return
	}

	instance, _, _ := procGetModuleHandle.Call(0)
	class := wndClassEx{
		size:      uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:   syscall.NewCallback(clipWndProc),
		instance:  instance,
		className: name,
	}
	if atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		clipClassErr = fmt.Errorf("webview: cannot register the clip window class: %w", err)
		return
	}
	runtime.KeepAlive(class)
	clipClassName = name
}

func clipWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	ret, _, _ := procDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return ret
}

// createClipWindow creates the child window the controller is parented to. It
// starts hidden: the host makes it visible once it has placed it.
func createClipWindow(parent uintptr) (uintptr, error) {
	clipClassOnce.Do(registerClipClass)
	if clipClassErr != nil {
		return 0, clipClassErr
	}

	// Keep the parent from painting over its children. Bellui draws the canvas
	// into this window with a GPU surface, and without the style the driver's
	// own painting can flash over the hole the browser occupies.
	if style, _, _ := procGetWindowLong.Call(parent, gwlStyle); style != 0 {
		procSetWindowLong.Call(parent, gwlStyle, style|wsClipChildren)
	}

	instance, _, _ := procGetModuleHandle.Call(0)
	hwnd, _, err := procCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(clipClassName)), 0,
		wsChild|wsClipChildren|wsClipSiblings,
		0, 0, 0, 0,
		parent, 0, instance, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("webview: cannot create the clip window: %w", err)
	}
	return hwnd, nil
}
