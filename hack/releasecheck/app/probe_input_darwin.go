//go:build darwin && !ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

static NSWindow* probeWindow(void) {
	for (NSWindow* w in [NSApp windows]) {
		if ([[w title] isEqualToString:@"Release check"]) {
			return w;
		}
	}
	return nil;
}

// probeContentWidth is the window's content width in points, or -1.
static double probeContentWidth(void) {
	__block double width = -1;
	dispatch_sync(dispatch_get_main_queue(), ^{
		NSWindow* w = probeWindow();
		if (w != nil) {
			width = [[w contentView] bounds].size.width;
		}
	});
	return width;
}

// probeMouse sends the window a mouse event at (x, y) points from the
// content's top-left: 0 moved, 1 left down, 2 left dragged, 3 left up. It goes
// to the window's own sendEvent:, the path a real click takes after
// NSApplication, so GLFW and the driver handle it as they would a real one,
// and no Accessibility permission is needed. The driver reads a click's
// position from the system cursor (glfwGetCursorPos), not from the event, so
// the cursor is first warped to the point: the probe moves the real pointer.
static void probeMouse(int kind, double x, double y) {
	dispatch_sync(dispatch_get_main_queue(), ^{
		NSWindow* w = probeWindow();
		if (w == nil) {
			return;
		}
		NSView* v = [w contentView];
		NSPoint p = NSMakePoint(x, [v isFlipped] ? y : [v bounds].size.height - y);
		p = [v convertPoint:p toView:nil];
		NSPoint s = [w convertPointToScreen:p];
		CGFloat top = NSMaxY([[[NSScreen screens] objectAtIndex:0] frame]);
		CGWarpMouseCursorPosition(CGPointMake(s.x, top - s.y));
		NSEventType t = NSEventTypeMouseMoved;
		float pressure = 0;
		NSInteger clicks = 0;
		switch (kind) {
		case 1: t = NSEventTypeLeftMouseDown; pressure = 1; clicks = 1; break;
		case 2: t = NSEventTypeLeftMouseDragged; pressure = 1; break;
		case 3: t = NSEventTypeLeftMouseUp; clicks = 1; break;
		}
		NSEvent* e = [NSEvent mouseEventWithType:t location:p modifierFlags:0
			timestamp:[[NSProcessInfo processInfo] systemUptime] windowNumber:[w windowNumber]
			context:nil eventNumber:0 clickCount:clicks pressure:pressure];
		[w sendEvent:e];
	});
}

// probeKey sends the window a key down and up with the virtual key code and
// the characters it types.
static void probeKey(unsigned short code, const char* chars) {
	NSString* s = [NSString stringWithUTF8String:chars];
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow* w = probeWindow();
		if (w == nil) {
			return;
		}
		NSEventType types[2] = {NSEventTypeKeyDown, NSEventTypeKeyUp};
		for (int i = 0; i < 2; i++) {
			NSEvent* e = [NSEvent keyEventWithType:types[i] location:NSZeroPoint modifierFlags:0
				timestamp:[[NSProcessInfo processInfo] systemUptime] windowNumber:[w windowNumber]
				context:nil characters:s charactersIgnoringModifiers:s isARepeat:NO keyCode:code];
			[w sendEvent:e];
		}
	});
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// macInput posts input to the companion window through AppKit.
type macInput struct{}

func newProbeInput() (probeInput, error) { return macInput{}, nil }

func (macInput) contentWidth() (float32, error) {
	w := float64(C.probeContentWidth())
	if w <= 0 {
		return 0, errors.New("no window titled \"Release check\"")
	}
	return float32(w), nil
}

func (macInput) mouse(kind mouseKind, x, y float32) {
	C.probeMouse(C.int(kind), C.double(x), C.double(y))
}

func (macInput) key(k probeKey) {
	cs := C.CString(k.chars)
	defer C.free(unsafe.Pointer(cs))
	C.probeKey(C.ushort(k.code), cs)
}
