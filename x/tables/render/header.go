package render

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

var (
	_ desktop.Mouseable  = (*HeaderWidget)(nil)
	_ event.Hoverable    = (*HeaderWidget)(nil)
	_ desktop.Cursorable = (*HeaderWidget)(nil)
	_ event.Draggable    = (*HeaderWidget)(nil)
	_ event.DragClaimer  = (*HeaderWidget)(nil)
)

// HeaderAccess is the interface the HeaderWidget needs from the DataGrid.
// The header renders ALL visible columns (pinned + main + right), so it uses the
// RegionLayoutAccess column set/anchors (HeaderColumnCount/HeaderColumnWidths,
// HeaderRegions, …) rather than the main-only ColumnCount/ColumnWidths used by
// the CellsRenderer viewport — the same geometry the floating-filter row uses so
// its labels stay above their data under horizontal scroll and with pinned bands.
type HeaderAccess interface {
	RegionLayoutAccess

	ColumnHeader(col int) string
	HeaderHeight() float32
	SortState(col int) SortIndicator
	// ColumnSortable reports whether a header tap on the column at the given
	// absolute display index would sort it. It gates the header cell's
	// accessible "press" action (headerCellA11y).
	ColumnSortable(col int) bool
	IsFilterActive(col int) bool
	OnHeaderTapped(col int, modifier event.KeyModifier)
	OnResizeDragStart(col int, startX float32)
	OnResizeDrag(posX float32)
	OnResizeDragEnd()
	HeaderBorderCol(x float32) int
	// ColumnAtX inverts the region geometry: a widget-X to an absolute display
	// column index (the same mapping the body uses), or -1 for no column.
	ColumnAtX(x float32) int
}

// HeaderTextHider is an optional capability of a HeaderAccess: a column it
// reports hidden gets a blank header label, while ColumnHeader still names the
// column for its accessible node. A grid without it paints every title.
type HeaderTextHider interface {
	ColumnHeaderTextHidden(col int) bool
}

// SelectAllAccess is an optional capability of a HeaderAccess: a grid that
// offers to select all its rows from the checkbox-selection column's header
// cell. The header then holds a select-all box there (#1498). A grid without
// it leaves that cell empty.
type SelectAllAccess interface {
	// SelectAllState reports whether the box shows, and whether every row or
	// only some are selected.
	SelectAllState() (shown, all, some bool)
	// SetAllRowsSelected selects every row, or clears them all.
	SetAllRowsSelected(selected bool)
}

// selectAllCheck is the header's select-all box: a check named for assistive
// tech, which would otherwise read a nameless checkbox in the header.
type selectAllCheck struct {
	widget.Check
}

func newSelectAllCheck() *selectAllCheck {
	c := &selectAllCheck{}
	c.ExtendBaseWidget(c)
	return c
}

// AccessibleLabel names the box.
func (c *selectAllCheck) AccessibleLabel() string { return "Select all rows" }

// SortIndicator represents the sort state of a column header.
type SortIndicator int

const (
	SortIndicatorNone SortIndicator = iota
	SortIndicatorAsc
	SortIndicatorDesc
)

// HeaderWidget is the non-scrolling (vertically) header row. Its columns share
// the body's region layout, so the main band scrolls horizontally in sync with
// the content while the pinned-left/right bands stay fixed (see headerRenderer).
type HeaderWidget struct {
	widget.BaseWidget

	Grid     HeaderAccess
	viewport *Viewport

	nearBorder bool // true when hovering near a column border
	resizing   bool // true during a resize drag
	// pressedMod is the modifiers of the mouse press the next tap ends, and
	// pressed whether there is one: a tap sorts, and a touch screen sends a
	// tap with no press, so it has none.
	pressed    bool
	pressedMod event.KeyModifier
}

// NewHeaderWidget creates a new header widget.
func NewHeaderWidget(grid HeaderAccess, viewport *Viewport) *HeaderWidget {
	h := &HeaderWidget{
		Grid:     grid,
		viewport: viewport,
	}
	h.ExtendBaseWidget(h)
	return h
}

func (h *HeaderWidget) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	return &headerRenderer{header: h}
}

// Tapped sorts by the column tapped. The sort is on the tap, not the mouse
// press: a touch screen sends a tap and no press, so a header that sorted on
// the press never sorted on a phone. A mouse click sends a press, then a
// tap, so it sorts once, with the press's modifiers (Shift adds a column).
// A drag, which is no tap, doesn't sort, and a tap on a column border is the
// border's, for resizing.
func (h *HeaderWidget) Tapped(e *event.PointEvent) {
	mod := event.KeyModifier(0)
	if h.pressed {
		mod = h.pressedMod
	}
	h.pressed, h.pressedMod = false, 0
	if gridDisabled(h.Grid) || h.Grid.HeaderBorderCol(e.Position.X) >= 0 {
		return
	}
	// region-aware column mapping, shared with the body
	if col := h.Grid.ColumnAtX(e.Position.X); col >= 0 {
		h.Grid.OnHeaderTapped(col, mod)
	}
}

func (h *HeaderWidget) MouseDown(e *desktop.MouseEvent) {
	// A disabled grid neither sorts nor resizes (see gridDisabled).
	if gridDisabled(h.Grid) {
		return
	}
	// Check for resize border first
	borderCol := h.Grid.HeaderBorderCol(e.Position.X)
	if borderCol >= 0 {
		h.resizing = true
		h.Grid.OnResizeDragStart(borderCol, e.Position.X)
		return
	}
	// a press that ends in a tap sorts there, with these modifiers
	h.pressed, h.pressedMod = true, e.Modifier
}

// MouseUp ends a column resize with the press that began it. A click on a
// border, a press with no drag, gets no DragEnd, and used to leave the resize
// armed: the next drag resized the column, even a finger scrolling the grid's
// body on a screen with a mouse and a touchscreen (#1511). A press that did
// drag has resized the column by its release, so ending the resize there
// loses nothing, whichever of MouseUp and DragEnd comes first.
func (h *HeaderWidget) MouseUp(e *desktop.MouseEvent) { h.endResize() }

func (h *HeaderWidget) HoverIn(e *event.HoverEvent) {
	h.nearBorder = !gridDisabled(h.Grid) && h.Grid.HeaderBorderCol(e.Position.X) >= 0
}

func (h *HeaderWidget) HoverMoved(e *event.HoverEvent) {
	h.nearBorder = !gridDisabled(h.Grid) && h.Grid.HeaderBorderCol(e.Position.X) >= 0
}

func (h *HeaderWidget) HoverOut() {
	h.nearBorder = false
}

func (h *HeaderWidget) Cursor() desktop.Cursor {
	if gridDisabled(h.Grid) {
		return desktop.DefaultCursor
	}
	if h.nearBorder || h.resizing {
		return desktop.HResizeCursor
	}
	return desktop.DefaultCursor
}

func (h *HeaderWidget) Dragged(e *event.DragEvent) {
	if h.resizing && !gridDisabled(h.Grid) {
		h.Grid.OnResizeDrag(e.Position.X)
	}
}

// ClaimsDrag takes a touch drag only while a column resize is under way
// (event.DragClaimer, #1266). A resize starts from a mouse press on a column
// border, and a touch sends no mouse press, so on touch the header claims
// nothing: the grid around it scrolls its body, or leaves the drag to the
// page.
func (h *HeaderWidget) ClaimsDrag(geo.Delta) bool {
	return h.resizing && !gridDisabled(h.Grid)
}

func (h *HeaderWidget) DragEnd() {
	h.endResize()
}

func (h *HeaderWidget) endResize() {
	if h.resizing {
		h.resizing = false
		h.Grid.OnResizeDragEnd()
	}
}

type headerRenderer struct {
	header     *HeaderWidget
	objects    []core.CanvasObject
	bg         *canvas.Rectangle
	labels     []*canvas.Text
	dividers   []*canvas.Line
	sortArrows []*canvas.Text    // "▲" or "▼" indicators
	filterDots []*canvas.Circle  // small dot when filter active
	a11y       []*headerCellA11y // invisible accessible node per column (#109)
	selectAll  *selectAllCheck   // in the checkbox-selection column, when the grid offers it

	// bands places the columns in the body's three bands and clips the scrolling
	// main band so its canvas objects cannot render outside [mainX, mainRight)
	// (shared with the floating-filter renderer).
	bands regionBands
}

func (r *headerRenderer) Layout(size geo.Size) {
	r.rebuild(size)
}

func (r *headerRenderer) MinSize() geo.Size {
	h := r.header.Grid.HeaderHeight()
	// Width is determined by the grid layout, not the header itself.
	return geo.NewSize(0, h)
}

func (r *headerRenderer) Refresh() {
	r.rebuild(r.header.Size())
}

func (r *headerRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *headerRenderer) Destroy() {}

func (r *headerRenderer) rebuild(size geo.Size) {
	grid := r.header.Grid
	tc := resolveColors(grid)
	colCount := grid.HeaderColumnCount()
	headerH := grid.HeaderHeight()
	padding := grid.CellPadding()

	// Ensure background
	if r.bg == nil {
		r.bg = canvas.NewRectangle(tc.HeaderBg)
	}
	r.bg.FillColor = tc.HeaderBg
	r.bg.Resize(size)

	// Ensure enough labels
	for len(r.labels) < colCount {
		t := canvas.NewText("", tc.HeaderFg)
		t.TextStyle = bellui.TextStyle{Bold: true}
		t.Alignment = bellui.TextAlignLeading
		r.labels = append(r.labels, t)
	}

	// Ensure enough sort arrows
	for len(r.sortArrows) < colCount {
		t := canvas.NewText("", tc.Primary)
		t.TextSize = 10
		t.Alignment = bellui.TextAlignTrailing
		r.sortArrows = append(r.sortArrows, t)
	}

	// Ensure enough filter dots
	for len(r.filterDots) < colCount {
		c := &canvas.Circle{
			FillColor: tc.Primary,
		}
		c.Hide()
		r.filterDots = append(r.filterDots, c)
	}

	// Ensure enough dividers
	for len(r.dividers) < colCount {
		l := canvas.NewLine(tc.Separator)
		l.StrokeWidth = 1
		r.dividers = append(r.dividers, l)
	}

	// Ensure enough accessible header nodes
	for len(r.a11y) < colCount {
		r.a11y = append(r.a11y, newHeaderCellA11y(grid))
	}

	sortIndicatorW := float32(14)
	filterDotR := float32(3)

	// placeCol positions column i's decorations at X x (widget-space for pinned
	// bands, clip-local for the main band), sizing to width w, and appends them to
	// dst. The shared regionBands helper drives it once per band.
	placeCol := func(i int, x, w float32, dst []core.CanvasObject) []core.CanvasObject {
		// Sort indicator
		sortState := grid.SortState(i)
		arrow := r.sortArrows[i]
		switch sortState {
		case SortIndicatorAsc:
			arrow.Text = "▲"
			arrow.Show()
		case SortIndicatorDesc:
			arrow.Text = "▼"
			arrow.Show()
		default:
			arrow.Text = ""
			arrow.Hide()
		}
		arrow.Color = tc.Primary
		arrowX := x + w - padding - sortIndicatorW
		arrow.Move(geo.NewPos(arrowX, padding))
		arrow.Resize(geo.NewSize(sortIndicatorW, headerH-padding*2))
		dst = append(dst, arrow)

		// Label (leave room for sort arrow)
		label := r.labels[i]
		label.Text = grid.ColumnHeader(i)
		if hider, ok := grid.(HeaderTextHider); ok && hider.ColumnHeaderTextHidden(i) {
			label.Text = ""
		}
		label.Color = tc.HeaderFg
		labelW := w - padding*2
		if sortState != SortIndicatorNone {
			labelW -= sortIndicatorW
		}
		label.Move(geo.NewPos(x+padding, padding))
		label.Resize(geo.NewSize(labelW, headerH-padding*2))
		dst = append(dst, label)

		// Filter active dot
		dot := r.filterDots[i]
		if grid.IsFilterActive(i) {
			dot.FillColor = tc.Primary
			dotX := x + padding
			dotY := headerH - padding - filterDotR*2
			dot.Move(geo.NewPos(dotX, dotY))
			dot.Resize(geo.NewSize(filterDotR*2, filterDotR*2))
			dot.Show()
		} else {
			dot.Hide()
		}
		dst = append(dst, dot)

		// Divider (vertical line on right edge)
		div := r.dividers[i]
		div.StrokeColor = tc.Separator
		div.Position1 = geo.NewPos(x+w, 0)
		div.Position2 = geo.NewPos(x+w, headerH)
		dst = append(dst, div)

		// Accessible header node (semantics only, invisible) covering the
		// column's header cell, so assistive tech can read the column title
		// and sort a sortable column (#109). Placed per band like the visuals:
		// a main-band node lives inside the clip, so it scrolls and clips with
		// its column; pinned-band nodes stay fixed.
		node := r.a11y[i]
		node.set(i, grid.ColumnHeader(i), sortState, grid.ColumnSortable(i))
		node.Move(geo.NewPos(x, 0))
		node.Resize(geo.NewSize(w, headerH))
		dst = append(dst, node)
		return dst
	}

	r.bands.placeLead = r.placeSelectAll

	// The full-width background sits behind everything; the shared helper layers
	// the clipped main band and the fixed pinned bands on top.
	r.objects = r.bands.layout(grid, size, placeCol, r.bg)
	theme.MatchScope(r.header, r.objects...) // see CellsRenderer.refreshCells (#758)
}

// placeSelectAll places the select-all box in the checkbox-selection column's
// header cell, at clip-local x and width w, when the grid offers it. Its state
// follows the rows: checked when all are selected, partial when some are. A tap
// on a partial box checks it, so it selects all.
func (r *headerRenderer) placeSelectAll(x, w float32, dst []core.CanvasObject) []core.CanvasObject {
	grid := r.header.Grid
	sa, ok := grid.(SelectAllAccess)
	if !ok {
		return dst
	}
	shown, all, some := sa.SelectAllState()
	if !shown {
		return dst
	}
	if r.selectAll == nil {
		r.selectAll = newSelectAllCheck()
		r.selectAll.OnChanged = func(checked bool) {
			if s, ok := r.header.Grid.(SelectAllAccess); ok {
				s.SetAllRowsSelected(checked)
			}
		}
	}
	c := r.selectAll
	// Set the state directly: SetChecked would report it back as a change.
	c.Checked, c.Partial = all, some && !all
	syncDisabled(c, gridDisabled(grid))
	c.Refresh()
	h := grid.HeaderHeight()
	c.Move(geo.NewPos(x, 0))
	c.Resize(geo.NewSize(checkboxCellWidth, h))
	return append(dst, c)
}
