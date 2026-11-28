package tables

import (
	"fmt"
	"image/color"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
	"github.com/belluikit/bellui/x/tables/pipeline"
	"github.com/belluikit/bellui/x/tables/render"
	gridTheme "github.com/belluikit/bellui/x/tables/theme"
)

// Compile-time interface checks.
var (
	_ core.Widget                        = (*DataGrid)(nil)
	_ core.Accessible                    = (*DataGrid)(nil)
	_ core.AccessibleGridDimensioner     = (*DataGrid)(nil)
	_ core.AccessibleActiveDescendant    = (*DataGrid)(nil) // pre-rebrand tracker #42: cursor cell as AT-focused descendant (pre-rebrand tracker #688)
	_ core.AccessibleVirtualCellRealizer = (*DataGrid)(nil) // #78: navigate off-viewport cells (pre-rebrand tracker #637)
	_ core.AccessibleScrollIntoView      = (*DataGrid)(nil) // #78: bring an off-viewport cell into view (pre-rebrand tracker #637)
	_ event.Focusable                    = (*DataGrid)(nil)
	_ event.Tappable                     = (*DataGrid)(nil)
	_ event.DoubleTappable               = (*DataGrid)(nil)
	_ event.SecondaryTappable            = (*DataGrid)(nil)
	_ event.Draggable                    = (*DataGrid)(nil)
	_ bellui.Shortcutable                = (*DataGrid)(nil)
	_ event.Hoverable                    = (*DataGrid)(nil)
	_ desktop.Mouseable                  = (*DataGrid)(nil)
	_ desktop.Cursorable                 = (*DataGrid)(nil)
)

// DataGrid is the main data table widget. It provides virtual scrolling for
// large datasets, column sorting, filtering, cell editing, and sparkline support.
type DataGrid struct {
	widget.BaseWidget

	// ColumnDefs defines the columns to display.
	ColumnDefs []*ColumnDef

	// DefaultColDef provides shared defaults for all columns.
	DefaultColDef *DefaultColumnDef

	// Length returns the total number of rows in the data source.
	Length func() int

	// RowData returns the row data for the given source row index.
	RowData func(row int) any

	// SelectionMode controls how cells/rows can be selected.
	SelectionMode SelectionMode

	// Callbacks
	// OnSelectionChanged reports the selected rows as stable source-row indices
	// (into the underlying data), not display positions, so they remain valid
	// across sort/filter/paginate.
	OnSelectionChanged  func(rows []int)
	OnCellClicked       func(id CellID)
	OnCellDoubleClicked func(id CellID)
	// OnRowActivated fires when the user activates the focused row from the
	// keyboard — Enter/Return or Space on a data row (not a group/detail/tree
	// disclosure, which those keys toggle instead). The row is reported as a
	// stable source-row index (like OnSelectionChanged), so it survives
	// sort/filter/paginate. This is the keyboard analog of a primary-click row
	// action; pointer clicks continue to route through OnCellClicked.
	OnRowActivated func(sourceRow int)
	// OnCellSecondaryTapped fires on a secondary (right / two-finger) tap on a
	// body cell. The header keeps its built-in column menu. The PointEvent lets
	// callers position a popup at e.AbsolutePosition (e.g. widget.ShowPopUpMenuAtPosition).
	OnCellSecondaryTapped func(id CellID, e *event.PointEvent)
	// OnCellContextMenu is the keyboard analog of OnCellSecondaryTapped: it
	// fires when the Menu (application) key or Shift+F10 is pressed with a
	// focused cell, after the cell has been scrolled into view and selected —
	// the same select-then-fire order as the pointer path. id is the focused
	// cell in display coordinates, the same form OnCellSecondaryTapped
	// delivers (SourceRow maps it to the stable source row). anchor is the
	// cell's bottom-left in window-absolute coordinates — the grid's
	// driver-reported absolute position plus the cell's widget-local rect,
	// the same space as event.PointEvent.AbsolutePosition — so it passes
	// straight through to widget.ShowPopUpMenuAtPosition.
	OnCellContextMenu  func(id CellID, anchor geo.Position)
	OnSortChanged      func(model []SortItem)
	OnFilterChanged    func()
	OnCellValueChanged func(id CellID, oldValue, newValue any)
	// OnRowMoved is called when the user moves a row (GridOptions
	// RowDragEnabled), with source rows: the app removes the row at
	// fromSourceRow and inserts it where toSourceRow is, so it ends up at
	// that place. The grid's own source-keyed state (selection, cursor,
	// expanded details) moves the same way.
	OnRowMoved func(fromSourceRow, toSourceRow int)
	// OnVisibleRowsChanged fires when the range of display rows in view
	// changes: by scrolling, a resize, or new data. first and last are
	// display rows, inclusive; an empty grid reports 0, -1. An app that loads
	// rows in pages compares last with RowCount to fetch the next page as the
	// listener nears the end (#983, player#277).
	OnVisibleRowsChanged func(first, last int)

	// LeadingSwipeActions and TrailingSwipeActions give the buttons a row
	// reveals when a finger swipes it sideways (#1483), for the row's source
	// row; nil, or an empty list, means none that way. Leading actions sit at
	// the row's leading edge, the left, or the right in a right-to-left
	// layout, and a swipe away from that edge uncovers them; trailing ones
	// sit at the other edge. The first of each list is the outermost. The
	// grid asks for a row's actions when a swipe on it starts.
	//
	// A sideways touch drag scrolls the body while the body can move that
	// way. One that starts with the body already at that edge, or on a grid
	// that fits its width, swipes the row under the finger instead; a drag
	// that scrolls into the edge stays a scroll. Released past half its
	// actions, or flicked open, the row stays open; otherwise it slides back.
	// A tap anywhere on the grid, a scroll or new data close an open row,
	// and a sideways drag on it moves it again. Group headers, detail bands
	// and a disabled grid do not swipe, and a mouse drag never does: on the
	// desktop the actions are in the context menu (see RowSwipeAction).
	//
	// Since: 1.7
	LeadingSwipeActions  func(sourceRow int) []RowSwipeAction
	TrailingSwipeActions func(sourceRow int) []RowSwipeAction

	// MasterDetail configures expandable detail rows.
	MasterDetail MasterDetailConfig

	// Options configures grid behaviour and appearance.
	Options GridOptions

	// Internal state
	api               *GridAPI
	pipe              *pipeline.Pipeline
	resolvedCols      []*resolvedColumn
	viewport          *render.Viewport
	reportedRows      render.Range // last range OnVisibleRowsChanged was given
	rowsReported      bool         // whether it has been given one
	pool              *render.CellPool
	cellsWidget       *render.CellsWidget
	headerWidget      *render.HeaderWidget
	scrollContent     *render.ScrollContainer
	pinnedLeftCells   *render.PinnedCellsWidget
	pinnedRightCells  *render.PinnedCellsWidget
	groupRowOverlay   *render.GroupRowOverlay
	rowSwipeLayer     *rowSwipeLayer    // draws the swiped row (#1483)
	rowLiftLayer      *rowSwipeLayer    // draws a row being moved (#1512)
	dropLine          *canvas.Rectangle // where a moved row will land (#1512)
	lift              rowLift
	floatingFilterRow *render.FloatingFilterRow // set by the renderer when floating filters are enabled
	gridTheme         *gridTheme.GridTheme

	sortModel         []SortItem
	filterModel       map[string]filterState
	quickFilter       string
	groupTransform    *pipeline.GroupTransform
	treeTransform     *pipeline.TreeTransform
	pivotTransform    *pipeline.PivotTransform
	paginateTransform *pipeline.PaginateTransform

	// Persistent selection is keyed by stable source-row identity (not display
	// position) so it survives re-sort/re-filter/paginate. Display<->source is
	// translated through pipeline.RowIndex at select/paint time.
	selectedRows map[int]bool // set of selected source rows
	selCellRow   int          // source row of the selected cell (-1 = none)
	selCellCol   int          // column of the selected cell
	// selectedCells is SelectionMultiCell's selection: source row and
	// absolute display column of each selected cell (#1401).
	selectedCells map[selCell]bool

	selectedCell *CellID // live cursor cell in DISPLAY coords (editing/scroll/API)
	hoveredCell  *CellID // transient hover cell in DISPLAY coords
	currentFocus CellID  // keyboard cursor in DISPLAY coords ({-1,-1} = cleared)
	focusSrcRow  int     // source row of the keyboard cursor (-1 = unset), for re-mapping across reindex
	focused      bool
	offset       geo.Position
	lastModifier event.KeyModifier
	anchorRow    int // display-row anchor for shift+click range selection
	anchorCol    int // absolute-column anchor for a shift+click cell rectangle

	// Editing state
	editingCell  *CellID
	editWidget   core.CanvasObject
	editClip     *bellui.Container // clips editWidget to the visible body (#1522)
	editFactory  cell.EditorFactory
	editOldValue any
	undoStack    []undoEntry
	undoPos      int // points to next undo slot

	collapsedGroups collapsedGroupsMap
	expandedDetails expandedDetails           // source-row keyed expansion set
	detailContent   map[int]core.CanvasObject // source-row keyed detail content cache
	loadingOverlay  *render.LoadingOverlay
	activeTooltip   *widget.PopUp

	dragCol          int
	dragRow          int     // for row drag reorder (-1 = none)
	dragPressY       float32 // where the press that armed dragRow went down
	dragStartSize    float32
	nearResizeBorder bool         // true when mouse is near a column header border
	resizeCol        int          // column index being resize-dragged (-1 = none)
	reorderFromCol   int          // column being dragged to reorder (-1 = none)
	dragStartPos     geo.Position // mouse position at drag start
	lastMousePos     geo.Position
	// mousePressed is set while a mouse button is down. Drags without it
	// come from touch, where the mouse gestures (resize, reorder, row drag)
	// cannot start, and scroll the grid instead.
	mousePressed bool
	// pressDragged is set once the mouse press under way has dragged. A press
	// that ends without it was a click, and its release disarms what the
	// press armed (disarmPress, #1511).
	pressDragged bool

	// disabled makes the grid visible but inert (Disable). scrollDir is the
	// body scroller's direction, kept while disabled sets it to ScrollNone.
	disabled  bool
	scrollDir container.ScrollDirection
	// rendered is set once CreateRenderer has run: before that nothing in
	// the grid can hold a focus, and no app need exist.
	rendered bool

	// swipe is the row a finger swipes, or that is open (#1483).
	swipe rowSwipe
	// rtl is whether the grid was last laid out right to left, which puts the
	// leading swipe actions on the right. Event handlers read it, since the
	// layout direction is only known during a layout.
	rtl bool
}

// filterState holds the active filter for a column.
type filterState struct {
	matches func(value any) bool
}

// undoEntry records a cell edit for undo/redo.
type undoEntry struct {
	Cell     CellID
	OldValue any
	NewValue any
}

// resolvedColumn is a flattened, resolved column definition with runtime state.
type resolvedColumn struct {
	def          *ColumnDef
	index        int // original index in ColumnDefs
	displayIndex int // index in display order (after pinning/reordering)
	width        float32
	renderer     cell.RendererFactory
	typeName     string
	// builtin marks a renderer the grid picked from the column type. A
	// column's CellRenderer is the app's, and its cells are app content: the
	// disabled look leaves them alone.
	builtin bool
}

// NewDataGrid creates a new DataGrid with the given column definitions and data callbacks.
func NewDataGrid(columnDefs []*ColumnDef, length func() int, rowData func(row int) any) *DataGrid {
	g := &DataGrid{
		ColumnDefs:     columnDefs,
		Length:         length,
		RowData:        rowData,
		SelectionMode:  SelectionSingleCell,
		selectedRows:   make(map[int]bool),
		selCellRow:     -1,
		focusSrcRow:    -1,
		filterModel:    make(map[string]filterState),
		dragCol:        -1,
		dragRow:        -1,
		resizeCol:      -1,
		reorderFromCol: -1,
		swipe:          rowSwipe{src: -1},
	}
	g.ExtendBaseWidget(g)
	g.init()
	return g
}

func (g *DataGrid) init() {
	// Create pipeline from callbacks
	g.pipe = pipeline.New(pipeline.NewCallbackSource(g.Length, g.RowData))

	// Resolve columns
	g.resolveColumns()

	// Create viewport
	g.viewport = &render.Viewport{
		DefaultRowHeight: g.rowHeight(),
		Padding:          g.cellPadding(),
	}
	g.updateViewportGeometry()

	// Create cell pool and register column types
	g.pool = render.NewCellPool()
	for _, col := range g.resolvedCols {
		g.pool.RegisterType(col.typeName, col.renderer.CreateCell)
	}

	// Create render widgets
	g.cellsWidget = render.NewCellsWidget(g, g.viewport, g.pool)
	g.headerWidget = render.NewHeaderWidget(g, g.viewport)
	g.pinnedLeftCells = render.NewPinnedCellsWidget(g, render.PinnedLeft, g.pool, g.viewport)
	g.pinnedRightCells = render.NewPinnedCellsWidget(g, render.PinnedRight, g.pool, g.viewport)
	g.groupRowOverlay = render.NewGroupRowOverlay(g, g.viewport)
	g.rowSwipeLayer = newRowSwipeLayer(g)
	g.rowLiftLayer = newRowLiftLayer(g)
	g.dropLine = canvas.NewRectangle(color.Transparent)
	g.dropLine.Hide()
	g.scrollContent = render.NewScrollContainer(g.cellsWidget)
	g.scrollContent.SetOnScrolled(g.onScrolled)

	// Set up pagination if enabled
	if g.Options.Pagination && g.Options.PageSize > 0 {
		g.paginateTransform = pipeline.NewPaginateTransform(g.Options.PageSize)
	}

	// Create API
	g.api = &GridAPI{grid: g}

	// Create grid theme
	th := g.Theme()
	g.gridTheme = gridTheme.NewGridTheme(th, g.Options.Density)
}

// ResolveColumns re-resolves column definitions after ColumnDefs is modified.
// Call this after changing ColumnDefs to update the grid's internal column state.
func (g *DataGrid) ResolveColumns() {
	g.resolveColumns()
	g.refreshAfterColumnChange()
}

func (g *DataGrid) resolveColumns() {
	// Preserve existing column widths so resize state survives re-resolution.
	oldWidths := make(map[string]float32)
	for _, col := range g.resolvedCols {
		oldWidths[col.def.ID()] = col.width
	}

	g.resolvedCols = nil
	idx := 0
	for _, def := range g.ColumnDefs {
		if def.IsGroup() {
			g.flattenGroup(def, &idx)
			continue
		}
		g.resolvedCols = append(g.resolvedCols, g.resolveColumn(def, idx))
		idx++
	}

	// Restore preserved widths.
	for _, col := range g.resolvedCols {
		if w, ok := oldWidths[col.def.ID()]; ok {
			col.width = w
		}
	}

	// Register any new column types in the pool.
	if g.pool != nil {
		for _, col := range g.resolvedCols {
			g.pool.RegisterType(col.typeName, col.renderer.CreateCell)
		}
	}
}

func (g *DataGrid) flattenGroup(group *ColumnDef, idx *int) {
	// If group is collapsed, skip its children
	if g.collapsedGroups != nil && g.collapsedGroups[group.ID()] {
		return
	}
	for _, child := range group.Children {
		if child.IsGroup() {
			g.flattenGroup(child, idx)
			continue
		}
		g.resolvedCols = append(g.resolvedCols, g.resolveColumn(child, *idx))
		*idx++
	}
}

func (g *DataGrid) resolveColumn(def *ColumnDef, index int) *resolvedColumn {
	col := &resolvedColumn{
		def:   def,
		index: index,
	}

	// Resolve width
	col.width = def.Width
	if col.width == 0 && g.DefaultColDef != nil && g.DefaultColDef.Width > 0 {
		col.width = g.DefaultColDef.Width
	}
	if col.width == 0 {
		col.width = 100 // default column width
	}

	// Resolve renderer
	if def.CellRenderer != nil {
		col.renderer = def.CellRenderer
	} else {
		col.builtin = true
		switch def.Type {
		case ColumnNumber:
			col.renderer = cell.NewNumberRenderer()
		case ColumnBoolean:
			col.renderer = cell.NewBooleanRenderer()
		case ColumnDate:
			col.renderer = cell.NewDateRenderer()
		default:
			col.renderer = cell.NewTextRenderer()
		}
	}

	// Resolve type name for pool keying
	col.typeName = fmt.Sprintf("col_%s_%d", def.ID(), def.Type)

	return col
}

func (g *DataGrid) updateViewportGeometry() {
	layout := g.visibleLayout()
	g.viewport.TotalRows = g.pipe.DisplayLength()

	// The viewport drives the main scroll area only — it should contain
	// only the main (non-pinned) columns. Pinned columns are rendered
	// separately by PinnedCellsWidget.
	mainWidths := make([]float32, len(layout.main))
	for i, col := range layout.main {
		mainWidths[i] = col.width
	}
	g.viewport.TotalCols = len(mainWidths)
	g.viewport.ColWidths = mainWidths
	g.viewport.Leading = g.selectionColumnWidth()
	g.viewport.RecomputeColPositions()
	g.viewport.DefaultRowHeight = g.rowHeight()
	g.viewport.Padding = g.cellPadding()
	g.buildRowHeights()
	g.viewport.RecomputeRowPositions()
}

// buildRowHeights populates the viewport's per-row height overrides, keyed by
// display row. Two sparse, source-keyed inputs are folded here: Options.
// RowHeightFunc (arbitrary per-row heights, pre-rebrand tracker #11) and master-detail expansion
// (an extra DetailHeight band beneath expanded rows). Only rows that differ from
// the default are stored, so a grid with neither input stays on the uniform O(1)
// path. When either is active the loop is O(n) over display rows, which is the
// cost of the prefix-sum rebuild the viewport needs anyway; RowHeightFunc is
// invoked once per row only when it is set.
func (g *DataGrid) buildRowHeights() {
	hasRowFunc := g.Options.RowHeightFunc != nil
	mdActive := g.masterDetailActive() && len(g.expandedDetails) > 0
	if !hasRowFunc && !mdActive {
		g.viewport.RowHeights = nil
		return
	}
	n := g.viewport.TotalRows
	def := g.rowHeight()
	idx := g.pipe.Index()
	var heights map[int]float32
	for d := 0; d < n; d++ {
		src := idx.SourceRow(d)
		if src < 0 {
			continue
		}
		h := def
		if hasRowFunc {
			if rh := g.Options.RowHeightFunc(src); rh > 0 {
				h = rh // <= 0 means "use the default"
			}
		}
		if mdActive && g.expandedDetails[src] {
			h += g.MasterDetail.DetailHeight
		}
		if h == def {
			continue
		}
		if heights == nil {
			heights = make(map[int]float32)
		}
		heights[d] = h
	}
	g.viewport.RowHeights = heights
}

func (g *DataGrid) rowHeight() float32 {
	if g.Options.RowHeight > 0 {
		return g.Options.RowHeight
	}
	return gridTheme.DefaultSize(gridTheme.SizeNameRowHeight, g.Options.Density)
}

func (g *DataGrid) cellPadding() float32 {
	return gridTheme.DefaultSize(gridTheme.SizeNameCellPaddingH, g.Options.Density)
}

func (g *DataGrid) onScrolled(pos geo.Position) {
	// A scroll closes an open row; the row a finger is swiping does not
	// scroll.
	if g.swipe.gesture != gestureSwipe {
		g.closeSwipe(true)
	}
	g.offset = geo.NewPos(g.flipScrollX(pos.X), pos.Y) // from the leading edge (#779)
	g.viewport.Update(g.offset, g.scrollContent.Scroll.Size())
	g.noteVisibleRows()
	g.cellsWidget.Refresh()

	// The header's main band scrolls horizontally in sync with the body, so it
	// must re-render on scroll to translate the main columns by the new offset.X.
	g.headerWidget.Refresh()

	// The floating-filter row shares the header's region layout, so it likewise
	// re-renders to keep its main-band entries above their columns.
	if g.floatingFilterRow != nil {
		g.floatingFilterRow.Refresh()
	}

	// Sync pinned cells vertical scroll
	g.pinnedLeftCells.SetOffsetY(pos.Y)
	g.pinnedRightCells.SetOffsetY(pos.Y)

	// The group-header overlay shares the body's vertical scroll. Its label is
	// fixed horizontally (it spans the full row across all bands), but its
	// aggregates sit in their columns, so the refresh SetOffsetY triggers also
	// moves them with the horizontal scroll.
	g.groupRowOverlay.SetOffsetY(pos.Y)

	// The inline editor overlay is anchored to its cell, so it must translate
	// with the new offset (and hide when the cell leaves the body).
	g.layoutEditOverlay(g.Size())
}

// noteVisibleRows tells OnVisibleRowsChanged the viewport's visible rows, if
// they differ from what it was last told. Call after each viewport.Update.
func (g *DataGrid) noteVisibleRows() {
	vis := g.viewport.VisibleRows
	if g.rowsReported && vis == g.reportedRows {
		return
	}
	g.reportedRows, g.rowsReported = vis, true
	if g.OnVisibleRowsChanged != nil {
		g.OnVisibleRowsChanged(vis.Min, vis.Max)
	}
}

// CreateRenderer returns the DataGrid's renderer.
func (g *DataGrid) CreateRenderer() core.WidgetRenderer {
	g.ExtendBaseWidget(g)
	g.rendered = true
	return newDataGridRenderer(g)
}

// API returns the programmatic control API for this grid.
func (g *DataGrid) API() *GridAPI {
	return g.api
}

// RefreshData re-reads data from the callbacks and refreshes the display.
func (g *DataGrid) RefreshData() {
	// The records may have moved under their source rows, so an open row
	// closes rather than follow a row that may now be another.
	g.closeSwipe(false)
	g.detailContent = nil // data changed; drop cached detail objects
	g.pipe.MarkDirty()
	g.updateViewportGeometry()
	g.viewport.Update(g.offset, g.scrollContent.Scroll.Size())
	g.noteVisibleRows()
	g.refreshRowBands()
}

// refreshRowBands redraws every band that draws rows: the cells, the column
// header, the pinned cells and the group-header overlay. RefreshData and
// rebuildPipeline change which rows show without a layout pass, and layout is
// otherwise the only place the pinned bands and the overlay are refreshed, so
// a grouped grid's headers kept the rows they last laid out (#1254).
func (g *DataGrid) refreshRowBands() {
	g.cellsWidget.Refresh()
	g.headerWidget.Refresh()
	g.pinnedLeftCells.Refresh()
	g.pinnedRightCells.Refresh()
	g.groupRowOverlay.Refresh()
}

// --- GridAccess interface implementation (for render.CellsWidget) ---

// visibleCol returns the resolvedColumn at the given visible display index
// (across ALL visible columns: pinned-left + main + pinned-right).
func (g *DataGrid) visibleCol(displayCol int) *resolvedColumn {
	layout := g.visibleLayout()
	if displayCol < 0 || displayCol >= len(layout.all) {
		return nil
	}
	return layout.all[displayCol]
}

// mainCol returns the resolvedColumn at the given main-column index
// (0-based into the non-pinned main columns only). This is what the
// CellsRenderer uses since the viewport only contains main columns.
func (g *DataGrid) mainCol(mainIdx int) *resolvedColumn {
	layout := g.visibleLayout()
	if mainIdx < 0 || mainIdx >= len(layout.main) {
		return nil
	}
	return layout.main[mainIdx]
}

// ColumnCount returns the number of main (non-pinned) columns.
// This is what the CellsRenderer/viewport uses.
func (g *DataGrid) ColumnCount() int {
	return len(g.visibleLayout().main)
}

func (g *DataGrid) RowCount() int {
	return g.pipe.DisplayLength()
}

// ColumnTypeName returns the type for a main-column index (used by CellsRenderer).
func (g *DataGrid) ColumnTypeName(col int) string {
	rc := g.mainCol(col)
	if rc == nil {
		return "default"
	}
	return rc.typeName
}

// CreateCell creates a cell for a main-column index (used by CellsRenderer).
func (g *DataGrid) CreateCell(col int) core.CanvasObject {
	rc := g.mainCol(col)
	if rc == nil {
		return nil
	}
	return rc.renderer.CreateCell()
}

// UpdateCell updates a cell for a main-column index (used by CellsRenderer).
func (g *DataGrid) UpdateCell(row, col int, obj core.CanvasObject) {
	rc := g.mainCol(col)
	if rc == nil {
		return
	}
	rowData := g.pipe.DisplayRow(row)
	if rowData == nil {
		return
	}

	var value any
	if rc.def.ValueGetter != nil {
		value = rc.def.ValueGetter(rowData)
	} else {
		value = defaultValueGetter(rowData, rc.def.Field)
	}

	if rc.def.ValueFormatter != nil {
		value = rc.def.ValueFormatter(value)
	}

	rc.renderer.UpdateCell(obj, cell.Params{
		RowIndex:  row,
		ColIndex:  col,
		Value:     value,
		RowData:   rowData,
		CellWidth: rc.width,
		// col is a main-column index; selection and a CellID use the
		// absolute display index, pinned-left columns first (#1401).
		IsSelected: g.isCellSelected(row, g.PinnedLeftCount()+col),
		IsHovered:  g.isCellHovered(row, g.PinnedLeftCount()+col),
	})
	g.dimBuiltinCell(rc, obj)
}

func (g *DataGrid) RowHeight() float32 {
	return g.rowHeight()
}

func (g *DataGrid) CellPadding() float32 {
	return g.cellPadding()
}

// ColumnWidths returns widths for main (non-pinned) columns.
func (g *DataGrid) ColumnWidths() []float32 {
	layout := g.visibleLayout()
	widths := make([]float32, len(layout.main))
	for i, col := range layout.main {
		widths[i] = col.width
	}
	return widths
}

func (g *DataGrid) IsSelected(row, col int) bool {
	return g.isCellSelected(row, col)
}

// IsFocused reports whether the cell at the given display row and absolute
// column is the keyboard-focus cursor cell while the grid holds focus. It backs
// the cell host's core.StateFocused (render.GridAccess) so the focused cell is
// the one assistive tech announces as the cursor moves, and it stays consistent
// with AccessibleActiveDescendant (both gate on g.focused and a live cursor). A
// cleared cursor ({-1,-1}) or an unfocused grid matches nothing.
func (g *DataGrid) IsFocused(row, col int) bool {
	return g.focused && row >= 0 && col >= 0 &&
		g.currentFocus.Row == row && g.currentFocus.Col == col
}

// SelectCell selects the cell at the given display row and absolute (layout)
// column, as a pointer tap would. It backs the cell host's accessible "select"
// action (render.cellHost); a negative column — the checkbox host's sentinel —
// selects the row via its first column.
func (g *DataGrid) SelectCell(row, col int) {
	if col < 0 {
		col = 0
	}
	g.selectCell(CellID{Row: row, Col: col})
}

// ScrollToRow scrolls vertically so the given display row is inside the
// viewport, without changing selection, focus, or the horizontal offset.
// SelectCell moves the selection but never the viewport; pair it with
// ScrollToRow (and DisplayRow, when starting from a source index) to reveal a
// programmatically selected row. An out-of-range row is ignored.
func (g *DataGrid) ScrollToRow(displayRow int) {
	if displayRow < 0 || displayRow >= g.RowCount() {
		return
	}
	// Col -1 never maps into the main band, so scrollToCell moves only the
	// vertical axis.
	g.scrollToCell(CellID{Row: displayRow, Col: -1})
}

func (g *DataGrid) IsHovered(row, col int) bool {
	return g.isCellHovered(row, col)
}

// --- Extended GridAccess methods ---

// pageRowOffset is the number of pre-paginate display rows preceding the current
// page — the offset to add to a page-local display row before indexing the
// transform metadata slices (GroupTransform.GroupRows / TreeTransform.TreeNodes),
// which are built over the full pre-paginate output while the visible display
// rows are only the current page's slice of it. Zero when pagination is inactive.
func (g *DataGrid) pageRowOffset() int {
	if !g.IsPaginated() || g.paginateTransform.PageSize <= 0 {
		return 0
	}
	return g.paginateTransform.CurrentPage * g.paginateTransform.PageSize
}

func (g *DataGrid) IsGroupRow(row int) bool {
	if g.groupTransform == nil {
		return false
	}
	row += g.pageRowOffset()
	if row < 0 || row >= len(g.groupTransform.GroupRows) {
		return false
	}
	return g.groupTransform.GroupRows[row].IsGroup
}

func (g *DataGrid) GroupRowInfo(row int) (key string, childCount int, expanded bool, depth int) {
	if g.groupTransform == nil {
		return "", 0, false, 0
	}
	row += g.pageRowOffset()
	if row < 0 || row >= len(g.groupTransform.GroupRows) {
		return "", 0, false, 0
	}
	gr := g.groupTransform.GroupRows[row]
	return fmt.Sprint(gr.GroupKey), gr.ChildCount, gr.Expanded, gr.Depth
}

func (g *DataGrid) ShowCheckboxColumn() bool {
	return g.Options.CheckboxSelection
}

// selectionColumnWidth is the width of the checkbox-selection column, the first
// column of the scrolling main band, or 0 when the grid does not show it. The
// main columns start this far into the band (#1498).
func (g *DataGrid) selectionColumnWidth() float32 {
	if g.ShowCheckboxColumn() {
		return render.SelectionColumnWidth
	}
	return 0
}

// ToggleRowSelected is what a click on display row's select checkbox does
// (render.RowSelectionToggler), as in AG Grid (#1504). With SelectionMultiRow
// it adds the row to the selection or removes it, keeping the others. With
// SelectionSingleRow it selects the row in place of the selected one, or clears
// it if it was the selected one. In a cell mode the box stands for the row's
// first cell, and a click selects that cell as a press on it does.
func (g *DataGrid) ToggleRowSelected(row int) {
	src := g.sourceRow(row)
	if g.disabled || src < 0 {
		g.Refresh() // the box toggled itself: show the row's state again
		return
	}
	id := CellID{Row: row, Col: 0}
	switch {
	case g.SelectionMode == SelectionMultiRow:
		g.selectCellWithModifiers(id, event.KeyModifierControl) // Ctrl toggles one row
	case g.SelectionMode == SelectionSingleRow && g.selectedRows[src]:
		g.selectedRows = make(map[int]bool)
		g.fireSelectionChanged()
		g.Refresh()
	case g.SelectionMode == SelectionNone:
		g.Refresh() // nothing is selectable: show the row's state again
	default:
		g.selectCellWithModifiers(id, 0)
	}
}

// SelectAllState reports whether the header shows a select-all box in the
// checkbox-selection column, and whether every row of the grid or only some
// are selected (render.SelectAllAccess). It counts every source row, those a
// filter hides and those on other pages too, as the box selects them. The box
// shows only when several rows can be selected.
func (g *DataGrid) SelectAllState() (shown, all, some bool) {
	if !g.ShowCheckboxColumn() || g.SelectionMode != SelectionMultiRow {
		return false, false, false
	}
	n := g.TotalRowCount()
	selected := 0
	for s, on := range g.selectedRows {
		if on && s >= 0 && s < n {
			selected++
		}
	}
	return true, n > 0 && selected == n, selected > 0
}

// SetAllRowsSelected backs the header's select-all box. true selects every row
// of the grid, those a filter hides and those on other pages too, as AG Grid's
// header checkbox does by default (selectAll 'all'); GridAPI.SelectAll selects
// only the displayed rows. false clears the row selection. Either reports the
// change through OnSelectionChanged.
func (g *DataGrid) SetAllRowsSelected(selected bool) {
	g.selectedRows = make(map[int]bool)
	if selected {
		for s, n := 0, g.TotalRowCount(); s < n; s++ {
			g.selectedRows[s] = true
		}
	}
	g.fireSelectionChanged()
	g.Refresh()
}

// ToggleGroup expands or collapses the row group with the given key and rebuilds
// the pipeline so the change is reflected. It is the single group-toggle entry
// point shared by the public GridAPI, a pointer tap on the group-header row
// (Tapped), Enter/Space on a focused header, and the header row's accessible
// expand/collapse action (see render.groupRowA11y).
func (g *DataGrid) ToggleGroup(groupKey string) {
	if g.groupTransform == nil {
		return
	}
	g.groupTransform.ToggleGroup(groupKey)
	g.rebuildPipeline()
}

// --- Master-detail GridAccess methods ---

// DetailToggleWidth returns the width reserved at the left of the first main
// column for the disclosure chevron, or 0 when master-detail is inactive. It is
// the single source of truth shared by the renderer (indent + chevron) and the
// hit-test (toggle click zone).
func (g *DataGrid) DetailToggleWidth() float32 {
	if !g.masterDetailActive() {
		return 0
	}
	return detailToggleWidth
}

// IsMasterRow reports whether the given display row is an expandable master row.
func (g *DataGrid) IsMasterRow(row int) bool {
	if !g.masterDetailActive() || g.IsGroupRow(row) {
		return false
	}
	return g.isRowMaster(g.sourceRow(row))
}

// DetailBandHeight returns the height of the expanded detail band beneath the
// given display row's cells (0 when the row is not an expanded master row). The
// row's data cells occupy vp.RowY height minus this band.
func (g *DataGrid) DetailBandHeight(row int) float32 {
	if !g.masterDetailActive() {
		return 0
	}
	if !g.isSourceExpanded(g.sourceRow(row)) {
		return 0
	}
	return g.MasterDetail.DetailHeight
}

// DetailContent returns the (cached) detail CanvasObject for the given display
// row's band, or nil when the row is not expanded. Content is created lazily and
// cached by source row so it keeps its state across scroll/refresh.
func (g *DataGrid) DetailContent(row int) core.CanvasObject {
	if g.DetailBandHeight(row) <= 0 {
		return nil
	}
	src := g.sourceRow(row)
	if src < 0 {
		return nil
	}
	if g.detailContent == nil {
		g.detailContent = make(map[int]core.CanvasObject)
	}
	obj, ok := g.detailContent[src]
	if !ok {
		obj = g.MasterDetail.DetailRenderer(src)
		g.detailContent[src] = obj
	}
	return obj
}

func (g *DataGrid) ColSpanAt(row, col int) int {
	return g.colSpanAt(row, col)
}

func (g *DataGrid) PinnedLeftCount() int {
	return len(g.visibleLayout().pinnedLeft)
}

func (g *DataGrid) PinnedRightCount() int {
	return len(g.visibleLayout().pinnedRight)
}

// PinnedColumnTypeName is the type name of the column at display index
// displayCol: pinned column access uses the absolute display index
// (visibleCol), not mainCol.
func (g *DataGrid) PinnedColumnTypeName(displayCol int) string {
	rc := g.visibleCol(displayCol)
	if rc == nil {
		return "default"
	}
	return rc.typeName
}

func (g *DataGrid) PinnedCreateCell(displayCol int) core.CanvasObject {
	rc := g.visibleCol(displayCol)
	if rc == nil {
		return nil
	}
	return rc.renderer.CreateCell()
}

func (g *DataGrid) PinnedUpdateCell(row, displayCol int, obj core.CanvasObject) {
	rc := g.visibleCol(displayCol)
	if rc == nil {
		return
	}
	rowData := g.pipe.DisplayRow(row)
	if rowData == nil {
		return
	}
	var value any
	if rc.def.ValueGetter != nil {
		value = rc.def.ValueGetter(rowData)
	} else {
		value = defaultValueGetter(rowData, rc.def.Field)
	}
	if rc.def.ValueFormatter != nil {
		value = rc.def.ValueFormatter(value)
	}
	rc.renderer.UpdateCell(obj, cell.Params{
		RowIndex:   row,
		ColIndex:   displayCol,
		Value:      value,
		RowData:    rowData,
		CellWidth:  rc.width,
		IsSelected: g.isCellSelected(row, displayCol), // a pinned cell is drawn selected too (#1401)
		IsHovered:  g.isCellHovered(row, displayCol),
	})
	g.dimBuiltinCell(rc, obj)
}

// --- HeaderAccess interface implementation (for render.HeaderWidget) ---
// The header uses visibleCol (all columns) not mainCol, because it renders
// all columns including pinned ones.

func (g *DataGrid) HeaderColumnCount() int {
	return g.visibleLayout().visibleColumnCount()
}

func (g *DataGrid) HeaderColumnWidths() []float32 {
	return g.visibleLayout().columnWidths()
}

func (g *DataGrid) ColumnHeader(col int) string {
	rc := g.visibleCol(col)
	if rc == nil {
		return ""
	}
	return rc.def.Header()
}

// ColumnHeaderTextHidden reports whether the column's header cell paints no
// text (ColumnDef.HideHeaderText); ColumnHeader still names it.
func (g *DataGrid) ColumnHeaderTextHidden(col int) bool {
	rc := g.visibleCol(col)
	return rc != nil && rc.def.HideHeaderText
}

func (g *DataGrid) HeaderHeight() float32 {
	if g.Options.HeaderHeight > 0 {
		return g.Options.HeaderHeight
	}
	return gridTheme.DefaultSize(gridTheme.SizeNameHeaderHeight, g.Options.Density)
}

func (g *DataGrid) SortState(col int) render.SortIndicator {
	rc := g.visibleCol(col)
	if rc == nil {
		return render.SortIndicatorNone
	}
	colID := rc.def.ID()
	for _, s := range g.sortModel {
		if s.ColID == colID {
			switch s.Direction {
			case SortAscending:
				return render.SortIndicatorAsc
			case SortDescending:
				return render.SortIndicatorDesc
			}
		}
	}
	return render.SortIndicatorNone
}

// ColumnSortable reports whether a header tap on the column at the given
// absolute display index would sort it — the same gate OnHeaderTapped applies.
// It backs the header cell's accessible "press" (sort) action.
func (g *DataGrid) ColumnSortable(col int) bool {
	rc := g.visibleCol(col)
	return rc != nil && rc.def.isSortable(g.DefaultColDef)
}

func (g *DataGrid) OnHeaderTapped(col int, modifier event.KeyModifier) {
	g.endEditOnPress()
	rc := g.visibleCol(col)
	if rc == nil {
		return
	}
	if !rc.def.isSortable(g.DefaultColDef) {
		return
	}
	order := rc.def.sortingOrder(g.DefaultColDef)
	if modifier&event.KeyModifierShift != 0 {
		g.multiSortColumn(rc.def.ID(), order)
	} else {
		g.cycleSortColumn(rc.def.ID(), order)
	}
}

func (g *DataGrid) HeaderBorderCol(x float32) int {
	return g.headerBorderCol(x)
}

// HeaderRegions exposes the body's column-band geometry to the header renderer
// so the header lays each band out where the body draws it.
func (g *DataGrid) HeaderRegions() (mainX, pinnedRightX, pinnedRightW float32) {
	return g.hitRegions()
}

// ScrollOffsetX is the main band's horizontal scroll offset, so the header's
// main columns can translate in sync with the body content.
func (g *DataGrid) ScrollOffsetX() float32 {
	return g.offset.X
}

// ColumnAtX maps a widget-X to an absolute display column, region-aware — the
// single coordinate model shared by the header and the body's hit-testing.
func (g *DataGrid) ColumnAtX(x float32) int {
	return g.columnAtX(x)
}

func (g *DataGrid) OnResizeDragStart(col int, startX float32) {
	g.endEditOnPress()
	rc := g.visibleCol(col)
	if rc == nil {
		return
	}
	g.resizeCol = col
	g.dragStartSize = rc.width
	g.dragStartPos = geo.NewPos(startX, 0)
}

// pinnedMaxWidth is the widest the pinned column rc can be while both pinned
// bands, with their gaps and the vertical bar's gutter that pinned-right keeps
// clear (#1257), fit in the grid.
func (g *DataGrid) pinnedMaxWidth(rc *resolvedColumn) float32 {
	layout := g.visibleLayout()
	padding := g.cellPadding()
	used := g.vScrollGutter(g.Size())
	for _, band := range [][]*resolvedColumn{layout.pinnedLeft, layout.pinnedRight} {
		for _, col := range band {
			used += padding
			if col != rc {
				used += col.width
			}
		}
	}
	return g.Size().Width - used
}

func (g *DataGrid) OnResizeDrag(posX float32) {
	if g.resizeCol < 0 {
		return
	}
	rc := g.visibleCol(g.resizeCol)
	if rc == nil {
		return
	}
	newWidth := g.dragStartSize + g.resizeDelta(g.dragStartPos.X, posX)

	// Clamp to min — use the header text width as the floor so headers don't overlap.
	headerText := rc.def.Header()
	if rc.def.HideHeaderText {
		headerText = ""
	}
	headerMinW := bellui.MeasureText(headerText, g.Theme().Size(bellui.ThemeSizeName("text")),
		bellui.TextStyle{Bold: true}).Width + g.cellPadding()*2
	minW := headerMinW
	if minW < 40 {
		minW = 40
	}
	if rc.def.MinWidth > 0 && rc.def.MinWidth > minW {
		minW = rc.def.MinWidth
	}
	if newWidth < minW {
		newWidth = minW
	}

	// Clamp to explicit max
	if rc.def.MaxWidth > 0 && newWidth > rc.def.MaxWidth {
		newWidth = rc.def.MaxWidth
	}

	// A main column has no cap on the grid's width: it scrolls with the body,
	// which scrolls sideways to its border however wide it grows. A pinned
	// column does not scroll, so it stops where the two pinned bands fill the
	// grid: past that they would overlap, and a pinned-left column's border
	// would leave the grid, where it could not be dragged back.
	// The cap used to count the main columns as well, which made it negative
	// once they overflowed the grid, so every drag in a grid that scrolls
	// sideways collapsed the column to its minimum (#1499).
	if rc.def.Pinned == PinLeft || rc.def.Pinned == PinRight {
		if maxFit := g.pinnedMaxWidth(rc); newWidth > maxFit {
			newWidth = geo.Max(maxFit, minW)
		}
	}

	rc.width = newWidth
	g.refreshAfterColumnChange()
}

// refreshAfterColumnChange updates the grid after column width/visibility changes.
func (g *DataGrid) refreshAfterColumnChange() {
	g.updateViewportGeometry()
	g.Refresh()
}

func (g *DataGrid) OnResizeDragEnd() {
	g.resizeCol = -1
}

// IsFilterActive returns true if the given column has an active filter.
func (g *DataGrid) IsFilterActive(col int) bool {
	rc := g.visibleCol(col)
	if rc == nil {
		return false
	}
	_, ok := g.filterModel[rc.def.ID()]
	return ok
}

// --- Sort and Filter ---

// cycleSortColumn cycles the sort direction for the given column ID: None → Asc → Desc → None.
// In single-sort mode (no shift held), clears other sorts. Multi-sort preserves them.
func (g *DataGrid) cycleSortColumn(colID string, order []SortDirection) {
	// Single-sort mode: if the model has other columns, replace entirely.
	// If this column is already the sole sort, it moves along its order
	// (by default Asc → Desc → None; ColumnDef.SortingOrder).
	cur := SortNone
	if len(g.sortModel) == 1 && g.sortModel[0].ColID == colID {
		cur = g.sortModel[0].Direction
	}
	if next := nextDirection(order, cur); next == SortNone {
		g.sortModel = nil
	} else {
		g.sortModel = []SortItem{{ColID: colID, Direction: next}}
	}

	g.rebuildPipeline()

	if g.OnSortChanged != nil {
		g.OnSortChanged(g.sortModel)
	}
}

// multiSortColumn adds or cycles a column in the multi-sort model (shift+click).
func (g *DataGrid) multiSortColumn(colID string, order []SortDirection) {
	idx := -1
	for i, s := range g.sortModel {
		if s.ColID == colID {
			idx = i
			break
		}
	}

	cur := SortNone
	if idx >= 0 {
		cur = g.sortModel[idx].Direction
	}
	switch next := nextDirection(order, cur); {
	case idx >= 0 && next == SortNone:
		g.sortModel = append(g.sortModel[:idx], g.sortModel[idx+1:]...)
	case idx >= 0:
		g.sortModel[idx].Direction = next
	case next != SortNone:
		g.sortModel = append(g.sortModel, SortItem{ColID: colID, Direction: next})
	}

	g.rebuildPipeline()

	if g.OnSortChanged != nil {
		g.OnSortChanged(g.sortModel)
	}
}

// setColumnFilter sets or clears a filter for the given column.
func (g *DataGrid) setColumnFilter(colID string, matches func(value any) bool) {
	if matches == nil {
		delete(g.filterModel, colID)
	} else {
		g.filterModel[colID] = filterState{matches: matches}
	}
	g.rebuildPipeline()
	if g.OnFilterChanged != nil {
		g.OnFilterChanged()
	}
}

// setQuickFilter sets a text search filter across all columns.
func (g *DataGrid) setQuickFilter(text string) {
	g.quickFilter = text
	g.rebuildPipeline()
	if g.OnFilterChanged != nil {
		g.OnFilterChanged()
	}
}

// buildTransforms assembles the pipeline transform chain from the current
// filter/sort/pivot/group state, in order. When includePaginate is true the
// paginate transform is appended (the live, current-page view); when false it is
// omitted, yielding the full pre-paginate order used to locate a row's global
// position and page (see pageForSource).
func (g *DataGrid) buildTransforms(includePaginate bool) []pipeline.Transform {
	var transforms []pipeline.Transform

	// 1. Column filters
	var colFilters []pipeline.ColumnFilter
	for colID, fs := range g.filterModel {
		getter := g.valueGetterForColID(colID)
		if getter == nil {
			continue
		}
		matches := fs.matches
		colFilters = append(colFilters, pipeline.ColumnFilter{
			ValueGetter: getter,
			Matches:     matches,
		})
	}

	// 2. Quick filter (text search across all columns)
	if g.quickFilter != "" {
		qf := g.buildQuickFilter(g.quickFilter)
		if qf != nil {
			colFilters = append(colFilters, *qf)
		}
	}

	if len(colFilters) > 0 {
		transforms = append(transforms, pipeline.NewFilterTransform(colFilters...))
	}

	// 3. Sort transforms (before group, since group inserts sentinel rows)
	if len(g.sortModel) > 0 {
		var sortFields []pipeline.SortField
		for _, si := range g.sortModel {
			getter := g.valueGetterForColID(si.ColID)
			if getter == nil {
				continue
			}
			comparator := g.comparatorForColID(si.ColID)
			sortFields = append(sortFields, pipeline.SortField{
				ValueGetter: getter,
				Ascending:   si.Direction == SortAscending,
				Comparator:  comparator,
			})
		}
		if len(sortFields) > 0 {
			transforms = append(transforms, pipeline.NewSortTransform(sortFields...))
		}
	}

	// 4. Pivot transform (after sort, produces new virtual rows)
	if g.pivotTransform != nil {
		transforms = append(transforms, g.pivotTransform)
	}

	// 5. Group transform (after sort so rows within groups are sorted)
	if g.groupTransform != nil {
		g.groupTransform.AggColumns = g.groupAggColumns()
		transforms = append(transforms, g.groupTransform)
	}

	// 5b. Tree transform (hierarchical parent/child ordering; an alternative to
	// grouping — after sort so siblings are sorted, before paginate).
	if g.treeTransform != nil {
		transforms = append(transforms, g.treeTransform)
	}

	// 6. Paginate transform
	if includePaginate && g.Options.Pagination && g.paginateTransform != nil {
		transforms = append(transforms, g.paginateTransform)
	}

	return transforms
}

// rebuildPipeline reconstructs the pipeline transforms from the current sort/filter state.
func (g *DataGrid) rebuildPipeline() {
	g.pipe.SetTransforms(g.buildTransforms(true))
	g.followCursorAcrossPages()
	g.remapCursorAfterReindex()
	g.updateViewportGeometry()
	g.viewport.Update(g.offset, g.scrollContent.Scroll.Size())
	g.noteVisibleRows()
	g.refreshRowBands()
	g.followSwipedRow()
}

// buildQuickFilter creates a cross-column filter that passes a row if any column
// value contains the search text (case-insensitive).
func (g *DataGrid) buildQuickFilter(text string) *pipeline.ColumnFilter {
	lower := toLower(text)
	getters := make([]func(any) any, len(g.resolvedCols))
	for i, rc := range g.resolvedCols {
		getters[i] = rc.def.valueGetter()
	}
	return &pipeline.ColumnFilter{
		ValueGetter: func(rowData any) any { return rowData },
		Matches: func(rowData any) bool {
			for _, getter := range getters {
				val := getter(rowData)
				if val == nil {
					continue
				}
				s := fmt.Sprint(val)
				if containsLower(s, lower) {
					return true
				}
			}
			return false
		},
	}
}

func (g *DataGrid) valueGetterForColID(colID string) func(any) any {
	for _, rc := range g.resolvedCols {
		if rc.def.ID() == colID {
			return rc.def.valueGetter()
		}
	}
	return nil
}

func (g *DataGrid) comparatorForColID(colID string) func(a, b any) int {
	for _, rc := range g.resolvedCols {
		if rc.def.ID() == colID {
			return rc.def.Comparator
		}
	}
	return nil
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsLower(s, lower string) bool {
	sl := toLower(s)
	if len(lower) > len(sl) {
		return false
	}
	for i := 0; i <= len(sl)-len(lower); i++ {
		if sl[i:i+len(lower)] == lower {
			return true
		}
	}
	return false
}

// --- Event handlers ---

// AccessibleRole reports the grid's semantic role to assistive technologies.
func (g *DataGrid) AccessibleRole() core.AccessibleRole {
	return core.RoleTable
}

// AccessibleLabel returns the grid's human-readable name, from
// Options.AccessibleLabel when set.
func (g *DataGrid) AccessibleLabel() string {
	return g.Options.AccessibleLabel
}

// AccessibleGridDimensions reports the grid's extent to assistive technologies
// (core.AccessibleGridDimensioner). The grid virtualizes — only visible cells
// are realized — so this is how a screen reader learns the full size. Rows is
// the current display-row count (after sort and filter, i.e. the "row R of N"
// the user hears), and cols the total visible column count in absolute display
// order — a leading checkbox-selection column (when shown) + pinned-left + main
// + pinned-right — matching the column space that every cell's
// AccessibleCellPosition uses. See the visual/current-view coordinate contract
// on core.AccessibleGridDimensioner.
func (g *DataGrid) AccessibleGridDimensions() (rows, cols int) {
	cols = len(g.visibleLayout().all)
	if g.ShowCheckboxColumn() {
		cols++ // leading checkbox-selection column occupies absolute index 0
	}
	return g.RowCount(), cols
}

// AccessibleCellText returns the full, untruncated text a screen reader should
// announce for the cell at the given display row and column — the column's
// ValueFormatter output when set, otherwise the raw value rendered plainly.
// row is a display coordinate (post sort/filter) and col is an ABSOLUTE display
// column index (pinned + main, left to right), matching the cell host's
// AccessibleCellPosition and AccessibleGridDimensions. Returns "" for an
// out-of-range cell.
func (g *DataGrid) AccessibleCellText(row, col int) string {
	rc := g.visibleCol(col)
	if rc == nil {
		return ""
	}
	rowData := g.pipe.DisplayRow(row)
	if rowData == nil {
		return ""
	}
	var value any
	if rc.def.ValueGetter != nil {
		value = rc.def.ValueGetter(rowData)
	} else {
		value = defaultValueGetter(rowData, rc.def.Field)
	}
	if rc.def.ValueFormatter != nil {
		return rc.def.ValueFormatter(value)
	}
	// With no explicit formatter, match the visible cell's formatting (Number
	// thousands separators, Date layout) rather than a raw fmt.Sprint, so a
	// screen reader announces what is shown. Falls back for renderers that don't
	// provide accessible text (e.g. sparklines).
	if tv, ok := rc.renderer.(cell.AccessibleValueTexter); ok {
		return tv.TextForValue(value)
	}
	return cell.AccessibleText(value)
}

// cellA11yID is a stable, comparable accessibility identity for a cell, keyed by
// the cell's SOURCE row (not its display position) plus its absolute display
// column, so a screen reader's cursor tracks the record across sort/filter/
// scroll. It flows onto core.AccessibleNode.NodeID for both realized cells
// (core.AccessibleIdentified) and off-screen virtual nodes
// (core.AccessibleVirtualNode.ID), unifying the two identity channels.
type cellA11yID struct {
	src int // source row identity
	col int // absolute display column
}

// AccessibleCellIdentity returns the stable identity for the cell at the given
// display row and absolute column, or nil for a synthetic row (group header)
// that has no source identity. See core.AccessibleIdentified.
func (g *DataGrid) AccessibleCellIdentity(row, col int) any {
	src := g.sourceRow(row)
	if src < 0 {
		return nil
	}
	return cellA11yID{src: src, col: col}
}

// AccessibleActiveDescendant returns the stable identity of the cell the
// keyboard cursor is on, letting the platform a11y bridge move screen-reader
// focus to that cell as the cursor travels — a focusable container designating
// its AT-focused descendant, the aria-activedescendant analog (pre-rebrand tracker #688). The
// value equals the focused cell's AccessibleCellIdentity, so it resolves through
// the same source-keyed node identity the realized host and off-screen virtual
// node already carry (core.AccessibleIdentified, pre-rebrand tracker #635); the bridge raises a UIA
// focus-change event when it changes.
//
// It is nil when the grid does not hold focus, the cursor is cleared, or the
// cursor sits on a synthetic row (e.g. a group header) whose cell has no source
// identity — the bridge then leaves AT focus on the grid container. The cursor
// is scrolled into the realized/virtual-margin window on every move (TypedKey ->
// scrollToCell), so the designated descendant is a reconciled node.
func (g *DataGrid) AccessibleActiveDescendant() any {
	if !g.focused || g.currentFocus.Row < 0 || g.currentFocus.Col < 0 {
		return nil
	}
	return g.AccessibleCellIdentity(g.currentFocus.Row, g.currentFocus.Col)
}

// AccessibleRealizeCell builds the semantic node for a single cell the screen
// reader has navigated to that is NOT in the realized viewport window, without
// materializing the rows between (core.AccessibleVirtualCellRealizer, pre-rebrand tracker #637).
// The grid virtualizes — only visible cells exist as real hosts — so this is the
// only way assistive tech reaches a cell in a million-row grid past the viewport.
//
// row and col are display coordinates in the current presentation order — the
// same space as AccessibleGridDimensions and every realized cell's
// AccessibleCellPosition (checkbox-selection column inserted at slot
// PinnedLeftCount; see render cellHost.AccessibleCellPosition). ok is false for
// an out-of-range coordinate. The returned node carries the same role, full
// label, selection/focus state and source-keyed identity a realized host would,
// so the AT reads an off-screen cell identically to an on-screen one and its
// cursor tracks the record across sort/scroll/filter. Reading has no visible
// side effect; the explicit bring-into-view path is AccessibleScrollCellIntoView.
func (g *DataGrid) AccessibleRealizeCell(row, col int) (core.AccessibleVirtualNode, bool) {
	rows, cols := g.AccessibleGridDimensions()
	if row < 0 || row >= rows || col < 0 || col >= cols {
		return core.AccessibleVirtualNode{}, false
	}
	layoutCol := g.layoutColForDisplay(col)

	var st core.AccessibleState
	if g.IsSelected(row, layoutCol) {
		st |= core.StateSelected
	}
	if g.IsFocused(row, layoutCol) {
		st |= core.StateFocused
	}

	pos, size := g.cellRealizeRect(row, layoutCol)
	return core.AccessibleVirtualNode{
		AccessibleNode: core.AccessibleNode{
			Role:            core.RoleTableCell,
			Label:           g.AccessibleCellText(row, layoutCol),
			State:           st,
			Actions:         []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this cell"}},
			HasCellPosition: true,
			CellRow:         row,
			CellCol:         col,
		},
		Pos:  pos,
		Size: size,
		ID:   g.AccessibleCellIdentity(row, layoutCol),
	}, true
}

// AccessibleScrollCellIntoView scrolls the grid so the cell at the given display
// (row, col) becomes visible and realized (core.AccessibleScrollIntoView,
// pre-rebrand tracker #637) — the explicit realize/activate path, kept separate from the read
// path so merely announcing an off-screen cell does not move the viewport. Once
// on-screen the cell exists as a real host, so its "select" action then routes
// normally. Out-of-range coordinates are ignored. Called on the main goroutine.
func (g *DataGrid) AccessibleScrollCellIntoView(row, col int) {
	if g.disabled {
		return // a disabled grid does not scroll (Disable)
	}
	rows, cols := g.AccessibleGridDimensions()
	if row < 0 || row >= rows || col < 0 || col >= cols {
		return
	}
	g.scrollToCell(CellID{Row: row, Col: g.layoutColForDisplay(col)})
}

// layoutColForDisplay maps an absolute display column — the coordinate space
// assistive tech navigates, with the checkbox-selection column inserted at slot
// PinnedLeftCount — back to the layout column index that AccessibleCellText /
// AccessibleCellIdentity / IsSelected / IsFocused use (an index into
// visibleLayout().all, or -1 for the checkbox column). It is the exact inverse of
// render cellHost.AccessibleCellPosition, so an off-viewport realized cell reads
// against the same data an on-screen host would. Callers pass an in-range col.
func (g *DataGrid) layoutColForDisplay(displayCol int) int {
	if !g.ShowCheckboxColumn() {
		return displayCol
	}
	p := g.PinnedLeftCount()
	switch {
	case displayCol == p:
		return -1 // the checkbox column occupies the slot right of pinned-left
	case displayCol > p:
		return displayCol - 1 // main + pinned-right shift one place right of it
	default:
		return displayCol // pinned-left columns keep their index
	}
}

// cellRealizeRect returns an off-viewport cell's rectangle relative to the grid
// origin in the current scroll state. A realized cell is always outside the
// viewport, so the meaningful axis is vertical: the row's live y (offset- and
// header-adjusted) places it clearly above or below the visible band, which is
// what a screen reader's spatial model expects. Horizontal geometry is exact for
// a main (horizontally-scrolling) column and a left-edge placeholder for the
// always-visible pinned-left / checkbox columns — precise where it matters and
// immaterial for an off-screen cell, whose exact bring-into-view is
// AccessibleScrollCellIntoView.
func (g *DataGrid) cellRealizeRect(row, layoutCol int) (geo.Position, geo.Size) {
	y, h := g.viewport.RowY(row)
	pos := geo.NewPos(0, g.bodyTop()+y-g.offset.Y)
	size := geo.NewSize(0, h)
	if mainCol := layoutCol - g.PinnedLeftCount(); mainCol >= 0 && mainCol < g.ColumnCount() {
		x, w := g.viewport.ColX(mainCol)
		pos.X = x - g.offset.X
		size.Width = w
	}
	pos.X = g.drawnX(pos.X, size.Width, g.Size().Width)
	return pos, size
}

// cellWidgetRect returns an on-screen cell's rectangle in widget-local
// coordinates in the current scroll state — the exact inverse of the pointer
// hit-test (cellAtPosition / columnAtX), band-aware where cellRealizeRect is
// deliberately approximate: pinned-left fixed at the left edge, the main band
// anchored at mainX and scrolled by offset.X, pinned-right a fixed overlay at
// pinnedRightX. It anchors the keyboard context-menu path, which scrolls the
// cell into view first, so exact geometry matters. id is in display
// coordinates (Col an absolute index into visibleLayout().all, like
// currentFocus); callers pass an in-range id.
func (g *DataGrid) cellWidgetRect(id CellID) (geo.Position, geo.Size) {
	y, h := g.viewport.RowY(id.Row)
	pos := geo.NewPos(0, g.bodyTop()+y-g.offset.Y)
	size := geo.NewSize(0, h)

	layout := g.visibleLayout()
	pad := g.cellPadding()
	mainX, pinnedRightX, _ := g.hitRegions()
	switch nLeft := len(layout.pinnedLeft); {
	case id.Col < nLeft:
		// Fixed pinned-left band at [0, mainX).
		pos.X, size.Width = colSpanIn(layout.pinnedLeft, id.Col, pad)
	case id.Col < nLeft+len(layout.main):
		// Main band: drawn at mainX, scrolled by offset.X.
		x, w := g.viewport.ColX(id.Col - nLeft)
		pos.X, size.Width = mainX+x-g.offset.X, w
	default:
		// Fixed pinned-right overlay at pinnedRightX.
		x, w := colSpanIn(layout.pinnedRight, id.Col-nLeft-len(layout.main), pad)
		pos.X, size.Width = pinnedRightX+x, w
	}
	pos.X = g.drawnX(pos.X, size.Width, g.Size().Width)
	return pos, size
}

// colSpanIn is the forward complement of colIndexIn: the band-local x and
// width of cols[i], measured from the band's left edge with the same
// width+padding accumulation. Callers pass an in-range i.
func colSpanIn(cols []*resolvedColumn, i int, pad float32) (x, width float32) {
	for j := 0; j < i; j++ {
		x += cols[j].width + pad
	}
	return x, cols[i].width
}

func (g *DataGrid) Tapped(e *event.PointEvent) {
	if g.disabled {
		return
	}
	// A touch screen sends a tap and no press.
	g.endEditOnPress()
	// A tap while a row is open only closes it (#1483).
	if g.swipe.src >= 0 {
		g.closeSwipe(true)
		return
	}
	if row := g.detailToggleAt(e.Position); row >= 0 {
		g.ToggleDetail(row)
		return
	}
	if row := g.treeToggleAt(e.Position); row >= 0 {
		g.ToggleTreeNode(row)
		return
	}
	if row := g.groupRowAt(e.Position); row >= 0 {
		key, _, _, _ := g.GroupRowInfo(row)
		g.ToggleGroup(key)
		return
	}
	id := g.cellAtPosition(e.Position)
	if id.Row < 0 || id.Col < 0 {
		return
	}
	// Selection is handled in MouseDown (which has modifier info).
	// Tapped fires the click callback only.
	if g.OnCellClicked != nil {
		g.OnCellClicked(id)
	}
}

func (g *DataGrid) DoubleTapped(e *event.PointEvent) {
	if g.disabled {
		return
	}
	id := g.cellAtPosition(e.Position)
	if id.Row < 0 || id.Col < 0 {
		return
	}
	if g.OnCellDoubleClicked != nil {
		g.OnCellDoubleClicked(id)
	}
	// Start editing on double-click
	g.StartEditing(id)
}

func (g *DataGrid) TappedSecondary(e *event.PointEvent) {
	if g.disabled {
		return
	}
	id := g.cellAtPosition(e.Position)

	// Check if we're in the header area
	headerH := g.HeaderHeight()
	if e.Position.Y < headerH {
		g.showHeaderContextMenu(e, id.Col)
		return
	}

	// Body cell: select, then hand off to the consumer's context-menu
	// callback, or, without one, list the row's swipe actions (#1483).
	if id.Row >= 0 && id.Col >= 0 {
		g.closeSwipe(true)
		g.selectCell(id)
		if g.OnCellSecondaryTapped != nil {
			g.OnCellSecondaryTapped(id, e)
		} else {
			g.showRowSwipeMenu(id.Row, e.AbsolutePosition)
		}
	}
}

func (g *DataGrid) showHeaderContextMenu(e *event.PointEvent, col int) {
	rc := g.visibleCol(col)
	if rc == nil {
		return
	}
	colID := rc.def.ID()

	items := make([]*bellui.MenuItem, 0, 6)

	if rc.def.isSortable(g.DefaultColDef) {
		items = append(
			items,
			bellui.NewMenuItem("Sort Ascending", func() {
				g.sortModel = []SortItem{{ColID: colID, Direction: SortAscending}}
				g.rebuildPipeline()
				if g.OnSortChanged != nil {
					g.OnSortChanged(g.sortModel)
				}
			}),
			bellui.NewMenuItem("Sort Descending", func() {
				g.sortModel = []SortItem{{ColID: colID, Direction: SortDescending}}
				g.rebuildPipeline()
				if g.OnSortChanged != nil {
					g.OnSortChanged(g.sortModel)
				}
			}),
		)
		// Only show "Clear Sort" if currently sorted
		for _, s := range g.sortModel {
			if s.ColID == colID {
				items = append(items, bellui.NewMenuItem("Clear Sort", func() {
					newModel := make([]SortItem, 0, len(g.sortModel))
					for _, s := range g.sortModel {
						if s.ColID != colID {
							newModel = append(newModel, s)
						}
					}
					g.sortModel = newModel
					g.rebuildPipeline()
					if g.OnSortChanged != nil {
						g.OnSortChanged(g.sortModel)
					}
				}))
				break
			}
		}
	}

	// Clear filter option if filter is active
	if _, ok := g.filterModel[colID]; ok {
		items = append(items, bellui.NewMenuItem("Clear Filter", func() {
			g.setColumnFilter(colID, nil)
		}))
	}

	// Filter popup
	if rc.def.isFilterable(g.DefaultColDef) {
		capturedCol := col
		items = append(items, bellui.NewMenuItem("Filter...", func() {
			g.ShowFilterPopup(capturedCol, e.AbsolutePosition)
		}))
	}

	// Pinning
	if rc.def.Pinned == PinNone {
		items = append(
			items,
			bellui.NewMenuItem("Pin Left", func() { g.SetColumnPinned(colID, PinLeft) }),
			bellui.NewMenuItem("Pin Right", func() { g.SetColumnPinned(colID, PinRight) }),
		)
	} else {
		items = append(items, bellui.NewMenuItem("Unpin", func() { g.SetColumnPinned(colID, PinNone) }))
	}

	// Hide column
	items = append(items, bellui.NewMenuItem("Hide Column", func() {
		g.SetColumnVisible(colID, false)
	}))

	if len(items) == 0 {
		return
	}

	menu := bellui.NewMenu("", items...)
	c := bellui.CurrentApp().Driver().CanvasForObject(g)
	if c == nil {
		return
	}
	widget.ShowPopUpMenuAtPosition(menu, c, e.AbsolutePosition)
}

func (g *DataGrid) FocusGained() {
	g.focused = true
	g.Refresh()
}

func (g *DataGrid) FocusLost() {
	g.focused = false
	g.Refresh()
}

func (g *DataGrid) TypedRune(_ rune) {}

// toggleFocusedGroup expands/collapses the group when the keyboard cursor is on
// a group-header row, routing through the same ToggleGroup path as a pointer tap
// and the accessible expand/collapse action. It returns true when it handled a
// group row, so the Enter/Space key does not also attempt a cell selection.
func (g *DataGrid) toggleFocusedGroup() bool {
	if !g.IsGroupRow(g.currentFocus.Row) {
		return false
	}
	key, _, _, _ := g.GroupRowInfo(g.currentFocus.Row)
	g.ToggleGroup(key)
	return true
}

// toggleFocusedDetail expands/collapses the detail band when the keyboard cursor
// is on a master row, routing through the same ToggleDetail path as a click on
// the disclosure chevron. It returns true when it handled a master row, so the
// Enter/Space key does not also attempt a cell selection.
func (g *DataGrid) toggleFocusedDetail() bool {
	if !g.IsMasterRow(g.currentFocus.Row) {
		return false
	}
	g.ToggleDetail(g.currentFocus.Row)
	return true
}

// activateFocusedRow selects the focused cell and fires OnRowActivated with the
// cursor's source row — the keyboard analog of a primary-click row action. It is
// the shared Enter/Space fall-through reached only after group/detail/tree
// disclosures have had their chance to consume the key, so it always targets a
// data row. A synthetic row with no source identity still selects but does not
// fire activation.
func (g *DataGrid) activateFocusedRow() {
	g.selectCell(g.currentFocus)
	if g.OnRowActivated == nil {
		return
	}
	if src := g.sourceRow(g.currentFocus.Row); src >= 0 {
		g.OnRowActivated(src)
	}
}

// contextMenuFocusedCell is the keyboard path to the cell context menu (Menu
// key / Shift+F10, issue #101): it scrolls the focused cell into view (à la
// AccessibleScrollCellIntoView, so the anchor geometry is on-screen), selects
// it exactly like the pointer path — TappedSecondary selects before firing,
// and still selects when the callback is nil — then fires OnCellContextMenu
// with the cell's display-space id (the same form TappedSecondary delivers)
// anchored at its window-absolute bottom-left. A cleared cursor (its row was
// filtered out) is a no-op, matching the other Enter/Space/F2 focus verbs.
func (g *DataGrid) contextMenuFocusedCell() {
	id := g.currentFocus
	if id.Row < 0 || id.Col < 0 || id.Row >= g.RowCount() || id.Col >= g.HeaderColumnCount() {
		return
	}
	// ScrollTo dispatches the offset update synchronously (render
	// ScrollContainer.ScrollTo), so cellWidgetRect below sees the post-scroll
	// geometry.
	g.scrollToCell(id)
	g.selectCell(id)
	if g.OnCellContextMenu == nil && g.LeadingSwipeActions == nil && g.TrailingSwipeActions == nil {
		return
	}
	pos, size := g.cellWidgetRect(id)
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(g)
	anchor := abs.Add(geo.NewPos(pos.X, pos.Y+size.Height))
	if g.OnCellContextMenu == nil {
		// Without the app's own menu, list the row's swipe actions (#1483).
		g.showRowSwipeMenu(id.Row, anchor)
		return
	}
	g.OnCellContextMenu(id, anchor)
}

func (g *DataGrid) TypedKey(e *event.KeyEvent) {
	if g.disabled {
		return
	}
	rows := g.RowCount()
	// The keyboard cursor lives in absolute display-column space (matching mouse
	// clicks, selection, and columnAtX), so bound and move it by the total
	// visible column count — arrow keys traverse pinned-left → main → pinned-right
	// in display order. scrollToCell maps it back to the main band for h-scroll.
	cols := g.HeaderColumnCount()
	if rows == 0 || cols == 0 {
		return
	}

	// A cleared cursor (its row was filtered out after a re-sort/re-filter) has
	// currentFocus == {-1,-1}: the first movement key re-enters the grid at the
	// origin, while Enter/Space/F2 stay no-ops until the user re-selects.
	focusCleared := g.currentFocus.Row < 0 || g.currentFocus.Col < 0
	if focusCleared {
		switch e.Name {
		case event.KeyTab, event.KeyDown, event.KeyUp, event.KeyLeft, event.KeyRight:
			g.currentFocus = CellID{Row: 0, Col: 0}
			g.focusSrcRow = g.sourceRow(0)
			g.scrollToCell(g.currentFocus)
			g.Refresh()
			return
		}
	}

	// A hardware keyboard on a touch device reports Alt+Up as a key with its
	// modifier; the desktop sends it as a shortcut (TypedShortcut).
	if d := reorderKey(e.Name, e.Modifier); d != 0 && !focusCleared && g.moveFocusedRow(d) {
		return
	}

	switch e.Name {
	case event.KeyEscape:
		if g.cancelLift() {
			return
		}
		if g.IsEditing() {
			g.StopEditing()
		}
		return
	case event.KeyReturn:
		if g.IsEditing() {
			// Commit the active edit. (A focused Entry editor consumes the key
			// itself and commits via OnSubmitted instead.)
			g.CommitEditing()
			return
		}
		if focusCleared {
			return
		}
		if g.toggleFocusedGroup() {
			return
		}
		if g.toggleFocusedDetail() {
			return
		}
		if g.toggleFocusedTreeNode() {
			return
		}
		g.activateFocusedRow()
		return
	case event.KeyTab:
		// Move to next cell (or previous with shift, tracked via lastModifier)
		if g.lastModifier&event.KeyModifierShift != 0 {
			if g.currentFocus.Col > 0 {
				g.currentFocus.Col--
			} else if g.currentFocus.Row > 0 {
				g.currentFocus.Row--
				g.currentFocus.Col = cols - 1
			}
		} else {
			if g.currentFocus.Col < cols-1 {
				g.currentFocus.Col++
			} else if g.currentFocus.Row < rows-1 {
				g.currentFocus.Row++
				g.currentFocus.Col = 0
			}
		}
	case event.KeyDown:
		if g.currentFocus.Row < rows-1 {
			g.currentFocus.Row++
		}
	case event.KeyUp:
		if g.currentFocus.Row > 0 {
			g.currentFocus.Row--
		}
	case g.towardLastColumn():
		if g.currentFocus.Col < cols-1 {
			g.currentFocus.Col++
		}
	case g.towardFirstColumn():
		if g.currentFocus.Col > 0 {
			g.currentFocus.Col--
		}
	case event.KeySpace:
		if focusCleared {
			return
		}
		if g.toggleFocusedGroup() {
			return
		}
		if g.toggleFocusedDetail() {
			return
		}
		if g.toggleFocusedTreeNode() {
			return
		}
		g.activateFocusedRow()
		return
	case event.KeyF2:
		// F2 starts editing the focused cell
		if focusCleared {
			return
		}
		g.StartEditing(g.currentFocus)
		return
	case desktop.KeyMenu:
		g.contextMenuFocusedCell()
		return
	case event.KeyF10:
		// Only the Shift+F10 chord opens the context menu; plain F10 falls
		// out unhandled like any other unrecognized key.
		if e.Modifier&event.KeyModifierShift != 0 {
			g.contextMenuFocusedCell()
		}
		return
	default:
		return
	}

	// A movement key ran: keep the focus source identity current so the cursor
	// can be re-mapped if a later re-sort/re-filter moves this row.
	g.focusSrcRow = g.sourceRow(g.currentFocus.Row)
	g.scrollToCell(g.currentFocus)
	g.Refresh()
}

func (g *DataGrid) Dragged(e *event.DragEvent) {
	if g.disabled {
		return
	}
	if g.mousePressed {
		g.pressDragged = true
	}
	// Column resize drag — use absolute position delta (not incremental DX)
	if g.resizeCol >= 0 {
		rc := g.visibleCol(g.resizeCol)
		if rc == nil {
			return
		}
		newWidth := g.dragStartSize + g.resizeDelta(g.dragStartPos.X, e.Position.X)
		minW := float32(40)
		if rc.def.MinWidth > 0 {
			minW = rc.def.MinWidth
		}
		if newWidth < minW {
			newWidth = minW
		}
		maxW := rc.def.MaxWidth
		if maxW > 0 && newWidth > maxW {
			newWidth = maxW
		}
		rc.width = newWidth
		g.updateViewportGeometry()
		g.Refresh()
		return
	}

	// Column reorder drag — just track position, actual move on DragEnd
	if g.reorderFromCol >= 0 {
		g.lastMousePos = e.Position
		return
	}

	// Row drag reorder: the press's first move lifts the row (#1512).
	if g.dragRow >= 0 {
		if !g.lift.active {
			g.liftRow(g.dragPressY)
		}
		g.lastMousePos = e.Position
		g.moveLift(e.Position.Y)
		return
	}

	// A touch drag: the grid latches it as a Draggable, so the touch
	// handler's scroll path never runs — scroll the body here, like a finger
	// on any other scrollable content. Mouse drags never scroll the body.
	if !g.mousePressed {
		if g.swipeDragged(e) {
			return
		}
		g.scrollContent.Scroll.Scrolled(&event.ScrollEvent{
			PointEvent: e.PointEvent,
			Scrolled:   e.Dragged,
			Source:     event.ScrollSourceTouch,
		})
	}
}

// ClaimsDrag takes a touch drag the grid will use and leaves the rest to the
// scroll around it (event.DragClaimer, #1266). A touch drag only scrolls the
// body (resize and reorder start from a mouse press), so the grid claims a
// drag whose main axis the body can still move along, and leaves one at the
// body's end, or along an axis it does not scroll, to the page. A gesture a
// mouse press began keeps every drag. A disabled grid claims every drag and
// ignores it, so nothing scrolls over it, as over a disabled List (#1251).
//
// With swipe actions (#1483) it also claims a sideways drag the body cannot
// scroll, toward an edge rows have actions on: that drag swipes the row under
// it. While a row is open it claims every drag, which closes the row, or
// moves it when the drag runs sideways along it.
func (g *DataGrid) ClaimsDrag(d geo.Delta) bool {
	if g.disabled || g.resizeCol >= 0 || g.reorderFromCol >= 0 || g.dragRow >= 0 {
		return true
	}
	if g.swipe.src >= 0 || g.swipeClaims(d) {
		return true
	}
	return scrollCanMove(g.scrollContent.Scroll, d)
}

// scrollCanMove reports whether a touch drag starting with d would move s's
// content along the axis d mostly follows, as bellui's Scroll.ClaimsDrag
// decides it. The grid cannot ask its Scroll: that method is built only for
// touch platforms and CI, not on the desktop, where a touchscreen still asks
// the grid.
func scrollCanMove(s *container.Scroll, d geo.Delta) bool {
	if s == nil || s.Content == nil || s.Direction == container.ScrollNone {
		return false
	}
	size, content := s.Size(), s.Content.MinSize()
	if abs32(d.DX) > abs32(d.DY) {
		return s.Direction != container.ScrollVerticalOnly && offsetMoves(s.Offset.X, d.DX, size.Width, content.Width)
	}
	return s.Direction != container.ScrollHorizontalOnly && offsetMoves(s.Offset.Y, d.DY, size.Height, content.Height)
}

// offsetMoves reports whether a finger moving by delta moves a scroll offset
// of a viewport over content of the given lengths.
func offsetMoves(offset, delta, viewport, content float32) bool {
	limit := content - viewport
	if limit <= 0 {
		return false
	}
	next := min(max(offset-delta, 0), limit)
	return next != offset
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

func (g *DataGrid) DragEnd() {
	g.endSwipeGesture()

	// Finalize column reorder
	if g.reorderFromCol >= 0 {
		// Region-aware mapping, shared with the header rendering and the body.
		dropCol := g.columnAtX(g.lastMousePos.X)
		if dropCol >= 0 && dropCol != g.reorderFromCol {
			g.MoveColumn(g.reorderFromCol, dropCol)
		}
		g.reorderFromCol = -1
	}

	// Finalize row drag reorder: the row goes where the line showed.
	if g.dragRow >= 0 {
		g.dropLift()
		g.dragRow = -1
	}

	g.dragCol = -1
	g.resizeCol = -1
	g.nearResizeBorder = false
	g.mousePressed = false
	g.pressDragged = false
}

// Scrolled implements event.Scrollable by forwarding the delta to the grid's
// internal scroll container, clamped exactly like a wheel event over the body.
//
// The grid scrolls through a container.Scroll *inside* its own subtree, which
// the a11y walker's ancestor-only stack never sees — so without this, the
// grid's virtual and realized-off-viewport cell nodes carry no scroll target
// and assistive-tech scroll gestures (VoiceOver three-finger scroll, TalkBack
// two-finger scroll) are inert over the grid (#82). Implementing Scrolled
// gives the grid itself the scroll-handling identity the walker keys on; the
// forwarded delta lands on the same path as a wheel event, so the offset,
// viewport, header, and pinned bands all stay in sync via onScrolled.
func (g *DataGrid) Scrolled(ev *event.ScrollEvent) {
	if g.disabled {
		return
	}
	g.scrollContent.Scroll.Scrolled(ev)
}

func (g *DataGrid) MouseDown(e *desktop.MouseEvent) {
	if g.disabled {
		return
	}
	g.endEditOnPress()
	g.lastModifier = e.Modifier
	g.mousePressed = true
	g.pressDragged = false

	// Check for column resize or reorder drag start. The header shares the body's
	// region layout (pinned bands fixed, main band scrolled by g.offset.X), so
	// header hit-testing uses the same region-aware mapping — headerBorderCol and
	// columnAtX — as the header renderer and HeaderWidget's own handlers.
	headerH := g.HeaderHeight()
	if e.Position.Y < headerH {
		borderCol := g.headerBorderCol(e.Position.X)
		if borderCol >= 0 {
			// Near border — start resize drag
			rc := g.visibleCol(borderCol)
			if rc != nil {
				g.resizeCol = borderCol
				g.dragStartSize = rc.width
				g.dragStartPos = e.Position
				return
			}
		} else {
			// Not near border — start reorder drag
			col := g.columnAtX(e.Position.X)
			if col >= 0 {
				rc := g.visibleCol(col)
				if rc != nil {
					movable := true
					if rc.def.Movable != nil {
						movable = *rc.def.Movable
					}
					if movable {
						g.reorderFromCol = col
						return
					}
				}
			}
		}
	}

	// A press on a disclosure chevron toggles detail/tree rather than selecting.
	if g.detailToggleAt(e.Position) >= 0 {
		return
	}
	if g.treeToggleAt(e.Position) >= 0 {
		return
	}

	// A press on a row's checkbox column selects nothing: the box's own tap
	// toggles the row (ToggleRowSelected), and a touch screen sends that tap
	// with no press (#1504).
	if g.selectionBoxRowAt(e.Position) >= 0 {
		return
	}

	id := g.cellAtPosition(e.Position)
	if id.Row >= 0 && id.Col >= 0 {
		g.selectCellWithModifiers(id, e.Modifier)
		// Start row drag if enabled
		if g.rowsReorderable() {
			g.dragRow = id.Row
			g.dragPressY = e.Position.Y
		}
	}
}

// MouseUp ends the mouse press. A press that never dragged was a click, so its
// release disarms the row drag, column reorder or column resize the press
// armed. Left armed, the next drag finished it, and on a screen with a mouse
// and a touchscreen that drag could be a finger scrolling the grid: it did not
// scroll, and it moved the clicked row or resized the column (#1511). The
// release of a press that did drag comes before its DragEnd, which finishes
// the gesture.
func (g *DataGrid) MouseUp(e *desktop.MouseEvent) {
	g.mousePressed = false
	if !g.pressDragged {
		g.dragRow = -1
		g.reorderFromCol = -1
		g.resizeCol = -1
	}
}

func (g *DataGrid) HoverIn(e *event.HoverEvent) {}

func (g *DataGrid) HoverMoved(e *event.HoverEvent) {
	if g.disabled {
		return
	}
	g.lastMousePos = e.Position

	// Check for header border proximity (for resize cursor). Header is static;
	// use the raw pointer X (see MouseDown).
	headerH := g.HeaderHeight()
	if e.Position.Y < headerH {
		borderCol := g.headerBorderCol(e.Position.X)
		g.nearResizeBorder = borderCol >= 0
	} else {
		g.nearResizeBorder = false
	}

	id := g.cellAtPosition(e.Position)
	if id.Row >= 0 && id.Col >= 0 {
		prevHover := g.hoveredCell
		g.hoveredCell = &id

		// Dismiss tooltip when moving to a different cell
		if prevHover == nil || prevHover.Row != id.Row || prevHover.Col != id.Col {
			g.dismissTooltip()
		}

		// Show tooltip: explicit TooltipGetter, or truncated cell content
		if g.activeTooltip == nil {
			tip := g.CellTooltip(id.Row, id.Col)
			if tip == "" {
				tip = g.truncatedCellTooltip(id.Row, id.Col)
			}
			if tip != "" {
				c := bellui.CurrentApp().Driver().CanvasForObject(g)
				if c != nil {
					g.activeTooltip = ShowTooltip(tip, e.AbsolutePosition, c)
				}
			}
		}
	} else {
		g.hoveredCell = nil
		g.dismissTooltip()
	}
	g.Refresh()
}

func (g *DataGrid) HoverOut() {
	g.hoveredCell = nil
	g.dismissTooltip()
	g.Refresh()
}

func (g *DataGrid) dismissTooltip() {
	if g.activeTooltip != nil {
		g.activeTooltip.Hide()
		g.activeTooltip = nil
	}
}

func (g *DataGrid) Cursor() desktop.Cursor {
	if g.disabled {
		return desktop.DefaultCursor
	}
	if g.nearResizeBorder || g.resizeCol >= 0 {
		return desktop.HResizeCursor
	}
	return desktop.DefaultCursor
}

// headerBorderCol returns the column index whose right border is near the given X position,
// or -1 if not near any border. Threshold is 4 pixels.
// headerBorderCol returns the absolute display column whose right-edge divider
// is within the resize threshold of widget-X x, or -1. It walks the same region
// bands the header is drawn in (pinned-left fixed, main scrolled by offset.X,
// pinned-right overlay) so the resize grab lines up with the visible divider.
// x is a pointer's, as drawn; right to left it is mapped back first (#779).
func (g *DataGrid) headerBorderCol(x float32) int {
	x = g.ltrX(x)
	layout := g.visibleLayout()
	pad := g.cellPadding()
	threshold := float32(4)
	mainX, pinnedRightX, pinnedRightW := g.hitRegions()

	borderIn := func(cols []*resolvedColumn, startX float32, base int) int {
		cx := startX
		for i, c := range cols {
			cx += c.width + pad
			rightEdge := cx - pad // right edge of column (before separator)
			if x >= rightEdge-threshold && x <= rightEdge+threshold {
				if c.def.isResizable(g.DefaultColDef) {
					return base + i
				}
				return -1
			}
		}
		return -1
	}

	plCount := len(layout.pinnedLeft)
	mainCount := len(layout.main)
	switch {
	case plCount > 0 && x < mainX:
		return borderIn(layout.pinnedLeft, 0, 0)
	case len(layout.pinnedRight) > 0 && pinnedRightW > 0 && g.Size().Width > 0 && x >= pinnedRightX:
		return borderIn(layout.pinnedRight, pinnedRightX, plCount+mainCount)
	default:
		return borderIn(layout.main, mainX-g.offset.X+g.selectionColumnWidth(), plCount)
	}
}

func (g *DataGrid) TypedShortcut(s core.Shortcut) {
	if g.disabled {
		return
	}
	if d := reorderShortcut(s); d != 0 {
		g.moveFocusedRow(d)
		return
	}
	switch s.(type) {
	case *bellui.ShortcutCopy:
		g.copySelection()
	case *bellui.ShortcutPaste:
		g.pasteToSelection()
	case *bellui.ShortcutSelectAll:
		g.SelectAll()
	case *bellui.ShortcutUndo:
		g.Undo()
	case *bellui.ShortcutRedo:
		g.Redo()
	}
}

// --- Selection helpers ---

func (g *DataGrid) selectCell(id CellID) {
	g.selectCellWithModifiers(id, 0)
}

// sourceRow translates a display row index to its stable source row index, or
// -1 if the display row is out of range. Selection is keyed by source row so it
// survives re-sort/re-filter/paginate (which only permute the display->source
// mapping in pipeline.RowIndex, never the source indices themselves).
func (g *DataGrid) sourceRow(displayRow int) int {
	return g.pipe.Index().SourceRow(displayRow)
}

// SourceRow maps a display-space row index — the CellID.Row delivered by
// OnCellClicked/OnCellSecondaryTapped — to its stable source-row index into the
// underlying data, the same indexing OnSelectionChanged and RowData use. It
// returns -1 for a synthetic display row (e.g. a group header) or one out of
// range. Use it to recover the clicked row after sort/filter/paginate, which
// permute the display<->source mapping but never the source indices themselves;
// RowData(id.Row) would read the wrong row once the grid is sorted or filtered.
func (g *DataGrid) SourceRow(displayRow int) int {
	return g.sourceRow(displayRow)
}

// DisplayRow maps a stable source-row index — the indexing OnSelectionChanged,
// OnRowActivated and RowData use — to the display row currently showing it, or
// -1 if that source row is not in the active view (filtered out, or on another
// page). It is the inverse of SourceRow: use it to select or reveal a row
// tracked by source index after sort/filter/paginate permute the view, since
// SelectCell and ScrollToRow take display rows.
func (g *DataGrid) DisplayRow(sourceRow int) int {
	return g.displayRowForSource(sourceRow)
}

// DisplayRowData returns the underlying row data shown at the given display-space
// row, or nil for a synthetic row (group header) or one out of range (or when no
// RowData accessor is set). It resolves the display row to its source row and
// calls RowData, so it stays correct across sort/filter/paginate where the
// source-indexed RowData(displayRow) would not.
func (g *DataGrid) DisplayRowData(displayRow int) any {
	src := g.sourceRow(displayRow)
	if src < 0 || g.RowData == nil {
		return nil
	}
	return g.RowData(src)
}

// displayRowForSource returns the display row currently showing source row src,
// or -1 if src is not in the active view (filtered out / on another page). It
// delegates to the pipeline index's lazily-built reverse map (O(1) after the
// first lookup on a given index), so a re-sort with an active cursor no longer
// costs an O(n) scan per remap.
func (g *DataGrid) displayRowForSource(src int) int {
	return g.pipe.Index().DisplayRow(src)
}

// remapCursorAfterReindex re-resolves the live interaction cursor to the display
// row now showing its source row, after a pipeline recompute (sort/filter/
// paginate) permutes the display<->source mapping. The selection highlight is
// already source-keyed (selCellRow, resolved at paint time) and needs no remap;
// only the display-space cursors — selectedCell (edit/paste/scroll/API target)
// and currentFocus (keyboard) — drift. Costs nothing when no cursor is set.
//
// Paginated grids: followCursorAcrossPages runs first (pre-rebrand issue #9, Option A), so a
// cursor whose row moved to another page has already been navigated onto that
// page by the time we get here and resolves normally. A row that is genuinely
// gone (filtered out of every page) still resolves to -1 and clears below.
func (g *DataGrid) remapCursorAfterReindex() {
	if g.selectedCell == nil && g.focusSrcRow < 0 {
		return // no live cursor: nothing to remap, no reverse scan
	}

	// Selected cell (edit / paste / scroll-to / API target).
	if g.selectedCell != nil {
		if d := g.displayRowForSource(g.selCellRow); d >= 0 {
			g.selectedCell.Row, g.selectedCell.Col = d, g.selCellCol
		} else {
			// Its source row is no longer in view: clear the cursor so Enter
			// and paste never act on the wrong row. The highlight clears on
			// its own (selCellRow no longer resolves to a visible display row).
			g.selectedCell = nil
			g.selCellRow, g.selCellCol = -1, 0
		}
	}

	// Keyboard focus cursor.
	if g.focusSrcRow >= 0 {
		if d := g.displayRowForSource(g.focusSrcRow); d >= 0 {
			g.currentFocus.Row = d
		} else {
			// Focused row filtered out: clear focus. A movement key re-enters
			// the grid at the origin; Enter/Space/F2 are no-ops until then.
			g.currentFocus = CellID{Row: -1, Col: -1}
			g.focusSrcRow = -1
		}
	}

	// Reset the shift-range anchor after a reindex (AG Grid behavior): the
	// visual range origin is no longer meaningful once rows have moved.
	if g.currentFocus.Row >= 0 {
		g.anchorRow, g.anchorCol = g.currentFocus.Row, g.currentFocus.Col
	} else {
		g.anchorRow, g.anchorCol = 0, 0
	}
}

func (g *DataGrid) selectCellWithModifiers(id CellID, mod event.KeyModifier) {
	g.currentFocus = id
	src := g.sourceRow(id.Row)
	g.focusSrcRow = src

	setSelectedCell := func() {
		g.selectedCell = &id
		g.selCellRow, g.selCellCol = src, id.Col
	}
	selectOnly := func(s int) {
		g.selectedRows = make(map[int]bool)
		if s >= 0 {
			g.selectedRows[s] = true
		}
	}

	switch g.SelectionMode {
	case SelectionNone:
		return

	case SelectionSingleCell:
		setSelectedCell()
		g.selectedRows = make(map[int]bool)

	case SelectionSingleRow:
		setSelectedCell()
		selectOnly(src)
		g.anchorRow = id.Row

	case SelectionMultiRow:
		shift := mod&event.KeyModifierShift != 0
		ctrl := mod&event.KeyModifierControl != 0 || mod&event.KeyModifierSuper != 0

		if shift {
			// Range select from anchor to current, keyed by source identity.
			g.selectedRows = make(map[int]bool)
			lo, hi := g.anchorRow, id.Row
			if lo > hi {
				lo, hi = hi, lo
			}
			for r := lo; r <= hi; r++ {
				if s := g.sourceRow(r); s >= 0 {
					g.selectedRows[s] = true
				}
			}
		} else if ctrl {
			// Toggle this row
			if src >= 0 {
				if g.selectedRows[src] {
					delete(g.selectedRows, src)
				} else {
					g.selectedRows[src] = true
				}
			}
			g.anchorRow = id.Row
		} else {
			// Single select, reset
			selectOnly(src)
			g.anchorRow = id.Row
		}
		setSelectedCell()

	case SelectionMultiCell:
		g.selectMultiCell(id, src, mod)
		setSelectedCell()
		g.selectedRows = make(map[int]bool) // cells are selected, not their rows
	}

	g.fireSelectionChanged()
	g.Refresh()
}

func (g *DataGrid) fireSelectionChanged() {
	if g.OnSelectionChanged == nil {
		return
	}
	g.OnSelectionChanged(g.selectedSourceRows())
}

// selectedSourceRows is the selection as source rows: the selected rows, or
// in SelectionMultiCell the rows that hold a selected cell, each once.
func (g *DataGrid) selectedSourceRows() []int {
	if g.SelectionMode == SelectionMultiCell {
		seen := map[int]bool{}
		rows := []int{}
		for c := range g.selectedCells {
			if !seen[c.src] {
				seen[c.src] = true
				rows = append(rows, c.src)
			}
		}
		sort.Ints(rows)
		return rows
	}
	rows := make([]int, 0, len(g.selectedRows))
	for r := range g.selectedRows {
		rows = append(rows, r)
	}
	return rows
}

// SelectAll selects all currently-displayed rows (for multi-row mode), keyed by
// source identity so the selection survives a later re-sort/re-filter.
func (g *DataGrid) SelectAll() {
	if g.SelectionMode != SelectionMultiRow {
		return
	}
	n := g.RowCount()
	g.selectedRows = make(map[int]bool, n)
	for i := 0; i < n; i++ {
		if s := g.sourceRow(i); s >= 0 {
			g.selectedRows[s] = true
		}
	}
	g.fireSelectionChanged()
	g.Refresh()
}

// selCell is a cell of a SelectionMultiCell selection: its source row and
// absolute display column.
type selCell struct{ src, col int }

// selectMultiCell applies a click on id, at source row src, to the
// SelectionMultiCell selection (#1401): a plain click selects that cell alone,
// Ctrl/Cmd toggles it and keeps the others, Shift selects the rectangle from
// the anchor cell to it. The anchor moves on a plain or Ctrl click.
func (g *DataGrid) selectMultiCell(id CellID, src int, mod event.KeyModifier) {
	shift := mod&event.KeyModifierShift != 0
	ctrl := mod&event.KeyModifierControl != 0 || mod&event.KeyModifierSuper != 0
	switch {
	case shift:
		g.selectedCells = make(map[selCell]bool)
		r0, r1 := min(g.anchorRow, id.Row), max(g.anchorRow, id.Row)
		c0, c1 := min(g.anchorCol, id.Col), max(g.anchorCol, id.Col)
		for r := r0; r <= r1; r++ {
			s := g.sourceRow(r)
			if s < 0 {
				continue
			}
			for c := c0; c <= c1; c++ {
				g.selectedCells[selCell{s, c}] = true
			}
		}
	case ctrl:
		if g.selectedCells == nil {
			g.selectedCells = make(map[selCell]bool)
		}
		if k := (selCell{src, id.Col}); g.selectedCells[k] {
			delete(g.selectedCells, k)
		} else if src >= 0 {
			g.selectedCells[k] = true
		}
		g.anchorRow, g.anchorCol = id.Row, id.Col
	default:
		g.selectedCells = make(map[selCell]bool)
		if src >= 0 {
			g.selectedCells[selCell{src, id.Col}] = true
		}
		g.anchorRow, g.anchorCol = id.Row, id.Col
	}
}

// isCellSelected reports whether the given DISPLAY cell is selected. It resolves
// the display row to its source identity before checking the (source-keyed)
// selection state, so highlights track the underlying data across re-sort.
// col is the absolute display column, pinned-left columns first.
func (g *DataGrid) isCellSelected(row, col int) bool {
	src := g.sourceRow(row)
	if src < 0 {
		return false
	}
	switch g.SelectionMode {
	case SelectionMultiCell:
		// The selected cells only: the cursor cell is not one after a
		// Ctrl/Cmd click took it out.
		return g.selectedCells[selCell{src, col}]
	case SelectionSingleRow, SelectionMultiRow:
		// A cell is selected exactly when its row is. The cursor cell is no
		// selection here: counting it kept a row drawn selected, and its box
		// checked, after a click deselected it (#1524).
		return g.selectedRows[src]
	}
	if g.selectedRows[src] {
		return true
	}
	if g.selCellRow == src && g.selCellCol == col {
		return true
	}
	return false
}

func (g *DataGrid) isCellHovered(row, col int) bool {
	if g.hoveredCell == nil {
		return false
	}
	return g.hoveredCell.Row == row && g.hoveredCell.Col == col
}

func (g *DataGrid) cellAtPosition(pos geo.Position) CellID {
	headerH := g.HeaderHeight()
	if pos.Y < headerH {
		// In header area: return Row=-1 with the correct column. The header shares
		// the body's region layout, so use the same region-aware mapping the body
		// uses (pinned bands fixed, main band scrolled by offset.X).
		col := g.columnAtX(pos.X)
		return CellID{Row: -1, Col: col}
	}

	row := g.bodyRowAt(pos)

	// Find column, honouring the fixed pinned bands vs the scrolling main band.
	// columnAtX returns an ABSOLUTE display index (pinned-left, main, pinned-
	// right), so bound it by the total visible column count — not the main-only
	// ColumnCount(), which would reject pinned-right and the tail main columns.
	col := g.columnAtX(pos.X)

	if row < 0 || row >= g.RowCount() || col < 0 || col >= g.HeaderColumnCount() {
		return CellID{Row: -1, Col: -1}
	}
	return CellID{Row: row, Col: col}
}

// bodyRowAt returns the display row whose cell band is under the widget
// position pos, or -1. The floating-filter row, between the header and the
// body, has no rows, and a tap inside an expanded detail band is in no cell:
// the band hosts its own content, which handles its own events.
func (g *DataGrid) bodyRowAt(pos geo.Position) int {
	top := g.bodyTop()
	if pos.Y < top {
		return -1
	}
	// Adjust for the rows above the body and the vertical scroll offset.
	adjustedY := pos.Y - top + g.offset.Y
	// Uniform O(1) or variable O(log n) via the viewport prefix sum.
	row := g.viewport.RowAtY(adjustedY)
	if row >= 0 {
		ry, rh := g.viewport.RowY(row)
		if band := g.DetailBandHeight(row); band > 0 && adjustedY >= ry+(rh-band) {
			return -1
		}
	}
	return row
}

// selectionBoxRowAt returns the display row whose checkbox-selection cell is
// under the widget position pos, or -1. That column is the start of the
// scrolling main band and holds no layout column, so columnAtX reports none
// there (#1498). pos is a pointer's, as drawn; right to left its x is mapped
// back first (#779).
func (g *DataGrid) selectionBoxRowAt(pos geo.Position) int {
	lead := g.selectionColumnWidth()
	if lead <= 0 {
		return -1
	}
	pos.X = g.ltrX(pos.X)
	mainX, pinnedRightX, pinnedRightW := g.hitRegions()
	if pos.X < mainX || (pinnedRightW > 0 && pos.X >= pinnedRightX) {
		return -1
	}
	if cx := pos.X - mainX + g.offset.X; cx < 0 || cx >= lead {
		return -1
	}
	row := g.bodyRowAt(pos)
	if row < 0 || row >= g.RowCount() || g.IsGroupRow(row) {
		return -1
	}
	return row
}

// disclosureHitX maps a widget X into the coordinate frame of the first-visible
// column's disclosure control (master-detail chevron or tree chevron), returning
// the tap's X and the zone's left edge. The disclosure sits on the first
// pinned-left column when any are pinned — fixed, so no scroll offset and no
// leading checkbox gutter — otherwise on the first main column, which scrolls
// with the body and sits past the checkbox gutter. depthIndent is the per-row
// indent before the chevron slot (0 for master-detail, depth-based for trees).
// posX is a pointer's, as drawn; right to left it is mapped back first (#779).
func (g *DataGrid) disclosureHitX(posX, depthIndent float32) (tapX, zoneLeft float32) {
	posX = g.ltrX(posX)
	if len(g.visibleLayout().pinnedLeft) > 0 {
		return posX, depthIndent
	}
	zoneLeft = depthIndent + g.selectionColumnWidth()
	return posX + g.offset.X, zoneLeft
}

// detailToggleAt returns the display row whose disclosure chevron is under the
// given widget position, or -1 if none. The chevron occupies the reserved left
// indent of the first visible column (the first pinned-left column when any are
// pinned, else the first main column), within the row's top cell band, on master
// rows only.
func (g *DataGrid) detailToggleAt(pos geo.Position) int {
	toggleW := g.DetailToggleWidth()
	if toggleW <= 0 {
		return -1
	}
	top := g.bodyTop()
	if pos.Y < top {
		return -1
	}
	adjustedY := pos.Y - top + g.offset.Y

	row := g.viewport.RowAtY(adjustedY)
	if row < 0 || row >= g.RowCount() || !g.IsMasterRow(row) {
		return -1
	}

	// Must be in the cell band (above any detail band).
	ry, rh := g.viewport.RowY(row)
	if band := g.DetailBandHeight(row); band > 0 && adjustedY >= ry+(rh-band) {
		return -1
	}

	tapX, zoneLeft := g.disclosureHitX(pos.X, 0)
	if tapX >= zoneLeft && tapX < zoneLeft+toggleW {
		return row
	}
	return -1
}

// treeToggleAt returns the display row whose tree disclosure chevron is under the
// given widget position, or -1 if none. The chevron occupies a fixed-width slot
// after the row's depth indent, at the left of the first VISIBLE column (the
// first pinned-left column when any are pinned, else the first main column), on
// tree parent rows (HasChildren) only.
// groupRowAt returns the display row of the group-header row under pos, or -1.
// The whole header row is the toggle target, so a tap anywhere on it —
// including past the last column — expands or collapses the group (#1252).
func (g *DataGrid) groupRowAt(pos geo.Position) int {
	if g.groupTransform == nil {
		return -1
	}
	top := g.bodyTop()
	if pos.Y < top {
		return -1
	}
	row := g.viewport.RowAtY(pos.Y - top + g.offset.Y)
	if row < 0 || row >= g.RowCount() || !g.IsGroupRow(row) {
		return -1
	}
	return row
}

func (g *DataGrid) treeToggleAt(pos geo.Position) int {
	if !g.treeActive() {
		return -1
	}
	top := g.bodyTop()
	if pos.Y < top {
		return -1
	}
	adjustedY := pos.Y - top + g.offset.Y

	row := g.viewport.RowAtY(adjustedY)
	if row < 0 || row >= g.RowCount() {
		return -1
	}
	depth, hasChildren, _ := g.TreeRowInfo(row)
	if !hasChildren {
		return -1
	}

	tapX, zoneLeft := g.disclosureHitX(pos.X, float32(depth)*render.TreeIndentPerLevel)
	if tapX >= zoneLeft && tapX < zoneLeft+render.TreeToggleWidth {
		return row
	}
	return -1
}

func (g *DataGrid) scrollToCell(id CellID) {
	if g.scrollContent == nil {
		return
	}

	y, h := g.viewport.RowY(id.Row)
	// The scroll is the body: its height is already below the header.
	scrollSize := g.scrollContent.Scroll.Size()

	newOffset := g.offset

	// Vertical scroll (rows are shared across all column bands).
	if y < g.offset.Y {
		newOffset.Y = y
	} else if y+h > g.offset.Y+scrollSize.Height {
		newOffset.Y = y + h - scrollSize.Height
	}

	// Horizontal scroll only for main columns — the pinned bands are always
	// visible. id.Col is an absolute display index, so map it into the main-only
	// viewport (subtract the pinned-left count) and skip when it lands in a pinned
	// band (mainCol out of [0, main count)).
	if mainCol := id.Col - g.PinnedLeftCount(); mainCol >= 0 && mainCol < g.ColumnCount() {
		x, w := g.viewport.ColX(mainCol)
		if mainCol == 0 {
			// The first main column brings the checkbox-selection column
			// before it into view too (#1498).
			x, w = 0, x+w
		}
		if x < g.offset.X {
			newOffset.X = x
		} else if x+w > g.offset.X+scrollSize.Width {
			newOffset.X = x + w - scrollSize.Width
		}
	}

	if newOffset != g.offset {
		g.scrollContent.ScrollTo(geo.NewPos(g.flipScrollX(newOffset.X), newOffset.Y))
	}
}

// --- StatusBarAccess interface implementation ---

func (g *DataGrid) TotalRowCount() int {
	return g.Length()
}

func (g *DataGrid) SelectedRowCount() int {
	return len(g.selectedRows)
}

func (g *DataGrid) CurrentPage() int {
	if g.paginateTransform == nil {
		return 0
	}
	return g.paginateTransform.CurrentPage
}

func (g *DataGrid) PageCount() int {
	if g.paginateTransform == nil {
		return 1
	}
	// We need the total rows BEFORE pagination. Compute by running
	// the pipeline without the paginate step.
	return g.paginateTransform.PageCount(g.prePaginatedRowCount())
}

// prePaginateIndices returns the display-order source indices after every
// transform except pagination — i.e. the full ordering the pages are sliced
// from. O(n) plus the transforms' own cost; used for page-count and page-of-row
// queries, not on the render hot path.
func (g *DataGrid) prePaginateIndices() []int {
	src := g.pipe.Source()
	n := src.Length()
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for _, t := range g.buildTransforms(false) {
		indices = t.Apply(indices, src)
	}
	return indices
}

// prePaginatedRowCount returns the row count after all transforms except pagination.
func (g *DataGrid) prePaginatedRowCount() int {
	return len(g.prePaginateIndices())
}

// pageForSource returns the 0-indexed page that source row src lands on in the
// current pre-paginate ordering, or -1 if src is filtered out (not present) or
// pagination is inactive.
func (g *DataGrid) pageForSource(src int) int {
	if src < 0 || g.paginateTransform == nil || g.paginateTransform.PageSize <= 0 {
		return -1
	}
	for pos, s := range g.prePaginateIndices() {
		if s == src {
			return pos / g.paginateTransform.PageSize
		}
	}
	return -1
}

// activeCursorSource returns the source row of the live interaction cursor —
// keyboard focus preferred, else the selected cell — or -1 when no cursor is set.
func (g *DataGrid) activeCursorSource() int {
	if g.focusSrcRow >= 0 {
		return g.focusSrcRow
	}
	if g.selectedCell != nil && g.selCellRow >= 0 {
		return g.selCellRow
	}
	return -1
}

// followCursorAcrossPages keeps the interaction cursor with its source row when a
// re-sort/re-filter pushes that row onto a different page (pre-rebrand issue #9, Option A:
// follow). Without it the row falls outside the current page's index and the
// cursor clears even though the row still exists. It navigates to the row's new
// page (updating paginateTransform.CurrentPage) and marks the pipeline dirty so
// the subsequent remap resolves on that page. No-op for non-paginated grids,
// for cursors already on the current page, and for genuinely filtered-out rows
// (those are left for remapCursorAfterReindex to clear).
func (g *DataGrid) followCursorAcrossPages() {
	if !g.IsPaginated() || g.paginateTransform.PageSize <= 0 {
		return
	}
	src := g.activeCursorSource()
	if src < 0 {
		return
	}
	if g.pipe.Index().DisplayRow(src) >= 0 {
		return // already visible on the current page
	}
	page := g.pageForSource(src)
	if page < 0 || page == g.paginateTransform.CurrentPage {
		return // filtered out entirely, or already the right page
	}
	g.paginateTransform.SetPage(page)
	g.pipe.MarkDirty() // remap's Index() call recomputes on the new page
}

func (g *DataGrid) IsPaginated() bool {
	return g.Options.Pagination && g.paginateTransform != nil
}

// --- FloatingFilterAccess ---

func (g *DataGrid) OnFloatingFilterChanged(col int, text string) {
	rc := g.visibleCol(col)
	if rc == nil {
		return
	}
	colID := rc.def.ID()
	if text == "" {
		g.setColumnFilter(colID, nil)
	} else {
		lower := toLower(text)
		g.setColumnFilter(colID, func(v any) bool {
			return containsLower(fmt.Sprint(v), lower)
		})
	}
}

// --- Loading Overlay ---

// ShowLoading displays a loading overlay with the given message.
func (g *DataGrid) ShowLoading(message string) {
	if g.loadingOverlay == nil {
		g.loadingOverlay = render.NewLoadingOverlay()
	}
	g.loadingOverlay.SetMessage(message)
	g.loadingOverlay.Show()
	g.Refresh()
}

// HideLoading hides the loading overlay.
func (g *DataGrid) HideLoading() {
	if g.loadingOverlay != nil {
		g.loadingOverlay.Hide()
		g.Refresh()
	}
}

// --- Tooltip ---

// CellTooltip returns the tooltip text for a cell, or "" if none.
func (g *DataGrid) CellTooltip(row, col int) string {
	rc := g.visibleCol(col)
	if rc == nil || rc.def.TooltipGetter == nil {
		return ""
	}
	rowData := g.pipe.DisplayRow(row)
	if rowData == nil {
		return ""
	}
	return rc.def.TooltipGetter(rowData)
}

// truncatedCellTooltip returns the full cell text if it's truncated (wider
// than the column), or "" if it fits. Used to show a tooltip on hover.
func (g *DataGrid) truncatedCellTooltip(row, col int) string {
	// Determine the column — check both main and pinned
	var rc *resolvedColumn
	layout := g.visibleLayout()
	if col >= 0 && col < len(layout.all) {
		rc = layout.all[col]
	}
	if rc == nil {
		return ""
	}

	rowData := g.pipe.DisplayRow(row)
	if rowData == nil {
		return ""
	}

	var value any
	if rc.def.ValueGetter != nil {
		value = rc.def.ValueGetter(rowData)
	} else {
		value = defaultFieldAccess(rowData, rc.def.Field)
	}
	if rc.def.ValueFormatter != nil {
		value = rc.def.ValueFormatter(value)
	}

	text := fmt.Sprint(value)
	if text == "" {
		return ""
	}

	// Measure text width vs column width
	textSize := g.Theme().Size(bellui.ThemeSizeName("text"))
	measured := bellui.MeasureText(text, textSize, bellui.TextStyle{})
	if measured.Width <= rc.width {
		return "" // fits, no tooltip needed
	}
	return text
}

// --- Row Alternation ---

// IsAlternateRow returns true if the given display row should use alternate coloring.
func (g *DataGrid) IsAlternateRow(row int) bool {
	return g.Options.RowAlternation && row%2 == 1
}

// bandAnchors computes the body's column-band geometry for a widget of the given
// width: the main band's left edge mainX, the pinned-right overlay's left edge
// pinnedRightX (clamped so it never overhangs the right edge), and the total
// pinned-right width. It is the single source of these anchors, shared by the
// draw path (dataGridRenderer.doLayout) and the hit-test/region path (hitRegions
// -> columnAtX, and HeaderRegions for the header/floating-filter rows).
//
// When the main scroll shows its vertical bar, pinned-right also keeps clear of
// the bar's gutter at the right edge (vScrollGutter), so the bar is not drawn
// under it (#1257).
//
// The size is passed explicitly rather than read from g.Size() because during a
// layout pass doLayout must use its incoming size, which may not yet equal
// g.Size(); hitRegions passes g.Size().
func (g *DataGrid) bandAnchors(size geo.Size) (mainX, pinnedRightX, pinnedRightW float32) {
	width := size.Width
	layout := g.visibleLayout()
	pad := g.cellPadding()
	pinnedLeftW := layout.pinnedLeftWidth(pad)
	pinnedRightW = layout.pinnedRightWidth(pad)
	mainX = pinnedLeftW
	if pinnedLeftW > 0 {
		mainX += pad
	}
	// The main band holds the checkbox-selection column, then the main columns.
	mainColsW := g.selectionColumnWidth()
	for _, col := range layout.main {
		mainColsW += col.width + pad
	}
	pinnedRightX = mainX + mainColsW
	if maxX := width - pinnedRightW - g.vScrollGutter(size); pinnedRightW > 0 && pinnedRightX > maxX {
		pinnedRightX = maxX
	}
	return
}

// vScrollGutter is the width pinned-right keeps clear at the grid's right edge
// for the main scroll's vertical bar (#1257): the bar width when there is a
// pinned-right band and the body overflows vertically, else 0. The scroll's
// direction is not consulted: Disable turns scrolling off, and the gutter
// staying put keeps the columns from jumping sideways.
func (g *DataGrid) vScrollGutter(size geo.Size) float32 {
	if len(g.visibleLayout().pinnedRight) == 0 {
		return 0
	}
	if g.viewport.ContentSize().Height <= g.bodyHeight(size.Height) {
		return 0
	}
	return g.Theme().Size(bellui.ThemeSizeName("scrollBar"))
}

// bodyTop is where the body (the main scroll and the bands beside it) starts:
// below the header and the floating-filter row, as dataGridRenderer.doLayout
// places it. Hit-testing and cell rectangles measure rows from here; measuring
// from the header alone put every tap on a grid with floating filters on the
// row below.
func (g *DataGrid) bodyTop() float32 {
	top := g.HeaderHeight()
	if g.floatingFilterRow != nil {
		top += floatingFilterHeight
	}
	return top
}

// bodyHeight is the height of the body (the main scroll and the bands beside
// it) in a grid of height h: below the header and floating-filter row, above
// the status bar.
func (g *DataGrid) bodyHeight(h float32) float32 {
	h -= g.HeaderHeight()
	if g.floatingFilterRow != nil {
		h -= floatingFilterHeight
	}
	if g.Options.ShowStatusBar {
		h -= statusBarHeight
	}
	return h
}

// hitRegions returns the widget-X geometry of the three cell-body column bands,
// mirroring dataGridRenderer.doLayout so pointer hit-testing inverts the same
// positions. Pinned-left is fixed at [0, mainX); the main band is drawn at
// [mainX, pinnedRightX) and scrolls by offset.X; pinned-right is a fixed overlay
// at [pinnedRightX, pinnedRightX+pinnedRightW).
func (g *DataGrid) hitRegions() (mainX, pinnedRightX, pinnedRightW float32) {
	return g.bandAnchors(g.Size())
}

// colIndexIn returns base+i for the column in cols whose fixed-origin span
// contains localX, or -1 if none. localX is measured from the band's left edge.
func colIndexIn(cols []*resolvedColumn, localX, pad float32, base int) int {
	if localX < 0 {
		return -1
	}
	cumX := float32(0)
	for i, c := range cols {
		w := c.width + pad
		if localX < cumX+w {
			return base + i
		}
		cumX += w
	}
	return -1
}

// columnAtX maps a widget-relative X to an absolute display column index (into
// layout.all), routing through the correct band: pinned-left and pinned-right are
// drawn at fixed positions (no horizontal scroll), while the main band scrolls by
// offset.X. This replaces the naive "pos.X + offset.X over layout.all", which
// wrongly added the main scroll offset to taps on the fixed pinned columns.
// x is a pointer's, as drawn; right to left it is mapped back first (#779).
func (g *DataGrid) columnAtX(x float32) int {
	x = g.ltrX(x)
	layout := g.visibleLayout()
	pad := g.cellPadding()
	mainX, pinnedRightX, pinnedRightW := g.hitRegions()

	// Pinned-left: fixed band at [0, mainX).
	if len(layout.pinnedLeft) > 0 && x < mainX {
		return colIndexIn(layout.pinnedLeft, x, pad, 0)
	}
	// Pinned-right: fixed overlay at the right (only trust the region when the
	// widget has a real width, so a pre-layout zero size can't swallow every tap).
	if len(layout.pinnedRight) > 0 && pinnedRightW > 0 && g.Size().Width > 0 && x >= pinnedRightX {
		base := len(layout.pinnedLeft) + len(layout.main)
		return colIndexIn(layout.pinnedRight, x-pinnedRightX, pad, base)
	}
	// Main band: scrolls; content-x = (x - mainX) + offset.X, and the main
	// columns start past the checkbox-selection column, which is no layout
	// column (#1498).
	return colIndexIn(layout.main, x-mainX+g.offset.X-g.selectionColumnWidth(), pad, len(layout.pinnedLeft))
}

// defaultValueGetter extracts a value from row data using the field name.
func defaultValueGetter(rowData any, field string) any {
	return defaultFieldAccess(rowData, field)
}
