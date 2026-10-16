//go:build accessibility

package mobile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/widget"
)

// fakeVirtualGrid models a virtualized table (fytables DataGrid): a huge
// logical extent via AccessibleGridDimensioner, a bounded near-viewport window
// of virtual cell nodes, and the pre-rebrand tracker #637 point-addressed realize seam for
// everything beyond it.
type fakeVirtualGrid struct {
	widget.BaseWidget
	totalRows, cols    int
	winOffset, winRows int
	scrolledTo         *[2]int
	lastScroll         *event.ScrollEvent
}

func newFakeVirtualGrid() *fakeVirtualGrid {
	g := &fakeVirtualGrid{totalRows: 1_000_000, cols: 3, winOffset: 0, winRows: 2}
	g.ExtendBaseWidget(g)
	return g
}

func (g *fakeVirtualGrid) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(""))
}
func (g *fakeVirtualGrid) AccessibleRole() core.AccessibleRole        { return core.RoleTable }
func (g *fakeVirtualGrid) AccessibleLabel() string                    { return "big grid" }
func (g *fakeVirtualGrid) AccessibleGridDimensions() (rows, cols int) { return g.totalRows, g.cols }

func (g *fakeVirtualGrid) cellNode(row, col int) core.AccessibleVirtualNode {
	return core.AccessibleVirtualNode{
		AccessibleNode: core.AccessibleNode{
			Role:            core.RoleTableCell,
			Label:           fmt.Sprintf("r%dc%d", row, col),
			HasCellPosition: true,
			CellRow:         row,
			CellCol:         col,
			Actions:         []core.AccessibleAction{{Name: core.ActionSelect}},
		},
		Pos:  geo.NewPos(float32(col*50), float32((row-g.winOffset)*20)),
		Size: geo.NewSize(50, 20),
		ID:   [2]int{row, col},
	}
}

func (g *fakeVirtualGrid) AccessibleVirtualNodes() []core.AccessibleVirtualNode {
	var out []core.AccessibleVirtualNode
	for r := g.winOffset; r < g.winOffset+g.winRows; r++ {
		for c := 0; c < g.cols; c++ {
			out = append(out, g.cellNode(r, c))
		}
	}
	return out
}

func (g *fakeVirtualGrid) AccessibleRealizeCell(row, col int) (core.AccessibleVirtualNode, bool) {
	if row < 0 || row >= g.totalRows || col < 0 || col >= g.cols {
		return core.AccessibleVirtualNode{}, false
	}
	return g.cellNode(row, col), true
}

func (g *fakeVirtualGrid) AccessibleScrollCellIntoView(row, col int) {
	if row < 0 || row >= g.totalRows || col < 0 || col >= g.cols {
		return
	}
	g.scrolledTo = &[2]int{row, col}
	g.winOffset = row
}

// Scrolled makes the grid its own scroll handler (event.Scrollable), so its
// virtual cells inherit it as their scroll target (pre-rebrand tracker #952).
func (g *fakeVirtualGrid) Scrolled(ev *event.ScrollEvent) { g.lastScroll = ev }

func newRealizeTestWindow(t *testing.T) (*window, *fakeVirtualGrid) {
	t.Helper()
	w := newA11yTestWindow(t)
	grid := newFakeVirtualGrid()
	w.SetContent(grid)
	w.Resize(geo.NewSize(200, 100))
	return w, grid
}

func gridWireNode(t *testing.T, nodes []mobileA11yNode) mobileA11yNode {
	t.Helper()
	for _, n := range nodes {
		if n.role == mobileRoleGrid {
			return n
		}
	}
	t.Fatal("no grid container node in the flat sequence")
	return mobileA11yNode{}
}

func TestMobileA11yGather_VirtualizedGridContainer(t *testing.T) {
	w, _ := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	grid := gridWireNode(t, nodes)
	assert.Equal(t, "big grid", grid.label)
	assert.Equal(t, 1_000_000, grid.rowCount, "container carries the full extent")
	assert.Equal(t, 3, grid.colCount)
	assert.Zero(t, grid.gridHandle, "the grid itself is not inside a grid")

	// The visible window's cells follow the container, associated to it and
	// carrying their display coordinates.
	var cells []mobileA11yNode
	for _, n := range nodes {
		if n.role == mobileRoleCell {
			cells = append(cells, n)
		}
	}
	require.Len(t, cells, 6, "2 window rows x 3 cols")
	for _, c := range cells {
		assert.Equal(t, grid.handle, c.gridHandle)
	}
	assert.Equal(t, 0, cells[0].cellRow)
	assert.Equal(t, 0, cells[0].cellCol)
	assert.Equal(t, 1, cells[5].cellRow)
	assert.Equal(t, 2, cells[5].cellCol)

	// A plain widget (non-realizer) still contributes no container element:
	// the grid role is gated on the realize seam.
	w2 := newA11yTestWindow(t)
	w2.SetContent(widget.NewLabel("plain"))
	w2.Resize(geo.NewSize(100, 50))
	plain, _ := w2.mobileA11yGather()
	for _, n := range plain {
		assert.NotEqual(t, mobileRoleGrid, n.role)
	}
}

func TestMobileA11yRealizeCell_OffViewport(t *testing.T) {
	w, _ := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	grid := gridWireNode(t, nodes)

	cell, evicted, ok := mobileA11yRealizeCellOnMain(grid.handle, 500_000, 1)
	require.True(t, ok, "an off-viewport cell realizes on demand")
	assert.Empty(t, evicted)
	assert.Equal(t, mobileRoleCell, cell.role)
	assert.Equal(t, "r500000c1", cell.label)
	assert.Equal(t, 500_000, cell.cellRow)
	assert.Equal(t, 1, cell.cellCol)
	assert.Equal(t, grid.handle, cell.gridHandle)
	assert.NotZero(t, cell.flags&mobileA11yFlagPressable, "declared select action makes it activatable")

	// The realized cell resolves as a dispatch target bound to its grid.
	target, found := mobileA11yLookup(cell.handle)
	require.True(t, found)
	assert.Equal(t, grid.handle, target.grid)
	assert.Nil(t, target.node.Object, "realized cells have no CanvasObject")

	// A repeat read returns the same element.
	again, _, ok := mobileA11yRealizeCellOnMain(grid.handle, 500_000, 1)
	require.True(t, ok)
	assert.Equal(t, cell.handle, again.handle, "identity is stable across reads")

	// Out-of-range coordinates and non-realizer handles are rejected.
	_, _, ok = mobileA11yRealizeCellOnMain(grid.handle, 1_000_000, 0)
	assert.False(t, ok)
	var cellHandle uintptr
	for _, n := range nodes {
		if n.role == mobileRoleCell {
			cellHandle = n.handle
			break
		}
	}
	require.NotZero(t, cellHandle)
	_, _, ok = mobileA11yRealizeCellOnMain(cellHandle, 1, 1)
	assert.False(t, ok, "realizing against a non-grid element fails")
	_, _, ok = mobileA11yRealizeCellOnMain(99999, 1, 1)
	assert.False(t, ok, "unknown handle fails")
}

func TestMobileA11yRealizeCell_VisibleCellReturnsWalkedElement(t *testing.T) {
	w, _ := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	grid := gridWireNode(t, nodes)
	var visible *mobileA11yNode
	for i := range nodes {
		if nodes[i].role == mobileRoleCell && nodes[i].cellRow == 1 && nodes[i].cellCol == 2 {
			visible = &nodes[i]
		}
	}
	require.NotNil(t, visible)

	cell, _, ok := mobileA11yRealizeCellOnMain(grid.handle, 1, 2)
	require.True(t, ok)
	assert.Equal(t, visible.handle, cell.handle,
		"a cell already in the walked window reuses its element, not a duplicate")
}

func TestMobileA11yRealizeCell_EvictionAndPromotion(t *testing.T) {
	w, grid := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	gridNode := gridWireNode(t, nodes)

	// Fill past the FIFO cap: the oldest realized cells are evicted and their
	// handles reported for element teardown.
	first, _, ok := mobileA11yRealizeCellOnMain(gridNode.handle, 1000, 0)
	require.True(t, ok)
	var totalEvicted []uintptr
	for i := 0; i < accessibility.RealizedCap; i++ {
		_, evicted, ok := mobileA11yRealizeCellOnMain(gridNode.handle, 2000+i, 0)
		require.True(t, ok)
		totalEvicted = append(totalEvicted, evicted...)
	}
	require.Len(t, totalEvicted, 1, "one over the cap evicts exactly the oldest")
	assert.Equal(t, first.handle, totalEvicted[0])
	_, found := mobileA11yLookup(first.handle)
	assert.False(t, found, "evicted cells leave the dispatch registry")

	// Promotion: realize a cell, then scroll the grid's window over it. The
	// walked element reuses the realized handle, and the realized cache drains.
	cell, _, ok := mobileA11yRealizeCellOnMain(gridNode.handle, 700_000, 1)
	require.True(t, ok)
	grid.winOffset = 700_000
	grid.Refresh()
	after, _ := w.mobileA11yGather()
	var promoted *mobileA11yNode
	for i := range after {
		if after[i].role == mobileRoleCell && after[i].cellRow == 700_000 && after[i].cellCol == 1 {
			promoted = &after[i]
		}
	}
	require.NotNil(t, promoted)
	assert.Equal(t, cell.handle, promoted.handle,
		"the AT cursor survives the realize -> visible transition")

	a11yMu.Lock()
	st := a11yWindows[w]
	_, stillRealized := st.realized[[2]int{700_000, 1}]
	a11yMu.Unlock()
	assert.False(t, stillRealized, "promotion drains the realized cache entry")
}

func TestMobileA11yDispatch_ActivateRealizedCellScrollsIntoView(t *testing.T) {
	w, grid := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	gridNode := gridWireNode(t, nodes)
	cell, _, ok := mobileA11yRealizeCellOnMain(gridNode.handle, 123_456, 2)
	require.True(t, ok)

	mobileA11yDispatchOnMain(cell.handle, mobileA11yActionActivate)
	require.NotNil(t, grid.scrolledTo, "activating an off-viewport cell brings it into view")
	assert.Equal(t, [2]int{123_456, 2}, *grid.scrolledTo)
}

// The AT scroll gesture over a virtualized grid's cells pages the grid itself:
// virtual cells inherit the Scrollable provider as their scroll target (pre-rebrand tracker #952 —
// the iOS accessibilityScroll: path and Android's per-item scroll actions both
// dispatch through this), and forward reveals the content below.
func TestMobileA11yScroll_VirtualCellPagesGrid(t *testing.T) {
	w, grid := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	var cell *mobileA11yNode
	for i := range nodes {
		if nodes[i].role == mobileRoleCell {
			cell = &nodes[i]
			break
		}
	}
	require.NotNil(t, cell)
	assert.NotZero(t, cell.flags&mobileA11yFlagScrollable,
		"cells of a Scrollable virtualized grid expose the AT scroll gesture")

	mobileA11yDispatchOnMain(cell.handle, mobileA11yActionScrollForward)
	require.NotNil(t, grid.lastScroll, "the gesture pages the grid itself")
	assert.Negative(t, grid.lastScroll.Scrolled.DY, "forward reveals content below")

	mobileA11yDispatchOnMain(cell.handle, mobileA11yActionScrollBackward)
	assert.Positive(t, grid.lastScroll.Scrolled.DY, "backward reveals content above")

	// A realized off-viewport cell carries the same scroll target.
	gridNode := gridWireNode(t, nodes)
	realized, _, ok := mobileA11yRealizeCellOnMain(gridNode.handle, 900_000, 0)
	require.True(t, ok)
	assert.NotZero(t, realized.flags&mobileA11yFlagScrollable)
}

func TestMobileA11yScrollCellIntoView_RoutesToGrid(t *testing.T) {
	w, grid := newRealizeTestWindow(t)

	nodes, _ := w.mobileA11yGather()
	gridNode := gridWireNode(t, nodes)

	target, found := mobileA11yLookup(gridNode.handle)
	require.True(t, found)
	assert.True(t, accessibility.ScrollCellIntoView(target.node, 42, 1),
		"the grid target routes the seam's bring-into-view")
	require.NotNil(t, grid.scrolledTo)
	assert.Equal(t, [2]int{42, 1}, *grid.scrolledTo)
}
