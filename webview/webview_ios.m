//go:build ios

#import <UIKit/UIKit.h>
#import <WebKit/WebKit.h>

#import "webview_ios.h"

// Implemented in Go (webview_ios.go). All are called on the UIKit main
// thread — which on this driver is NOT the bellui main goroutine (the app
// loop runs on its own thread, darwin_ios.go) — so the Go side queues each
// delivery onto the bellui goroutine; only the navigation decision waits.
extern void belluiWebViewMessage(uint64_t token, char *origin, char *body);
extern void belluiWebViewEvalDone(uint64_t token, uint64_t callID, char *result, char *errMsg);
extern void belluiWebViewLoad(uint64_t token, int state, char *url, char *errMsg);
extern void belluiWebViewNavigate(uint64_t token, char *url, int kind, bool mainFrame,
                                  bool newWindow, uintptr_t decision);

// Implemented in internal/driver/mobile/app/darwin_ios.m. The iOS window's
// RunNative hands out only an UnknownContext, so this accessor is how a native
// view reaches the window it has to be hosted in.
extern UIWindow *belluiAppWindow(void);

// MRC (no ARC), matching the other ObjC bridges in this repo.

// Load states, matching webview.LoadState in navigation.go.
enum { belluiLoadIdle = 0, belluiLoadLoading = 1, belluiLoadComplete = 2, belluiLoadFailed = 3 };

// Navigation kinds, matching webview.NavigationKind in navigation.go.
enum {
    belluiNavOther = 0,
    belluiNavLink = 1,
    belluiNavForm = 2,
    belluiNavBackForward = 3,
    belluiNavReload = 4
};

// Operations, matching webview.navOp in platform.go.
enum {
    belluiOpCanGoBack = 0,
    belluiOpGoBack = 1,
    belluiOpCanGoForward = 2,
    belluiOpGoForward = 3,
    belluiOpReload = 4,
    belluiOpStopLoading = 5
};

// cStr answers the UTF-8 bytes of s, or the empty string for nil, so a Go
// callback never has to distinguish "absent" from "empty".
static char *cStr(NSString *s) {
    if (!s) {
        return (char *)"";
    }
    const char *utf8 = [s UTF8String];
    return (char *)(utf8 ? utf8 : "");
}

// belluiOriginOf serializes a frame's security origin the way the bridge's
// OriginPolicy expects: "https://example.com", "http://localhost:8080",
// "file://", or "null" for content with no origin of its own (a data: URL,
// about:blank, or a sandboxed frame).
static NSString *belluiOriginOf(WKFrameInfo *frame) {
    WKSecurityOrigin *origin = [frame securityOrigin];
    NSString *scheme = [origin protocol];
    if (!scheme || [scheme length] == 0) {
        return @"null";
    }
    if ([scheme isEqualToString:@"file"]) {
        return @"file://";
    }
    NSString *host = [origin host];
    if (!host || [host length] == 0) {
        return @"null";
    }
    NSInteger port = [origin port];
    if (port == 0) {
        return [NSString stringWithFormat:@"%@://%@", scheme, host];
    }
    return [NSString stringWithFormat:@"%@://%@:%ld", scheme, host, (long)port];
}

// belluiJSONOf encodes an evaluateJavaScript result as a JSON document. The
// value is wrapped in an array before serializing and unwrapped afterwards,
// which is how a bare fragment (a number, a string, null) is encoded without
// depending on NSJSONWritingFragmentsAllowed. A value WebKit produced that
// JSON cannot represent — a Date, for instance — is reported as its
// description string rather than dropped.
static NSString *belluiJSONOf(id value) {
    if (!value || value == [NSNull null]) {
        return @"null";
    }
    id wrapped = @[ value ];
    if (![NSJSONSerialization isValidJSONObject:wrapped]) {
        wrapped = @[ [value description] ];
    }
    NSError *err = nil;
    NSData *data = [NSJSONSerialization dataWithJSONObject:wrapped options:0 error:&err];
    if (!data) {
        return @"null";
    }
    NSString *text = [[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding] autorelease];
    if ([text length] < 2) {
        return @"null";
    }
    return [text substringWithRange:NSMakeRange(1, [text length] - 2)];
}

// belluiFindFirstResponder searches a view subtree for the current first
// responder. UIKit, unlike AppKit, offers no way to ask a window who its first
// responder is, and WKWebView installs its own internal content view rather
// than becoming one itself — so "is the keyboard going to the web engine?" can
// only be answered by walking the subtree.
static UIView *belluiFindFirstResponder(UIView *view) {
    if (!view) {
        return nil;
    }
    if ([view isFirstResponder]) {
        return view;
    }
    for (UIView *child in [view subviews]) {
        UIView *found = belluiFindFirstResponder(child);
        if (found) {
            return found;
        }
    }
    return nil;
}

// BelluiWebViewBridge receives window.webkit.messageHandlers.bellui.postMessage
// from the page and forwards it to Go by token, with the sending frame's
// origin so the Go side can apply its OriginPolicy. It is installed only when
// the bridge is enabled, so a view without one has no channel to reject.
@interface BelluiWebViewBridge : NSObject <WKScriptMessageHandler>
@property(assign) uint64_t token;
@end

@implementation BelluiWebViewBridge

- (void)userContentController:(WKUserContentController *)controller
      didReceiveScriptMessage:(WKScriptMessage *)message {
    // Message delivery is on the main thread, which is where the bellui main
    // goroutine sits, so no locking is needed on the Go side.
    NSString *body = [NSString stringWithFormat:@"%@", [message body]];
    belluiWebViewMessage(self.token, cStr(belluiOriginOf([message frameInfo])), cStr(body));
}

@end

// BelluiWebViewDelegate turns WebKit's navigation callbacks into the two
// portable seams: a policy decision the app can veto, and a load state the app
// can watch.
@class BelluiWebViewClip;

// BelluiWebViewHandle is what crosses the cgo boundary: a plain NSObject that
// can be allocated on the bellui goroutine's thread, holding the UIKit pieces
// that may only be touched on the main thread, plus atomic shadows for the
// queries the Go side asks synchronously. See the threading-model comment at
// its @implementation below.
@interface BelluiWebViewHandle : NSObject
@property(retain) BelluiWebViewClip *clip; // set on main once built
@property(assign) uint64_t token;
@property(assign) BOOL shadowFocus;
@property(assign) BOOL shadowCanGoBack;
@property(assign) BOOL shadowCanGoForward;
@end

@interface BelluiWebViewDelegate : NSObject <WKNavigationDelegate, WKUIDelegate>
@property(assign) uint64_t token;
// Back-pointer to the handle whose shadows the load callbacks refresh; assign,
// so the reference cycle handle->clip->delegate->handle never forms.
@property(assign) BelluiWebViewHandle *handle;
@end

@implementation BelluiWebViewDelegate

- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationAction:(WKNavigationAction *)action
                    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    int kind = belluiNavOther;
    switch ([action navigationType]) {
    case WKNavigationTypeLinkActivated:
        kind = belluiNavLink;
        break;
    case WKNavigationTypeFormSubmitted:
    case WKNavigationTypeFormResubmitted:
        kind = belluiNavForm;
        break;
    case WKNavigationTypeBackForward:
        kind = belluiNavBackForward;
        break;
    case WKNavigationTypeReload:
        kind = belluiNavReload;
        break;
    default:
        break;
    }

    // A missing target frame is a target="_blank" or window.open navigation.
    // There is no second window to put it in, so an allowed one replaces the
    // main frame — which makes it a main-frame navigation for policy purposes.
    WKFrameInfo *target = [action targetFrame];
    bool newWindow = (target == nil);
    bool mainFrame = newWindow ? true : [target isMainFrame];

    NSString *url = [[[action request] URL] absoluteString];

    // The answer comes from app code on the bellui goroutine, which must not
    // be waited for from this thread: the driver dispatch_syncs onto the main
    // queue from that goroutine, so blocking here completes a deadlock cycle
    // (#75 froze the whole app on the first page load this way). WebKit
    // explicitly allows the decision handler to be called later — so the
    // request crosses to Go carrying the copied handler, and the decision
    // comes back through belluiWebViewNavigateDecided below.
    void (^held)(WKNavigationActionPolicy) = [decisionHandler copy];
    belluiWebViewNavigate(self.token, cStr(url), kind, mainFrame, newWindow, (uintptr_t)held);
}

- (void)refreshNav:(WKWebView *)webView {
    // Runs on the main thread (WebKit delivers here); keeps the atomic
    // shadows the bellui goroutine reads synchronously in belluiWebViewNav.
    [[self handle] setShadowCanGoBack:[webView canGoBack]];
    [[self handle] setShadowCanGoForward:[webView canGoForward]];
}

- (void)webView:(WKWebView *)webView didStartProvisionalNavigation:(WKNavigation *)navigation {
    [self refreshNav:webView];
    belluiWebViewLoad(self.token, belluiLoadLoading, cStr([[webView URL] absoluteString]), (char *)"");
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    [self refreshNav:webView];
    belluiWebViewLoad(self.token, belluiLoadComplete, cStr([[webView URL] absoluteString]), (char *)"");
}

// reportFailure filters the two errors that are not failures to report: a
// cancellation, and a policy change. Both are navigations bellui itself
// stopped (StopLoading, or a denied OnNavigate), and the Go side already
// reported them — reporting again would deliver the same cancel twice.
- (void)reportFailure:(NSError *)error forView:(WKWebView *)webView {
    [self refreshNav:webView];
    if ([[error domain] isEqualToString:NSURLErrorDomain] && [error code] == NSURLErrorCancelled) {
        return;
    }
    if ([[error domain] isEqualToString:@"WebKitErrorDomain"] && [error code] == 102) {
        return; // frame load interrupted by a policy change
    }
    belluiWebViewLoad(self.token, belluiLoadFailed, cStr([[webView URL] absoluteString]),
                      cStr([error localizedDescription]));
}

- (void)webView:(WKWebView *)webView
    didFailNavigation:(WKNavigation *)navigation
            withError:(NSError *)error {
    [self reportFailure:error forView:webView];
}

- (void)webView:(WKWebView *)webView
    didFailProvisionalNavigation:(WKNavigation *)navigation
                       withError:(NSError *)error {
    [self reportFailure:error forView:webView];
}

// createWebViewWithConfiguration is WebKit asking for a second web view to put
// a target="_blank" navigation in. There is no second view, so the request is
// loaded in this one — the policy hook above has already been given the chance
// to refuse it. Returning nil tells WebKit no view was created.
- (WKWebView *)webView:(WKWebView *)webView
    createWebViewWithConfiguration:(WKWebViewConfiguration *)configuration
               forNavigationAction:(WKNavigationAction *)action
                    windowFeatures:(WKWindowFeatures *)windowFeatures {
    if (![action targetFrame]) {
        [webView loadRequest:[action request]];
    }
    return nil;
}

@end

// belluiWebViewNavigateDecided delivers the Go side's navigation verdict back
// to the WebKit decision handler held above, on the main thread, exactly
// once. The block was copied with +1; invoking releases it here.
void belluiWebViewNavigateDecided(uintptr_t decision, bool allow) {
    dispatch_async(dispatch_get_main_queue(), ^{
        void (^handler)(WKNavigationActionPolicy) = (void (^)(WKNavigationActionPolicy))decision;
        handler(allow ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
        [handler release];
    });
}

// BelluiWebViewClip is the hole-punch container: a plain UIView whose
// clipsToBounds turns the host's bellui clip bounds into a real clip, so a web
// view inside a container.Scroll is cut off at the scroll's edge instead of
// overflowing it.
@interface BelluiWebViewClip : UIView
@property(assign) WKWebView *webView;
@property(assign) BelluiWebViewDelegate *navDelegate;
@property(assign) uint64_t token;
@end

@implementation BelluiWebViewClip
@end

// BelluiWebViewHandle is what crosses the cgo boundary: a plain NSObject that
// can be allocated on the bellui goroutine's thread, holding the UIKit pieces
// that may only be touched on the main thread, plus atomic shadows for the
// queries the Go side asks synchronously.
//
// # Threading model (the #75 lesson)
//
// On this driver the bellui main goroutine is NOT the UIKit main thread — the
// app loop runs on its own thread (darwin_ios.go). UIKit tolerates no
// off-main mutation, so every entry point below that changes UIKit state does
// its work in a dispatch_async block on the main queue. The queue is serial,
// so creation is always fully applied before any later operation's block
// runs, and geometry, visibility and navigation stay ordered exactly as the
// Go side issued them — the Android backend's runOnUiThread discipline,
// spoken in GCD.
//
// Synchronous questions (HasFocus, CanGoBack/Forward) are answered from the
// atomic shadow properties, refreshed on the main thread by the delegate's
// load callbacks and by the per-frame geometry sync. dispatch_sync is
// deliberately never used: the navigation policy hook blocks the main thread
// on the bellui goroutine (the one documented waiter), and a single
// dispatch_sync in the other direction would complete the deadlock cycle.
@implementation BelluiWebViewHandle
@end

// belluiRefreshShadows re-reads the queryable state on the main thread.
static void belluiRefreshShadows(BelluiWebViewHandle *h) {
    BelluiWebViewClip *clip = [h clip];
    WKWebView *web = [clip webView];
    if (!web) {
        [h setShadowFocus:NO];
        [h setShadowCanGoBack:NO];
        [h setShadowCanGoForward:NO];
        return;
    }
    [h setShadowFocus:belluiFindFirstResponder(web) != nil];
    [h setShadowCanGoBack:[web canGoBack]];
    [h setShadowCanGoForward:[web canGoForward]];
}

// belluiWebViewEnsureHosted parents the clip into the app window as soon as
// one exists. Main thread only. Creation must not require the window: the
// host attaches on the FIRST layout pass, which can run before the UIWindow
// is up (#75 measured the whole backend inert because of it), and the host's
// Attach is deliberately once-only. Geometry arrives every frame, so hosting
// retries here until the window appears — usually the very next frame.
static void belluiWebViewEnsureHosted(BelluiWebViewClip *clip) {
    if (!clip || [clip superview] != nil) {
        return;
    }
    UIWindow *window = belluiAppWindow();
    if (!window) {
        return;
    }
    // Added to the WINDOW, as a sibling of the GLKView, never as its subview.
    // A view hosted inside the GLKView entangles with its CAMetalLayer and
    // stalls rendering — the #1124 lesson from BelluiTextInputView, which is
    // hosted here for the same reason.
    [window addSubview:clip];
}

void *belluiWebViewCreate(uint64_t token, const char *bridgeScript) {
    BelluiWebViewHandle *h = [[BelluiWebViewHandle alloc] init]; // +1, Destroy releases
    [h setToken:token];

    // The C string dies when this call returns; the block below runs later.
    NSString *script = nil;
    if (bridgeScript && bridgeScript[0] != '\0') {
        script = [NSString stringWithUTF8String:bridgeScript];
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        BelluiWebViewClip *clip = [[BelluiWebViewClip alloc] initWithFrame:CGRectZero];
        [clip setClipsToBounds:YES];
        [clip setHidden:YES]; // shown once the host reports real geometry

        WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];

        // The bridge is installed here or not at all: with no script, the page
        // has neither window.bellui nor a message handler to post to, which is
        // the guarantee behind EnableBridge being opt-in.
        if (script) {
            BelluiWebViewBridge *bridge = [[BelluiWebViewBridge alloc] init];
            [bridge setToken:token];
            [[config userContentController] addScriptMessageHandler:bridge name:@"bellui"];
            [bridge release]; // the controller retains it

            WKUserScript *user =
                [[WKUserScript alloc] initWithSource:script
                                       injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                                    forMainFrameOnly:NO];
            [[config userContentController] addUserScript:user];
            [user release];
        }

        WKWebView *web = [[WKWebView alloc] initWithFrame:CGRectZero configuration:config];
        [config release];

        BelluiWebViewDelegate *delegate = [[BelluiWebViewDelegate alloc] init];
        [delegate setToken:token];
        [delegate setHandle:h];
        [web setNavigationDelegate:delegate]; // weak, so the clip owns the +1
        [web setUIDelegate:delegate];

        [clip addSubview:web];
        [clip setWebView:web];
        [clip setNavDelegate:delegate];
        [clip setToken:token];

        [h setClip:clip]; // retained by the property
        [clip release];   // the handle owns it now

        belluiWebViewEnsureHosted(clip);
    });
    return h;
}

void belluiWebViewLoadURL(void *handle, const char *url) {
    if (!handle || !url) {
        return;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    NSString *str = [NSString stringWithUTF8String:url];
    dispatch_async(dispatch_get_main_queue(), ^{
        WKWebView *web = [[h clip] webView];
        NSURL *target = [NSURL URLWithString:str];
        if (!web || !target || ![target scheme]) {
            return;
        }
        if ([[target scheme] isEqualToString:@"file"]) {
            [web loadFileURL:target allowingReadAccessToURL:[target URLByDeletingLastPathComponent]];
            return;
        }
        [web loadRequest:[NSURLRequest requestWithURL:target]];
    });
}

void belluiWebViewLoadHTML(void *handle, const char *html, const char *baseURL) {
    if (!handle || !html) {
        return;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    NSString *content = [NSString stringWithUTF8String:html];
    NSString *base = nil;
    if (baseURL && baseURL[0] != '\0') {
        base = [NSString stringWithUTF8String:baseURL];
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        WKWebView *web = [[h clip] webView];
        if (!web) {
            return;
        }
        [web loadHTMLString:content baseURL:(base ? [NSURL URLWithString:base] : nil)];
    });
}

bool belluiWebViewNav(void *handle, int op) {
    if (!handle) {
        return false;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    switch (op) {
    case belluiOpCanGoBack:
        return [h shadowCanGoBack];
    case belluiOpCanGoForward:
        return [h shadowCanGoForward];
    case belluiOpGoBack:
        if (![h shadowCanGoBack]) {
            return false;
        }
        dispatch_async(dispatch_get_main_queue(), ^{ [[[h clip] webView] goBack]; });
        return true;
    case belluiOpGoForward:
        if (![h shadowCanGoForward]) {
            return false;
        }
        dispatch_async(dispatch_get_main_queue(), ^{ [[[h clip] webView] goForward]; });
        return true;
    case belluiOpReload:
        dispatch_async(dispatch_get_main_queue(), ^{ [[[h clip] webView] reload]; });
        return true;
    case belluiOpStopLoading:
        dispatch_async(dispatch_get_main_queue(), ^{ [[[h clip] webView] stopLoading]; });
        return true;
    }
    return false;
}

void belluiWebViewEval(void *handle, const char *js, uint64_t callID, bool wantResult) {
    if (!handle || !js) {
        return;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    NSString *source = [NSString stringWithUTF8String:js];
    uint64_t token = [h token];
    dispatch_async(dispatch_get_main_queue(), ^{
        WKWebView *web = [[h clip] webView];
        if (!web) {
            if (wantResult) {
                belluiWebViewEvalDone(token, callID, (char *)"", (char *)"webview: no engine");
            }
            return;
        }
        if (!wantResult) {
            [web evaluateJavaScript:source completionHandler:nil];
            return;
        }
        // The completion handler runs on the main thread, and carries only
        // scalars plus the token, so a result arriving after the view was
        // destroyed simply finds no registry entry on the Go side.
        [web evaluateJavaScript:source
              completionHandler:^(id value, NSError *error) {
                  if (error) {
                      // localizedDescription is only ever "A JavaScript
                      // exception occurred"; the message the script actually
                      // threw is in the userInfo, and is the only part worth
                      // reporting to the app.
                      NSString *msg = [[error userInfo] objectForKey:@"WKJavaScriptExceptionMessage"];
                      if (!msg || [msg length] == 0) {
                          msg = [error localizedDescription];
                      }
                      belluiWebViewEvalDone(token, callID, (char *)"", cStr(msg));
                      return;
                  }
                  belluiWebViewEvalDone(token, callID, cStr(belluiJSONOf(value)), (char *)"");
              }];
    });
}

void belluiWebViewSetGeometry(void *handle,
                              double clipX, double clipY, double clipW, double clipH,
                              double viewX, double viewY, double viewW, double viewH) {
    if (!handle) {
        return;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    dispatch_async(dispatch_get_main_queue(), ^{
        BelluiWebViewClip *clip = [h clip];
        if (!clip) {
            return;
        }
        belluiWebViewEnsureHosted(clip);

        // The incoming values are pixels (bellui points x the canvas scale).
        // UIKit frames are in UIKit points — pixels / the screen scale, the
        // same transform the driver's text-layout bridge uses
        // (darwin_ios.m). On macOS the equivalent multiply is invisible
        // because the desktop canvas scale is 1.0; here it is the display
        // scale, and using pixels as points doubled every rectangle — the
        // clip box spilling over the widgets below was how #75's device
        // pass caught it.
        CGFloat ss = [[UIScreen mainScreen] scale];
        if (ss <= 0) {
            ss = 1;
        }
        // UIKit already uses a top-left origin, so unlike the macOS bridge
        // no flip is needed.
        [clip setFrame:CGRectMake(clipX / ss, clipY / ss, clipW / ss, clipH / ss)];
        [[clip webView] setFrame:CGRectMake((viewX - clipX) / ss, (viewY - clipY) / ss,
                                            viewW / ss, viewH / ss)];
    });
}

void belluiWebViewSetVisible(void *handle, bool visible) {
    if (!handle) {
        return;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    dispatch_async(dispatch_get_main_queue(), ^{
        BelluiWebViewClip *clip = [h clip];
        if (!clip) {
            return;
        }
        belluiWebViewEnsureHosted(clip);
        [clip setHidden:!visible];
    });
}

bool belluiWebViewHasFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    // The host polls this every frame, which makes the poll itself the
    // refresh heartbeat: answer from the shadow (one frame stale at worst)
    // and queue a main-thread re-read for the next poll. Hanging the refresh
    // on SetGeometry does not work — geometry is only pushed when it CHANGES,
    // so the shadow would freeze the moment the layout settles (#75 measured
    // the focus bridge dead because of exactly that).
    dispatch_async(dispatch_get_main_queue(), ^{ belluiRefreshShadows(h); });
    return [h shadowFocus];
}

bool belluiWebViewTakeFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    if (![h clip]) {
        return false;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        [[[h clip] webView] becomeFirstResponder];
        belluiRefreshShadows(h);
    });
    // Optimistic, like the Android backend's shadow state: the poll corrects
    // the host within a frame if the platform refused.
    return true;
}

bool belluiWebViewResignFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    dispatch_async(dispatch_get_main_queue(), ^{
        // Resign whichever view inside the web view actually holds it —
        // WKWebView installs its own content view, so resigning the WKWebView
        // itself is not always enough.
        UIView *responder = belluiFindFirstResponder([[h clip] webView]);
        if (responder) {
            [responder resignFirstResponder];
        }
        // Deliberately nothing is made first responder in its place. On macOS
        // the focus has to land on the GLFW content view or keystrokes reach
        // nothing, but on iOS there is no key-handling view to hand it to:
        // bellui's own keyboard input goes through BelluiTextInputView, which
        // claims the responder only while an entry is focused. Claiming it
        // here would raise the software keyboard with nothing being edited.
        belluiRefreshShadows(h);
    });
    return true;
}

void belluiWebViewDestroy(void *handle) {
    if (!handle) {
        return;
    }
    BelluiWebViewHandle *h = (BelluiWebViewHandle *)handle;
    dispatch_async(dispatch_get_main_queue(), ^{
        BelluiWebViewClip *clip = [h clip];
        WKWebView *web = [clip webView];

        // Hand the keyboard focus back before the view goes away, so the
        // window is not left with a dead first responder.
        UIView *responder = belluiFindFirstResponder(web);
        if (responder) {
            [responder resignFirstResponder];
        }

        // Stop the engine before tearing the view down, so an in-flight load
        // cannot call back into a half-released view — and clear the
        // delegates first, so nothing already in flight reports through a
        // token the Go side has just dropped.
        [web setNavigationDelegate:nil];
        [web setUIDelegate:nil];
        [web stopLoading];
        [web removeFromSuperview];
        [clip setWebView:nil];
        [web release]; // balances the +1 from alloc; addSubview's went with removeFromSuperview
        [[clip navDelegate] release];
        [clip setNavDelegate:nil];
        [clip removeFromSuperview];
        [h setClip:nil];  // releases the handle's retain
        [h release];      // balances belluiWebViewCreate's alloc
    });
}
