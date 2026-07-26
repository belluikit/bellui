package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

func TestDataGrid_CellSecondaryTapped(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}, {"v": 3}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionSingleCell

	var gotID CellID
	var gotEvent *event.PointEvent
	fired := 0
	g.OnCellSecondaryTapped = func(id CellID, e *event.PointEvent) {
		fired++
		gotID = id
		gotEvent = e
	}

	headerH := g.HeaderHeight()
	rowH := g.rowHeight() + g.cellPadding()

	// Secondary-tap a body cell (row 0, col 0).
	bodyEvent := &event.PointEvent{Position: geo.NewPos(10, headerH+rowH/2)}
	g.TappedSecondary(bodyEvent)

	if fired != 1 {
		t.Fatalf("expected callback to fire once, got %d", fired)
	}
	if gotID.Row != 0 || gotID.Col != 0 {
		t.Errorf("callback id = %+v, want {Row:0 Col:0}", gotID)
	}
	if gotEvent != bodyEvent {
		t.Error("callback did not receive the original PointEvent")
	}
	// A body secondary-tap also selects the cell (parity with the prior behavior).
	if sel := g.API().GetSelectedCells(); sel == nil || sel.Row != 0 || sel.Col != 0 {
		t.Errorf("expected body cell {0,0} selected, got %+v", sel)
	}
}

func TestDataGrid_HeaderSecondaryTapDoesNotFireCellCallback(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	fired := 0
	g.OnCellSecondaryTapped = func(id CellID, e *event.PointEvent) { fired++ }

	// A secondary-tap in the header area routes to the column menu, not the cell callback.
	headerH := g.HeaderHeight()
	g.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, headerH/2)})

	if fired != 0 {
		t.Errorf("header secondary-tap fired the cell callback %d times, want 0", fired)
	}
}

// A nil callback must never panic (all the other gesture handlers guard the same way).
func TestDataGrid_CellSecondaryTappedNilCallback(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	headerH := g.HeaderHeight()
	rowH := g.rowHeight() + g.cellPadding()
	g.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, headerH+rowH/2)})
}
