package webview

// This file is the contract between the portable half of this package and a
// platform backend. Everything a backend has to supply is listed here, so a
// new backend (#62 wasm, #63 Android, #64 iOS, #65 Windows) can be written
// against one page rather than by reading the macOS implementation.
//
// A backend supplies, on *WebView:
//
//	attachRaw(win bellui.Window) error   create the native view. The page-side
//	                                     bridge is installed here and ONLY when
//	                                     bridgeScript() returns a non-empty
//	                                     string — with the bridge off there
//	                                     must be no message channel in the page
//	                                     at all, not merely an ignored one.
//	destroyRaw()                         release the native view
//	attachedRaw() bool                   is there a live native view?
//	evalRaw(js string)                   run js, discard the result
//	evalResultRaw(js string, id uint64)  run js, then report through
//	                                     deliverEvalResult(id, …)
//	loadURLRaw(url string)               navigate to url
//	loadHTMLRaw(html, baseURL string)    load literal content
//	navRaw(op navOp) bool                history and load control
//
// plus the geometry, visibility and focus methods of the widget.NativeView and
// widget.NativeViewFocuser contracts, and the exported field w.url, which the
// portable half reads back through WebView.URL.
//
// A backend calls in, always on the main goroutine:
//
//	deliverMessage(origin, body string)          a script message from the page
//	deliverEvalResult(id uint64, res, errMsg)    an eval finished
//	deliverLoad(state LoadState, url, errMsg)    the main frame's state changed
//	decideNavigation(req NavigationRequest) bool may this navigation proceed?
//
// The message body is the JSON envelope the page-side shim posts; a backend
// never parses it. The origin is the page's serialized security origin
// ("https://example.com", "file://") or [OriginOpaque] for content that has
// none — a data: URL, about:blank, or a sandboxed frame.

// navOp is one of the history and load-control operations a backend answers
// through navRaw. Query operations report their answer; commands report
// whether they were carried out.
type navOp int

const (
	navCanGoBack navOp = iota
	navGoBack
	navCanGoForward
	navGoForward
	navReload
	navStopLoading
)
