package export

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestExcel_Basic(t *testing.T) {
	data := &testData{rows: []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30},
	}}

	cols := []Column{
		{Header: "Name", Field: "name"},
		{Header: "Age", Field: "age"},
	}

	var buf bytes.Buffer
	err := Excel(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's a valid zip (xlsx is a zip file)
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal("not a valid zip:", err)
	}

	// Check for required files
	expectedFiles := map[string]bool{
		"[Content_Types].xml":        false,
		"_rels/.rels":                false,
		"xl/workbook.xml":            false,
		"xl/_rels/workbook.xml.rels": false,
		"xl/worksheets/sheet1.xml":   false,
		"xl/sharedStrings.xml":       false,
		"xl/styles.xml":              false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("missing required file in xlsx: %s", name)
		}
	}
}

func TestExcel_Empty(t *testing.T) {
	data := &testData{rows: nil}
	cols := []Column{{Header: "A", Field: "a"}}

	var buf bytes.Buffer
	err := Excel(&buf, cols, data)
	if err != nil {
		t.Fatal(err)
	}

	// Should still be a valid zip
	_, err = zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal("empty data should still produce valid xlsx:", err)
	}
}

func TestCellRef(t *testing.T) {
	tests := []struct {
		row, col int
		expected string
	}{
		{0, 0, "A1"},
		{0, 1, "B1"},
		{0, 25, "Z1"},
		{0, 26, "AA1"},
		{1, 0, "A2"},
		{9, 2, "C10"},
	}
	for _, tc := range tests {
		got := cellRef(tc.row, tc.col)
		if got != tc.expected {
			t.Errorf("cellRef(%d,%d) = %q, want %q", tc.row, tc.col, got, tc.expected)
		}
	}
}
