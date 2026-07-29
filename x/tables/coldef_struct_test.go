package tables

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

type structTestRow struct {
	Name string
	Age  int
	note string // unexported: must resolve to nil, not panic
}

func newStructTestGrid(t *testing.T, data []structTestRow, cols []*ColumnDef) *DataGrid {
	t.Helper()
	test.NewTempApp(t)
	return NewDataGrid(
		cols,
		func() int { return len(data) },
		func(row int) any { return data[row] },
	)
}

// ColumnDef.Field must resolve against struct rows, not only maps — a grid fed
// structs otherwise renders every cell blank and sorts/filters against all-nil
// values (silently, as goed's Trace tab demonstrated).
func TestDataGrid_StructRows_FieldAccess(t *testing.T) {
	data := []structTestRow{
		{Name: "Charlie", Age: 30},
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 35},
	}
	g := newStructTestGrid(t, data, []*ColumnDef{
		{Field: "Name"},
		{Field: "Age", Type: ColumnNumber},
	})

	if got := g.AccessibleCellText(0, 0); got != "Charlie" {
		t.Fatalf("cell(0,0) = %q, want Charlie", got)
	}

	g.API().SetSort(SortItem{ColID: "Age", Direction: SortAscending})
	wantSrc := []int{1, 0, 2} // Alice(25), Charlie(30), Bob(35)
	for disp, src := range wantSrc {
		if got := g.SourceRow(disp); got != src {
			t.Errorf("after numeric sort, SourceRow(%d) = %d, want %d", disp, got, src)
		}
	}

	g.API().ClearSort()
	g.API().SetQuickFilter("bob")
	if got := g.RowCount(); got != 1 {
		t.Fatalf("quick filter over struct rows kept %d rows, want 1", got)
	}
	if got := g.SourceRow(0); got != 2 {
		t.Fatalf("filtered view shows source %d, want 2 (Bob)", got)
	}
}

// Pointer rows resolve through the indirection; missing and unexported fields
// resolve to nil (blank cell) instead of panicking.
func TestDataGrid_StructRows_PointerAndEdgeFields(t *testing.T) {
	if got := defaultFieldAccess(&structTestRow{Name: "Ann"}, "Name"); got != "Ann" {
		t.Fatalf("pointer row Name = %v, want Ann", got)
	}
	if got := defaultFieldAccess((*structTestRow)(nil), "Name"); got != nil {
		t.Fatalf("nil pointer row = %v, want nil", got)
	}
	if got := defaultFieldAccess(structTestRow{}, "Missing"); got != nil {
		t.Fatalf("missing field = %v, want nil", got)
	}
	if got := defaultFieldAccess(structTestRow{note: "x"}, "note"); got != nil {
		t.Fatalf("unexported field = %v, want nil", got)
	}
	if got := defaultFieldAccess(42, "Name"); got != nil {
		t.Fatalf("non-struct row = %v, want nil", got)
	}
}
