//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/belluikit/bellui/core"
)

// newTestAtspiProvider builds a bus-less provider for pure-logic tests: the
// exercised handlers only read provider state under p.mu (no conn emits, no
// export), matching the P0/P1 test harness style.
func newTestAtspiProvider() *atspiProvider {
	return &atspiProvider{
		busName: ":test.1",
		nodes:   map[dbus.ObjectPath]*atspiNode{},
		wins:    map[*window]*atspiWinState{},
	}
}

// addTestNode registers a bare node at the given path.
func addTestNode(p *atspiProvider, path dbus.ObjectPath) *atspiNode {
	n := &atspiNode{p: p, path: path}
	p.nodes[path] = n
	return n
}

func TestSnapshotP3_Interfaces(t *testing.T) {
	p := newTestAtspiProvider()

	// A list container advertises Selection (role-based).
	list := addTestNode(p, "/a/1")
	list.role, list.roleName = atspiRole(core.RoleList)
	list.ifaces = []string{ifaceAccessible, ifaceComponent}
	p.snapshotP3Locked(list, semNode(core.RoleList, 0), nil)
	if !list.hasSelection || !containsString(list.ifaces, ifaceSelection) {
		t.Errorf("list: hasSelection=%v ifaces=%v; want Selection advertised", list.hasSelection, list.ifaces)
	}
	if containsString(list.ifaces, ifaceTable) {
		t.Error("list without grid dims must not advertise Table")
	}

	// A grid with dimensions advertises Table (and Selection).
	grid := addTestNode(p, "/a/2")
	grid.role, grid.roleName = atspiRole(core.RoleTable)
	grid.ifaces = []string{ifaceAccessible, ifaceComponent}
	gsn := semNode(core.RoleTable, 0)
	gsn.HasGridDims = true
	gsn.GridRows, gsn.GridCols = 500, 3
	p.snapshotP3Locked(grid, gsn, nil)
	if !grid.hasGridDims || grid.gridRows != 500 || grid.gridCols != 3 {
		t.Errorf("grid dims = %v %dx%d; want 500x3", grid.hasGridDims, grid.gridRows, grid.gridCols)
	}
	if !containsString(grid.ifaces, ifaceTable) || !containsString(grid.ifaces, ifaceSelection) {
		t.Errorf("grid ifaces = %v; want Table + Selection", grid.ifaces)
	}
	if grid.gridSem != gsn {
		t.Error("grid must keep the semantic node for on-demand realize")
	}

	// A cell advertises TableCell and learns its owning table's path.
	cell := addTestNode(p, "/a/3")
	cell.role, cell.roleName = atspiRole(core.RoleTableCell)
	cell.ifaces = []string{ifaceAccessible, ifaceComponent}
	csn := semNode(core.RoleTableCell, 0)
	csn.HasCellPosition = true
	csn.CellRow, csn.CellCol = 41, 2
	p.snapshotP3Locked(cell, csn, grid)
	if !cell.hasCell || cell.cellRow != 41 || cell.cellCol != 2 || cell.tablePath != grid.path {
		t.Errorf("cell = %+v; want (41,2) under %s", cell, grid.path)
	}
	if !containsString(cell.ifaces, ifaceTableCell) {
		t.Errorf("cell ifaces = %v; want TableCell", cell.ifaces)
	}

	// A plain widget gains none of the P3 interfaces.
	btn := addTestNode(p, "/a/4")
	btn.role, btn.roleName = atspiRole(core.RoleButton)
	btn.ifaces = []string{ifaceAccessible, ifaceComponent}
	p.snapshotP3Locked(btn, semNode(core.RoleButton, 0), nil)
	for _, iface := range []string{ifaceSelection, ifaceTable, ifaceTableCell} {
		if containsString(btn.ifaces, iface) {
			t.Errorf("button advertises %s", iface)
		}
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// gridFixture builds a window state holding a 500x3 grid with two visible
// cells — (0,0) unselected and (1,1) selected — plus the frame bookkeeping the
// handlers resolve through.
func gridFixture(t *testing.T) (*atspiProvider, *atspiNode, *window) {
	t.Helper()
	p := newTestAtspiProvider()
	w := &window{}
	ws := &atspiWinState{
		elements: map[core.CanvasObject]*atspiNode{},
		virtual:  map[any]*atspiNode{},
		realized: map[any]*atspiNode{},
	}
	p.wins[w] = ws

	grid := addTestNode(p, "/g/1")
	grid.win = w
	grid.role, grid.roleName = atspiRole(core.RoleTable)
	grid.hasGridDims = true
	grid.gridRows, grid.gridCols = 500, 3
	grid.hasSelection = true

	mkCell := func(path dbus.ObjectPath, row, col int, selected bool) *atspiNode {
		c := addTestNode(p, path)
		c.win = w
		c.role, c.roleName = atspiRole(core.RoleTableCell)
		c.hasCell = true
		c.cellRow, c.cellCol = row, col
		c.tablePath = grid.path
		setState(&c.states, atspiStateSelectable)
		if selected {
			setState(&c.states, atspiStateSelected)
		}
		ws.virtual[path] = c
		grid.children = append(grid.children, path)
		return c
	}
	mkCell("/g/c00", 0, 0, false)
	mkCell("/g/c11", 1, 1, true)
	return p, grid, w
}

func TestTableIndexMath(t *testing.T) {
	_, grid, _ := gridFixture(t)

	if idx, _ := grid.GetIndexAt(1, 2); idx != 5 {
		t.Errorf("GetIndexAt(1,2) = %d; want 5", idx)
	}
	if row, _ := grid.GetRowAtIndex(5); row != 1 {
		t.Errorf("GetRowAtIndex(5) = %d; want 1", row)
	}
	if col, _ := grid.GetColumnAtIndex(5); col != 2 {
		t.Errorf("GetColumnAtIndex(5) = %d; want 2", col)
	}
	// Out of range → -1.
	if idx, _ := grid.GetIndexAt(500, 0); idx != -1 {
		t.Errorf("GetIndexAt(500,0) = %d; want -1", idx)
	}
	if row, _ := grid.GetRowAtIndex(1500); row != -1 {
		t.Errorf("GetRowAtIndex(1500) = %d; want -1", row)
	}
	if row, _ := grid.GetRowAtIndex(-1); row != -1 {
		t.Errorf("GetRowAtIndex(-1) = %d; want -1", row)
	}

	ok, row, col, rowExt, colExt, selected, _ := grid.GetRowColumnExtentsAtIndex(4)
	if !ok || row != 1 || col != 1 || rowExt != 1 || colExt != 1 || !selected {
		t.Errorf("GetRowColumnExtentsAtIndex(4) = %v (%d,%d) %dx%d sel=%v; want (1,1) 1x1 selected",
			ok, row, col, rowExt, colExt, selected)
	}
}

func TestTableCellQueries(t *testing.T) {
	p, grid, _ := gridFixture(t)

	// GetAccessibleAt resolves a visible cell without realizing.
	ref, _ := grid.GetAccessibleAt(1, 1)
	if ref.Path != "/g/c11" {
		t.Errorf("GetAccessibleAt(1,1) = %s; want /g/c11", ref.Path)
	}
	// Out-of-range coordinates → null, no realize attempt.
	ref, _ = grid.GetAccessibleAt(500, 0)
	if ref.Path != atspiNullPath {
		t.Errorf("GetAccessibleAt(500,0) = %s; want null", ref.Path)
	}

	cell := p.nodes["/g/c11"]
	ok, row, col, rowSpan, colSpan, _ := cell.GetRowColumnSpan()
	if !ok || row != 1 || col != 1 || rowSpan != 1 || colSpan != 1 {
		t.Errorf("GetRowColumnSpan = %v (%d,%d) %dx%d; want (1,1) 1x1", ok, row, col, rowSpan, colSpan)
	}

	// TableCell properties.
	if v, ok := cell.propValue(ifaceTableCell, "Position"); !ok {
		t.Error("Position property missing")
	} else if pos := v.Value().(atspiCellPosition); pos.Row != 1 || pos.Col != 1 {
		t.Errorf("Position = %+v; want (1,1)", pos)
	}
	if v, ok := cell.propValue(ifaceTableCell, "Table"); !ok {
		t.Error("Table property missing")
	} else if ref := v.Value().(objectRef); ref.Path != grid.path {
		t.Errorf("Table = %s; want %s", ref.Path, grid.path)
	}
	if v, ok := cell.propValue(ifaceTableCell, "RowSpan"); !ok || v.Value().(int32) != 1 {
		t.Errorf("RowSpan = %v ok=%v; want 1", v.Value(), ok)
	}
	// A non-cell serves no TableCell properties.
	if _, ok := grid.propValue(ifaceTableCell, "Position"); ok {
		t.Error("grid must not serve TableCell properties")
	}

	// Table properties.
	if v, _ := grid.propValue(ifaceTable, "NRows"); v.Value().(int32) != 500 {
		t.Errorf("NRows = %v; want 500", v.Value())
	}
	if v, _ := grid.propValue(ifaceTable, "NColumns"); v.Value().(int32) != 3 {
		t.Errorf("NColumns = %v; want 3", v.Value())
	}
	if v, _ := grid.propValue(ifaceTable, "NSelectedRows"); v.Value().(int32) != 1 {
		t.Errorf("NSelectedRows = %v; want 1", v.Value())
	}
}

func TestTableSelectionQueries(t *testing.T) {
	_, grid, _ := gridFixture(t)

	if sel, _ := grid.IsSelected(1, 1); !sel {
		t.Error("IsSelected(1,1) = false; want true")
	}
	if sel, _ := grid.IsSelected(0, 0); sel {
		t.Error("IsSelected(0,0) = true; want false")
	}
	if sel, _ := grid.IsRowSelected(1); !sel {
		t.Error("IsRowSelected(1) = false; want true")
	}
	if sel, _ := grid.IsColumnSelected(0); sel {
		t.Error("IsColumnSelected(0) = true; want false")
	}
	rows, _ := grid.GetSelectedRows()
	if len(rows) != 1 || rows[0] != 1 {
		t.Errorf("GetSelectedRows = %v; want [1]", rows)
	}
	cols, _ := grid.GetSelectedColumns()
	if len(cols) != 1 || cols[0] != 1 {
		t.Errorf("GetSelectedColumns = %v; want [1]", cols)
	}
}

func TestSelectionInterface(t *testing.T) {
	p, grid, _ := gridFixture(t)

	// NSelectedChildren counts the selected children.
	if v, ok := grid.propValue(ifaceSelection, "NSelectedChildren"); !ok || v.Value().(int32) != 1 {
		t.Errorf("NSelectedChildren = %v ok=%v; want 1", v.Value(), ok)
	}
	ref, _ := grid.GetSelectedChild(0)
	if ref.Path != "/g/c11" {
		t.Errorf("GetSelectedChild(0) = %s; want /g/c11", ref.Path)
	}
	ref, _ = grid.GetSelectedChild(1)
	if ref.Path != atspiNullPath {
		t.Errorf("GetSelectedChild(1) = %s; want null", ref.Path)
	}

	if sel, _ := grid.IsChildSelected(1); !sel {
		t.Error("IsChildSelected(1) = false; want true (child 1 = /g/c11)")
	}
	if sel, _ := grid.IsChildSelected(0); sel {
		t.Error("IsChildSelected(0) = true; want false")
	}
	if sel, _ := grid.IsChildSelected(99); sel {
		t.Error("IsChildSelected(99) = true; want false (out of range)")
	}

	// Deselect: succeeds only when already unselected (no deselect affordance).
	if ok, _ := grid.DeselectChild(0); !ok {
		t.Error("DeselectChild(0) on unselected child = false; want trivially true")
	}
	if ok, _ := grid.DeselectChild(1); ok {
		t.Error("DeselectChild(1) on selected child = true; want false")
	}
	if ok, _ := grid.DeselectSelectedChild(0); ok {
		t.Error("DeselectSelectedChild(0) = true; want false")
	}
	if ok, _ := grid.ClearSelection(); ok {
		t.Error("ClearSelection with a live selection = true; want false")
	}
	if ok, _ := grid.SelectAll(); ok {
		t.Error("SelectAll = true; want false (single-select)")
	}

	// A non-container answers inert.
	cell := p.nodes["/g/c00"]
	if ok, _ := cell.IsChildSelected(0); ok {
		t.Error("cell.IsChildSelected = true; want false (no Selection)")
	}
	if _, ok := cell.propValue(ifaceSelection, "NSelectedChildren"); ok {
		t.Error("cell must not serve Selection properties")
	}
}

func TestComputeTreeRelations(t *testing.T) {
	p := newTestAtspiProvider()
	tree := addTestNode(p, "/t/0")
	tree.role, tree.roleName = atspiRole(core.RoleTree)

	// Flat rows with levels: r0 L1, r1 L2, r2 L3, r3 L2, r4 L1.
	levels := []int{1, 2, 3, 2, 1}
	rows := make([]*atspiNode, len(levels))
	for i, lvl := range levels {
		r := addTestNode(p, dbus.ObjectPath("/t/r"+itoa(uint64(i))))
		r.level = lvl
		rows[i] = r
		tree.children = append(tree.children, r.path)
	}
	p.computeTreeRelationsLocked(tree)

	childOf := func(r *atspiNode) dbus.ObjectPath {
		for _, rel := range r.relations {
			if rel.Type == atspiRelationNodeChildOf && len(rel.Targets) == 1 {
				return rel.Targets[0].Path
			}
		}
		return ""
	}
	parentOf := func(r *atspiNode) []dbus.ObjectPath {
		for _, rel := range r.relations {
			if rel.Type == atspiRelationNodeParentOf {
				out := make([]dbus.ObjectPath, len(rel.Targets))
				for i, tgt := range rel.Targets {
					out[i] = tgt.Path
				}
				return out
			}
		}
		return nil
	}

	if got := childOf(rows[1]); got != rows[0].path {
		t.Errorf("r1 NODE_CHILD_OF = %s; want r0", got)
	}
	if got := childOf(rows[2]); got != rows[1].path {
		t.Errorf("r2 NODE_CHILD_OF = %s; want r1", got)
	}
	if got := childOf(rows[3]); got != rows[0].path {
		t.Errorf("r3 NODE_CHILD_OF = %s; want r0", got)
	}
	if got := childOf(rows[0]); got != "" {
		t.Errorf("r0 NODE_CHILD_OF = %s; want none (top level)", got)
	}
	if got := childOf(rows[4]); got != "" {
		t.Errorf("r4 NODE_CHILD_OF = %s; want none (top level)", got)
	}

	if got := parentOf(rows[0]); len(got) != 2 || got[0] != rows[1].path || got[1] != rows[3].path {
		t.Errorf("r0 NODE_PARENT_OF = %v; want [r1 r3]", got)
	}
	if got := parentOf(rows[1]); len(got) != 1 || got[0] != rows[2].path {
		t.Errorf("r1 NODE_PARENT_OF = %v; want [r2]", got)
	}
	if got := parentOf(rows[4]); got != nil {
		t.Errorf("r4 NODE_PARENT_OF = %v; want none (leaf)", got)
	}

	// Recompute after a churn must not duplicate relations.
	p.computeTreeRelationsLocked(tree)
	if got := parentOf(rows[0]); len(got) != 2 {
		t.Errorf("relations accumulated across recomputes: r0 NODE_PARENT_OF = %v", got)
	}

	// GetRelationSet serves the computed set.
	rels, _ := rows[1].GetRelationSet()
	if len(rels) != 2 {
		t.Errorf("r1 GetRelationSet len = %d; want 2 (child-of + parent-of)", len(rels))
	}
	if rels0, _ := rows[4].GetRelationSet(); rels0 == nil || len(rels0) != 0 {
		t.Errorf("r4 GetRelationSet = %v; want empty non-nil slice", rels0)
	}
}

// TestRealizedPromotion locks the realize→visible identity contract: a NodeID
// that reconciles while sitting in the realized cache is promoted into the
// virtual map keeping the same exported node (and path), and leaves the FIFO.
func TestRealizedPromotion(t *testing.T) {
	p := newTestAtspiProvider()
	w := &window{}
	ws := &atspiWinState{
		elements: map[core.CanvasObject]*atspiNode{},
		virtual:  map[any]*atspiNode{},
		realized: map[any]*atspiNode{},
	}
	p.wins[w] = ws

	cell := addTestNode(p, "/r/1")
	cell.win = w
	id := "row-41"
	ws.realized[id] = cell
	ws.realizedOrder = []any{id}

	rc := &atspiReconcile{p: p, w: w, ws: ws, seen: map[*atspiNode]bool{}}
	sn := semNode(core.RoleTableCell, 0)
	sn.NodeID = id

	got := p.nodeForLocked(rc, sn)
	if got != cell {
		t.Fatalf("nodeForLocked returned a fresh node; want the realized one promoted")
	}
	if _, still := ws.realized[id]; still {
		t.Error("promoted cell still in realized cache")
	}
	if len(ws.realizedOrder) != 0 {
		t.Errorf("realizedOrder = %v; want empty after promotion", ws.realizedOrder)
	}
	if ws.virtual[id] != cell {
		t.Error("promoted cell not adopted into virtual map")
	}
	// A second reconcile resolves it from virtual as usual.
	if again := p.nodeForLocked(rc, sn); again != cell {
		t.Error("second reconcile lost the promoted identity")
	}
}

func TestHasStateBit(t *testing.T) {
	var s [2]uint32
	setState(&s, atspiStateSelected)
	setState(&s, atspiStateRequired) // bit 33, second word
	if !hasStateBit(s, atspiStateSelected) || !hasStateBit(s, atspiStateRequired) {
		t.Error("set bits not read back")
	}
	if hasStateBit(s, atspiStateChecked) {
		t.Error("unset bit read back as set")
	}
}
