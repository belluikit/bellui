package render

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// fakeGrid is a minimal GridAccess for exercising the cell host wrapping.
// cols is the main (non-pinned) column count; pinnedLeft/pinnedRight extend the
// absolute display order on either side.
type fakeGrid struct {
	rows, cols           int
	pinnedLeft, pinnedRt int
	selected             map[cellKey]bool
	focused              map[cellKey]bool
	checkbox             bool
	toggled              []string  // group keys passed to ToggleGroup
	selectedCells        []cellKey // cells passed to SelectCell
	toggledDetails       []int     // rows passed to ToggleDetail

	treeActive   bool         // IsTreeRow returns this
	treeDepth    map[int]int  // per-row depth
	treeParent   map[int]bool // per-row has-children
	treeExpanded map[int]bool // per-row expanded state
	toggledTree  []int        // rows passed to ToggleTreeNode

	masterActive   bool         // master-detail active (DetailToggleWidth > 0)
	masterRows     map[int]bool // per-row IsMasterRow
	masterExpanded map[int]bool // per-row expanded (DetailBandHeight > 0)

	groupRows map[int]fakeGroupInfo // display row -> group-header info
	groupAggs map[int][]GroupAggregate
	scrollX   float32
}

// fakeGroupInfo is one group-header row's metadata (GroupRowInfo return values).
type fakeGroupInfo struct {
	key      string
	count    int
	expanded bool
	depth    int
}

func (f *fakeGrid) ColumnCount() int              { return f.cols }
func (f *fakeGrid) RowCount() int                 { return f.rows }
func (f *fakeGrid) ColumnTypeName(col int) string { return "text" }
func (f *fakeGrid) CreateCell(col int) core.CanvasObject {
	return canvas.NewText("", nil)
}

func (f *fakeGrid) UpdateCell(row, col int, c core.CanvasObject) {
	if t, ok := c.(*canvas.Text); ok {
		t.Text = f.AccessibleCellText(row, col)
	}
}
func (f *fakeGrid) RowHeight() float32           { return 30 }
func (f *fakeGrid) CellPadding() float32         { return 2 }
func (f *fakeGrid) ColumnWidths() []float32      { return repeat(100, f.cols) }
func (f *fakeGrid) IsSelected(row, col int) bool { return f.selected[cellKey{row, col}] }
func (f *fakeGrid) IsFocused(row, col int) bool  { return f.focused[cellKey{row, col}] }
func (f *fakeGrid) SelectCell(row, col int) {
	f.selectedCells = append(f.selectedCells, cellKey{row, col})
}

func (f *fakeGrid) AccessibleCellText(row, col int) string {
	return fmt.Sprintf("r%dc%d", row, col)
}

func (f *fakeGrid) AccessibleCellIdentity(row, col int) any {
	return [2]int{row, col} // the fake has no source mapping; key by (row,col)
}
func (f *fakeGrid) IsHovered(row, col int) bool { return false }
func (f *fakeGrid) IsGroupRow(row int) bool     { _, ok := f.groupRows[row]; return ok }
func (f *fakeGrid) GroupRowInfo(row int) (string, int, bool, int) {
	gi := f.groupRows[row]
	return gi.key, gi.count, gi.expanded, gi.depth
}

func (f *fakeGrid) GroupRowAggregates(row int) []GroupAggregate { return f.groupAggs[row] }

// HeaderRegions mirrors the real grid: main starts after the pinned-left
// columns; pinned-right occupies the trailing columns of a 600-wide body.
func (f *fakeGrid) HeaderRegions() (mainX, pinnedRightX, pinnedRightW float32) {
	mainX = float32(f.pinnedLeft) * 102
	if f.pinnedRt > 0 {
		pinnedRightW = float32(f.pinnedRt)*102 - 2
		pinnedRightX = 600 - pinnedRightW
	}
	return mainX, pinnedRightX, pinnedRightW
}
func (f *fakeGrid) ScrollOffsetX() float32 { return f.scrollX }

func (f *fakeGrid) ToggleGroup(groupKey string)                { f.toggled = append(f.toggled, groupKey) }
func (f *fakeGrid) IsAlternateRow(row int) bool                { return false }
func (f *fakeGrid) ShowCheckboxColumn() bool                   { return f.checkbox }
func (f *fakeGrid) ColSpanAt(row, col int) int                 { return 1 }
func (f *fakeGrid) PinnedLeftCount() int                       { return f.pinnedLeft }
func (f *fakeGrid) PinnedRightCount() int                      { return f.pinnedRt }
func (f *fakeGrid) PinnedColumnTypeName(displayCol int) string { return "text" }
func (f *fakeGrid) PinnedCreateCell(displayCol int) core.CanvasObject {
	return canvas.NewText("", nil)
}

func (f *fakeGrid) PinnedUpdateCell(row, displayCol int, c core.CanvasObject) {
	if t, ok := c.(*canvas.Text); ok {
		t.Text = f.AccessibleCellText(row, displayCol)
	}
}

func (f *fakeGrid) HeaderColumnWidths() []float32 {
	return repeat(100, f.pinnedLeft+f.cols+f.pinnedRt)
}

func (f *fakeGrid) DetailToggleWidth() float32 {
	if f.masterActive {
		return 18
	}
	return 0
}
func (f *fakeGrid) IsMasterRow(row int) bool { return f.masterActive && f.masterRows[row] }
func (f *fakeGrid) DetailBandHeight(row int) float32 {
	// Kept below the fake viewport's row height so the cell band stays positive
	// (the real grid grows the row height by the band; this fake uses uniform rows).
	if f.masterExpanded[row] {
		return 10
	}
	return 0
}
func (f *fakeGrid) DetailContent(row int) core.CanvasObject { return nil }
func (f *fakeGrid) ToggleDetail(row int)                    { f.toggledDetails = append(f.toggledDetails, row) }

func (f *fakeGrid) IsTreeRow(row int) bool { return f.treeActive }
func (f *fakeGrid) TreeRowInfo(row int) (int, bool, bool) {
	return f.treeDepth[row], f.treeParent[row], f.treeExpanded[row]
}

func (f *fakeGrid) TreeFirstColIndent(row int) float32 {
	if !f.treeActive {
		return 0
	}
	return float32(f.treeDepth[row])*TreeIndentPerLevel + TreeToggleWidth
}
func (f *fakeGrid) ToggleTreeNode(row int) { f.toggledTree = append(f.toggledTree, row) }

func repeat(v float32, n int) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func TestCellHost_Accessibility(t *testing.T) {
	g := &fakeGrid{rows: 3, cols: 2, selected: map[cellKey]bool{{1, 0}: true}}
	h := newCellHost(g, g.CreateCell(0))
	h.row, h.col = 1, 0

	if h.AccessibleRole() != core.RoleTableCell {
		t.Errorf("role = %v, want RoleTableCell", h.AccessibleRole())
	}
	if got := h.AccessibleLabel(); got != "r1c0" {
		t.Errorf("label = %q, want %q", got, "r1c0")
	}
	if r, c := h.AccessibleCellPosition(); r != 1 || c != 0 {
		t.Errorf("position = (%d,%d), want (1,0)", r, c)
	}
	if h.AccessibleState()&core.StateSelected == 0 {
		t.Error("selected cell should report StateSelected")
	}

	h.row = 0 // an unselected cell
	if h.AccessibleState()&core.StateSelected != 0 {
		t.Error("unselected cell must not report StateSelected")
	}
}

// The keyboard-focus cursor cell reports StateFocused, independent of selection,
// so assistive tech can announce the cell the cursor is on as it moves.
func TestCellHost_StateFocused(t *testing.T) {
	g := &fakeGrid{
		rows: 3, cols: 2,
		selected: map[cellKey]bool{{1, 0}: true}, // selected but not the cursor
		focused:  map[cellKey]bool{{2, 1}: true}, // the cursor cell (unselected)
	}
	h := newCellHost(g, g.CreateCell(0))

	h.row, h.col = 2, 1 // the cursor cell
	st := h.AccessibleState()
	if st&core.StateFocused == 0 {
		t.Error("cursor cell should report StateFocused")
	}
	if st&core.StateSelected != 0 {
		t.Error("cursor cell is not selected; must not report StateSelected")
	}

	h.row, h.col = 1, 0 // the selected, non-cursor cell
	st = h.AccessibleState()
	if st&core.StateFocused != 0 {
		t.Error("non-cursor cell must not report StateFocused")
	}
	if st&core.StateSelected == 0 {
		t.Error("selected cell should still report StateSelected")
	}

	// A cell that is both the cursor and selected reports both bits.
	g.focused[cellKey{1, 0}] = true
	if st := h.AccessibleState(); st&core.StateFocused == 0 || st&core.StateSelected == 0 {
		t.Errorf("cursor+selected cell state = %b, want both StateFocused and StateSelected", st)
	}
}

// A cell host declares the "select" action (so the platform exposes its
// SelectionItem pattern carrying StateSelected) and performing it selects the
// cell like a tap. The checkbox host's sentinel column -1 selects the row.
func TestCellHost_SelectAction(t *testing.T) {
	g := &fakeGrid{rows: 3, cols: 2}
	h := newCellHost(g, g.CreateCell(0))
	h.row, h.col = 2, 1

	acts := h.AccessibleActions()
	if len(acts) != 1 || acts[0].Name != core.ActionSelect {
		t.Fatalf("actions = %v, want single select", acts)
	}
	h.PerformAccessibleAction(core.ActionSelect)
	if len(g.selectedCells) != 1 || g.selectedCells[0] != (cellKey{2, 1}) {
		t.Fatalf("select routed to %v, want [(2,1)]", g.selectedCells)
	}

	// A non-select action is a no-op.
	h.PerformAccessibleAction(core.ActionExpand)
	if len(g.selectedCells) != 1 {
		t.Errorf("non-select action must not select: %v", g.selectedCells)
	}
}

// With a checkbox column the checkbox reports its visual slot (PinnedLeftCount):
// pinned-left columns keep their indices, the checkbox sits just right of them,
// and main/pinned-right columns shift one place right.
func TestCellHost_CheckboxVisualSlot(t *testing.T) {
	g := &fakeGrid{checkbox: true, pinnedLeft: 2}
	h := newCellHost(g, g.CreateCell(0))
	cases := []struct{ internal, want int }{
		{0, 0}, {1, 1}, // pinned-left columns unchanged
		{-1, 2},        // checkbox host -> PinnedLeftCount
		{2, 3}, {5, 6}, // main + pinned-right shift right
	}
	for _, c := range cases {
		h.col = c.internal
		if _, got := h.AccessibleCellPosition(); got != c.want {
			t.Errorf("checkbox+pinnedLeft=2: internal col %d -> %d, want %d", c.internal, got, c.want)
		}
	}
	// No checkbox column: indices are untouched.
	g.checkbox = false
	h.col = 2
	if _, got := h.AccessibleCellPosition(); got != 2 {
		t.Errorf("no checkbox: internal col 2 -> %d, want 2", got)
	}
}

func TestCellsRenderer_WrapsVisibleCellsAsHosts(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 100, cols: 2, selected: map[cellKey]bool{{0, 1}: true}}
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        g.rows,
		TotalCols:        g.cols,
		ColWidths:        g.ColumnWidths(),
	}
	vp.RecomputeColPositions()
	vp.Update(geo.NewPos(0, 0), geo.NewSize(400, 200))

	cw := NewCellsWidget(g, vp, NewCellPool())
	r := NewCellsRenderer(cw)
	r.Refresh()

	hosts := collectHosts(r.objects)
	if len(hosts) == 0 {
		t.Fatal("no cell hosts realized")
	}
	// Every visible data cell should be an accessible RoleTableCell whose
	// coordinates are its display position and whose label is the cell text.
	for _, h := range hosts {
		if h.AccessibleRole() != core.RoleTableCell {
			t.Fatalf("host (%d,%d) role = %v", h.row, h.col, h.AccessibleRole())
		}
		if want := fmt.Sprintf("r%dc%d", h.row, h.col); h.AccessibleLabel() != want {
			t.Errorf("host (%d,%d) label = %q, want %q", h.row, h.col, h.AccessibleLabel(), want)
		}
	}
	// The one selected cell (0,1) should carry StateSelected.
	var sawSelected bool
	for _, h := range hosts {
		if h.row == 0 && h.col == 1 {
			sawSelected = h.AccessibleState()&core.StateSelected != 0
		}
	}
	if !sawSelected {
		t.Error("selected cell (0,1) did not report StateSelected")
	}

	// Scrolling far down and refreshing must recycle hosts, not leak one per
	// row: the pool plus visible set stays bounded by the visible window.
	before := len(r.hostPool["text"]) + len(r.visible)
	vp.Update(geo.NewPos(0, 3000), geo.NewSize(400, 200))
	r.Refresh()
	after := len(r.hostPool["text"]) + len(r.visible)
	if after > before*3 {
		t.Errorf("host count grew unexpectedly after scroll: before=%d after=%d", before, after)
	}
}

// With a checkbox-selection column the select control is wrapped in a cellHost
// reporting RoleTableCell at absolute column 0 (carrying the row's selection
// state), and every data column shifts one place right so coordinates stay
// unique and match AccessibleGridDimensions.
func TestCellsRenderer_CheckboxColumnHost(t *testing.T) {
	test.NewTempApp(t)
	// The checkbox host forwards IsSelected(row, host.col); its col is the
	// sentinel -1, which in the real grid resolves to the row's selection state.
	g := &fakeGrid{rows: 10, cols: 2, checkbox: true, selected: map[cellKey]bool{{0, -1}: true}}
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        g.rows,
		TotalCols:        g.cols,
		ColWidths:        g.ColumnWidths(),
	}
	vp.RecomputeColPositions()
	vp.Update(geo.NewPos(0, 0), geo.NewSize(400, 200))

	cw := NewCellsWidget(g, vp, NewCellPool())
	r := NewCellsRenderer(cw)
	r.Refresh()

	// Collect reported positions per row.
	type pos struct{ row, col int }
	seenCols := map[pos]bool{}
	var sawCheckboxRow0 bool
	for _, h := range collectHosts(r.objects) {
		row, col := h.AccessibleCellPosition()
		if seenCols[pos{row, col}] {
			t.Errorf("duplicate cell position (%d,%d)", row, col)
		}
		seenCols[pos{row, col}] = true
		if h.col == -1 { // the checkbox host
			if col != 0 {
				t.Errorf("checkbox host reported col %d, want 0", col)
			}
			if h.AccessibleRole() != core.RoleTableCell {
				t.Errorf("checkbox host role = %v, want RoleTableCell", h.AccessibleRole())
			}
			if row == 0 {
				sawCheckboxRow0 = true
				if h.AccessibleState()&core.StateSelected == 0 {
					t.Error("checkbox host on selected row 0 should report StateSelected")
				}
			}
		} else if col == 0 {
			t.Errorf("data host (internal col %d) collided with checkbox at reported col 0", h.col)
		}
	}
	if !sawCheckboxRow0 {
		t.Fatal("no checkbox host realized for row 0")
	}
	// Data cells occupy reported cols 1 and 2 (shifted right by the checkbox).
	if !seenCols[pos{0, 1}] || !seenCols[pos{0, 2}] {
		t.Errorf("data cells not shifted right of checkbox: %v", seenCols)
	}
}

func collectHosts(objs []core.CanvasObject) []*cellHost {
	var out []*cellHost
	for _, o := range objs {
		if h, ok := o.(*cellHost); ok {
			out = append(out, h)
		}
	}
	return out
}

// A tree-active grid materializes a RoleTreeItem disclosure overlay (carrying the
// row's depth as AccessibleLevel and its expanded state) plus a chevron for parent
// rows only — leaf rows get neither, though every tree row is still indented.
func TestCellsRenderer_TreeRowDisclosure(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows:         3,
		cols:         1,
		treeActive:   true,
		treeDepth:    map[int]int{0: 0, 1: 1, 2: 1},
		treeParent:   map[int]bool{0: true, 1: false, 2: true},
		treeExpanded: map[int]bool{0: true, 2: false},
	}
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        g.rows,
		TotalCols:        g.cols,
		ColWidths:        g.ColumnWidths(),
	}
	vp.RecomputeColPositions()
	vp.Update(geo.NewPos(0, 0), geo.NewSize(400, 200))

	cw := NewCellsWidget(g, vp, NewCellPool())
	r := NewCellsRenderer(cw)
	r.Refresh()

	var nodes []*treeRowA11y
	for _, o := range r.objects {
		if n, ok := o.(*treeRowA11y); ok {
			nodes = append(nodes, n)
		}
	}
	// Rows 0 and 2 are parents → two overlays; row 1 (leaf) gets none.
	if len(nodes) != 2 {
		t.Fatalf("realized %d tree overlays, want 2 (parents only)", len(nodes))
	}
	byRow := map[int]*treeRowA11y{}
	for _, n := range nodes {
		byRow[n.row] = n
	}
	if _, ok := byRow[1]; ok {
		t.Error("leaf row 1 must not get a disclosure overlay")
	}
	// Depth-0 expanded parent: level 1, StateExpanded.
	if n := byRow[0]; n == nil || n.AccessibleLevel() != 1 || n.AccessibleState()&core.StateExpanded == 0 {
		t.Errorf("row 0 overlay = %+v, want level 1 + expanded", n)
	}
	// Depth-1 collapsed parent: level 2, no StateExpanded.
	if n := byRow[2]; n == nil || n.AccessibleLevel() != 2 || n.AccessibleState()&core.StateExpanded != 0 {
		t.Errorf("row 2 overlay = %+v, want level 2 + collapsed", n)
	}

	// A chevron is drawn per parent (two total).
	var chevrons int
	for _, o := range r.objects {
		if txt, ok := o.(*canvas.Text); ok && (txt.Text == "▼" || txt.Text == "▶") {
			chevrons++
		}
	}
	if chevrons != 2 {
		t.Errorf("drew %d chevrons, want 2 (one per parent)", chevrons)
	}
}
