package tables

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

// With RowHeightFunc set, the viewport gets per-display-row overrides resolved
// through source identity, and total content height reflects them.
func TestDataGrid_VariableRowHeightPopulation(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0 — tall
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	// Only source row 0 is tall; others use the default.
	g.Options.RowHeightFunc = func(sourceRow int) float32 {
		if sourceRow == 0 {
			return 90
		}
		return 0
	}
	g.RefreshData()

	rh := g.viewport.RowHeights
	if len(rh) != 1 {
		t.Fatalf("expected 1 override, got %d: %v", len(rh), rh)
	}
	if rh[0] != 90 {
		t.Errorf("display row 0 height = %v, want 90", rh[0])
	}

	// RowY reflects the tall first row: row 1 starts at 90 + padding.
	pad := g.cellPadding()
	def := g.rowHeight()
	if y, h := g.viewport.RowY(0); y != 0 || h != 90 {
		t.Errorf("RowY(0) = (%v,%v), want (0,90)", y, h)
	}
	if y, _ := g.viewport.RowY(1); y != 90+pad {
		t.Errorf("RowY(1).y = %v, want %v", y, 90+pad)
	}
	wantTotal := (90 + pad) + 2*(def+pad)
	if got := g.viewport.ContentSize().Height; got != wantTotal {
		t.Errorf("ContentSize().Height = %v, want %v", got, wantTotal)
	}
}

// Heights are keyed by SOURCE row, so a tall row keeps its height at its new
// display position after a sort.
func TestDataGrid_VariableRowHeightFollowsSourceAcrossSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0 — tall
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.Options.RowHeightFunc = func(sourceRow int) float32 {
		if sourceRow == 0 {
			return 90
		}
		return 0
	}
	g.RefreshData()

	// Before sort: Charlie (source 0) is display row 0.
	if g.viewport.RowHeights[0] != 90 {
		t.Fatalf("pre-sort: expected tall display row 0, got %v", g.viewport.RowHeights)
	}

	// Sort ascending: Charlie moves to display row 2.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	rh := g.viewport.RowHeights
	if len(rh) != 1 || rh[2] != 90 {
		t.Errorf("post-sort: expected tall display row 2, got %v", rh)
	}
	if _, h := g.viewport.RowY(2); h != 90 {
		t.Errorf("post-sort: RowY(2) height = %v, want 90", h)
	}
}

// Hit-testing must land on the correct row with variable heights.
func TestDataGrid_VariableRowHeightHitTest(t *testing.T) {
	data := []map[string]any{{"name": "Charlie"}, {"name": "Alice"}, {"name": "Bob"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.Options.RowHeightFunc = func(sourceRow int) float32 {
		if sourceRow == 0 {
			return 90
		}
		return 0
	}
	g.RefreshData()

	headerH := g.HeaderHeight()
	pad := g.cellPadding()
	def := g.rowHeight()

	// A Y inside the tall first row (display 0, spans 0..90 in content space).
	if id := g.cellAtPosition(geo.NewPos(10, headerH+80)); id.Row != 0 {
		t.Errorf("hit inside tall row 0: got row %d, want 0", id.Row)
	}
	// A Y inside the second row (starts at 90+pad in content space).
	secondRowY := headerH + (90 + pad) + def/2
	if id := g.cellAtPosition(geo.NewPos(10, secondRowY)); id.Row != 1 {
		t.Errorf("hit inside row 1: got row %d, want 1", id.Row)
	}
}

// Without RowHeightFunc the grid stays uniform — no overrides, no prefix sum.
func TestDataGrid_UniformWhenNoRowHeightFunc(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}, {"name": "C"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.RefreshData()

	if g.viewport.RowHeights != nil {
		t.Errorf("expected no row-height overrides, got %v", g.viewport.RowHeights)
	}
	pitch := g.rowHeight() + g.cellPadding()
	if y, _ := g.viewport.RowY(2); y != 2*pitch {
		t.Errorf("uniform RowY(2).y = %v, want %v", y, 2*pitch)
	}
}
