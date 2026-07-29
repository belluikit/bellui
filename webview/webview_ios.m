//go:build ios

#import <UIKit/UIKit.h>
#import <WebKit/WebKit.h>

#import "webview_ios.h"

// Implemented in Go (webview_ios.go), all called on the main thread.
extern void belluiWebViewMessage(uint64_t token, char *origin, char *body);
extern void belluiWebViewEvalDone(uint64_t token, uint64_t callID, char *result, char *errMsg);
extern void belluiWebViewLoad(uint64_t token, int state, char *url, char *errMsg);
extern bool belluiWebViewNavigate(uint64_t token, char *url, int kind, bool mainFrame, bool newWindow);

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
@interface BelluiWebViewDelegate : NSObject <WKNavigationDelegate, WKUIDelegate>
@property(assign) uint64_t token;
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
    bool allow = belluiWebViewNavigate(self.token, cStr(url), kind, mainFrame, newWindow);
    decisionHandler(allow ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
}

- (void)webView:(WKWebView *)webView didStartProvisionalNavigation:(WKNavigation *)navigation {
    belluiWebViewLoad(self.token, belluiLoadLoading, cStr([[webView URL] absoluteString]), (char *)"");
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    belluiWebViewLoad(self.token, belluiLoadComplete, cStr([[webView URL] absoluteString]), (char *)"");
}

// reportFailure filters the two errors that are not failures to report: a
// cancellation, and a policy change. Both are navigations bellui itself
// stopped (StopLoading, or a denied OnNavigate), and the Go side already
// reported them — reporting again would deliver the same cancel twice.
- (void)reportFailure:(NSError *)error forView:(WKWebView *)webView {
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

void *belluiWebViewCreate(uint64_t token, const char *bridgeScript) {
    UIWindow *window = belluiAppWindow();
    if (!window) {
        return NULL;
    }

    BelluiWebViewClip *clip = [[BelluiWebViewClip alloc] initWithFrame:CGRectZero];
    [clip setClipsToBounds:YES];
    [clip setHidden:YES]; // shown once the host reports real geometry

    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];

    // The bridge is installed here or not at all: with no script, the page has
    // neither window.bellui nor a message handler to post to, which is the
    // guarantee behind EnableBridge being opt-in.
    if (bridgeScript && bridgeScript[0] != '\0') {
        BelluiWebViewBridge *bridge = [[BelluiWebViewBridge alloc] init];
        [bridge setToken:token];
        [[config userContentController] addScriptMessageHandler:bridge name:@"bellui"];
        [bridge release]; // the controller retains it

        WKUserScript *script =
            [[WKUserScript alloc] initWithSource:[NSString stringWithUTF8String:bridgeScript]
                                   injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                                forMainFrameOnly:NO];
        [[config userContentController] addUserScript:script];
        [script release];
    }

    WKWebView *web = [[WKWebView alloc] initWithFrame:CGRectZero configuration:config];
    [config release];

    BelluiWebViewDelegate *delegate = [[BelluiWebViewDelegate alloc] init];
    [delegate setToken:token];
    [web setNavigationDelegate:delegate]; // weak, so the clip owns the +1
    [web setUIDelegate:delegate];

    [clip addSubview:web];
    [clip setWebView:web];
    [clip setNavDelegate:delegate];
    [clip setToken:token];

    // Added to the WINDOW, as a sibling of the GLKView, never as its subview.
    // A view hosted inside the GLKView entangles with its CAMetalLayer and
    // stalls rendering — the #1124 lesson from BelluiTextInputView, which is
    // hosted here for the same reason. addSubview: retains the clip; it keeps
    // the +1 from alloc until belluiWebViewDestroy releases it.
    [window addSubview:clip];
    return clip;
}

void belluiWebViewLoadURL(void *handle, const char *url) {
    if (!handle || !url) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSString *str = [NSString stringWithUTF8String:url];
    NSURL *target = [NSURL URLWithString:str];
    if (!target || ![target scheme]) {
        return;
    }
    if ([[target scheme] isEqualToString:@"file"]) {
        [[clip webView] loadFileURL:target allowingReadAccessToURL:[target URLByDeletingLastPathComponent]];
        return;
    }
    [[clip webView] loadRequest:[NSURLRequest requestWithURL:target]];
}

void belluiWebViewLoadHTML(void *handle, const char *html, const char *baseURL) {
    if (!handle || !html) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSURL *base = nil;
    if (baseURL && baseURL[0] != '\0') {
        base = [NSURL URLWithString:[NSString stringWithUTF8String:baseURL]];
    }
    [[clip webView] loadHTMLString:[NSString stringWithUTF8String:html] baseURL:base];
}

bool belluiWebViewNav(void *handle, int op) {
    if (!handle) {
        return false;
    }
    WKWebView *web = [(BelluiWebViewClip *)handle webView];
    if (!web) {
        return false;
    }
    switch (op) {
    case belluiOpCanGoBack:
        return [web canGoBack] ? true : false;
    case belluiOpGoBack:
        return [web goBack] != nil;
    case belluiOpCanGoForward:
        return [web canGoForward] ? true : false;
    case belluiOpGoForward:
        return [web goForward] != nil;
    case belluiOpReload:
        return [web reload] != nil;
    case belluiOpStopLoading:
        [web stopLoading];
        return true;
    }
    return false;
}

void belluiWebViewEval(void *handle, const char *js, uint64_t callID, bool wantResult) {
    if (!handle || !js) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSString *source = [NSString stringWithUTF8String:js];
    if (!wantResult) {
        [[clip webView] evaluateJavaScript:source completionHandler:nil];
        return;
    }

    // The completion handler runs on the main thread, and carries only scalars
    // plus the token, so a result arriving after the view was destroyed simply
    // finds no registry entry on the Go side.
    uint64_t token = [clip token];
    [[clip webView] evaluateJavaScript:source
                     completionHandler:^(id value, NSError *error) {
                         if (error) {
                             // localizedDescription is only ever "A JavaScript
                             // exception occurred"; the message the script
                             // actually threw is in the userInfo, and is the
                             // only part worth reporting to the app.
                             NSString *msg = [[error userInfo] objectForKey:@"WKJavaScriptExceptionMessage"];
                             if (!msg || [msg length] == 0) {
                                 msg = [error localizedDescription];
                             }
                             belluiWebViewEvalDone(token, callID, (char *)"", cStr(msg));
                             return;
                         }
                         belluiWebViewEvalDone(token, callID, cStr(belluiJSONOf(value)), (char *)"");
                     }];
}

void belluiWebViewSetGeometry(void *handle,
                              double clipX, double clipY, double clipW, double clipH,
                              double viewX, double viewY, double viewW, double viewH) {
    if (!handle) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;

    // UIKit already uses a top-left origin, so unlike the macOS bridge the
    // incoming rectangles are used as they arrive.
    [clip setFrame:CGRectMake(clipX, clipY, clipW, clipH)];
    [[clip webView] setFrame:CGRectMake(viewX - clipX, viewY - clipY, viewW, viewH)];
}

void belluiWebViewSetVisible(void *handle, bool visible) {
    if (!handle) {
        return;
    }
    [(BelluiWebViewClip *)handle setHidden:!visible];
}

bool belluiWebViewHasFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    WKWebView *web = [clip webView];
    if (!web) {
        return false;
    }
    return belluiFindFirstResponder(web) != nil;
}

bool belluiWebViewTakeFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    WKWebView *web = [clip webView];
    if (!web) {
        return false;
    }
    [web becomeFirstResponder];
    return belluiWebViewHasFocus(handle);
}

bool belluiWebViewResignFocus(void *handle) {
    if (!handle) {
        return false;
    }
    if (!belluiWebViewHasFocus(handle)) {
        return true; // nothing of ours to give back
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;

    // Resign whichever view inside the web view actually holds it — WKWebView
    // installs its own content view, so resigning the WKWebView itself is not
    // always enough.
    UIView *responder = belluiFindFirstResponder([clip webView]);
    if (responder) {
        [responder resignFirstResponder];
    }

    // Deliberately nothing is made first responder in its place. On macOS the
    // focus has to land on the GLFW content view or keystrokes reach nothing,
    // but on iOS there is no key-handling view to hand it to: bellui's own
    // keyboard input goes through BelluiTextInputView, which claims the
    // responder only while an entry is focused. Claiming it here would raise
    // the software keyboard with nothing being edited.
    return !belluiWebViewHasFocus(handle);
}

void belluiWebViewDestroy(void *handle) {
    if (!handle) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    WKWebView *web = [clip webView];

    // Hand the keyboard focus back before the view goes away, so the window is
    // not left with a dead first responder.
    belluiWebViewResignFocus(handle);

    // Stop the engine before tearing the view down, so an in-flight load
    // cannot call back into a half-released view — and clear the delegates
    // first, so nothing that is already in flight reports through a token the
    // Go side has just dropped.
    [web setNavigationDelegate:nil];
    [web setUIDelegate:nil];
    [web stopLoading];
    [web removeFromSuperview];
    [clip setWebView:nil];
    [web release]; // balances the +1 from alloc; addSubview's went with removeFromSuperview
    [[clip navDelegate] release];
    [clip setNavDelegate:nil];
    [clip removeFromSuperview];
    [clip release];
}
