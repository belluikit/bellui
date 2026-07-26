package tables

import "testing"

func TestDataGrid_RowAlternation(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}, {"v": 3}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.RowAlternation = true

	if g.IsAlternateRow(0) {
		t.Error("row 0 should not be alternate")
	}
	if !g.IsAlternateRow(1) {
		t.Error("row 1 should be alternate")
	}
	if g.IsAlternateRow(2) {
		t.Error("row 2 should not be alternate")
	}
}

func TestDataGrid_RowAlternationDisabled(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	// RowAlternation defaults to false

	if g.IsAlternateRow(1) {
		t.Error("should not be alternate when disabled")
	}
}

func TestDataGrid_CellTooltip(t *testing.T) {
	data := []map[string]any{{"name": "Alice", "detail": "Senior Engineer"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{
			Field:         "name",
			TooltipGetter: func(row any) string { return row.(map[string]any)["detail"].(string) },
		},
	})

	tip := g.CellTooltip(0, 0)
	if tip != "Senior Engineer" {
		t.Errorf("expected tooltip 'Senior Engineer', got %q", tip)
	}
}

func TestDataGrid_CellTooltipNone(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	tip := g.CellTooltip(0, 0)
	if tip != "" {
		t.Errorf("expected empty tooltip, got %q", tip)
	}
}

func TestDataGrid_ShowHideLoading(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	g.ShowLoading("Computing...")
	if g.loadingOverlay == nil {
		t.Error("expected loading overlay to be created")
	}
	if g.loadingOverlay.Message != "Computing..." {
		t.Errorf("expected message 'Computing...', got %q", g.loadingOverlay.Message)
	}

	g.HideLoading()
}

func TestDataGrid_MoveRow(t *testing.T) {
	data := []map[string]any{{"v": "a"}, {"v": "b"}, {"v": "c"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	var fromSrc, toSrc int
	called := false
	g.SetOnRowMoved(func(from, to int) {
		called = true
		fromSrc = from
		toSrc = to
	})

	g.MoveRow(0, 2)
	if !called {
		t.Error("expected OnRowMoved callback")
	}
	if fromSrc != 0 || toSrc != 2 {
		t.Errorf("expected from=0, to=2, got from=%d, to=%d", fromSrc, toSrc)
	}
}

func TestDataGrid_DensityCompact(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	// Default density is comfortable
	defaultH := g.rowHeight()

	// Switch to compact
	g.Options.Density = 1 // DensityCompact
	compactH := g.rowHeight()

	if compactH >= defaultH {
		t.Errorf("expected compact height < comfortable height, got %f >= %f", compactH, defaultH)
	}
}

func TestDataGrid_DensitySpacious(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})

	defaultH := g.rowHeight()

	g.Options.Density = 2 // DensitySpacious
	spaciousH := g.rowHeight()

	if spaciousH <= defaultH {
		t.Errorf("expected spacious height > comfortable height, got %f <= %f", spaciousH, defaultH)
	}
}
