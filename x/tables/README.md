# Bellui X — tables

An AG Grid-equivalent data table widget for the [Bellui](https://bellui.com) GUI framework.

x/tables provides a high-performance, feature-rich data grid with virtual scrolling, column sorting/filtering, cell editing, sparklines, column pinning/grouping, row grouping with aggregation, pagination, pivot tables, and more.

## Features

### Core
- **Virtual scrolling** - O(1) viewport calculation; handles 100K+ rows at 14 ns/op
- **Column sorting** - Single click to sort, shift+click for multi-column sort
- **Column filtering** - Per-column text/number/date/set filters, quick filter across all columns
- **Cell editing** - Double-click or F2 to edit; text, number, select, checkbox, date editors
- **Selection** - Single/multi row, shift+click range, ctrl+click toggle, select all
- **Keyboard navigation** - Arrow keys, Tab, Enter, Escape, F2
- **Clipboard** - Ctrl+C copy, Ctrl+V paste
- **Undo/redo** - Ctrl+Z / Ctrl+Shift+Z for cell edits
- **Context menu** - Right-click headers for sort/filter/pin/hide options

### Columns
- **Pinning** - Freeze columns to left or right edge
- **Grouping** - Nested column groups with expand/collapse
- **Reordering** - Drag headers to reorder columns
- **Resizing** - Drag header borders with min/max constraints and header-text-aware minimum
- **Visibility** - Show/hide columns programmatically
- **Spanning** - ColSpan callback for cells spanning multiple columns
- **State persistence** - Save/restore column widths, order, pinning, visibility

### Data
- **Row grouping** - Group rows by column values with expand/collapse, and per-column aggregates (sum, average, min, max, count, first, last) on each group header
- **Aggregation** - Sum, Avg, Min, Max, Count, First, Last + custom functions
- **Pagination** - Configurable page size with navigation API
- **Pivot tables** - Dynamic columns from unique values with aggregation
- **Tree data** - Hierarchical parent-child rows with expand/collapse
- **Master-detail** - Expandable detail rows

### Group aggregates

Give a column an `AggFunc` and, when the grid is grouped, each group header row
shows that column's aggregate over the group's rows, in that column. A collapsed
group still aggregates all of its rows. The header row's accessible label reads
the aggregates too, e.g. "Engineering (12), Salary sum $1250000".

```go
cols := []*tables.ColumnDef{
    {Field: "dept"},
    {Field: "name"},
    {Field: "salary", HeaderName: "Salary", AggFunc: bellui.Ptr(pipeline.AggSum),
        ValueFormatter: func(v any) string { return fmt.Sprintf("$%.0f", v) }},
    {Field: "age", HeaderName: "Age", AggFunc: bellui.Ptr(pipeline.AggAvg)},
}
grid.API().SetRowGroupColumn("dept")
```

`AggFunc` is a pointer because `AggSum` is the zero value; nil means no
aggregate. Cell values are read as numbers (any Go integer or float type), and
anything else, nil included, counts as 0. The aggregate is a `float64`: it is
shown through the column's `ValueFormatter` when set (so the formatter must
accept a `float64`), else through a `NumberRenderer`'s `Format`, else rounded to
two decimals. A count is always shown as a whole number.

Code that drives the pipeline directly sets the same thing on the transform:
`GroupTransform.AggColumns` fills each header's `GroupRow.AggValues`, keyed by
`AggColumn.Field`.

## Sparklines
Four chart types rendered in cells via `canvas.Raster`:
- **Line** - With optional markers and area fill
- **Area** - Line with filled area below
- **Bar** - Horizontal bars with negative value coloring
- **Column** - Vertical columns with axis line

Options: stroke/fill colors, markers (circle/square/diamond), axis line, domain scaling, highlight on hover with tooltip, per-point ItemStyler, raster caching.

### Visual
- **Theme-aware** - All colors from Bellui theme; works in light and dark modes
- **Density modes** - Compact, Comfortable, Spacious
- **Row alternation** - Alternating row background colors
- **Text truncation** - Ellipsis with full-text tooltip on hover
- **Status bar** - Row count, selected count, page info
- **Loading overlay** - Semi-transparent overlay with message
- **Floating filters** - Inline filter inputs below headers

### Export
- **CSV** - Standard CSV with headers
- **TSV** - Tab-separated values
- **Excel** - Zero-dependency .xlsx via OOXML zip format
- **PDF** - Zero-dependency paginated report: repeated headers per page, fit-to-width columns, title block, page numbers, streaming with a row cap (hand off to the OS viewer for printing)
- **PXF** - Typed rows via the protowire `@dataset` directive (draft §3.4.4) — the schema-backed CSV replacement

## Quick Start

```go
package main

import (
    "github.com/belluikit/bellui"
    "github.com/belluikit/bellui/app"
    "github.com/belluikit/bellui/geo"
    "github.com/belluikit/bellui/x/tables"
)

func main() {
    data := []map[string]any{
        {"name": "Alice", "age": 30, "dept": "Engineering"},
        {"name": "Bob", "age": 25, "dept": "Design"},
        {"name": "Charlie", "age": 35, "dept": "Engineering"},
    }

    grid := tables.NewDataGrid(
        []*tables.ColumnDef{
            {Field: "name", HeaderName: "Name", Width: 150},
            {Field: "age", HeaderName: "Age", Type: tables.ColumnNumber, Width: 80},
            {Field: "dept", HeaderName: "Department", Width: 150},
        },
        func() int { return len(data) },
        func(row int) any { return data[row] },
    )

    a := app.New()
    w := a.NewWindow("tables")
    w.SetContent(grid)
    w.Resize(geo.NewSize(500, 400))
    bellui.ShowAndRun(w)
}
```

## Demo

Run the comprehensive demo app with 8 tabs showcasing all features:

```bash
# Demo lives in the gallery app (../gallery):
go run ../gallery
```

## API

The `GridAPI` provides programmatic control:

```go
api := grid.API()

// Sorting
api.SetSort(tables.SortItem{ColID: "name", Direction: tables.SortAscending})

// Filtering
api.SetColumnFilter("age", func(v any) bool { return v.(int) >= 30 })
api.SetQuickFilter("engineer")

// Pagination
api.EnablePagination(50)
api.SetPage(2)

// Grouping
api.SetRowGroupColumn("dept")
api.ToggleGroup("Engineering")

// Columns
api.SetColumnWidth("name", 200)
api.SetColumnPinned("name", tables.PinLeft)
api.SetColumnVisible("dept", false)
api.MoveColumn(0, 3)
api.SizeColumnsToFit()

// Export
var buf bytes.Buffer
api.ExportCSV(&buf)
api.ExportExcel(&buf)
api.ExportPDF(&buf, export.PDFOptions{Title: "Trades", Landscape: true})
api.ExportPXF(&buf, "trades.v1.Trade")

// State
state := api.GetGridState()
api.SetGridState(state)

// Editing
api.Undo()
api.Redo()
```

## Group aggregates

Give a column an `AggFunc` and, when the grid is grouped, each group header row
shows that column's aggregate over the group's rows, in that column. A collapsed
group still aggregates all of its rows. The header row's accessible label reads
the aggregates too, e.g. "Engineering (12), Salary sum $1250000".

```go
cols := []*tables.ColumnDef{
    {Field: "dept"},
    {Field: "name"},
    {Field: "salary", HeaderName: "Salary", AggFunc: bellui.Ptr(pipeline.AggSum),
        ValueFormatter: func(v any) string { return fmt.Sprintf("$%.0f", v) }},
    {Field: "age", HeaderName: "Age", AggFunc: bellui.Ptr(pipeline.AggAvg)},
}
grid.API().SetRowGroupColumn("dept")
```

`AggFunc` is a pointer because `AggSum` is the zero value; nil means no
aggregate. Cell values are read as numbers (any Go integer or float type), and
anything else, nil included, counts as 0. The aggregate is a `float64`: it is
shown through the column's `ValueFormatter` when set (so the formatter must
accept a `float64`), else through a `NumberRenderer`'s `Format`, else rounded to
two decimals. A count is always shown as a whole number.

Code that drives the pipeline directly sets the same thing on the transform:
`GroupTransform.AggColumns` fills each header's `GroupRow.AggValues`, keyed by
`AggColumn.Field`.

## Sparklines

```go
{
    Field: "trend", HeaderName: "Trend", Width: 140,
    CellRenderer: sparkline.NewCellRenderer(sparkline.Line,
        sparkline.WithStroke(color.NRGBA{R: 33, G: 150, B: 243, A: 255}),
        sparkline.WithFill(color.NRGBA{R: 33, G: 150, B: 243, A: 30}),
        sparkline.WithMarkers(2),
        sparkline.WithAxis(),
    ),
}
```

## Architecture

```
github.com/belluikit/bellui/x/tables/
├── Root package     - DataGrid widget, ColumnDef, GridAPI, options, state
├── cell/            - Cell renderers (Text, Number, Boolean, Date) + editors
├── render/          - Virtual rendering engine (Viewport, CellPool, CellsWidget,
│                      Header, PinnedCells, StatusBar, LoadingOverlay)
├── pipeline/        - Data transforms (Sort, Filter, Group, Aggregate,
│                      Paginate, Pivot, Tree)
├── filter/          - Filter implementations (Text, Number, Date, Set)
├── sparkline/       - In-cell charts (Line, Area, Bar, Column)
├── export/          - CSV, TSV, Excel, PDF, PXF exporters
├── theme/           - Grid-specific theme (colors, sizes, density modes)
└── (demo lives in the gallery app: ../gallery, "Data Grid" page)
```

**Key design decisions:**
- Callback-based API following Bellui conventions (`Length`, `RowData`, `OnSelected`)
- Per-column-type cell pools for efficient recycling
- `canvas.Raster` for sparklines (single texture per cell, not 50+ Line objects)
- Pipeline architecture: Source -> Filter -> Sort -> Pivot -> Group -> Paginate
- O(1) viewport calculation for uniform row heights (14 ns for 100K rows)
- Separate coordinate systems for pinned and main columns

## Performance

```
BenchmarkViewport_UniformRows_100K    84M ops    14 ns/op
BenchmarkViewport_UniformRows_1M      67M ops    18 ns/op
```

## Testing

```bash
go test ./...
```

93 Go files, 162 tests across 7 test suites.

## License

See repository for license information.
