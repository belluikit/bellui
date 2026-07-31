// Package webview embeds a web engine in a bellui layout.
//
// It spans two hosting models, chosen per platform by [NewHost]. On macOS,
// iOS, Android, Windows and the web target the engine is an OS view drawn
// *over* bellui's surface through [widget.NativeViewHost], with every one of
// that host's documented limitations — nothing bellui draws can appear above
// the content, and [core.Canvas.Capture] does not see it; read the
// [widget.NativeViewHost] documentation before designing around those
// targets. On Linux the engine (CEF) renders offscreen and the content is
// composited *inside* bellui's canvas: overlays, dialogs and menus draw
// above it, Capture sees it, clipping and opacity apply, and the host is a
// Tab stop, because bellui owns the keyboard rather than the OS.
//
// The package is an optional import: the web engine binding is only linked
// into binaries that import it, so an app that embeds no web content pays
// nothing.
//
// Support is per-platform — see [Supported]. On an unsupported platform
// [New] still returns a usable value whose [WebView.Attach] fails, leaving
// the host inert (it occupies its space and draws nothing) rather than
// panicking, so a cross-platform app compiles and runs everywhere.
//
// # Platforms
//
//	macOS      WKWebView, hosted in the GLFW content view
//	iOS        WKWebView, hosted in the app window beside the GLKView
//	Android    android.webkit.WebView, hosted on the decor view
//	Windows    WebView2, hosted in a child window of the app window
//	Web        an <iframe> positioned over the canvas
//	Linux      CEF, bundled by the app, rendered offscreen and composited
//
// Every backend except Linux uses the platform's **own** engine, so no web
// engine is bundled into those binaries and security updates stay with the
// OS vendor; Windows's engine is a separate OS component rather than part of
// the OS image — see [ErrRuntimeMissing].
//
// Linux is the deliberate exception to that policy, because it is the one
// target with no engine an app can lean on: WebKitGTK is LGPL (outside this
// project's allowed dependency set) and needs a GTK main loop, WPE — measured
// in #60 — is unbundled, ABI-coupled to the distro and absent from Ubuntu
// since 22.04, and no distribution packages CEF. So on Linux **the
// application bundles the engine**: `bellui package` lays the pinned CEF
// payload out beside the binary, this package finds it at run time
// (BELLUI_CEF_DIR overrides the search), and a binary running without it
// reports [ErrRuntimeMissing] and degrades to an inert host. The app owner
// accepts the payload size and the Chromium update cadence — that trade was
// made knowingly (#60, #105), and what it buys is the composited model
// described above.
//
// The browsing profile — cookies, localStorage, the HTTP cache — persists by
// default at "webview-cef" under the app's [bellui.Storage] root, matching
// what every other backend's engine does in its own platform-managed
// location, so a login survives a restart everywhere. [SetCEFProfileDir]
// moves it; [SetCEFProfileInMemory] opts out and discards it on exit. Both
// must run before the first [WebView.Attach], which is when the engine
// initializes, once per process.
//
// # Supplying a Linux backend yourself
//
// The built-in backend is not the only option. [widget.NativeView] is a
// public interface with no unexported methods, so an application — or a
// third-party module — that wants a different engine can implement one and
// host it with [widget.NewNativeViewHost], with no change to bellui:
//
//	view := myengine.New(...)       // implements widget.NativeView
//	host := widget.NewNativeViewHost(view)
//
// Whoever supplies it pins their own engine version, owns their own update
// cadence, and ships it in their own Flatpak/AppImage/tarball. Such a
// backend must satisfy the whole [widget.NativeView] contract — including a
// native clip in SetGeometry, or the view will overflow an enclosing
// [container.Scroll] — and should implement [widget.NativeViewFocuser] as
// well, or bellui's focus and the OS keyboard focus will drift apart. This
// package is not involved: a supplied backend is used through the host
// directly, and being window-hosted it carries the hole punch's limitations,
// not the composited model's.
//
// Each platform bends the API where its engine does. Every such difference is
// documented on the backend that has it, and the ones worth knowing before
// choosing a design are:
//
//   - On the web target the bridge, [WebView.Eval] and [WebView.Call] reach
//     same-origin content only, [WebView.OnNavigate] sees only the
//     navigations this package starts, and [WebView.CanGoBack] always reports
//     false. All three are browser policy for framed content.
//   - On the web target and Android the page-side bridge is injected when the
//     document loads rather than before it runs, so a page's own first lines
//     may not see window.bellui. [WebView.LoadHTML] on the web target is the
//     exception: the shim travels in the content itself.
//   - On Android and Windows a script that throws reports a null result rather
//     than an error, because neither evaluateJavascript nor ExecuteScript has a
//     channel for an exception. A [WebView.Call] made against a page with no
//     shim therefore stays pending on those two until the view is destroyed,
//     instead of failing at once.
//   - On Windows and Linux [WebView.LoadHTML] ignores its baseURL — WebView2's
//     NavigateToString takes no base, and CEF has no literal-content API at
//     all so the content travels as a data: URL — so the content's relative
//     references do not resolve and its origin is always [OriginOpaque].
//   - On Windows a navigation the user started is reported as
//     [NavigationLinkClicked] whatever they did to start it, including
//     submitting a form: WebView2 reports that the user initiated it, not how.
//   - On Linux the host is a Tab stop, but the page's *own* Tab order is not
//     keyboard-reachable — the canvas consumes Tab for focus traversal before
//     the page sees it (#126) — and IME composition is not yet forwarded, so
//     CJK text cannot be typed into the page (#121). Both are properties of
//     bellui owning the keyboard, which is also what makes the host
//     focusable at all.
//
// # Talking to the page
//
// Two channels connect bellui logic and page logic, and neither hosting
// model constrains them: the models differ in what can be drawn over the
// view and who owns the keyboard, not in what can be exchanged with it.
//
//   - [WebView.Eval] and friends run JavaScript in the page, optionally
//     reporting its result.
//   - The bridge — [WebView.EnableBridge], [WebView.Handle], [WebView.Call],
//     [WebView.Post] and [WebView.OnMessage] — is a two-way, JSON-typed,
//     request/response channel with a page-side `window.bellui` object.
//
// The bridge is **off until it is enabled**, and enabling it takes an
// [OriginPolicy] naming the content that may use it. See [WebView.EnableBridge]
// for why untrusted remote content must not be handed a message channel.
//
// # Navigation
//
// [WebView.LoadURL], [WebView.LoadHTML], [WebView.GoBack],
// [WebView.GoForward], [WebView.Reload] and [WebView.StopLoading] drive the
// engine; [WebView.OnLoadChanged] reports progress and failure, and
// [WebView.OnNavigate] can veto a navigation before it starts.
//
// # Threading
//
// Every method must be called on the main goroutine, and every callback this
// package makes — messages, results, load events, navigation decisions — runs
// on it too.
//
// Since: 2.19
package webview

import (
	"encoding/json"
	"errors"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/widget"
)

// ErrRuntimeMissing is reported by [WebView.Attach] when the platform's web
// engine exists as a separate component and that component is not there. Two
// platforms report it, for different reasons:
//
//   - On Windows it means no WebView2 Evergreen Runtime is installed, or the
//     installed one is older than this package requires. The runtime ships
//     with Windows 11 and with Microsoft Edge, but a Windows Server or LTSC
//     image may not have one — an app that must work everywhere should ship
//     the Evergreen Bootstrapper alongside it
//     (https://developer.microsoft.com/microsoft-edge/webview2/).
//   - On Linux it means no bundled CEF payload was found. CEF is not a
//     system component on any distribution — it travels with the application
//     (`bellui package` lays it out; BELLUI_CEF_DIR overrides the search),
//     so this is the error an unpackaged binary reports on a machine that
//     never received the payload.
//
// Either way Attach failing leaves the host inert — it occupies its space
// and draws nothing — exactly as on a platform with no binding at all, so
// handling the error is a chance to offer the content another way, not a
// requirement for the app to keep running.
//
// It is declared on every platform so that a cross-platform app can test for
// it with [errors.Is] without a build tag; no other platform reports it.
//
// Since: 2.19
var ErrRuntimeMissing = errors.New("webview: the web engine runtime is not installed")

// WebView is a platform web engine hosted in a bellui layout. Create one with
// [New] and place it with [NewHost] (or hand it to
// [widget.NewNativeViewHost] yourself).
//
// All methods must be called on the main goroutine.
//
// Since: 2.19
type WebView struct {
	webView // platform implementation

	// OnMessage is called when the page posts a one-way notification:
	//
	//	window.bellui.post({hello: "world"})
	//
	// It runs on the main goroutine. It is only ever called while the bridge
	// is enabled and the sending page passed the [OriginPolicy]; a message
	// that arrives with no OnMessage set is dropped.
	//
	// Use [WebView.Handle] instead when the page expects an answer.
	OnMessage func(Message)

	// OnLoadChanged reports every change of [WebView.LoadState] for the main
	// frame: a navigation starting, finishing, or failing. It runs on the
	// main goroutine.
	OnLoadChanged func(LoadEvent)

	// OnNavigate is consulted before each navigation the engine is about to
	// start, and cancels it by returning false. A nil hook allows everything.
	//
	// It is the place to keep a view pinned to content you trust — pair it
	// with the bridge's [OriginPolicy], which decides who may *talk* to
	// bellui, where this decides who may be *shown*. It runs on the main
	// goroutine and must return promptly: the engine is waiting on it.
	OnNavigate func(NavigationRequest) bool

	bridge bridgeState
	eval   evalState
	load   LoadState

	// evalHook replaces the platform eval call in tests, so the whole bridge
	// is exercisable on a target with no web engine.
	evalHook func(js string, callID uint64, wantResult bool)
}

// New returns a web view that will load url once it is attached to a window.
// An empty url leaves the engine on a blank page.
//
// Since: 2.19
func New(url string) *WebView {
	w := &WebView{}
	w.url = url
	w.init()
	return w
}

// URL returns the URL the view was last asked to load. It is stored with the
// platform state, since a backend loads it as soon as it attaches.
func (w *WebView) URL() string {
	return w.url
}

// Attach creates the platform web view inside the given window and loads the
// URL the view was created with, if any.
//
// The bridge is installed here, so [WebView.EnableBridge] has to have been
// called first: a view attached with the bridge off has no page-side
// `window.bellui` object and no message channel behind it at all.
func (w *WebView) Attach(win bellui.Window) error {
	if err := w.attachRaw(win); err != nil {
		return err
	}
	if w.url != "" {
		w.loadURLRaw(w.url)
	}
	return nil
}

// Destroy removes the native view from the window and releases it.
//
// Work still in flight is not left hanging: every pending [WebView.Call] and
// [WebView.EvalResult] callback is invoked with [ErrNotAttached] rather than
// never being called at all. Registered [WebView.Handle] functions are kept,
// so a view can be attached to another window and keep working; further calls
// into a destroyed view fail immediately with the same error.
func (w *WebView) Destroy() {
	w.destroyRaw()
	w.failPending(ErrNotAttached)
}

// Assert the platform implementation satisfies the host's backend contract,
// including the optional focus capability the host bridges to canvas focus.
var (
	_ widget.NativeView        = (*WebView)(nil)
	_ widget.NativeViewFocuser = (*WebView)(nil)
)

// attached reports whether there is a live engine to talk to. It is the one
// predicate the portable half uses, so a test can stand in for the platform
// by installing evalHook and exercise the whole bridge on a target that has
// no web engine at all.
func (w *WebView) attached() bool {
	return w.evalHook != nil || w.attachedRaw()
}

// jsonOrNull marshals v, returning the JSON null literal when it cannot be
// encoded. Callers that care about the failure check the error.
func jsonOrNull(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage("null"), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null"), err
	}
	return data, nil
}
