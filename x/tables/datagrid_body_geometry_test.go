package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// With floating filters on, the body starts below the filter row, so a tap
// on a row reaches that row, not the one a filter row's height above it. The
// row's centre is taken from where the body scroll was laid out, not from the
// hit-test's own arithmetic.
func TestDataGrid_FloatingFiltersTapReachesTheRowDrawn(t *testing.T) {
	g := newTestGrid(t, manyRows(50), []*ColumnDef{{ColID: "n", Field: "n", Width: 120}})
	g.Options.FloatingFilters = true
	var clicked []CellID
	g.OnCellClicked = func(id CellID) { clicked = append(clicked, id) }

	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(400, 400))

	bodyTop := g.scrollContent.Scroll.Position().Y
	require.Greater(t, bodyTop, g.HeaderHeight(), "the filter row sits between the header and the body")

	for _, row := range []int{0, 2, 5} {
		y, h := g.viewport.RowY(row)
		test.TapCanvas(w.Canvas(), geo.NewPos(40, bodyTop+y+h/2))
	}
	require.Len(t, clicked, 3)
	assert.Equal(t, 0, clicked[0].Row)
	assert.Equal(t, 2, clicked[1].Row)
	assert.Equal(t, 5, clicked[2].Row)
}

// Scrolling a row into view from below brings its bottom edge to the body's
// bottom edge: the scroll's height is the body's, so nothing more is taken
// off it.
func TestDataGrid_ScrollToCellBringsTheRowToTheBodyBottom(t *testing.T) {
	g := newTestGrid(t, manyRows(200), []*ColumnDef{{ColID: "n", Field: "n", Width: 120}})
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(400, 400))

	g.scrollToCell(CellID{Row: 40, Col: 0})
	y, h := g.viewport.RowY(40)
	bottom := y + h - g.scrollContent.Offset().Y
	assert.Equal(t, g.scrollContent.Scroll.Size().Height, bottom,
		"row 40's bottom edge sits at the body's bottom edge")
}
