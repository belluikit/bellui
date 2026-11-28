package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
)

// labelRenderer makes widget.Label cells, a different object from the text
// renderer's canvas.Text.
type labelRenderer struct{}

func (labelRenderer) CreateCell() core.CanvasObject { return widget.NewLabel("") }

func (labelRenderer) UpdateCell(o core.CanvasObject, p cell.Params) {
	o.(*widget.Label).SetText("x")
}

// Hiding a column whose cells are another kind of object than the next
// column's leaves no cell of the old kind in its place: a visible cell is
// reused only for its own column type, and pooled as that type. A text
// renderer used to be handed the label and panic (#1512, where a hidden grip
// column did it).
func TestDataGrid_HidingAColumnReusesNoCellOfAnotherType(t *testing.T) {
	g := newTestGrid(t, manyRows(20), []*ColumnDef{
		{ColID: "l", Width: 60, CellRenderer: labelRenderer{}},
		{ColID: "n", Field: "n", Width: 100},
	})
	layOut(t, g)
	assert.NotPanics(t, func() {
		g.ColumnDefs[0].Hidden = true
		g.ResolveColumns()
		g.Refresh()
		g.ColumnDefs[0].Hidden = false
		g.ResolveColumns()
		g.Refresh()
	})
}
