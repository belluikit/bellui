//go:build accessibility && darwin && !wasm

package glfw

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit

#include <stdlib.h>
#include "accessibility_darwin.h"
*/
import "C"

import (
	"sort"
	"unsafe"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
)

// a11yElement pairs the native NSAccessibilityElement with the stable handle
// the ObjC bridge uses to call back into Go for actions and focus.
type a11yElement struct {
	ref    C.AccessibilityElementRef
	handle uintptr
}

type a11yState struct {
	elements map[core.CanvasObject]a11yElement
	// virtual holds the elements for nodes keyed by a stable NodeID rather than
	// a CanvasObject: nodes with no object of their own (RichText headings,
	// #533) and realized nodes of a virtualized collection whose object is
	// recycled across records (AccessibleIdentified, #635).
	virtual map[any]a11yElement
	// tableRows holds the bridge-synthesized AXRow elements grouping a
	// table's realized cells (#574), keyed by table object then 0-based row
	// index, so a row keeps its element across virtualization scrolls.
	tableRows map[core.CanvasObject]map[int]a11yElement
	// prevText/prevSel remember each text element's last published text and
	// selection so edits and caret moves can be classified for VoiceOver's
	// notifications. Holding the string is cheap: the entry's rope memo
	// keeps the same string object between edits, so this is one string
	// header per text element, not a copy.
	prevText map[core.CanvasObject]string
	prevSel  map[core.CanvasObject][2]int
	// realized holds cells built on demand for VoiceOver navigating a
	// virtualized table past its viewport (fyne#637), keyed by the cell's
	// stable NodeID. They are delivered outside the reconcile walk (via
	// accessibilityCellForColumn:row:), so the per-frame sweep never touches
	// them; they are dropped by FIFO eviction past a cap, when their table is
	// destroyed, or promoted into virtual when the cell scrolls into view.
	realized      map[any]realizedCell
	realizedOrder []any // FIFO of realized NodeIDs, oldest first, for eviction
}

// realizedCell pairs an on-demand cell element with the handle of the table it
// belongs to, so the cell can be dropped when its table is destroyed and its
// bring-into-view request routed back to the table.
type realizedCell struct {
	el          a11yElement
	tableHandle uintptr
}

// a11yTarget is what an AX action resolves against: the window the element
// belongs to and its node snapshot from the most recent tree update.
type a11yTarget struct {
	win  *window
	node *accessibility.Node
}

// All accessibility state is owned by the main goroutine: updates run from
// the draw loop, and AX callbacks arrive on the main thread while it sits in
// glfw.WaitEventsTimeout — the same re-entry model as the GLFW input
// callbacks — so no locking is needed.
var (
	a11yWindows    = map[*window]*a11yState{}
	a11yTargets    = map[uintptr]a11yTarget{}
	a11yNextHandle uintptr
)

func init() {
	// Live-region announcements (#525): deliver accessibility.Announce calls
	// as NSAccessibilityAnnouncementRequested notifications so VoiceOver
	// speaks transient status (Toast, Snackbar) without moving its cursor.
	// Announce runs on the main goroutine per its contract, which under this
	// driver is the main thread the AX API requires.
	accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.AccessibilityAnnounce(ctext, C.bool(assertive))
	})
}

// updateAccessibility synchronises the NSAccessibility element tree for this
// window with the platform-neutral semantic tree (#484). Elements are keyed
// by canvas object identity and reconciled in place: an unchanged widget
// keeps its NSAccessibilityElement across frames, so VoiceOver's cursor is
// not disturbed by unrelated repaints, and AX notifications fire only for
// properties that actually changed.
func (w *window) updateAccessibility() {
	view := w.view()
	if view == nil {
		return
	}
	nsWindow := view.GetCocoaWindow()
	if nsWindow == nil {
		return
	}

	st := a11yWindows[w]
	if st == nil {
		st = &a11yState{
			elements:  make(map[core.CanvasObject]a11yElement),
			virtual:   make(map[any]a11yElement),
			tableRows: make(map[core.CanvasObject]map[int]a11yElement),
			prevText:  make(map[core.CanvasObject]string),
			prevSel:   make(map[core.CanvasObject][2]int),
			realized:  make(map[any]realizedCell),
		}
		a11yWindows[w] = st
	}

	// Only the key window's focused widget reports AXFocused (#554): each
	// bellui canvas remembers its own focused widget across window switches,
	// but macOS convention is that exactly one element — the key window's
	// first responder — is focused. processFocused triggers an update on
	// key-window changes, so the property diff re-posts the focus
	// notification when this window becomes key again.
	var focusedObj core.CanvasObject
	if curWindow == w {
		if f := w.canvas.Focused(); f != nil {
			focusedObj, _ = f.(core.CanvasObject)
		}
	}

	// A focusable container may delegate AT focus to an internal-cursor
	// descendant (a grid cell, tree/list row) by stable identity (#688); when
	// it does, AXFocused (and AXFocusedUIElement) resolves to that descendant
	// rather than the container.
	activeDescendant := a11yActiveDescendant(focusedObj)

	forest := accessibility.CanvasNodes(w.canvas, w.canvas.menu)

	seen := make(map[core.CanvasObject]bool, len(st.elements))
	seenVirtual := make(map[any]bool, len(st.virtual))
	seenRows := make(map[core.CanvasObject]map[int]bool)
	rc := &a11yReconcile{
		st: st, nsWindow: nsWindow, focusedObj: focusedObj, activeDescendant: activeDescendant,
		seen: seen, seenVirtual: seenVirtual, seenRows: seenRows,
	}
	changed := false
	var roots []C.AccessibilityElementRef
	for _, n := range forest {
		if ref, ok := w.reconcileA11yNode(rc, n, &changed); ok {
			roots = append(roots, ref)
		}
	}

	var rootsPtr *C.AccessibilityElementRef
	if len(roots) > 0 {
		rootsPtr = &roots[0]
	}
	if C.AccessibilityViewSetRoots(nsWindow, rootsPtr, C.int(len(roots))) {
		changed = true
	}

	for obj, el := range st.elements {
		if seen[obj] {
			continue
		}
		C.AccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
		delete(st.elements, obj)
		delete(st.prevText, obj)
		delete(st.prevSel, obj)
		changed = true
	}
	for id, el := range st.virtual {
		if seenVirtual[id] {
			continue
		}
		C.AccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
		delete(st.virtual, id)
		changed = true
	}
	for table, rows := range st.tableRows {
		for idx, el := range rows {
			if seenRows[table][idx] {
				continue
			}
			C.AccessibilityElementDestroy(el.ref)
			delete(rows, idx)
			changed = true
		}
		if len(rows) == 0 {
			delete(st.tableRows, table)
		}
	}
	// Drop on-demand realized cells whose table was destroyed this frame (its
	// handle is gone from the target registry): they parent directly to the
	// table element, so they must not outlive it. Cells whose table survives
	// persist until FIFO eviction or promotion.
	for id, rc := range st.realized {
		if _, ok := a11yTargets[rc.tableHandle]; ok {
			continue
		}
		C.AccessibilityElementDestroy(rc.el.ref)
		delete(a11yTargets, rc.el.handle)
		st.realizedOrder = accessibility.DropRealizedID(st.realizedOrder, id)
		delete(st.realized, id)
		changed = true
	}

	if changed {
		C.AccessibilityViewNotifyLayoutChanged(nsWindow)
	}
}

// cleanupAccessibilityForWindow releases all native elements for a window
// that is being destroyed. It must run while the GLFW viewport is still
// alive so the roots attached to the content view can be detached first.
func (w *window) cleanupAccessibilityForWindow() {
	st := a11yWindows[w]
	if st == nil {
		return
	}

	if view := w.viewport; view != nil {
		if nsWindow := view.GetCocoaWindow(); nsWindow != nil {
			C.AccessibilityViewClearRoots(nsWindow)
		}
	}
	for _, el := range st.elements {
		C.AccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
	}
	for _, el := range st.virtual {
		C.AccessibilityElementDestroy(el.ref)
		delete(a11yTargets, el.handle)
	}
	for _, rows := range st.tableRows {
		for _, el := range rows {
			C.AccessibilityElementDestroy(el.ref)
		}
	}
	for _, rc := range st.realized {
		C.AccessibilityElementDestroy(rc.el.ref)
		delete(a11yTargets, rc.el.handle)
	}
	delete(a11yWindows, w)
}

// reconcileA11yNode creates or updates the native element for one semantic
// node and recurses into its children. ok is false when the node (and its
// subtree) is culled because it lies entirely outside its clip bounds, e.g.
// a list row scrolled out of view. The keyboard-focused object is exempt
// from culling (#501): destroying its element while the user scrolls it out
// of view would drop the screen reader's cursor to the window.
// a11yReconcile bundles the per-update state threaded through the tree
// reconcile: the window state maps, the native window, the focused object
// and the seen sets used for pruning.
type a11yReconcile struct {
	st         *a11yState
	nsWindow   unsafe.Pointer
	focusedObj core.CanvasObject
	// activeDescendant is the stable identity the focused container delegates
	// AT focus to, or nil when focus stays on focusedObj itself (#688).
	activeDescendant any
	seen             map[core.CanvasObject]bool
	seenVirtual      map[any]bool
	seenRows         map[core.CanvasObject]map[int]bool
}

func (w *window) reconcileA11yNode(rc *a11yReconcile, n *accessibility.Node, changed *bool) (ref C.AccessibilityElementRef, ok bool) {
	focused := a11yNodeFocused(n, rc.focusedObj, rc.activeDescendant)
	if n.HasClip && !n.Size.IsZero() && !rectsOverlap(geo.Rectangle{Pos: n.Pos, Size: n.Size}, n.Clip) {
		// The focused element (the container, or the active descendant the
		// cursor scrolled to) is exempt from culling: destroying its element
		// would drop VoiceOver's cursor to the window.
		if !focused {
			return nil, false
		}
	}

	var el a11yElement
	var exists bool
	switch {
	case n.NodeID != nil:
		// Nodes with a stable identity reconcile against it rather than their
		// CanvasObject: virtual nodes (RichText headings, #533) that have no
		// object, and realized nodes of a virtualized collection whose object
		// is recycled across records (AccessibleIdentified, #635). Keying by
		// identity keeps VoiceOver's cursor on the record across sort/scroll.
		el, exists = rc.st.virtual[n.NodeID]
		if !exists {
			if realized, ok := rc.st.realized[n.NodeID]; ok {
				// This off-viewport cell scrolled into view: reuse its on-demand
				// element so VoiceOver's cursor and the AX element identity
				// survive the realize→visible transition. It leaves the realized
				// cache and reconciles normally from here (it becomes a real
				// cell grouped into a synthetic row).
				el, exists = realized.el, true
				rc.st.realizedOrder = accessibility.DropRealizedID(rc.st.realizedOrder, n.NodeID)
				delete(rc.st.realized, n.NodeID)
			}
		}
	case n.Object != nil:
		el, exists = rc.st.elements[n.Object]
	default:
		return nil, false
	}
	if !exists {
		a11yNextHandle++
		el = a11yElement{
			ref:    C.AccessibilityElementCreate(C.uintptr_t(a11yNextHandle), rc.nsWindow),
			handle: a11yNextHandle,
		}
		*changed = true
	}
	if n.NodeID != nil {
		rc.st.virtual[n.NodeID] = el
		rc.seenVirtual[n.NodeID] = true
	} else {
		rc.st.elements[n.Object] = el
		rc.seen[n.Object] = true
	}
	a11yTargets[el.handle] = a11yTarget{win: w, node: n}

	if w.applyA11yProps(el.ref, n, focused) {
		*changed = true
	}
	if tx, isTexter := n.Object.(core.AccessibleTexter); isTexter {
		w.postA11yTextEdit(rc.st, el.ref, n.Object, tx)
	}

	var childRefs []C.AccessibilityElementRef
	if n.Role == core.RoleTable || n.Role == core.RoleGrid {
		// macOS derives table structure from AXRow children — bare cells
		// read as an empty table (#574). Group the realized cells into
		// bridge-synthesized row elements.
		childRefs = w.reconcileTableChildren(rc, n, changed)
	} else {
		for _, c := range n.Children {
			if childRef, keep := w.reconcileA11yNode(rc, c, changed); keep {
				childRefs = append(childRefs, childRef)
			}
		}
	}
	var childPtr *C.AccessibilityElementRef
	if len(childRefs) > 0 {
		childPtr = &childRefs[0]
	}
	if C.AccessibilityElementSetChildren(el.ref, childPtr, C.int(len(childRefs))) {
		*changed = true
	}

	return el.ref, true
}

// reconcileTableChildren reconciles a table's children, grouping cell nodes
// by their 0-based row coordinate into synthetic AXRow elements (#574). The
// row elements are keyed by (table, row index) so they stay stable while
// virtualization realizes and releases cells; non-cell children pass
// through ungrouped after the rows.
func (w *window) reconcileTableChildren(rc *a11yReconcile, table *accessibility.Node, changed *bool) []C.AccessibilityElementRef {
	type rowGroup struct {
		cells  []C.AccessibilityElementRef
		bounds geo.Rectangle
	}
	groups := map[int]*rowGroup{}
	var rowOrder []int
	var loose []C.AccessibilityElementRef

	for _, c := range table.Children {
		ref, keep := w.reconcileA11yNode(rc, c, changed)
		if !keep {
			continue
		}
		if !c.HasCellPosition {
			loose = append(loose, ref)
			continue
		}
		g := groups[c.CellRow]
		if g == nil {
			g = &rowGroup{bounds: geo.Rectangle{Pos: c.Pos, Size: c.Size}}
			groups[c.CellRow] = g
			rowOrder = append(rowOrder, c.CellRow)
		} else {
			g.bounds = unionRect(g.bounds, geo.Rectangle{Pos: c.Pos, Size: c.Size})
		}
		g.cells = append(g.cells, ref)
	}
	sort.Ints(rowOrder)

	rows := rc.st.tableRows[table.Object]
	if rows == nil {
		rows = make(map[int]a11yElement)
		rc.st.tableRows[table.Object] = rows
	}
	seen := rc.seenRows[table.Object]
	if seen == nil {
		seen = make(map[int]bool)
		rc.seenRows[table.Object] = seen
	}

	refs := make([]C.AccessibilityElementRef, 0, len(rowOrder)+len(loose))
	for _, idx := range rowOrder {
		g := groups[idx]
		el, exists := rows[idx]
		if !exists {
			a11yNextHandle++
			el = a11yElement{
				ref:    C.AccessibilityElementCreate(C.uintptr_t(a11yNextHandle), rc.nsWindow),
				handle: a11yNextHandle,
			}
			rows[idx] = el
			*changed = true
		}
		seen[idx] = true
		if w.applyA11yRowProps(el.ref, idx, g.bounds) {
			*changed = true
		}
		var cellPtr *C.AccessibilityElementRef
		if len(g.cells) > 0 {
			cellPtr = &g.cells[0]
		}
		if C.AccessibilityElementSetChildren(el.ref, cellPtr, C.int(len(g.cells))) {
			*changed = true
		}
		refs = append(refs, el.ref)
	}
	return append(refs, loose...)
}

// applyA11yRowProps updates a synthetic table-row element: an AXRow at the
// union of its cells' bounds, carrying the 0-based row index as AXIndex.
func (w *window) applyA11yRowProps(ref C.AccessibilityElementRef, rowIdx int, bounds geo.Rectangle) bool {
	sc := float64(w.canvas.Scale())
	empty := C.CString("")
	defer C.free(unsafe.Pointer(empty))
	props := C.AccessibilityElementProps{
		role:      C.int(C.BelluiAXRoleRow),
		label:     empty,
		help:      empty,
		value:     empty,
		x:         C.double(float64(bounds.Pos.X) * sc),
		y:         C.double(float64(bounds.Pos.Y) * sc),
		width:     C.double(float64(bounds.Size.Width) * sc),
		height:    C.double(float64(bounds.Size.Height) * sc),
		enabled:   C.bool(true),
		checked:   C.int(-1),
		subrole:   C.int(C.BelluiAXSubroleNone),
		disclosed: C.int(-1),
		cellRow:   C.int(-1),
		cellCol:   C.int(-1),
		axIndex:   C.int(rowIdx),
		selected:  C.int(-1),
	}
	return bool(C.AccessibilityElementUpdate(ref, &props))
}

func unionRect(a, b geo.Rectangle) geo.Rectangle {
	x1 := min(a.Pos.X, b.Pos.X)
	y1 := min(a.Pos.Y, b.Pos.Y)
	x2 := max(a.Pos.X+a.Size.Width, b.Pos.X+b.Size.Width)
	y2 := max(a.Pos.Y+a.Size.Height, b.Pos.Y+b.Size.Height)
	return geo.Rectangle{Pos: geo.NewPos(x1, y1), Size: geo.NewSize(x2-x1, y2-y1)}
}

func (w *window) applyA11yProps(ref C.AccessibilityElementRef, n *accessibility.Node, focused bool) bool {
	// dp → window points; on macOS the content view works in points, the
	// retina backing scale never enters window coordinates.
	sc := float64(w.canvas.Scale())

	label := C.CString(n.Label)
	help := C.CString(n.Description)
	value := C.CString(n.Value)
	defer C.free(unsafe.Pointer(label))
	defer C.free(unsafe.Pointer(help))
	defer C.free(unsafe.Pointer(value))

	_, tappable := n.Object.(event.Tappable)
	props := C.AccessibilityElementProps{
		role:         C.int(a11yRole(n.Role)),
		label:        label,
		help:         help,
		value:        value,
		x:            C.double(float64(n.Pos.X) * sc),
		y:            C.double(float64(n.Pos.Y) * sc),
		width:        C.double(float64(n.Size.Width) * sc),
		height:       C.double(float64(n.Size.Height) * sc),
		enabled:      C.bool(!n.State.Has(core.StateDisabled)),
		focused:      C.bool(focused),
		pressable:    C.bool(a11yPressAction(n) != "" || tappable),
		checked:      C.int(a11yChecked(n)),
		hasRange:     C.bool(n.HasRange),
		rangeMin:     C.double(n.RangeMin),
		rangeMax:     C.double(n.RangeMax),
		rangeValue:   C.double(n.RangeCurrent),
		canIncrement: C.bool(a11yHasAction(n, core.ActionIncrement)),
		canDecrement: C.bool(a11yHasAction(n, core.ActionDecrement)),
		subrole:      C.int(a11ySubrole(n)),
		level:        C.int(n.Level),
		disclosed:    C.int(a11yDisclosed(n)),
		cellRow:      C.int(-1),
		cellCol:      C.int(-1),
		axIndex:      C.int(-1),
		selected:     C.int(a11ySelected(n)),
	}
	if n.HasCellPosition {
		props.cellRow = C.int(n.CellRow)
		props.cellCol = C.int(n.CellCol)
	}
	if n.HasGridDims {
		props.gridRows = C.int(n.GridRows)
		props.gridCols = C.int(n.GridCols)
	}
	if tx, isTexter := n.Object.(core.AccessibleTexter); isTexter {
		props.textElement = C.bool(true)
		// Rune offsets, purely for the ObjC-side diff that posts
		// AXSelectedTextChanged — deliberately not converted to UTF-16 here,
		// which would materialize the text on every frame.
		selStart, selEnd := tx.AccessibleSelection()
		props.selStart = C.long(selStart)
		props.selEnd = C.long(selEnd)
	}

	custom := a11yCustomActions(n)
	if len(custom) > 0 {
		// The name/title arrays must live in C memory: props is passed to a
		// cgo call, and a struct argument must not contain pointers into the
		// Go heap ("Go pointer to unpinned Go pointer" panic).
		ptrSize := C.size_t(unsafe.Sizeof((*C.char)(nil)))
		names := (**C.char)(C.malloc(ptrSize * C.size_t(len(custom))))
		titles := (**C.char)(C.malloc(ptrSize * C.size_t(len(custom))))
		nameSlice := unsafe.Slice(names, len(custom))
		titleSlice := unsafe.Slice(titles, len(custom))
		for i, act := range custom {
			title := act.Description
			if title == "" {
				title = act.Name
			}
			nameSlice[i] = C.CString(act.Name)
			titleSlice[i] = C.CString(title)
		}
		defer func() {
			for i := range custom {
				C.free(unsafe.Pointer(nameSlice[i]))
				C.free(unsafe.Pointer(titleSlice[i]))
			}
			C.free(unsafe.Pointer(names))
			C.free(unsafe.Pointer(titles))
		}()
		props.customNames = names
		props.customTitles = titles
		props.customCount = C.int(len(custom))
	}

	return bool(C.AccessibilityElementUpdate(ref, &props))
}

// postA11yTextEdit posts the per-edit text notifications for a text
// element, replicating native AppKit fields exactly: plain
// AXSelectedTextChanged then AXValueChanged (VoiceOver diffs the text
// itself and classifies the edit from the caret movement — order matters,
// and userInfo payloads are not part of the native protocol). A pure caret
// move posts only the selection notification.
func (w *window) postA11yTextEdit(st *a11yState, ref C.AccessibilityElementRef, obj core.CanvasObject, tx core.AccessibleTexter) {
	cur := tx.AccessibleText()
	selStart, selEnd := tx.AccessibleSelection()
	sel := [2]int{selStart, selEnd}

	prev, seenBefore := st.prevText[obj]
	prevSel, selSeen := st.prevSel[obj]
	st.prevText[obj] = cur
	st.prevSel[obj] = sel
	if !seenBefore || !selSeen {
		return
	}

	valueChanged := prev != cur
	selChanged := sel != prevSel
	if !valueChanged && !selChanged {
		return
	}
	C.AccessibilityElementPostTextChanged(ref, C.bool(selChanged || valueChanged), C.bool(valueChanged))
}

// a11yRole maps the platform-neutral role vocabulary onto the NSAccessibility
// role subset the bridge understands. Roles without a good AppKit equivalent
// fall back to AXGroup, which VoiceOver navigates through transparently.
func a11yRole(r core.AccessibleRole) C.BelluiAXRole {
	switch r {
	case core.RoleButton:
		return C.BelluiAXRoleButton
	case core.RoleCheckbox, core.RoleSwitch:
		return C.BelluiAXRoleCheckBox
	case core.RoleRadioButton, core.RoleTab: // macOS tabs are AXRadioButton in an AXTabGroup
		return C.BelluiAXRoleRadioButton
	case core.RoleRadioGroup:
		return C.BelluiAXRoleRadioGroup
	case core.RoleTextBox, core.RoleSearchField, core.RolePasswordField:
		return C.BelluiAXRoleTextField
	case core.RoleMultiLineTextBox:
		return C.BelluiAXRoleTextArea
	case core.RoleComboBox:
		return C.BelluiAXRolePopUpButton
	case core.RoleSlider:
		return C.BelluiAXRoleSlider
	case core.RoleSpinButton:
		return C.BelluiAXRoleIncrementor
	case core.RoleProgressBar:
		return C.BelluiAXRoleProgressIndicator
	case core.RoleLabel, core.RoleStatus, core.RoleTooltip, core.RoleBadge:
		return C.BelluiAXRoleStaticText
	case core.RoleHeading:
		// The WebKit convention VoiceOver's heading rotor understands:
		// role AXHeading, AXValue = level, text in the title.
		return C.BelluiAXRoleHeading
	case core.RoleListItem:
		return C.BelluiAXRoleRow
	case core.RoleLink:
		return C.BelluiAXRoleLink
	case core.RoleImage:
		return C.BelluiAXRoleImage
	case core.RoleList:
		return C.BelluiAXRoleList
	case core.RoleTree:
		return C.BelluiAXRoleOutline
	case core.RoleTreeItem:
		return C.BelluiAXRoleRow
	case core.RoleTable, core.RoleGrid:
		return C.BelluiAXRoleTable
	case core.RoleTableRow:
		return C.BelluiAXRoleRow
	case core.RoleTableCell:
		return C.BelluiAXRoleCell
	case core.RoleTabList:
		return C.BelluiAXRoleTabGroup
	case core.RoleMenu:
		return C.BelluiAXRoleMenu
	case core.RoleMenuItem:
		return C.BelluiAXRoleMenuItem
	case core.RoleMenuBar:
		return C.BelluiAXRoleMenuBar
	case core.RoleToolbar:
		return C.BelluiAXRoleToolbar
	case core.RoleScrollBar:
		return C.BelluiAXRoleScrollBar
	default:
		return C.BelluiAXRoleGroup
	}
}

// a11ySubrole picks the NSAccessibility subrole refining the mapped role:
// secure text fields for password entries, content-list for lists, outline
// rows for tree items.
func a11ySubrole(n *accessibility.Node) C.BelluiAXSubrole {
	switch n.Role {
	case core.RolePasswordField:
		return C.BelluiAXSubroleSecureTextField
	case core.RoleList:
		return C.BelluiAXSubroleContentList
	case core.RoleTreeItem:
		return C.BelluiAXSubroleOutlineRow
	}
	return C.BelluiAXSubroleNone
}

// a11yChecked returns the AXValue for checkable roles:
// -1 not checkable, 0 off, 1 on, 2 mixed.
func a11yChecked(n *accessibility.Node) int {
	switch n.Role {
	case core.RoleCheckbox, core.RoleSwitch, core.RoleRadioButton, core.RoleTab:
		if n.State.Has(core.StatePartial) {
			return 2
		}
		if n.State.Has(core.StateChecked) || n.State.Has(core.StateSelected) {
			return 1
		}
		return 0
	}
	return -1
}

// a11yCustomActions returns the node's actions that no standard AX action
// consumes, in declaration order: the single action AXPress maps to and the
// increment/decrement pair are excluded, everything else (expand, collapse,
// rename, delete, and any future names) is offered through
// NSAccessibilityCustomActions so VoiceOver lists it in the actions rotor.
func a11yCustomActions(n *accessibility.Node) []core.AccessibleAction {
	press := a11yPressAction(n)
	var custom []core.AccessibleAction
	for _, act := range n.Actions {
		switch act.Name {
		case press, core.ActionIncrement, core.ActionDecrement:
			continue
		}
		custom = append(custom, act)
	}
	return custom
}

// belluiAccessibilityPerformPress is called by the ObjC bridge when assistive
// technology performs AXPress on an element. It arrives on the main thread
// inside glfw.WaitEventsTimeout, i.e. on the main goroutine — the same
// dispatch context as GLFW input callbacks — so widget code is called
// directly.
//
//export belluiAccessibilityPerformPress
func belluiAccessibilityPerformPress(handle C.uintptr_t) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}

	obj := t.node.Object
	if obj == nil {
		// An on-demand realized cell has no widget to activate; pressing it
		// (VO-space on an off-screen cell) brings it into view instead, after
		// which the real host handles selection (fyne#637).
		return C.bool(t.win.scrollRealizedIntoView(t))
	}
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

// belluiAccessibilityPerformAction is called by the ObjC bridge for the
// non-press AX actions: AXIncrement/AXDecrement and the custom actions built
// from the node's declared action list (expand, collapse, rename, delete,
// ...). Same dispatch context as PerformPress: main thread, main goroutine.
//
//export belluiAccessibilityPerformAction
func belluiAccessibilityPerformAction(handle C.uintptr_t, name *C.char) C.bool {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return C.bool(false)
	}

	goName := C.GoString(name)
	if !a11yHasAction(t.node, goName) {
		return C.bool(false)
	}
	actioner, ok := t.node.Object.(core.AccessibleActioner)
	if !ok {
		return C.bool(false)
	}
	actioner.PerformAccessibleAction(goName)
	return C.bool(true)
}

// belluiAccessibilitySetFocus is called by the ObjC bridge when assistive
// technology focuses an element; it moves the canvas keyboard focus when the
// target widget is focusable. Main-thread/main-goroutine, like PerformPress.
//
//export belluiAccessibilitySetFocus
func belluiAccessibilitySetFocus(handle C.uintptr_t) {
	t, ok := a11yTargets[uintptr(handle)]
	if !ok {
		return
	}
	if f, ok := t.node.Object.(event.Focusable); ok {
		t.win.canvas.Focus(f)
	}
}

// belluiAccessibilityRealizeCell is called by the ObjC bridge from a table's
// accessibilityCellForColumn:row: when the requested cell is not a realized
// (visible) one: it builds an on-demand element for the off-viewport cell
// through the neutral realize seam (fyne#637) so VoiceOver can read past the
// viewport of a virtualized table. It returns the element (nil when the table
// is not a realizer or the coordinates are out of range), keeps it in the
// per-window realized cache keyed by the cell's stable identity so repeat reads
// return the same element, and parents it to the table for AX navigation. Main
// goroutine, like the other AX callbacks.
//
//export belluiAccessibilityRealizeCell
func belluiAccessibilityRealizeCell(tableHandle C.uintptr_t, row, col C.int) C.AccessibilityElementRef {
	t, ok := a11yTargets[uintptr(tableHandle)]
	if !ok {
		return nil
	}
	st := a11yWindows[t.win]
	if st == nil {
		return nil
	}
	node, ok := accessibility.RealizeCell(t.node, int(row), int(col))
	if !ok || node.NodeID == nil {
		return nil
	}

	// Repeat request for the same cell: refresh its properties and return the
	// existing element so its AX identity is stable across reads.
	if rc, ok := st.realized[node.NodeID]; ok {
		a11yTargets[rc.el.handle] = a11yTarget{win: t.win, node: node}
		t.win.applyA11yProps(rc.el.ref, node, false)
		return rc.el.ref
	}

	nsWindow := t.win.a11yNSWindow()
	tableRef := st.refForNode(t.node)
	if nsWindow == nil || tableRef == nil {
		return nil
	}

	a11yNextHandle++
	el := a11yElement{
		ref:    C.AccessibilityElementCreate(C.uintptr_t(a11yNextHandle), nsWindow),
		handle: a11yNextHandle,
	}
	a11yTargets[el.handle] = a11yTarget{win: t.win, node: node}
	// A realized cell is delivered outside the reconcile walk and must never be
	// culled (focused=false).
	t.win.applyA11yProps(el.ref, node, false)
	C.AccessibilityRealizedElementConfigure(el.ref, tableRef)

	st.realized[node.NodeID] = realizedCell{el: el, tableHandle: uintptr(tableHandle)}
	st.realizedOrder = append(st.realizedOrder, node.NodeID)
	evictRealized(st)
	return el.ref
}

// scrollRealizedIntoView routes an on-demand cell's bring-into-view request back
// to its table: the cell node carries the display (row, col) and the realized
// cache maps it to the table whose AccessibleScrollCellIntoView the seam calls.
func (w *window) scrollRealizedIntoView(t a11yTarget) bool {
	st := a11yWindows[t.win]
	if st == nil || t.node.NodeID == nil {
		return false
	}
	rc, ok := st.realized[t.node.NodeID]
	if !ok {
		return false
	}
	tt, ok := a11yTargets[rc.tableHandle]
	if !ok {
		return false
	}
	return accessibility.ScrollCellIntoView(tt.node, t.node.CellRow, t.node.CellCol)
}

// a11yNSWindow returns the window's Cocoa NSWindow pointer, or nil when the
// viewport is not available.
func (w *window) a11yNSWindow() unsafe.Pointer {
	view := w.view()
	if view == nil {
		return nil
	}
	return view.GetCocoaWindow()
}

// refForNode returns the native element backing a reconciled node, keyed the
// same way reconcileA11yNode stores it (identity first, then object). Realized
// cells use it to find their table's element to parent against.
func (st *a11yState) refForNode(n *accessibility.Node) C.AccessibilityElementRef {
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
			continue // already dropped by promotion or table teardown
		}
		C.AccessibilityElementDestroy(rc.el.ref)
		delete(a11yTargets, rc.el.handle)
		delete(st.realized, id)
	}
}
