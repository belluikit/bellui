package webview

import "errors"

// LoadState is what the view's main frame is doing. It changes only for main
// frame navigations; a sub-frame loading its own content does not move it.
//
// Since: 2.19
type LoadState int

const (
	// LoadStateIdle is a view that has not been asked to load anything yet.
	LoadStateIdle LoadState = iota
	// LoadStateLoading is a navigation in progress.
	LoadStateLoading
	// LoadStateComplete is a navigation that finished.
	LoadStateComplete
	// LoadStateFailed is a navigation that did not finish. The reason is in
	// [LoadEvent.Err].
	LoadStateFailed
)

// String returns the state's name, for logs and debugging.
func (s LoadState) String() string {
	switch s {
	case LoadStateIdle:
		return "idle"
	case LoadStateLoading:
		return "loading"
	case LoadStateComplete:
		return "complete"
	case LoadStateFailed:
		return "failed"
	}
	return "unknown"
}

// LoadEvent reports a change of [WebView.LoadState], delivered to
// [WebView.OnLoadChanged] on the main goroutine.
//
// Since: 2.19
type LoadEvent struct {
	// State is the state the view has just entered.
	State LoadState

	// URL is the address being loaded, as the engine resolved it. It may
	// differ from the URL that was requested, after a redirect.
	URL string

	// Err says why a load failed, and is nil for every other state. A
	// navigation the app itself cancelled from [WebView.OnNavigate] is
	// reported as a failure with [ErrNavigationCancelled], so a cancel is
	// never mistaken for a network problem.
	Err error
}

// ErrNavigationCancelled is the [LoadEvent.Err] of a navigation that
// [WebView.OnNavigate] refused, and of one stopped with
// [WebView.StopLoading].
//
// Since: 2.19
var ErrNavigationCancelled = errors.New("webview: navigation cancelled")

// NavigationKind says what started a navigation, so a policy can treat a link
// the user clicked differently from one the page started on its own.
//
// Since: 2.19
type NavigationKind int

const (
	// NavigationOther is a navigation with no more specific cause — most
	// often one the page started with script, or the initial load.
	NavigationOther NavigationKind = iota
	// NavigationLinkClicked is the user activating a link.
	NavigationLinkClicked
	// NavigationFormSubmitted is a form submission.
	NavigationFormSubmitted
	// NavigationBackForward is a move through session history.
	NavigationBackForward
	// NavigationReload is a reload.
	NavigationReload
)

// String returns the kind's name, for logs and debugging.
func (k NavigationKind) String() string {
	switch k {
	case NavigationOther:
		return "other"
	case NavigationLinkClicked:
		return "link"
	case NavigationFormSubmitted:
		return "form"
	case NavigationBackForward:
		return "back-forward"
	case NavigationReload:
		return "reload"
	}
	return "unknown"
}

// NavigationRequest is a navigation the engine is about to start, offered to
// [WebView.OnNavigate] before it does.
//
// Since: 2.19
type NavigationRequest struct {
	// URL is where the engine is about to go.
	URL string

	// Kind is what started it.
	Kind NavigationKind

	// MainFrame reports whether this is the view's own navigation rather than
	// one inside an embedded frame.
	MainFrame bool

	// NewWindow reports a navigation the page asked to open elsewhere —
	// target="_blank" or window.open. This package has no second window to
	// open it in, so an allowed one is loaded in this view instead; deny it
	// to keep such links from replacing the current page.
	NewWindow bool
}

// LoadState reports what the view's main frame is currently doing.
//
// Since: 2.19
func (w *WebView) LoadState() LoadState {
	return w.load
}

// LoadURL navigates the view to url.
//
// Before the view is attached the URL is only remembered, and is loaded as
// soon as it attaches; [WebView.URL] reports it either way.
func (w *WebView) LoadURL(url string) {
	w.url = url
	if !w.attachedRaw() {
		return
	}
	w.loadURLRaw(url)
}

// LoadHTML displays html directly, resolving the page's relative references
// against baseURL. An empty baseURL leaves the content with no origin of its
// own, which the bridge reports as [OriginOpaque].
//
// It is a no-op until the view is attached — unlike [WebView.LoadURL], there
// is no pending-content slot; load the content from [WebView.OnLoadChanged]
// or after [WebView.Attach] returns.
//
// Content loaded this way may not become a session history entry, so
// [WebView.CanGoBack] can still report false afterwards. That is the engine's
// own behaviour for literal content with no base URL — measured on WebKit —
// not something this package can arrange around; give the content a real
// baseURL, or serve it, if it has to be navigable.
//
// Since: 2.19
func (w *WebView) LoadHTML(html, baseURL string) {
	if !w.attachedRaw() {
		return
	}
	w.url = baseURL
	w.loadHTMLRaw(html, baseURL)
}

// Reload loads the current page again.
//
// Since: 2.19
func (w *WebView) Reload() {
	w.navRaw(navReload)
}

// StopLoading stops a navigation in progress. A load stopped this way is
// reported to [WebView.OnLoadChanged] as failed with
// [ErrNavigationCancelled].
//
// Since: 2.19
func (w *WebView) StopLoading() {
	if !w.attachedRaw() {
		return
	}
	wasLoading := w.load == LoadStateLoading
	w.navRaw(navStopLoading)
	if wasLoading {
		w.reportLoad(LoadStateFailed, w.url, ErrNavigationCancelled)
	}
}

// CanGoBack reports whether there is an earlier entry in session history.
//
// Since: 2.19
func (w *WebView) CanGoBack() bool {
	return w.navRaw(navCanGoBack)
}

// GoBack moves to the previous entry in session history, reporting whether
// there was one to move to.
//
// Since: 2.19
func (w *WebView) GoBack() bool {
	return w.navRaw(navGoBack)
}

// CanGoForward reports whether there is a later entry in session history.
//
// Since: 2.19
func (w *WebView) CanGoForward() bool {
	return w.navRaw(navCanGoForward)
}

// GoForward moves to the next entry in session history, reporting whether
// there was one to move to.
//
// Since: 2.19
func (w *WebView) GoForward() bool {
	return w.navRaw(navGoForward)
}

// ---- inbound ----

// deliverLoad is the backend entry point for a main-frame load state change.
// An empty errMsg means the state carries no error. A backend can only hand
// across a string, so this is where one becomes an error value; the
// cancellations bellui raises itself go through reportLoad instead, which
// keeps [ErrNavigationCancelled] matchable with errors.Is.
func (w *WebView) deliverLoad(state LoadState, url, errMsg string) {
	var err error
	if errMsg != "" {
		err = errors.New(errMsg)
	}
	w.reportLoad(state, url, err)
}

func (w *WebView) reportLoad(state LoadState, url string, err error) {
	w.load = state
	// A failed navigation is one the view did not make: it stays on the page
	// it was already showing, so URL() has to as well. Recording the target of
	// a refused or broken load would have URL() name a page that was never
	// reached, which is exactly what a policy hook exists to prevent.
	if url != "" && state != LoadStateFailed {
		w.url = url
	}
	if w.OnLoadChanged == nil {
		return
	}
	w.OnLoadChanged(LoadEvent{State: state, URL: url, Err: err})
}

// decideNavigation is the backend entry point for the navigation policy hook.
// The engine is blocked on the answer, so the hook has to be quick — which is
// why it is a plain predicate rather than anything asynchronous.
func (w *WebView) decideNavigation(req NavigationRequest) bool {
	if w.OnNavigate == nil {
		return true
	}
	if w.OnNavigate(req) {
		return true
	}
	if req.MainFrame {
		w.reportLoad(LoadStateFailed, req.URL, ErrNavigationCancelled)
	}
	return false
}
