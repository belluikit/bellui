package tables

import (
	"bytes"
	"strings"
	"testing"
)

func TestDataGrid_ExportCSV(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
		{"name": "Charlie", "age": 35},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	var buf bytes.Buffer
	err := g.API().ExportCSV(&buf)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (header + 3 rows), got %d: %v", len(lines), lines)
	}
	if lines[0] != "Name,Age" {
		t.Errorf("expected header 'Name,Age', got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "Alice,") {
		t.Errorf("expected first data row to start with 'Alice', got %q", lines[1])
	}
}

func TestDataGrid_ExportCSVWithFilter(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
		{"name": "Charlie", "age": 35},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	// Filter to age >= 30
	g.API().SetColumnFilter("age", func(v any) bool {
		n, _ := v.(int)
		return n >= 30
	})

	var buf bytes.Buffer
	err := g.API().ExportCSV(&buf)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (header + 2 filtered rows), got %d", len(lines))
	}
}

func TestDataGrid_ExportCSVWithSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"},
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
	})

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	var buf bytes.Buffer
	_ = g.API().ExportCSV(&buf)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[1] != "Alice" {
		t.Errorf("expected sorted: Alice first, got %q", lines[1])
	}
}

func TestDataGrid_ExportTSV(t *testing.T) {
	data := []map[string]any{{"x": "a"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "x", HeaderName: "X"},
	})

	var buf bytes.Buffer
	err := g.API().ExportTSV(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "X\n") || !strings.Contains(buf.String(), "a\n") {
		t.Errorf("expected TSV output, got %q", buf.String())
	}
}

func TestDataGrid_ExportHiddenColumn(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", HeaderName: "A"},
		{Field: "b", HeaderName: "B", Hidden: true},
	})

	var buf bytes.Buffer
	_ = g.API().ExportCSV(&buf)

	header := strings.Split(strings.TrimSpace(buf.String()), "\n")[0]
	if header != "A" {
		t.Errorf("expected only visible column 'A' in export, got %q", header)
	}
}
