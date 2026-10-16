//go:build accessibility && ios

package mobile

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UIKit -framework CoreGraphics

#include <stdlib.h>
#include "accessibility_ios.h"
*/
import "C"

import (
	"slices"
	"unsafe"

	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/lang"
)

func init() {
	// Live-region announcements (pre-rebrand tracker #525): deliver accessibility.Announce calls
	// as UIAccessibilityAnnouncementNotification so VoiceOver speaks
	// transient status (Toast, Snackbar) without moving its cursor.
	accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		var flag C.int
		if assertive {
			flag = 1
		}
		C.BelluiA11yAnnounce(ctext, flag)
	})
}

// The last state pushed to the ObjC side, used to elide bridge calls for
// unchanged elements. The tree is published unconditionally — there is no
// public "any assistive technology running" check on iOS (Voice Control and
// the Accessibility Inspector run without the VoiceOver flag) — so steady
// frames must cost no ObjC dispatch. Main-goroutine only, like the gather.
var (
	iosA11yLastSent  = map[uintptr]mobileA11yNode{}
	iosA11yLastOrder []uintptr
	// iosA11yLastFocusText tracks which text element held the canvas focus
	// last frame: the ObjC side swaps the real input proxy in for it (pre-rebrand tracker #570),
	// so a focus change must reach BelluiA11ySetElements even when the element
	// order is unchanged.
	iosA11yLastFocusText uintptr
)

func iosA11yIsTextRole(role int) bool {
	switch role {
	case mobileRoleTextField, mobileRoleTextArea, mobileRolePassword, mobileRoleSearch:
		return true
	}
	return false
}

// updateAccessibility publishes the current semantic tree to UIKit as a
// flat, reading-ordered UIAccessibilityElement list on the root view.
// Elements are keyed by stable handles and reconciled in place, so an
// unchanged widget keeps its element across frames and VoiceOver's cursor
// survives unrelated repaints. Runs on the main goroutine from the paint
// loop; the ObjC side forwards all UIKit work to the main dispatch queue.
func (w *window) updateAccessibility() {
	nodes, removed := w.mobileA11yGather()

	if len(removed) > 0 {
		C.BelluiA11yDestroyElements((*C.uintptr_t)(unsafe.Pointer(&removed[0])), C.int(len(removed)))
		for _, h := range removed {
			delete(iosA11yLastSent, h)
		}
	}

	handles := make([]uintptr, 0, len(nodes))
	var focusText uintptr
	for i := range nodes {
		n := &nodes[i]
		if !mobileA11yNodesEqual(iosA11yLastSent[n.handle], *n) {
			w.applyA11yProps(n)
			iosA11yLastSent[n.handle] = *n
		}
		if focusText == 0 && n.flags&mobileA11yFlagFocused != 0 && iosA11yIsTextRole(n.role) {
			focusText = n.handle
		}
		handles = append(handles, n.handle)
	}

	if slices.Equal(handles, iosA11yLastOrder) && focusText == iosA11yLastFocusText {
		return // same membership, order and editing target: nothing to do
	}
	iosA11yLastOrder = handles
	iosA11yLastFocusText = focusText

	var handlePtr *C.uintptr_t
	if len(handles) > 0 {
		handlePtr = (*C.uintptr_t)(unsafe.Pointer(&handles[0]))
	}
	C.BelluiA11ySetElements(handlePtr, C.int(len(handles)))
}

func (w *window) applyA11yProps(n *mobileA11yNode) {
	value := n.value
	if value == "" && n.flags&mobileA11yFlagCheckable != 0 {
		// UIAccessibilityElement has no public toggle trait that speaks
		// on/off, so surface the state as the value like web checkboxes do.
		if n.flags&mobileA11yFlagChecked != 0 {
			value = lang.L("checked")
		} else {
			value = lang.L("not checked")
		}
	}

	label := C.CString(n.label)
	cvalue := C.CString(value)
	hint := C.CString(n.hint)
	customActs := C.CString(a11yEncodeCustomActs(n.customActs))
	textLayout := C.CString(n.textLayout)
	defer C.free(unsafe.Pointer(label))
	defer C.free(unsafe.Pointer(cvalue))
	defer C.free(unsafe.Pointer(hint))
	defer C.free(unsafe.Pointer(customActs))
	defer C.free(unsafe.Pointer(textLayout))

	C.BelluiA11yUpdateElement(C.uintptr_t(n.handle), C.BelluiA11yProps{
		role:       C.int(n.role),
		flags:      C.int(n.flags),
		label:      label,
		value:      cvalue,
		hint:       hint,
		customActs: customActs,
		textLayout: textLayout,
		x:          C.float(n.x),
		y:          C.float(n.y),
		w:          C.float(n.w),
		h:          C.float(n.h),
		gridHandle: C.uintptr_t(n.gridHandle),
		cellRow:    C.long(n.cellRow),
		cellCol:    C.long(n.cellCol),
		rowCount:   C.long(n.rowCount),
		colCount:   C.long(n.colCount),
	})
}

func (w *window) initAccessibilityForWindow() {
	// Nothing to set up: the ObjC side resolves the container view lazily.
}

func (w *window) cleanupAccessibilityForWindow() {
	mobileA11yForgetWindow(w)
	iosA11yLastSent = map[uintptr]mobileA11yNode{}
	iosA11yLastOrder = nil
	iosA11yLastFocusText = 0
	C.BelluiA11yClearAll()
}

// belluiMobileA11yAction is called by the ObjC bridge when VoiceOver performs
// an action on an element. It arrives on the UIKit main thread — under
// gomobile not the main goroutine — so the dispatch is queued.
//
//export belluiMobileA11yAction
func belluiMobileA11yAction(handle C.uintptr_t, action C.int) {
	mobileA11yDispatch(uintptr(handle), int(action))
}

// belluiMobileA11yRealizeCell is called by the ObjC bridge from a grid element's
// accessibilityDataTableCellElementForRow:column: when the requested cell is
// not among the visible (walked) ones: it realizes the off-viewport cell
// through the neutral seam (pre-rebrand tracker #817, the iOS analog of the desktop bridges'
// GridPattern.GetItem / accessibilityCellForColumn:row:). It fills out with the
// cell's wire properties — the string fields are Go-allocated C strings the
// caller must free() — and returns the cell's element handle, or 0 when the
// grid is unknown, the coordinates are out of range, or the main goroutine did
// not answer within the bounded wait (the UIKit main thread must never block
// indefinitely on the main goroutine, which itself sometimes blocks on the main
// queue). Evicted older realized cells are destroyed here as a side effect.
//
//export belluiMobileA11yRealizeCell
func belluiMobileA11yRealizeCell(grid C.uintptr_t, row, col C.long, out *C.BelluiA11yProps) C.uintptr_t {
	node, evicted, ok := mobileA11yRealizeCellWait(uintptr(grid), int(row), int(col))
	if len(evicted) > 0 {
		// Evicted handles are always still-realized cells: a promoted cell left
		// the cache when it entered the walk, so no iosA11yLastSent entry (a
		// main-goroutine-only map) ever needs cleaning from this OS thread.
		C.BelluiA11yDestroyElements((*C.uintptr_t)(unsafe.Pointer(&evicted[0])), C.int(len(evicted)))
	}
	if !ok || out == nil {
		return 0
	}
	*out = C.BelluiA11yProps{
		role:       C.int(node.role),
		flags:      C.int(node.flags),
		label:      C.CString(node.label),
		value:      C.CString(node.value),
		hint:       C.CString(node.hint),
		customActs: C.CString(a11yEncodeCustomActs(node.customActs)),
		// Grid cells never carry the editable-text sidecar, but the ObjC realize
		// path frees every string field, so provide an allocation to free.
		textLayout: C.CString(""),
		x:          C.float(node.x),
		y:          C.float(node.y),
		w:          C.float(node.w),
		h:          C.float(node.h),
		gridHandle: C.uintptr_t(node.gridHandle),
		cellRow:    C.long(node.cellRow),
		cellCol:    C.long(node.cellCol),
		rowCount:   C.long(node.rowCount),
		colCount:   C.long(node.colCount),
	}
	return C.uintptr_t(node.handle)
}

// belluiMobileA11ySetFocus is called by the ObjC bridge when VoiceOver focus
// enters or leaves an element. It drives the in-scene focus ring (pre-rebrand tracker #759/#798):
// VoiceOver's own focus rectangle has nothing real to outline over Bellui's flat
// GPU (Metal) surface — the elements are zero-frame proxies — so Bellui paints its
// own ring over the focused element, the iOS mirror of the Android
// accessibilityFocusChanged hook. It arrives on the UIKit main thread (not the
// main goroutine); mobileA11ySetFocus stores the handle atomically and queues the
// repaint.
//
//export belluiMobileA11ySetFocus
func belluiMobileA11ySetFocus(handle C.uintptr_t, focused C.int) {
	mobileA11ySetFocus(uintptr(handle), focused != 0)
}
