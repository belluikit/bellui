// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && ios

#include "_cgo_export.h"
#include <pthread.h>
#include <stdio.h>
#include <sys/utsname.h>

#import <UIKit/UIKit.h>
#import <MobileCoreServices/MobileCoreServices.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <GLKit/GLKit.h>
#import <UserNotifications/UserNotifications.h>

struct utsname sysInfo;

static CGFloat keyboardHeight;

@interface GoAppAppController : GLKViewController<UIContentContainer, GLKViewDelegate>
@end

// BelluiTextInputProxy is the interface the app controller and the C entry points
// (setKeyboardState / belluiInputViewSetTextLayout / belluiKeyboardInputView) use to
// drive the hidden text-input surface: BelluiTextInputView, a bare
// UIView<UITextInput> that intercepts edits at source (insertText:/
// deleteBackward:/setMarkedText:), the Flutter FlutterTextInputView model, and
// owns its own tokenizer (pre-rebrand tracker #1113). It replaced the GoInputView UITextView proxy
// (which reverse-engineered edits by diffing textViewDidChange:) at the pre-rebrand tracker #1119
// cutover, after the pre-rebrand tracker #1118 on-device IME/CJK + VoiceOver gate passed.
@protocol BelluiTextInputProxy <NSObject>
// belluiSetDocument mirrors the focused editor's text + selection (rune offsets,
// pushed from Go via setKeyboardState) so the IME composes against real content.
- (void)belluiSetDocument:(NSString *)text selStart:(int)selStart selEnd:(int)selEnd;
// belluiSetTextLayoutWire pushes bellui's pre-rebrand tracker #1107 visual-layout sidecar so the view
// answers VoiceOver's per-line/caret geometry from bellui's real layout.
- (void)belluiSetTextLayoutWire:(const char *)wire originX:(double)originX originY:(double)originY scale:(double)scale;
@end

@class BelluiTextInputView;

// BelluiTextPosition/BelluiTextRange are the bare BelluiTextInputView's own
// UITextPosition/UITextRange (pre-rebrand tracker #1117), backed by a UTF-16 code-unit index into
// the document. A UITextView owns these types internally; a bare UITextInput
// must supply them.
@interface BelluiTextPosition : UITextPosition
@property(nonatomic, readonly) NSUInteger index;
+ (instancetype)positionWithIndex:(NSUInteger)index;
@end

@interface BelluiTextRange : UITextRange<NSCopying>
@property(nonatomic, readonly) NSRange range;
+ (instancetype)rangeWithNSRange:(NSRange)range;
@end

// BelluiTextInputView is the bare UIView<UITextInput> text-input surface (pre-rebrand tracker #1117,
// default since the pre-rebrand tracker #1119 cutover), the Flutter FlutterTextInputView model. It
// owns an authoritative UTF-16 document mirror (text + selection + marked range)
// that it mutates optimistically the instant UIKit calls insertText:/
// deleteBackward:/setMarkedText:/replaceRange: — so UIKit reads back consistent
// state — and forwards each edit to the focused bellui editor at source via the
// keyboardTyped/keyboardReplace/keyboardComposing/keyboardComposeCommit/
// keyboardSetSelection up-calls. belluiSetDocument resynchronises the mirror from
// Go; the pre-rebrand tracker #1107 sidecar and the pre-rebrand tracker #570 VoiceOver focus handshake ride on it.
@interface BelluiTextInputView : UIView<UITextInput, UITextInputTraits, BelluiTextInputProxy> {
    NSMutableString *_text;   // UTF-16 document mirror
    NSRange _selectedRange;   // caret/selection, UTF-16 units
    NSRange _markedRange;     // IME preedit, {NSNotFound,0} when none
    // Dedicated undo manager holding only PROXY actions mirroring bellui's
    // undo/redo availability (pre-rebrand tracker #1134, the Flutter FlutterUndoManagerPlugin
    // model): bellui owns the history; the manager exists so the iPad keyboard's
    // undo/redo bar keys and the system undo gestures light up and route back
    // to bellui (keyboardUndoRedo). Lazily created by the undoManager override.
    NSUndoManager *_belluiUndoManager;
    UITextInputStringTokenizer *_tokenizer;
    // a11yActive is YES only while this view stands in as the VoiceOver editing
    // element (the a11y bridge's swap, pre-rebrand tracker #1124). The view lives in the window from
    // startup with a zero frame; claiming accessibility while parked would leave
    // a stray zero-frame text-input element in every bellui window — an invalid
    // focus target (VoiceOver refuses zero-frame elements) sitting beside the
    // real element tree (pre-rebrand tracker #951).
@public
    BOOL a11yActive;
    // syncingFromGo is YES while belluiSetDocument: applies a Go-pushed state, so
    // the selection setter does not echo it back with keyboardSetSelection.
    BOOL syncingFromGo;
    // Native-selection confirmation tracking (pre-rebrand tracker #1124) — the Flutter guarantee
    // (native _selectedTextRange is authoritative, never clobbered by a stale
    // framework push) adapted to bellui's ASYNC editor bridge. When a native caret
    // move (setSelectedTextRange:, e.g. VoiceOver line/word/char navigation) sets
    // _selectedRange and forwards it to bellui, bellui applies it only asynchronously,
    // so the per-repaint bellui→native push (belluiSetDocument:) briefly still carries
    // bellui's OLD caret. Letting that stale push overwrite _selectedRange collapses
    // VoiceOver's move — and VoiceOver, reading back the stale value, resets the
    // caret to 0. So after a native move we HOLD _selectedRange authoritatively and
    // reject every push whose selection differs from the awaited range, until bellui
    // echoes back exactly that range (it has caught up) — then normal sync resumes.
    // _awaitingPushes bounds the wait (a never-echoed native set, e.g. one bellui
    // clamped, can't wedge the sync forever). Text always syncs regardless.
    NSRange _awaitingSel;
    BOOL _awaitingConfirm;
    int _awaitingPushes;
    // Visual-layout sidecar (pre-rebrand tracker #1107).
    CGRect *belluiLayoutRects;
    int belluiLayoutBase;
    int belluiLayoutCount;
    // Visual line boundaries from the sidecar L: section (pre-rebrand tracker #995), driving the
    // BelluiTokenizer's Line granularity so VoiceOver navigates bellui's real
    // visual lines (pre-rebrand tracker #1113). belluiLineBounds[k] is the UTF-16 start of visual
    // line k; line k spans [belluiLineBounds[k], belluiLineBounds[k+1]).
    int *belluiLineBounds;
    int belluiLineCount;
}
@property(nonatomic, assign) id<UITextInputDelegate> inputDelegate;
@property(nonatomic, copy) NSDictionary *markedTextStyle;
@property(nonatomic) UITextAutocapitalizationType autocapitalizationType;
@property(nonatomic) UITextAutocorrectionType autocorrectionType;
@property(nonatomic) UIKeyboardType keyboardType;
@property(nonatomic) UIReturnKeyType returnKeyType;
@property(nonatomic) UITextSpellCheckingType spellCheckingType;
@property(nonatomic) BOOL enablesReturnKeyAutomatically;
@property(nonatomic) UIKeyboardAppearance keyboardAppearance;
// belluiLineRangeForOffset returns the visual-line range (UTF-16) enclosing an
// offset, from the sidecar's real bellui lines (falling back to logical '\n'
// lines); the BelluiTokenizer's Line granularity is built on it (pre-rebrand tracker #1113).
- (NSRange)belluiLineRangeForOffset:(NSUInteger)off;
// belluiSetUndoState mirrors bellui's undo/redo availability into the proxy
// NSUndoManager (pre-rebrand tracker #1134); called from setKeyboardUndoState on the main queue.
- (void)belluiSetUndoState:(BOOL)canUndo canRedo:(BOOL)canRedo;
@end

// BelluiTokenizer is the BelluiTextInputView's Line-granularity tokenizer (pre-rebrand tracker #1113),
// the counterpart of Flutter's FlutterTokenizer: the stock
// UITextInputStringTokenizer mishandles UITextGranularityLine (VoiceOver's
// line-swipe reject-beeps), so this subclass answers Line queries from bellui's
// real visual lines. A UITextView subclass cannot replace its tokenizer —
// owning it is exactly what the bare-UIView rewrite unlocked (the retired
// GoInputView UITextView proxy could not).
@interface BelluiTokenizer : UITextInputStringTokenizer
@end

// Armed by the accessibility bridge just before it moves VoiceOver focus
// onto the input proxy; consumed by accessibilityElementDidBecomeFocused
// (pre-rebrand tracker #570). Main thread only.
static BOOL belluiKeyboardRaisePending = NO;

void belluiKeyboardExpectRaise(void) {
    belluiKeyboardRaisePending = YES;
}

@interface GoAppAppDelegate : UIResponder<UIApplicationDelegate>
@property (strong, nonatomic) UIWindow *window;
@property (strong, nonatomic) GoAppAppController *controller;
@end

@implementation GoAppAppDelegate
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
    int scale = 1;
    if ([[UIScreen mainScreen] respondsToSelector:@selector(displayLinkWithTarget:selector:)]) {
		scale = (int)[UIScreen mainScreen].scale; // either 1.0, 2.0, or 3.0.
	}
    CGSize size = [UIScreen mainScreen].nativeBounds.size;
    setDisplayMetrics((int)size.width, (int)size.height, scale);

	lifecycleAlive();
	self.window = [[UIWindow alloc] initWithFrame:[[UIScreen mainScreen] bounds]];
	self.controller = [[GoAppAppController alloc] initWithNibName:nil bundle:nil];
	self.window.rootViewController = self.controller;
	[self.window makeKeyAndVisible];

    // update insets once key window is set
	UIInterfaceOrientation orientation = [[UIApplication sharedApplication] statusBarOrientation];
	updateConfig((int)size.width, (int)size.height, orientation);

	UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
	center.delegate = (id) self;

	return YES;
}

- (void)applicationDidBecomeActive:(UIApplication * )application {
	lifecycleFocused();
}

- (void)applicationWillResignActive:(UIApplication *)application {
	lifecycleVisible();
}

- (void)applicationDidEnterBackground:(UIApplication *)application {
	lifecycleAlive();
}

- (void)applicationWillTerminate:(UIApplication *)application {
	lifecycleDead();
}

- (void)applicationDidReceiveMemoryWarning:(UIApplication *)application {
	lifecycleMemoryWarning();
}

- (void)documentPicker:(UIDocumentPickerViewController *)controller didPickDocumentsAtURLs:(NSArray <NSURL *>*)urls {
    if (controller.allowsMultipleSelection) {
        // Multi-select session (#255): hand every URL across, each with its
        // own retained pointer for the per-file security scope, then complete.
        for (NSURL* url in urls) {
            NSURL* toClose = NULL;
            BOOL secured = [url startAccessingSecurityScopedResource];
            if (secured) {
                toClose = url;
                CFRetain((CFTypeRef)toClose);
            }
            filePickerMultiAppend((char*)[[url description] UTF8String], toClose);
        }
        filePickerMultiComplete();
        return;
    }

    if ([urls count] == 0) {
        return;
    }

    NSURL* url = urls[0];
    NSURL* toClose = NULL;
    BOOL secured = [url startAccessingSecurityScopedResource];
    if (secured) {
        // Go keeps this pointer until the app closes the picked file, long
        // after this delegate's autorelease pool drains — retain it and let
        // closeFileResource release it (the #250 bridge invariant).
        toClose = url;
        CFRetain((CFTypeRef)toClose);
    }

    filePickerReturned((char*)[[url description] UTF8String], toClose);
}

- (void)documentPickerWasCancelled:(UIDocumentPickerViewController *)controller {
    if (controller.allowsMultipleSelection) {
        // Nothing appended: the empty completion is the cancel signal.
        filePickerMultiComplete();
        return;
    }
    filePickerReturned("", NULL);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler {
	completionHandler(UNNotificationPresentationOptionAlert);
}

// forwardOpenURL hands an "open with" URL to Go. For a file:// URL handed over by
// another app it does the security-scoped-resource dance (pre-rebrand tracker #622): access the
// original, copy it into our own sandbox (NSTemporaryDirectory) under a
// coordinated read, release access, and forward the readable copy's URL — so the
// app can read it regardless of open-in-place vs the default copy-to-Inbox mode,
// and without leaking the security scope across the async (buffered) delivery.
// Custom-scheme / web URLs are forwarded verbatim.
static void forwardOpenURL(NSURL *url) {
	if (![url isFileURL]) {
		onOpenIntent((char*)[[url absoluteString] UTF8String]);
		return;
	}

	BOOL secured = [url startAccessingSecurityScopedResource];
	NSString *name = [url lastPathComponent];
	if ([name length] == 0) {
		name = @"bellui-open-file";
	}
	NSURL *dest = [NSURL fileURLWithPath:[NSTemporaryDirectory() stringByAppendingPathComponent:name]];

	NSFileManager *fm = [NSFileManager defaultManager];
	[fm removeItemAtURL:dest error:NULL]; // clear any stale copy from a prior open

	__block BOOL copied = NO;
	NSError *coordErr = nil;
	NSFileCoordinator *coord = [[[NSFileCoordinator alloc] initWithFilePresenter:nil] autorelease];
	[coord coordinateReadingItemAtURL:url options:NSFileCoordinatorReadingWithoutChanges
	                            error:&coordErr byAccessor:^(NSURL *readURL) {
		copied = [fm copyItemAtURL:readURL toURL:dest error:NULL];
	}];

	if (secured) {
		[url stopAccessingSecurityScopedResource];
	}

	// Forward the sandbox copy when it succeeded, else fall back to the original
	// URL (already-in-sandbox Inbox files copy fine; the fallback covers the rest).
	onOpenIntent((char*)[[(copied ? dest : url) absoluteString] UTF8String]);
}

// application:openURL: delivers an "open with" launch or deep-link — a custom
// URL scheme (myapp://…) or a file:// URL handed over by another app (pre-rebrand tracker #622).
// The URL is forwarded to Go via onOpenIntent, which buffers it until the app
// registers a handler; also fires on cold launch (after didFinishLaunching).
- (BOOL)application:(UIApplication *)application
            openURL:(NSURL *)url
            options:(NSDictionary<UIApplicationOpenURLOptionsKey, id> *)options {
	if (url == nil) {
		return NO;
	}
	forwardOpenURL(url);
	return YES;
}

// application:continueUserActivity: delivers a Universal Link (a web URL routed
// to the app via an associated domain), forwarded the same way (pre-rebrand tracker #622).
- (BOOL)application:(UIApplication *)application
continueUserActivity:(NSUserActivity *)userActivity
  restorationHandler:(void (^)(NSArray<id<UIUserActivityRestoring>> *restorableObjects))restorationHandler {
	if ([userActivity.activityType isEqualToString:NSUserActivityTypeBrowsingWeb] &&
	    userActivity.webpageURL != nil) {
		onOpenIntent((char*)[[userActivity.webpageURL absoluteString] UTF8String]);
		return YES;
	}
	return NO;
}
@end

@interface GoAppAppController ()
@property (strong, nonatomic) EAGLContext *context;
@property (strong, nonatomic) GLKView *glview;
@property (strong, nonatomic) UIView<BelluiTextInputProxy, UITextInputTraits> *inputView;
@end

@implementation GoAppAppController
- (void)viewWillAppear:(BOOL)animated
{
	// TODO: replace by swapping out GLKViewController for a UIVIewController.
	[super viewWillAppear:animated];
	self.paused = YES;

    [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(keyboardWillShow:) name:UIKeyboardWillShowNotification object:nil];
    [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(keyboardWillHide:) name:UIKeyboardWillHideNotification object:nil];
}

- (void)viewWillDisappear:(BOOL)animated {
    [super viewWillDisappear:animated];

    [[NSNotificationCenter defaultCenter] removeObserver:self name:UIKeyboardWillShowNotification object:nil];
    [[NSNotificationCenter defaultCenter] removeObserver:self name:UIKeyboardWillHideNotification object:nil];
}

- (void)viewDidLoad {
	[super viewDidLoad];
	self.context = [[EAGLContext alloc] initWithAPI:kEAGLRenderingAPIOpenGLES2];
	// The bare-UIView UITextInput proxy (pre-rebrand tracker #1117): no UITextView delegate/editable/
	// scroll knobs — it owns its document model directly. A non-No autocorrection
	// type is required for the IME suggestion / CJK candidate bar to appear over
	// the hidden proxy (pre-rebrand tracker #771); it is suppressed entirely when set to No.
	BelluiTextInputView *bare = [[BelluiTextInputView alloc] initWithFrame:CGRectMake(0, 0, 0, 0)];
	bare.autocapitalizationType = UITextAutocapitalizationTypeNone;
	bare.autocorrectionType = UITextAutocorrectionTypeDefault;
	self.inputView = bare;
	// Host the input view OUTSIDE the GLKView — the render view whose layer
	// carries the CAMetalLayer (metalLayerForKeyWindow adds it as a sublayer of
	// rootViewController.view.layer). Flutter's separate inputHider model: as a
	// GLKView subview, the bare view's keyboard presentation stalled the render
	// surface — the pre-rebrand tracker #1124 real-tap freeze.
	UIWindow *win = ((GoAppAppDelegate *)[UIApplication sharedApplication].delegate).window;
	[win addSubview:self.inputView];
	self.glview = (GLKView*)self.view;
	self.glview.drawableDepthFormat = GLKViewDrawableDepthFormat24;
	self.glview.multipleTouchEnabled = true; // TODO expose setting to user.
	self.glview.context = self.context;
	self.glview.userInteractionEnabled = YES;
	//self.glview.enableSetNeedsDisplay = YES; // only invoked once

	// Do not use the GLKViewController draw loop.
	//self.paused = YES;
	//self.resumeOnDidBecomeActive = NO;
	//self.preferredFramesPerSecond = 0;

	int scale = 1;
	if ([[UIScreen mainScreen] respondsToSelector:@selector(displayLinkWithTarget:selector:)]) {
		scale = (int)[UIScreen mainScreen].scale; // either 1.0, 2.0, or 3.0.
	}
	setScreen(scale);

	CGSize size = [UIScreen mainScreen].nativeBounds.size;
	UIInterfaceOrientation orientation = [[UIApplication sharedApplication] statusBarOrientation];
	updateConfig((int)size.width, (int)size.height, orientation);

    self.glview.enableSetNeedsDisplay = NO;
    CADisplayLink* displayLink = [CADisplayLink displayLinkWithTarget:self selector:@selector(render:)];
    // NSRunLoopCommonModes (not NSDefaultRunLoopMode): keep the render tick — and
    // thus the WebGPU frame-service (drawtick), which paces the paint loop — firing
    // during UIKit tracking/modal runloop modes (native text selection menu,
    // keyboard, scrolling). Under the default mode the link stops in those modes,
    // which under WebGPU wedges the paint goroutine on the publish handshake when a
    // bare UIView<UITextInput> first responder raises native UI (pre-rebrand tracker #1124/#1125).
    [displayLink addToRunLoop:[NSRunLoop currentRunLoop] forMode:NSRunLoopCommonModes];
}

- (void)viewWillTransitionToSize:(CGSize)ptSize withTransitionCoordinator:(id<UIViewControllerTransitionCoordinator>)coordinator {
	[coordinator animateAlongsideTransition:^(id<UIViewControllerTransitionCoordinatorContext> context) {
		// TODO(crawshaw): come up with a plan to handle animations.
	} completion:^(id<UIViewControllerTransitionCoordinatorContext> context) {
		UIInterfaceOrientation orientation = [[UIApplication sharedApplication] statusBarOrientation];
		CGSize size = [UIScreen mainScreen].nativeBounds.size;
		updateConfig((int)size.width, (int)size.height, orientation);
	}];
}

- (void)render:(CADisplayLink*)displayLink {
#ifndef BELLUI_WEBGPU
    [self.glview display];
#else
    // Under -tags webgpu the GLKView/GLES draw is skipped (it is the broken path
    // on the iOS Simulator, pre-rebrand tracker #195); the bellui mobile driver's own paint ticker
    // drives the WebGPU painter, which renders into the CAMetalLayer. But the GL
    // worker + Publish handshake (app.Publish's vestigial gl.Flush) still needs
    // servicing every frame, and drawInRect (below) — which used to do it — stops
    // firing when a bare UIView<UITextInput> first responder raises native UI,
    // deadlocking the renderer (pre-rebrand tracker #1124/#1125). This CADisplayLink keeps firing
    // regardless, so service the handshake here, non-blocking.
    drawtick();
#endif
}

- (void)glkView:(GLKView *)view drawInRect:(CGRect)rect {
    // GLES-fallback path only; under WebGPU render: drives drawtick() instead
    // (drawInRect is unreliable there, pre-rebrand tracker #1125), so do not double-drive the worker.
#ifndef BELLUI_WEBGPU
    drawloop();
#endif
}

#define TOUCH_TYPE_BEGIN 0 // touch.TypeBegin
#define TOUCH_TYPE_MOVE  1 // touch.TypeMove
#define TOUCH_TYPE_END   2 // touch.TypeEnd
#define TOUCH_TYPE_CANCEL 3 // touch.TypeCancel

// sendTouches reports touches to Go. When touches begin and no other touch is
// down (every touch of the event is one of these), any id Go still holds is
// from a touch whose end UIKit never reported: touchesFresh frees them, once,
// before the first of these touches is given an id. Freeing them per touch
// gave two fingers that landed together the same id (#392).
static void sendTouches(int change, NSSet* touches, UIEvent* event) {
	CGFloat scale = [UIScreen mainScreen].nativeScale;
	if (change == TOUCH_TYPE_BEGIN) {
		BOOL fresh = YES;
		for (UITouch* t in [event allTouches]) {
			if (![touches containsObject:t]) {
				fresh = NO;
				break;
			}
		}
		if (fresh) {
			touchesFresh();
		}
	}
	for (UITouch* touch in touches) {
		CGPoint p = [touch locationInView:touch.view];
		sendTouch((GoUintptr)touch, (GoUintptr)change, p.x*scale, p.y*scale);
	}
}

- (void)touchesBegan:(NSSet*)touches withEvent:(UIEvent*)event {
	sendTouches(TOUCH_TYPE_BEGIN, touches, event);
}

- (void)touchesMoved:(NSSet*)touches withEvent:(UIEvent*)event {
	sendTouches(TOUCH_TYPE_MOVE, touches, event);
}

- (void)touchesEnded:(NSSet*)touches withEvent:(UIEvent*)event {
	sendTouches(TOUCH_TYPE_END, touches, event);
}

- (void)touchesCanceled:(NSSet*)touches withEvent:(UIEvent*)event {
    sendTouches(TOUCH_TYPE_CANCEL, touches, event);
}

- (void) traitCollectionDidChange: (UITraitCollection *) previousTraitCollection {
    [super traitCollectionDidChange: previousTraitCollection];

	UIInterfaceOrientation orientation = [[UIApplication sharedApplication] statusBarOrientation];
	CGSize size = [UIScreen mainScreen].nativeBounds.size;
	updateConfig((int)size.width, (int)size.height, orientation);
}

- (void)keyboardWillShow:(NSNotification *)note {
    CGSize keyboardSize = [[[note userInfo] objectForKey:UIKeyboardFrameEndUserInfoKey] CGRectValue].size;
    keyboardHeight = keyboardSize.height;

    CGSize size = [UIScreen mainScreen].nativeBounds.size;
	UIInterfaceOrientation orientation = [[UIApplication sharedApplication] statusBarOrientation];
	updateConfig((int)size.width, (int)size.height, orientation);
}

- (void)keyboardWillHide:(NSNotification *)note {
    keyboardHeight = 0;

    CGSize size = [UIScreen mainScreen].nativeBounds.size;
	UIInterfaceOrientation orientation = [[UIApplication sharedApplication] statusBarOrientation];
	updateConfig((int)size.width, (int)size.height, orientation);
}

@end

// belluiRuneOffset returns the number of Unicode code points (Go runes) in the
// first utf16idx UTF-16 code units of s — converting an iOS UITextInput offset
// into the rune-offset space core.AccessibleTexter uses (pre-rebrand tracker #771). A surrogate
// pair (one code point, two UTF-16 units) counts once.
static int belluiRuneOffset(NSString *s, NSInteger utf16idx) {
    if (s == nil || utf16idx <= 0) {
        return 0;
    }
    NSInteger len = (NSInteger)s.length;
    if (utf16idx > len) {
        utf16idx = len;
    }
    int runes = 0;
    NSInteger i = 0;
    while (i < utf16idx) {
        unichar c = [s characterAtIndex:i];
        if (c >= 0xD800 && c <= 0xDBFF && i + 1 < len) {
            i += 2;
        } else {
            i += 1;
        }
        runes++;
    }
    return runes;
}

// belluiUTF16Offset is the inverse of belluiRuneOffset: the UTF-16 code-unit offset
// of the runeIdx-th code point in s. Used to map a rune-offset selection pushed
// from Go back onto an iOS UITextPosition (pre-rebrand tracker #771).
static NSInteger belluiUTF16Offset(NSString *s, int runeIdx) {
    if (s == nil || runeIdx <= 0) {
        return 0;
    }
    NSInteger len = (NSInteger)s.length;
    int runes = 0;
    NSInteger i = 0;
    while (i < len && runes < runeIdx) {
        unichar c = [s characterAtIndex:i];
        if (c >= 0xD800 && c <= 0xDBFF && i + 1 < len) {
            i += 2;
        } else {
            i += 1;
        }
        runes++;
    }
    return i;
}

// belluiCommonPrefix returns the number of leading UTF-16 units a and b share,
// snapped back so a surrogate pair is never split.
static NSUInteger belluiCommonPrefix(NSString *a, NSString *b) {
    NSUInteger n = MIN(a.length, b.length);
    NSUInteger i = 0;
    while (i < n && [a characterAtIndex:i] == [b characterAtIndex:i]) {
        i++;
    }
    if (i > 0) {
        unichar c = [a characterAtIndex:i - 1];
        if (c >= 0xD800 && c <= 0xDBFF) {
            i--; // do not split a surrogate pair
        }
    }
    return i;
}

// belluiCommonSuffix returns the number of trailing UTF-16 units a and b share,
// capped at maxLen and snapped so a surrogate pair is never split.
static NSUInteger belluiCommonSuffix(NSString *a, NSString *b, NSUInteger maxLen) {
    NSUInteger i = 0;
    while (i < maxLen && [a characterAtIndex:a.length - 1 - i] == [b characterAtIndex:b.length - 1 - i]) {
        i++;
    }
    if (i > 0) {
        unichar c = [a characterAtIndex:a.length - i];
        if (c >= 0xDC00 && c <= 0xDFFF) {
            i--; // do not split a surrogate pair
        }
    }
    return i;
}

// BelluiTextSelectionRect carries one sidecar-derived rectangle to VoiceOver via
// selectionRectsForRange: (pre-rebrand tracker #1107) — the method VoiceOver actually uses to draw
// the editing/line cursor ring (device-confirmed; it does not call
// firstRectForRange: for this). UITextSelectionRect is abstract, so a concrete
// subclass overriding the geometry getters is required.
@interface BelluiTextSelectionRect : UITextSelectionRect {
    CGRect _belluiRect;
}
+ (instancetype)withRect:(CGRect)r;
@end

@implementation BelluiTextSelectionRect
+ (instancetype)withRect:(CGRect)r {
    BelluiTextSelectionRect *x = [[[self alloc] init] autorelease];
    x->_belluiRect = r;
    return x;
}
- (CGRect)rect { return _belluiRect; }
- (NSWritingDirection)writingDirection { return NSWritingDirectionNatural; }
- (BOOL)containsStart { return YES; }
- (BOOL)containsEnd { return YES; }
- (BOOL)isVertical { return NO; }
@end

// belluiParseLayoutWire parses bellui's visual-layout sidecar (pre-rebrand tracker #1107) into an array
// of point-space per-UTF-16-character rectangles for the BelluiTextInputView
// geometry overrides. wire is the L:/C: encoding from
// mobileA11yTextLayout; only the C: section (per-UTF-16 rects, native pixels,
// canvas space) is used. originX/originY is the node's origin in native pixels
// and scale the native screen scale, so a canvas-pixel rect maps to view-local
// points as ((px - origin) / scale) — the same transform the bridge uses for
// the element frame. A NULL/empty wire or missing C: section yields no rects
// (outputs cleared), so the callers fall back to their native geometry. The
// caller owns *outRects (free it before calling / in dealloc).
static void belluiParseLayoutWire(const char *wire, double originX, double originY, double scale,
                                CGRect **outRects, int *outBase, int *outCount) {
    *outRects = NULL;
    *outBase = 0;
    *outCount = 0;
    if (wire == NULL || wire[0] == '\0' || scale <= 0) {
        return;
    }
    const char *c = strstr(wire, "C:");
    if (c == NULL) {
        return; // only the L: section present (no selection window) — no rects
    }
    c += 2;
    char *end = NULL;
    long base = strtol(c, &end, 10);
    if (end == c || *end != ':') {
        return;
    }
    c = end + 1;

    int cap = 16, n = 0;
    CGRect *rects = malloc(sizeof(CGRect) * cap);
    while (*c != '\0') {
        if (n == cap) {
            cap *= 2;
            rects = realloc(rects, sizeof(CGRect) * cap);
        }
        if (*c == '-') {
            rects[n++] = CGRectNull;
            c++;
        } else {
            long x = strtol(c, &end, 10); if (*end != ',') break; c = end + 1;
            long y = strtol(c, &end, 10); if (*end != ',') break; c = end + 1;
            long w = strtol(c, &end, 10); if (*end != ',') break; c = end + 1;
            long h = strtol(c, &end, 10); if (end == c) break; c = end;
            rects[n++] = CGRectMake((x - originX) / scale, (y - originY) / scale,
                                    w / scale, h / scale);
        }
        if (*c == ';') {
            c++;
        } else {
            break;
        }
    }
    *outRects = rects;
    *outBase = (int)base;
    *outCount = n;
}

// belluiParseLineWire parses the sidecar's L: section (pre-rebrand tracker #995) — a comma-separated
// list of UTF-16 boundary offsets where visual line k spans [b_k, b_{k+1}) —
// into an int array, for the BelluiTokenizer line model (pre-rebrand tracker #1113). Outputs cleared
// when there is no L: section or fewer than two boundaries (no line). The caller
// owns *outBounds (free it before calling / in dealloc).
static void belluiParseLineWire(const char *wire, int **outBounds, int *outCount) {
    *outBounds = NULL;
    *outCount = 0;
    if (wire == NULL || wire[0] == '\0') {
        return;
    }
    const char *c = strstr(wire, "L:");
    if (c == NULL) {
        return;
    }
    c += 2;
    int cap = 16, n = 0;
    int *bounds = malloc(sizeof(int) * cap);
    while (*c != '\0' && *c != '\n') {
        char *end = NULL;
        long v = strtol(c, &end, 10);
        if (end == c) {
            break;
        }
        if (n == cap) {
            cap *= 2;
            bounds = realloc(bounds, sizeof(int) * cap);
        }
        bounds[n++] = (int)v;
        c = end;
        if (*c == ',') {
            c++;
        } else {
            break;
        }
    }
    if (n < 2) {
        free(bounds);
        return;
    }
    *outBounds = bounds;
    *outCount = n;
}

// belluiCharRectFromArray returns the local-point rectangle for a UTF-16 offset,
// or CGRectNull when the sidecar has no geometry for it.
static CGRect belluiCharRectFromArray(CGRect *rects, int base, int count, NSInteger off) {
    NSInteger i = off - base;
    if (i < 0 || i >= count) {
        return CGRectNull;
    }
    return rects[i];
}

// belluiCaretRectFromArray returns the zero-width caret rectangle at a UTF-16
// offset: the leading edge of the character there, or the trailing edge of the
// preceding one at a line/document end. CGRectNull when there is no geometry.
static CGRect belluiCaretRectFromArray(CGRect *rects, int base, int count, NSInteger off) {
    CGRect r = belluiCharRectFromArray(rects, base, count, off);
    if (CGRectIsNull(r) && off > 0) {
        CGRect prev = belluiCharRectFromArray(rects, base, count, off - 1);
        if (!CGRectIsNull(prev)) {
            return CGRectMake(CGRectGetMaxX(prev), prev.origin.y, 0, prev.size.height);
        }
        return CGRectNull;
    }
    if (CGRectIsNull(r)) {
        return CGRectNull;
    }
    return CGRectMake(r.origin.x, r.origin.y, 0, r.size.height);
}

#pragma mark - BelluiTextInputView (#1117, bare UIView<UITextInput>)

@implementation BelluiTextPosition
+ (instancetype)positionWithIndex:(NSUInteger)index {
    BelluiTextPosition *p = [[[self alloc] init] autorelease];
    p->_index = index;
    return p;
}
@end

@implementation BelluiTextRange
+ (instancetype)rangeWithNSRange:(NSRange)range {
    BelluiTextRange *r = [[[self alloc] init] autorelease];
    r->_range = range;
    return r;
}
- (UITextPosition *)start { return [BelluiTextPosition positionWithIndex:_range.location]; }
- (UITextPosition *)end { return [BelluiTextPosition positionWithIndex:_range.location + _range.length]; }
- (BOOL)isEmpty { return _range.length == 0; }
- (id)copyWithZone:(NSZone *)zone {
    BelluiTextRange *r = [[BelluiTextRange allocWithZone:zone] init];
    r->_range = _range;
    return r;
}
@end

@implementation BelluiTextInputView

- (instancetype)initWithFrame:(CGRect)frame {
    self = [super initWithFrame:frame];
    if (self) {
        _text = [[NSMutableString alloc] init];
        _selectedRange = NSMakeRange(0, 0);
        _markedRange = NSMakeRange(NSNotFound, 0);
    }
    return self;
}

- (void)dealloc {
    free(belluiLayoutRects);
    free(belluiLineBounds);
    [_text release];
    [_tokenizer release];
    [_markedTextStyle release];
    [_belluiUndoManager removeAllActionsWithTarget:self];
    [_belluiUndoManager release];
    [super dealloc];
}

- (BOOL)canBecomeFirstResponder {
    return YES;
}

// A view-based UITextInput is its own text-input view — the backing view whose
// coordinate space the UITextInput geometry is expressed in (pre-rebrand tracker #1124).
- (UIView *)textInputView {
    return self;
}

#pragma mark undo/redo proxy manager (#1134, Flutter FlutterUndoManagerPlugin model)

// The responder's undo manager drives the iPad keyboard's undo/redo bar keys,
// the three-finger undo gestures and shake-to-undo. Return a dedicated manager
// (not the UIResponder-chain/window one) holding only bellui's proxy actions.
- (NSUndoManager *)undoManager {
    if (_belluiUndoManager == nil) {
        _belluiUndoManager = [[NSUndoManager alloc] init];
    }
    return _belluiUndoManager;
}

// belluiRegisterUndoProxy registers one proxy action in the given direction
// (NO = undo, YES = redo). Invoking it forwards to bellui — which owns the real
// history — and re-registers a proxy in the OPPOSITE direction, keeping the
// manager's canUndo/canRedo mirroring bellui's stacks between state pushes
// (Flutter's registerUndoWithDirection:).
- (void)belluiRegisterUndoProxy:(BOOL)redoDirection {
    NSUndoManager *um = self.undoManager;
    [um beginUndoGrouping];
    [um registerUndoWithTarget:self
                       handler:^(BelluiTextInputView *target) {
                           [target belluiRegisterUndoProxy:!redoDirection];
                           keyboardUndoRedo(redoDirection ? 1 : 0);
                       }];
    [um endUndoGrouping];
}

// belluiRegisterRedoProxy seeds redo availability: NSUndoManager has no direct
// "register a redo action", so register an undo action and immediately undo it
// — the action re-registers itself on the redo stack (Flutter's registerRedo).
- (void)belluiRegisterRedoProxy {
    NSUndoManager *um = self.undoManager;
    [um beginUndoGrouping];
    [um registerUndoWithTarget:self
                       handler:^(BelluiTextInputView *target) {
                           [target belluiRegisterUndoProxy:YES];
                       }];
    [um endUndoGrouping];
    [um undo];
}

- (void)belluiSetUndoState:(BOOL)canUndo canRedo:(BOOL)canRedo {
    NSUndoManager *um = self.undoManager;
    BOOL groupsByEvent = um.groupsByEvent;
    um.groupsByEvent = NO;
    [um removeAllActionsWithTarget:self];
    if (canUndo) {
        [self belluiRegisterUndoProxy:NO];
    }
    if (canRedo) {
        [self belluiRegisterRedoProxy];
    }
    // Poke the input assistant so the iPadOS keyboard refreshes its undo/redo
    // bar buttons immediately; without it the NSUndoManager change only shows
    // on the next keystroke (Flutter's workaround, kept verbatim).
    UITextInputAssistantItem *assistantItem = self.inputAssistantItem;
    assistantItem.leadingBarButtonGroups = assistantItem.leadingBarButtonGroups;
    um.groupsByEvent = groupsByEvent;
}

// This view IS the VoiceOver-focused editing element (pre-rebrand tracker #1124): in every working
// VoiceOver text field the focused accessibility element and the first responder
// are the SAME object — that is what makes VoiceOver drive the insertion point on
// it (a detached forwarding element engages reading/cut but never caret
// navigation; device-verified). A plain UIView is not an accessibility element by
// default, so opt in (a UITextView already is) — but ONLY while actually standing
// in for an edited entry: parked (zero-frame, no editing), the view must stay
// invisible to VoiceOver instead of floating as a stray text-input element (pre-rebrand tracker #951).
- (BOOL)isAccessibilityElement {
    return a11yActive;
}

// pre-rebrand tracker #570 keyboard-raise handshake: when VoiceOver moves focus onto this element the
// bridge arms belluiKeyboardRaisePending; re-claim first responder so iOS raises the
// software keyboard (it only does so when VO focus sits on the real first responder).
- (void)accessibilityElementDidBecomeFocused {
    if (!belluiKeyboardRaisePending) {
        return;
    }
    belluiKeyboardRaisePending = NO;
    if (keyboardHeight > 0) {
        return;
    }
    if ([self isFirstResponder]) {
        [self resignFirstResponder];
    }
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.6 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
        [self becomeFirstResponder];
    });
}

// bellui draws its own caret, selection highlight and context menu; this proxy is
// an invisible off-screen IME sink. It must NOT present the native selection UI
// / edit menu (the Cut/Copy/Paste callout), which is a duplicate of bellui's own
// menu and, for a bare UITextInput first responder with a selection, pops up
// over the entry (pre-rebrand tracker #1126). Suppress every edit action (classic UIMenuController
// path) and the modern edit menu (iOS 16+).
- (BOOL)canPerformAction:(SEL)action withSender:(id)sender {
    return NO;
}

- (UIMenu *)editMenuForTextRange:(UITextRange *)textRange
                suggestedActions:(NSArray<UIMenuElement *> *)suggestedActions API_AVAILABLE(ios(16.0)) {
    return nil;
}

#pragma mark index/range helpers

- (NSUInteger)clampIndex:(NSInteger)i {
    if (i < 0) {
        return 0;
    }
    if (i > (NSInteger)_text.length) {
        return _text.length;
    }
    return (NSUInteger)i;
}

- (NSRange)clampRange:(NSRange)r {
    NSUInteger len = _text.length;
    if (r.location > len) {
        r.location = len;
    }
    if (r.location + r.length > len) {
        r.length = len - r.location;
    }
    return r;
}

#pragma mark UIKeyInput

- (BOOL)hasText {
    return _text.length > 0;
}

// insertText: is the source interception point for plain typing and IME
// candidate commits (pre-rebrand tracker #1117) — no textViewDidChange: diff. Apply to the mirror
// optimistically, then forward at source:
//   - committing an active preedit -> keyboardComposeCommit;
//   - a caret insert -> keyboardTyped (per-rune, so NumericEntry's filter runs);
//   - typing over a selection -> keyboardReplace (pre-rebrand tracker #748).
- (void)insertText:(NSString *)text {
    if (text == nil) {
        text = @"";
    }
    // The Done return key dismisses rather than inserting a newline (a Done key
    // means a single-line / number field).
    if ([text isEqualToString:@"\n"] && self.returnKeyType == UIReturnKeyDone) {
        dispatch_async(dispatch_get_main_queue(), ^{
            [self resignFirstResponder];
        });
        return;
    }
    BOOL wasComposing = _markedRange.location != NSNotFound;
    NSRange rr = wasComposing ? _markedRange : _selectedRange;
    // Rune offsets must be read from the pre-edit mirror.
    int rs = belluiRuneOffset(_text, rr.location);
    int re = belluiRuneOffset(_text, rr.location + rr.length);

    [_text replaceCharactersInRange:rr withString:text];
    _selectedRange = NSMakeRange(rr.location + text.length, 0);
    _markedRange = NSMakeRange(NSNotFound, 0);

    if (wasComposing) {
        keyboardComposeCommit((char *)[text UTF8String]);
    } else if (rr.length == 0) {
        keyboardTyped((char *)[text UTF8String]);
    } else {
        keyboardReplace(rs, re, (char *)[text UTF8String]);
    }
}

- (void)deleteBackward {
    if (_selectedRange.length > 0) {
        NSRange rr = _selectedRange;
        int rs = belluiRuneOffset(_text, rr.location);
        int re = belluiRuneOffset(_text, rr.location + rr.length);
        [_text replaceCharactersInRange:rr withString:@""];
        _selectedRange = NSMakeRange(rr.location, 0);
        _markedRange = NSMakeRange(NSNotFound, 0);
        keyboardReplace(rs, re, (char *)"");
        return;
    }
    if (_selectedRange.location == 0) {
        return;
    }
    // Delete the whole composed-character sequence before the caret (an emoji
    // ZWJ run is one grapheme to iOS). A single-rune grapheme maps to bellui's
    // backspace key path (preserving its word-merge undo); a multi-rune one to
    // an explicit range delete since one backspace removes only one rune.
    NSRange del = [_text rangeOfComposedCharacterSequenceAtIndex:_selectedRange.location - 1];
    int rs = belluiRuneOffset(_text, del.location);
    int re = belluiRuneOffset(_text, del.location + del.length);
    int delRunes = belluiRuneOffset([_text substringWithRange:del], del.length);
    [_text replaceCharactersInRange:del withString:@""];
    _selectedRange = NSMakeRange(del.location, 0);
    if (delRunes == 1) {
        keyboardDelete();
    } else {
        keyboardReplace(rs, re, (char *)"");
    }
}

#pragma mark UITextInput — text + selection

- (NSString *)textInRange:(UITextRange *)range {
    NSRange r = [self clampRange:((BelluiTextRange *)range).range];
    NSString *s = [_text substringWithRange:r];
    return s;
}

- (void)replaceRange:(UITextRange *)range withText:(NSString *)text {
    if (text == nil) {
        text = @"";
    }
    NSRange rr = [self clampRange:((BelluiTextRange *)range).range];
    int rs = belluiRuneOffset(_text, rr.location);
    int re = belluiRuneOffset(_text, rr.location + rr.length);
    [_text replaceCharactersInRange:rr withString:text];
    _selectedRange = NSMakeRange(rr.location + text.length, 0);
    _markedRange = NSMakeRange(NSNotFound, 0);
    keyboardReplace(rs, re, (char *)[text UTF8String]);
}

- (UITextRange *)selectedTextRange {
    return [BelluiTextRange rangeWithNSRange:_selectedRange];
}

// setSelectedTextRange: fires for a native caret/selection move (a VoiceOver
// pass-through tap, a selection-grip drag, the text rotor). Mirror it to bellui
// as rune offsets — unless it is a Go-pushed echo or the IME owns the selection
// mid-preedit.
- (void)setSelectedTextRange:(UITextRange *)selectedTextRange {
    NSRange nr = selectedTextRange ? ((BelluiTextRange *)selectedTextRange).range
                                   : NSMakeRange(_text.length, 0);
    _selectedRange = [self clampRange:nr];
    if (syncingFromGo) {
        return;
    }
    if (_markedRange.location != NSNotFound) {
        // Mid-composition. A move that stays inside the marked region is the
        // IME's own (candidate cycling): keep it local, as before. A move
        // OUTSIDE it is an external caret placement — with VoiceOver on, a tap
        // arrives here (VoiceOver drives the native view), never through bellui's
        // touch pipeline — and silently dropping it strands the composition
        // (pre-rebrand tracker #1118). Finalize DETERMINISTICALLY ourselves: commit the marked text
        // as typed (an as-typed commit leaves _text unchanged, so the target
        // offsets stay valid), move bellui's caret to the target, then tell the
        // keyboard its composition is gone (textDidChange resets it; a late
        // keyboard unmarkText no-ops on the cleared marked range). Merely
        // *notifying* and letting the keyboard finalize is IME-dependent:
        // Apple's Pinyin commits and then restores ITS OWN caret after the
        // committed text, yanking the user's placement back (pre-rebrand tracker #1118 device
        // finding — typed letters landed at the old position).
        BOOL inside = _selectedRange.location >= _markedRange.location &&
            NSMaxRange(_selectedRange) <= NSMaxRange(_markedRange);
        if (!inside) {
            NSString *committed = [_text substringWithRange:_markedRange];
            _markedRange = NSMakeRange(NSNotFound, 0);
            keyboardComposeCommit((char *)[committed UTF8String]);
            keyboardSetSelection((int)belluiRuneOffset(_text, _selectedRange.location),
                                 (int)belluiRuneOffset(_text, NSMaxRange(_selectedRange)));
            [self.inputDelegate textWillChange:self];
            [self.inputDelegate textDidChange:self];
        }
        return;
    }
    // A genuine native caret move (not a Go echo): hold this range authoritatively
    // until bellui echoes it back, so a lagging per-repaint push cannot clobber it
    // (pre-rebrand tracker #1124). See the _awaitingSel ivar comment.
    _awaitingSel = _selectedRange;
    _awaitingConfirm = YES;
    _awaitingPushes = 0;
    keyboardSetSelection((int)belluiRuneOffset(_text, _selectedRange.location),
                         (int)belluiRuneOffset(_text, _selectedRange.location + _selectedRange.length));
}

#pragma mark UITextInput — marked (IME preedit) text

- (UITextRange *)markedTextRange {
    if (_markedRange.location == NSNotFound) {
        return nil;
    }
    return [BelluiTextRange rangeWithNSRange:_markedRange];
}

// setMarkedText:selectedRange: shows the IME preedit inline (pre-rebrand tracker #1117). Splice the
// marked text into the mirror over the previous marked region (or the
// selection), and report it as composition — display-only in bellui until commit
// (pre-rebrand tracker #747). selectedRange is relative to the
// marked text; its end is the caret within the preedit.
- (void)setMarkedText:(NSString *)markedText selectedRange:(NSRange)selectedRange {
    if (markedText == nil) {
        markedText = @"";
    }
    NSRange replace = _markedRange.location != NSNotFound ? _markedRange : _selectedRange;
    [_text replaceCharactersInRange:replace withString:markedText];
    _markedRange = NSMakeRange(replace.location, markedText.length);
    _selectedRange = [self clampRange:NSMakeRange(replace.location + selectedRange.location,
                                                  selectedRange.length)];
    NSUInteger cur = selectedRange.location + selectedRange.length;
    if (cur > markedText.length) {
        cur = markedText.length;
    }
    keyboardComposing((char *)[markedText UTF8String], (int)belluiRuneOffset(markedText, cur));
}

- (void)unmarkText {
    if (_markedRange.location == NSNotFound) {
        return;
    }
    NSString *committed = [_text substringWithRange:_markedRange];
    // Keep a selection already moved OUTSIDE the marked region (the external
    // caret placement that triggered this finalize, pre-rebrand tracker #1118) — an as-typed commit
    // leaves the text unchanged, so its offsets stay valid, and bellui preserves
    // the same caret across its commit. The normal in-composition case keeps
    // the caret-after-committed convention.
    BOOL inside = _selectedRange.location >= _markedRange.location &&
        NSMaxRange(_selectedRange) <= NSMaxRange(_markedRange);
    if (inside) {
        _selectedRange = NSMakeRange(_markedRange.location + _markedRange.length, 0);
    }
    _markedRange = NSMakeRange(NSNotFound, 0);
    keyboardComposeCommit((char *)[committed UTF8String]);
}

#pragma mark UITextInput — positions and ranges

- (UITextPosition *)beginningOfDocument {
    return [BelluiTextPosition positionWithIndex:0];
}

- (UITextPosition *)endOfDocument {
    return [BelluiTextPosition positionWithIndex:_text.length];
}

- (UITextRange *)textRangeFromPosition:(UITextPosition *)fromPosition toPosition:(UITextPosition *)toPosition {
    NSUInteger a = ((BelluiTextPosition *)fromPosition).index;
    NSUInteger b = ((BelluiTextPosition *)toPosition).index;
    NSUInteger lo = MIN(a, b), hi = MAX(a, b);
    return [BelluiTextRange rangeWithNSRange:NSMakeRange(lo, hi - lo)];
}

- (UITextPosition *)positionFromPosition:(UITextPosition *)position offset:(NSInteger)offset {
    NSInteger idx = (NSInteger)((BelluiTextPosition *)position).index + offset;
    if (idx < 0 || idx > (NSInteger)_text.length) {
        return nil;
    }
    return [BelluiTextPosition positionWithIndex:(NSUInteger)idx];
}

- (UITextPosition *)positionFromPosition:(UITextPosition *)position
                             inDirection:(UITextLayoutDirection)direction
                                  offset:(NSInteger)offset {
    NSInteger idx = (NSInteger)((BelluiTextPosition *)position).index;
    // Vertical (Up/Down) movement is line navigation — VoiceOver's line-swipe
    // drives the insertion point through this (pre-rebrand tracker #1124). Move `offset` visual lines
    // in the direction, keeping the column (clamped to the target line), and return
    // nil only at the document edge. Returning a DIFFERENT position each step (nil
    // only at the limit) is what avoids the pre-rebrand tracker #1126 spin — that loop only ran forever
    // because the old code returned the *same* input position; a real adjacent-line
    // position both navigates AND terminates the walk.
    if (direction == UITextLayoutDirectionUp || direction == UITextLayoutDirectionDown) {
        if (offset <= 0) {
            return position; // no vertical movement requested
        }
        BOOL down = (direction == UITextLayoutDirectionDown);
        NSRange line = [self belluiLineRangeForOffset:(NSUInteger)idx];
        NSUInteger column = (NSUInteger)idx >= line.location ? (NSUInteger)idx - line.location : 0;
        NSRange cur = line;
        for (NSInteger s = 0; s < offset; s++) {
            if (down) {
                NSUInteger afterCur = cur.location + cur.length;
                if (afterCur >= _text.length) {
                    return nil; // last line — no line below
                }
                NSUInteger nextStart = afterCur;
                if ([_text characterAtIndex:nextStart] == '\n') {
                    nextStart++;
                }
                if (nextStart > _text.length) {
                    return nil;
                }
                cur = [self belluiLineRangeForOffset:nextStart];
            } else {
                if (cur.location == 0) {
                    return nil; // first line — no line above
                }
                cur = [self belluiLineRangeForOffset:cur.location - 1];
            }
        }
        NSUInteger target = cur.location + MIN(column, cur.length);
        return [BelluiTextPosition positionWithIndex:target];
    }

    NSInteger result;
    switch (direction) {
        case UITextLayoutDirectionRight: result = idx + offset; break;
        case UITextLayoutDirectionLeft: result = idx - offset; break;
        default:
            return nil;
    }
    if (result < 0) {
        result = 0;
    }
    if (result > (NSInteger)_text.length) {
        result = _text.length;
    }
    if (result == idx) {
        return nil; // at a document limit: no further position in this direction
    }
    return [BelluiTextPosition positionWithIndex:(NSUInteger)result];
}

- (NSComparisonResult)comparePosition:(UITextPosition *)position toPosition:(UITextPosition *)other {
    NSUInteger a = ((BelluiTextPosition *)position).index;
    NSUInteger b = ((BelluiTextPosition *)other).index;
    if (a < b) {
        return NSOrderedAscending;
    }
    if (a > b) {
        return NSOrderedDescending;
    }
    return NSOrderedSame;
}

- (NSInteger)offsetFromPosition:(UITextPosition *)from toPosition:(UITextPosition *)toPosition {
    return (NSInteger)((BelluiTextPosition *)toPosition).index - (NSInteger)((BelluiTextPosition *)from).index;
}

- (id<UITextInputTokenizer>)tokenizer {
    if (_tokenizer == nil) {
        // The BelluiTokenizer answers Line granularity from bellui's visual lines
        // (pre-rebrand tracker #1113); all other granularities defer to UITextInputStringTokenizer.
        _tokenizer = [[BelluiTokenizer alloc] initWithTextInput:self];
    }
    return _tokenizer;
}

- (NSRange)belluiLineRangeForOffset:(NSUInteger)off {
    NSUInteger len = _text.length;
    if (off > len) {
        off = len;
    }
    // Sidecar visual lines (matches the pre-rebrand tracker #1107 ring, handles soft-wrap): line k
    // spans [b_k, b_{k+1}). A caret at an interior boundary belongs to the line
    // it starts; at the final boundary it belongs to the last line.
    if (belluiLineCount >= 2) {
        NSUInteger first = (NSUInteger)belluiLineBounds[0];
        NSUInteger last = (NSUInteger)belluiLineBounds[belluiLineCount - 1];
        if (off >= first && off <= last) {
            for (int k = 0; k + 1 < belluiLineCount; k++) {
                NSUInteger s = (NSUInteger)belluiLineBounds[k];
                NSUInteger e = (NSUInteger)belluiLineBounds[k + 1];
                if (off >= s && (off < e || k + 2 == belluiLineCount)) {
                    // Clamp against a momentary sidecar/mirror frame skew.
                    if (s > len) {
                        s = len;
                    }
                    if (e > len) {
                        e = len;
                    }
                    // Trim a trailing '\n' so the line unit excludes the
                    // separator (the Flutter convention; VoiceOver otherwise
                    // reads "new line" at each line end).
                    if (e > s && [_text characterAtIndex:e - 1] == '\n') {
                        e--;
                    }
                    return NSMakeRange(s, e >= s ? e - s : 0);
                }
            }
        }
    }
    // Fallback: logical ('\n'-delimited) lines over the whole mirror — covers a
    // field with no sidecar (single-line, no AccessibleTextLayouter) or an
    // offset outside the sidecar window. This is Flutter's FlutterTokenizer path.
    NSUInteger start = 0, end = len;
    if (off > 0) {
        NSRange before = [_text rangeOfString:@"\n" options:NSBackwardsSearch range:NSMakeRange(0, off)];
        if (before.location != NSNotFound) {
            start = before.location + 1;
        }
    }
    NSRange after = [_text rangeOfString:@"\n" options:0 range:NSMakeRange(off, len - off)];
    if (after.location != NSNotFound) {
        end = after.location; // exclude the newline itself
    }
    return NSMakeRange(start, end - start);
}

- (UITextPosition *)positionWithinRange:(UITextRange *)range farthestInDirection:(UITextLayoutDirection)direction {
    NSRange r = ((BelluiTextRange *)range).range;
    NSUInteger idx = (direction == UITextLayoutDirectionRight || direction == UITextLayoutDirectionDown)
        ? r.location + r.length : r.location;
    return [BelluiTextPosition positionWithIndex:idx];
}

- (UITextRange *)characterRangeByExtendingPosition:(UITextPosition *)position inDirection:(UITextLayoutDirection)direction {
    NSUInteger idx = ((BelluiTextPosition *)position).index;
    NSRange r = (direction == UITextLayoutDirectionRight || direction == UITextLayoutDirectionDown)
        ? NSMakeRange(idx, _text.length - idx)
        : NSMakeRange(0, idx);
    return [BelluiTextRange rangeWithNSRange:r];
}

- (NSWritingDirection)baseWritingDirectionForPosition:(UITextPosition *)position inDirection:(UITextStorageDirection)direction {
    return NSWritingDirectionLeftToRight; // bellui owns bidi ordering
}

- (void)setBaseWritingDirection:(NSWritingDirection)writingDirection forRange:(UITextRange *)range {
    // no-op: bellui owns bidi ordering
}

#pragma mark UITextInput — geometry (fed from the #1107 sidecar)

- (CGRect)firstRectForRange:(UITextRange *)range {
    if (belluiLayoutCount == 0) {
        return CGRectZero;
    }
    NSRange r = ((BelluiTextRange *)range).range;
    if (r.length == 0) {
        CGRect c = belluiCaretRectFromArray(belluiLayoutRects, belluiLayoutBase, belluiLayoutCount, r.location);
        return CGRectIsNull(c) ? CGRectZero : c;
    }
    CGRect line = CGRectNull;
    for (NSUInteger i = r.location; i < r.location + r.length; i++) {
        CGRect cr = belluiCharRectFromArray(belluiLayoutRects, belluiLayoutBase, belluiLayoutCount, i);
        if (CGRectIsNull(cr)) {
            continue;
        }
        if (CGRectIsNull(line)) {
            line = cr;
        } else if (fabs(cr.origin.y - line.origin.y) < 0.5) {
            line = CGRectUnion(line, cr);
        } else {
            break; // the first visual line only
        }
    }
    return CGRectIsNull(line) ? CGRectZero : line;
}

- (CGRect)caretRectForPosition:(UITextPosition *)position {
    if (belluiLayoutCount == 0) {
        return CGRectZero;
    }
    NSUInteger off = ((BelluiTextPosition *)position).index;
    CGRect c = belluiCaretRectFromArray(belluiLayoutRects, belluiLayoutBase, belluiLayoutCount, off);
    if (CGRectIsNull(c)) {
        return CGRectZero;
    }
    c.size.width = 2; // a nominal caret width, like UIKit
    return c;
}

// selectionRectsForRange: is what VoiceOver queries to draw the editing/line
// ring (pre-rebrand tracker #1107). One sidecar-derived rectangle per visual line the range covers.
- (NSArray<UITextSelectionRect *> *)selectionRectsForRange:(UITextRange *)range {
    NSMutableArray<UITextSelectionRect *> *out = [NSMutableArray array];
    if (belluiLayoutCount == 0) {
        return out;
    }
    NSRange r = ((BelluiTextRange *)range).range;
    if (r.length == 0) {
        CGRect c = belluiCaretRectFromArray(belluiLayoutRects, belluiLayoutBase, belluiLayoutCount, r.location);
        if (!CGRectIsNull(c)) {
            [out addObject:[BelluiTextSelectionRect withRect:c]];
        }
        return out;
    }
    CGRect cur = CGRectNull;
    for (NSUInteger i = r.location; i < r.location + r.length; i++) {
        CGRect cr = belluiCharRectFromArray(belluiLayoutRects, belluiLayoutBase, belluiLayoutCount, i);
        if (CGRectIsNull(cr)) {
            continue;
        }
        if (CGRectIsNull(cur)) {
            cur = cr;
        } else if (fabs(cr.origin.y - cur.origin.y) < 0.5) {
            cur = CGRectUnion(cur, cr); // same visual line
        } else {
            [out addObject:[BelluiTextSelectionRect withRect:cur]];
            cur = cr;
        }
    }
    if (!CGRectIsNull(cur)) {
        [out addObject:[BelluiTextSelectionRect withRect:cur]];
    }
    if (out.count > 0) {
        CGRect f = [(UITextSelectionRect *)out[0] rect];
    }
    return out;
}

- (UITextPosition *)closestPositionToPoint:(CGPoint)point {
    if (belluiLayoutCount == 0) {
        return [BelluiTextPosition positionWithIndex:_selectedRange.location];
    }
    NSInteger best = -1;
    CGFloat bestDist = CGFLOAT_MAX;
    for (int i = 0; i < belluiLayoutCount; i++) {
        CGRect cr = belluiLayoutRects[i];
        if (CGRectIsNull(cr)) {
            continue;
        }
        CGFloat dx = CGRectGetMidX(cr) - point.x;
        CGFloat dy = CGRectGetMidY(cr) - point.y;
        CGFloat d = dx * dx + dy * dy;
        if (d < bestDist) {
            bestDist = d;
            best = belluiLayoutBase + i;
        }
    }
    if (best < 0) {
        return [BelluiTextPosition positionWithIndex:_selectedRange.location];
    }
    return [BelluiTextPosition positionWithIndex:[self clampIndex:best]];
}

- (UITextPosition *)closestPositionToPoint:(CGPoint)point withinRange:(UITextRange *)range {
    return [self closestPositionToPoint:point];
}

- (UITextRange *)characterRangeAtPoint:(CGPoint)point {
    UITextPosition *p = [self closestPositionToPoint:point];
    return [BelluiTextRange rangeWithNSRange:NSMakeRange(((BelluiTextPosition *)p).index, 0)];
}

#pragma mark Go bridge (BelluiTextInputProxy)

// belluiSetDocument resynchronises the mirror from Go (pre-rebrand tracker #771), pushed each repaint
// by syncKeyboardState. It updates the ivars *without* notifying the input
// delegate: on a first-responder view, textDidChange:/selectionDidChange: make
// UIKit re-evaluate the keyboard/accessory, which changes the viewport, which
// dirties the canvas, which drives the next repaint's syncKeyboardState back
// into here — a self-sustaining relayout storm (pre-rebrand tracker #1126). UITextInput reads the
// fresh ivars on demand, so no proactive notification is needed. While composing
// the IME owns the field (matching Android) and the push is dropped — except the
// external-caret-move finalize below.
- (void)belluiSetDocument:(NSString *)text selStart:(int)selStart selEnd:(int)selEnd {
    if (_markedRange.location != NSNotFound) {
        // While composing the IME owns the mirror, so the push is not applied —
        // BUT it still carries one signal worth reading: bellui pushes committed-only
        // text (the seam subtracts the preedit, pre-rebrand tracker #1057) with the selection collapsed
        // to the composition anchor, so a pushed caret that differs from the anchor
        // means the user moved the caret externally mid-composition (a tap in the
        // entry, with VoiceOver off — the touch went through bellui's pipeline).
        // Silently ignoring that desynchronises the keyboard — it keeps composing
        // at the old spot while bellui's caret is elsewhere, leaving the preedit
        // stranded (pre-rebrand tracker #1118 CJK gate). Finalize deterministically ourselves, the
        // same self-serve as setSelectedTextRange:'s external-move branch: commit
        // the marked text as typed (bellui's ComposeCommit preserves the externally
        // placed caret), update the local selection into post-commit space, and
        // tell the keyboard its composition is gone. One-shot by construction —
        // the cleared marked range ends this branch — so the per-repaint push
        // loop cannot storm the delegate (pre-rebrand tracker #1126).
        NSString *cwant = text != nil ? text : @"";
        NSUInteger cus = (NSUInteger)belluiUTF16Offset(cwant, selStart);
        NSUInteger cue = (NSUInteger)belluiUTF16Offset(cwant, selEnd);
        if (!(cus == _markedRange.location && cue == _markedRange.location)) {
            NSString *committed = [_text substringWithRange:_markedRange];
            // Map the pushed committed-space selection into post-commit space
            // (the as-typed commit inserts the marked text back at the anchor).
            NSUInteger n = _markedRange.length;
            NSUInteger anchor = _markedRange.location;
            _selectedRange = [self clampRange:NSMakeRange(cus > anchor ? cus + n : cus,
                                                          (cue > anchor ? cue + n : cue) -
                                                          (cus > anchor ? cus + n : cus))];
            _markedRange = NSMakeRange(NSNotFound, 0);
            keyboardComposeCommit((char *)[committed UTF8String]);
            [self.inputDelegate textWillChange:self];
            [self.inputDelegate textDidChange:self];
        }
        return;
    }
    NSString *want = text != nil ? text : @"";
    NSUInteger us = (NSUInteger)belluiUTF16Offset(want, selStart);
    NSUInteger ue = (NSUInteger)belluiUTF16Offset(want, selEnd);
    if (us > want.length) {
        us = want.length;
    }
    if (ue > want.length) {
        ue = want.length;
    }
    NSRange newSel = NSMakeRange(us, ue >= us ? ue - us : 0);

    // Confirmation gate (pre-rebrand tracker #1124): while awaiting bellui's echo of a native caret move,
    // accept the selection only once it matches the awaited range (bellui caught up)
    // — then clear the wait and resume normal sync. A differing selection is a
    // stale push from before bellui applied the move; reject it (hold _selectedRange)
    // so it can't collapse VoiceOver's caret. _awaitingPushes bounds the wait in
    // case bellui never echoes an exact match (e.g. a clamped range). Text always
    // syncs; a non-awaiting push accepts the selection as before.
    BOOL applySelection = YES;
    if (_awaitingConfirm) {
        if (NSEqualRanges(newSel, _awaitingSel)) {
            _awaitingConfirm = NO; // bellui echoed the awaited move — caught up
        } else if (++_awaitingPushes > 30) {
            _awaitingConfirm = NO; // safety valve: never wedge the sync
        } else {
            applySelection = NO; // stale push — hold the authoritative selection
        }
    }

    if ([_text isEqualToString:want] &&
        (!applySelection || NSEqualRanges(newSel, _selectedRange))) {
        return;
    }

    syncingFromGo = YES;
    [_text setString:want];
    if (applySelection) {
        _selectedRange = newSel;
    }
    syncingFromGo = NO;
}

- (void)belluiSetTextLayoutWire:(const char *)wire originX:(double)originX originY:(double)originY scale:(double)scale {
    free(belluiLayoutRects);
    belluiParseLayoutWire(wire, originX, originY, scale,
                        &belluiLayoutRects, &belluiLayoutBase, &belluiLayoutCount);
    free(belluiLineBounds);
    belluiParseLineWire(wire, &belluiLineBounds, &belluiLineCount);
}

@end

#pragma mark - BelluiTokenizer (#1113 Line granularity)

@implementation BelluiTokenizer {
    // The text input, kept as a BelluiTextInputView to reach its line model. Not
    // retained — the view owns (retains) its tokenizer, so this back-reference
    // must be weak to avoid a cycle; it outlives the tokenizer.
    __unsafe_unretained BelluiTextInputView *_belluiView;
}

- (instancetype)initWithTextInput:(UIResponder<UITextInput> *)textInput {
    self = [super initWithTextInput:textInput];
    if (self && [textInput isKindOfClass:[BelluiTextInputView class]]) {
        _belluiView = (BelluiTextInputView *)textInput;
    }
    return self;
}

// belluiIsForward reports whether a UITextDirection advances toward the document
// end — both the storage (Forward) and layout (Right/Down) senses VoiceOver
// mixes for line movement.
static BOOL belluiIsForward(UITextDirection direction) {
    return direction == UITextStorageDirectionForward ||
           direction == UITextLayoutDirectionRight ||
           direction == UITextLayoutDirectionDown;
}

- (UITextRange *)rangeEnclosingPosition:(UITextPosition *)position
                        withGranularity:(UITextGranularity)granularity
                            inDirection:(UITextDirection)direction {
    if (granularity == UITextGranularityLine && _belluiView != nil) {
        NSRange line = [_belluiView belluiLineRangeForOffset:((BelluiTextPosition *)position).index];
        return [BelluiTextRange rangeWithNSRange:line];
    }
    return [super rangeEnclosingPosition:position withGranularity:granularity inDirection:direction];
}

- (UITextPosition *)positionFromPosition:(UITextPosition *)position
                              toBoundary:(UITextGranularity)granularity
                             inDirection:(UITextDirection)direction {
    if (granularity == UITextGranularityLine && _belluiView != nil) {
        NSRange line = [_belluiView belluiLineRangeForOffset:((BelluiTextPosition *)position).index];
        NSUInteger target = belluiIsForward(direction) ? line.location + line.length : line.location;
        return [BelluiTextPosition positionWithIndex:target];
    }
    return [super positionFromPosition:position toBoundary:granularity inDirection:direction];
}

- (BOOL)isPosition:(UITextPosition *)position
        atBoundary:(UITextGranularity)granularity
       inDirection:(UITextDirection)direction {
    if (granularity == UITextGranularityLine && _belluiView != nil) {
        NSUInteger off = ((BelluiTextPosition *)position).index;
        NSRange line = [_belluiView belluiLineRangeForOffset:off];
        BOOL r = belluiIsForward(direction) ? off == line.location + line.length : off == line.location;
        return r;
    }
    return [super isPosition:position atBoundary:granularity inDirection:direction];
}

- (BOOL)isPosition:(UITextPosition *)position
    withinTextUnit:(UITextGranularity)granularity
       inDirection:(UITextDirection)direction {
    if (granularity == UITextGranularityLine && _belluiView != nil) {
        NSUInteger off = ((BelluiTextPosition *)position).index;
        NSRange line = [_belluiView belluiLineRangeForOffset:off];
        BOOL r = off >= line.location && off <= line.location + line.length;
        return r;
    }
    return [super isPosition:position withinTextUnit:granularity inDirection:direction];
}

@end

void runApp(void) {
	char * argv[] = {};
	@autoreleasepool {
		UIApplicationMain(0, argv, nil, NSStringFromClass([GoAppAppDelegate class]));
	}
}

void makeCurrentContext(GLintptr context) {
	EAGLContext* ctx = (EAGLContext*)context;
	if (![EAGLContext setCurrentContext:ctx]) {
		// TODO(crawshaw): determine how terrible this is. Exit?
		NSLog(@"failed to set current context");
	}
}

void swapBuffers(GLintptr context) {
	__block EAGLContext* ctx = (EAGLContext*)context;
	dispatch_sync(dispatch_get_main_queue(), ^{
		[EAGLContext setCurrentContext:ctx];
		[ctx presentRenderbuffer:GL_RENDERBUFFER];
	});
}

uint64_t threadID() {
	uint64_t id;
	if (pthread_threadid_np(pthread_self(), &id)) {
		abort();
	}
	return id;
}

UIEdgeInsets getDevicePadding() {
    if (@available(iOS 11.0, *)) {
        UIWindow *window = UIApplication.sharedApplication.keyWindow;

        UIEdgeInsets inset = window.safeAreaInsets;
        if (keyboardHeight != 0) {
            inset.bottom = keyboardHeight;
        }
        return inset;
    }

    return UIEdgeInsetsZero;
}

bool isDark() {
    UIViewController *rootVC = [[[[UIApplication sharedApplication] delegate] window] rootViewController];
    return rootVC.traitCollection.userInterfaceStyle == UIUserInterfaceStyleDark;
}

#define DEFAULT_KEYBOARD_CODE 0
#define SINGLELINE_KEYBOARD_CODE 1
#define NUMBER_KEYBOARD_CODE 2

void showKeyboard(int keyboardType) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView<BelluiTextInputProxy, UITextInputTraits> *view = appDelegate.controller.inputView;

    dispatch_async(dispatch_get_main_queue(), ^{
        switch (keyboardType)
        {
            case DEFAULT_KEYBOARD_CODE:
                [view setKeyboardType:UIKeyboardTypeDefault];
                [view setReturnKeyType:UIReturnKeyDefault];
                break;
            case SINGLELINE_KEYBOARD_CODE:
                [view setKeyboardType:UIKeyboardTypeDefault];
                [view setReturnKeyType:UIReturnKeyDone];
                break;
            case NUMBER_KEYBOARD_CODE:
                [view setKeyboardType:UIKeyboardTypeNumberPad];
                [view setReturnKeyType:UIReturnKeyDone];
                break;
            default:
                NSLog(@"unknown keyboard type, use default");
                [view setKeyboardType:UIKeyboardTypeDefault];
                [view setReturnKeyType:UIReturnKeyDefault];
                break;
        }
        // refresh settings if keyboard is already open
        [view reloadInputViews];

        // Give the proxy a non-zero frame while it is first responder. iOS
        // suppresses the IME suggestion / CJK candidate bar for a zero-size input
        // view, so the empty default frame (kept as the pre-rebrand tracker #570 VO "not overlaid"
        // sentinel) must be widened here (pre-rebrand tracker #771) — Flutter's tiny 1x1 frame
        // suffices (pre-rebrand tracker #1118 device-verified: the candidate bar appears). Under
        // VoiceOver the accessibility bridge overlays the view on the entry
        // instead; hideKeyboard restores the empty frame.
        if (CGRectIsEmpty(view.frame)) {
            view.frame = CGRectMake(0, 0, 1, 1);
        }

        BOOL ret = [view becomeFirstResponder];
        (void)ret;
    });
}

void hideKeyboard() {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView<BelluiTextInputProxy, UITextInputTraits> *view = appDelegate.controller.inputView;

    dispatch_async(dispatch_get_main_queue(), ^{
        belluiKeyboardRaisePending = NO;
        [view resignFirstResponder];
        // Restore the empty frame (the pre-rebrand tracker #570 VoiceOver "not overlaid" sentinel).
        // Under VoiceOver the accessibility bridge owns the frame and clears it
        // itself; guard so we do not fight an active overlay.
        if (!UIAccessibilityIsVoiceOverRunning()) {
            view.frame = CGRectZero;
        }
    });
}

// setKeyboardState mirrors the focused editor's real text + selection (rune
// offsets) from Go into the input view's document (pre-rebrand tracker #771 state mirror), so the
// IME composes against real content and the field's selection round-trips. It
// hops to the main thread and is a no-op while the field is composing (handled
// in belluiSetDocument).
void setKeyboardState(char *text, int selStart, int selEnd) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView<BelluiTextInputProxy, UITextInputTraits> *view = appDelegate.controller.inputView;
    NSString *s = text != NULL ? [NSString stringWithUTF8String:text] : @"";

    dispatch_async(dispatch_get_main_queue(), ^{
        [view belluiSetDocument:s selStart:selStart selEnd:selEnd];
    });
}

// setKeyboardUndoState mirrors bellui's undo/redo availability into the input
// view's proxy NSUndoManager (pre-rebrand tracker #1134), lighting up the iPad keyboard's undo/redo
// bar keys and the system undo gestures. Pushed from Go on change only, beside
// the per-repaint keyboard-state sync.
void setKeyboardUndoState(int canUndo, int canRedo) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView<BelluiTextInputProxy, UITextInputTraits> *view = appDelegate.controller.inputView;

    dispatch_async(dispatch_get_main_queue(), ^{
        if ([view isKindOfClass:[BelluiTextInputView class]]) {
            [(BelluiTextInputView *)view belluiSetUndoState:canUndo != 0 canRedo:canRedo != 0];
        }
    });
}

// belluiKeyboardInputView exposes the hidden text-input proxy to the
// accessibility bridge (accessibility_ios.m): under VoiceOver the bridge
// substitutes this real UITextView for the focused entry's synthetic
// element, because iOS only raises the software keyboard when VoiceOver
// focus sits on the actual first responder (pre-rebrand tracker #570).
UIView *belluiKeyboardInputView(void) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    return appDelegate.controller.inputView;
}

// belluiAppWindow exposes the application's UIWindow to the native-view
// embedding backends (webview/webview_ios.m, #64), which host their platform
// view as a sibling of the GLKView in this window.
//
// It exists because the iOS window's RunNative hands out only an
// UnknownContext — unlike macOS, which passes the NSWindow — so a backend has
// no other way to reach the hosting window. Reaching it through this accessor
// keeps that a detail of the driver rather than new public API in the driver
// package for one consumer.
UIWindow *belluiAppWindow(void) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    return appDelegate.window;
}

// belluiTextEditingA11yElement returns the accessibility element that stands in for
// the focused entry while VoiceOver edits it (pre-rebrand tracker #1124): the input view itself, which
// is BOTH the first responder and the VoiceOver-focused element — the same-object
// model every native text field uses, and what makes VoiceOver drive the
// insertion point (a detached forwarding element engages reading/cut but never
// caret navigation). Returns nil when there is no input view.
id belluiTextEditingA11yElement(void) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    return appDelegate.controller.inputView;
}

// belluiPrepareSwapSurrogateFrame overlays the input view on the edited entry
// (pre-rebrand tracker #1124): while it stands in for the entry's synthetic element it needs a real
// on-screen frame (UIKit deems a zero-frame view invisible and VoiceOver refuses
// to focus it) and a clear caret tint so it stays visually silent.
void belluiPrepareSwapSurrogateFrame(CGRect entryFrameInView) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView *input = appDelegate.controller.inputView;
    if (input == nil) {
        return;
    }
    input.frame = entryFrameInView;
    input.tintColor = [UIColor clearColor];
    if ([input isKindOfClass:[BelluiTextInputView class]]) {
        ((BelluiTextInputView *)input)->a11yActive = YES;
    }
}

// belluiParkSwapSurrogate is the counterpart of belluiPrepareSwapSurrogateFrame:
// editing ended, so the input view returns to its parked state — zero frame
// and invisible to VoiceOver (pre-rebrand tracker #951; before this, isAccessibilityElement was
// unconditionally YES, leaving a stray zero-frame accessible text-input
// element in every bellui window from launch).
void belluiParkSwapSurrogate(void) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView *input = appDelegate.controller.inputView;
    if (input == nil) {
        return;
    }
    input.frame = CGRectZero;
    if ([input isKindOfClass:[BelluiTextInputView class]]) {
        ((BelluiTextInputView *)input)->a11yActive = NO;
    }
}

// belluiInputViewSetTextLayout pushes bellui's visual-layout sidecar into the input
// proxy so it answers VoiceOver's per-line/caret geometry from bellui's real
// layout (pre-rebrand tracker #1107). Called from the accessibility bridge, already on the main
// thread (inside its main-queue block), so it forwards synchronously.
void belluiInputViewSetTextLayout(const char *wire, double originX, double originY, double scale) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];
    UIView<BelluiTextInputProxy, UITextInputTraits> *view = appDelegate.controller.inputView;
    if (view == nil) {
        return;
    }
    [view belluiSetTextLayoutWire:wire originX:originX originY:originY scale:scale];
}

// contentTypesForMimeExts maps the picker filter to UTTypes via the modern
// (iOS 14+) UniformTypeIdentifiers API. The previous
// UTTypeCreatePreferredIdentifierForTag + initWithDocumentTypes: path is
// deprecated and left matching files un-pickable on current iOS (#251).
NSMutableArray<UTType *> *contentTypesForMimeExts(char *mimes, char *exts) {
    NSMutableArray<UTType *> *types = [NSMutableArray array];
    if (mimes != NULL && strlen(mimes) > 0) {
        NSString *mimeList = [NSString stringWithUTF8String:mimes];

        if ([mimeList isEqualToString:@"application/x-directory"]) {
            [types addObject:UTTypeFolder];
        } else {
            for (NSString *mime in [mimeList componentsSeparatedByString:@"|"]) {
                UTType *type = [UTType typeWithMIMEType:mime];
                if (type != nil) {
                    [types addObject:type];
                }
            }
        }
    } else if (exts != NULL && strlen(exts) > 0) {
        NSString *extList = [NSString stringWithUTF8String:exts];
        for (NSString *ext in [extList componentsSeparatedByString:@"|"]) {
            UTType *type = [UTType typeWithFilenameExtension:ext];
            if (type != nil) {
                [types addObject:type];
            }
        }
    }
    if (types.count == 0) {
        [types addObject:UTTypeData];
    }

    return types;
}

void showFileOpenPicker(char* mimes, char *exts) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];

    NSArray<UTType *> *types = contentTypesForMimeExts(mimes, exts);

    UIDocumentPickerViewController *documentPicker = [[UIDocumentPickerViewController alloc]
        initForOpeningContentTypes:types asCopy:NO];
    documentPicker.delegate = (id) appDelegate;

    dispatch_async(dispatch_get_main_queue(), ^{
        [appDelegate.controller presentViewController:documentPicker animated:YES completion:nil];
        // No ARC here: presentation owns the picker now, balance the alloc.
        [documentPicker release];
    });
}

void showFileOpenMultiPicker(char* mimes, char *exts) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];

    NSArray<UTType *> *types = contentTypesForMimeExts(mimes, exts);

    UIDocumentPickerViewController *documentPicker = [[UIDocumentPickerViewController alloc]
        initForOpeningContentTypes:types asCopy:NO];
    // allowsMultipleSelection also routes the delegate callbacks to the
    // multi-session bridge (filePickerMultiAppend/Complete, #255).
    documentPicker.allowsMultipleSelection = YES;
    documentPicker.delegate = (id) appDelegate;

    dispatch_async(dispatch_get_main_queue(), ^{
        [appDelegate.controller presentViewController:documentPicker animated:YES completion:nil];
        // No ARC here: presentation owns the picker now, balance the alloc.
        [documentPicker release];
    });
}

void showFileSavePicker(char* mimes, char *exts, char *filename) {
    GoAppAppDelegate *appDelegate = (GoAppAppDelegate *)[[UIApplication sharedApplication] delegate];

    // The staged temporary file carries the name the picker offers, so it must
    // be the caller's SetFileName, not a hardcoded placeholder (#252). Only the
    // last path component is used — a separator in the name would stage the
    // file outside the temporary directory, or nowhere at all.
    NSString *name = nil;
    if (filename != NULL && strlen(filename) > 0) {
        name = [[NSString stringWithUTF8String:filename] lastPathComponent];
    }
    if (name == nil || name.length == 0) {
        name = @"Untitled";
    }

    NSURL *temporaryDirectoryURL = [NSURL fileURLWithPath: NSTemporaryDirectory() isDirectory: YES];
    NSURL *temporaryFileURL = [temporaryDirectoryURL URLByAppendingPathComponent:name];

    char* bytes = "\n";
    NSData *data = [NSData dataWithBytes:bytes length:1];
    if (![data writeToURL:temporaryFileURL atomically:YES]) {
        // Exporting a URL that failed to stage would present a picker for a
        // file that does not exist; a plain fallback name always stages.
        temporaryFileURL = [temporaryDirectoryURL URLByAppendingPathComponent:@"Untitled"];
        [data writeToURL:temporaryFileURL atomically:YES];
    }

    UIDocumentPickerViewController *documentPicker = [[UIDocumentPickerViewController alloc]
        initForExportingURLs:@[temporaryFileURL] asCopy:NO];
    documentPicker.delegate = (id) appDelegate;

    dispatch_async(dispatch_get_main_queue(), ^{
        [appDelegate.controller presentViewController:documentPicker animated:YES completion:nil];
        // No ARC here: presentation owns the picker now, balance the alloc.
        [documentPicker release];
    });
}

void closeFileResource(void* urlPtr) {
    if (urlPtr == NULL) {
        return;
    }

    NSURL* url = (NSURL*) urlPtr;
    [url stopAccessingSecurityScopedResource];
    CFRelease((CFTypeRef)urlPtr);
}
