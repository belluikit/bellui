# CLAUDE.md

## Project Overview

fytables is an AG Grid-equivalent data table widget for the Fyne GUI framework (Go). It provides a high-performance, virtual-scrolling data grid with sorting, filtering, cell editing, sparklines, column pinning/grouping, row grouping, aggregation, pagination, pivot tables, tree data, and export.

## Build & Test

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run demo app
go run ../gallery   # demo lives in the gallery app

# Run benchmarks
go test -bench=. ./render/
```

The module builds against the pinned Fyne fork release (`github.com/belluikit/bellui` in `go.mod`, currently `v2.17.0`) resolved from the module cache. There is **no** `go.work` and no `replace` directive — a local checkout of the fork at `../fyne` is *not* consumed by default. To build fytables against local fork changes (e.g. new a11y interfaces), add a `replace github.com/belluikit/bellui => ../fyne` to `go.mod` or create a `go.work` spanning both modules, then bump the pinned version once the fork change is released.

## Architecture

### Package Structure
- **Root** (`fytables`): `DataGrid` widget, `ColumnDef`, `GridAPI`, options, state, event handlers
- **`cell/`**: Cell renderers (`TextRenderer`, `NumberRenderer`, `BooleanRenderer`, `DateRenderer`) and editors. All use `canvas.Text` (not `widget.Label`) because canvas primitives render correctly in renderer `Objects()` slices.
- **`render/`**: Virtual rendering engine. `Viewport` computes visible ranges. `CellPool` recycles cells. `CellsWidget`/`CellsRenderer` render main columns. `PinnedCellsWidget` renders pinned columns. `HeaderWidget` renders column headers with sort/filter indicators.
- **`pipeline/`**: Data transformation chain. `Pipeline` orchestrates: Source -> Filter -> Sort -> Pivot -> Group -> Paginate. `RowIndex` maps display rows to source rows.
- **`filter/`**: Filter implementations (Text, Number, Date, Set).
- **`sparkline/`**: In-cell chart widget using `canvas.Raster`.
- **`export/`**: CSV, TSV, Excel (.xlsx) exporters.
- **`theme/`**: Grid-specific theme with density modes.

### Key Patterns

**Two column index systems:**
- **Main index** (0 = first non-pinned column): Used by `CellsRenderer`, `Viewport`, `ColumnCount()`, `ColumnTypeName()`, `CreateCell()`, `UpdateCell()`, `mainCol(i)`
- **Absolute display index** (0 = first visible column including pinned): Used by `Header`, `PinnedCellsRenderer`, `HeaderColumnCount()`, `PinnedColumnTypeName()`, `PinnedCreateCell()`, `PinnedUpdateCell()`, `visibleCol(i)`

**Cell lifecycle:**
1. `CreateCell()` creates a `canvas.Text` template
2. `UpdateCell()` sets text, alignment, color, size, truncation on every call (cells may be recycled across column types)
3. Pool keyed by `col_<field>_<type>` string

**Rendering refresh chain:**
- `g.Refresh()` -> `dataGridRenderer.Refresh()` -> `doLayout(g.Size())` -> positions all children + `viewport.Update()` + refreshes all child widgets

**Column resize:**
- `HeaderWidget` implements `Draggable`/`Hoverable`/`Cursorable`
- Resize detection via `headerBorderCol()` (4px threshold)
- Uses absolute position delta (not incremental DX)
- Clamped by: header text width (min), `gridWidth - otherColumns - scrollBarReserve` (max)

### Gotchas

- `canvas.Text` does NOT clip. Text truncation must be done manually via `cell.TruncateText()` before setting the string.
- `BaseWidget.Resize(sameSize)` is a no-op — won't trigger `Layout()`. Use `Refresh()` which calls `doLayout()`.
- Pinned-right columns are overlaid on the scroll area. The viewport's effective width is reduced by `pinnedRightW` so cells don't render behind the overlay.
- `resolveColumns()` rebuilds from `ColumnDefs` — preserves widths via ID-keyed map.
- The Fyne fork has a fix in `widget/richtext.go`: `insertAt()` and `deleteFromTo()` increment `rowBoundsVersion` to invalidate stale row bounds cache.

## Conventions

- All colors come from `render.ResolveThemeColors()`, never hardcoded
- Cell `UpdateCell()` must set alignment/style every time (cells recycle across types)
- Tests use `test.NewTempApp(t)` for Fyne widget tests
- Pipeline transforms run in order: Filter -> Sort -> Pivot -> Group -> Paginate
