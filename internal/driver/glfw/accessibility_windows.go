//go:build accessibility && windows && !wasm

package glfw

/*
#cgo LDFLAGS: -lole32 -loleaut32 -luiautomationcore -luuid

#include <stdlib.h>
#include "accessibility_windows.h"
*/
import "C"

import (
	"math"
	"unsafe"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
)

// a11yElement pairs the native UIA provider with the stable handle the C
// bridge uses to call back into Go for actions, focus and text queries.
type a11yElement struct {
	ref    C.WinAccessibilityElementRef
	handle uintptr
}

type a11yState struct {
	elements map[core.CanvasObject]a11yElement
	// virtual holds the elements for nodes keyed by a stable NodeID rather than
	// a CanvasObject: nodes with no object (RichText headings, #533) and
	// realized nodes of a virtualized collection whose object is recycled
	// across records (AccessibleIdentified, #635).
	virtual map[any]a11yElement
	// realized holds cells built on demand for a screen reader that has
	// navigated off the viewport (fyne#637), keyed by the cell's stable NodeID.
	// They are delivered outside the reconcile walk — the walker never emits an
	// off-viewport cell — so the per-frame seenVirtual sweep never touches them;
	// they are dropped by FIFO eviction past a cap, when their grid is
	// destroyed, or promoted into virtual when the cell scrolls into view.
	realized      map[any]realizedCell
	realizedOrder []any // FIFO of realized NodeIDs, oldest first, for eviction
}

// realizedCell pairs an on-demand cell provider with the handle of the grid it
// belongs to, so the cell can be dropped when its grid is destroyed and its
// bring-into-view request routed back to the grid.
type realizedCell struct {
	el         a11yElement
	gridHandle uintptr
}

// a11yTarget is what a UIA callback resolves against: the window the element
// belongs to and its node snapshot from the most recent tree update.
type a11yTarget struct {
	win  *window
	node *accessibility.Node
}

// All accessibility state is owned by the main goroutine: updates run from
// the draw loop, and UIA client calls are marshalled onto the main thread's
// STA message pump inside glfw.WaitEventsTimeout — the same re-entry model
// as the GLFW input callbacks — so no locking is needed.
var (
	a11yWindows    = map[*window]*a11yState{}
	a11yTargets    = map[uintptr]a11yTarget{}
	a11yNextHandle uintptr
)

func init() {
	// Live-region announcements (#525): deliver accessibility.Announce calls
	// as UIA notification events so Narrator/NVDA speak transient status
	// (Toast, Snackbar) without moving their cursor. Announce runs on the
	// main goroutine per its contract, which under this driver is the thread
	// the providers live on.
	accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.WinAccessibilityAnnounce(ctext, C.bool(assertive))
	})
}

// updateAccessibility synchronises the UIA provider tree for this window
// with the platform-neutral semantic tree (#484). Elements are keyed by
// canvas object identity and reconciled in place: an unchanged widget keeps
// its provider across frames, so a screen reader's cursor is not disturbed
// by unrelated repaints, and UIA events fire only for properties that
// actually changed.
func (w *window) updateAccessibility() {
	view := w.view()
	if view == nil {
		return
	}
	hwnd := unsafe.Pointer(view.GetWin32Window())
	if hwnd == nil {
		return
	}

	st := a11yWindows[w]
	if st == nil {
		st = &a11yState{
			elements: make(map[core.CanvasObject]a11yElement),
			virtual:  make(map[any]a11yElement),
			realized: make(map[any]realizedCell),
		}
		a11yWindows[w] = st
	}

	// Only the foreground window's focused widget reports keyboard focus
	// (#554): each bellui canvas remembers its own focused widget across
	// window switches, but exactly one element system-wide should report
	// HasKeyboardFocus. processFocused triggers an update on focus changes,
	// so the property diff re-raises the focus event when this window
	// becomes foreground again.
	var focusedObj core.CanvasObject
	if curWindow == w {
		if f := w.canvas.Focused(); f != nil {
			focusedObj, _ = f.(core.CanvasObject)
		}
	}

	// A focusable container may delegate AT focus to an internal-cursor
	// descendant (a grid cell, tree/list row) by stable identity (#688); when
	// it does, focus resolves to that descendant rather than the container.
	activeDescendant := a11yActiveDescendant(focusedObj)

	forest := accessibility.CanvasNodes(w.canvas, w.canvas.menu)

	seen := make(map[core.CanvasObject]bool, len(st.elements))
	seenVirtual := make(map[any]bool, len(st.virtual))
	var roots []C.WinAccessibilityElementRef
	for _, n := range forest {
		if ref, ok := w.reconcileA11yNode(st, n, hwnd, focusedObj, activeDescendant, seen, seenVirtual); ok {
			roots = append(roots, ref)
		}
	}

	var rootsPtr *C.WinAccessibilityElementRef
	if len(roots) > 0 {
		rootsPtr = &roots[0]
	}
	var focusedRef C.WinAccessibilityElementRef
	if activeDescendant != nil {
		// The container designated a descendant by identity; its element lives
		// in the identity-keyed map (the #635 virtualized/recycled path). The
		// SetRoots diff raises UIA_AutomationFocusChangedEventId when this ref
		// changes, so a cursor move to a different cell announces for free.
		if el, ok := st.virtual[activeDescendant]; ok {
			focusedRef = el.ref
		}
	} else if focusedObj != nil {
		if el, ok := st.elements[focusedObj]; ok {
			focusedRef = el.ref
		} else if id, ok := focusedObj.(core.AccessibleIdentified); ok {
			// A focused object of a virtualized collection is keyed by its
			// stable identity, not its (recycled) pointer (#635).
			if key := id.AccessibleIdentity(); key != nil {
				if el, ok := st.virtual[key]; ok {
					focusedRef = el.ref
				}
			}
		}
	}
	C.WinAccessibilityWindowSetRoots(hwnd, rootsPtr, C.int(len(roots)), focusedRef)

	for obj, el := range st.elements {
		if seen[obj] {
			continue
		}
		C.WinAccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
		delete(st.elements, obj)
	}
	for id, el := range st.virtual {
		if seenVirtual[id] {
			continue
		}
		C.WinAccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
		delete(st.virtual, id)
	}
	// Drop on-demand realized cells whose grid was destroyed this frame (its
	// handle is gone from the target registry): they parent directly to the
	// grid element, so they must not outlive it. Cells whose grid survives
	// persist until FIFO eviction or promotion.
	for id, rc := range st.realized {
		if _, ok := a11yTargets[rc.gridHandle]; ok {
			continue
		}
		C.WinAccessibilityElementDestroy(rc.el.ref)
		delete(a11yTargets, rc.el.handle)
		st.realizedOrder = accessibility.DropRealizedID(st.realizedOrder, id)
		delete(st.realized, id)
	}
}

// cleanupAccessibilityForWindow releases all native providers for a window
// that is being destroyed. It must run while the GLFW viewport is still
// alive so the window subclass can be removed and the fragment root
// disconnected first.
func (w *window) cleanupAccessibilityForWindow() {
	st := a11yWindows[w]
	if st == nil {
		return
	}

	if view := w.viewport; view != nil {
		if hwnd := unsafe.Pointer(view.GetWin32Window()); hwnd != nil {
			C.WinAccessibilityWindowCleanup(hwnd)
		}
	}
	for _, el := range st.elements {
		C.WinAccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
	}
	for _, el := range st.virtual {
		C.WinAccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
	}
	for _, rc := range st.realized {
		C.WinAccessibilityElementDestroy(rc.el.ref)
		delete(a11yTargets, rc.el.handle)
	}
	delete(a11yWindows, w)
}

// reconcileA11yNode creates or updates the native provider for one semantic
// node and recurses into its children. ok is false when the node (and its
// subtree) is culled because it lies entirely outside its clip bounds, e.g.
// a list row scrolled out of view. The keyboard-focused object is exempt
// from culling: destroying its provider while the user scrolls it out of
// view would drop the screen reader's cursor to the window; it reports
// IsOffscreen instead.
func (w *window) reconcileA11yNode(st *a11yState, n *accessibility.Node, hwnd unsafe.Pointer,
	focusedObj core.CanvasObject, activeDescendant any, seen map[core.CanvasObject]bool, seenVirtual map[any]bool,
) (ref C.WinAccessibilityElementRef, ok bool) {
	focused := a11yNodeFocused(n, focusedObj, activeDescendant)
	culled := n.HasClip && !n.Size.IsZero() && !rectsOverlap(geo.Rectangle{Pos: n.Pos, Size: n.Size}, n.Clip)
	// The focused element (the container, or the active descendant the cursor
	// scrolled to) is exempt from culling — destroying its provider would drop
	// the screen reader's cursor to the window; it reports IsOffscreen instead.
	if culled && !focused {
		return nil, false
	}

	var el a11yElement
	var exists bool
	switch {
	case n.NodeID != nil:
		// Nodes with a stable identity reconcile against it rather than their
		// CanvasObject: virtual nodes with no object (#533) and realized nodes
		// of a virtualized collection whose object is recycled across records
		// (AccessibleIdentified, #635), so the UIA element tracks the record.
		el, exists = st.virtual[n.NodeID]
		if !exists {
			if rc, ok := st.realized[n.NodeID]; ok {
				// This off-viewport cell scrolled into view: reuse its on-demand
				// provider so the screen reader's cursor and the UIA element
				// identity survive the realize→visible transition. It leaves the
				// realized cache and reconciles normally from here (it becomes a
				// real grid child via SetChildren).
				el, exists = rc.el, true
				st.realizedOrder = accessibility.DropRealizedID(st.realizedOrder, n.NodeID)
				delete(st.realized, n.NodeID)
			}
		}
	case n.Object != nil:
		el, exists = st.elements[n.Object]
	default:
		return nil, false
	}
	if !exists {
		a11yNextHandle++
		el = a11yElement{
			ref:    C.WinAccessibilityElementCreate(C.uintptr_t(a11yNextHandle), hwnd),
			handle: a11yNextHandle,
		}
	}
	if n.NodeID != nil {
		st.virtual[n.NodeID] = el
		seenVirtual[n.NodeID] = true
	} else {
		st.elements[n.Object] = el
		seen[n.Object] = true
	}
	a11yTargets[el.handle] = a11yTarget{win: w, node: n}

	w.applyWinA11yProps(el.ref, n, focused, culled)

	var childRefs []C.WinAccessibilityElementRef
	for _, c := range n.Children {
		if childRef, keep := w.reconcileA11yNode(st, c, hwnd, focusedObj, activeDescendant, seen, seenVirtual); keep {
			childRefs = append(childRefs, childRef)
		}
	}
	var childPtr *C.WinAccessibilityElementRef
	if len(childRefs) > 0 {
		childPtr = &childRefs[0]
	}
	C.WinAccessibilityElementSetChildren(el.ref, childPtr, C.int(len(childRefs)))

	return el.ref, true
}

func (w *window) applyWinA11yProps(ref C.WinAccessibilityElementRef, n *accessibility.Node,
	focused, culled bool,
) {
	// dp → device pixels; the Windows client area works in physical pixels,
	// and the canvas scale already folds in the monitor DPI.
	sc := float64(w.canvas.Scale())

	name := C.CString(n.Label)
	help := C.CString(n.Description)
	defer C.free(unsafe.Pointer(name))
	defer C.free(unsafe.Pointer(help))

	controlType := a11yControlType(n.Role)
	_, tappable := n.Object.(event.Tappable)
	_, focusable := n.Object.(event.Focusable)
	_, texter := n.Object.(core.AccessibleTexter)
	toggleState := a11yToggleState(n)

	props := C.WinAccessibilityElementProps{
		controlType: C.int(controlType),
		name:        name,
		help:        help,
		x:           C.double(float64(n.Pos.X) * sc),
		y:           C.double(float64(n.Pos.Y) * sc),
		width:       C.double(float64(n.Size.Width) * sc),
		height:      C.double(float64(n.Size.Height) * sc),
		enabled:     C.bool(!n.State.Has(core.StateDisabled)),
		focusable:   C.bool(focusable),
		focused:     C.bool(focused),
		offscreen:   C.bool(culled),
		isPassword:  C.bool(n.Role == core.RolePasswordField),
		isDialog:    C.bool(n.Role == core.RoleDialog || n.Role == core.RoleAlertDialog || n.State.Has(core.StateModal)),
		structural:  C.bool(controlType == uiaControlGroup && n.Label == "" && len(n.Children) > 0),
		// Toggles activate through the Toggle pattern and text widgets edit
		// through the Value/Text patterns; a tap on those is caret placement,
		// not an action, so neither offers Invoke.
		invokable:   C.bool(toggleState < 0 && !texter && (a11yPressAction(n) != "" || tappable)),
		toggleState: C.int(toggleState),
		expandState: C.int(a11yDisclosed(n)),
		selectable:  C.int(a11ySelectable(n)),
		hasRange:    C.bool(n.HasRange),
		rangeMin:    C.double(n.RangeMin),
		rangeMax:    C.double(n.RangeMax),
		rangeValue:  C.double(n.RangeCurrent),
		rangeSettable: C.bool(a11yHasAction(n, core.ActionIncrement) ||
			a11yHasAction(n, core.ActionDecrement)),
		level: C.int(n.Level),
	}

	if n.Role == core.RoleHeading {
		props.headingLevel = C.int(min(max(n.Level, 1), 9))
	}
	if n.HasGridDims {
		props.hasGridDims = C.bool(true)
		props.gridRows = C.int(n.GridRows)
		props.gridCols = C.int(n.GridCols)
	}
	if n.HasCellPosition {
		props.hasCell = C.bool(true)
		props.cellRow = C.int(n.CellRow)
		props.cellCol = C.int(n.CellCol)
	}

	if tx, isTexter := n.Object.(core.AccessibleTexter); isTexter {
		props.textElement = C.bool(true)
		props.readOnly = C.bool(n.State.Has(core.StateReadOnly) || n.State.Has(core.StateDisabled))
		// Rune offsets plus a content-version stamp, purely for the C-side
		// diff that raises the text-changed/selection-changed events —
		// clients re-query the real content through the callbacks, so the
		// text itself is never snapshotted per frame.
		selStart, selEnd := tx.AccessibleSelection()
		props.selStart = C.long(selStart)
		props.selEnd = C.long(selEnd)
		props.textVersion = a11yTextVersion(n, tx)
	} else {
		// Non-text values (a slider's "42%", a date picker's date) travel as
		// a snapshot; text widgets resolve their Value pattern live instead.
		value := C.CString(n.Value)
		defer C.free(unsafe.Pointer(value))
		props.value = value
	}

	C.WinAccessibilityElementUpdate(ref, &props)
}

// a11yTextVersion derives a cheap content-version stamp for a text element.
// For regular widgets the rope behind Entry memoizes its string and returns
// the same object until the next splice (the docs/notes/text.md identity contract),
// so the string data pointer changes exactly when the content does. Password
// fields rebuild their mask on every call, so the pointer is useless there;
// the mask length is the best O(1) proxy (its content is uniform anyway).
func a11yTextVersion(n *accessibility.Node, tx core.AccessibleTexter) C.long {
	text := tx.AccessibleText()
	if n.Role == core.RolePasswordField {
		return C.long(len(text))
	}
	return C.long(uintptr(unsafe.Pointer(unsafe.StringData(text))))
}

// UIA control type ids (UIAutomationClient.h); only the ones the role map
// below produces.
const (
	uiaControlButton      = 50000
	uiaControlCheckBox    = 50002
	uiaControlComboBox    = 50003
	uiaControlEdit        = 50004
	uiaControlHyperlink   = 50005
	uiaControlImage       = 50006
	uiaControlListItem    = 50007
	uiaControlList        = 50008
	uiaControlMenu        = 50009
	uiaControlMenuBar     = 50010
	uiaControlMenuItem    = 50011
	uiaControlProgressBar = 50012
	uiaControlRadioButton = 50013
	uiaControlScrollBar   = 50014
	uiaControlSlider      = 50015
	uiaControlSpinner     = 50016
	uiaControlTab         = 50018
	uiaControlTabItem     = 50019
	uiaControlText        = 50020
	uiaControlToolBar     = 50021
	uiaControlTree        = 50023
	uiaControlTreeItem    = 50024
	uiaControlGroup       = 50026
	uiaControlDataItem    = 50029
	uiaControlPane        = 50033
	uiaControlTable       = 50036
	uiaControlSeparator   = 50038
)

// a11yControlType maps the platform-neutral role vocabulary onto UIA control
// types. Roles without a good UIA equivalent fall back to Group, which
// screen readers navigate through transparently.
func a11yControlType(r core.AccessibleRole) int {
	switch r {
	case core.RoleButton:
		return uiaControlButton
	case core.RoleCheckbox, core.RoleSwitch:
		return uiaControlCheckBox
	case core.RoleRadioButton:
		return uiaControlRadioButton
	case core.RoleTextBox, core.RoleSearchField, core.RolePasswordField, core.RoleMultiLineTextBox:
		return uiaControlEdit
	case core.RoleComboBox:
		return uiaControlComboBox
	case core.RoleSlider:
		return uiaControlSlider
	case core.RoleSpinButton:
		return uiaControlSpinner
	case core.RoleProgressBar:
		return uiaControlProgressBar
	case core.RoleLabel, core.RoleStatus, core.RoleTooltip, core.RoleBadge, core.RoleHeading:
		// Headings are Text with UIA_HeadingLevelPropertyId set, the
		// convention Narrator's heading navigation keys off.
		return uiaControlText
	case core.RoleLink:
		return uiaControlHyperlink
	case core.RoleImage:
		return uiaControlImage
	case core.RoleList:
		return uiaControlList
	case core.RoleListItem:
		return uiaControlListItem
	case core.RoleTree:
		return uiaControlTree
	case core.RoleTreeItem:
		return uiaControlTreeItem
	case core.RoleTable, core.RoleGrid:
		return uiaControlTable
	case core.RoleTableCell:
		return uiaControlDataItem
	case core.RoleTab:
		return uiaControlTabItem
	case core.RoleTabList:
		return uiaControlTab
	case core.RoleTabPanel, core.RoleDialog, core.RoleAlertDialog:
		return uiaControlPane
	case core.RoleMenu:
		return uiaControlMenu
	case core.RoleMenuItem:
		return uiaControlMenuItem
	case core.RoleMenuBar:
		return uiaControlMenuBar
	case core.RoleToolbar:
		return uiaControlToolBar
	case core.RoleScrollBar:
		return uiaControlScrollBar
	case core.RoleSeparator:
		return uiaControlSeparator
	default:
		return uiaControlGroup
	}
}

// a11yToggleState returns the UIA ToggleState for checkable roles:
// -1 no Toggle pattern, 0 off, 1 on, 2 indeterminate. Radio buttons and
// tabs express their state through the SelectionItem pattern instead (the
// UIA convention), unlike the darwin bridge where they are AXRadioButtons
// with a checked value.
func a11yToggleState(n *accessibility.Node) int {
	switch n.Role {
	case core.RoleCheckbox, core.RoleSwitch:
		if n.State.Has(core.StatePartial) {
			return 2
		}
		if n.State.Has(core.StateChecked) {
			return 1
		}
		return 0
	}
	return -1
}

// a11ySelectable reports SelectionItem pattern support: -1 none, else the
// current IsSelected as 0/1. One-of-N items get the pattern from their role;
// anything else that declares a "select" action (table cells, day cells)
// gets it too.
func a11ySelectable(n *accessibility.Node) int {
	switch n.Role {
	case core.RoleRadioButton, core.RoleTab, core.RoleListItem, core.RoleTreeItem:
	default:
		if !a11yHasAction(n, core.ActionSelect) {
			return -1
		}
	}
	if n.State.Has(core.StateSelected) || n.State.Has(core.StateChecked) {
		return 1
	}
	return 0
}

// belluiWinA11yInvoke is called by the C bridge when a UIA client performs the
// Invoke pattern on an element. It arrives on the main thread inside
// glfw.WaitEventsTimeout, i.e. on the main goroutine — the same dispatch
// context as the GLFW input callbacks — so widget code is called directly.
//
//export belluiWinA11yInvoke
func belluiWinA11yInvoke(handle C.uintptr_t) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}

	obj := t.node.Object
	if name := a11yPressAction(t.node); name != "" {
		if actioner, ok := obj.(core.AccessibleActioner); ok {
			actioner.PerformAccessibleAction(name)
			core.NotifyActivation(obj, false)
			return C.bool(true)
		}
	}
	if tap, ok := obj.(event.Tappable); ok {
		center := geo.NewPos(t.node.Size.Width/2, t.node.Size.Height/2)
		tap.Tapped(&event.PointEvent{
			AbsolutePosition: n2AbsCenter(t.node),
			Position:         center,
		})
		core.NotifyActivation(obj, false)
		return C.bool(true)
	}
	return C.bool(false)
}

// belluiWinA11yToggle handles the Toggle pattern: the declared "toggle" action
// when present, otherwise the same press routing as Invoke.
//
//export belluiWinA11yToggle
func belluiWinA11yToggle(handle C.uintptr_t) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}
	if actioner, isActioner := t.node.Object.(core.AccessibleActioner); isActioner && a11yHasAction(t.node, core.ActionToggle) {
		actioner.PerformAccessibleAction(core.ActionToggle)
		core.NotifyActivation(t.node.Object, false)
		return C.bool(true)
	}
	return belluiWinA11yInvoke(handle)
}

// belluiWinA11ySelect handles SelectionItem.Select: the declared "select"
// action when present, otherwise a synthesized tap.
//
//export belluiWinA11ySelect
func belluiWinA11ySelect(handle C.uintptr_t) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}
	if actioner, isActioner := t.node.Object.(core.AccessibleActioner); isActioner && a11yHasAction(t.node, core.ActionSelect) {
		actioner.PerformAccessibleAction(core.ActionSelect)
		core.NotifyActivation(t.node.Object, false)
		return C.bool(true)
	}
	return belluiWinA11yInvoke(handle)
}

// belluiWinA11yExpandCollapse handles the ExpandCollapse pattern via the
// node's declared expand/collapse actions.
//
//export belluiWinA11yExpandCollapse
func belluiWinA11yExpandCollapse(handle C.uintptr_t, expand C.int) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}
	name := core.ActionCollapse
	if expand != 0 {
		name = core.ActionExpand
	}
	actioner, isActioner := t.node.Object.(core.AccessibleActioner)
	if !isActioner || !a11yHasAction(t.node, name) {
		// Already in the requested state: report success so clients treat
		// the call as idempotent.
		return C.bool(a11yDisclosed(t.node) == 1 == (expand != 0))
	}
	actioner.PerformAccessibleAction(name)
	return C.bool(true)
}

// belluiWinA11yRangeSetValue implements RangeValue.SetValue on top of the
// neutral increment/decrement actions: the widget owns its step size, so the
// bridge steps toward the requested value until it is reached, crossed, or
// stops making progress (whichever comes first).
//
//export belluiWinA11yRangeSetValue
func belluiWinA11yRangeSetValue(handle C.uintptr_t, value C.double) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}
	valuer, isValuer := t.node.Object.(core.AccessibleValuer)
	actioner, isActioner := t.node.Object.(core.AccessibleActioner)
	if !isValuer || !isActioner {
		return C.bool(false)
	}

	target := float64(value)
	const maxSteps = 4096
	for i := 0; i < maxSteps; i++ {
		_, _, cur := valuer.AccessibleRange()
		diff := target - cur
		if diff == 0 {
			return C.bool(true)
		}
		name := core.ActionIncrement
		if diff < 0 {
			name = core.ActionDecrement
		}
		if !a11yHasAction(t.node, name) {
			return C.bool(i > 0)
		}
		actioner.PerformAccessibleAction(name)
		_, _, next := valuer.AccessibleRange()
		if math.Abs(target-next) >= math.Abs(diff) {
			// No progress (bound reached) or stepped past the closest
			// representable value.
			return C.bool(true)
		}
	}
	return C.bool(true)
}

// belluiWinA11ySetFocus is called by the C bridge when a UIA client focuses an
// element; it moves the canvas keyboard focus when the target widget is
// focusable. Main-thread/main-goroutine, like Invoke.
//
//export belluiWinA11ySetFocus
func belluiWinA11ySetFocus(handle C.uintptr_t) {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return
	}
	if f, ok := t.node.Object.(event.Focusable); ok {
		t.win.canvas.Focus(f)
	}
}

// belluiWinA11yRealizeCell is called by the C bridge from IGridProvider::GetItem
// when the requested cell is not a realized (visible) host: it builds an
// on-demand provider for the off-viewport cell through the neutral realize seam
// (fyne#637) so a screen reader can read past the viewport of a virtualized
// grid. It returns the element (NULL when the container is not a realizer or the
// coordinates are out of range), keeps it in the per-window realized cache keyed
// by the cell's stable identity so repeat reads return the same provider, and
// parents it to the grid element for fragment navigation. Main goroutine, like
// the other UIA callbacks.
//
//export belluiWinA11yRealizeCell
func belluiWinA11yRealizeCell(gridHandle C.uintptr_t, row, col C.int) C.WinAccessibilityElementRef {
	gt, ok := a11yTargets[uintptr(gridHandle)]
	if !ok {
		return nil
	}
	st := a11yWindows[gt.win]
	if st == nil {
		return nil
	}
	node, ok := accessibility.RealizeCell(gt.node, int(row), int(col))
	if !ok || node.NodeID == nil {
		return nil
	}

	// Repeat request for the same cell: refresh its properties and return the
	// existing provider so its UIA runtime id is stable across reads.
	if rc, ok := st.realized[node.NodeID]; ok {
		a11yTargets[rc.el.handle] = a11yTarget{win: gt.win, node: node}
		gt.win.applyWinA11yProps(rc.el.ref, node, false, false)
		return rc.el.ref
	}

	hwnd := gt.win.a11yHWND()
	gridRef := st.refForNode(gt.node)
	if hwnd == nil || gridRef == nil {
		return nil
	}

	a11yNextHandle++
	el := a11yElement{
		ref:    C.WinAccessibilityElementCreate(C.uintptr_t(a11yNextHandle), hwnd),
		handle: a11yNextHandle,
	}
	a11yTargets[el.handle] = a11yTarget{win: gt.win, node: node}
	// A realized cell is delivered outside the reconcile walk and must never be
	// culled (focused=false, culled=false).
	gt.win.applyWinA11yProps(el.ref, node, false, false)
	C.WinAccessibilityRealizedElementConfigure(el.ref, gridRef)

	st.realized[node.NodeID] = realizedCell{el: el, gridHandle: uintptr(gridHandle)}
	st.realizedOrder = append(st.realizedOrder, node.NodeID)
	evictRealized(st)
	return el.ref
}

// belluiWinA11yRealize is called by the C bridge from IVirtualizedItemProvider::
// Realize on an on-demand cell: it scrolls the cell into the viewport through
// the neutral seam so it materializes as a real host. Reading a cell has no
// side effect; only this explicit realize scrolls (fyne#637).
//
//export belluiWinA11yRealize
func belluiWinA11yRealize(handle C.uintptr_t) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}
	return C.bool(t.win.scrollRealizedIntoView(t))
}

// scrollRealizedIntoView routes an on-demand cell's bring-into-view request back
// to its grid: the cell node carries the display (row, col) and the realized
// cache maps it to the grid whose AccessibleScrollCellIntoView the seam calls.
func (w *window) scrollRealizedIntoView(t a11yTarget) bool {
	st := a11yWindows[t.win]
	if st == nil || t.node.NodeID == nil {
		return false
	}
	rc, ok := st.realized[t.node.NodeID]
	if !ok {
		return false
	}
	gt, ok := a11yTargets[rc.gridHandle]
	if !ok {
		return false
	}
	return accessibility.ScrollCellIntoView(gt.node, t.node.CellRow, t.node.CellCol)
}

// a11yHWND returns the window's Win32 HWND as an unsafe pointer, or nil when the
// viewport is not available.
func (w *window) a11yHWND() unsafe.Pointer {
	view := w.view()
	if view == nil {
		return nil
	}
	return unsafe.Pointer(view.GetWin32Window())
}

// refForNode returns the native element backing a reconciled node, keyed the
// same way reconcileA11yNode stores it (identity first, then object). Realized
// cells use it to find their grid's provider to parent against.
func (st *a11yState) refForNode(n *accessibility.Node) C.WinAccessibilityElementRef {
	if n.NodeID != nil {
		if el, ok := st.virtual[n.NodeID]; ok {
			return el.ref
		}
	}
	if n.Object != nil {
		if el, ok := st.elements[n.Object]; ok {
			return el.ref
		}
	}
	return nil
}

// evictRealized drops the oldest realized cells once the cache exceeds its cap.
func evictRealized(st *a11yState) {
	keep, evict := accessibility.OverflowRealized(st.realizedOrder, accessibility.RealizedCap)
	st.realizedOrder = keep
	for _, id := range evict {
		rc, ok := st.realized[id]
		if !ok {
			continue // already dropped by promotion or grid teardown
		}
		C.WinAccessibilityElementDestroy(rc.el.ref)
		delete(a11yTargets, rc.el.handle)
		delete(st.realized, id)
	}
}
