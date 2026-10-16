package render

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func testViewport(g *fakeGrid) *Viewport {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        g.rows,
		TotalCols:        g.cols,
		ColWidths:        g.ColumnWidths(),
	}
	vp.RecomputeColPositions()
	vp.Update(geo.NewPos(0, 0), geo.NewSize(400, 200))
	return vp
}

// Pinned-column cells are wrapped in accessible hosts carrying RoleTableCell,
// the cell label, and their ABSOLUTE display coordinate (pre-rebrand issue #15).
func TestPinnedCells_WrappedAsAccessibleHosts(t *testing.T) {
	test.NewTempApp(t)
	// One pinned-left column (absolute index 0), two main columns (1,2).
	g := &fakeGrid{rows: 10, cols: 2, pinnedLeft: 1}
	vp := testViewport(g)

	w := NewPinnedCellsWidget(g, PinnedLeft, NewCellPool(), vp)
	w.Resize(geo.NewSize(100, 200))
	r := w.CreateRenderer().(*pinnedCellsRenderer)
	r.Refresh()

	hosts := collectHosts(r.objects)
	if len(hosts) == 0 {
		t.Fatal("no pinned cell hosts realized")
	}
	for _, h := range hosts {
		if h.AccessibleRole() != core.RoleTableCell {
			t.Errorf("pinned host (%d,%d) role = %v, want RoleTableCell", h.row, h.col, h.AccessibleRole())
		}
		// Pinned-left occupies absolute column 0.
		if h.col != 0 {
			t.Errorf("pinned-left host col = %d, want 0 (absolute display index)", h.col)
		}
		if want := fmt.Sprintf("r%dc0", h.row); h.AccessibleLabel() != want {
			t.Errorf("pinned host label = %q, want %q", h.AccessibleLabel(), want)
		}
	}
}

// When the first visible column is pinned-left, the tree disclosure (chevron +
// depth indent + RoleTreeItem overlay) is rendered by the pinned region on that
// column, and every pinned-left cell content is indented by its row's tree indent
// (pre-rebrand tracker #50). Parent rows get a chevron + overlay; leaf rows are indented only.
func TestPinnedCells_TreeDisclosure(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 3, cols: 1, pinnedLeft: 1,
		treeActive:   true,
		treeDepth:    map[int]int{0: 0, 1: 1, 2: 1},
		treeParent:   map[int]bool{0: true, 1: false, 2: true},
		treeExpanded: map[int]bool{0: true},
	}
	vp := testViewport(g)
	w := NewPinnedCellsWidget(g, PinnedLeft, NewCellPool(), vp)
	w.Resize(geo.NewSize(120, 200))
	r := w.CreateRenderer().(*pinnedCellsRenderer)
	r.Layout(geo.NewSize(120, 200))

	// Overlays: rows 0 and 2 are parents; row 1 (leaf) gets none.
	byRow := map[int]*treeRowA11y{}
	for _, o := range r.objects {
		if n, ok := o.(*treeRowA11y); ok {
			byRow[n.row] = n
		}
	}
	if len(byRow) != 2 {
		t.Fatalf("realized %d pinned tree overlays, want 2 (parents only)", len(byRow))
	}
	if _, ok := byRow[1]; ok {
		t.Error("leaf row 1 must not get a disclosure overlay")
	}
	if n := byRow[0]; n == nil || n.AccessibleLevel() != 1 || n.AccessibleState()&core.StateExpanded == 0 {
		t.Errorf("row 0 overlay = %+v, want level 1 + expanded", n)
	}
	if n := byRow[2]; n == nil || n.AccessibleLevel() != 2 || n.AccessibleState()&core.StateExpanded != 0 {
		t.Errorf("row 2 overlay = %+v, want level 2 + collapsed", n)
	}

	// One chevron per parent.
	var chevrons int
	for _, o := range r.objects {
		if txt, ok := o.(*canvas.Text); ok && (txt.Text == "▼" || txt.Text == "▶") {
			chevrons++
		}
	}
	if chevrons != 2 {
		t.Errorf("drew %d pinned chevrons, want 2", chevrons)
	}

	// The leaf row's pinned cell content is indented by its tree indent (depth 1).
	wantIndent := g.TreeFirstColIndent(1) // 1*TreeIndentPerLevel + TreeToggleWidth
	for _, h := range collectHosts(r.objects) {
		if h.row == 1 && h.col == 0 {
			if got := h.Position().X; got != wantIndent {
				t.Errorf("leaf pinned cell x = %v, want indent %v", got, wantIndent)
			}
		}
	}
}

// With pinned-left columns present, the MAIN renderer does not draw the tree
// disclosure (it belongs to the pinned region) — otherwise it would render twice.
func TestCellsRenderer_TreeDisclosureSkippedWhenPinnedLeft(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 3, cols: 1, pinnedLeft: 1,
		treeActive:   true,
		treeDepth:    map[int]int{0: 0, 1: 1, 2: 1},
		treeParent:   map[int]bool{0: true, 1: false, 2: true},
		treeExpanded: map[int]bool{0: true},
	}
	vp := testViewport(g)
	cw := NewCellsWidget(g, vp, NewCellPool())
	r := NewCellsRenderer(cw)
	r.Refresh()

	for _, o := range r.objects {
		if _, ok := o.(*treeRowA11y); ok {
			t.Fatal("main renderer drew a tree overlay while pinned-left columns exist")
		}
	}
}

// When the first visible column is pinned-left, the master-detail disclosure
// (chevron + reserved indent + Button a11y overlay) renders in the pinned region
// on that column: master rows get a chevron + overlay, and every pinned-left cell
// is indented by the toggle width for alignment (pre-rebrand tracker #53, mirroring pre-rebrand tracker #50 for trees).
func TestPinnedCells_MasterDetailDisclosure(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 3, cols: 1, pinnedLeft: 1,
		masterActive:   true,
		masterRows:     map[int]bool{0: true, 2: true}, // row 1 is not a master row
		masterExpanded: map[int]bool{0: true},
	}
	vp := testViewport(g)
	w := NewPinnedCellsWidget(g, PinnedLeft, NewCellPool(), vp)
	w.Resize(geo.NewSize(120, 200))
	r := w.CreateRenderer().(*pinnedCellsRenderer)
	r.Layout(geo.NewSize(120, 200))

	byRow := map[int]*masterRowA11y{}
	for _, o := range r.objects {
		if m, ok := o.(*masterRowA11y); ok {
			byRow[m.row] = m
		}
	}
	if len(byRow) != 2 {
		t.Fatalf("realized %d pinned master overlays, want 2 (master rows only)", len(byRow))
	}
	if _, ok := byRow[1]; ok {
		t.Error("non-master row 1 must not get a disclosure overlay")
	}
	if m := byRow[0]; m == nil || m.AccessibleState()&core.StateExpanded == 0 {
		t.Errorf("row 0 (expanded) overlay = %+v, want StateExpanded", m)
	}
	if m := byRow[2]; m == nil || m.AccessibleState()&core.StateExpanded != 0 {
		t.Errorf("row 2 (collapsed) overlay = %+v, want no StateExpanded", m)
	}

	// Every pinned-left cell is indented by the toggle width (column alignment).
	wantIndent := g.DetailToggleWidth()
	for _, h := range collectHosts(r.objects) {
		if h.col == 0 && h.Position().X != wantIndent {
			t.Errorf("row %d pinned cell x = %v, want indent %v", h.row, h.Position().X, wantIndent)
		}
	}
}

// With pinned-left columns present, the MAIN renderer does not draw the
// master-detail chevron (it belongs to the pinned region) — no double render.
func TestCellsRenderer_MasterDisclosureSkippedWhenPinnedLeft(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 3, cols: 1, pinnedLeft: 1,
		masterActive: true,
		masterRows:   map[int]bool{0: true},
	}
	vp := testViewport(g)
	cw := NewCellsWidget(g, vp, NewCellPool())
	r := NewCellsRenderer(cw)
	r.Refresh()

	for _, o := range r.objects {
		if _, ok := o.(*masterRowA11y); ok {
			t.Fatal("main renderer drew a master overlay while pinned-left columns exist")
		}
	}
}

// Pinned hosts recycle across vertical scroll instead of leaking one per row.
func TestPinnedCells_HostsRecycle(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 500, cols: 1, pinnedLeft: 1}
	vp := testViewport(g)
	w := NewPinnedCellsWidget(g, PinnedLeft, NewCellPool(), vp)
	w.Resize(geo.NewSize(100, 200))
	r := w.CreateRenderer().(*pinnedCellsRenderer)
	r.Refresh()

	before := len(r.hostPool["text"]) + len(r.visible)
	vp.Update(geo.NewPos(0, 6000), geo.NewSize(400, 200))
	w.SetOffsetY(6000)
	r.Refresh()
	after := len(r.hostPool["text"]) + len(r.visible)
	if after > before*3 {
		t.Errorf("pinned host count grew after scroll: before=%d after=%d", before, after)
	}
}

// Main-column cell hosts report ABSOLUTE display columns (offset past the
// pinned-left block), not main-column indices.
func TestCellsRenderer_HostColumnsAreAbsolute(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 5, cols: 2, pinnedLeft: 1}
	vp := testViewport(g)

	cw := NewCellsWidget(g, vp, NewCellPool())
	r := NewCellsRenderer(cw)
	r.Refresh()

	seen := map[int]bool{}
	for _, h := range collectHosts(r.objects) {
		seen[h.col] = true
	}
	// Main cols 0,1 → absolute 1,2; nothing should claim the pinned-left slot 0.
	if !seen[1] || !seen[2] {
		t.Errorf("main hosts should report absolute cols 1,2; saw %v", seen)
	}
	if seen[0] {
		t.Errorf("main host claimed col 0 (the pinned-left slot); saw %v", seen)
	}
}

// The group-header accessible node reports RoleTableRow, its label, and toggles
// StateExpanded with the group's open/closed state.
func TestGroupRowA11y(t *testing.T) {
	fg := &fakeGrid{}
	g := newGroupRowA11y(fg)

	g.set("East", 12, true)
	if g.AccessibleRole() != core.RoleTableRow {
		t.Errorf("role = %v, want RoleTableRow", g.AccessibleRole())
	}
	if g.AccessibleLabel() != "East (12)" {
		t.Errorf("label = %q, want %q", g.AccessibleLabel(), "East (12)")
	}
	if g.AccessibleState()&core.StateExpanded == 0 {
		t.Error("expanded group should report StateExpanded")
	}

	g.set("East", 12, false)
	if g.AccessibleState()&core.StateExpanded != 0 {
		t.Error("collapsed group must not report StateExpanded")
	}
}

// The group-header node advertises the expand/collapse action matching its
// current state and routes PerformAccessibleAction through ToggleGroup only when
// the action actually changes state.
func TestGroupRowA11y_ExpandCollapseAction(t *testing.T) {
	fg := &fakeGrid{}
	g := newGroupRowA11y(fg)

	// Collapsed group offers Expand; performing it toggles.
	g.set("East", 12, false)
	if acts := g.AccessibleActions(); len(acts) != 1 || acts[0].Name != core.ActionExpand {
		t.Fatalf("collapsed actions = %v, want single expand", acts)
	}
	g.PerformAccessibleAction(core.ActionCollapse) // wrong direction: no-op
	g.PerformAccessibleAction(core.ActionExpand)
	if want := []string{"East"}; len(fg.toggled) != 1 || fg.toggled[0] != want[0] {
		t.Fatalf("toggled = %v, want %v", fg.toggled, want)
	}

	// Expanded group offers Collapse; the mismatched Expand is a no-op.
	g.set("East", 12, true)
	if acts := g.AccessibleActions(); len(acts) != 1 || acts[0].Name != core.ActionCollapse {
		t.Fatalf("expanded actions = %v, want single collapse", acts)
	}
	g.PerformAccessibleAction(core.ActionExpand) // wrong direction: no-op
	g.PerformAccessibleAction(core.ActionCollapse)
	if len(fg.toggled) != 2 {
		t.Fatalf("expected a second toggle, got %v", fg.toggled)
	}
}

// The master-row disclosure node is a Button that advertises the expand/collapse
// action matching its state, carries StateExpanded, and routes through
// ToggleDetail only when the action changes state.
func TestMasterRowA11y_ExpandCollapseAction(t *testing.T) {
	fg := &fakeGrid{}
	m := newMasterRowA11y(fg)

	// Collapsed row offers Expand; its label is derived from the row value.
	m.set(3, "Alice", false)
	if m.AccessibleRole() != core.RoleButton {
		t.Errorf("role = %v, want RoleButton", m.AccessibleRole())
	}
	if m.AccessibleLabel() != "Alice details" {
		t.Errorf("label = %q, want %q", m.AccessibleLabel(), "Alice details")
	}
	if m.AccessibleState()&core.StateExpanded != 0 {
		t.Error("collapsed row must not report StateExpanded")
	}
	if acts := m.AccessibleActions(); len(acts) != 1 || acts[0].Name != core.ActionExpand {
		t.Fatalf("collapsed actions = %v, want single expand", acts)
	}
	m.PerformAccessibleAction(core.ActionCollapse) // wrong direction: no-op
	m.PerformAccessibleAction(core.ActionExpand)
	if len(fg.toggledDetails) != 1 || fg.toggledDetails[0] != 3 {
		t.Fatalf("toggledDetails = %v, want [3]", fg.toggledDetails)
	}

	// Expanded row offers Collapse and reports StateExpanded.
	m.set(3, "Alice", true)
	if m.AccessibleState()&core.StateExpanded == 0 {
		t.Error("expanded row should report StateExpanded")
	}
	if acts := m.AccessibleActions(); len(acts) != 1 || acts[0].Name != core.ActionCollapse {
		t.Fatalf("expanded actions = %v, want single collapse", acts)
	}
	m.PerformAccessibleAction(core.ActionExpand) // wrong direction: no-op
	m.PerformAccessibleAction(core.ActionCollapse)
	if len(fg.toggledDetails) != 2 {
		t.Fatalf("expected a second toggle, got %v", fg.toggledDetails)
	}

	// Empty row value falls back to a generic label.
	m.set(0, "", false)
	if m.AccessibleLabel() != "Row details" {
		t.Errorf("empty-value label = %q, want %q", m.AccessibleLabel(), "Row details")
	}
}

// The tree-node disclosure node reports RoleTreeItem, its 1-based nesting level,
// its label, and toggles StateExpanded with the node's open/closed state.
func TestTreeRowA11y(t *testing.T) {
	fg := &fakeGrid{}
	tr := newTreeRowA11y(fg)

	tr.set(4, "Child A1", 2, true)
	if tr.AccessibleRole() != core.RoleTreeItem {
		t.Errorf("role = %v, want RoleTreeItem", tr.AccessibleRole())
	}
	if tr.AccessibleLabel() != "Child A1" {
		t.Errorf("label = %q, want %q", tr.AccessibleLabel(), "Child A1")
	}
	if tr.AccessibleLevel() != 2 {
		t.Errorf("level = %d, want 2", tr.AccessibleLevel())
	}
	if tr.AccessibleState()&core.StateExpanded == 0 {
		t.Error("expanded node should report StateExpanded")
	}

	tr.set(4, "Child A1", 2, false)
	if tr.AccessibleState()&core.StateExpanded != 0 {
		t.Error("collapsed node must not report StateExpanded")
	}
}

// The tree node advertises the expand/collapse action matching its current state
// and routes PerformAccessibleAction through ToggleTreeNode only when the action
// actually changes state.
func TestTreeRowA11y_ExpandCollapseAction(t *testing.T) {
	fg := &fakeGrid{}
	tr := newTreeRowA11y(fg)

	// Collapsed node offers Expand; performing it toggles the node's row.
	tr.set(5, "Root A", 1, false)
	if acts := tr.AccessibleActions(); len(acts) != 1 || acts[0].Name != core.ActionExpand {
		t.Fatalf("collapsed actions = %v, want single expand", acts)
	}
	tr.PerformAccessibleAction(core.ActionCollapse) // wrong direction: no-op
	tr.PerformAccessibleAction(core.ActionExpand)
	if len(fg.toggledTree) != 1 || fg.toggledTree[0] != 5 {
		t.Fatalf("toggledTree = %v, want [5]", fg.toggledTree)
	}

	// Expanded node offers Collapse; the mismatched Expand is a no-op.
	tr.set(5, "Root A", 1, true)
	if acts := tr.AccessibleActions(); len(acts) != 1 || acts[0].Name != core.ActionCollapse {
		t.Fatalf("expanded actions = %v, want single collapse", acts)
	}
	tr.PerformAccessibleAction(core.ActionExpand) // wrong direction: no-op
	tr.PerformAccessibleAction(core.ActionCollapse)
	if len(fg.toggledTree) != 2 {
		t.Fatalf("expected a second toggle, got %v", fg.toggledTree)
	}
}
