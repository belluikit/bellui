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
// Since: 2.19
package webview

import (
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

	// OnMessage is called when the page posts a message to bellui, e.g.
	//
	//	window.webkit.messageHandlers.bellui.postMessage("hello")   // macOS
	//
	// It runs on the main goroutine. Set it before [WebView.Attach] (that is,
	// before the host is laid out) so no early message is missed.
	//
	// With [WebView.Eval] going the other way, bellui logic and page logic can
	// drive each other freely. The hole punch constrains what can be drawn over
	// the view and who owns the keyboard — it does not constrain data flow.
	OnMessage func(string)
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

// Assert the platform implementation satisfies the host's backend contract,
// including the optional focus capability the host bridges to canvas focus.
var (
	_ widget.NativeView        = (*WebView)(nil)
	_ widget.NativeViewFocuser = (*WebView)(nil)
)
