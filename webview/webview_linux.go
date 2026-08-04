//go:build linux && !android && cgo

package webview

// The Linux backend, on CEF offscreen rendering (#105, reversing #60's "no
// backend" with the trigger that decision recorded).
//
// Unlike every other backend this one is not a hole punch: CEF renders the
// page offscreen and hands frames to bellui, which composites them like any
// other widget (see host_offscreen_linux.go). This file is the seam half —
// the platform methods webview/platform.go names — plus the engine handler
// that receives everything gocef reports.
//
// # The bridge rides the DevTools protocol
//
// The portable bridge needs document-start injection, a page→host channel and
// eval-with-result. CEF's native route for those needs renderer-process code
// (a CefV8Handler; CefMessageRouter is C++-only and absent from the C API),
// but CDP does all three from the browser process:
//
//   - Page.addScriptToEvaluateOnNewDocument injects the shim at document
//     start — earlier than Android and wasm, which can only inject at load.
//   - Runtime.addBinding gives every frame a window.<name>(string) function,
//     the shim's /*POST*/ transport; Runtime.bindingCalled carries the
//     execution context id, and Runtime.executionContextCreated events carry
//     each context's origin, which is what the OriginPolicy is fed.
//   - Runtime.evaluate reports exceptionDetails, so a throwing script is an
//     error here rather than the null Windows and Android report (#83).
//
// With the bridge off, none of that is installed: no Runtime.enable, no
// binding, no script — the page has no channel at all, per the contract.
// Navigation stays on CEF's native API (gocef's NavigationHandler and
// LoadHandler), not CDP.
//
// # Creation is asynchronous
//
// A CEF browser exists only after its Created callback, and the bridge is
// only installed once the CDP setup commands are acknowledged. Everything
// asked of the view before that moment — the initial URL, evals, LoadHTML —
// is queued and replayed in order, exactly the Windows backend's answer to
// the same shape of problem.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/gocef"
)

// Supported reports whether this platform has a web engine binding.
//
// Since: 1.0
const Supported = true

// cefBindingName is the page-side function Runtime.addBinding installs; the
// bridge shim posts its envelopes through it. Namespaced so a page cannot
// plausibly collide with it by accident.
const cefBindingName = "__belluiCefPost"

// webView holds the Linux platform state: the engine handler that owns the
// gocef browser, and the URL the portable half reads back through URL().
type webView struct {
	url string
	eng *cefEngine
}

func (w *WebView) init() {
	w.eng = &cefEngine{view: w}
}

// attachRaw brings the CEF runtime up if needed and requests the browser.
// The window is unused: an offscreen browser has no native parent, which is
// the whole reason this backend exists on Linux.
func (w *WebView) attachRaw(bellui.Window) error {
	e := w.eng
	if e.browser != nil {
		return nil
	}
	if err := ensureCEFRuntime(); err != nil {
		return err
	}

	vw, vh := e.createSize()
	cefBrowserCreated() // the pump must run for creation to complete
	b, err := gocef.NewBrowser("about:blank", vw, vh, false, e)
	if err != nil {
		cefBrowserClosed()
		return err
	}
	e.browser = b
	return nil
}

func (w *WebView) attachedRaw() bool { return w.eng.browser != nil }

// destroyRaw closes the browser and resets the engine for a possible
// re-attach. The Closed callback — which balances the pump count — arrives
// later, through the pump; state that must not serve a stale callback is
// cleared here rather than there.
func (w *WebView) destroyRaw() {
	e := w.eng
	if e.browser == nil {
		return
	}
	b := e.browser
	e.browser = nil
	e.ready = false
	e.pending = nil
	e.cdpEvals = nil
	e.cdpSetup = nil
	e.contexts = nil
	b.Close()
}

func (w *WebView) loadURLRaw(url string) {
	e := w.eng
	e.run(func() { e.browser.LoadURL(url) })
}

// loadHTMLRaw displays literal content as a data: URL — CEF has no
// load-string API. baseURL is ignored: a data: URL resolves nothing relative
// and always has the opaque origin, the same shape as the Windows backend's
// NavigateToString limitation, and documented on [WebView.LoadHTML].
func (w *WebView) loadHTMLRaw(html, _ string) {
	url := "data:text/html;charset=utf-8;base64," +
		base64.StdEncoding.EncodeToString([]byte(html))
	e := w.eng
	e.run(func() { e.browser.LoadURL(url) })
}

func (w *WebView) navRaw(op navOp) bool {
	b := w.eng.browser
	if b == nil {
		return false
	}
	switch op {
	case navCanGoBack:
		return b.CanGoBack()
	case navCanGoForward:
		return b.CanGoForward()
	case navGoBack:
		if !b.CanGoBack() {
			return false
		}
		b.GoBack()
		return true
	case navGoForward:
		if !b.CanGoForward() {
			return false
		}
		b.GoForward()
		return true
	case navReload:
		b.Reload(false)
		return true
	case navStopLoading:
		b.StopLoad()
		return true
	}
	return false
}

func (w *WebView) evalRaw(js string) {
	w.eng.evaluate(js, 0, false)
}

func (w *WebView) evalResultRaw(js string, callID uint64) {
	w.eng.evaluate(js, callID, true)
}

// SetGeometry is a no-op: the composited host is a real widget, so layout,
// clipping and scrolling reach the content without a platform call.
func (w *WebView) SetGeometry(rect, clip geo.Rectangle, scale float32) {}

// SetVisible drives CEF's WasHidden, which stops compositing while hidden.
func (w *WebView) SetVisible(visible bool) {
	if b := w.eng.browser; b != nil {
		b.SetVisible(visible)
	}
}

// TakeFocus focuses the composited host in bellui's own canvas — on this
// backend bellui owns the keyboard, so "native focus" is canvas focus on the
// widget that forwards keys to the page. Unhosted views just tell Blink.
func (w *WebView) TakeFocus() bool {
	if h := w.eng.host; h != nil {
		return h.focusSelf()
	}
	if b := w.eng.browser; b != nil {
		b.SetFocus(true)
		w.eng.rawFocus = true
		return true
	}
	return false
}

// ResignFocus unfocuses the host, so keys stop reaching the page.
func (w *WebView) ResignFocus() bool {
	if h := w.eng.host; h != nil {
		h.unfocusSelf()
		return true
	}
	if b := w.eng.browser; b != nil {
		b.SetFocus(false)
	}
	w.eng.rawFocus = false
	return true
}

// HasFocus reports whether keys are currently being forwarded to the page.
func (w *WebView) HasFocus() bool {
	if h := w.eng.host; h != nil {
		return h.focused
	}
	return w.eng.rawFocus
}

// ---------------------------------------------------------------------------
// The engine handler.

// cefEngine implements gocef.Handler and its optional capabilities. It is a
// separate type rather than methods on webView so that gocef's exported
// callback names (Paint, Created, ViewSize…) do not become public API on
// WebView. Every method runs on the main goroutine: gocef delivers callbacks
// inside gocef.Work, which the pump animation ticks there.
type cefEngine struct {
	view *WebView
	host *offscreenHost // nil for an unhosted view

	browser *gocef.Browser

	// ready gates the queue: true once the browser exists and the bridge's
	// CDP setup (if any) is acknowledged, so injected scripts always precede
	// the first real navigation.
	ready   bool
	pending []func()

	// rawFocus is the focus shadow for a view with no host.
	rawFocus bool

	// failedNav marks that the current navigation already reported
	// LoadStateFailed, so the loading→false edge that follows every failure
	// is not also reported as complete.
	failedNav bool

	// CDP correlation: our message id → what the reply means.
	cdpNext  uint64
	cdpEvals map[uint64]uint64 // cdp id → portable eval call id
	cdpSetup map[uint64]bool   // outstanding bridge-setup commands
	contexts map[uint64]string // execution context id → serialized origin
}

// createSize is the size the browser is created at, in points. A hosted view
// reads the host; before layout (or unhosted) a plausible default keeps CEF's
// compositor happy until the first real resize.
func (e *cefEngine) createSize() (int, int) {
	if e.host != nil {
		if w, h := e.host.viewSize(); w > 0 && h > 0 {
			return w, h
		}
	}
	return 640, 480
}

// run executes f now, or queues it until the engine is ready. The queue is
// what makes Attach-then-LoadURL-then-Eval behave identically here and on the
// synchronous backends.
func (e *cefEngine) run(f func()) {
	if e.ready && e.browser != nil {
		f()
		return
	}
	e.pending = append(e.pending, f)
}

func (e *cefEngine) becomeReady() {
	e.ready = true
	pending := e.pending
	e.pending = nil
	for _, f := range pending {
		if e.browser == nil {
			return // destroyed by an earlier callback
		}
		f()
	}
}

// evaluate is the one path both evalRaw and evalResultRaw use. Delivery
// failure must fail the callback — every callback this package accepts is
// called exactly once — so an eval that cannot reach the engine reports
// instead of vanishing.
func (e *cefEngine) evaluate(js string, callID uint64, wantResult bool) {
	e.run(func() {
		params := map[string]any{"expression": js, "returnByValue": true}
		id, ok := e.sendCDP("Runtime.evaluate", params)
		if !ok {
			if wantResult {
				e.view.deliverEvalResult(callID, "", "webview: eval could not be delivered to the engine")
			}
			return
		}
		if wantResult {
			if e.cdpEvals == nil {
				e.cdpEvals = map[uint64]uint64{}
			}
			e.cdpEvals[id] = callID
		}
	})
}

// sendCDP sends one DevTools-protocol command and reports its id.
func (e *cefEngine) sendCDP(method string, params any) (uint64, bool) {
	if e.browser == nil {
		return 0, false
	}
	e.cdpNext++
	id := e.cdpNext
	msg, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		bellui.LogError("webview: cannot encode DevTools command", err)
		return 0, false
	}
	if !e.browser.SendDevTools(msg) {
		return 0, false
	}
	return id, true
}

// Created installs the bridge over CDP, then opens the queue. With the bridge
// off nothing is installed at all — the queue opens immediately and the page
// gets no channel, not an ignored one.
func (e *cefEngine) Created() {
	// Creation is asynchronous, so the size and scale CEF captured on the way
	// up can already be stale — a canvas whose scale settled after the first
	// layout (BELLUI_SCALE, a monitor pick during startup) has no later
	// layout to notify it through, because the browser was nil while it was
	// changing. One WasResized here makes CEF re-read ViewSize and
	// GetScreenInfo at their current values (#127's HiDPI leg measured
	// devicePixelRatio stuck at the creation-time scale without this).
	if e.host != nil && e.browser != nil {
		if w, h := e.host.viewSize(); w > 0 && h > 0 {
			e.browser.Resize(w, h)
		}
	}

	script := e.view.bridgeScript("window." + cefBindingName + "(s);")
	if script == "" {
		e.becomeReady()
		return
	}
	e.cdpSetup = map[uint64]bool{}
	setup := []struct {
		method string
		params any
	}{
		// Page.enable activates the page agent, without which the
		// document-start injection below never fires (measured on the probe:
		// the shim simply does not appear); Runtime.enable turns on the event
		// stream bindingCalled and executionContextCreated arrive on; the
		// last two install the transport and the shim.
		{"Page.enable", map[string]any{}},
		{"Runtime.enable", map[string]any{}},
		{"Runtime.addBinding", map[string]any{"name": cefBindingName}},
		{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": script}},
	}
	for _, cmd := range setup {
		if id, ok := e.sendCDP(cmd.method, cmd.params); ok {
			e.cdpSetup[id] = true
		}
	}
	if len(e.cdpSetup) == 0 {
		// Nothing could be sent — the browser is already going away. Open the
		// queue so pending evals fail through their own delivery path.
		e.becomeReady()
	}
}

func (e *cefEngine) Closed() {
	cefBrowserClosed()
}

// DevToolsMessage demultiplexes protocol traffic: acks for the bridge setup,
// replies to evals, and the events the bridge listens on.
func (e *cefEngine) DevToolsMessage(msg []byte) {
	var m struct {
		ID     uint64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &m); err != nil {
		bellui.LogError("webview: malformed DevTools message", err)
		return
	}

	if m.Method != "" {
		e.handleCDPEvent(m.Method, m.Params)
		return
	}

	if e.cdpSetup[m.ID] {
		delete(e.cdpSetup, m.ID)
		if m.Error != nil {
			bellui.LogError("webview: bridge setup command failed: "+m.Error.Message, nil)
		}
		if len(e.cdpSetup) == 0 {
			e.becomeReady()
		}
		return
	}

	if callID, ok := e.cdpEvals[m.ID]; ok {
		delete(e.cdpEvals, m.ID)
		e.deliverEval(callID, m.Result, m.Error)
	}
}

func (e *cefEngine) deliverEval(callID uint64, result json.RawMessage, cdpErr *struct {
	Message string `json:"message"`
},
) {
	if cdpErr != nil {
		e.view.deliverEvalResult(callID, "", cdpErr.Message)
		return
	}
	var res struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text      string `json:"text"`
			Exception *struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &res); err != nil {
		e.view.deliverEvalResult(callID, "", "webview: malformed eval result: "+err.Error())
		return
	}
	if ex := res.ExceptionDetails; ex != nil {
		// The useful message is the exception's own description
		// ("ReferenceError: x is not defined"); Text alone is just "Uncaught".
		msg := ex.Text
		if ex.Exception != nil && ex.Exception.Description != "" {
			msg = ex.Exception.Description
		}
		e.view.deliverEvalResult(callID, "", msg)
		return
	}
	// An undefined result has no value field; the portable half turns the
	// empty string into JSON null.
	e.view.deliverEvalResult(callID, string(res.Result.Value), "")
}

// handleCDPEvent tracks execution contexts — the origin source for the
// policy — and delivers bridge messages.
func (e *cefEngine) handleCDPEvent(method string, params json.RawMessage) {
	switch method {
	case "Runtime.executionContextCreated":
		var p struct {
			Context struct {
				ID     uint64 `json:"id"`
				Origin string `json:"origin"`
			} `json:"context"`
		}
		if json.Unmarshal(params, &p) != nil {
			return
		}
		if e.contexts == nil {
			e.contexts = map[uint64]string{}
		}
		// Chromium serializes an opaque origin as "://", which serializeOrigin
		// reduces to OriginOpaque along with every other originless shape.
		e.contexts[p.Context.ID] = serializeOrigin(p.Context.Origin)

	case "Runtime.executionContextDestroyed":
		var p struct {
			ID uint64 `json:"executionContextId"`
		}
		if json.Unmarshal(params, &p) == nil {
			delete(e.contexts, p.ID)
		}

	case "Runtime.executionContextsCleared":
		e.contexts = nil

	case "Runtime.bindingCalled":
		var p struct {
			Name      string `json:"name"`
			Payload   string `json:"payload"`
			ContextID uint64 `json:"executionContextId"`
		}
		if json.Unmarshal(params, &p) != nil || p.Name != cefBindingName {
			return
		}
		origin, ok := e.contexts[p.ContextID]
		if !ok {
			origin = OriginOpaque
		}
		e.view.deliverMessage(origin, p.Payload)
	}
}

// ---- load progress and navigation policy ----

func (e *cefEngine) LoadingStateChanged(loading bool, url string) {
	// The about:blank the browser is created on is an implementation detail;
	// reporting it would have every view emit a load cycle no other backend
	// emits. A page that explicitly navigates to about:blank still reports.
	if url == "about:blank" && e.view.url != "about:blank" {
		return
	}
	if loading {
		e.failedNav = false
		e.view.deliverLoad(LoadStateLoading, url, "")
		return
	}
	if e.failedNav {
		e.failedNav = false
		return
	}
	e.view.deliverLoad(LoadStateComplete, url, "")
}

func (e *cefEngine) LoadError(mainFrame bool, code int, errorText, url string) {
	if !mainFrame || code == gocef.ErrAborted {
		// Aborts are navigations this package already accounts for — a veto
		// or StopLoading reports through the portable half — or a load simply
		// replaced by a newer one, which no backend reports as failure.
		return
	}
	e.failedNav = true
	e.view.deliverLoad(LoadStateFailed, url, fmt.Sprintf("%s (net error %d)", errorText, code))
}

func (e *cefEngine) BeforeBrowse(ev gocef.NavigationEvent) bool {
	if ev.URL == "about:blank" && !e.ready {
		return true // the creation page, before any content was asked for
	}
	return e.view.decideNavigation(NavigationRequest{
		URL:       ev.URL,
		Kind:      navigationKindOf(ev.Transition),
		MainFrame: ev.MainFrame,
	})
}

// BeforePopup is a target="_blank"/window.open request. There is no second
// window, so an allowed one loads in place — the documented NewWindow
// semantics — and a denied one simply never happens (gocef always refuses
// the actual popup).
func (e *cefEngine) BeforePopup(url string, userGesture bool) {
	kind := NavigationOther
	if userGesture {
		kind = NavigationLinkClicked
	}
	allowed := e.view.decideNavigation(NavigationRequest{
		URL:       url,
		Kind:      kind,
		MainFrame: true,
		NewWindow: true,
	})
	if allowed && e.browser != nil {
		e.browser.LoadURL(url)
	}
}

func navigationKindOf(transition uint32) NavigationKind {
	if transition&gocef.TransitionForwardBackFlag != 0 {
		return NavigationBackForward
	}
	switch transition & gocef.TransitionSourceMask {
	case gocef.TransitionLink:
		return NavigationLinkClicked
	case gocef.TransitionFormSubmit:
		return NavigationFormSubmitted
	case gocef.TransitionReload:
		return NavigationReload
	}
	return NavigationOther
}

// ---- the offscreen surface, delegated to the host ----

func (e *cefEngine) ViewSize() (int, int) {
	if e.host != nil {
		if w, h := e.host.viewSize(); w > 0 && h > 0 {
			return w, h
		}
	}
	return 640, 480
}

func (e *cefEngine) ScreenScale() float32 {
	if e.host != nil {
		return e.host.screenScale()
	}
	return 1
}

func (e *cefEngine) Paint(buf []byte, w, h int, popup bool) {
	if e.host != nil && e.browser != nil {
		e.host.enginePaint(buf, w, h, popup)
	}
}

// ImeCompositionRangeChanged implements gocef.ImeHandler: the caret rect for
// the candidate window flows to the host's PreeditPositioner (#121).
func (e *cefEngine) ImeCompositionRangeChanged(sel gocef.Range, bounds []gocef.Rect) {
	if e.host != nil && e.browser != nil {
		e.host.engineImeCompositionRange(sel, bounds)
	}
}

func (e *cefEngine) AcceleratedPaint(gocef.AcceleratedFrame) {
	// Never requested: browsers are created with sharedTexture off. The
	// dma-buf import into the WebGPU painter is #113, unscheduled — the CPU
	// path measured 1.9 ms/frame at 1080p in hack/cefprobe.
}

func (e *cefEngine) PopupShow(show bool) {
	if e.host != nil {
		e.host.enginePopupShow(show)
	}
}

func (e *cefEngine) PopupRect(x, y, w, h int) {
	if e.host != nil {
		e.host.enginePopupRect(x, y, w, h)
	}
}

func (e *cefEngine) CursorChange(cursor int) {
	if e.host != nil {
		e.host.engineCursorChange(cursor)
	}
}

// The optional capabilities the engine relies on: a compile error here means
// gocef and this backend have drifted apart.
var (
	_ gocef.Handler           = (*cefEngine)(nil)
	_ gocef.DevToolsHandler   = (*cefEngine)(nil)
	_ gocef.LoadHandler       = (*cefEngine)(nil)
	_ gocef.NavigationHandler = (*cefEngine)(nil)
	_ gocef.ImeHandler        = (*cefEngine)(nil)
)
