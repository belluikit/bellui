package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVirtualGrid is a rows×cols virtualized grid that realizes off-viewport
// cells on demand — the shape a real DataGrid adapter (#637) implements. It
// backs the whole extent by a formula rather than materialized objects, proving
// AccessibleRealizeCell can answer for any coordinate without a per-cell object.
type fakeVirtualGrid struct {
	mockAccessible
	rows, cols int
	scrolledTo *[2]int // records the last AccessibleScrollCellIntoView request
}

func newFakeVirtualGrid(rows, cols int) *fakeVirtualGrid {
	return &fakeVirtualGrid{
		mockAccessible: mockAccessible{role: RoleTable, visible: true},
		rows:           rows,
		cols:           cols,
	}
}

func (g *fakeVirtualGrid) AccessibleGridDimensions() (int, int) { return g.rows, g.cols }

func (g *fakeVirtualGrid) AccessibleRealizeCell(row, col int) (AccessibleVirtualNode, bool) {
	if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
		return AccessibleVirtualNode{}, false
	}
	return AccessibleVirtualNode{
		AccessibleNode: AccessibleNode{
			Role:            RoleTableCell,
			Label:           fmt.Sprintf("R%dC%d", row, col),
			HasCellPosition: true,
			CellRow:         row,
			CellCol:         col,
			NodeID:          [2]int{row, col},
		},
		ID: [2]int{row, col},
	}, true
}

func (g *fakeVirtualGrid) AccessibleScrollCellIntoView(row, col int) {
	g.scrolledTo = &[2]int{row, col}
}

// Compile-time proof the fake satisfies the seam.
var (
	_ AccessibleVirtualCellRealizer = (*fakeVirtualGrid)(nil)
	_ AccessibleScrollIntoView      = (*fakeVirtualGrid)(nil)
	_ AccessibleGridDimensioner     = (*fakeVirtualGrid)(nil)
)

func TestAccessibleRealizeCell_OffViewportAndBounds(t *testing.T) {
	g := newFakeVirtualGrid(1_000_000, 3)

	// A far off-viewport cell realizes without materializing the rows between.
	node, ok := g.AccessibleRealizeCell(500_000, 2)
	require.True(t, ok)
	assert.Equal(t, "R500000C2", node.Label)
	assert.Equal(t, RoleTableCell, node.Role)
	assert.True(t, node.HasCellPosition)
	assert.Equal(t, 500_000, node.CellRow)
	assert.Equal(t, 2, node.CellCol)
	// The virtual node carries stable identity on both channels a bridge uses.
	assert.Equal(t, [2]int{500_000, 2}, node.ID)
	assert.Equal(t, [2]int{500_000, 2}, node.NodeID)
	// Off-viewport cells are pure virtual nodes — no backing object.
	assert.Nil(t, node.Object)

	// Out-of-range coordinates report ok=false, not a zero node treated as real.
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {1_000_000, 0}, {0, 3}} {
		_, ok := g.AccessibleRealizeCell(c[0], c[1])
		assert.False(t, ok, "row=%d col=%d should be out of range", c[0], c[1])
	}
}

func TestAccessibleScrollCellIntoView_RecordsRequest(t *testing.T) {
	g := newFakeVirtualGrid(100, 4)
	require.Nil(t, g.scrolledTo)

	g.AccessibleScrollCellIntoView(42, 1)
	require.NotNil(t, g.scrolledTo)
	assert.Equal(t, [2]int{42, 1}, *g.scrolledTo)
}

// The seam is asserted from a plain CanvasObject by the platform bridges, so a
// grid reached through the accessible tree must resolve back to it.
func TestVirtualCellRealizer_AssertableFromCanvasObject(t *testing.T) {
	var obj CanvasObject = newFakeVirtualGrid(10, 2)

	realizer, ok := obj.(AccessibleVirtualCellRealizer)
	require.True(t, ok)
	node, ok := realizer.AccessibleRealizeCell(9, 1)
	require.True(t, ok)
	assert.Equal(t, "R9C1", node.Label)

	scroller, ok := obj.(AccessibleScrollIntoView)
	require.True(t, ok)
	scroller.AccessibleScrollCellIntoView(9, 1)
	assert.Equal(t, [2]int{9, 1}, *obj.(*fakeVirtualGrid).scrolledTo)
}
