#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

void setFullScreen(bool full, void *win) {
    NSWindow *window = (NSWindow*)win;

    NSUInteger masks = [window styleMask];
    bool isFull = masks & NSWindowStyleMaskFullScreen;
    if (isFull == full) {
        return;
    }

    if (full) {
        // macOS 26 ignores the window's current screen for toggleFullScreen, so re-seat
        // the frame on that screen explicitly to force fullscreen on the right display.
        NSScreen *targetScreen = [window screen];
        if (targetScreen != nil) {
            NSRect frame = [window frame];
            NSRect screen = [targetScreen frame];
            frame.origin.x = screen.origin.x + (screen.size.width - frame.size.width) / 2;
            frame.origin.y = screen.origin.y + (screen.size.height - frame.size.height) / 2;
            [window setFrame:frame display:YES];
        }
    }

    [window toggleFullScreen:NULL];
}

// setWindowParent establishes an owner relationship so AppKit keeps child above
// parent, orders/minimizes them together and treats child as owned. A NULL
// parent removes the relationship. (fyne#598)
void setWindowParent(void *childPtr, void *parentPtr) {
    NSWindow *child = (NSWindow*)childPtr;
    if (child == nil) {
        return;
    }
    NSWindow *existing = [child parentWindow];
    if (existing != nil) {
        [existing removeChildWindow:child];
    }
    NSWindow *parent = (NSWindow*)parentPtr;
    if (parent != nil) {
        [parent addChildWindow:child ordered:NSWindowAbove];
    }
}

// canBecomeKey/Main override returning NO. While a parent window's class is
// swapped for a subclass carrying these, AppKit refuses to make it key or main,
// so keyboard focus cannot reach it and its title bar renders inactive — the
// closest AppKit gets to window-modally blocking a single free-floating window
// without a sheet (not free-floating) or an app-modal session (blocks every
// window). (fyne#676)
static BOOL belluiModalCannotBecomeKey(id self, SEL _cmd) { return NO; }

// Associated-object key holding a blocked parent's original class so it can be
// restored on unblock (and to guard against double-blocking).
static const char belluiModalOrigClassKey = 0;

// setWindowModalBlocked blocks (blocked=true) or restores keyboard/main focus on
// parentPtr by swapping its class, per instance, to a dynamically-created
// subclass overriding canBecomeKeyWindow/canBecomeMainWindow. The original class
// is stashed via an associated object and restored on unblock. Idempotent.
void setWindowModalBlocked(void *parentPtr, bool blocked) {
    NSWindow *parent = (NSWindow*)parentPtr;
    if (parent == nil) {
        return;
    }

    if (blocked) {
        if (objc_getAssociatedObject(parent, &belluiModalOrigClassKey) != nil) {
            return; // already blocked
        }
        Class current = object_getClass(parent);
        NSString *subName =
            [NSString stringWithFormat:@"BelluiModalBlocked_%s", class_getName(current)];
        Class sub = objc_lookUpClass([subName UTF8String]);
        if (sub == nil) {
            sub = objc_allocateClassPair(current, [subName UTF8String], 0);
            if (sub != nil) {
                // BOOL(id, SEL): "c@:" is the classic BOOL return encoding and
                // works for direct msgSend dispatch on both x86_64 and arm64.
                class_addMethod(sub, @selector(canBecomeKeyWindow),
                                (IMP)belluiModalCannotBecomeKey, "c@:");
                class_addMethod(sub, @selector(canBecomeMainWindow),
                                (IMP)belluiModalCannotBecomeKey, "c@:");
                objc_registerClassPair(sub);
            }
        }
        if (sub != nil) {
            objc_setAssociatedObject(parent, &belluiModalOrigClassKey,
                                     current, OBJC_ASSOCIATION_ASSIGN);
            object_setClass(parent, sub);
            // If the parent currently holds focus, hand it to the topmost child
            // dialog so the keyboard follows the block.
            if ([parent isKeyWindow] || [parent isMainWindow]) {
                NSWindow *childDialog = [[parent childWindows] lastObject];
                if (childDialog != nil) {
                    [childDialog makeKeyAndOrderFront:nil];
                }
            }
        }
    } else {
        Class orig = (Class)objc_getAssociatedObject(parent, &belluiModalOrigClassKey);
        if (orig != nil) {
            object_setClass(parent, orig);
            objc_setAssociatedObject(parent, &belluiModalOrigClassKey,
                                     nil, OBJC_ASSOCIATION_ASSIGN);
        }
    }
}

// isWindowModalBlocked reports whether parentPtr is currently key/main-blocked by
// setWindowModalBlocked (introspection for tests).
bool isWindowModalBlocked(void *parentPtr) {
    NSWindow *parent = (NSWindow*)parentPtr;
    if (parent == nil) {
        return false;
    }
    return objc_getAssociatedObject(parent, &belluiModalOrigClassKey) != nil
        && ![parent canBecomeKeyWindow];
}

void setFullScreenSecondary(bool full, void *win) {
    NSWindow *window = (NSWindow*)win;

    NSUInteger masks = [window styleMask];
    bool isFull = (masks & NSWindowStyleMaskFullScreen) != 0;
    if (isFull == full) {
        return;
    }

    if (full) {
        // pick a screen that is not the one the window is already on, so launching
        // on a secondary display still sends "secondary fullscreen" to the other display.
        NSScreen *currentScreen = [window screen];
        NSScreen *targetScreen = nil;
        for (NSScreen *screen in [NSScreen screens]) {
            if (screen != currentScreen) {
                targetScreen = screen;
                break;
            }
        }
        if (targetScreen != nil) {
            NSRect frame = [window frame];
            NSRect screen = [targetScreen frame];
            frame.origin.x = screen.origin.x + (screen.size.width - frame.size.width) / 2;
            frame.origin.y = screen.origin.y + (screen.size.height - frame.size.height) / 2;
            [window setFrame:frame display:YES];
        }
    }

    [window toggleFullScreen:NULL];
}
