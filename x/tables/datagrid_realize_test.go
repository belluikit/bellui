package tables

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func manyRows(n int) []map[string]any {
	data := make([]map[string]any, n)
	for i := range data {
		data[i] = map[string]any{"n": fmt.Sprintf("row%d", i)}
	}
	return data
}

// layOut renders the grid in a small window so only the first handful of rows
// are realized and the viewport/scroll plumbing exists (AccessibleRealizeCell
// reads live geometry; AccessibleScrollCellIntoView needs the scroll content).
func layOut(t *testing.T, g *DataGrid) {
	t.Helper()
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 300))
}

// A screen reader realizes a cell far outside the realized viewport window and
// reads it exactly as an on-screen cell — full label, display position, and
// source-keyed identity — without materializing the rows between, and without
// scrolling.
func TestDataGrid_AccessibleRealizeCell_OffViewport(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)

	const row = 900 // far below the ~10 realized rows
	node, ok := g.AccessibleRealizeCell(row, 0)
	require.True(t, ok)

	assert.Equal(t, core.RoleTableCell, node.Role)
	assert.Equal(t, "row900", node.Label)
	assert.Equal(t, g.AccessibleCellText(row, 0), node.Label, "same text as the on-screen path")
	assert.True(t, node.HasCellPosition)
	assert.Equal(t, row, node.CellRow)
	assert.Equal(t, 0, node.CellCol)
	assert.Equal(t, g.AccessibleCellIdentity(row, 0), node.ID, "off-screen cell keyed by source identity")
	require.NotNil(t, node.ID)

	// Reading has no visible side effect — in particular it does not scroll.
	assert.Equal(t, geo.NewPos(0, 0), g.scrollContent.Offset(), "AccessibleRealizeCell must not scroll")
	// Vertically it lands well below the 300px viewport (the virtualization axis).
	assert.Greater(t, node.Pos.Y, float32(300), "off-viewport row positioned below the visible band")
}

func TestDataGrid_AccessibleRealizeCell_OutOfRange(t *testing.T) {
	g := newTestGrid(t, manyRows(50), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)

	rows, cols := g.AccessibleGridDimensions() // (50, 1)
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {rows, 0}, {0, cols}} {
		node, ok := g.AccessibleRealizeCell(c[0], c[1])
		assert.False(t, ok, "out-of-range %v should not realize", c)
		assert.Zero(t, node)
	}
}

// The realized cell reflects selection state and declares the select action —
// declaring it is what makes the platform expose the cell's selection to the AT.
func TestDataGrid_AccessibleRealizeCell_SelectionState(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	g.SelectionMode = SelectionSingleCell
	layOut(t, g)

	g.SelectCell(900, 0) // select an off-viewport cell

	node, ok := g.AccessibleRealizeCell(900, 0)
	require.True(t, ok)
	assert.NotZero(t, node.State&core.StateSelected, "realized cell reflects selection")
	require.Len(t, node.Actions, 1)
	assert.Equal(t, core.ActionSelect, node.Actions[0].Name)
}

// With a checkbox-selection column and a pinned-left column, the display column
// space the AT navigates inserts the checkbox at slot PinnedLeftCount and shifts
// the columns right of it. AccessibleRealizeCell reverses that shift, so each
// display column realizes the correct underlying cell — the exact inverse of a
// realized host's AccessibleCellPosition.
func TestDataGrid_AccessibleRealizeCell_CheckboxColumnShift(t *testing.T) {
	data := make([]map[string]any, 100)
	for i := range data {
		data[i] = map[string]any{"a": "AA", "b": "BB", "c": "CC"}
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{ColID: "a", Field: "a", Pinned: PinLeft},
		{ColID: "b", Field: "b"},
		{ColID: "c", Field: "c"},
	})
	g.Options.CheckboxSelection = true
	g.SelectionMode = SelectionMultiRow
	layOut(t, g)

	// Layout order: pinned-left [a] then main [b, c]; the checkbox column sits at
	// display slot PinnedLeftCount (1), shifting b and c right.
	rows, cols := g.AccessibleGridDimensions()
	require.Equal(t, 100, rows)
	require.Equal(t, 4, cols, "pinned-left + checkbox + 2 main")

	// display col -> expected full label at an off-viewport row.
	const row = 80
	for display, want := range map[int]string{
		0: "AA", // pinned-left column a
		1: "",   // checkbox column (no text)
		2: "BB", // main column b, shifted right of the checkbox
		3: "CC", // main column c
	} {
		node, ok := g.AccessibleRealizeCell(row, display)
		require.True(t, ok, "display col %d", display)
		assert.Equal(t, want, node.Label, "display col %d label", display)
		assert.Equal(t, display, node.CellCol, "reported position is the display column")
	}
}

// AccessibleScrollCellIntoView brings an off-viewport cell into view — the
// explicit realize/activate path — via the grid's own scrollToCell.
func TestDataGrid_AccessibleScrollCellIntoView(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)

	require.Equal(t, geo.NewPos(0, 0), g.scrollContent.Offset())

	g.AccessibleScrollCellIntoView(900, 0)
	assert.Greater(t, g.scrollContent.Offset().Y, float32(0), "scrolled down to the off-viewport row")

	// Out-of-range is ignored (no panic, no scroll change beyond the clamp).
	before := g.scrollContent.Offset()
	g.AccessibleScrollCellIntoView(5000, 0)
	assert.Equal(t, before, g.scrollContent.Offset(), "out-of-range row ignored")
}
