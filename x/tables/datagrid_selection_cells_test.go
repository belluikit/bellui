package tables

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/tables/cell"
)

// selectionSpy is a CellRenderer that records, per column, which display rows
// the grid last drew as selected.
type selectionSpy struct {
	field    string
	selected map[string]map[int]bool
	hovered  map[string]map[int]bool
}

func (s *selectionSpy) CreateCell() core.CanvasObject { return canvas.NewText("", nil) }
func (s *selectionSpy) UpdateCell(_ core.CanvasObject, p cell.Params) {
	if s.selected[s.field] == nil {
		s.selected[s.field] = map[int]bool{}
	}
	s.selected[s.field][p.RowIndex] = p.IsSelected
	if s.hovered != nil {
		if s.hovered[s.field] == nil {
			s.hovered[s.field] = map[int]bool{}
		}
		s.hovered[s.field][p.RowIndex] = p.IsHovered
	}
}

// selectedDrawn lists the "field:row" cells last drawn selected.
func selectedDrawn(sel map[string]map[int]bool) []string {
	var out []string
	for f, rows := range sel {
		for r, on := range rows {
			if on {
				out = append(out, f+":"+string(rune('0'+r)))
			}
		}
	}
	sort.Strings(out)
	return out
}

func spyGrid(t *testing.T, mode SelectionMode) (*DataGrid, map[string]map[int]bool) {
	t.Helper()
	sel := map[string]map[int]bool{}
	col := func(id string, pin PinSide) *ColumnDef {
		return &ColumnDef{ColID: id, Field: "n", HeaderName: id, Pinned: pin, CellRenderer: &selectionSpy{field: id, selected: sel}}
	}
	g := newTestGrid(t, manyRows(4), []*ColumnDef{col("p", PinLeft), col("a", PinNone), col("b", PinNone)})
	g.SelectionMode = mode
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(500, 300))
	return g, sel
}

// A selected cell is drawn selected in its own column, also with a pinned
// column on its left: the main cells were asked with the main-column index
// while a CellID carries the absolute one, so the highlight landed one
// column right, and a pinned cell was never drawn selected (#1401).
func TestDataGrid_SelectedCellDrawnInItsColumn(t *testing.T) {
	g, sel := spyGrid(t, SelectionSingleCell)
	g.API().SelectCell(CellID{Row: 1, Col: 1}) // "a", the first main column
	g.Refresh()
	assert.Equal(t, []string{"a:1"}, selectedDrawn(sel))

	g.API().SelectCell(CellID{Row: 2, Col: 0}) // "p", pinned left
	g.Refresh()
	assert.Equal(t, []string{"p:2"}, selectedDrawn(sel))
}

// The hovered cell is drawn hovered in its own column too: the same main
// against absolute index mix-up (#1401).
func TestDataGrid_HoveredCellDrawnInItsColumn(t *testing.T) {
	sel, hov := map[string]map[int]bool{}, map[string]map[int]bool{}
	col := func(id string, pin PinSide) *ColumnDef {
		return &ColumnDef{ColID: id, Field: "n", HeaderName: id, Pinned: pin, CellRenderer: &selectionSpy{field: id, selected: sel, hovered: hov}}
	}
	g := newTestGrid(t, manyRows(4), []*ColumnDef{col("p", PinLeft), col("a", PinNone), col("b", PinNone)})
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(500, 300))
	g.hoveredCell = &CellID{Row: 1, Col: 1} // "a"
	g.Refresh()
	assert.Equal(t, []string{"a:1"}, selectedDrawn(hov))
	g.hoveredCell = &CellID{Row: 2, Col: 0} // "p", pinned left
	g.Refresh()
	assert.Equal(t, []string{"p:2"}, selectedDrawn(hov))
}
