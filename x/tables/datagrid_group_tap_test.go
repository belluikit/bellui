package tables

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A tap on a group-header row toggles that group, like the detail and tree
// chevrons, and does not report a cell click: without it a touch user, who has
// no Enter key, could not collapse a group at all (#1252). A tap on a data row
// still reports the click.
func TestDataGrid_TapGroupHeaderToggles(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "dept": "A", "name": "Alice"},
		{"id": 2, "dept": "B", "name": "Bob"},
		{"id": 3, "dept": "A", "name": "Charlie"},
		{"id": 4, "dept": "B", "name": "Dana"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 60},
		{Field: "dept", Width: 80},
		{Field: "name", Width: 100},
	})
	g.API().SetRowGroupColumn("dept")
	var clicked []CellID
	g.OnCellClicked = func(id CellID) { clicked = append(clicked, id) }

	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(600, 400))
	c := w.Canvas()

	// Display rows: A header, Alice, Charlie, B header, Bob, Dana.
	const headerB = 3
	require.True(t, g.IsGroupRow(headerB))
	key, count, expanded, _ := g.GroupRowInfo(headerB)
	require.Equal(t, "B", key)
	require.Equal(t, 2, count)
	require.True(t, expanded)

	rowCenter := func(row int) float32 {
		y, h := g.viewport.RowY(row)
		return g.HeaderHeight() + y - g.offset.Y + h/2
	}
	tapHeaderB := func(x float32) {
		t.Helper()
		test.TapCanvas(c, geo.NewPos(x, rowCenter(headerB)))
	}

	tapHeaderB(120)
	_, _, expanded, _ = g.GroupRowInfo(headerB)
	assert.False(t, expanded, "first tap collapses group B")
	assert.Equal(t, 4, g.RowCount(), "B's two rows are hidden")

	// Past the last column is still the header row.
	tapHeaderB(500)
	_, _, expanded, _ = g.GroupRowInfo(headerB)
	assert.True(t, expanded, "second tap expands group B")
	assert.Equal(t, 6, g.RowCount())

	assert.Empty(t, clicked, "a tap on a group header is not a cell click")

	test.TapCanvas(c, geo.NewPos(120, rowCenter(1)))
	require.Len(t, clicked, 1, "a tap on a data row still reports the click")
	assert.Equal(t, 1, clicked[0].Row)
	_, _, expanded, _ = g.GroupRowInfo(0)
	assert.True(t, expanded, "a data-row tap leaves its group alone")
}
