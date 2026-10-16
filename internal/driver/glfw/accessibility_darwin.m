//go:build accessibility && darwin && !wasm && !test_web_driver

#import "accessibility_darwin.h"

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

// Implemented in Go (accessibility_darwin.go), called when assistive
// technology invokes an action on an element.
extern bool belluiAccessibilityPerformPress(uintptr_t handle);
extern bool belluiAccessibilityPerformAction(uintptr_t handle, const char* name);
extern void belluiAccessibilitySetFocus(uintptr_t handle);
extern AccessibilityElementRef belluiAccessibilityRealizeCell(uintptr_t tableHandle, int row, int col);

// Text protocol (accessibility_text_darwin.go). Returned char* are
// C.CString allocations owned by the caller — free() after wrapping.
extern long belluiAccessibilityTextLength(uintptr_t handle);
extern bool belluiAccessibilityTextSelection(uintptr_t handle, long* loc, long* length);
extern char* belluiAccessibilityTextSelected(uintptr_t handle);
extern char* belluiAccessibilityTextForRange(uintptr_t handle, long loc, long length);
extern long belluiAccessibilityTextInsertionLine(uintptr_t handle);
extern long belluiAccessibilityTextLineForIndex(uintptr_t handle, long index);
extern bool belluiAccessibilityTextRangeForLine(uintptr_t handle, long line, long* loc, long* length);
extern bool belluiAccessibilityTextFrameForRange(uintptr_t handle, long loc, long length,
    double* x, double* y, double* w, double* h);
extern void belluiAccessibilityTextSetSelection(uintptr_t handle, long loc, long length);
extern void belluiAccessibilityTextReplaceSelection(uintptr_t handle, const char* text);
extern void belluiAccessibilityTextSetValue(uintptr_t handle, const char* text);

// Wraps a Go-allocated C string as NSString and frees it.
static NSString* belluiAXTakeString(char* s) {
    if (!s) {
        return @"";
    }
    NSString* str = [NSString stringWithUTF8String:s];
    free(s);
    return str ?: @"";
}

static char kBelluiAXRootsKey;
static Class swizzledViewClass = nil;
static IMP originalChildrenIMP = NULL;
static IMP originalFocusedIMP = NULL;

@interface BelluiAccessibleElement : NSAccessibilityElement

@property (nonatomic, assign) uintptr_t handle;
@property (nonatomic, assign) int role;
@property (nonatomic, retain) NSString* label;
@property (nonatomic, retain) NSString* help;
@property (nonatomic, retain) NSString* value;
@property (nonatomic, assign) NSRect localFrame;
@property (nonatomic, assign) BOOL enabled;
@property (nonatomic, assign) BOOL focused;
@property (nonatomic, assign) BOOL pressable;
@property (nonatomic, assign) int checked;
@property (nonatomic, assign) BOOL hasRange;
@property (nonatomic, assign) double rangeMin;
@property (nonatomic, assign) double rangeMax;
@property (nonatomic, assign) double rangeValue;
@property (nonatomic, assign) BOOL canIncrement;
@property (nonatomic, assign) BOOL canDecrement;
@property (nonatomic, assign) BOOL textElement;
@property (nonatomic, assign) long selStart;
@property (nonatomic, assign) long selEnd;
@property (nonatomic, assign) int subrole;
@property (nonatomic, assign) int level;
@property (nonatomic, assign) int disclosed;
@property (nonatomic, assign) int cellRow;
@property (nonatomic, assign) int cellCol;
@property (nonatomic, assign) int gridRows;
@property (nonatomic, assign) int gridCols;
@property (nonatomic, assign) int axIndex;
@property (nonatomic, assign) int selected;
// Machine names of the custom actions, parallel to customActions. The
// custom-action objects are rebuilt only when names or titles change.
@property (nonatomic, retain) NSArray<NSString*>* customActionNames;
@property (nonatomic, retain) NSArray<NSAccessibilityCustomAction*>* customActions;
@property (nonatomic, retain) NSMutableArray<BelluiAccessibleElement*>* childElements;
// The parent element, or nil for a root (whose parent is the content view).
// assign (not retain): parents retain children via childElements, so a
// retained back-reference would cycle under manual memory management.
@property (nonatomic, assign) id parentElement;
// The NSView the element belongs to, used to resolve screen frames lazily.
@property (nonatomic, assign) NSView* contentView;

@end

@implementation BelluiAccessibleElement

- (instancetype)init {
    self = [super init];
    if (self) {
        _childElements = [[NSMutableArray alloc] init];
        _cellRow = -1;
        _cellCol = -1;
        _axIndex = -1;
        _selected = -1;
        _enabled = YES;
        _checked = -1;
        _disclosed = -1;
    }
    return self;
}

- (void)dealloc {
    [_childElements release];
    [_label release];
    [_help release];
    [_value release];
    [_customActionNames release];
    [_customActions release];
    [super dealloc];
}

- (NSAccessibilityRole)accessibilityRole {
    switch (self.role) {
        case BelluiAXRoleButton:
            return NSAccessibilityButtonRole;
        case BelluiAXRoleCheckBox:
            return NSAccessibilityCheckBoxRole;
        case BelluiAXRoleRadioButton:
            return NSAccessibilityRadioButtonRole;
        case BelluiAXRoleRadioGroup:
            return NSAccessibilityRadioGroupRole;
        case BelluiAXRoleTextField:
            return NSAccessibilityTextFieldRole;
        case BelluiAXRoleTextArea:
            return NSAccessibilityTextAreaRole;
        case BelluiAXRolePopUpButton:
            return NSAccessibilityPopUpButtonRole;
        case BelluiAXRoleSlider:
            return NSAccessibilitySliderRole;
        case BelluiAXRoleIncrementor:
            return NSAccessibilityIncrementorRole;
        case BelluiAXRoleProgressIndicator:
            return NSAccessibilityProgressIndicatorRole;
        case BelluiAXRoleStaticText:
            return NSAccessibilityStaticTextRole;
        case BelluiAXRoleLink:
            return NSAccessibilityLinkRole;
        case BelluiAXRoleImage:
            return NSAccessibilityImageRole;
        case BelluiAXRoleList:
            return NSAccessibilityListRole;
        case BelluiAXRoleOutline:
            return NSAccessibilityOutlineRole;
        case BelluiAXRoleTable:
            return NSAccessibilityTableRole;
        case BelluiAXRoleRow:
            return NSAccessibilityRowRole;
        case BelluiAXRoleCell:
            return NSAccessibilityCellRole;
        case BelluiAXRoleTabGroup:
            return NSAccessibilityTabGroupRole;
        case BelluiAXRoleMenu:
            return NSAccessibilityMenuRole;
        case BelluiAXRoleMenuItem:
            return NSAccessibilityMenuItemRole;
        case BelluiAXRoleMenuBar:
            return NSAccessibilityMenuBarRole;
        case BelluiAXRoleToolbar:
            return NSAccessibilityToolbarRole;
        case BelluiAXRoleScrollBar:
            return NSAccessibilityScrollBarRole;
        case BelluiAXRoleHeading:
            // No AppKit constant: "AXHeading" is the WebKit convention the
            // VoiceOver heading rotor keys off (level travels in AXValue).
            return @"AXHeading";
        case BelluiAXRoleGroup:
        default:
            return NSAccessibilityGroupRole;
    }
}

- (NSAccessibilitySubrole)accessibilitySubrole {
    switch (self.subrole) {
        case BelluiAXSubroleSecureTextField:
            return NSAccessibilitySecureTextFieldSubrole;
        case BelluiAXSubroleContentList:
            return NSAccessibilityContentListSubrole;
        case BelluiAXSubroleOutlineRow:
            return NSAccessibilityOutlineRowSubrole;
        default:
            return [super accessibilitySubrole];
    }
}

- (NSString*)accessibilityLabel {
    return self.label;
}

- (NSString*)accessibilityTitle {
    return self.label;
}

- (NSString*)accessibilityHelp {
    return self.help.length > 0 ? self.help : nil;
}

- (id)accessibilityValue {
    if (self.checked >= 0) {
        return @(self.checked);
    }
    if (self.role == BelluiAXRoleHeading && self.level > 0) {
        // Headings report their level as AXValue (WebKit convention); the
        // text itself is the title.
        return @(self.level);
    }
    // Range-based controls report numeric values; everything else prefers
    // the textual value (an entry's AXValue is its text, not a number).
    BOOL numeric = self.role == BelluiAXRoleSlider || self.role == BelluiAXRoleIncrementor ||
                   self.role == BelluiAXRoleProgressIndicator || self.role == BelluiAXRoleScrollBar;
    if (self.hasRange && numeric) {
        return @(self.rangeValue);
    }
    if (self.value.length > 0) {
        return self.value;
    }
    if (self.hasRange) {
        return @(self.rangeValue);
    }
    if (self.role == BelluiAXRoleStaticText) {
        // Static text is announced via AXValue, not AXTitle.
        return self.label;
    }
    return nil;
}

- (id)accessibilityMinValue {
    return self.hasRange ? @(self.rangeMin) : nil;
}

- (id)accessibilityMaxValue {
    return self.hasRange ? @(self.rangeMax) : nil;
}

- (NSRect)accessibilityFrame {
    NSView* v = self.contentView;
    if (!v || ![v window]) {
        return NSZeroRect;
    }

    // View rectangle in screen coordinates (bottom-left origin), then place
    // the top-left-origin local frame inside it.
    NSRect viewScreen = [[v window] convertRectToScreen:[v convertRect:[v bounds] toView:nil]];
    NSRect lf = self.localFrame;
    double screenX = viewScreen.origin.x + lf.origin.x;
    double screenY = viewScreen.origin.y + viewScreen.size.height - lf.origin.y - lf.size.height;
    return NSMakeRect(screenX, screenY, lf.size.width, lf.size.height);
}

- (id)accessibilityParent {
    if (self.parentElement) {
        return self.parentElement;
    }
    return self.contentView;
}

// Table structure (pre-rebrand tracker #574): macOS derives a table's shape from its AXRow
// children — without them VoiceOver announces "empty table". The bridge
// synthesizes row elements grouping the realized cells (virtualized tables
// only expose visible rows; the full extent travels in AXRowCount).
// The table-protocol selectors are only OFFERED on the roles they apply to
// (gated in isAccessibilitySelectorAllowed below): NSAccessibilityElement
// advertises every implemented selector as an attribute, and a static text
// or tree row that suddenly reports AXIndex, AXRowIndexRange and AXSelected
// (with NSNotFound values) stops being announced by VoiceOver at all — the
// regression the first cut of pre-rebrand tracker #574 shipped.
static void belluiAXCollectRows(NSArray* children, NSMutableArray* into) {
    for (BelluiAccessibleElement* c in children) {
        if (c.role == BelluiAXRoleRow) {
            [into addObject:c];
            // Outline rows nest: a branch row's child rows are part of the
            // flattened visible row list VoiceOver expects.
            belluiAXCollectRows(c.childElements, into);
        }
    }
}

- (NSArray*)accessibilityRows {
    if (self.role != BelluiAXRoleTable && self.role != BelluiAXRoleOutline) {
        return nil;
    }
    NSMutableArray* rows = [NSMutableArray array];
    belluiAXCollectRows(self.childElements, rows);
    return rows;
}

- (NSArray*)accessibilityVisibleRows {
    return [self accessibilityRows];
}

// VoiceOver derives a table's spoken selection summary ("No selection" /
// the selected cell's content) from the table element's selected-cells and
// selected-rows attributes, not from the cells' own AXSelected state.
- (NSArray*)accessibilitySelectedCells {
    NSMutableArray* sel = [NSMutableArray array];
    for (BelluiAccessibleElement* row in [self accessibilityRows]) {
        for (BelluiAccessibleElement* cell in row.childElements) {
            if (cell.role == BelluiAXRoleCell && cell.selected == 1) {
                [sel addObject:cell];
            }
        }
    }
    return sel;
}

- (NSArray*)accessibilitySelectedRows {
    NSMutableArray* sel = [NSMutableArray array];
    for (BelluiAccessibleElement* row in [self accessibilityRows]) {
        if (row.selected == 1) {
            [sel addObject:row];
            continue;
        }
        for (BelluiAccessibleElement* cell in row.childElements) {
            if (cell.role == BelluiAXRoleCell && cell.selected == 1) {
                [sel addObject:row];
                break;
            }
        }
    }
    return sel;
}

- (NSArray*)accessibilitySelectedChildren {
    return [self accessibilitySelectedRows];
}

- (NSInteger)accessibilityRowCount {
    if (self.role == BelluiAXRoleTable && self.gridRows > 0) {
        return self.gridRows;
    }
    NSArray* rows = [self accessibilityRows];
    return rows != nil ? (NSInteger)rows.count : 0;
}

- (NSInteger)accessibilityColumnCount {
    if (self.role == BelluiAXRoleTable && self.gridCols > 0) {
        return self.gridCols;
    }
    return 0;
}

// VoiceOver's table navigation addresses cells by coordinate through this
// method (the NSAccessibilityCellForColumnAndRow attribute). For a virtualized
// table only the visible cells exist as real elements, so a coordinate past the
// viewport is realized on demand through the neutral seam (pre-rebrand tracker #637) — the
// macOS analog of the UIA GridPattern.GetItem path. A visible cell already in a
// synthetic row is returned directly so its AX identity stays stable.
- (id)accessibilityCellForColumn:(NSInteger)column row:(NSInteger)row {
    if (self.role != BelluiAXRoleTable) {
        return nil;
    }
    for (BelluiAccessibleElement* r in [self accessibilityRows]) {
        for (BelluiAccessibleElement* cell in r.childElements) {
            if (cell.role == BelluiAXRoleCell && cell.cellRow == (int)row && cell.cellCol == (int)column) {
                return cell;
            }
        }
    }
    return (id)belluiAccessibilityRealizeCell(self.handle, (int)row, (int)column);
}

- (NSInteger)accessibilityIndex {
    return self.axIndex >= 0 ? self.axIndex : NSNotFound;
}

- (NSRange)accessibilityRowIndexRange {
    if (self.role == BelluiAXRoleCell && self.cellRow >= 0) {
        return NSMakeRange((NSUInteger)self.cellRow, 1);
    }
    return NSMakeRange(NSNotFound, 0);
}

- (NSRange)accessibilityColumnIndexRange {
    if (self.role == BelluiAXRoleCell && self.cellCol >= 0) {
        return NSMakeRange((NSUInteger)self.cellCol, 1);
    }
    return NSMakeRange(NSNotFound, 0);
}

- (BOOL)isAccessibilitySelected {
    return self.selected == 1;
}

- (NSArray*)accessibilityChildren {
    // Native outlines expose their rows FLAT: every visible row (nested
    // ones included) is a direct child of the outline, with the hierarchy
    // expressed through AXDisclosedRows/AXDisclosedByRow links and the
    // disclosure level — never by nesting row elements inside each other.
    // Nested rows reachable both through AXRows and through their parent
    // row's children send VoiceOver's traversal in circles (pre-rebrand tracker #485
    // walkthrough: the tree worked once, then VoiceOver wedged).
    if (self.role == BelluiAXRoleOutline) {
        NSMutableArray* flat = [NSMutableArray array];
        belluiAXCollectRows(self.childElements, flat);
        for (BelluiAccessibleElement* c in self.childElements) {
            if (c.role != BelluiAXRoleRow) {
                [flat addObject:c];
            }
        }
        return flat;
    }
    if (self.role == BelluiAXRoleRow) {
        NSMutableArray* nonRows = [NSMutableArray array];
        for (BelluiAccessibleElement* c in self.childElements) {
            if (c.role != BelluiAXRoleRow) {
                [nonRows addObject:c];
            }
        }
        return nonRows;
    }
    return [[self.childElements copy] autorelease];
}

// The tree's semantic rows are flat siblings with 1-based levels
// (AccessibleLeveler), so the disclosure links VoiceOver navigates are
// derived from the level sequence of the outline's row list: a row
// discloses the following rows exactly one level deeper, up to the next
// row at its own level.
- (NSArray*)accessibilityDisclosedRows {
    BelluiAccessibleElement* outline = (BelluiAccessibleElement*)self.parentElement;
    if (outline == nil) {
        return @[];
    }
    NSArray* rows = [outline accessibilityRows];
    NSMutableArray* disclosed = [NSMutableArray array];
    NSUInteger i = [rows indexOfObjectIdenticalTo:self];
    if (i == NSNotFound) {
        return disclosed;
    }
    for (NSUInteger j = i + 1; j < rows.count; j++) {
        BelluiAccessibleElement* r = rows[j];
        if (r.level <= self.level) {
            break;
        }
        if (r.level == self.level + 1) {
            [disclosed addObject:r];
        }
    }
    return disclosed;
}

- (id)accessibilityDisclosedByRow {
    BelluiAccessibleElement* outline = (BelluiAccessibleElement*)self.parentElement;
    if (outline == nil) {
        return nil;
    }
    NSArray* rows = [outline accessibilityRows];
    NSUInteger i = [rows indexOfObjectIdenticalTo:self];
    if (i == NSNotFound) {
        return nil;
    }
    for (NSInteger j = (NSInteger)i - 1; j >= 0; j--) {
        BelluiAccessibleElement* r = rows[j];
        if (r.level == self.level - 1) {
            return r;
        }
        if (r.level < self.level - 1) {
            break;
        }
    }
    return nil;
}

- (BOOL)isAccessibilityEnabled {
    return self.enabled;
}

- (BOOL)isAccessibilityFocused {
    return self.focused;
}

- (void)setAccessibilityFocused:(BOOL)flag {
    if (flag) {
        belluiAccessibilitySetFocus(self.handle);
    }
}

- (BOOL)accessibilityPerformPress {
    if (!self.pressable || !self.enabled) {
        return NO;
    }
    return belluiAccessibilityPerformPress(self.handle) ? YES : NO;
}

- (BOOL)accessibilityPerformIncrement {
    if (!self.canIncrement || !self.enabled) {
        return NO;
    }
    return belluiAccessibilityPerformAction(self.handle, "increment") ? YES : NO;
}

- (BOOL)accessibilityPerformDecrement {
    if (!self.canDecrement || !self.enabled) {
        return NO;
    }
    return belluiAccessibilityPerformAction(self.handle, "decrement") ? YES : NO;
}

// Gate the optional AX actions per element: implementing the methods on the
// class would otherwise advertise AXPress/AXIncrement/AXDecrement on every
// element regardless of what the widget declares.
- (BOOL)isAccessibilitySelectorAllowed:(SEL)selector {
    if (selector == @selector(accessibilityPerformPress)) {
        return self.pressable && self.enabled;
    }
    if (selector == @selector(accessibilityPerformIncrement)) {
        return self.canIncrement && self.enabled;
    }
    if (selector == @selector(accessibilityPerformDecrement)) {
        return self.canDecrement && self.enabled;
    }
    if (selector == @selector(accessibilityNumberOfCharacters) ||
        selector == @selector(accessibilitySelectedText) ||
        selector == @selector(accessibilitySelectedTextRange) ||
        selector == @selector(accessibilityInsertionPointLineNumber) ||
        selector == @selector(accessibilityVisibleCharacterRange) ||
        selector == @selector(accessibilityStringForRange:) ||
        selector == @selector(accessibilityLineForIndex:) ||
        selector == @selector(accessibilityRangeForLine:) ||
        selector == @selector(accessibilityFrameForRange:)) {
        return self.textElement;
    }
    if (selector == @selector(setAccessibilitySelectedTextRange:) ||
        selector == @selector(setAccessibilitySelectedText:) ||
        selector == @selector(setAccessibilityValue:)) {
        // Read-only entries still reject the edit widget-side; the selector
        // stays allowed so AT can at least move the selection.
        return self.textElement && self.enabled;
    }
    if (selector == @selector(isAccessibilityDisclosed) ||
        selector == @selector(setAccessibilityDisclosed:)) {
        return self.disclosed >= 0;
    }
    if (selector == @selector(accessibilityDisclosedRows) ||
        selector == @selector(accessibilityDisclosedByRow)) {
        return self.role == BelluiAXRoleRow;
    }
    if (selector == @selector(accessibilitySelectedCells)) {
        return self.role == BelluiAXRoleTable;
    }
    if (selector == @selector(accessibilitySelectedRows) ||
        selector == @selector(accessibilitySelectedChildren)) {
        return self.role == BelluiAXRoleTable || self.role == BelluiAXRoleOutline;
    }
    if (selector == @selector(accessibilityRows) ||
        selector == @selector(accessibilityVisibleRows) ||
        selector == @selector(accessibilityRowCount)) {
        // Table structure (pre-rebrand tracker #574): tables and outlines carry the row shape.
        // The row list must be COMPLETE — the flattened visible set with
        // nested outline rows included; a partial list traps VoiceOver's
        // cursor on the container (it cannot navigate out), and no list at
        // all reads as "empty table".
        return self.role == BelluiAXRoleTable || self.role == BelluiAXRoleOutline;
    }
    if (selector == @selector(accessibilityColumnCount)) {
        return self.role == BelluiAXRoleTable;
    }
    if (selector == @selector(accessibilityCellForColumn:row:)) {
        // Off-viewport cell realize (pre-rebrand tracker #637): only tables answer it.
        return self.role == BelluiAXRoleTable;
    }
    if (selector == @selector(accessibilityIndex)) {
        return self.axIndex >= 0;
    }
    if (selector == @selector(accessibilityRowIndexRange) ||
        selector == @selector(accessibilityColumnIndexRange)) {
        return self.role == BelluiAXRoleCell && self.cellRow >= 0;
    }
    if (selector == @selector(isAccessibilitySelected)) {
        return self.selected >= 0;
    }
    if (selector == @selector(accessibilityDisclosureLevel)) {
        return self.disclosed >= 0 || (self.subrole == BelluiAXSubroleOutlineRow && self.level > 0);
    }
    return [super isAccessibilitySelectorAllowed:selector];
}

- (NSArray<NSAccessibilityCustomAction*>*)accessibilityCustomActions {
    return self.customActions;
}

// --- disclosure (tree rows, pre-rebrand tracker #501) -----------------------------------------

- (BOOL)isAccessibilityDisclosed {
    return self.disclosed == 1;
}

- (void)setAccessibilityDisclosed:(BOOL)flag {
    if (!self.enabled || self.disclosed < 0 || (self.disclosed == 1) == (bool)flag) {
        return;
    }
    belluiAccessibilityPerformAction(self.handle, flag ? "expand" : "collapse");
}

- (NSInteger)accessibilityDisclosureLevel {
    // AX disclosure levels are 0-based; the semantic layer's are 1-based.
    return self.level > 0 ? self.level - 1 : 0;
}

// --- NSAccessibility text protocol (pre-rebrand tracker #501) --------------------------------
// Every query resolves live against the widget through the
// belluiAccessibilityText* callbacks; ranges are UTF-16 code units, converted
// on the Go side. Gated per element by isAccessibilitySelectorAllowed.

- (NSInteger)accessibilityNumberOfCharacters {
    return belluiAccessibilityTextLength(self.handle);
}

- (NSRange)accessibilitySelectedTextRange {
    long loc = 0, len = 0;
    if (!belluiAccessibilityTextSelection(self.handle, &loc, &len)) {
        return NSMakeRange(NSNotFound, 0);
    }
    return NSMakeRange(loc, len);
}

- (void)setAccessibilitySelectedTextRange:(NSRange)range {
    belluiAccessibilityTextSetSelection(self.handle, range.location, range.length);
}

- (NSString*)accessibilitySelectedText {
    return belluiAXTakeString(belluiAccessibilityTextSelected(self.handle));
}

- (void)setAccessibilitySelectedText:(NSString*)text {
    belluiAccessibilityTextReplaceSelection(self.handle, text.UTF8String ?: "");
}

- (void)setAccessibilityValue:(id)value {
    if (![value isKindOfClass:[NSString class]]) {
        return;
    }
    belluiAccessibilityTextSetValue(self.handle, ((NSString*)value).UTF8String ?: "");
}

- (NSInteger)accessibilityInsertionPointLineNumber {
    return belluiAccessibilityTextInsertionLine(self.handle);
}

- (NSRange)accessibilityVisibleCharacterRange {
    return NSMakeRange(0, belluiAccessibilityTextLength(self.handle));
}

- (NSString*)accessibilityStringForRange:(NSRange)range {
    return belluiAXTakeString(belluiAccessibilityTextForRange(self.handle, range.location, range.length));
}

- (NSInteger)accessibilityLineForIndex:(NSInteger)index {
    return belluiAccessibilityTextLineForIndex(self.handle, index);
}

- (NSRange)accessibilityRangeForLine:(NSInteger)line {
    long loc = 0, len = 0;
    if (!belluiAccessibilityTextRangeForLine(self.handle, line, &loc, &len)) {
        return NSMakeRange(NSNotFound, 0);
    }
    return NSMakeRange(loc, len);
}

// AXBoundsForRange (pre-rebrand tracker #990): the screen bounds of a character range, behind
// VoiceOver's cursor tracking and magnifier box. Go answers in the same
// window-local space as localFrame (top-left origin, points); convert to the
// bottom-left-origin screen exactly as accessibilityFrame does. An editor
// without layout geometry (or a fully scrolled-out range) falls back to the
// element frame.
- (NSRect)accessibilityFrameForRange:(NSRange)range {
    double x = 0, y = 0, w = 0, h = 0;
    if (!belluiAccessibilityTextFrameForRange(self.handle, range.location, range.length,
            &x, &y, &w, &h)) {
        return [self accessibilityFrame];
    }
    NSView* v = self.contentView;
    if (!v || ![v window]) {
        return NSZeroRect;
    }
    NSRect viewScreen = [[v window] convertRectToScreen:[v convertRect:[v bounds] toView:nil]];
    double screenX = viewScreen.origin.x + x;
    double screenY = viewScreen.origin.y + viewScreen.size.height - y - h;
    return NSMakeRect(screenX, screenY, w, h);
}

// Target of every NSAccessibilityCustomAction on this element; resolves the
// invoked action object back to its machine name and forwards to Go.
- (BOOL)belluiAXPerformCustomAction:(NSAccessibilityCustomAction*)action {
    if (!self.enabled) {
        return NO;
    }
    NSUInteger idx = [self.customActions indexOfObjectIdenticalTo:action];
    if (idx == NSNotFound || idx >= self.customActionNames.count) {
        return NO;
    }
    return belluiAccessibilityPerformAction(self.handle, [self.customActionNames[idx] UTF8String]) ? YES : NO;
}

- (BOOL)isAccessibilityElement {
    // Unlabelled groups with children are pure structure: report NO so
    // VoiceOver navigates through them to the real controls.
    if (self.role == BelluiAXRoleGroup && self.childElements.count > 0 && self.label.length == 0) {
        return NO;
    }
    return YES;
}

- (id)accessibilityHitTest:(NSPoint)point {
    for (BelluiAccessibleElement* child in [self.childElements reverseObjectEnumerator]) {
        id hit = [child accessibilityHitTest:point];
        if (hit) {
            return hit;
        }
    }
    if (NSPointInRect(point, [self accessibilityFrame]) && [self isAccessibilityElement]) {
        return self;
    }
    return nil;
}

@end

// Swizzled accessibilityChildren for the GLFW content view: expose the bellui
// element roots associated with this particular view (multi-window safe),
// appended to whatever the original implementation reports.
static NSArray* belluiAXViewChildren(id self, SEL _cmd) {
    NSArray* roots = objc_getAssociatedObject(self, &kBelluiAXRootsKey);
    NSArray* original = nil;
    if (originalChildrenIMP) {
        original = ((NSArray * (*)(id, SEL)) originalChildrenIMP)(self, _cmd);
    }
    if (!roots || roots.count == 0) {
        return original;
    }
    if (!original || original.count == 0) {
        return roots;
    }
    return [original arrayByAddingObjectsFromArray:roots];
}

static id findFocusedElement(NSArray* elements) {
    for (BelluiAccessibleElement* e in elements) {
        if (e.focused) {
            return e;
        }
        id inner = findFocusedElement(e.childElements);
        if (inner) {
            return inner;
        }
    }
    return nil;
}

// Swizzled accessibilityFocusedUIElement: report the bellui element that has
// keyboard focus so VoiceOver tracks focus changes inside the canvas.
static id belluiAXViewFocusedUIElement(id self, SEL _cmd) {
    NSArray* roots = objc_getAssociatedObject(self, &kBelluiAXRootsKey);
    id focused = findFocusedElement(roots);
    if (focused) {
        return focused;
    }
    if (originalFocusedIMP) {
        return ((id(*)(id, SEL))originalFocusedIMP)(self, _cmd);
    }
    return self;
}

static void ensureViewSwizzled(NSView* view) {
    Class viewClass = [view class];
    if (swizzledViewClass == viewClass) {
        return;
    }

    Method children = class_getInstanceMethod(viewClass, @selector(accessibilityChildren));
    if (children) {
        originalChildrenIMP = method_getImplementation(children);
        method_setImplementation(children, (IMP)belluiAXViewChildren);
    }
    Method focused = class_getInstanceMethod(viewClass, @selector(accessibilityFocusedUIElement));
    if (focused) {
        originalFocusedIMP = method_getImplementation(focused);
        method_setImplementation(focused, (IMP)belluiAXViewFocusedUIElement);
    }
    swizzledViewClass = viewClass;
}

AccessibilityElementRef AccessibilityElementCreate(uintptr_t handle, void* nsWindow) {
    @autoreleasepool {
        BelluiAccessibleElement* elem = [[BelluiAccessibleElement alloc] init];
        elem.handle = handle;
        NSWindow* win = (NSWindow*)nsWindow;
        elem.contentView = [win contentView];
        return (void*)elem; // +1 from alloc, balanced by AccessibilityElementDestroy
    }
}

// AccessibilityRealizedElementConfigure parents a cell realized on demand for an
// off-viewport table coordinate (pre-rebrand tracker #637) to the table element, so its
// accessibilityParent (and lazy screen-frame resolution) works even though it is
// not one of the table's synthetic-row children. parentElement is an assign (weak)
// back-reference — Go owns the sole strong reference to a realized cell and
// destroys it in the same update pass that tears down its table, so the link
// never dangles across a client access.
void AccessibilityRealizedElementConfigure(AccessibilityElementRef ref, AccessibilityElementRef table) {
    @autoreleasepool {
        BelluiAccessibleElement* e = (BelluiAccessibleElement*)ref;
        if (!e) {
            return;
        }
        e.parentElement = (BelluiAccessibleElement*)table;
    }
}

bool AccessibilityElementUpdate(AccessibilityElementRef ref, AccessibilityElementProps* p) {
    @autoreleasepool {
        BelluiAccessibleElement* e = (BelluiAccessibleElement*)ref;
        bool changed = false;
        bool fresh = (e.label == nil); // first update after creation

        if (e.role != p->role) {
            e.role = p->role;
            changed = true;
        }

        NSString* label = p->label ? [NSString stringWithUTF8String:p->label] : @"";
        if (![e.label isEqualToString:label]) {
            e.label = label;
            changed = true;
            if (!fresh) {
                NSAccessibilityPostNotification(e, NSAccessibilityTitleChangedNotification);
            }
        }

        NSString* help = p->help ? [NSString stringWithUTF8String:p->help] : @"";
        if (![e.help isEqualToString:help]) {
            e.help = help;
            changed = true;
        }

        bool valueChanged = false;
        NSString* value = p->value ? [NSString stringWithUTF8String:p->value] : @"";
        if (![e.value isEqualToString:value]) {
            e.value = value;
            valueChanged = true;
        }
        if (e.checked != p->checked) {
            e.checked = p->checked;
            valueChanged = true;
        }
        if (e.hasRange != (BOOL)p->hasRange || e.rangeMin != p->rangeMin ||
            e.rangeMax != p->rangeMax || e.rangeValue != p->rangeValue) {
            e.hasRange = p->hasRange;
            e.rangeMin = p->rangeMin;
            e.rangeMax = p->rangeMax;
            e.rangeValue = p->rangeValue;
            valueChanged = true;
        }
        if (valueChanged) {
            changed = true;
            // Text elements get an enriched value-changed post carrying the
            // edit metadata (AccessibilityElementPostTextEdit, called by the
            // Go reconcile right after this update) — a bare notification
            // here would double-announce the edit.
            if (!fresh && !e.textElement) {
                NSAccessibilityPostNotification(e, NSAccessibilityValueChangedNotification);
            }
        }

        NSRect frame = NSMakeRect(p->x, p->y, p->width, p->height);
        if (!NSEqualRects(e.localFrame, frame)) {
            e.localFrame = frame;
            changed = true;
        }

        if (e.enabled != (BOOL)p->enabled) {
            e.enabled = p->enabled;
            changed = true;
        }
        if (e.pressable != (BOOL)p->pressable) {
            e.pressable = p->pressable;
            changed = true;
        }
        if (e.canIncrement != (BOOL)p->canIncrement) {
            e.canIncrement = p->canIncrement;
            changed = true;
        }
        if (e.canDecrement != (BOOL)p->canDecrement) {
            e.canDecrement = p->canDecrement;
            changed = true;
        }
        if (e.textElement != (BOOL)p->textElement) {
            e.textElement = p->textElement;
            changed = true;
        }
        if (e.subrole != p->subrole) {
            e.subrole = p->subrole;
            changed = true;
        }
        if (e.level != p->level) {
            e.level = p->level;
            changed = true;
        }
        if (e.disclosed != p->disclosed) {
            bool wasDisclosable = (e.disclosed >= 0);
            e.disclosed = p->disclosed;
            changed = true;
            if (!fresh && wasDisclosable && p->disclosed >= 0) {
                NSAccessibilityPostNotification(e, p->disclosed == 1
                                                       ? NSAccessibilityRowExpandedNotification
                                                       : NSAccessibilityRowCollapsedNotification);
            }
        }
        if (e.cellRow != p->cellRow || e.cellCol != p->cellCol) {
            e.cellRow = p->cellRow;
            e.cellCol = p->cellCol;
            changed = true;
        }
        if (e.gridRows != p->gridRows || e.gridCols != p->gridCols) {
            e.gridRows = p->gridRows;
            e.gridCols = p->gridCols;
            changed = true;
        }
        if (e.axIndex != p->axIndex) {
            e.axIndex = p->axIndex;
            changed = true;
        }
        if (e.selected != p->selected) {
            bool becameSelected = (e.selected == 0 && p->selected == 1);
            e.selected = p->selected;
            changed = true;
            if (!fresh && becameSelected) {
                // Tell VoiceOver a selection happened so it speaks the newly
                // selected item (pre-rebrand tracker #574): cells-changed on an enclosing table,
                // rows-changed on an enclosing outline (tree rows).
                id t = e.parentElement;
                while (t != nil && ((BelluiAccessibleElement*)t).role != BelluiAXRoleTable &&
                       ((BelluiAccessibleElement*)t).role != BelluiAXRoleOutline) {
                    t = ((BelluiAccessibleElement*)t).parentElement;
                }
                if (t != nil) {
                    BelluiAccessibleElement* container = (BelluiAccessibleElement*)t;
                    NSAccessibilityPostNotification(container,
                        container.role == BelluiAXRoleTable
                            ? NSAccessibilitySelectedCellsChangedNotification
                            : NSAccessibilitySelectedRowsChangedNotification);
                }
            }
        }
        if (e.selStart != p->selStart || e.selEnd != p->selEnd) {
            e.selStart = p->selStart;
            e.selEnd = p->selEnd;
            if (e.textElement) {
                changed = true;
                // The selection-changed notification is posted by the Go
                // reconcile (AccessibilityElementPostSelectionChanged) with
                // the WebKit-convention state-change type, so VoiceOver can
                // tell an edit's caret movement from cursor navigation.
            }
        }

        // Custom actions: rebuild the NSAccessibilityCustomAction objects only
        // when the (name, title) list actually changed.
        NSMutableArray<NSString*>* names = [NSMutableArray arrayWithCapacity:p->customCount];
        NSMutableArray<NSString*>* titles = [NSMutableArray arrayWithCapacity:p->customCount];
        for (int i = 0; i < p->customCount; i++) {
            [names addObject:(p->customNames[i] ? [NSString stringWithUTF8String:p->customNames[i]] : @"")];
            [titles addObject:(p->customTitles[i] ? [NSString stringWithUTF8String:p->customTitles[i]] : @"")];
        }
        NSMutableArray<NSString*>* oldTitles = [NSMutableArray arrayWithCapacity:e.customActions.count];
        for (NSAccessibilityCustomAction* a in e.customActions) {
            [oldTitles addObject:a.name];
        }
        if (![names isEqualToArray:(e.customActionNames ?: @[])] || ![titles isEqualToArray:oldTitles]) {
            e.customActionNames = names;
            NSMutableArray<NSAccessibilityCustomAction*>* actions = [NSMutableArray arrayWithCapacity:p->customCount];
            for (int i = 0; i < p->customCount; i++) {
                NSAccessibilityCustomAction* action =
                    [[[NSAccessibilityCustomAction alloc] initWithName:titles[i]
                                                                target:e
                                                              selector:@selector(belluiAXPerformCustomAction:)] autorelease];
                [actions addObject:action];
            }
            e.customActions = actions;
            changed = true;
        }

        if (e.focused != (BOOL)p->focused) {
            e.focused = p->focused;
            changed = true;
            // Deliberately not gated on !fresh: an element can be born
            // already focused — a tree row's inline rename entry is created
            // and keyboard-focused in the same frame — and VoiceOver must
            // still be told to move there.
            if (p->focused) {
                NSAccessibilityPostNotification(e, NSAccessibilityFocusedUIElementChangedNotification);
            }
        }

        return changed;
    }
}

bool AccessibilityElementSetChildren(AccessibilityElementRef ref, AccessibilityElementRef* children, int count) {
    @autoreleasepool {
        BelluiAccessibleElement* e = (BelluiAccessibleElement*)ref;

        BOOL same = ((int)e.childElements.count == count);
        if (same) {
            for (int i = 0; i < count; i++) {
                if ([e.childElements objectAtIndex:i] != (id)children[i]) {
                    same = NO;
                    break;
                }
            }
        }
        if (same) {
            return false;
        }

        [e.childElements removeAllObjects];
        for (int i = 0; i < count; i++) {
            BelluiAccessibleElement* child = (BelluiAccessibleElement*)children[i];
            [e.childElements addObject:child];
            child.parentElement = e;
        }
        return true;
    }
}

void AccessibilityElementDestroy(AccessibilityElementRef ref) {
    @autoreleasepool {
        if (!ref) {
            return;
        }
        BelluiAccessibleElement* e = (BelluiAccessibleElement*)ref;
        e.parentElement = nil;
        e.contentView = nil;
        [e release];
    }
}

bool AccessibilityViewSetRoots(void* nsWindow, AccessibilityElementRef* roots, int count) {
    @autoreleasepool {
        NSWindow* win = (NSWindow*)nsWindow;
        NSView* view = [win contentView];
        if (!view) {
            return false;
        }

        ensureViewSwizzled(view);

        NSArray* old = objc_getAssociatedObject(view, &kBelluiAXRootsKey);
        BOOL same = ((int)old.count == count);
        if (same) {
            for (int i = 0; i < count; i++) {
                if ([old objectAtIndex:i] != (id)roots[i]) {
                    same = NO;
                    break;
                }
            }
        }
        if (same) {
            return false;
        }

        NSMutableArray* arr = [NSMutableArray arrayWithCapacity:count];
        for (int i = 0; i < count; i++) {
            BelluiAccessibleElement* root = (BelluiAccessibleElement*)roots[i];
            root.parentElement = nil; // accessibilityParent falls back to the view
            [arr addObject:root];
        }
        objc_setAssociatedObject(view, &kBelluiAXRootsKey, arr, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        return true;
    }
}

void AccessibilityViewClearRoots(void* nsWindow) {
    @autoreleasepool {
        NSWindow* win = (NSWindow*)nsWindow;
        NSView* view = [win contentView];
        if (!view) {
            return;
        }
        objc_setAssociatedObject(view, &kBelluiAXRootsKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        NSAccessibilityPostNotification(view, NSAccessibilityLayoutChangedNotification);
    }
}

// AccessibilityElementPostTextChanged mirrors what native AppKit text views
// emit per edit — plain notifications, no userInfo, selection BEFORE value
// (observed from TextEdit with an AX observer): VoiceOver diffs the text
// itself and needs the caret update first to classify a deletion as a
// deletion; the reverse order (or WebKit-style userInfo payloads, which
// AppKit does not use) makes deletions sound identical to typing.
void AccessibilityElementPostTextChanged(AccessibilityElementRef ref, bool selChanged, bool valueChanged) {
    @autoreleasepool {
        BelluiAccessibleElement* e = (BelluiAccessibleElement*)ref;
        if (selChanged) {
            NSAccessibilityPostNotification(e, NSAccessibilitySelectedTextChangedNotification);
        }
        if (valueChanged) {
            NSAccessibilityPostNotification(e, NSAccessibilityValueChangedNotification);
        }
    }
}

void AccessibilityAnnounce(const char* text, bool assertive) {
    @autoreleasepool {
        NSString* msg = text ? [NSString stringWithUTF8String:text] : @"";
        if (msg.length == 0) {
            return;
        }
        // Post against the frontmost app window so the announcement is
        // associated with the UI the user is in; NSApp is the documented
        // fallback when no window is up yet.
        id target = [NSApp keyWindow];
        if (!target) {
            target = [NSApp mainWindow];
        }
        if (!target) {
            target = NSApp;
        }
        NSDictionary* info = @{
            NSAccessibilityAnnouncementKey : msg,
            NSAccessibilityPriorityKey : @(assertive ? NSAccessibilityPriorityHigh : NSAccessibilityPriorityMedium),
        };
        NSAccessibilityPostNotificationWithUserInfo(
            target, NSAccessibilityAnnouncementRequestedNotification, info);
    }
}

void AccessibilityViewNotifyLayoutChanged(void* nsWindow) {
    @autoreleasepool {
        NSWindow* win = (NSWindow*)nsWindow;
        NSView* view = [win contentView];
        if (!view) {
            return;
        }
        NSAccessibilityPostNotification(view, NSAccessibilityLayoutChangedNotification);
    }
}
