package export

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"
)

type sliceData []map[string]any

func (d sliceData) RowCount() int       { return len(d) }
func (d sliceData) RowData(row int) any { return d[row] }

// datasetPart returns the document lines from the @dataset header on,
// skipping the embedded @proto schema block.
func datasetPart(t *testing.T, out string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "@dataset ") {
			return lines[i:]
		}
	}
	t.Fatalf("no @dataset header in output %q", out)
	return nil
}

func TestPXF_Header(t *testing.T) {
	cols := []Column{
		{Header: "Name", Field: "name", Type: FieldString},
		{Header: "Age", Field: "age", Type: FieldInt64},
	}
	var buf bytes.Buffer
	err := PXF(&buf, "users.v1.User", cols, sliceData{})
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// The schema comes from the declarations alone — the dataset is
	// empty, yet age is int64.
	want := `@proto """
syntax = "proto3";
package users.v1;

message User {
  string name = 1;
  int64 age = 2;
}
"""
@dataset users.v1.User (name, age)
`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPXF_TypedLiterals(t *testing.T) {
	ts := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	cols := []Column{
		{Field: "s", Type: FieldString},
		{Field: "i", Type: FieldInt64},
		{Field: "f", Type: FieldDouble},
		{Field: "b", Type: FieldBool},
		{Field: "t", Type: FieldTimestamp},
		{Field: "d", Type: FieldDuration},
		{Field: "raw", Type: FieldBytes},
	}
	data := sliceData{{
		"s": "hello", "i": 42, "f": 3.14, "b": true,
		"t": ts, "d": 90 * time.Minute, "raw": []byte{0x01, 0x02},
	}}
	var buf bytes.Buffer
	if err := PXF(&buf, "test.v1.AllTypes", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	want := `("hello", 42, 3.14, true, 2026-05-11T10:00:00Z, 1h30m0s, b"AQI=")`
	if lines[1] != want {
		t.Errorf("row = %q, want %q", lines[1], want)
	}
}

func TestPXF_NilIsEmptyCell(t *testing.T) {
	cols := []Column{
		{Field: "a", Type: FieldString},
		{Field: "b", Type: FieldString},
		{Field: "c", Type: FieldString},
	}
	data := sliceData{
		{"a": "x", "c": "z"},           // b missing entirely
		{"a": nil, "b": "y", "c": nil}, // explicit nils at row start and end
	}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if lines[1] != `("x", , "z")` {
		t.Errorf("row 1 = %q", lines[1])
	}
	if lines[2] != `(, "y", )` {
		t.Errorf("row 2 = %q", lines[2])
	}
}

func TestPXF_SingleColumnNilRow(t *testing.T) {
	cols := []Column{{Field: "a", Type: FieldString}}
	data := sliceData{{"other": 1}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if lines[1] != "()" {
		t.Errorf("row = %q, want ()", lines[1])
	}
}

func TestPXF_StringEscaping(t *testing.T) {
	cols := []Column{{Field: "s", Type: FieldString}}
	data := sliceData{{"s": "a\"b\\c\nd\te\x01"}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	want := `("a\"b\\c\nd\te\x01")`
	if lines[1] != want {
		t.Errorf("row = %q, want %q", lines[1], want)
	}
}

func TestPXF_FloatSpecials(t *testing.T) {
	cols := []Column{
		{Field: "a", Type: FieldDouble},
		{Field: "b", Type: FieldDouble},
		{Field: "c", Type: FieldDouble},
	}
	data := sliceData{{"a": math.Inf(1), "b": math.Inf(-1), "c": math.NaN()}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if lines[1] != "(inf, -inf, nan)" {
		t.Errorf("row = %q", lines[1])
	}
}

func TestPXF_IntWidths(t *testing.T) {
	cols := []Column{
		{Field: "i", Type: FieldInt64},
		{Field: "u", Type: FieldUint64},
		{Field: "f", Type: FieldDouble},
	}
	data := sliceData{{"i": int8(-3), "u": uint16(7), "f": float32(1.5)}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if lines[1] != "(-3, 7, 1.5)" {
		t.Errorf("row = %q", lines[1])
	}
}

func TestPXF_FormatterEmitsQuotedString(t *testing.T) {
	cols := []Column{{
		Field:     "price",
		Formatter: func(v any) string { return "$42.00" },
	}}
	data := sliceData{{"price": 42.0}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if lines[1] != `("$42.00")` {
		t.Errorf("row = %q", lines[1])
	}
}

func TestPXF_DeclaredTypeOverridesFormatter(t *testing.T) {
	// An explicit non-string declaration wins over the Formatter: the
	// raw typed value is exported and the schema says double — the
	// formatter stays a display concern.
	cols := []Column{{
		Field:     "price",
		Type:      FieldDouble,
		Formatter: func(v any) string { return "$42.00" },
	}}
	data := sliceData{{"price": 42.5}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	if schema := schemaPart(t, buf.String()); !strings.Contains(schema, "double price = 1;") {
		t.Errorf("declared type should govern the schema:\n%s", schema)
	}
	if lines := datasetPart(t, buf.String()); lines[1] != `(42.5)` {
		t.Errorf("row = %q, want raw typed value (42.5)", lines[1])
	}
}

func TestPXF_ValueGetter(t *testing.T) {
	type rec struct{ Name string }
	cols := []Column{{
		Field:       "name",
		Type:        FieldString,
		ValueGetter: func(rowData any) any { return rowData.(rec).Name },
	}}
	data := structData{rec{Name: "alice"}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `("alice")`) {
		t.Errorf("output = %q", buf.String())
	}
}

type structData []any

func (d structData) RowCount() int       { return len(d) }
func (d structData) RowData(row int) any { return d[row] }

func TestPXF_Validation(t *testing.T) {
	data := sliceData{}
	cases := []struct {
		name    string
		msgType string
		cols    []Column
	}{
		{"empty type", "", []Column{{Field: "a", Type: FieldString}}},
		{"bad type segment", "users..User", []Column{{Field: "a", Type: FieldString}}},
		{"type starts with digit", "users.1v.User", []Column{{Field: "a", Type: FieldString}}},
		{"no columns", "t.v1.T", nil},
		{"empty field", "t.v1.T", []Column{{Field: "", Type: FieldString}}},
		{"dotted field", "t.v1.T", []Column{{Field: "addr.city", Type: FieldString}}},
		{"reserved field", "t.v1.T", []Column{{Field: "null", Type: FieldString}}},
		{"duplicate field", "t.v1.T", []Column{{Field: "a", Type: FieldString}, {Field: "a", Type: FieldString}}},
		{"no field type", "t.v1.T", []Column{{Field: "a"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := PXF(&buf, tc.msgType, tc.cols, data); err == nil {
				t.Errorf("expected error, got output %q", buf.String())
			}
		})
	}
}

func TestPXF_ValueTypeMismatchErrors(t *testing.T) {
	cases := []struct {
		name  string
		kind  FieldType
		value any
	}{
		{"string field, int value", FieldString, 42},
		{"int64 field, string value", FieldInt64, "42"},
		{"int64 field, unsigned value", FieldInt64, uint(42)},
		{"uint64 field, signed value", FieldUint64, 42},
		{"double field, int value", FieldDouble, 42},
		{"bool field, string value", FieldBool, "true"},
		{"timestamp field, string value", FieldTimestamp, "2026-05-11T10:00:00Z"},
		{"duration field, int value", FieldDuration, 90},
		{"bytes field, string value", FieldBytes, "AQI="},
		{"string field, stringer value", FieldString, time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cols := []Column{{Field: "a", Type: tc.kind}}
			data := sliceData{{"a": tc.value}}
			var buf bytes.Buffer
			err := PXF(&buf, "t.v1.T", cols, data)
			if err == nil {
				t.Fatalf("expected error, got output %q", buf.String())
			}
			for _, want := range []string{"row 0", `column "a"`, tc.kind.String()} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q missing %q", err, want)
				}
			}
		})
	}
}

func TestPXF_SkipsNilRows(t *testing.T) {
	cols := []Column{{Field: "a", Type: FieldString}}
	data := nilRowData{}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	lines := datasetPart(t, buf.String())
	if len(lines) != 2 {
		t.Errorf("expected header + 1 row, got %d lines: %v", len(lines), lines)
	}
}

// schemaPart returns the document lines up to (excluding) the
// @dataset header — the embedded @proto block.
func schemaPart(t *testing.T, out string) string {
	t.Helper()
	i := strings.Index(out, "@dataset ")
	if i < 0 {
		t.Fatalf("no @dataset header in output %q", out)
	}
	return out[:i]
}

func TestPXF_DeclaredSchema(t *testing.T) {
	cols := []Column{
		{Field: "s", Type: FieldString},
		{Field: "i", Type: FieldInt64},
		{Field: "u", Type: FieldUint64},
		{Field: "f", Type: FieldDouble},
		{Field: "b", Type: FieldBool},
		{Field: "t", Type: FieldTimestamp},
		{Field: "d", Type: FieldDuration},
		{Field: "raw", Type: FieldBytes},
	}
	// No rows at all: the schema (including well-known-type imports)
	// comes entirely from the declarations.
	var buf bytes.Buffer
	if err := PXF(&buf, "test.v1.AllTypes", cols, sliceData{}); err != nil {
		t.Fatal(err)
	}
	want := `@proto """
syntax = "proto3";
package test.v1;
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

message AllTypes {
  string s = 1;
  int64 i = 2;
  uint64 u = 3;
  double f = 4;
  bool b = 5;
  google.protobuf.Timestamp t = 6;
  google.protobuf.Duration d = 7;
  bytes raw = 8;
}
"""
`
	if got := schemaPart(t, buf.String()); got != want {
		t.Errorf("schema = %q, want %q", got, want)
	}
}

func TestPXF_SchemaFormatterIsString(t *testing.T) {
	cols := []Column{{
		Field:     "price",
		Formatter: func(v any) string { return "$42.00" },
	}}
	data := sliceData{{"price": 42.0}}
	var buf bytes.Buffer
	if err := PXF(&buf, "t.v1.T", cols, data); err != nil {
		t.Fatal(err)
	}
	if schema := schemaPart(t, buf.String()); !strings.Contains(schema, "string price = 1;") {
		t.Errorf("formatter column should be string:\n%s", schema)
	}
}

func TestPXF_SchemaUnqualifiedType(t *testing.T) {
	cols := []Column{{Field: "a", Type: FieldString}}
	var buf bytes.Buffer
	if err := PXF(&buf, "Trade", cols, sliceData{}); err != nil {
		t.Fatal(err)
	}
	schema := schemaPart(t, buf.String())
	if strings.Contains(schema, "package") {
		t.Errorf("unqualified type must not emit a package line:\n%s", schema)
	}
	if !strings.Contains(schema, "message Trade {") {
		t.Errorf("schema missing message declaration:\n%s", schema)
	}
}

type nilRowData struct{}

func (nilRowData) RowCount() int { return 2 }
func (nilRowData) RowData(row int) any {
	if row == 0 {
		return nil
	}
	return map[string]any{"a": "x"}
}
