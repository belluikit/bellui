//go:build ios

#import <stdbool.h>
#import <stdint.h>

// WKWebView bridge for widget.NativeViewHost on iOS (#64).
//
// Shape: a clipping UIView is added to the application's UIWindow as a sibling
// of the GLKView, and the WKWebView lives inside it. The clip view carries the
// host's clip rectangle with clipsToBounds, which is what gives a
// NativeViewHost inside a container.Scroll a native equivalent of bellui's
// clipping; the web view carries the host's own rectangle relative to it.
//
// It is deliberately NOT a subview of the GLKView. That is the pre-rebrand tracker #1124 lesson,
// learned from BelluiTextInputView: a view hosted inside the GLKView entangles
// with its CAMetalLayer and stalls rendering. macOS is the other way round —
// there the clip view IS a subview of the GLFW content view and composites
// cleanly — so the two darwin backends differ here on purpose.
//
// UIKit uses a top-left origin, like bellui, so unlike the macOS bridge no
// coordinate flipping is needed anywhere.
//
// Every function must be called on the main thread. The handle returned by
// belluiWebViewCreate is the retained clip view; Go owns it until it calls
// belluiWebViewDestroy.

// belluiWebViewCreate builds the clip view and its WKWebView in the
// application's window, and returns an opaque handle. It returns NULL when
// there is no window yet.
//
// token identifies the Go-side WebView in the callback registry: script
// messages, evaluation results, load state changes and navigation decisions
// are all delivered back to Go carrying it.
//
// bridgeScript is the page-side bridge, injected at document start. When it
// is NULL or empty NO script message handler and NO user script are
// installed, so a page has no channel to bellui at all — which is the
// guarantee behind webview.WebView.EnableBridge being opt-in.
void *belluiWebViewCreate(uint64_t token, const char *bridgeScript);

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
// All coordinates are in UIKit points with a top-left origin, relative to the
// application window. The view rectangle is relative to the window, not the
// clip; the bridge expresses it relative to the clip's origin.
void belluiWebViewSetGeometry(void *handle,
                              double clipX, double clipY, double clipW, double clipH,
                              double viewX, double viewY, double viewW, double viewH);

// belluiWebViewSetVisible shows or hides the clip view and everything in it.
void belluiWebViewSetVisible(void *handle, bool visible);

// belluiWebViewTakeFocus makes the web view the first responder, which is what
// a tap inside it does. It returns whether UIKit accepted the change.
bool belluiWebViewTakeFocus(void *handle);

// belluiWebViewResignFocus makes the web view give up the first responder, so
// that keystrokes and input-method composition reach bellui again. Unlike the
// macOS bridge it does not hand the responder to anything else — see the
// implementation for why. A web view that does not hold the focus is left
// alone and reports success.
bool belluiWebViewResignFocus(void *handle);

// belluiWebViewHasFocus reports whether the web view (or any view inside it)
// is currently the first responder — i.e. whether keystrokes are going to the
// web engine rather than to bellui.
bool belluiWebViewHasFocus(void *handle);

// belluiWebViewDestroy removes the views from the window and releases them.
// Passing NULL, or the same handle twice, is safe.
void belluiWebViewDestroy(void *handle);

// belluiWebViewNavigateDecided answers a navigation-policy request that
// belluiWebViewNavigate carried to Go. decision is the WebKit handler block
// the ObjC side copied and holds until exactly this call; allow is the app's
// verdict. Called from the bellui goroutine; the handler is invoked on the
// main queue.
void belluiWebViewNavigateDecided(uintptr_t decision, bool allow);
