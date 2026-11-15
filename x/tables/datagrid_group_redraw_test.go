package tables

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// A grouped grid already on screen redraws its group headers and its pinned
// cells when its rows or its grouping change, with no layout pass (#1254). A
// collapse moves the next header up to its new row, new rows draw their
// group's header, and the pinned band shows the new rows. Before, RefreshData
// and rebuildPipeline (which ToggleGroup, sort, filter and paging run)
// refreshed only the cells and the column header, so the header overlay kept
// the rows it last laid out until a layout, a scroll or a tap.
func TestDataGrid_GroupHeadersRedrawWithoutLayout(t *testing.T) {
	test.NewTempApp(t)
	data := []map[string]any{
		{"id": 1, "dept": "A", "name": "Alice"},
		{"id": 2, "dept": "B", "name": "Bob"},
		{"id": 3, "dept": "A", "name": "Charlie"},
	}
	g := NewDataGrid([]*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft},
		{Field: "dept", Width: 80},
		{Field: "name", Width: 100},
	}, func() int { return len(data) }, func(row int) any { return data[row] })
	g.API().SetRowGroupColumn("dept")
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(600, 400))

	// texts is every drawn text under o, read without laying anything out.
	texts := func(o core.CanvasObject) map[string]float32 {
		out := map[string]float32{}
		walkCanvasObjects(o, func(c core.CanvasObject) {
			if txt, ok := c.(*canvas.Text); ok && txt.Visible() && txt.Text != "" {
				out[txt.Text] = txt.Position().Y
			}
		})
		return out
	}
	// labelY is where a header row's label sits in the overlay: 2 below the
	// row's top (groupRowWidget.update).
	labelY := func(row int) float32 {
		y, _ := g.viewport.RowY(row)
		return y - g.offset.Y + 2
	}

	// Display rows: A header, Alice, Charlie, B header, Bob.
	if y, ok := texts(g.groupRowOverlay)["B (1)"]; !ok || y != labelY(3) {
		t.Fatalf("laid out: B's header at y %v (drawn %v), want %v", y, ok, labelY(3))
	}

	// Collapse A: B's header is display row 1 now.
	g.ToggleGroup("A")
	if y, ok := texts(g.groupRowOverlay)["B (1)"]; !ok || y != labelY(1) {
		t.Errorf("after collapsing A: B's header at y %v (drawn %v), want %v (row 1)", y, ok, labelY(1))
	}

	// New rows in a new group: C's header is drawn, and the pinned id too.
	data = append(data, map[string]any{"id": 4, "dept": "C", "name": "Dana"})
	g.RefreshData()
	if _, ok := texts(g.groupRowOverlay)["C (1)"]; !ok {
		t.Errorf("after RefreshData with a C row: C's header not drawn; overlay texts %v", texts(g.groupRowOverlay))
	}
	if _, ok := texts(g.pinnedLeftCells)["4"]; !ok {
		t.Errorf("after RefreshData with id 4: the pinned id column doesn't show 4; texts %v", texts(g.pinnedLeftCells))
	}
}
