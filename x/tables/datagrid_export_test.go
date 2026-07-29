package tables

import (
	"bytes"
	"strings"
	"testing"

	"github.com/belluikit/bellui/x/tables/export"
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

func TestDataGrid_ExportPXF(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber, ExportType: export.FieldInt64},
	})

	var buf bytes.Buffer
	if err := g.API().ExportPXF(&buf, "people.v1.Person"); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "message Person {") ||
		!strings.Contains(out, "string name = 1;") ||
		!strings.Contains(out, "int64 age = 2;") {
		t.Errorf("missing embedded @proto schema in %q", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	idx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "@dataset ") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no @dataset header in %q", out)
	}
	lines = lines[idx:]
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d: %v", len(lines), lines)
	}
	if lines[0] != "@dataset people.v1.Person (name, age)" {
		t.Errorf("header = %q", lines[0])
	}
	if lines[1] != `("Alice", 25)` {
		t.Errorf("row 1 = %q", lines[1])
	}
}

func TestDataGrid_ExportPXFExportTypeOverridesFormatter(t *testing.T) {
	data := []map[string]any{{"price": 42.5}}
	g := newTestGrid(t, data, []*ColumnDef{
		{
			Field: "price", HeaderName: "Price", Type: ColumnNumber,
			ExportType:     export.FieldDouble,
			ValueFormatter: func(v any) string { return "$42.50" },
		},
	})

	var buf bytes.Buffer
	if err := g.API().ExportPXF(&buf, "shop.v1.Item"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "double price = 1;") {
		t.Errorf("schema should use the declared type: %q", out)
	}
	if !strings.Contains(out, "(42.5)") || strings.Contains(out, "$42.50") {
		t.Errorf("PXF should export the raw typed value, not the formatted string: %q", out)
	}
}

func TestDataGrid_ExportPXFRequiresDeclaredNumberType(t *testing.T) {
	data := []map[string]any{{"name": "Alice", "age": 25}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		// ColumnNumber has no unambiguous field type and no
		// ExportType is declared — PXF must refuse, not infer.
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	var buf bytes.Buffer
	err := g.API().ExportPXF(&buf, "people.v1.Person")
	if err == nil {
		t.Fatalf("expected error for undeclared number column, got output %q", buf.String())
	}
	if !strings.Contains(err.Error(), `"age"`) {
		t.Errorf("error should name the column: %q", err)
	}
}

func TestDataGrid_ExportPXFHonorsFilter(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber, ExportType: export.FieldInt64},
	})
	g.API().SetColumnFilter("age", func(v any) bool {
		n, _ := v.(int)
		return n >= 30
	})

	var buf bytes.Buffer
	if err := g.API().ExportPXF(&buf, "people.v1.Person"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "Alice") {
		t.Errorf("filtered row leaked into export: %q", out)
	}
	if !strings.Contains(out, `("Bob", 30)`) {
		t.Errorf("expected Bob row, got %q", out)
	}
}

func TestDataGrid_ExportPDF(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	var buf bytes.Buffer
	err := g.API().ExportPDF(&buf, export.PDFOptions{Title: "People"})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "%PDF-1.4\n") || !strings.HasSuffix(out, "%%EOF\n") {
		t.Fatalf("not a well-formed PDF: %d bytes", len(out))
	}
	for _, want := range []string{"(People)", "(Name)", "(Alice)", "(Bob)"} {
		if !strings.Contains(out, want) {
			t.Errorf("PDF content missing %q", want)
		}
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
