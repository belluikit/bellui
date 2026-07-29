package export

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func pdfTestData(n int) sliceData {
	d := make(sliceData, n)
	for i := range d {
		d[i] = map[string]any{"name": fmt.Sprintf("Row %d", i), "value": i}
	}
	return d
}

var pdfTestCols = []Column{
	{Header: "Name", Field: "name"},
	{Header: "Value", Field: "value"},
}

func TestPDF_ValidStructure(t *testing.T) {
	var buf bytes.Buffer
	if err := PDF(&buf, pdfTestCols, pdfTestData(3), PDFOptions{Title: "My Report"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "%PDF-1.4\n") {
		t.Errorf("missing PDF header, got %q", out[:20])
	}
	if !strings.HasSuffix(out, "%%EOF\n") {
		t.Errorf("missing EOF marker, got %q", out[len(out)-20:])
	}
	for _, want := range []string{"/Type /Catalog", "/Type /Pages", "/Type /Page ",
		"/BaseFont /Helvetica", "/BaseFont /Helvetica-Bold", "xref", "trailer", "startxref"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
	// Content streams are uncompressed — title, headers, data, and the
	// page footer should appear as literal text.
	for _, want := range []string{"(My Report)", "(Name)", "(Value)", "(Row 0)", "(Page 1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("content missing %q", want)
		}
	}
}

func TestPDF_XrefOffsets(t *testing.T) {
	var buf bytes.Buffer
	if err := PDF(&buf, pdfTestCols, pdfTestData(5), PDFOptions{}); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	// startxref points at the xref table.
	s := string(out)
	i := strings.LastIndex(s, "startxref\n")
	if i < 0 {
		t.Fatal("no startxref")
	}
	rest := s[i+len("startxref\n"):]
	off, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(rest, "%%EOF\n")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s[off:], "xref\n") {
		t.Errorf("startxref %d does not point at xref table (found %q)", off, s[off:off+10])
	}
	// Every in-use xref entry points at "<n> 0 obj".
	xref := s[off:]
	lines := strings.Split(xref, "\n")
	for n, line := range lines[2:] { // skip "xref", "0 N", then entries
		if !strings.HasSuffix(line, " n ") {
			break
		}
		objOff, _ := strconv.Atoi(strings.Fields(line)[0])
		want := fmt.Sprintf("%d 0 obj", n+1)
		if !strings.HasPrefix(s[objOff:], want) {
			t.Errorf("xref entry %d: offset %d points at %q, want %q", n+1, objOff, s[objOff:objOff+12], want)
		}
	}
}

func TestPDF_PaginationRepeatsHeaders(t *testing.T) {
	var buf bytes.Buffer
	// 200 rows at default 9pt on A4 must span multiple pages.
	if err := PDF(&buf, pdfTestCols, pdfTestData(200), PDFOptions{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	pages := strings.Count(out, "/Type /Page ")
	if pages < 3 {
		t.Errorf("expected >= 3 pages for 200 rows, got %d", pages)
	}
	if got := strings.Count(out, "(Name)"); got != pages {
		t.Errorf("header row on %d of %d pages", got, pages)
	}
	if !strings.Contains(out, fmt.Sprintf("(Page %d)", pages)) {
		t.Errorf("missing footer for last page %d", pages)
	}
	if !strings.Contains(out, "(Row 199)") {
		t.Error("last row missing")
	}
}

func TestPDF_RowCap(t *testing.T) {
	var buf bytes.Buffer
	if err := PDF(&buf, pdfTestCols, pdfTestData(50), PDFOptions{MaxRows: 10}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "(Row 9)") {
		t.Error("row 9 should be included")
	}
	if strings.Contains(out, "(Row 10)") {
		t.Error("row 10 should be capped")
	}
	if !strings.Contains(out, "truncated at 10 rows \\(50 rows in grid\\)") {
		t.Errorf("missing truncation notice, got: %s", out)
	}
}

func TestPDF_UnlimitedRows(t *testing.T) {
	var buf bytes.Buffer
	if err := PDF(&buf, pdfTestCols, pdfTestData(120), PDFOptions{MaxRows: -1}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "(Row 119)") {
		t.Error("all rows should be included with MaxRows < 0")
	}
	if strings.Contains(out, "truncated") {
		t.Error("no truncation notice expected")
	}
}

func TestPDF_TextEscaping(t *testing.T) {
	data := sliceData{{"name": "paren (x) \\ back", "value": 1}}
	var buf bytes.Buffer
	if err := PDF(&buf, pdfTestCols, data, PDFOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `(paren \(x\) \\ back)`) {
		t.Error("PDF string escaping missing")
	}
}

func TestPDF_LandscapeLetter(t *testing.T) {
	var buf bytes.Buffer
	err := PDF(&buf, pdfTestCols, pdfTestData(1), PDFOptions{PageSize: PageLetter, Landscape: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "/MediaBox [0 0 792.00 612.00]") {
		t.Error("expected landscape letter media box")
	}
}

func TestPDF_NoColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := PDF(&buf, nil, pdfTestData(1), PDFOptions{}); err == nil {
		t.Error("expected error for zero columns")
	}
}

func TestPDF_Formatter(t *testing.T) {
	cols := []Column{
		{Header: "V", Field: "value", Formatter: func(v any) string { return fmt.Sprintf("$%v.00", v) }},
	}
	var buf bytes.Buffer
	if err := PDF(&buf, cols, sliceData{{"value": 5}}, PDFOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "($5.00)") {
		t.Error("formatter output missing")
	}
}

func TestFitText(t *testing.T) {
	full := "a long string that will not fit"
	got := fitText(full, fontRegular, 9, 40)
	if got == full {
		t.Fatal("expected truncation")
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if w := textWidth(got, fontRegular, 9); w > 40 {
		t.Errorf("truncated width %.1f > 40", w)
	}
	if got := fitText("ok", fontRegular, 9, 40); got != "ok" {
		t.Errorf("short string should be untouched, got %q", got)
	}
}
