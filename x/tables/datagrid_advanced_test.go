package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
)

func TestDataGrid_Pagination(t *testing.T) {
	data := make([]map[string]any, 25)
	for i := range data {
		data[i] = map[string]any{"i": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "i", Type: ColumnNumber}})

	g.API().EnablePagination(10)

	if !g.IsPaginated() {
		t.Fatal("expected pagination enabled")
	}
	if g.RowCount() != 10 {
		t.Errorf("expected 10 rows on page 0, got %d", g.RowCount())
	}
	if g.PageCount() != 3 {
		t.Errorf("expected 3 pages, got %d", g.PageCount())
	}

	// Go to page 1
	g.API().SetPage(1)
	if g.CurrentPage() != 1 {
		t.Errorf("expected page 1, got %d", g.CurrentPage())
	}
	if g.RowCount() != 10 {
		t.Errorf("expected 10 rows on page 1, got %d", g.RowCount())
	}

	// Go to last page
	g.API().SetPage(2)
	if g.RowCount() != 5 {
		t.Errorf("expected 5 rows on last page, got %d", g.RowCount())
	}

	// Disable pagination
	g.API().DisablePagination()
	if g.IsPaginated() {
		t.Error("expected pagination disabled")
	}
	if g.RowCount() != 25 {
		t.Errorf("expected all 25 rows after disabling pagination, got %d", g.RowCount())
	}
}

func TestDataGrid_PaginationWithFilter(t *testing.T) {
	data := make([]map[string]any, 50)
	for i := range data {
		data[i] = map[string]any{"i": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "i", Type: ColumnNumber}})

	// Filter to even numbers only (25 rows)
	g.API().SetColumnFilter("i", func(v any) bool {
		n, _ := v.(int)
		return n%2 == 0
	})

	// Enable pagination
	g.API().EnablePagination(10)

	if g.RowCount() != 10 {
		t.Errorf("expected 10 rows on filtered+paginated view, got %d", g.RowCount())
	}
}

func TestDataGrid_Grouping(t *testing.T) {
	data := []map[string]any{
		{"dept": "Sales", "name": "Alice"},
		{"dept": "Eng", "name": "Bob"},
		{"dept": "Sales", "name": "Charlie"},
		{"dept": "Eng", "name": "Diana"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept"}, {Field: "name"},
	})

	g.API().SetRowGroupColumn("dept")

	// Should have group headers + data rows
	// 2 groups * (1 header + children) = 2 + 4 = 6
	if g.RowCount() != 6 {
		t.Errorf("expected 6 rows (2 group headers + 4 data), got %d", g.RowCount())
	}
}

func TestDataGrid_GroupToggle(t *testing.T) {
	data := []map[string]any{
		{"dept": "A", "name": "Alice"},
		{"dept": "B", "name": "Bob"},
		{"dept": "A", "name": "Charlie"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept"}, {Field: "name"},
	})

	g.API().SetRowGroupColumn("dept")

	// Initially all expanded: 2 headers + 3 data = 5
	if g.RowCount() != 5 {
		t.Fatalf("expected 5 rows, got %d", g.RowCount())
	}

	// Collapse group A
	g.API().ToggleGroup("A")
	// Now: A header (collapsed) + B header + Bob = 3
	if g.RowCount() != 3 {
		t.Errorf("expected 3 rows after collapsing A, got %d", g.RowCount())
	}

	// Expand A again
	g.API().ToggleGroup("A")
	if g.RowCount() != 5 {
		t.Errorf("expected 5 rows after expanding A, got %d", g.RowCount())
	}
}

func TestDataGrid_KeyboardTogglesFocusedGroup(t *testing.T) {
	data := []map[string]any{
		{"dept": "A", "name": "Alice"},
		{"dept": "B", "name": "Bob"},
		{"dept": "A", "name": "Charlie"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "dept"}, {Field: "name"}})
	g.API().SetRowGroupColumn("dept")

	if !g.IsGroupRow(0) {
		t.Fatal("expected row 0 to be a group header")
	}
	full := g.RowCount() // 2 headers + 3 data = 5

	// Put the keyboard cursor on the group header (as arrow navigation would:
	// a group row has no source, so focusSrcRow is -1).
	g.currentFocus = CellID{Row: 0, Col: 0}
	g.focusSrcRow = g.sourceRow(0)

	// Enter collapses the focused group.
	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if got := g.RowCount(); got >= full {
		t.Fatalf("Enter on group header did not collapse it: rows=%d (was %d)", got, full)
	}
	if !g.IsGroupRow(0) {
		t.Fatal("cursor should still be on the group header after collapse")
	}

	// Space re-expands it.
	g.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	if got := g.RowCount(); got != full {
		t.Fatalf("Space did not re-expand the group: rows=%d, want %d", got, full)
	}

	// Enter on a non-group (data) row must not change the row count (it selects).
	dataRow := -1
	for r := 0; r < g.RowCount(); r++ {
		if !g.IsGroupRow(r) {
			dataRow = r
			break
		}
	}
	if dataRow < 0 {
		t.Fatal("no data row found")
	}
	g.currentFocus = CellID{Row: dataRow, Col: 0}
	g.focusSrcRow = g.sourceRow(dataRow)
	before := g.RowCount()
	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if got := g.RowCount(); got != before {
		t.Errorf("Enter on a data row changed row count %d -> %d (should only select)", before, got)
	}
}

func TestDataGrid_ClearGrouping(t *testing.T) {
	data := []map[string]any{
		{"dept": "A"}, {"dept": "B"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "dept"}})

	g.API().SetRowGroupColumn("dept")
	if g.RowCount() != 4 { // 2 headers + 2 data
		t.Fatalf("expected 4 rows with grouping, got %d", g.RowCount())
	}

	g.API().SetRowGroupColumn("") // clear
	if g.RowCount() != 2 {
		t.Errorf("expected 2 rows after clearing grouping, got %d", g.RowCount())
	}
}

func TestDataGrid_GroupWithSort(t *testing.T) {
	data := []map[string]any{
		{"dept": "B", "name": "Zara"},
		{"dept": "A", "name": "Alice"},
		{"dept": "B", "name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept"}, {Field: "name"},
	})

	g.API().SetRowGroupColumn("dept")
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	// With grouping, sort applies within groups
	// Group order depends on first occurrence: B first, then A
	// But sort should reorder within the group
	if g.RowCount() < 5 {
		t.Errorf("expected at least 5 rows, got %d", g.RowCount())
	}
}
