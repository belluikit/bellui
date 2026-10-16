//go:build accessibility && ios

// VoiceOver bridge for the mobile driver (pre-rebrand tracker #488). The Go side calls these
// from the main goroutine (not the UIKit main thread); every UIKit mutation
// is forwarded to the main dispatch queue, which also serialises access to
// the element registry. Strings are copied before the calling stack returns.

#include <stdint.h>

typedef struct {
	int role;  // mobileRole* wire values (accessibility.go)
	int flags; // mobileA11yFlag* wire values
	const char *label;
	const char *value;
	const char *hint;
	// Bounds in native pixels within the root view; converted to points on
	// the ObjC side and to screen coordinates lazily per accessibilityFrame
	// read.
	float x, y, w, h;
	// Virtualized-grid association (pre-rebrand tracker #817): the handle of the enclosing
	// BELLUI_ROLE_GRID container (0 when none), the element's display grid
	// coordinates (-1 when not a grid item), and — for the grid container
	// itself — the collection's full extent.
	uintptr_t gridHandle;
	long cellRow, cellCol;
	long rowCount, colCount;
	// Custom accessibility actions (pre-rebrand tracker #1027): declared widget actions with no
	// standard iOS semantic (undo/redo, ...), encoded as goId U+001F label
	// entries separated by U+001E ("" = none) — the same wire form the
	// Android bridge parses. Offered in VoiceOver's actions rotor; performing
	// one dispatches kind BELLUI_A11Y_ACTION_CUSTOM + goId.
	const char *customActs;
	// Visual-layout sidecar of the FOCUSED editable node (pre-rebrand tracker #995), "" otherwise:
	// the L:/C: wire (mobileA11yTextLayout) carrying per-character rectangles in
	// native pixels (canvas space). Consumed on iOS (pre-rebrand tracker #1107) to answer VoiceOver's
	// per-line/caret geometry from bellui's real layout — the swapped-in input
	// proxy's own UITextView metrics do not match bellui's shaping, so its native
	// rects put the line ring between lines.
	const char *textLayout;
} BelluiA11yProps;

// BelluiA11yUpdateElement creates the element for handle on first use and
// applies props to it.
void BelluiA11yUpdateElement(uintptr_t handle, BelluiA11yProps props);

// BelluiA11ySetElements publishes the ordered element list for the current
// frame on the root view, posting a layout-changed notification only when
// membership or order actually changed.
void BelluiA11ySetElements(uintptr_t *handles, int count);

// BelluiA11yDestroyElements releases the elements for handles that left the
// tree.
void BelluiA11yDestroyElements(uintptr_t *handles, int count);

// BelluiA11yClearAll releases every element and detaches the list from the
// root view (window teardown).
void BelluiA11yClearAll(void);

// BelluiA11yAnnounce speaks transient status text; assertive interrupts the
// current utterance, polite queues after it.
void BelluiA11yAnnounce(const char *text, int assertive);
