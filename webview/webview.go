// Package webview embeds the platform's own web engine in a bellui layout.
//
// It is the first consumer of [widget.NativeViewHost] and inherits every one
// of that host's limitations — most importantly that the web content is an
// OS view drawn *over* bellui's surface, so nothing bellui draws can appear
// above it and [core.Canvas.Capture] does not see it. Read the
// [widget.NativeViewHost] documentation before using this package.
//
// The package is an optional import: the platform web engine is only linked
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
//	Web        an <iframe> positioned over the canvas
//	Windows    not yet — WebView2 is tracked in #65
//	Linux      not supported, and not a bug to be fixed: the engine that
//	           could be embedded is LGPL, which is outside this project's
//	           allowed dependency set, and Wayland has no cross-toolkit
//	           embedding primitive. See #60 for the alternatives weighed.
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
//   - On Android a script that throws reports a null result rather than an
//     error, because evaluateJavascript has no channel for an exception.
//
// # Talking to the page
//
// Two channels connect bellui logic and page logic, and the hole punch
// constrains neither: it limits what can be drawn over the view and who owns
// the keyboard, not what can be exchanged with it.
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

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/widget"
)

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

// NewHost returns a [widget.NativeViewHost] embedding a new web view for url.
// It is the one-call form of wrapping [New] in a host.
//
// Since: 2.19
func NewHost(url string) (*widget.NativeViewHost, *WebView) {
	w := New(url)
	return widget.NewNativeViewHost(w), w
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
