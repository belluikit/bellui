//go:build wasm

package webview

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// Supported reports whether this platform has a web engine binding.
//
// Since: 2.19
const Supported = true

// The web target is the one platform where the "native view" is already a web
// engine: the content goes in an <iframe> positioned over bellui's <canvas>.
// That makes it the cheapest backend to host and the most constrained one to
// talk to, because everything this package does to a page — inject the bridge,
// evaluate script, read session history, veto a navigation — is governed by the
// browser's same-origin policy rather than by us.
//
// What that costs, stated plainly rather than worked around:
//
//   - Eval, EvalResult, EvalFunc and Call reach same-origin content only.
//     Against a cross-origin page the browser refuses, and they report that
//     error rather than hanging.
//   - The bridge likewise needs same-origin content, since the page-side shim
//     has to be injected into the frame. A cross-origin page never gets a
//     window.bellui, which is also the safe default.
//   - The shim is injected when the frame's document has loaded, not at
//     document start: a browser offers no user-script hook. A page's own first
//     lines therefore run before window.bellui exists — use OnLoadChanged, or
//     LoadHTML, whose content carries the shim in its own markup.
//   - OnNavigate is consulted for navigations this package starts. A browser
//     exposes no navigation-policy hook for an iframe, so a link the user
//     follows inside the page cannot be vetoed; constrain such content with the
//     frame's own sandboxing instead.
//   - CanGoBack and CanGoForward always report false. An iframe shares the
//     top-level session history, and neither the browser nor this package can
//     tell whether the next entry belongs to the frame. GoBack and GoForward do
//     work for same-origin content.
//
// Some sites cannot be framed at all — X-Frame-Options or a CSP
// frame-ancestors directive makes the browser refuse — which is the site's
// decision and shows up as a frame that stays blank.

// webView is the wasm platform state: the <iframe> holding the content, the
// clipping wrapper around it, and the bellui <canvas> the pair is positioned
// against.
//
// Unlike the other backends there is no handle registry: a browser callback is
// a Go closure, so each listener simply captures its own view. The js.Func
// values are kept so Destroy can release them — an unreleased one leaks its
// closure for the life of the page.
type webView struct {
	url string

	frame  js.Value // the <iframe> showing the content
	clip   js.Value // wrapper div with overflow:hidden, the native clip
	canvas js.Value // bellui's canvas, the origin geometry is relative to

	onLoad    js.Func
	onMessage js.Func

	// shim is the page-side bridge script, empty when the bridge is disabled.
	// It is captured at attach because that is when the bridge is installed.
	shim string
}

func (w *WebView) init() {}

// attachRaw creates the iframe over bellui's canvas.
//
// win is unused: the web target draws every bellui window into the single
// canvas the page owns — extra windows are emulated inside it as in-canvas
// InnerWindows (#499) — so there is exactly one surface to position against,
// and the host's canvas-absolute geometry is already in its coordinate space.
func (w *WebView) attachRaw(win bellui.Window) error {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return errors.New("webview: no document")
	}
	canvas := doc.Call("querySelector", "canvas")
	if !canvas.Truthy() {
		return errors.New("webview: no bellui canvas in the document")
	}
	w.canvas = canvas

	w.clip = doc.Call("createElement", "div")
	setStyle(w.clip, map[string]string{
		"position": "fixed",
		"overflow": "hidden",
		"padding":  "0",
		"margin":   "0",
		"border":   "0",
		// Above the canvas: a hosted native view is drawn over bellui's
		// surface, never composited into it.
		"z-index": "1000",
	})

	w.frame = doc.Call("createElement", "iframe")
	setStyle(w.frame, map[string]string{
		"position": "absolute",
		"border":   "0",
		"margin":   "0",
		"padding":  "0",
	})
	w.clip.Call("appendChild", w.frame)
	doc.Get("body").Call("appendChild", w.clip)

	// The bridge is installed here and only here, so a view attached with it
	// disabled has no page-side channel at all rather than an ignored one.
	w.shim = w.bridgeScript("window.parent.postMessage(s, '*');")

	w.onLoad = js.FuncOf(func(js.Value, []js.Value) any {
		w.frameLoaded()
		return nil
	})
	w.frame.Call("addEventListener", "load", w.onLoad)

	if w.shim != "" {
		w.onMessage = js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 {
				w.pageMessage(args[0])
			}
			return nil
		})
		js.Global().Call("addEventListener", "message", w.onMessage)
	}
	return nil
}

func (w *WebView) attachedRaw() bool { return w.frame.Truthy() }

func (w *WebView) destroyRaw() {
	if !w.frame.Truthy() {
		return
	}
	w.frame.Call("removeEventListener", "load", w.onLoad)
	w.onLoad.Release()
	if w.onMessage.Truthy() {
		js.Global().Call("removeEventListener", "message", w.onMessage)
		w.onMessage.Release()
	}
	if parent := w.clip.Get("parentNode"); parent.Truthy() {
		parent.Call("removeChild", w.clip)
	}
	w.frame, w.clip, w.canvas = js.Undefined(), js.Undefined(), js.Undefined()
	w.shim = ""
}

// ---- page callbacks ----

// frameLoaded runs when the frame's document has finished loading. It is both
// where the bridge shim is (re)installed — so it survives navigation, exactly
// as a document-start user script would elsewhere — and where a completed load
// is reported.
func (w *WebView) frameLoaded() {
	if w.shim != "" {
		if _, err := w.evalIn(w.shim); err != nil {
			// Cross-origin content cannot be injected into. That is the browser
			// enforcing the same-origin policy, not a failure to report: such a
			// page simply has no bridge.
			bellui.LogError("webview: bridge unavailable for cross-origin content", err)
		}
	}
	w.deliverLoad(LoadStateComplete, w.currentURL(), "")
}

// pageMessage handles a window "message" event, which is how the page-side shim
// reaches us on this platform.
func (w *WebView) pageMessage(event js.Value) {
	// window receives every frame's messages, and any script on the page can
	// post one. Only our own frame's are ours: without this check another frame
	// could impersonate the page and reach the app's Handle functions with an
	// origin it does not have.
	if src := event.Get("source"); !src.Truthy() || !src.Equal(w.frame.Get("contentWindow")) {
		return
	}
	data := event.Get("data")
	if data.Type() != js.TypeString {
		return // not one of ours; the shim always posts a JSON string
	}
	// The origin is the browser's own attribution of the sending frame, never
	// anything the message itself claims — which is what makes the OriginPolicy
	// meaningful for framed content.
	w.deliverMessage(event.Get("origin").String(), data.String())
}

// ---- eval ----

func (w *WebView) evalRaw(script string) {
	if _, err := w.evalIn(script); err != nil {
		bellui.LogError("webview: Eval", err)
	}
}

func (w *WebView) evalResultRaw(script string, callID uint64) {
	v, err := w.evalIn(script)
	if err != nil {
		w.deliverEvalResult(callID, "", err.Error())
		return
	}
	encoded, err := jsonStringify(v)
	if err != nil {
		w.deliverEvalResult(callID, "", err.Error())
		return
	}
	w.deliverEvalResult(callID, encoded, "")
}

// evalIn runs script inside the frame and returns its completion value. It
// fails when the frame has no document yet, and when the content is
// cross-origin — the browser denies access to another origin's window, which
// surfaces here as an error rather than as a silently discarded script.
func (w *WebView) evalIn(script string) (v js.Value, err error) {
	if !w.frame.Truthy() {
		return js.Undefined(), ErrNotAttached
	}
	defer catchJS(&err)

	win := w.frame.Get("contentWindow")
	if !win.Truthy() {
		return js.Undefined(), ErrNotAttached
	}
	return win.Call("eval", script), nil
}

func jsonStringify(v js.Value) (s string, err error) {
	defer catchJS(&err)

	out := js.Global().Get("JSON").Call("stringify", v)
	if out.Type() != js.TypeString {
		// undefined, a function, or a symbol: no JSON representation, which the
		// eval contract reports as the null literal.
		return "null", nil
	}
	return out.String(), nil
}

// catchJS turns a JavaScript exception — which syscall/js delivers as a panic —
// into an error on *err. Every call this package makes into the page can throw:
// a SecurityError for cross-origin access, a TypeError for a circular value, or
// whatever the evaluated script itself raises.
func catchJS(err *error) {
	r := recover()
	if r == nil {
		return
	}
	if jsErr, ok := r.(js.Error); ok {
		*err = errors.New(strings.TrimPrefix(jsErr.Error(), "JavaScript error: "))
		return
	}
	*err = fmt.Errorf("webview: %v", r)
}

// ---- navigation ----

func (w *WebView) loadURLRaw(url string) {
	if !w.frame.Truthy() {
		return
	}
	// A browser offers no navigation-policy hook for an iframe, so the loads
	// this package starts are the ones OnNavigate can still see. Asking here
	// keeps a programmatic LoadURL vetoable, matching the other backends.
	if !w.decideNavigation(NavigationRequest{URL: url, Kind: NavigationOther, MainFrame: true}) {
		return
	}
	w.frame.Call("removeAttribute", "srcdoc")
	w.frame.Set("src", url)
	w.deliverLoad(LoadStateLoading, url, "")
}

func (w *WebView) loadHTMLRaw(html, baseURL string) {
	if !w.frame.Truthy() {
		return
	}
	// srcdoc is the only way to hand literal markup to a frame, and it carries
	// the bridge in the document itself — so unlike a loaded URL the page really
	// does get window.bellui before its own first line runs.
	var b strings.Builder
	if baseURL != "" {
		b.WriteString(`<base href="` + escapeAttr(baseURL) + `">`)
	}
	if w.shim != "" {
		b.WriteString("<script>" + w.shim + "</script>")
	}
	b.WriteString(html)

	w.frame.Set("srcdoc", b.String())
	w.deliverLoad(LoadStateLoading, baseURL, "")
}

func (w *WebView) navRaw(op navOp) bool {
	if !w.frame.Truthy() {
		return false
	}
	switch op {
	case navCanGoBack, navCanGoForward:
		// An iframe shares the top-level session history, so history.length
		// counts the parent's entries too and says nothing about whether the
		// frame itself can move. Reporting false is the honest answer; GoBack
		// and GoForward still work for same-origin content.
		return false
	case navGoBack:
		return w.historyCall("back")
	case navGoForward:
		return w.historyCall("forward")
	case navReload:
		// Re-assigning the frame's own source reloads whatever it is showing
		// without needing access to its document, so this works cross-origin
		// too. Literal content lives in srcdoc rather than src.
		if src := w.frame.Get("src"); src.Truthy() && src.String() != "" {
			url := src.String()
			w.frame.Set("src", url)
			w.deliverLoad(LoadStateLoading, url, "")
			return true
		}
		if doc := w.frame.Get("srcdoc"); doc.Truthy() && doc.String() != "" {
			w.frame.Set("srcdoc", doc.String())
			w.deliverLoad(LoadStateLoading, w.url, "")
			return true
		}
		return false
	case navStopLoading:
		return w.frameWindowCall("stop")
	}
	return false
}

// historyCall moves the frame through session history. It needs same-origin
// content, since another origin's history object is not reachable — a denial
// arrives as a JavaScript exception, which leaves the named result false.
func (w *WebView) historyCall(name string) (ok bool) {
	var ignored error
	defer catchJS(&ignored)

	win := w.frame.Get("contentWindow")
	if !win.Truthy() {
		return false
	}
	win.Get("history").Call(name)
	return true
}

// frameWindowCall invokes a method on the framed window, reporting whether it
// was reachable — cross-origin content is not.
func (w *WebView) frameWindowCall(name string) (ok bool) {
	var ignored error
	defer catchJS(&ignored)

	win := w.frame.Get("contentWindow")
	if !win.Truthy() {
		return false
	}
	win.Call(name)
	return true
}

// currentURL reports where the frame actually is, falling back to the address
// it was asked for when the content is cross-origin and will not say.
func (w *WebView) currentURL() (url string) {
	url = w.url // the fallback, kept if reading the frame's location throws

	var ignored error
	defer catchJS(&ignored)

	if win := w.frame.Get("contentWindow"); win.Truthy() {
		if href := win.Get("location").Get("href"); href.Type() == js.TypeString {
			return href.String()
		}
	}
	return url
}

// ---- geometry, visibility and focus ----

// SetGeometry places the clip wrapper and the frame inside it.
//
// Bellui works in device-independent points and CSS works in CSS pixels, which
// differ by the ratio between the canvas scale and the browser's device pixel
// ratio — normally 1, since bellui derives its wasm scale from that same ratio,
// but not when the app has scaled itself. Positions are viewport-relative
// (the wrapper is position:fixed) and taken from the canvas's own client rect,
// so a page that puts other content around the canvas still lines up.
func (w *WebView) SetGeometry(rect, clip geo.Rectangle, scale float32) {
	if !w.frame.Truthy() {
		return
	}
	px := float64(scale) / js.Global().Get("devicePixelRatio").Float()
	if px == 0 || px != px { // a zero or NaN ratio would place the view nowhere
		px = 1
	}

	origin := w.canvas.Call("getBoundingClientRect")
	ox, oy := origin.Get("left").Float(), origin.Get("top").Float()

	setStyle(w.clip, map[string]string{
		"left":   cssPx(ox + float64(clip.Pos.X)*px),
		"top":    cssPx(oy + float64(clip.Pos.Y)*px),
		"width":  cssPx(float64(clip.Size.Width) * px),
		"height": cssPx(float64(clip.Size.Height) * px),
	})
	// The frame is positioned inside the wrapper, so the part of it that falls
	// outside the clip is masked by overflow:hidden — the native clip the host
	// requires, without which the content would overflow an enclosing Scroll.
	setStyle(w.frame, map[string]string{
		"left":   cssPx((float64(rect.Pos.X) - float64(clip.Pos.X)) * px),
		"top":    cssPx((float64(rect.Pos.Y) - float64(clip.Pos.Y)) * px),
		"width":  cssPx(float64(rect.Size.Width) * px),
		"height": cssPx(float64(rect.Size.Height) * px),
	})
}

// SetVisible shows or hides the frame.
func (w *WebView) SetVisible(visible bool) {
	if !w.clip.Truthy() {
		return
	}
	display := "none"
	if visible {
		display = "block"
	}
	w.clip.Get("style").Call("setProperty", "display", display)
}

// TakeFocus gives the page keyboard focus, as a click inside it would.
//
// While the frame holds the focus, keystrokes and input-method composition go
// to the page and bellui's canvas receives nothing. A hosted view is kept in
// step with bellui's own focus by [widget.NativeViewHost], which drives these
// three methods; call them directly only on an unhosted view.
func (w *WebView) TakeFocus() bool {
	if !w.frame.Truthy() {
		return false
	}
	w.frame.Call("focus")
	return w.HasFocus()
}

// ResignFocus hands the keyboard focus back to bellui's canvas, so that keys
// and composition reach it again. It reports whether the frame gave the focus
// up; a frame that does not hold it reports success and changes nothing.
func (w *WebView) ResignFocus() bool {
	if !w.frame.Truthy() {
		return false
	}
	if !w.HasFocus() {
		return true
	}
	w.frame.Call("blur")
	if w.canvas.Truthy() && w.canvas.Get("focus").Truthy() {
		w.canvas.Call("focus")
	}
	return !w.HasFocus()
}

// HasFocus reports whether the keyboard focus is inside the page, meaning
// bellui is not receiving key events. A browser reports focus within a frame as
// the frame element being the document's active element.
func (w *WebView) HasFocus() bool {
	if !w.frame.Truthy() {
		return false
	}
	active := js.Global().Get("document").Get("activeElement")
	return active.Truthy() && active.Equal(w.frame)
}

// ---- small DOM helpers ----

func setStyle(el js.Value, props map[string]string) {
	style := el.Get("style")
	for name, value := range props {
		style.Call("setProperty", name, value)
	}
}

func cssPx(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64) + "px"
}

// escapeAttr quotes a value for use inside a double-quoted HTML attribute. It
// is used for the <base href> of literal content, whose baseURL comes from the
// app rather than the page, but which still must not be able to close the
// attribute and inject markup.
func escapeAttr(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		`"`, "&quot;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}
