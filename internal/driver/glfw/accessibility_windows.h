//go:build accessibility && windows && !wasm && !test_web_driver

#ifndef BELLUI_ACCESSIBILITY_WINDOWS_H
#define BELLUI_ACCESSIBILITY_WINDOWS_H

#include <stdbool.h>
#include <stdint.h>

// Opaque reference to a BelluiUIAElement (a UI Automation provider defined in
// accessibility_windows.c).
typedef void* WinAccessibilityElementRef;

// Property snapshot for one element. The bridge diffs against the element's
// current state and raises UIA events only for actual changes; the return
// value of WinAccessibilityElementUpdate reports whether anything changed.
typedef struct {
    int controlType; // UIA_*ControlTypeId, mapped from core.AccessibleRole in Go
    const char* name;
    const char* help;
    const char* value; // textual value, "" when none (text elements resolve live instead)
    // Local frame relative to the window client area, in device pixels,
    // top-left origin. Screen conversion happens lazily in
    // get_BoundingRectangle so window moves need no tree update.
    double x, y, width, height;
    bool enabled;
    bool focusable;
    bool focused;
    // The element is kept alive despite being scrolled outside its clip
    // bounds (the keyboard-focused widget); reported via IsOffscreen.
    bool offscreen;
    bool isPassword; // password fields: UIA_IsPasswordPropertyId
    bool isDialog;   // modal dialogs: UIA_IsDialogPropertyId
    // Pure structure (an unlabeled group): excluded from the control and
    // content views so screen readers walk through to the real controls.
    bool structural;
    bool invokable;   // Invoke pattern (press)
    int  toggleState; // -1 no Toggle pattern, else ToggleState_Off/On/Indeterminate
    int  expandState; // -1 no ExpandCollapse pattern, 0 collapsed, 1 expanded
    int  selectable;  // -1 no SelectionItem pattern, else 0/1 = IsSelected
    bool hasRange;    // RangeValue pattern
    double rangeMin, rangeMax, rangeValue;
    bool rangeSettable; // the node declares increment/decrement actions
    // The element answers the UIA Text pattern via the belluiWinA11yText*
    // callbacks; it also gets a live-resolved Value pattern.
    bool textElement;
    bool readOnly; // Value pattern IsReadOnly
    // Selection/caret in rune offsets plus a cheap content-version stamp
    // (string identity on the Go side), used only to diff between updates so
    // the bridge raises the text-selection/text-changed events (clients
    // re-query the real content through the callbacks).
    long selStart, selEnd, textVersion;
    int level;        // 1-based hierarchy level (tree items); 0 when not applicable
    int headingLevel; // 1..9 for headings, 0 otherwise
    bool hasGridDims; // Grid pattern on table/grid containers
    int gridRows, gridCols;
    bool hasCell; // GridItem pattern on realized cells
    int cellRow, cellCol;
} WinAccessibilityElementProps;

WinAccessibilityElementRef WinAccessibilityElementCreate(uintptr_t handle, void* hwnd);
bool WinAccessibilityElementUpdate(WinAccessibilityElementRef elem, WinAccessibilityElementProps* props);
// Configures an element realized on demand for an off-viewport cell (pre-rebrand tracker #637):
// parents it to the grid element for fragment navigation and exposes the
// VirtualizedItem pattern. Called after WinAccessibilityElementUpdate.
void WinAccessibilityRealizedElementConfigure(WinAccessibilityElementRef elem, WinAccessibilityElementRef grid);
bool WinAccessibilityElementSetChildren(WinAccessibilityElementRef elem, WinAccessibilityElementRef* children, int count);
void WinAccessibilityElementDestroy(WinAccessibilityElementRef elem);

// Attaches the forest roots (and the keyboard-focused element, or NULL) to
// the window's fragment root, installing the WM_GETOBJECT subclass on first
// use. Raises the UIA focus-changed event when the focused element changes.
bool WinAccessibilityWindowSetRoots(void* hwnd, WinAccessibilityElementRef* roots, int count,
    WinAccessibilityElementRef focused);
void WinAccessibilityWindowCleanup(void* hwnd);

// Raises a UIA notification event so a running screen reader speaks the text
// without the user navigating to it (live region, pre-rebrand tracker #525). assertive maps to
// NotificationProcessing_ImportantAll (interrupts current speech), polite to
// NotificationProcessing_All.
void WinAccessibilityAnnounce(const char* text, bool assertive);

#endif // BELLUI_ACCESSIBILITY_WINDOWS_H
