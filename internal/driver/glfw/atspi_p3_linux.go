//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"github.com/godbus/dbus/v5"

	bellui "github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/accessibility"
)

// P3 of the Linux AT-SPI2 backend (pre-rebrand tracker #988): the Selection, Table and TableCell
// interfaces plus tree relations, layered on the P0 provider the same way P1
// (pre-rebrand tracker #986) layered Action/Value/Text. Collection structure is served from
// snapshots captured during reconcile (grid dimensions, cell coordinates, the
// per-row relation set), so D-Bus reads on godbus goroutines never touch a
// live widget. The one deliberate exception is Table.GetAccessibleAt for an
// off-viewport cell: it hops to the main goroutine (bellui.DoAndWait) to realize
// the cell through the neutral seam (accessibility.RealizeCell, pre-rebrand tracker #637) —
// mirroring the darwin accessibilityCell(forColumn:row:) and UIA
// IGridProvider::GetItem paths, whose callbacks arrive on the main thread.

// AT-SPI2 relation types (AtspiRelationType enum) used by the tree outline.
const (
	atspiRelationNodeChildOf  = 7
	atspiRelationNodeParentOf = 8
)

// hasStateBit reports whether bit b is set in a two-word AT-SPI state field.
func hasStateBit(s [2]uint32, b uint) bool { return s[b/32]&(1<<(b%32)) != 0 }

// atspiCellPosition is the TableCell.Position (ii) property value.
type atspiCellPosition struct{ Row, Col int32 }

// p3PropValueLocked serves the Selection / Table / TableCell properties,
// delegated from propValue. Caller holds p.mu.
func (n *atspiNode) p3PropValueLocked(iface, name string) (dbus.Variant, bool) {
	switch iface {
	case ifaceSelection:
		if n.hasSelection && name == "NSelectedChildren" {
			return dbus.MakeVariant(int32(len(n.selectedChildrenLocked()))), true
		}
	case ifaceTable:
		if !n.hasGridDims {
			break
		}
		switch name {
		case "NRows":
			return dbus.MakeVariant(int32(n.gridRows)), true
		case "NColumns":
			return dbus.MakeVariant(int32(n.gridCols)), true
		case "NSelectedRows":
			seen := map[int]bool{}
			for _, c := range n.selectedCellsLocked() {
				seen[c.cellRow] = true
			}
			return dbus.MakeVariant(int32(len(seen))), true
		case "NSelectedColumns":
			seen := map[int]bool{}
			for _, c := range n.selectedCellsLocked() {
				seen[c.cellCol] = true
			}
			return dbus.MakeVariant(int32(len(seen))), true
		case "Caption", "Summary":
			return dbus.MakeVariant(n.p.ref(atspiNullPath)), true
		}
	case ifaceTableCell:
		if !n.hasCell {
			break
		}
		switch name {
		case "RowSpan", "ColumnSpan":
			return dbus.MakeVariant(int32(1)), true
		case "Position":
			return dbus.MakeVariant(atspiCellPosition{
				Row: int32(n.cellRow), Col: int32(n.cellCol),
			}), true
		case "Table":
			return dbus.MakeVariant(n.tableRefLocked()), true
		}
	}
	return dbus.Variant{}, false
}

// snapshotP3Locked captures the collection state a node exposes — Selection on
// item containers, Table on grids, TableCell on cells — and appends the
// advertised interfaces to the list snapshotP1Locked rebuilt. table is the
// nearest ancestor grid (nil outside one). Caller holds p.mu.
func (p *atspiProvider) snapshotP3Locked(n *atspiNode, sn *accessibility.Node, table *atspiNode) {
	// Selection lives on the containers whose items carry select actions:
	// lists, tables/grids, trees and tab lists. Advertised role-based (not per
	// frame contents) so the interface list stays stable while items churn.
	n.hasSelection = false
	switch n.role {
	case atspiRoleList, atspiRoleTable, atspiRoleTree, atspiRolePageTabList:
		n.hasSelection = true
		n.ifaces = append(n.ifaces, ifaceSelection)
	}

	n.hasGridDims = sn.HasGridDims
	if sn.HasGridDims {
		n.gridRows, n.gridCols = sn.GridRows, sn.GridCols
		// Keep the semantic node so a realize request can rebuild the
		// container ref RealizeCell needs (Object, absolute Pos, scroll
		// ancestor); the walk rebuilds nodes per frame, so this snapshot is
		// refreshed every reconcile.
		n.gridSem = sn
		n.ifaces = append(n.ifaces, ifaceTable)
	} else {
		n.gridSem = nil
	}

	n.hasCell = sn.HasCellPosition
	if sn.HasCellPosition {
		n.cellRow, n.cellCol = sn.CellRow, sn.CellCol
		if table != nil {
			n.tablePath = table.path
		}
		n.ifaces = append(n.ifaces, ifaceTableCell)
	}
}

// --- org.a11y.atspi.Selection ---------------------------------------------
//
// Child indexes are positions in the container's accessible child list (the
// GetChildAtIndex space): items nest directly under their List/Table/Tree
// container, so the spaces line up. bellui's stock collections are
// single-select with no deselect affordance, so deselecting an unselected
// child succeeds trivially and clearing a live selection reports failure.

// selectionChildLocked resolves child i of a Selection container. Caller holds
// p.mu.
func (n *atspiNode) selectionChildLocked(i int32) *atspiNode {
	if !n.hasSelection || i < 0 || int(i) >= len(n.children) {
		return nil
	}
	return n.p.nodes[n.children[i]]
}

// selectedChildrenLocked returns the container's currently selected children,
// in child order. Caller holds p.mu.
func (n *atspiNode) selectedChildrenLocked() []*atspiNode {
	if !n.hasSelection {
		return nil
	}
	var out []*atspiNode
	for _, cp := range n.children {
		if c := n.p.nodes[cp]; c != nil && hasStateBit(c.states, atspiStateSelected) {
			out = append(out, c)
		}
	}
	return out
}

func (n *atspiNode) GetSelectedChild(selectedIndex int32) (objectRef, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	sel := n.selectedChildrenLocked()
	if selectedIndex < 0 || int(selectedIndex) >= len(sel) {
		return n.p.ref(atspiNullPath), nil
	}
	return n.p.ref(sel[selectedIndex].path), nil
}

func (n *atspiNode) IsChildSelected(i int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	c := n.selectionChildLocked(i)
	return c != nil && hasStateBit(c.states, atspiStateSelected), nil
}

func (n *atspiNode) SelectChild(i int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	c := n.selectionChildLocked(i)
	var obj core.CanvasObject
	if c != nil && a11yActionsHave(c.actions, core.ActionSelect) {
		obj = c.obj
	}
	n.p.mu.Unlock()
	act, ok := obj.(core.AccessibleActioner)
	if !ok {
		return false, nil
	}
	bellui.Do(func() { act.PerformAccessibleAction(core.ActionSelect) })
	return true, nil
}

func (n *atspiNode) DeselectChild(i int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	c := n.selectionChildLocked(i)
	// No deselect affordance in the action vocabulary: succeed only when the
	// child is already unselected.
	return c != nil && !hasStateBit(c.states, atspiStateSelected), nil
}

func (n *atspiNode) DeselectSelectedChild(selectedIndex int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	sel := n.selectedChildrenLocked()
	// Every entry is selected by construction, and there is no deselect
	// affordance — an in-range index always fails.
	return !(selectedIndex >= 0 && int(selectedIndex) < len(sel)), nil
}

func (n *atspiNode) SelectAll() (bool, *dbus.Error) {
	return false, nil // single-select collections
}

func (n *atspiNode) ClearSelection() (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	// Succeeds only when there is nothing to clear (no deselect affordance).
	return len(n.selectedChildrenLocked()) == 0, nil
}

// a11yActionsHave reports whether the snapshot declares the named action.
func a11yActionsHave(actions []core.AccessibleAction, name string) bool {
	for _, a := range actions {
		if a.Name == name {
			return true
		}
	}
	return false
}

// --- org.a11y.atspi.Table -------------------------------------------------

// cellAtLocked finds the exported cell of grid n at display (row, col) among
// this window's visible and realized cells. Caller holds p.mu.
func (n *atspiNode) cellAtLocked(row, col int32) *atspiNode {
	ws := n.p.wins[n.win]
	if ws == nil {
		return nil
	}
	match := func(c *atspiNode) bool {
		return c.hasCell && c.tablePath == n.path &&
			c.cellRow == int(row) && c.cellCol == int(col)
	}
	for _, c := range ws.elements {
		if match(c) {
			return c
		}
	}
	for _, c := range ws.virtual {
		if match(c) {
			return c
		}
	}
	for _, c := range ws.realized {
		if match(c) {
			return c
		}
	}
	return nil
}

func (n *atspiNode) GetAccessibleAt(row, col int32) (objectRef, *dbus.Error) {
	n.p.mu.Lock()
	if !n.hasGridDims || row < 0 || col < 0 ||
		int(row) >= n.gridRows || int(col) >= n.gridCols {
		n.p.mu.Unlock()
		return n.p.ref(atspiNullPath), nil
	}
	if c := n.cellAtLocked(row, col); c != nil {
		ref := n.p.ref(c.path)
		n.p.mu.Unlock()
		return ref, nil
	}
	n.p.mu.Unlock()

	// Off-viewport: realize on demand through the neutral seam, on the main
	// goroutine (RealizeCell touches the widget; extents need the canvas
	// scale). p.mu is NOT held across the hop — the reconcile runs on the main
	// goroutine under p.mu, so holding it here would deadlock.
	if path, ok := n.realizeCellOnMain(row, col); ok {
		return n.p.ref(path), nil
	}
	return n.p.ref(atspiNullPath), nil
}

// realizeCellOnMain materializes grid cell (row, col) as a transient exported
// object, keyed by the cell's stable identity in the per-window realized cache
// (FIFO-bounded, accessibility.RealizedCap) so repeat reads return the same
// object. Called off the main goroutine with p.mu released.
func (n *atspiNode) realizeCellOnMain(row, col int32) (path dbus.ObjectPath, ok bool) {
	p := n.p
	bellui.DoAndWait(func() {
		p.mu.Lock()
		gridSem, w := n.gridSem, n.win
		ws := p.wins[w]
		if gridSem == nil || ws == nil {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()

		// The widget call runs outside p.mu: nothing here races the reconcile
		// (both are on the main goroutine).
		wire, realized := accessibility.RealizeCell(gridSem, int(row), int(col))
		if !realized || wire.NodeID == nil {
			return
		}

		var events []func()
		p.mu.Lock()
		cell, exists := ws.realized[wire.NodeID]
		if !exists {
			cell = p.newNodeLocked(w)
			ws.realized[wire.NodeID] = cell
			ws.realizedOrder = append(ws.realizedOrder, wire.NodeID)
		}
		rc := &atspiReconcile{p: p, w: w, ws: ws, events: &events}
		p.applyNodeLocked(rc, cell, wire, n, n, false)
		if !exists {
			item := p.cacheItemLocked(cell)
			events = append(events, func() { p.emitCacheAdd(item) })
		}
		p.evictRealizedLocked(ws, &events)
		path = cell.path
		p.mu.Unlock()

		for _, emit := range events {
			emit()
		}
		ok = true
	})
	return path, ok
}

// evictRealizedLocked drops the oldest realized cells once the cache exceeds
// its cap, unexporting them and queuing the mandatory Cache.RemoveAccessible.
// Caller holds p.mu.
func (p *atspiProvider) evictRealizedLocked(ws *atspiWinState, events *[]func()) {
	keep, evict := accessibility.OverflowRealized(ws.realizedOrder, accessibility.RealizedCap)
	ws.realizedOrder = keep
	for _, id := range evict {
		cell, ok := ws.realized[id]
		if !ok {
			continue // already dropped by promotion or grid teardown
		}
		ref := p.ref(cell.path)
		p.unexportNode(cell)
		delete(ws.realized, id)
		*events = append(*events, func() { p.emitCacheRemove(ref) })
	}
}

func (n *atspiNode) GetIndexAt(row, col int32) (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.hasGridDims || n.gridCols <= 0 || row < 0 || col < 0 ||
		int(row) >= n.gridRows || int(col) >= n.gridCols {
		return -1, nil
	}
	return row*int32(n.gridCols) + col, nil
}

func (n *atspiNode) GetRowAtIndex(index int32) (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.hasGridDims || n.gridCols <= 0 || index < 0 ||
		int(index) >= n.gridRows*n.gridCols {
		return -1, nil
	}
	return index / int32(n.gridCols), nil
}

func (n *atspiNode) GetColumnAtIndex(index int32) (int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.hasGridDims || n.gridCols <= 0 || index < 0 ||
		int(index) >= n.gridRows*n.gridCols {
		return -1, nil
	}
	return index % int32(n.gridCols), nil
}

func (n *atspiNode) GetRowDescription(int32) (string, *dbus.Error) { return "", nil }

func (n *atspiNode) GetColumnDescription(int32) (string, *dbus.Error) { return "", nil }

func (n *atspiNode) GetRowExtentAt(int32, int32) (int32, *dbus.Error) { return 1, nil }

func (n *atspiNode) GetColumnExtentAt(int32, int32) (int32, *dbus.Error) { return 1, nil }

// Header accessors: bellui's Table headers are not modelled semantically (the
// header row is a separate widget layer), so no header objects exist.
func (n *atspiNode) GetRowHeader(int32) (objectRef, *dbus.Error) {
	return n.p.ref(atspiNullPath), nil
}

func (n *atspiNode) GetColumnHeader(int32) (objectRef, *dbus.Error) {
	return n.p.ref(atspiNullPath), nil
}

// selectedCellsLocked returns grid n's currently selected exported cells.
// Caller holds p.mu.
func (n *atspiNode) selectedCellsLocked() []*atspiNode {
	ws := n.p.wins[n.win]
	if ws == nil {
		return nil
	}
	var out []*atspiNode
	collect := func(c *atspiNode) {
		if c.hasCell && c.tablePath == n.path && hasStateBit(c.states, atspiStateSelected) {
			out = append(out, c)
		}
	}
	for _, c := range ws.elements {
		collect(c)
	}
	for _, c := range ws.virtual {
		collect(c)
	}
	for _, c := range ws.realized {
		collect(c)
	}
	return out
}

// Selected rows/columns are derived from the selected cells (bellui Table
// selection is per-cell; a "selected row" is one containing a selected cell).
func (n *atspiNode) GetSelectedRows() ([]int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	seen := map[int32]bool{}
	rows := []int32{}
	for _, c := range n.selectedCellsLocked() {
		if r := int32(c.cellRow); !seen[r] {
			seen[r] = true
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func (n *atspiNode) GetSelectedColumns() ([]int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	seen := map[int32]bool{}
	cols := []int32{}
	for _, c := range n.selectedCellsLocked() {
		if col := int32(c.cellCol); !seen[col] {
			seen[col] = true
			cols = append(cols, col)
		}
	}
	return cols, nil
}

func (n *atspiNode) IsRowSelected(row int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	for _, c := range n.selectedCellsLocked() {
		if int32(c.cellRow) == row {
			return true, nil
		}
	}
	return false, nil
}

func (n *atspiNode) IsColumnSelected(col int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	for _, c := range n.selectedCellsLocked() {
		if int32(c.cellCol) == col {
			return true, nil
		}
	}
	return false, nil
}

func (n *atspiNode) IsSelected(row, col int32) (bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	c := n.cellAtLocked(row, col)
	return c != nil && hasStateBit(c.states, atspiStateSelected), nil
}

func (n *atspiNode) GetRowColumnExtentsAtIndex(index int32) (bool, int32, int32, int32, int32, bool, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.hasGridDims || n.gridCols <= 0 || index < 0 ||
		int(index) >= n.gridRows*n.gridCols {
		return false, 0, 0, 0, 0, false, nil
	}
	row := index / int32(n.gridCols)
	col := index % int32(n.gridCols)
	selected := false
	if c := n.cellAtLocked(row, col); c != nil {
		selected = hasStateBit(c.states, atspiStateSelected)
	}
	return true, row, col, 1, 1, selected, nil
}

// Row/column selection mutation: bellui Table selection is per-cell, so
// whole-row/column selection is not expressible.
func (n *atspiNode) AddRowSelection(int32) (bool, *dbus.Error) { return false, nil }

func (n *atspiNode) AddColumnSelection(int32) (bool, *dbus.Error) { return false, nil }

func (n *atspiNode) RemoveRowSelection(int32) (bool, *dbus.Error) { return false, nil }

func (n *atspiNode) RemoveColumnSelection(int32) (bool, *dbus.Error) { return false, nil }

// --- org.a11y.atspi.TableCell ---------------------------------------------

func (n *atspiNode) GetRowColumnSpan() (bool, int32, int32, int32, int32, *dbus.Error) {
	n.p.mu.Lock()
	defer n.p.mu.Unlock()
	if !n.hasCell {
		return false, 0, 0, 0, 0, nil
	}
	return true, int32(n.cellRow), int32(n.cellCol), 1, 1, nil
}

func (n *atspiNode) GetRowHeaderCells() ([]objectRef, *dbus.Error) {
	return []objectRef{}, nil
}

func (n *atspiNode) GetColumnHeaderCells() ([]objectRef, *dbus.Error) {
	return []objectRef{}, nil
}

// tableRefLocked resolves the (so) reference of a cell's owning grid. Caller
// holds p.mu.
func (n *atspiNode) tableRefLocked() objectRef {
	if n.hasCell && n.tablePath != "" && n.p.nodes[n.tablePath] != nil {
		return n.p.ref(n.tablePath)
	}
	return n.p.ref(atspiNullPath)
}

// --- tree relations (org.a11y.atspi.Accessible.GetRelationSet) -------------

// computeTreeRelationsLocked derives NODE_CHILD_OF / NODE_PARENT_OF relations
// for a tree container's rows. Rendered bellui trees lay rows out flat with a
// 1-based nesting level (core.AccessibleLeveler → the "level" attribute), so a
// row's parent is the nearest preceding row one level up — the GTK flat-tree
// convention Orca reads outline structure from. Runs once per reconcile of the
// container, O(rows). Caller holds p.mu.
func (p *atspiProvider) computeTreeRelationsLocked(tree *atspiNode) {
	// stack[l-1] is the most recent row seen at level l.
	var stack []*atspiNode
	for _, cp := range tree.children {
		row := p.nodes[cp]
		if row == nil {
			continue
		}
		row.relations = nil
		lvl := row.level
		if lvl <= 0 {
			stack = stack[:0]
			continue
		}
		if lvl-1 < len(stack) {
			stack = stack[:lvl-1]
		}
		if lvl >= 2 && len(stack) >= lvl-1 {
			if parent := stack[lvl-2]; parent != nil {
				row.relations = append(row.relations, atspiRelation{
					Type:    atspiRelationNodeChildOf,
					Targets: []objectRef{p.ref(parent.path)},
				})
				addRelationTargetLocked(parent, atspiRelationNodeParentOf, p.ref(row.path))
			}
		}
		for len(stack) < lvl-1 {
			stack = append(stack, nil) // level gap in a malformed forest
		}
		stack = append(stack, row)
	}
}

// addRelationTargetLocked appends a target to n's relation of the given type,
// creating the relation on first use. Caller holds p.mu.
func addRelationTargetLocked(n *atspiNode, relType uint32, target objectRef) {
	for i := range n.relations {
		if n.relations[i].Type == relType {
			n.relations[i].Targets = append(n.relations[i].Targets, target)
			return
		}
	}
	n.relations = append(n.relations, atspiRelation{Type: relType, Targets: []objectRef{target}})
}
