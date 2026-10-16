package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/tables/render"
)

// treeFixture is a small 3-level hierarchy (collapsed by default), returned as
// []map[string]any so the default value-getter resolves "name". IDs are ints so
// they compare correctly through TreeTransform's any-keyed parent map.
//
//	Root A (1) ─ Child A1 (2) ─ Leaf A1a (3)
//	           └ Child A2 (4)
//	Root B (5)
func treeFixture() []map[string]any {
	return []map[string]any{
		{"id": 1, "parent": -1, "name": "Root A"},
		{"id": 2, "parent": 1, "name": "Child A1"},
		{"id": 3, "parent": 2, "name": "Leaf A1a"},
		{"id": 4, "parent": 1, "name": "Child A2"},
		{"id": 5, "parent": -1, "name": "Root B"},
	}
}

// newTreeGrid builds a rendered grid with tree data configured over treeFixture
// (or the supplied data). The hierarchy renders on the single "name" column.
func newTreeGrid(t *testing.T, data []map[string]any) *DataGrid {
	t.Helper()
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 400))
	g.API().SetTreeData(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)
	return g
}

// nameAt returns the rendered name for a display row.
func nameAt(g *DataGrid, displayRow int) string {
	row, _ := g.pipe.DisplayRow(displayRow).(map[string]any)
	if row == nil {
		return ""
	}
	name, _ := row["name"].(string)
	return name
}

// With tree data on and nothing expanded, only root rows are visible.
func TestTreeData_CollapsedByDefaultShowsRoots(t *testing.T) {
	g := newTreeGrid(t, treeFixture())

	if g.RowCount() != 2 {
		t.Fatalf("collapsed tree should show 2 roots, got %d", g.RowCount())
	}
	if got := nameAt(g, 0); got != "Root A" {
		t.Errorf("row 0 = %q, want Root A", got)
	}
	if got := nameAt(g, 1); got != "Root B" {
		t.Errorf("row 1 = %q, want Root B", got)
	}

	// Root A has children; Root B does not.
	if _, hasChildren, _ := g.TreeRowInfo(0); !hasChildren {
		t.Error("Root A should report HasChildren")
	}
	if _, hasChildren, _ := g.TreeRowInfo(1); hasChildren {
		t.Error("Root B should not report HasChildren")
	}
}

// Expanding a parent reveals its (still-collapsed) children; collapsing hides them.
func TestTreeData_ToggleRevealsAndHidesChildren(t *testing.T) {
	g := newTreeGrid(t, treeFixture())

	g.ToggleTreeNode(0) // expand Root A
	if g.RowCount() != 4 {
		t.Fatalf("after expanding Root A, want 4 rows, got %d", g.RowCount())
	}
	want := []string{"Root A", "Child A1", "Child A2", "Root B"}
	for i, w := range want {
		if got := nameAt(g, i); got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
	if _, _, expanded := g.TreeRowInfo(0); !expanded {
		t.Error("Root A should read as expanded")
	}

	// Child A1 (row 1) has a grandchild but is collapsed; Child A2 is a leaf.
	if _, hasChildren, expanded := g.TreeRowInfo(1); !hasChildren || expanded {
		t.Errorf("Child A1: hasChildren/expanded = %v/%v, want true/false", hasChildren, expanded)
	}
	if _, hasChildren, _ := g.TreeRowInfo(2); hasChildren {
		t.Error("Child A2 should be a leaf")
	}

	g.ToggleTreeNode(0) // collapse Root A
	if g.RowCount() != 2 {
		t.Fatalf("after collapsing Root A, want 2 rows, got %d", g.RowCount())
	}
}

// Depth accumulates down the hierarchy, driving indentation and AccessibleLevel.
func TestTreeData_DepthAcrossLevels(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	g.ToggleTreeNode(0) // expand Root A → Child A1 at row 1
	g.ToggleTreeNode(1) // expand Child A1 → Leaf A1a at row 2

	if g.RowCount() != 5 {
		t.Fatalf("fully expanded A branch should show 5 rows, got %d", g.RowCount())
	}
	if got := nameAt(g, 2); got != "Leaf A1a" {
		t.Fatalf("row 2 = %q, want Leaf A1a", got)
	}
	for row, wantDepth := range map[int]int{0: 0, 1: 1, 2: 2} {
		if depth, _, _ := g.TreeRowInfo(row); depth != wantDepth {
			t.Errorf("row %d depth = %d, want %d", row, depth, wantDepth)
		}
	}
}

// The first-column indent grows by one level per depth on top of the chevron
// slot, and is zero for a non-tree row / inactive tree.
func TestTreeData_FirstColIndent(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	g.ToggleTreeNode(0)
	g.ToggleTreeNode(1)

	want0 := render.TreeToggleWidth
	want2 := 2*render.TreeIndentPerLevel + render.TreeToggleWidth
	if got := g.TreeFirstColIndent(0); got != want0 {
		t.Errorf("depth-0 indent = %v, want %v", got, want0)
	}
	if got := g.TreeFirstColIndent(2); got != want2 {
		t.Errorf("depth-2 indent = %v, want %v", got, want2)
	}
}

// ExpandAllNodes reveals the whole tree; CollapseAllNodes returns to roots only.
func TestTreeData_ExpandAllAndCollapseAll(t *testing.T) {
	g := newTreeGrid(t, treeFixture())

	g.API().ExpandAllNodes()
	if g.RowCount() != 5 {
		t.Fatalf("ExpandAllNodes should show all 5 rows, got %d", g.RowCount())
	}

	g.API().CollapseAllNodes()
	if g.RowCount() != 2 {
		t.Fatalf("CollapseAllNodes should return to 2 roots, got %d", g.RowCount())
	}
}

// Enter and Space on a focused tree parent toggle it, like a chevron click (pre-rebrand tracker #47).
func TestTreeData_KeyboardTogglesFocusedNode(t *testing.T) {
	g := newTreeGrid(t, treeFixture())

	g.currentFocus = CellID{Row: 0, Col: 0} // Root A
	g.focusSrcRow = g.sourceRow(0)

	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if _, _, expanded := g.TreeRowInfo(0); !expanded {
		t.Fatal("Enter on a tree parent did not expand it")
	}
	g.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	if _, _, expanded := g.TreeRowInfo(0); expanded {
		t.Fatal("Space on a tree parent did not collapse it")
	}
}

// Enter/Space on a focused leaf falls through to selection (no toggle handled).
func TestTreeData_KeyboardOnLeafDoesNotToggle(t *testing.T) {
	g := newTreeGrid(t, treeFixture())

	g.currentFocus = CellID{Row: 1, Col: 0} // Root B (leaf)
	g.focusSrcRow = g.sourceRow(1)

	if g.toggleFocusedTreeNode() {
		t.Error("a leaf row should not be handled as a tree toggle")
	}
}

// The chevron hit-zone starts after the depth indent, is one slot wide, and only
// fires on parent rows.
func TestTreeData_ChevronHitZoneToggles(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	headerH := g.HeaderHeight()
	mid := headerH + 5

	// Root A (row 0, depth 0): chevron zone is [0, TreeToggleWidth).
	if row := g.treeToggleAt(geo.NewPos(4, mid)); row != 0 {
		t.Fatalf("depth-0 chevron hit = %d, want 0", row)
	}
	// Past the chevron slot is not a toggle.
	if row := g.treeToggleAt(geo.NewPos(render.TreeToggleWidth+40, mid)); row != -1 {
		t.Errorf("beyond chevron slot should not toggle, got %d", row)
	}

	// Root B (row 1) is a leaf → no chevron.
	rowBY := headerH + g.rowHeight() + g.cellPadding() + 5
	if row := g.treeToggleAt(geo.NewPos(4, rowBY)); row != -1 {
		t.Errorf("leaf row should have no chevron zone, got %d", row)
	}
}

// The chevron zone shifts right by depth: a nested parent's chevron sits past the
// indent, and the indent region before it is not a toggle.
func TestTreeData_ChevronHitZoneShiftsWithDepth(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	g.ToggleTreeNode(0) // expand Root A → Child A1 (parent) at row 1
	headerH := g.HeaderHeight()

	y := headerH + g.rowHeight() + g.cellPadding() + 5 // row 1 (Child A1, depth 1)
	if _, hasChildren, _ := g.TreeRowInfo(1); !hasChildren {
		t.Fatal("expected Child A1 to be a parent")
	}
	// A press inside the depth-1 chevron slot [TreeIndentPerLevel, +TreeToggleWidth) hits.
	hitX := render.TreeIndentPerLevel + 4
	if row := g.treeToggleAt(geo.NewPos(hitX, y)); row != 1 {
		t.Errorf("depth-1 chevron hit at x=%v = %d, want 1", hitX, row)
	}
	// A press in the indent region before the chevron does not toggle.
	if row := g.treeToggleAt(geo.NewPos(2, y)); row != -1 {
		t.Errorf("indent region before chevron should not toggle, got %d", row)
	}
}

// A pointer tap on the chevron routes through ToggleTreeNode.
func TestTreeData_TapTogglesNode(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	headerH := g.HeaderHeight()

	g.Tapped(&event.PointEvent{Position: geo.NewPos(4, headerH+5)})
	if _, _, expanded := g.TreeRowInfo(0); !expanded {
		t.Fatal("tap on Root A chevron did not expand it")
	}
}

// Expansion is keyed by node ID, so it survives a re-sort that reorders siblings.
func TestTreeData_ExpansionSurvivesSort(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	g.ToggleTreeNode(0) // expand Root A; Root B is the other root

	// Sort by name descending: Root B sorts before Root A among the roots.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortDescending})

	// Find Root A's new display row and confirm it is still expanded with children.
	rootA := -1
	for i := 0; i < g.RowCount(); i++ {
		if nameAt(g, i) == "Root A" {
			rootA = i
			break
		}
	}
	if rootA < 0 {
		t.Fatal("Root A not found after sort")
	}
	if _, _, expanded := g.TreeRowInfo(rootA); !expanded {
		t.Error("Root A should stay expanded across a sort (expansion keyed by ID)")
	}
	// Its children should immediately follow it.
	if got := nameAt(g, rootA+1); got != "Child A1" && got != "Child A2" {
		t.Errorf("expected a child of Root A after it, got %q", got)
	}
}

// newPinnedTreeGrid pins the hierarchy ("name") column to the left, with a second
// plain main column, and configures tree data.
func newPinnedTreeGrid(t *testing.T, data []map[string]any) *DataGrid {
	t.Helper()
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Pinned: PinLeft},
		{Field: "id"},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 400))
	g.API().SetTreeData(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)
	return g
}

// With the hierarchy column pinned left, the disclosure chevron sits at the very
// left of the grid (grid-x 0, no pinned-width offset) and the hit-test toggles
// from there, shifting right by depth (pre-rebrand tracker #50).
func TestTreeData_PinnedLeftChevronHitZone(t *testing.T) {
	g := newPinnedTreeGrid(t, treeFixture())
	g.API().ExpandAllNodes() // Root A, Child A1, Leaf A1a, Child A2, Root B
	headerH := g.HeaderHeight()

	// Root A (row 0, depth 0): zone [0, TreeToggleWidth) at the very left.
	if row := g.treeToggleAt(geo.NewPos(4, headerH+5)); row != 0 {
		t.Fatalf("pinned depth-0 chevron hit = %d, want 0", row)
	}

	// Child A1 (row 1, depth 1): zone shifts right one level, still 0-origin
	// (no pinned-width offset, unlike a main-column tree).
	y := headerH + g.rowHeight() + g.cellPadding() + 5
	if _, hasChildren, _ := g.TreeRowInfo(1); !hasChildren {
		t.Fatal("expected Child A1 (row 1) to be a parent")
	}
	if row := g.treeToggleAt(geo.NewPos(render.TreeIndentPerLevel+4, y)); row != 1 {
		t.Errorf("pinned depth-1 chevron hit = %d, want 1", row)
	}
	// The indent region before the depth-1 chevron is not a toggle.
	if row := g.treeToggleAt(geo.NewPos(2, y)); row != -1 {
		t.Errorf("indent before pinned depth-1 chevron should not toggle, got %d", row)
	}
}

// A tap on the pinned-left chevron routes through ToggleTreeNode.
func TestTreeData_PinnedLeftTapToggles(t *testing.T) {
	g := newPinnedTreeGrid(t, treeFixture())
	headerH := g.HeaderHeight()

	g.Tapped(&event.PointEvent{Position: geo.NewPos(4, headerH+5)})
	if _, _, expanded := g.TreeRowInfo(0); !expanded {
		t.Fatal("tap on pinned Root A chevron did not expand it")
	}
}

// With no tree configured, the tree accessors are inert and the toggle is a no-op.
func TestTreeData_InactiveIsInert(t *testing.T) {
	g := newTestGrid(t, treeFixture(), []*ColumnDef{{Field: "name"}})

	if g.IsTreeRow(0) {
		t.Error("IsTreeRow should be false with no tree configured")
	}
	if got := g.TreeFirstColIndent(0); got != 0 {
		t.Errorf("inactive TreeFirstColIndent = %v, want 0", got)
	}
	before := g.RowCount()
	g.ToggleTreeNode(0) // no-op
	if g.RowCount() != before {
		t.Errorf("ToggleTreeNode on an inactive grid changed row count %d -> %d", before, g.RowCount())
	}
}

// TreeRowInfo/IsTreeRow read the right node on pages after the first: TreeNodes
// is built over the full pre-paginate output, so a page-local display row must be
// offset by the page start before indexing it (pre-rebrand tracker #51).
func TestTreeData_MetadataAlignedUnderPagination(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	g.API().ExpandAllNodes() // display: Root A, Child A1, Leaf A1a, Child A2, Root B
	g.API().EnablePagination(2)

	// Independent oracle: the fully-expanded tree's depth by row name.
	wantDepth := map[string]int{
		"Root A": 0, "Child A1": 1, "Leaf A1a": 2, "Child A2": 1, "Root B": 0,
	}
	for _, page := range []int{0, 1, 2} {
		g.API().SetPage(page)
		for r := 0; r < g.RowCount(); r++ {
			name := nameAt(g, r)
			if !g.IsTreeRow(r) {
				t.Errorf("page %d row %d (%q): IsTreeRow = false, want true", page, r, name)
				continue
			}
			depth, _, _ := g.TreeRowInfo(r)
			if depth != wantDepth[name] {
				t.Errorf("page %d row %d (%q): depth = %d, want %d", page, r, name, depth, wantDepth[name])
			}
		}
	}
}

// SetTreeData(nil, …) clears tree mode and returns to a flat view.
func TestTreeData_ClearReturnsToFlat(t *testing.T) {
	g := newTreeGrid(t, treeFixture())
	if g.RowCount() != 2 {
		t.Fatalf("precondition: want 2 roots, got %d", g.RowCount())
	}

	g.API().SetTreeData(nil, nil)
	if g.treeActive() {
		t.Error("SetTreeData(nil,nil) should disable tree data")
	}
	if g.RowCount() != len(treeFixture()) {
		t.Errorf("flat view should show all %d rows, got %d", len(treeFixture()), g.RowCount())
	}
}
