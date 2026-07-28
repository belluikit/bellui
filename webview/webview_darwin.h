//go:build darwin && !ios && !wasm

#import <stdbool.h>
#import <stdint.h>

// WKWebView bridge for widget.NativeViewHost on macOS (#45, #66).
//
// Shape: a flipped, layer-backed clip NSView is added as a subview of the
// GLFW window's contentView, and the WKWebView lives inside it. The clip view
// carries the host's clip rectangle with masksToBounds, which is what gives a
// NativeViewHost inside a container.Scroll a native equivalent of bellui's
// clipping; the web view carries the host's own rectangle relative to it.
//
// Every function must be called on the main thread. The handle returned by
// belluiWebViewCreate is the retained clip view; Go owns it until it calls
// belluiWebViewDestroy.

// belluiWebViewCreate builds the clip view and its WKWebView inside the
// contentView of the given NSWindow, and returns an opaque handle. It returns
// NULL when the window has no content view.
//
// token identifies the Go-side WebView in the callback registry: script
// messages, evaluation results, load state changes and navigation decisions
// are all delivered back to Go carrying it.
//
// bridgeScript is the page-side bridge, injected at document start. When it
// is NULL or empty NO script message handler and NO user script are
// installed, so a page has no channel to bellui at all — which is the
// guarantee behind webview.WebView.EnableBridge being opt-in.
void *belluiWebViewCreate(uintptr_t nsWindow, uint64_t token, const char *bridgeScript);

// belluiWebViewEval runs JavaScript in the page. When wantResult is false the
// result is discarded; otherwise the value (encoded as JSON) or the error is
// reported to Go through belluiWebViewEvalDone carrying callID.
void belluiWebViewEval(void *handle, const char *js, uint64_t callID, bool wantResult);

// belluiWebViewLoadURL navigates the view to url. A url that does not parse
// is ignored.
void belluiWebViewLoadURL(void *handle, const char *url);

// belluiWebViewLoadHTML displays html directly, resolving relative references
// against baseURL. An empty or unparseable baseURL leaves the content with an
// opaque origin.
void belluiWebViewLoadHTML(void *handle, const char *html, const char *baseURL);

// belluiWebViewNav performs one history or load-control operation, matching
// the navOp constants in platform.go: 0 canGoBack, 1 goBack, 2 canGoForward,
// 3 goForward, 4 reload, 5 stopLoading. Queries report their answer, commands
// report whether they were carried out.
bool belluiWebViewNav(void *handle, int op);

// belluiWebViewSetGeometry places the clip view and the web view inside it.
// All coordinates are in AppKit points with a top-left origin, relative to
// the window's content view; the bridge flips them into AppKit's bottom-left
// space. The view rectangle is relative to the content view, not the clip.
void belluiWebViewSetGeometry(void *handle,
                              double clipX, double clipY, double clipW, double clipH,
                              double viewX, double viewY, double viewW, double viewH);

// belluiWebViewSetVisible shows or hides the clip view and everything in it.
void belluiWebViewSetVisible(void *handle, bool visible);

// belluiWebViewTakeFocus makes the web view the window's first responder,
// which is what a click inside it does. It returns whether the window
// accepted the change.
bool belluiWebViewTakeFocus(void *handle);

// belluiWebViewResignFocus hands the window's first responder back to the
// GLFW content view, so keystrokes reach bellui again. It returns whether the
// window accepted the change; a web view that does not hold the focus is left
// alone and reports success.
bool belluiWebViewResignFocus(void *handle);

// belluiWebViewHasFocus reports whether the web view (or any view inside it)
// is currently the window's first responder — i.e. whether keystrokes are
// going to the web engine rather than to bellui.
bool belluiWebViewHasFocus(void *handle);

// belluiWebViewDestroy removes the views from the window and releases them.
// Passing NULL, or the same handle twice, is safe.
void belluiWebViewDestroy(void *handle);
