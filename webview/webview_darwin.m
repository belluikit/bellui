//go:build darwin && !ios && !wasm

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

#import "webview_darwin.h"

// Implemented in Go (webview_darwin.go), called on the main thread when the
// page posts a script message.
extern void belluiWebViewMessage(uint64_t token, char *body);

// MRC (no ARC), matching the other ObjC bridges in this repo.

// BelluiWebViewBridge receives window.webkit.messageHandlers.bellui.postMessage
// from the page and forwards it to Go by token. It is retained by the
// WKUserContentController, so it is released with the web view.
@interface BelluiWebViewBridge : NSObject <WKScriptMessageHandler>
@property(assign) uint64_t token;
@end

@implementation BelluiWebViewBridge

- (void)userContentController:(WKUserContentController *)controller
      didReceiveScriptMessage:(WKScriptMessage *)message {
    // Message delivery is on the main thread, which is where the bellui main
    // goroutine sits inside the GLFW event loop — the same model as the
    // accessibility AXPress callbacks, so no locking is needed on the Go side.
    NSString *body = [NSString stringWithFormat:@"%@", [message body]];
    belluiWebViewMessage(self.token, (char *)[body UTF8String]);
}

@end

// BelluiWebViewClip is the hole-punch container: a flipped, layer-backed view
// whose masksToBounds turns the host's bellui clip bounds into a real clip, so
// a web view inside a container.Scroll is cut off at the scroll's edge instead
// of overflowing it. Flipping gives it a top-left origin, so the web view's
// frame inside it needs no second coordinate conversion.
@interface BelluiWebViewClip : NSView
@property(assign) WKWebView *webView;
@end

@implementation BelluiWebViewClip

- (BOOL)isFlipped {
    return YES;
}

@end

void *belluiWebViewCreate(uintptr_t nsWindow, uint64_t token) {
    NSWindow *window = (NSWindow *)(void *)nsWindow;
    NSView *content = [window contentView];
    if (!content) {
        return NULL;
    }

    BelluiWebViewClip *clip = [[BelluiWebViewClip alloc] initWithFrame:NSZeroRect];
    [clip setWantsLayer:YES];
    [clip setLayer:[CALayer layer]];
    [[clip layer] setMasksToBounds:YES];
    [clip setHidden:YES]; // shown once the host reports real geometry

    // The page talks back through a named script message handler, so bellui
    // logic and page logic can drive each other (Go -> JS is belluiWebViewEval).
    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
    BelluiWebViewBridge *bridge = [[BelluiWebViewBridge alloc] init];
    [bridge setToken:token];
    [[config userContentController] addScriptMessageHandler:bridge name:@"bellui"];
    [bridge release]; // the controller retains it

    WKWebView *web = [[WKWebView alloc] initWithFrame:NSZeroRect configuration:config];
    [config release];

    [clip addSubview:web];
    [clip setWebView:web];

    // Added as a subview of the GLFW content view, so it composites above
    // everything bellui draws — the documented "nothing draws above this"
    // property of the hole punch. addSubview: retains the web view; the clip
    // keeps the +1 from alloc until belluiWebViewDestroy releases it.
    [content addSubview:clip];
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

void belluiWebViewEval(void *handle, const char *js) {
    if (!handle || !js) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    [[clip webView] evaluateJavaScript:[NSString stringWithUTF8String:js]
                    completionHandler:nil];
}

void belluiWebViewSetGeometry(void *handle,
                              double clipX, double clipY, double clipW, double clipH,
                              double viewX, double viewY, double viewW, double viewH) {
    if (!handle) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSView *content = [clip superview];
    if (!content) {
        return;
    }

    // The content view is not flipped, so the incoming top-left origin has to
    // be measured from the bottom of the content view instead.
    CGFloat height = [content bounds].size.height;
    [clip setFrame:NSMakeRect(clipX, height - clipY - clipH, clipW, clipH)];

    // The clip view *is* flipped, so the web view's frame stays in top-left
    // space; it is expressed relative to the clip's origin.
    [[clip webView] setFrame:NSMakeRect(viewX - clipX, viewY - clipY, viewW, viewH)];
}

void belluiWebViewSetVisible(void *handle, bool visible) {
    if (!handle) {
        return;
    }
    [(BelluiWebViewClip *)handle setHidden:!visible];
}

bool belluiWebViewTakeFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSWindow *window = [clip window];
    if (!window) {
        return false;
    }
    return [window makeFirstResponder:[clip webView]] ? true : false;
}

bool belluiWebViewHasFocus(void *handle) {
    if (!handle) {
        return false;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSWindow *window = [clip window];
    if (!window) {
        return false;
    }
    // WKWebView installs its own content view as first responder, so the
    // question is whether the responder is anywhere inside the web view.
    id responder = [window firstResponder];
    if (![responder isKindOfClass:[NSView class]]) {
        return false;
    }
    return [(NSView *)responder isDescendantOf:[clip webView]] ? true : false;
}

bool belluiWebViewResignFocus(void *handle) {
    if (!handle) {
        return false;
    }
    if (!belluiWebViewHasFocus(handle)) {
        return true; // nothing of ours to give back
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    NSWindow *window = [clip window];

    // The focus has to land on the GLFW content view specifically, not merely
    // leave the web view: GLFW installs its key handling there, so making the
    // *window* the first responder would route keystrokes to nothing at all.
    NSView *content = [window contentView];
    if (content && [window makeFirstResponder:content]) {
        return true;
    }
    return [window makeFirstResponder:nil] ? true : false;
}

void belluiWebViewDestroy(void *handle) {
    if (!handle) {
        return;
    }
    BelluiWebViewClip *clip = (BelluiWebViewClip *)handle;
    WKWebView *web = [clip webView];

    // Hand the keyboard focus back before the view goes away, so a window
    // outliving the web view is not left with a dead first responder.
    belluiWebViewResignFocus(handle);

    // Stop the engine before tearing the view down, so an in-flight load
    // cannot call back into a half-released view.
    [web stopLoading];
    [web removeFromSuperview];
    [clip setWebView:nil];
    [clip removeFromSuperview];
    [clip release];
}
