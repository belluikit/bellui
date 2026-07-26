package tables

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestDataGrid_RendererLayout(t *testing.T) {
	data := []map[string]any{{"a": 1}, {"a": 2}, {"a": 3}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a"}})

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	// Grid should render without panic
	if g.ColumnCount() != 1 {
		t.Errorf("expected 1 column, got %d", g.ColumnCount())
	}
	if g.RowCount() != 3 {
		t.Errorf("expected 3 rows, got %d", g.RowCount())
	}
}

func TestDataGrid_RendererWithStatusBar(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.ShowStatusBar = true

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	// Should not panic with status bar enabled
	if g.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", g.RowCount())
	}
}

func TestDataGrid_RendererWithPinnedColumns(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Width: 80, Pinned: PinLeft},
		{Field: "b", Width: 200},
		{Field: "c", Width: 80, Pinned: PinRight},
	})

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(500, 300))

	layout := g.visibleLayout()
	if len(layout.pinnedLeft) != 1 {
		t.Errorf("expected 1 pinned left col, got %d", len(layout.pinnedLeft))
	}
	if len(layout.pinnedRight) != 1 {
		t.Errorf("expected 1 pinned right col, got %d", len(layout.pinnedRight))
	}
}

func TestDataGrid_RendererWithGroupRows(t *testing.T) {
	data := []map[string]any{
		{"dept": "A", "name": "Alice"},
		{"dept": "B", "name": "Bob"},
		{"dept": "A", "name": "Charlie"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept"}, {Field: "name"},
	})

	g.API().SetRowGroupColumn("dept")

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	// Should render group headers + data rows
	if g.RowCount() < 5 {
		t.Errorf("expected at least 5 rows with groups, got %d", g.RowCount())
	}

	// Check group row detection
	if !g.IsGroupRow(0) {
		t.Error("expected row 0 to be a group row")
	}
}

func TestDataGrid_RendererWithCheckboxSelection(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.CheckboxSelection = true
	g.SelectionMode = SelectionMultiRow

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	if !g.ShowCheckboxColumn() {
		t.Error("expected checkbox column to be shown")
	}
}

func TestDataGrid_RendererWithRowAlternation(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"v": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.RowAlternation = true

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	if !g.IsAlternateRow(1) {
		t.Error("expected row 1 to be alternate")
	}
	if g.IsAlternateRow(0) {
		t.Error("expected row 0 to not be alternate")
	}
}

func TestDataGrid_RendererWithColSpan(t *testing.T) {
	data := []map[string]any{{"a": "spans two", "b": "hidden", "c": "normal"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", ColSpan: func(row int) int { return 2 }},
		{Field: "b"},
		{Field: "c"},
	})

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	span := g.ColSpanAt(0, 0)
	if span != 2 {
		t.Errorf("expected col span 2, got %d", span)
	}
}

func TestDataGrid_RendererWithPagination(t *testing.T) {
	data := make([]map[string]any, 50)
	for i := range data {
		data[i] = map[string]any{"v": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.ShowStatusBar = true

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 400))

	g.API().EnablePagination(10)

	if g.RowCount() != 10 {
		t.Errorf("expected 10 rows on page, got %d", g.RowCount())
	}
}

func TestDataGrid_FloatingFilterAccess(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}, {"name": "Bob"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})

	// Simulate floating filter change
	g.OnFloatingFilterChanged(0, "ali")
	if g.RowCount() != 1 {
		t.Errorf("expected 1 row after floating filter, got %d", g.RowCount())
	}

	// Clear
	g.OnFloatingFilterChanged(0, "")
	if g.RowCount() != 2 {
		t.Errorf("expected 2 rows after clearing floating filter, got %d", g.RowCount())
	}
}
