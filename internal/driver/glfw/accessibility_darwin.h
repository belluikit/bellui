//go:build accessibility && darwin && !wasm && !test_web_driver

#ifndef BELLUI_ACCESSIBILITY_DARWIN_H
#define BELLUI_ACCESSIBILITY_DARWIN_H

#include <stdbool.h>
#include <stdint.h>

// Opaque reference to a BelluiAccessibleElement (an NSAccessibilityElement
// subclass defined in accessibility_darwin.m).
typedef void* AccessibilityElementRef;

// The subset of NSAccessibility roles the Go layer maps core.AccessibleRole
// values onto (see a11yRole in accessibility_darwin.go).
typedef enum {
    BelluiAXRoleGroup = 0,
    BelluiAXRoleButton,
    BelluiAXRoleCheckBox,
    BelluiAXRoleRadioButton,
    BelluiAXRoleRadioGroup,
    BelluiAXRoleTextField,
    BelluiAXRoleTextArea,
    BelluiAXRolePopUpButton,
    BelluiAXRoleSlider,
    BelluiAXRoleIncrementor,
    BelluiAXRoleProgressIndicator,
    BelluiAXRoleStaticText,
    BelluiAXRoleLink,
    BelluiAXRoleImage,
    BelluiAXRoleList,
    BelluiAXRoleOutline,
    BelluiAXRoleTable,
    BelluiAXRoleRow,
    BelluiAXRoleCell,
    BelluiAXRoleTabGroup,
    BelluiAXRoleMenu,
    BelluiAXRoleMenuItem,
    BelluiAXRoleMenuBar,
    BelluiAXRoleToolbar,
    BelluiAXRoleScrollBar,
    BelluiAXRoleHeading
} BelluiAXRole;

// Subroles refining a BelluiAXRole (see a11ySubrole in accessibility_darwin.go).
typedef enum {
    BelluiAXSubroleNone = 0,
    BelluiAXSubroleSecureTextField, // password fields
    BelluiAXSubroleContentList,     // lists (vs. table-backed lists)
    BelluiAXSubroleOutlineRow       // tree rows
} BelluiAXSubrole;

// Property snapshot for one element. The bridge diffs against the element's
// current state and posts AX notifications only for actual changes; the
// return value of AccessibilityElementUpdate reports whether anything changed.
typedef struct {
    int role; // BelluiAXRole
    const char* label;
    const char* help;
    const char* value; // textual value, "" when none
    // Local frame relative to the window content view, in points (window
    // coordinates), top-left origin. Screen conversion happens lazily in
    // accessibilityFrame so window moves need no tree update.
    double x, y, width, height;
    bool enabled;
    bool focused;
    bool pressable; // supports AXPress
    int checked;    // -1 not checkable, 0 off, 1 on, 2 mixed
    bool hasRange;
    double rangeMin, rangeMax, rangeValue;
    bool canIncrement; // supports AXIncrement
    bool canDecrement; // supports AXDecrement
    // The element answers the NSAccessibility text protocol (character
    // count, selection, caret line, settable range/text) via the
    // belluiAccessibilityText* callbacks.
    bool textElement;
    // Selection/caret in rune offsets, used only to diff between updates so
    // the bridge posts AXSelectedTextChanged (VoiceOver re-queries the real
    // UTF-16 range through the callbacks). Meaningful only for textElement.
    long selStart, selEnd;
    int subrole; // BelluiAXSubrole
    // 1-based hierarchy level (tree rows, headings); 0 when not applicable.
    int level;
    // AXDisclosing: -1 not disclosable, 0 collapsed, 1 expanded. Setting it
    // fires the node's expand/collapse action.
    int disclosed;
    // Actions with no standard AX equivalent, exposed as
    // NSAccessibilityCustomActions: parallel arrays of machine names (passed
    // back to Go on perform) and human-readable titles (shown by VoiceOver).
    const char** customNames;
    const char** customTitles;
    int customCount;
    // Grid coordinates for table cells (0-based); -1 when not a cell.
    int cellRow, cellCol;
    // Full virtual extent for tables (AXRowCount/AXColumnCount); 0 unknown.
    int gridRows, gridCols;
    // 0-based AXIndex for synthetic table rows; -1 when not applicable.
    int axIndex;
    // AXSelected: -1 not selectable, 0 not selected, 1 selected. A 0→1 flip
    // posts AXSelectedCellsChanged on the enclosing table (#574).
    int selected;
} AccessibilityElementProps;

AccessibilityElementRef AccessibilityElementCreate(uintptr_t handle, void* nsWindow);
// Parents a cell realized on demand for an off-viewport table coordinate
// (fyne#637) to the table element for AX navigation; it is not one of the
// table's synthetic-row children. Called after AccessibilityElementUpdate.
void AccessibilityRealizedElementConfigure(AccessibilityElementRef elem, AccessibilityElementRef table);
bool AccessibilityElementUpdate(AccessibilityElementRef elem, AccessibilityElementProps* props);
bool AccessibilityElementSetChildren(AccessibilityElementRef elem, AccessibilityElementRef* children, int count);
void AccessibilityElementDestroy(AccessibilityElementRef elem);

bool AccessibilityViewSetRoots(void* nsWindow, AccessibilityElementRef* roots, int count);
void AccessibilityViewClearRoots(void* nsWindow);
void AccessibilityViewNotifyLayoutChanged(void* nsWindow);

// Posts the per-edit text notifications in the order native AppKit text
// views emit them (selection first, then value), which is what VoiceOver's
// own edit classifier expects.
void AccessibilityElementPostTextChanged(AccessibilityElementRef elem, bool selChanged, bool valueChanged);

// Posts an AX announcement so a running screen reader speaks the text
// without the user navigating to it (live region, #525). assertive maps to
// high priority (interrupts current speech), polite to medium.
void AccessibilityAnnounce(const char* text, bool assertive);

#endif // BELLUI_ACCESSIBILITY_DARWIN_H
