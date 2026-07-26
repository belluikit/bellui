// Package export provides data export functionality for the data grid.
package export

import (
	"encoding/csv"
	"fmt"
	"io"
)

// Column describes a column to export.
type Column struct {
	Header      string
	Field       string
	ValueGetter func(rowData any) any
	Formatter   func(value any) string
}

// DataAccess provides the data to export.
type DataAccess interface {
	RowCount() int
	RowData(displayRow int) any
}

// CSV writes the grid data as CSV to the given writer.
func CSV(w io.Writer, columns []Column, data DataAccess) error {
	cw := csv.NewWriter(w)

	// Write header row
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Header
	}
	if err := cw.Write(headers); err != nil {
		return err
	}

	// Write data rows
	row := make([]string, len(columns))
	for r := 0; r < data.RowCount(); r++ {
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		for c, col := range columns {
			var value any
			if col.ValueGetter != nil {
				value = col.ValueGetter(rowData)
			} else if m, ok := rowData.(map[string]any); ok {
				value = m[col.Field]
			}
			if col.Formatter != nil {
				row[c] = col.Formatter(value)
			} else {
				row[c] = formatAny(value)
			}
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

// TSV writes tab-separated values.
func TSV(w io.Writer, columns []Column, data DataAccess) error {
	cw := csv.NewWriter(w)
	cw.Comma = '\t'

	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Header
	}
	if err := cw.Write(headers); err != nil {
		return err
	}

	row := make([]string, len(columns))
	for r := 0; r < data.RowCount(); r++ {
		rowData := data.RowData(r)
		if rowData == nil {
			continue
		}
		for c, col := range columns {
			var value any
			if col.ValueGetter != nil {
				value = col.ValueGetter(rowData)
			} else if m, ok := rowData.(map[string]any); ok {
				value = m[col.Field]
			}
			if col.Formatter != nil {
				row[c] = col.Formatter(value)
			} else {
				row[c] = formatAny(value)
			}
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

func formatAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
