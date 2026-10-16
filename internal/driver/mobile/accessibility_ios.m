//go:build accessibility && ios

#import <UIKit/UIKit.h>

#include <string.h>

#include "accessibility_ios.h"

// Wire values from accessibility.go — keep in sync.
enum {
	BELLUI_ROLE_BUTTON     = 1,
	BELLUI_ROLE_TEXT       = 2,
	BELLUI_ROLE_LINK       = 3,
	BELLUI_ROLE_IMAGE      = 4,
	BELLUI_ROLE_CHECKBOX   = 5,
	BELLUI_ROLE_SWITCH     = 6,
	BELLUI_ROLE_RADIO      = 7,
	BELLUI_ROLE_TAB        = 8,
	BELLUI_ROLE_TEXTFIELD  = 9,
	BELLUI_ROLE_TEXTAREA   = 10,
	BELLUI_ROLE_PASSWORD   = 11,
	BELLUI_ROLE_SEARCH     = 12,
	BELLUI_ROLE_SLIDER     = 13,
	BELLUI_ROLE_PROGRESS   = 14,
	BELLUI_ROLE_SPIN       = 15,
	BELLUI_ROLE_HEADING    = 16,
	BELLUI_ROLE_LISTITEM   = 17,
	BELLUI_ROLE_COMBO      = 18,
	BELLUI_ROLE_MENUITEM   = 19,
	BELLUI_ROLE_TREEITEM   = 20,
	BELLUI_ROLE_CELL       = 21,
	BELLUI_ROLE_GRID       = 22,
};

enum {
	BELLUI_FLAG_DISABLED    = 1 << 0,
	BELLUI_FLAG_SELECTED    = 1 << 1,
	BELLUI_FLAG_CHECKED     = 1 << 2,
	BELLUI_FLAG_CHECKABLE   = 1 << 3,
	BELLUI_FLAG_PRESSABLE   = 1 << 4,
	BELLUI_FLAG_ADJUSTABLE  = 1 << 5,
	BELLUI_FLAG_DISMISSABLE = 1 << 6,
	BELLUI_FLAG_FOCUSED     = 1 << 7,
	BELLUI_FLAG_SCROLLABLE  = 1 << 8,
};

enum {
	BELLUI_A11Y_ACTIVATE        = 1,
	BELLUI_A11Y_INCREMENT       = 2,
	BELLUI_A11Y_DECREMENT       = 3,
	BELLUI_A11Y_DISMISS         = 4,
	BELLUI_A11Y_SCROLL_FORWARD  = 6,
	BELLUI_A11Y_SCROLL_BACKWARD = 7,
	// Custom-action kind block (pre-rebrand tracker #1027): a declared widget action with no
	// standard iOS semantic dispatches as base + its Go registry id. Keep in
	// sync with accessibility.go's mobileA11yActionCustom.
	BELLUI_A11Y_ACTION_CUSTOM   = 0x1000,
};

// Go exports (accessibility_ios.go).
extern void belluiMobileA11yAction(uintptr_t handle, int action);
extern void belluiMobileA11ySetFocus(uintptr_t handle, int focused);
extern uintptr_t belluiMobileA11yRealizeCell(uintptr_t grid, long row, long col, BelluiA11yProps *out);

// The hidden text-input proxy (app/darwin_ios.m). When a text element holds
// the canvas focus, the bridge publishes this real UITextField in its place:
// iOS raises the software keyboard only when VoiceOver focus sits on the
// actual first responder, so a synthetic element can never start editing
// (pre-rebrand tracker #570 — the same substitution Flutter performs for its text fields).
extern UIView *belluiKeyboardInputView(void);

// Arms the proxy's one-shot responder cycle, performed when VoiceOver focus
// lands on it (app/darwin_ios.m) — the moment iOS is willing to raise the
// keyboard.
extern void belluiKeyboardExpectRaise(void);

// Pushes the focused editable node's visual-layout sidecar (pre-rebrand tracker #1107) into the
// input proxy so it answers VoiceOver's per-line/caret geometry from bellui's
// real per-character rectangles rather than the proxy's own UITextView layout.
// wire is the L:/C' encoding (mobileA11yTextLayout); originX/originY are the
// node's origin in native pixels and scale the native screen scale, so the
// proxy can map the canvas-pixel rects into its own point-space bounds. A NULL
// or empty wire clears the stored layout (the overrides then fall back to the
// proxy's native geometry).
extern void belluiInputViewSetTextLayout(const char *wire, double originX, double originY, double scale);

// The element that stands in for the focused entry while VoiceOver edits it
// (app/darwin_ios.m, pre-rebrand tracker #1124): the BelluiTextInputView input view itself, which is
// both the first responder and the VoiceOver-focused element (the same-object
// model native text fields use, required for VoiceOver
// to drive the insertion point). nil when there is no input view.
extern id belluiTextEditingA11yElement(void);

// Overlays the input view on the edited entry (pre-rebrand tracker #1124): a real on-screen frame
// (VoiceOver refuses to focus a zero-frame view) with a clear caret tint.
extern void belluiPrepareSwapSurrogateFrame(CGRect entryFrameInView);

// Parks the input view again after editing: zero frame and invisible to
// accessibility (pre-rebrand tracker #951) — a parked view must not linger as a stray zero-frame
// text-input element beside the real element tree.
extern void belluiParkSwapSurrogate(void);

// The root view hosting the accessibility elements: the GL view controller's
// view, looked up dynamically so a recreated window keeps working.
static UIView *belluiA11yContainerView(void) {
	UIWindow *window = nil;
	for (UIWindow *w in [[UIApplication sharedApplication] windows]) {
		if (w.isKeyWindow) {
			window = w;
			break;
		}
	}
	if (window == nil) {
		NSArray<UIWindow *> *windows = [[UIApplication sharedApplication] windows];
		if (windows.count > 0) {
			window = windows[0];
		}
	}
	return window.rootViewController.view;
}

@class BelluiA11yElement;

// handle → BelluiA11yElement. Touched only on the main dispatch queue (the
// realize path in accessibilityDataTableCellElementForRow:column: also runs
// there — VoiceOver calls arrive on the UIKit main thread).
static NSMutableDictionary<NSNumber *, BelluiA11yElement *> *belluiA11yElements = nil;

static void belluiA11yApplyProps(BelluiA11yElement *elem, int role, int flags,
	NSString *label, NSString *value, NSString *hint, NSString *customActs,
	NSString *textLayout, float x, float y, float w, float h,
	uintptr_t gridHandle, long cellRow, long cellCol, long rowCount, long colCount);

// BelluiA11yCustomAction carries the Go-side registry id of a declared widget
// action (pre-rebrand tracker #1027) so the rotor handler can dispatch it without a lookup table.
@interface BelluiA11yCustomAction : UIAccessibilityCustomAction {
@public
	uintptr_t elemHandle;
	long goId;
}
@end

@implementation BelluiA11yCustomAction
@end

@interface BelluiA11yElement : UIAccessibilityElement {
@public
	uintptr_t handle;
	CGRect frameInView; // view points; converted to screen coords per read
	int role;
	int flags;
	BOOL pressable;
	BOOL adjustable;
	BOOL dismissable;
	BOOL scrollable;
	uintptr_t gridHandle; // enclosing BELLUI_ROLE_GRID element, 0 when none (pre-rebrand tracker #817)
	long cellRow, cellCol; // display grid coordinates, -1 when not a grid item
	// Visual-layout sidecar (pre-rebrand tracker #1107), retained so it can be re-pushed to the input
	// proxy on the frame the swap actually happens (when the swapped handle is
	// only established later, in BelluiA11ySetElements). "" for non-text elements.
	NSString *textLayoutWire;
	// Last-applied custom-actions wire (pre-rebrand tracker #951): the rotor entries are rebuilt
	// only when this changes, so an unrelated repaint never churns the focused
	// element's action objects.
	NSString *customActsWire;
	float layoutOriginX, layoutOriginY; // node origin in native px, for the map
}
@end

@implementation BelluiA11yElement

- (void)dealloc {
	[textLayoutWire release];
	[customActsWire release];
	[super dealloc];
}

// The stored frame is view-local so window moves/rotation need no tree
// update; VoiceOver reads the screen frame lazily on the main thread where
// the conversion is legal.
- (CGRect)accessibilityFrame {
	UIView *container = belluiA11yContainerView();
	if (container == nil) {
		return CGRectZero;
	}
	return UIAccessibilityConvertFrameToScreenCoordinates(frameInView, container);
}

- (BOOL)accessibilityActivate {
	if (!pressable) {
		// NO makes VoiceOver fall back to a synthesized tap at the
		// activation point, which flows through the normal touch path.
		return NO;
	}
	belluiMobileA11yAction(handle, BELLUI_A11Y_ACTIVATE);
	return YES;
}

- (void)accessibilityIncrement {
	if (adjustable) {
		belluiMobileA11yAction(handle, BELLUI_A11Y_INCREMENT);
	}
}

- (void)accessibilityDecrement {
	if (adjustable) {
		belluiMobileA11yAction(handle, BELLUI_A11Y_DECREMENT);
	}
}

- (BOOL)accessibilityPerformEscape {
	if (!dismissable) {
		return NO;
	}
	belluiMobileA11yAction(handle, BELLUI_A11Y_DISMISS);
	return YES;
}

// VoiceOver's three-finger scroll (pre-rebrand tracker #952): an element inside a scroll area
// routes the gesture to its nearest scroller, which pages by ~a screenful —
// the iOS mirror of TalkBack's ACTION_SCROLL_FORWARD/BACKWARD (pre-rebrand tracker #762). A swipe
// up (or left / "next") reveals the content beyond the viewport, matching a
// physical one-finger swipe without VoiceOver. Returning NO on non-scrollable
// elements lets UIKit bubble the request to an ancestor.
- (BOOL)accessibilityScroll:(UIAccessibilityScrollDirection)direction {
	if (!scrollable) {
		return NO;
	}
	int action;
	switch (direction) {
	case UIAccessibilityScrollDirectionUp:
	case UIAccessibilityScrollDirectionLeft:
	case UIAccessibilityScrollDirectionNext:
		action = BELLUI_A11Y_SCROLL_FORWARD;
		break;
	case UIAccessibilityScrollDirectionDown:
	case UIAccessibilityScrollDirectionRight:
	case UIAccessibilityScrollDirectionPrevious:
		action = BELLUI_A11Y_SCROLL_BACKWARD;
		break;
	default:
		return NO;
	}
	belluiMobileA11yAction(handle, action);
	// The scroll itself runs on the Go main goroutine; give VoiceOver its
	// page-turn feedback now (no page numbers are available to announce), and
	// let the element diff from the repaint reposition the cursor.
	UIAccessibilityPostNotification(UIAccessibilityPageScrolledNotification, nil);
	return YES;
}

// VoiceOver focus entering or leaving this element drives the in-scene focus
// ring (pre-rebrand tracker #798): the OS ring cannot draw over Bellui's flat Metal surface, so Bellui
// paints its own over the focused element — the iOS mirror of the Android
// accessibilityFocusChanged hook. Text elements are represented by the real
// input view while editing (pre-rebrand tracker #570), which VoiceOver rings natively, so this
// covers the non-text elements that have no real view to outline.
- (void)accessibilityElementDidBecomeFocused {
	belluiMobileA11ySetFocus(handle, 1);
}

- (void)accessibilityElementDidLoseFocus {
	belluiMobileA11ySetFocus(handle, 0);
}

// VoiceOver actions-rotor handler (pre-rebrand tracker #1027): dispatch the picked custom action
// back to Go as kind = custom base + registry id. The Go side re-validates
// against the node's current declaration, so a stale rotor pick is a no-op.
- (BOOL)belluiPerformCustomAction:(BelluiA11yCustomAction *)action {
	belluiMobileA11yAction(action->elemHandle, (int)(BELLUI_A11Y_ACTION_CUSTOM + action->goId));
	return YES;
}

@end

// BelluiA11yCellElement is an element that is a cell of a virtualized grid
// (pre-rebrand tracker #817). Only cells adopt UIAccessibilityContainerDataTableCell — protocol
// conformance is class-level, and a non-cell element that conforms (answering
// NSNotFound for its ranges) reads to VoiceOver as a malformed table cell,
// derailing its whole flick-navigation pass (pre-rebrand tracker #951: every flick rejection-
// beeped while the base class carried the protocol).
@interface BelluiA11yCellElement : BelluiA11yElement <UIAccessibilityContainerDataTableCell>
@end

@implementation BelluiA11yCellElement

// VoiceOver announces "row R column C" and navigates by row/column when the
// enclosing grid element adopts the data-table container protocol.
- (NSRange)accessibilityRowRange {
	if (cellRow < 0) {
		return NSMakeRange(NSNotFound, 0);
	}
	return NSMakeRange((NSUInteger)cellRow, 1);
}

- (NSRange)accessibilityColumnRange {
	if (cellCol < 0) {
		return NSMakeRange(NSNotFound, 0);
	}
	return NSMakeRange((NSUInteger)cellCol, 1);
}

@end

// belluiA11yParseCustomActions builds the rotor entries from the encoded wire
// string (pre-rebrand tracker #1027): goId U+001F label entries separated by U+001E — the same
// form the Android bridge parses. Returns nil when the node offers none.
// target is the element whose handler dispatches; malformed entries are
// skipped.
static NSArray<UIAccessibilityCustomAction *> *belluiA11yParseCustomActions(
	NSString *encoded, BelluiA11yElement *target) {
	if (encoded.length == 0) {
		return nil;
	}
	NSMutableArray *actions = [NSMutableArray array];
	NSString *rs = [NSString stringWithFormat:@"%C", (unichar)0x1e];
	NSString *us = [NSString stringWithFormat:@"%C", (unichar)0x1f];
	for (NSString *entry in [encoded componentsSeparatedByString:rs]) {
		NSRange sep = [entry rangeOfString:us];
		if (sep.location == NSNotFound || sep.location == 0) {
			continue;
		}
		NSString *idPart = [entry substringToIndex:sep.location];
		NSString *label = [entry substringFromIndex:sep.location + sep.length];
		if (label.length == 0) {
			continue;
		}
		BelluiA11yCustomAction *action = [[[BelluiA11yCustomAction alloc]
			initWithName:label
			      target:target
			    selector:@selector(belluiPerformCustomAction:)] autorelease];
		action->elemHandle = target->handle;
		action->goId = [idPart integerValue];
		[actions addObject:action];
	}
	return actions.count > 0 ? actions : nil;
}

// BelluiA11yGridElement is the container element for a virtualized grid (pre-rebrand tracker #817):
// the one container the flat sequence keeps. It adopts
// UIAccessibilityContainerDataTable so VoiceOver hears the full logical extent
// (accessibilityRowCount x accessibilityColumnCount) and can address any cell —
// including one far past the viewport, realized on demand through the Go seam —
// while its accessibilityElements hold only the visible (walked) cells. The
// container itself is not an accessibility element; VoiceOver descends into it.
@interface BelluiA11yGridElement : BelluiA11yElement <UIAccessibilityContainerDataTable> {
@public
	long rowCount, colCount;
}
@end

@implementation BelluiA11yGridElement

- (BOOL)isAccessibilityElement {
	return NO;
}

- (NSUInteger)accessibilityRowCount {
	return rowCount > 0 ? (NSUInteger)rowCount : 0;
}

- (NSUInteger)accessibilityColumnCount {
	return colCount > 0 ? (NSUInteger)colCount : 0;
}

- (id<UIAccessibilityContainerDataTableCell>)accessibilityDataTableCellElementForRow:(NSUInteger)row
                                                                              column:(NSUInteger)column {
	// A visible (walked) cell answers directly, keeping its element identity.
	for (id candidate in self.accessibilityElements) {
		if (![candidate isKindOfClass:[BelluiA11yCellElement class]]) {
			continue;
		}
		BelluiA11yCellElement *cell = (BelluiA11yCellElement *)candidate;
		if (cell->cellRow == (long)row && cell->cellCol == (long)column) {
			return cell;
		}
	}

	// Off-viewport: realize through the Go seam. The call is synchronous on
	// the UIKit main thread with a bounded wait on the Go side; the returned
	// strings are Go-allocated and freed here.
	BelluiA11yProps props;
	memset(&props, 0, sizeof(props));
	uintptr_t cellHandle = belluiMobileA11yRealizeCell(handle, (long)row, (long)column, &props);
	if (cellHandle == 0) {
		return nil;
	}

	NSString *label = props.label ? [NSString stringWithUTF8String:props.label] : @"";
	NSString *value = props.value ? [NSString stringWithUTF8String:props.value] : @"";
	NSString *hint = props.hint ? [NSString stringWithUTF8String:props.hint] : @"";
	NSString *customActs = props.customActs ? [NSString stringWithUTF8String:props.customActs] : @"";
	NSString *textLayout = props.textLayout ? [NSString stringWithUTF8String:props.textLayout] : @"";
	free((void *)props.label);
	free((void *)props.value);
	free((void *)props.hint);
	free((void *)props.customActs);
	free((void *)props.textLayout);

	if (belluiA11yElements == nil) {
		belluiA11yElements = [[NSMutableDictionary alloc] init];
	}
	NSNumber *key = [NSNumber numberWithUnsignedLong:(unsigned long)cellHandle];
	BelluiA11yCellElement *elem = (BelluiA11yCellElement *)belluiA11yElements[key];
	if (elem == nil) {
		elem = [[[BelluiA11yCellElement alloc] initWithAccessibilityContainer:self] autorelease];
		elem->handle = cellHandle;
		belluiA11yElements[key] = elem; // dictionary retains
	}
	belluiA11yApplyProps(elem, props.role, props.flags, label, value, hint, customActs,
		textLayout, props.x, props.y, props.w, props.h,
		props.gridHandle, props.cellRow, props.cellCol, props.rowCount, props.colCount);
	return elem;
}

@end

// Handle of the text element currently represented by the real input proxy,
// 0 when none. Main queue only.
static uintptr_t belluiA11ySwappedHandle = 0;

static BOOL belluiA11yIsTextRole(int role) {
	switch (role) {
	case BELLUI_ROLE_TEXTFIELD:
	case BELLUI_ROLE_TEXTAREA:
	case BELLUI_ROLE_PASSWORD:
	case BELLUI_ROLE_SEARCH:
		return YES;
	}
	return NO;
}

static UIAccessibilityTraits belluiA11yTraits(int role, int flags) {
	UIAccessibilityTraits traits = UIAccessibilityTraitNone;
	switch (role) {
	case BELLUI_ROLE_BUTTON:
	case BELLUI_ROLE_CHECKBOX:
	case BELLUI_ROLE_SWITCH:
	case BELLUI_ROLE_RADIO:
	case BELLUI_ROLE_TAB:
	case BELLUI_ROLE_SPIN:
		traits |= UIAccessibilityTraitButton;
		break;
	case BELLUI_ROLE_LINK:
		traits |= UIAccessibilityTraitLink;
		break;
	case BELLUI_ROLE_IMAGE:
		traits |= UIAccessibilityTraitImage;
		break;
	case BELLUI_ROLE_TEXT:
	case BELLUI_ROLE_PROGRESS:
		traits |= UIAccessibilityTraitStaticText;
		break;
	case BELLUI_ROLE_HEADING:
		traits |= UIAccessibilityTraitHeader;
		break;
	case BELLUI_ROLE_SEARCH:
		traits |= UIAccessibilityTraitSearchField;
		break;
	default:
		break;
	}
	if (flags & BELLUI_FLAG_SELECTED) {
		traits |= UIAccessibilityTraitSelected;
	}
	if (flags & BELLUI_FLAG_CHECKED) {
		traits |= UIAccessibilityTraitSelected;
	}
	if (flags & BELLUI_FLAG_DISABLED) {
		traits |= UIAccessibilityTraitNotEnabled;
	}
	if (flags & BELLUI_FLAG_ADJUSTABLE) {
		traits |= UIAccessibilityTraitAdjustable;
	}
	return traits;
}

// belluiPushTextLayout hands the element's sidecar to the input proxy (pre-rebrand tracker #1107),
// mapping its canvas-pixel rects into the proxy's point-space via the node
// origin and native scale. Called for the swapped (edited) element only.
static void belluiPushTextLayout(BelluiA11yElement *elem) {
	if (belluiKeyboardInputView() == nil) {
		return;
	}
	CGFloat scale = [UIScreen mainScreen].nativeScale;
	if (scale <= 0) {
		scale = 1;
	}
	const char *wire = elem->textLayoutWire.length > 0 ? [elem->textLayoutWire UTF8String] : NULL;
	// The focused editing element is the real input view, positioned at the
	// entry (elem->frameInView, pre-rebrand tracker #1124). Its UITextInput geometry is therefore
	// view-local: map canvas-pixel rects to the view's own coordinate space via
	// the node origin.
	belluiInputViewSetTextLayout(wire, elem->layoutOriginX, elem->layoutOriginY, scale);
}

// belluiA11yStringEq reports whether two property strings are equal (nil-safe),
// so setters below run only on a real change. The desktop bridges diff props
// in C/ObjC so the AT cursor survives unrelated repaints (pre-rebrand tracker #485/#487); the same
// discipline applies here — without it every repaint reassigned the focused
// element's label/traits and rebuilt its custom-action objects (pre-rebrand tracker #951).
static BOOL belluiA11yStringEq(NSString *a, NSString *b) {
	return (a == b) || (a != nil && b != nil && [a isEqualToString:b]);
}

// belluiA11yApplyProps writes one element's properties; main dispatch queue only.
// Shared by the per-frame update path (BelluiA11yUpdateElement) and the
// synchronous realize path (accessibilityDataTableCellElementForRow:column:).
static void belluiA11yApplyProps(BelluiA11yElement *elem, int role, int flags,
	NSString *label, NSString *value, NSString *hint, NSString *customActs,
	NSString *textLayout, float x, float y, float w, float h,
	uintptr_t gridHandle, long cellRow, long cellCol, long rowCount, long colCount) {
	if (!belluiA11yStringEq(elem.accessibilityLabel, label)) {
		elem.accessibilityLabel = label;
	}
	// Declared widget actions ride VoiceOver's actions rotor (pre-rebrand tracker #1027). Rebuild
	// the action objects only when the declaration wire actually changed.
	if (!belluiA11yStringEq(elem->customActsWire, customActs)) {
		[elem->customActsWire release];
		elem->customActsWire = [customActs copy];
		elem.accessibilityCustomActions = belluiA11yParseCustomActions(customActs, elem);
	}
	NSString *newValue = value.length > 0 ? value : nil;
	if (!belluiA11yStringEq(elem.accessibilityValue, newValue)) {
		elem.accessibilityValue = newValue;
	}
	NSString *newHint = hint.length > 0 ? hint : nil;
	if (!belluiA11yStringEq(elem.accessibilityHint, newHint)) {
		elem.accessibilityHint = newHint;
	}
	UIAccessibilityTraits newTraits = belluiA11yTraits(role, flags);
	if (elem.accessibilityTraits != newTraits) {
		elem.accessibilityTraits = newTraits;
	}
	elem->role = role;
	elem->flags = flags;
	elem->pressable = (flags & BELLUI_FLAG_PRESSABLE) != 0;
	elem->adjustable = (flags & BELLUI_FLAG_ADJUSTABLE) != 0;
	elem->dismissable = (flags & BELLUI_FLAG_DISMISSABLE) != 0;
	elem->scrollable = (flags & BELLUI_FLAG_SCROLLABLE) != 0;
	elem->gridHandle = gridHandle;
	elem->cellRow = cellRow;
	elem->cellCol = cellCol;
	if ([elem isKindOfClass:[BelluiA11yGridElement class]]) {
		BelluiA11yGridElement *grid = (BelluiA11yGridElement *)elem;
		grid->rowCount = rowCount;
		grid->colCount = colCount;
		if (@available(iOS 11.0, *)) {
			grid.accessibilityContainerType = UIAccessibilityContainerTypeDataTable;
		}
	}

	CGFloat scale = [UIScreen mainScreen].nativeScale;
	if (scale <= 0) {
		scale = 1;
	}
	elem->frameInView = CGRectMake(x / scale, y / scale, w / scale, h / scale);

	// Retain the visual-layout sidecar and the node origin so the swap path can
	// re-push it even on a frame where only the swap (not the props) changed.
	if (![elem->textLayoutWire isEqualToString:textLayout]) {
		[elem->textLayoutWire release];
		elem->textLayoutWire = [textLayout copy];
	}
	elem->layoutOriginX = x;
	elem->layoutOriginY = y;

	if (elem->handle == belluiA11ySwappedHandle) {
		// The entry being edited moved, relabeled or changed its action set:
		// keep the swap element glued to it — including the rotor entries, so
		// an availability flip (canUndo/canRedo) mid-edit reaches VoiceOver
		// (pre-rebrand tracker #1027).
		id swap = belluiTextEditingA11yElement();
		if (swap != nil) {
			[swap setAccessibilityLabel:label];
			[swap setAccessibilityCustomActions:elem.accessibilityCustomActions];
			[swap setAccessibilityFrame:[elem accessibilityFrame]];
			belluiPrepareSwapSurrogateFrame(elem->frameInView);
		}
		// Keep the proxy's per-line/caret geometry in step with the caret (pre-rebrand tracker #1107):
		// the sidecar re-encodes on every caret move (its window follows selEnd).
		belluiPushTextLayout(elem);
	}
}

void BelluiA11yUpdateElement(uintptr_t handle, BelluiA11yProps props) {
	// Copy everything out of the Go-owned memory before leaving this stack.
	NSString *label = props.label ? [NSString stringWithUTF8String:props.label] : @"";
	NSString *value = props.value ? [NSString stringWithUTF8String:props.value] : @"";
	NSString *hint = props.hint ? [NSString stringWithUTF8String:props.hint] : @"";
	NSString *customActs = props.customActs ? [NSString stringWithUTF8String:props.customActs] : @"";
	NSString *textLayout = props.textLayout ? [NSString stringWithUTF8String:props.textLayout] : @"";
	int role = props.role, flags = props.flags;
	float x = props.x, y = props.y, w = props.w, h = props.h;
	uintptr_t gridHandle = props.gridHandle;
	long cellRow = props.cellRow, cellCol = props.cellCol;
	long rowCount = props.rowCount, colCount = props.colCount;

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			if (belluiA11yElements == nil) {
				belluiA11yElements = [[NSMutableDictionary alloc] init];
			}
			NSNumber *key = [NSNumber numberWithUnsignedLong:(unsigned long)handle];
			BelluiA11yElement *elem = belluiA11yElements[key];
			BOOL wantGrid = role == BELLUI_ROLE_GRID;
			// A grid container, a grid cell (the only class carrying the
			// DataTableCell protocol) and a plain element are distinct classes.
			Class cls = wantGrid ? [BelluiA11yGridElement class]
			                     : (cellRow >= 0 ? [BelluiA11yCellElement class]
			                                     : [BelluiA11yElement class]);
			if (elem != nil && [elem class] != cls) {
				// A handle switched representation class (essentially never):
				// rebuild it with the right one.
				[belluiA11yElements removeObjectForKey:key];
				elem = nil;
			}
			if (elem == nil) {
				// A cell inside a virtualized grid parents to the grid element
				// (published before its cells, so it already exists); everything
				// else parents to the root view. The container drives VoiceOver's
				// data-table traversal (pre-rebrand tracker #817).
				id container = nil;
				if (gridHandle != 0) {
					container = belluiA11yElements[
						[NSNumber numberWithUnsignedLong:(unsigned long)gridHandle]];
				}
				if (container == nil) {
					container = belluiA11yContainerView();
				}
				if (container == nil) {
					return;
				}
				elem = [[[cls alloc] initWithAccessibilityContainer:container] autorelease];
				elem->handle = handle;
				belluiA11yElements[key] = elem; // dictionary retains
			}

			belluiA11yApplyProps(elem, role, flags, label, value, hint, customActs,
				textLayout, x, y, w, h, gridHandle, cellRow, cellCol, rowCount, colCount);
		}
	});
}

void BelluiA11ySetElements(uintptr_t *handles, int count) {
	// Copy the handle list; the Go slice backing it is reused after return.
	NSMutableArray<NSNumber *> *keys = [NSMutableArray arrayWithCapacity:(NSUInteger)count];
	for (int i = 0; i < count; i++) {
		[keys addObject:[NSNumber numberWithUnsignedLong:(unsigned long)handles[i]]];
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			UIView *container = belluiA11yContainerView();
			if (container == nil) {
				return;
			}

			// Under VoiceOver, a text element holding the canvas focus is
			// represented by the real UITextField input proxy instead of its
			// synthetic element, overlaid on the entry — VoiceOver can then
			// sit on the genuine first responder, which is what makes iOS
			// raise (and keep) the software keyboard (pre-rebrand tracker #570). Only while
			// VoiceOver runs: without it the overlay would intercept real
			// touches meant for the canvas, and the keyboard rises normally
			// anyway.
			uintptr_t swapped = 0;
			BOOL voRunning = UIAccessibilityIsVoiceOverRunning();
			UIView *inputView = belluiKeyboardInputView();

			// Cells inside a virtualized grid (pre-rebrand tracker #817) nest under their grid
			// element instead of the root list, so VoiceOver traverses them as
			// a data table; gridCells collects them per grid handle in order.
			NSMutableArray *elems = [NSMutableArray arrayWithCapacity:keys.count];
			// The editing proxy added this frame (pre-rebrand tracker #1124), for the focus handshake.
			id swapView = nil;
			NSMutableDictionary<NSNumber *, NSMutableArray *> *gridCells =
				[NSMutableDictionary dictionary];
			for (NSNumber *key in keys) {
				BelluiA11yElement *elem = belluiA11yElements[key];
				if (elem == nil) {
					continue;
				}
				if (elem->gridHandle != 0 && ![elem isKindOfClass:[BelluiA11yGridElement class]]) {
					NSNumber *gridKey =
						[NSNumber numberWithUnsignedLong:(unsigned long)elem->gridHandle];
					if (belluiA11yElements[gridKey] != nil) {
						NSMutableArray *cells = gridCells[gridKey];
						if (cells == nil) {
							cells = [NSMutableArray array];
							gridCells[gridKey] = cells;
						}
						[cells addObject:elem];
						continue;
					}
					// Grid element missing (not yet created): fall through and
					// keep the cell reachable at the root.
				}
				if (voRunning && inputView != nil && swapped == 0 && belluiA11yIsTextRole(elem->role) &&
					(elem->flags & BELLUI_FLAG_FOCUSED) != 0) {
					swapped = elem->handle;
					// Represent the focused entry by the real input view itself
					// (pre-rebrand tracker #1124): first responder and VoiceOver-focused element must
					// be the same object for insertion-point navigation.
					id swap = belluiTextEditingA11yElement();
					if (swap == nil) {
						swap = inputView;
					}
					[swap setAccessibilityLabel:elem.accessibilityLabel];
					[swap setAccessibilityHint:elem.accessibilityHint];
					// The rotor must keep offering the entry's declared actions
					// while the proxy stands in (pre-rebrand tracker #1027).
					[swap setAccessibilityCustomActions:elem.accessibilityCustomActions];
					[swap setAccessibilityFrame:[elem accessibilityFrame]];
					// Overlay the input view on the entry with a real frame
					// (VoiceOver refuses to focus a zero-frame view).
					belluiPrepareSwapSurrogateFrame(elem->frameInView);
					// Seed the proxy with bellui's real per-line/caret geometry so
					// VoiceOver's line ring is correct from the first edited frame
					// (pre-rebrand tracker #1107), not just after the first caret move.
					belluiPushTextLayout(elem);
					swapView = swap;
					[elems addObject:swap];
					continue;
				}
				[elems addObject:elem];
			}

			// Seat each grid's visible cells; a scroll changes membership here
			// even when the root list is unchanged.
			BOOL gridChanged = NO;
			for (id rootElem in elems) {
				if (![rootElem isKindOfClass:[BelluiA11yGridElement class]]) {
					continue;
				}
				BelluiA11yGridElement *grid = (BelluiA11yGridElement *)rootElem;
				NSNumber *gridKey =
					[NSNumber numberWithUnsignedLong:(unsigned long)grid->handle];
				NSArray *cells = gridCells[gridKey];
				if (cells == nil) {
					cells = @[];
				}
				NSArray *currentCells = grid.accessibilityElements;
				if (currentCells == nil || ![currentCells isEqualToArray:cells]) {
					grid.accessibilityElements = cells;
					gridChanged = YES;
				}
			}

			uintptr_t prevSwapped = belluiA11ySwappedHandle;
			belluiA11ySwappedHandle = swapped;

			NSArray *current = container.accessibilityElements;
			if (!gridChanged && current != nil && [current isEqualToArray:elems]) {
				return; // same membership and order: keep VoiceOver's cursor
			}
			container.accessibilityElements = elems;

			if (swapped != 0 && swapped != prevSwapped) {
				// Editing starts: move VoiceOver onto the swap element. It
				// re-claims first responder for the keyboard sink when the focus
				// actually lands (the one-shot armed here), which raises the
				// keyboard.
				belluiKeyboardExpectRaise();
				UIAccessibilityPostNotification(UIAccessibilityLayoutChangedNotification, swapView);
			} else if (swapped == 0 && prevSwapped != 0) {
				// Editing ended: park the proxy again. If VoiceOver's cursor
				// was sitting on it, hand focus back to the entry's element
				// so it does not reset to the first element on screen; when
				// the user is already elsewhere (they activated another
				// control), leave their cursor alone.
				id parkElem = belluiTextEditingA11yElement();
				BOOL hadCursor = parkElem != nil &&
					UIAccessibilityFocusedElement(UIAccessibilityNotificationVoiceOverIdentifier) == parkElem;
				if (inputView != nil) {
					// Park the input view (zero frame, a11y-invisible, pre-rebrand tracker #951)
					// and drop the sidecar so a later focus on a field without
					// one (single-line, or a widget with no
					// AccessibleTextLayouter) falls back to the proxy's native
					// geometry (pre-rebrand tracker #1107).
					belluiParkSwapSurrogate();
					inputView.tintColor = nil;
					belluiInputViewSetTextLayout(NULL, 0, 0, 1);
				}
				if (parkElem != nil) {
					[parkElem setAccessibilityCustomActions:nil];
				}
				if (hadCursor) {
					BelluiA11yElement *back = belluiA11yElements[[NSNumber numberWithUnsignedLong:(unsigned long)prevSwapped]];
					UIAccessibilityPostNotification(UIAccessibilityLayoutChangedNotification, back);
				}
			} else {
				UIAccessibilityPostNotification(UIAccessibilityLayoutChangedNotification, nil);
			}
		}
	});
}

void BelluiA11yDestroyElements(uintptr_t *handles, int count) {
	NSMutableArray<NSNumber *> *keys = [NSMutableArray arrayWithCapacity:(NSUInteger)count];
	for (int i = 0; i < count; i++) {
		[keys addObject:[NSNumber numberWithUnsignedLong:(unsigned long)handles[i]]];
	}
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			[belluiA11yElements removeObjectsForKeys:keys];
		}
	});
}

void BelluiA11yClearAll(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			belluiA11ySwappedHandle = 0;
			[belluiA11yElements removeAllObjects];
			UIView *container = belluiA11yContainerView();
			if (container != nil) {
				container.accessibilityElements = nil;
				UIAccessibilityPostNotification(UIAccessibilityLayoutChangedNotification, nil);
			}
		}
	});
}

void BelluiA11yAnnounce(const char *text, int assertive) {
	if (text == NULL) {
		return;
	}
	NSString *message = [NSString stringWithUTF8String:text];
	BOOL interrupt = assertive != 0;
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			id payload = message;
			if (!interrupt) {
				// Queue politely after the current utterance instead of
				// interrupting it.
				payload = [[[NSAttributedString alloc]
					initWithString:message
					    attributes:@{UIAccessibilitySpeechAttributeQueueAnnouncement : @YES}] autorelease];
			}
			UIAccessibilityPostNotification(UIAccessibilityAnnouncementNotification, payload);
		}
	});
}
