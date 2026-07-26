package export

import (
	"bytes"
	"strings"
	"testing"
)

type testData struct {
	rows []map[string]any
}

func (d *testData) RowCount() int       { return len(d.rows) }
func (d *testData) RowData(row int) any { return d.rows[row] }

func TestCSV_Basic(t *testing.T) {
	data := &testData{rows: []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
	}}

	cols := []Column{
		{Header: "Name", Field: "name"},
		{Header: "Age", Field: "age"},
	}

	var buf bytes.Buffer
	err := CSV(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d", len(lines))
	}
	if lines[0] != "Name,Age" {
		t.Errorf("expected header 'Name,Age', got %q", lines[0])
	}
	if lines[1] != "Alice,25" {
		t.Errorf("expected 'Alice,25', got %q", lines[1])
	}
}

func TestCSV_WithFormatter(t *testing.T) {
	data := &testData{rows: []map[string]any{
		{"price": 99.99},
	}}

	cols := []Column{
		{
			Header:    "Price",
			Field:     "price",
			Formatter: func(v any) string { return "$" + formatAny(v) },
		},
	}

	var buf bytes.Buffer
	err := CSV(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "$99.99") {
		t.Errorf("expected formatted price, got %q", output)
	}
}

func TestCSV_Empty(t *testing.T) {
	data := &testData{rows: nil}
	cols := []Column{{Header: "A", Field: "a"}}

	var buf bytes.Buffer
	err := CSV(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected header only, got %d lines", len(lines))
	}
}

func TestTSV_Basic(t *testing.T) {
	data := &testData{rows: []map[string]any{
		{"a": "x", "b": "y"},
	}}

	cols := []Column{
		{Header: "A", Field: "a"},
		{Header: "B", Field: "b"},
	}

	var buf bytes.Buffer
	err := TSV(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "A\tB") {
		t.Errorf("expected TSV header with tabs, got %q", output)
	}
	if !strings.Contains(output, "x\ty") {
		t.Errorf("expected TSV data with tabs, got %q", output)
	}
}

func TestCSV_WithValueGetter(t *testing.T) {
	data := &testData{rows: []map[string]any{
		{"nested": map[string]any{"val": "deep"}},
	}}

	cols := []Column{
		{
			Header: "Value",
			ValueGetter: func(row any) any {
				m := row.(map[string]any)
				if nested, ok := m["nested"].(map[string]any); ok {
					return nested["val"]
				}
				return nil
			},
		},
	}

	var buf bytes.Buffer
	err := CSV(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "deep") {
		t.Errorf("expected 'deep' in output, got %q", buf.String())
	}
}
