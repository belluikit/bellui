package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OnVisibleRowsChanged reports the display rows in view when they change, so
// an app loading rows in pages can fetch the next page as the listener nears
// the end (#983, player#277).
func TestDataGrid_OnVisibleRowsChanged(t *testing.T) {
	g := newTestGrid(t, manyRows(100), []*ColumnDef{{ColID: "n", Field: "n"}})
	type rng struct{ first, last int }
	var got []rng
	g.OnVisibleRowsChanged = func(first, last int) { got = append(got, rng{first, last}) }

	layOut(t, g)
	require.NotEmpty(t, got, "laying the grid out reported no rows")
	first := got[len(got)-1]
	assert.Equal(t, 0, first.first, "the top row is in view at first")
	assert.Greater(t, first.last, 0)
	assert.Less(t, first.last, 99, "a few rows are in view, not all 100")

	got = nil
	g.ScrollToRow(99)
	require.NotEmpty(t, got, "scrolling to the end reported nothing")
	assert.Equal(t, 99, got[len(got)-1].last, "the last row is in view")

	// The same rows again is not a change.
	got = nil
	g.RefreshData()
	assert.Empty(t, got, "a refresh that left the same rows in view reported %v", got)
}
