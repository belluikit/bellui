// Package tables provides an AG Grid-equivalent data table widget for the Bellui GUI framework.
//
// The main entry point is [NewDataGrid], which creates a virtual-scrolling data grid
// that can handle 100k+ rows with features including sorting, filtering, cell editing,
// column pinning/grouping, sparklines, and row grouping with aggregation.
//
// Basic usage:
//
//	grid := tables.NewDataGrid(
//	    []*tables.ColumnDef{
//	        {Field: "name", HeaderName: "Name", Flex: 1},
//	        {Field: "age", HeaderName: "Age", Type: tables.ColumnNumber, Width: 80},
//	    },
//	    func() int { return len(data) },
//	    func(row int) any { return data[row] },
//	)
package tables
