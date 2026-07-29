package export

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// FieldType declares the protobuf field type of an exported column.
// PXF requires a declared type for every column — field types are
// never inferred from cell values, so the emitted schema is exactly
// what the caller declared, and a cell that does not match it is an
// error rather than a silent coercion.
type FieldType int

const (
	// FieldUnspecified is the zero value: no type declared. PXF
	// rejects it, except on Formatter columns, which are FieldString
	// by definition (formatted cells always emit as quoted strings).
	FieldUnspecified FieldType = iota
	// FieldString accepts string cell values.
	FieldString
	// FieldBool accepts bool cell values.
	FieldBool
	// FieldInt64 accepts int, int8, int16, int32, and int64 cell values.
	FieldInt64
	// FieldUint64 accepts uint, uint8, uint16, uint32, and uint64 cell values.
	FieldUint64
	// FieldDouble accepts float32 and float64 cell values.
	FieldDouble
	// FieldBytes accepts []byte cell values.
	FieldBytes
	// FieldTimestamp is google.protobuf.Timestamp; accepts time.Time cell values.
	FieldTimestamp
	// FieldDuration is google.protobuf.Duration; accepts time.Duration cell values.
	FieldDuration
)

// String returns the protobuf type name of the field type.
func (t FieldType) String() string {
	switch t {
	case FieldString:
		return "string"
	case FieldBool:
		return "bool"
	case FieldInt64:
		return "int64"
	case FieldUint64:
		return "uint64"
	case FieldDouble:
		return "double"
	case FieldBytes:
		return "bytes"
	case FieldTimestamp:
		return "google.protobuf.Timestamp"
	case FieldDuration:
		return "google.protobuf.Duration"
	default:
		return "unspecified"
	}
}

// PXF writes the grid data as a self-describing PXF document: an
// embedded schema in a source-form @proto directive (protowire draft
// §3.4.5) followed by a @dataset directive (draft §3.4.4) whose header
// names the row message type and the exported columns, with one
// positional row per grid row.
//
//	@proto """
//	syntax = "proto3";
//	package trades.v1;
//
//	message Trade {
//	  string symbol = 1;
//	  double price = 2;
//	  int64 qty = 3;
//	}
//	"""
//	@dataset trades.v1.Trade (symbol, price, qty)
//	("AAPL", 192.34, 100)
//	("MSFT", 410.10, 50)
//
// msgType is the fully qualified protobuf message type the rows bind
// to; column Field names become the top-level field names of the
// embedded definition (unqualified — the v1 cell grammar forbids
// dotted paths). The source form is used (not the named form) because
// it is the only @proto shape that can import the well-known types
// that time.Time and time.Duration cells bind to.
//
// Field types are declared per column via Column.Type — never
// inferred from cell values. Columns with a Formatter and no declared
// type are string (formatted cells emit as quoted strings); an
// explicit non-string declaration wins over the Formatter and exports
// the raw typed value — the formatter is a display concern, the
// declaration governs the data export. Every other column must
// declare a type, and a cell value whose Go type does not match the
// declaration (see the FieldType constants for the accepted Go types)
// fails the export — cells are never coerced or stringified. Field
// numbers are assigned in column order, starting at 1. Note the
// embedded schema is authoritative for decoders (draft §3.4.5); if
// the rows are meant to bind to an externally defined type, the
// declared field types and numbers must match that definition.
//
// Cell emission rules:
//   - a nil value produces an empty cell (field absent), never `null` —
//     a null cell rejects on non-nullable scalar fields at bind time
//   - string columns with a Formatter emit the formatted value as a
//     quoted string, matching the CSV/Excel fidelity rules
//   - otherwise values emit as typed PXF literals: quoted strings,
//     integers, floats (inf/-inf/nan), booleans, bare RFC 3339
//     timestamps for time.Time, Go duration literals for
//     time.Duration, and b"…" base64 for []byte
//
// On error the document is invalid; nothing already written to w is
// unwound.
func PXF(w io.Writer, msgType string, columns []Column, data DataAccess) error {
	if err := validateDatasetType(msgType); err != nil {
		return err
	}
	if len(columns) == 0 {
		return fmt.Errorf("export: PXF requires at least one column")
	}
	seen := make(map[string]bool, len(columns))
	kinds := make([]FieldType, len(columns))
	for i, col := range columns {
		if err := validateDatasetColumn(col.Field); err != nil {
			return err
		}
		if seen[col.Field] {
			return fmt.Errorf("export: duplicate @dataset column %q", col.Field)
		}
		seen[col.Field] = true
		kind, err := columnFieldType(col)
		if err != nil {
			return err
		}
		kinds[i] = kind
	}

	bw := bufio.NewWriter(w)

	writeProtoDirective(bw, msgType, columns, kinds)

	bw.WriteString("@dataset ")
	bw.WriteString(msgType)
	bw.WriteString(" (")
	for i, col := range columns {
		if i > 0 {
			bw.WriteString(", ")
		}
		bw.WriteString(col.Field)
	}
	bw.WriteString(")\n")

	for r := 0; r < data.RowCount(); r++ {
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		bw.WriteByte('(')
		for c, col := range columns {
			if c > 0 {
				bw.WriteString(", ")
			}
			if err := writePXFCell(bw, col, kinds[c], rowData); err != nil {
				return fmt.Errorf("export: PXF row %d: %w", r, err)
			}
		}
		bw.WriteString(")\n")
	}

	return bw.Flush()
}

// columnFieldType resolves a column's declared field type. A
// Formatter column with no declaration is string (formatted cells
// emit as quoted strings); an explicit declaration wins over the
// Formatter — the formatter is a display concern, the declaration
// governs the data export. Every other column must declare a type.
func columnFieldType(col Column) (FieldType, error) {
	if col.Type == FieldUnspecified {
		if col.Formatter != nil {
			return FieldString, nil
		}
		return 0, fmt.Errorf("export: PXF column %q: no field type declared", col.Field)
	}
	return col.Type, nil
}

// fieldImport returns the .proto import a field type requires, or "".
func fieldImport(t FieldType) string {
	switch t {
	case FieldTimestamp:
		return "google/protobuf/timestamp.proto"
	case FieldDuration:
		return "google/protobuf/duration.proto"
	default:
		return ""
	}
}

// writeProtoDirective embeds the row message definition as a
// source-form @proto directive (a complete .proto file inside a
// triple-quoted string), making the document self-describing.
func writeProtoDirective(bw *bufio.Writer, msgType string, columns []Column, kinds []FieldType) {
	pkg, msg := "", msgType
	if i := strings.LastIndex(msgType, "."); i >= 0 {
		pkg, msg = msgType[:i], msgType[i+1:]
	}

	bw.WriteString("@proto \"\"\"\n")
	bw.WriteString("syntax = \"proto3\";\n")
	if pkg != "" {
		bw.WriteString("package ")
		bw.WriteString(pkg)
		bw.WriteString(";\n")
	}
	var imports []string
	for _, k := range kinds {
		if imp := fieldImport(k); imp != "" && !slices.Contains(imports, imp) {
			imports = append(imports, imp)
		}
	}
	slices.Sort(imports)
	for _, imp := range imports {
		bw.WriteString("import \"")
		bw.WriteString(imp)
		bw.WriteString("\";\n")
	}

	bw.WriteString("\nmessage ")
	bw.WriteString(msg)
	bw.WriteString(" {\n")
	for i, col := range columns {
		bw.WriteString("  ")
		bw.WriteString(kinds[i].String())
		bw.WriteByte(' ')
		bw.WriteString(col.Field)
		bw.WriteString(" = ")
		bw.WriteString(strconv.Itoa(i + 1))
		bw.WriteString(";\n")
	}
	bw.WriteString("}\n\"\"\"\n")
}

// validateDatasetType checks msgType is a dotted identifier such as
// "trades.v1.Trade".
func validateDatasetType(msgType string) error {
	if msgType == "" {
		return fmt.Errorf("export: PXF requires a message type for the @dataset header")
	}
	for _, seg := range strings.Split(msgType, ".") {
		if !isPXFIdent(seg) {
			return fmt.Errorf("export: invalid @dataset message type %q", msgType)
		}
	}
	return nil
}

// validateDatasetColumn checks the field name is a legal, reachable
// @dataset column: a bare identifier (v1 forbids dotted paths) that is
// not a PXF value keyword (draft §3.13 — a field named null, true, or
// false lexes as a literal and can never be bound).
func validateDatasetColumn(field string) error {
	if field == "" {
		return fmt.Errorf("export: PXF requires a Field name for every column")
	}
	if strings.Contains(field, ".") {
		return fmt.Errorf("export: @dataset column %q: dotted paths are not allowed in v1", field)
	}
	if !isPXFIdent(field) {
		return fmt.Errorf("export: invalid @dataset column name %q", field)
	}
	switch field {
	case "null", "true", "false":
		return fmt.Errorf("export: @dataset column %q collides with a PXF value keyword", field)
	}
	return nil
}

func isPXFIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, ch := range s {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch == '_':
		case ch >= '0' && ch <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// writePXFCell writes one row cell as a literal of the declared field
// type. A nil raw value always emits an empty cell (field absent),
// even when a Formatter is set. A value whose Go type does not match
// the declared field type is an error — never a coercion.
func writePXFCell(bw *bufio.Writer, col Column, kind FieldType, rowData any) error {
	value := cellValue(col, rowData)
	if value == nil {
		return nil // empty cell — field absent
	}
	if col.Formatter != nil && kind == FieldString {
		writePXFString(bw, col.Formatter(value))
		return nil
	}
	switch kind {
	case FieldString:
		if v, ok := value.(string); ok {
			writePXFString(bw, v)
			return nil
		}
	case FieldBool:
		if v, ok := value.(bool); ok {
			if v {
				bw.WriteString("true")
			} else {
				bw.WriteString("false")
			}
			return nil
		}
	case FieldInt64:
		if v, ok := int64Value(value); ok {
			bw.WriteString(strconv.FormatInt(v, 10))
			return nil
		}
	case FieldUint64:
		if v, ok := uint64Value(value); ok {
			bw.WriteString(strconv.FormatUint(v, 10))
			return nil
		}
	case FieldDouble:
		switch v := value.(type) {
		case float32:
			writePXFFloat(bw, float64(v), 32)
			return nil
		case float64:
			writePXFFloat(bw, v, 64)
			return nil
		}
	case FieldBytes:
		if v, ok := value.([]byte); ok {
			bw.WriteString(`b"`)
			bw.WriteString(base64.StdEncoding.EncodeToString(v))
			bw.WriteByte('"')
			return nil
		}
	case FieldTimestamp:
		if v, ok := value.(time.Time); ok {
			bw.WriteString(v.Format(time.RFC3339Nano))
			return nil
		}
	case FieldDuration:
		if v, ok := value.(time.Duration); ok {
			bw.WriteString(v.String())
			return nil
		}
	}
	return fmt.Errorf("column %q: %T value does not match declared %s", col.Field, value, kind)
}

// int64Value unpacks any signed integer value.
func int64Value(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	}
	return 0, false
}

// uint64Value unpacks any unsigned integer value.
func uint64Value(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	}
	return 0, false
}

func writePXFFloat(bw *bufio.Writer, f float64, bits int) {
	switch {
	case math.IsInf(f, 1):
		bw.WriteString("inf")
	case math.IsInf(f, -1):
		bw.WriteString("-inf")
	case math.IsNaN(f):
		bw.WriteString("nan")
	default:
		bw.WriteString(strconv.FormatFloat(f, 'g', -1, bits))
	}
}

// writePXFString writes a Go-style quoted PXF string literal.
func writePXFString(bw *bufio.Writer, s string) {
	bw.WriteByte('"')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '"':
			bw.WriteString(`\"`)
		case '\\':
			bw.WriteString(`\\`)
		case '\n':
			bw.WriteString(`\n`)
		case '\r':
			bw.WriteString(`\r`)
		case '\t':
			bw.WriteString(`\t`)
		default:
			if ch < 0x20 {
				bw.WriteString(`\x`)
				bw.WriteByte("0123456789abcdef"[ch>>4])
				bw.WriteByte("0123456789abcdef"[ch&0xf])
			} else {
				bw.WriteByte(ch)
			}
		}
	}
	bw.WriteByte('"')
}
