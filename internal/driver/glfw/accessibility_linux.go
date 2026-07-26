//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"strconv"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/scale"
)

// This file wires the bellui glfw window lifecycle into the pure-Go AT-SPI2
// provider (atspi_linux.go): it reconciles the platform-neutral semantic tree
// (internal/accessibility) into the exported D-Bus object tree once per dirty
// frame, exactly as the darwin/windows backends do, and registers the
// live-region announcement listener (#525).

var (
	atspiOnce sync.Once
	atspiInst *atspiProvider
)

// ensureAtspi lazily brings up the process-wide AT-SPI provider on the first
// update. It returns nil (and stays nil) when no a11y bus is available, so the
// backend degrades to a no-op without probing the bus every frame.
func ensureAtspi() *atspiProvider {
	atspiOnce.Do(func() { atspiInst = newAtspiProvider() })
	return atspiInst
}

func init() {
	// Deliver accessibility.Announce (Toast/Snackbar live regions, #525) as an
	// AT-SPI Announcement event from the application root. Announce runs on the
	// main goroutine per its contract. Only emit once a provider already exists;
	// an announcement never forces the bus connection up on its own.
	accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		p := atspiInst
		if p == nil {
			return
		}
		politeness := int32(1) // ATSPI_LIVE_POLITE
		if assertive {
			politeness = 2 // ATSPI_LIVE_ASSERTIVE
		}
		_ = p.conn.Emit(atspiRootPath, ifaceEventObject+".Announcement",
			"", politeness, int32(0), dbus.MakeVariant(text), map[string]dbus.Variant{})
	})
}

// updateAccessibility synchronises this window's slice of the AT-SPI tree with
// the semantic tree. Nodes are reconciled in place keyed by object identity (or
// NodeID) so an unchanged widget keeps its exported object across frames — Orca's
// cursor is not disturbed by unrelated repaints, and events fire only on real
// changes. Runs on the main goroutine (the draw loop); D-Bus emits are deferred
// until after the provider lock is released.
func (w *window) updateAccessibility() {
	p := ensureAtspi()
	if p == nil {
		return
	}

	var events []func()
	p.mu.Lock()
	p.reconcileWindowLocked(w, &events)
	p.mu.Unlock()

	for _, emit := range events {
		emit()
	}
}

// cleanupAccessibilityForWindow tears down a window's exported frame subtree
// when the window is destroyed, emitting the children-changed:remove Orca needs
// and unexporting every object so libatspi does not leak them.
func (w *window) cleanupAccessibilityForWindow() {
	p := atspiInst
	if p == nil {
		return
	}

	var events []func()
	p.mu.Lock()
	ws := p.wins[w]
	if ws != nil {
		// Emit Cache.RemoveAccessible for every torn-down node — mandatory or
		// libatspi leaks the items (GNOME bug 650090).
		removeNode := func(n *atspiNode) {
			ref := p.ref(n.path)
			p.unexportNode(n)
			events = append(events, func() { p.emitCacheRemove(ref) })
		}
		for _, n := range ws.elements {
			removeNode(n)
		}
		for _, n := range ws.virtual {
			removeNode(n)
		}
		for _, n := range ws.realized {
			removeNode(n)
		}
		if idx := indexOfPath(p.root.children, ws.frame.path); idx >= 0 {
			p.root.children = append(p.root.children[:idx], p.root.children[idx+1:]...)
			childRef := p.ref(ws.frame.path)
			events = append(events, func() {
				p.emitChildrenChanged(atspiRootPath, false, int32(idx), childRef)
			})
		}
		removeNode(ws.frame)
		delete(p.wins, w)
	}
	p.mu.Unlock()

	for _, emit := range events {
		emit()
	}
}

// atspiReconcile threads the per-update state through the recursive tree walk.
type atspiReconcile struct {
	p                *atspiProvider
	w                *window
	ws               *atspiWinState
	focusedObj       core.CanvasObject
	activeDescendant any
	seen             map[*atspiNode]bool
	events           *[]func()
	foundFocus       *atspiNode
	added            []*atspiNode // nodes created this frame, for Cache.AddAccessible
}

// reconcileWindowLocked reconciles one window's frame subtree. Caller holds p.mu.
func (p *atspiProvider) reconcileWindowLocked(w *window, events *[]func()) {
	ws := p.wins[w]
	newFrame := ws == nil
	if ws == nil {
		frame := &atspiNode{
			p:        p,
			win:      w,
			path:     p.newPath(),
			role:     atspiRoleFrame,
			roleName: "frame",
			layer:    7, // ATSPI_LAYER_WINDOW
			parent:   p.ref(atspiRootPath),
			ifaces:   []string{ifaceAccessible, ifaceComponent},
		}
		// Base states so Orca treats the frame as a live, on-screen top level;
		// active is toggled per key-window change below.
		for _, bit := range []uint{atspiStateVisible, atspiStateShowing, atspiStateEnabled, atspiStateSensitive} {
			setState(&frame.states, bit)
		}
		ws = &atspiWinState{
			frame:    frame,
			elements: map[core.CanvasObject]*atspiNode{},
			virtual:  map[any]*atspiNode{},
			realized: map[any]*atspiNode{},
		}
		p.nodes[frame.path] = frame
		p.exportNode(frame)
		p.wins[w] = ws

		p.root.children = append(p.root.children, frame.path)
		idx := len(p.root.children) - 1
		childRef := p.ref(frame.path)
		*events = append(*events, func() {
			p.emitChildrenChanged(atspiRootPath, true, int32(idx), childRef)
		})
	}

	frame := ws.frame
	frame.name = w.title
	if w.viewport != nil {
		x, y := w.viewport.GetPos()
		frame.screenX, frame.screenY = int32(x), int32(y)
	}
	sz := w.canvas.Size()
	frame.extents = atspiRect{
		X: 0, Y: 0,
		W: int32(scale.ToScreenCoordinate(w.canvas, sz.Width)),
		H: int32(scale.ToScreenCoordinate(w.canvas, sz.Height)),
	}

	var focusedObj core.CanvasObject
	if curWindow == w {
		if f := w.canvas.Focused(); f != nil {
			focusedObj, _ = f.(core.CanvasObject)
		}
	}

	rc := &atspiReconcile{
		p: p, w: w, ws: ws,
		focusedObj:       focusedObj,
		activeDescendant: a11yActiveDescendant(focusedObj),
		seen:             map[*atspiNode]bool{},
		events:           events,
	}
	if newFrame {
		rc.added = append(rc.added, frame)
	}

	forest := accessibility.CanvasNodes(w.canvas, w.canvas.menu)
	var childPaths []dbus.ObjectPath
	for _, sn := range forest {
		if cn := p.reconcileNodeLocked(rc, sn, frame, nil); cn != nil {
			childPaths = append(childPaths, cn.path)
		}
	}
	p.setChildrenLocked(frame, childPaths, events)

	// Prune objects no longer present, unexporting their D-Bus handlers and
	// emitting Cache.RemoveAccessible (mandatory — else libatspi leaks them).
	for obj, n := range ws.elements {
		if !rc.seen[n] {
			ref := p.ref(n.path)
			p.unexportNode(n)
			delete(ws.elements, obj)
			if ws.focused == n {
				ws.focused = nil
			}
			*events = append(*events, func() { p.emitCacheRemove(ref) })
		}
	}
	for id, n := range ws.virtual {
		if !rc.seen[n] {
			ref := p.ref(n.path)
			p.unexportNode(n)
			delete(ws.virtual, id)
			if ws.focused == n {
				ws.focused = nil
			}
			*events = append(*events, func() { p.emitCacheRemove(ref) })
		}
	}

	// Drop on-demand realized cells whose grid was torn down this frame (#988):
	// their coordinates are meaningless without the owning table, and nothing
	// else prunes them (they live outside the reconcile walk by design).
	for id, n := range ws.realized {
		if p.nodes[n.tablePath] != nil {
			continue
		}
		ref := p.ref(n.path)
		p.unexportNode(n)
		ws.realizedOrder = accessibility.DropRealizedID(ws.realizedOrder, id)
		delete(ws.realized, id)
		*events = append(*events, func() { p.emitCacheRemove(ref) })
	}

	// Cache.AddAccessible for nodes created this frame, now fully populated
	// (parent-before-child, the order rc.added was filled during the walk).
	for _, n := range rc.added {
		item := p.cacheItemLocked(n)
		*events = append(*events, func() { p.emitCacheAdd(item) })
	}

	// Focus: emit state-changed:focused off→on when the focused node changes.
	if rc.foundFocus != ws.focused {
		old := ws.focused
		ws.focused = rc.foundFocus
		if old != nil {
			old.states[atspiStateFocused/32] &^= 1 << (atspiStateFocused % 32)
			oldPath := old.path
			*events = append(*events, func() { p.emitStateChanged(oldPath, "focused", 0) })
		}
		if rc.foundFocus != nil {
			setState(&rc.foundFocus.states, atspiStateFocused)
			newPath := rc.foundFocus.path
			*events = append(*events, func() { p.emitStateChanged(newPath, "focused", 1) })
		}
	}

	// Window activation follows the current key window.
	active := curWindow == w
	if active != ws.active {
		ws.active = active
		if active {
			setState(&frame.states, atspiStateActive)
		} else {
			frame.states[atspiStateActive/32] &^= 1 << (atspiStateActive % 32)
		}
		framePath, frameName := frame.path, frame.name
		*events = append(*events, func() { p.emitWindowActivate(framePath, frameName, active) })
	}
}

// reconcileNodeLocked creates or updates the exported object for one semantic
// node and recurses into its children. It returns nil when the node is culled
// for lying entirely outside its clip bounds (a row scrolled out of view) — the
// keyboard-focused node is exempt so the AT cursor is not dropped. table is the
// nearest enclosing grid (#988), threaded down so cells learn their owning
// Table's exported path at any nesting depth. Caller holds p.mu.
func (p *atspiProvider) reconcileNodeLocked(rc *atspiReconcile, sn *accessibility.Node, parent, table *atspiNode) *atspiNode {
	focused := a11yNodeFocused(sn, rc.focusedObj, rc.activeDescendant)
	if sn.HasClip && !sn.Size.IsZero() &&
		!rectsOverlap(geo.Rectangle{Pos: sn.Pos, Size: sn.Size}, sn.Clip) && !focused {
		return nil
	}

	n := p.nodeForLocked(rc, sn)
	if n == nil {
		return nil
	}
	rc.seen[n] = true

	p.applyNodeLocked(rc, n, sn, parent, table, focused)
	if focused {
		rc.foundFocus = n
	}
	if n.hasGridDims {
		table = n
	}

	var childPaths []dbus.ObjectPath
	for _, child := range sn.Children {
		if cn := p.reconcileNodeLocked(rc, child, n, table); cn != nil {
			childPaths = append(childPaths, cn.path)
		}
	}
	p.setChildrenLocked(n, childPaths, rc.events)

	// Tree outline (#988): rows are flat siblings with levels; derive their
	// NODE_CHILD_OF / NODE_PARENT_OF relations once the child list is final.
	if n.role == atspiRoleTree {
		p.computeTreeRelationsLocked(n)
	}
	return n
}

// nodeForLocked finds or creates the exported object backing a semantic node,
// keyed by stable NodeID when present (virtual/realized nodes) else by the
// CanvasObject. Caller holds p.mu.
func (p *atspiProvider) nodeForLocked(rc *atspiReconcile, sn *accessibility.Node) *atspiNode {
	switch {
	case sn.NodeID != nil:
		if n, ok := rc.ws.virtual[sn.NodeID]; ok {
			return n
		}
		// A cell realized on demand (#988) that scrolled into view: promote it
		// into the reconcile-owned map so its exported path — and the AT
		// cursor on it — survive the realize→visible transition. It leaves the
		// realized cache and reconciles normally from here.
		if n, ok := rc.ws.realized[sn.NodeID]; ok {
			rc.ws.realizedOrder = accessibility.DropRealizedID(rc.ws.realizedOrder, sn.NodeID)
			delete(rc.ws.realized, sn.NodeID)
			rc.ws.virtual[sn.NodeID] = n
			return n
		}
		n := p.newNodeLocked(rc.w)
		rc.ws.virtual[sn.NodeID] = n
		rc.added = append(rc.added, n)
		return n
	case sn.Object != nil:
		if n, ok := rc.ws.elements[sn.Object]; ok {
			return n
		}
		n := p.newNodeLocked(rc.w)
		rc.ws.elements[sn.Object] = n
		rc.added = append(rc.added, n)
		return n
	default:
		return nil
	}
}

func (p *atspiProvider) newNodeLocked(w *window) *atspiNode {
	n := &atspiNode{
		p:      p,
		win:    w,
		path:   p.newPath(),
		layer:  3, // ATSPI_LAYER_WIDGET
		ifaces: []string{ifaceAccessible, ifaceComponent},
	}
	p.nodes[n.path] = n
	p.exportNode(n)
	return n
}

// applyNodeLocked maps a semantic node's fields onto its exported object.
// table is the nearest enclosing grid (nil outside one), for TableCell (#988).
func (p *atspiProvider) applyNodeLocked(rc *atspiReconcile, n *atspiNode, sn *accessibility.Node, parent, table *atspiNode, focused bool) {
	n.role, n.roleName = atspiRole(sn.Role)
	n.name = sn.Label
	n.desc = sn.Description
	n.obj = sn.Object
	n.pressAction = a11yPressAction(sn)
	n.parent = p.ref(parent.path)
	n.level = sn.Level

	_, focusableObj := nodeInterface[event.Focusable](sn.Object)
	in := decodeNodeState(sn)
	in.focusable = focusableObj || sn.State.Has(core.StateFocused)
	n.focusable = in.focusable
	n.states = atspiStates(&in, focused)

	sc := rc.w.canvas
	n.extents = atspiRect{
		X: int32(scale.ToScreenCoordinate(sc, sn.Pos.X)),
		Y: int32(scale.ToScreenCoordinate(sc, sn.Pos.Y)),
		W: int32(scale.ToScreenCoordinate(sc, sn.Size.Width)),
		H: int32(scale.ToScreenCoordinate(sc, sn.Size.Height)),
	}
	n.screenX, n.screenY = rc.ws.frame.screenX, rc.ws.frame.screenY

	if sn.Level > 0 {
		if n.attrs == nil {
			n.attrs = map[string]string{}
		}
		n.attrs["level"] = strconv.Itoa(sn.Level)
	}

	// P1: snapshot Action/Value/Text state and rebuild the advertised interface
	// list; emits text-changed / caret-moved for any change since last frame.
	p.snapshotP1Locked(n, sn, rc.events)
	// P3: collection snapshots (Selection/Table/TableCell), appended to the
	// interface list P1 rebuilt.
	p.snapshotP3Locked(n, sn, table)
}

// setChildrenLocked updates a node's child list, emitting children-changed
// add/remove events for the difference so Orca keeps an accurate structure.
// Caller holds p.mu; the emits run after the lock is released.
func (p *atspiProvider) setChildrenLocked(n *atspiNode, next []dbus.ObjectPath, events *[]func()) {
	if pathsEqual(n.children, next) {
		return
	}
	prev := n.children
	prevSet := make(map[dbus.ObjectPath]bool, len(prev))
	for _, c := range prev {
		prevSet[c] = true
	}
	nextSet := make(map[dbus.ObjectPath]bool, len(next))
	for _, c := range next {
		nextSet[c] = true
	}
	parentPath := n.path
	for i, c := range prev {
		if !nextSet[c] {
			idx, child := int32(i), p.ref(c)
			*events = append(*events, func() { p.emitChildrenChanged(parentPath, false, idx, child) })
		}
	}
	for i, c := range next {
		if !prevSet[c] {
			idx, child := int32(i), p.ref(c)
			*events = append(*events, func() { p.emitChildrenChanged(parentPath, true, idx, child) })
		}
	}
	n.children = next
}

// decodeNodeState reduces a semantic node to the platform-neutral boolean state
// atspiStates consumes. focusable is filled by the caller (it needs the object).
func decodeNodeState(sn *accessibility.Node) nodeStateInput {
	in := nodeStateInput{
		disabled: sn.State.Has(core.StateDisabled),
		readOnly: sn.State.Has(core.StateReadOnly),
		required: sn.State.Has(core.StateRequired),
		modal:    sn.State.Has(core.StateModal),
	}
	switch sn.Role {
	case core.RoleCheckbox, core.RoleSwitch, core.RoleRadioButton:
		in.checkable = true
		in.checked = sn.State.Has(core.StateChecked)
		in.indeterminate = sn.State.Has(core.StatePartial)
	case core.RoleTextBox, core.RoleSearchField, core.RoleComboBox:
		in.editable = !in.readOnly
	case core.RoleMultiLineTextBox:
		in.editable = !in.readOnly
		in.multiLine = true
	case core.RolePasswordField:
		in.editable = !in.readOnly
	}
	switch a11yDisclosed(sn) {
	case 1:
		in.expandable, in.expanded = true, true
	case 0:
		in.expandable = true
	}
	if s := a11ySelected(sn); s >= 0 {
		in.selectable = true
		in.selected = s == 1
	}
	return in
}

// nodeInterface asserts obj to interface T, tolerating a nil object.
func nodeInterface[T any](obj core.CanvasObject) (T, bool) {
	if obj == nil {
		var zero T
		return zero, false
	}
	v, ok := obj.(T)
	return v, ok
}

func pathsEqual(a, b []dbus.ObjectPath) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOfPath(paths []dbus.ObjectPath, p dbus.ObjectPath) int {
	for i, x := range paths {
		if x == p {
			return i
		}
	}
	return -1
}
