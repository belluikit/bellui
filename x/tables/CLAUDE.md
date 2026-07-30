# CLAUDE.md

## Project Overview

`x/tables` (module `github.com/belluikit/bellui/x/tables`, historically "fytables") is an AG Grid-equivalent data table widget for the Bellui GUI framework (Go). It provides a high-performance, virtual-scrolling data grid with sorting, filtering, cell editing, sparklines, column pinning/grouping, row grouping, aggregation, pagination, pivot tables, tree data, and export.

## Build & Test

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run benchmarks
go test -bench=. ./render/

# Run the demo — it lives in the separate gallery repo, cloned beside bellui
cd ../../../gallery && go run .
```

**Own module, pinned to a released bellui.** This directory is a nested module inside the bellui repo, and its `go.mod` pins `github.com/belluikit/bellui v1.0.0-rc.5` — resolved from the module cache, **not** from the enclosing checkout. There is **no `go.work` anywhere** in the repo and no `replace` directive, so local edits to the surrounding bellui tree are *not* picked up by default. To build against local bellui changes (e.g. a new a11y interface), add `replace github.com/belluikit/bellui => ../..` to this `go.mod`, then bump the pin once the change is released and drop the replace.

The demo consumes `x/tables` as a *published* module too (`gallery/go.mod` requires `github.com/belluikit/bellui/x/tables`), so local grid changes need the same treatment there — a `replace github.com/belluikit/bellui/x/tables => <path>/bellui/x/tables` in the gallery module.

Two consequences of being a nested module, both of which have bitten:
- `go build ./...` / `go vet ./...` / `staticcheck` run from the bellui repo root **do not descend into this module**, and neither does any CI leg. Breakage here is invisible until someone builds this directory directly.
- The same is true one level deeper: `hack/a11yprobe/` is its own module again. Its `go.mod` sat unbuildable for weeks after the fyne→bellui rename (an invalid `v3.0.0-rc.8` pin plus `bellui.io/systray`, `fyne-io/*`, `srwiley/*` and `trendvidia/*` requirements). After any rename, grep `--include=go.mod` for stale module paths across the whole tree.

## Architecture

### Package Structure
- **Root** (package `tables`): `DataGrid` widget, `ColumnDef`, `GridAPI`, options, state, event handlers
- **`cell/`**: Cell renderers (`TextRenderer`, `NumberRenderer`, `BooleanRenderer`, `DateRenderer`) and editors. All use `canvas.Text` (not `widget.Label`) because canvas primitives render correctly in renderer `Objects()` slices.
- **`render/`**: Virtual rendering engine. `Viewport` computes visible ranges. `CellPool` recycles cells. `CellsWidget`/`CellsRenderer` render main columns. `PinnedCellsWidget` renders pinned columns. `HeaderWidget` renders column headers with sort/filter indicators.
- **`pipeline/`**: Data transformation chain. `Pipeline` orchestrates: Source -> Filter -> Sort -> Pivot -> Group -> Paginate. `RowIndex` maps display rows to source rows.
- **`filter/`**: Filter implementations (Text, Number, Date, Set).
- **`sparkline/`**: In-cell chart widget using `canvas.Raster`.
- **`export/`**: CSV, TSV, Excel (.xlsx), paginated PDF report, and PXF `@dataset` exporters. PDF and Excel are dependency-free (hand-written minimal PDF/OOXML). PDF streams page-by-page with a default 10k row cap.
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
- Bellui's `widget/richtext.go` relies on `insertAt()` and `deleteFromTo()` incrementing `rowBoundsVersion` (at the end of each function) to invalidate the stale row-bounds cache. The lazy single-segment edit path bumps it too, via `lazySpliceText` in `widget/richtext_lazy.go`.

## Conventions

- All colors come from `render.ResolveThemeColors()`, never hardcoded
- Cell `UpdateCell()` must set alignment/style every time (cells recycle across types)
- Tests use `test.NewTempApp(t)` for Bellui widget tests
- Pipeline transforms run in order: Filter -> Sort -> Pivot -> Group -> Paginate
