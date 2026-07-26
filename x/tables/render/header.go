package render

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

var (
	_ desktop.Mouseable  = (*HeaderWidget)(nil)
	_ event.Hoverable    = (*HeaderWidget)(nil)
	_ desktop.Cursorable = (*HeaderWidget)(nil)
	_ event.Draggable    = (*HeaderWidget)(nil)
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

func (h *HeaderWidget) Tapped(e *event.PointEvent) {
	// Selection handled in MouseDown for modifier capture
}

func (h *HeaderWidget) MouseDown(e *desktop.MouseEvent) {
	// Check for resize border first
	borderCol := h.Grid.HeaderBorderCol(e.Position.X)
	if borderCol >= 0 {
		h.resizing = true
		h.Grid.OnResizeDragStart(borderCol, e.Position.X)
		return
	}
	// Normal header tap (sort) — region-aware column mapping, shared with the body.
	col := h.Grid.ColumnAtX(e.Position.X)
	if col >= 0 {
		h.Grid.OnHeaderTapped(col, e.Modifier)
	}
}

func (h *HeaderWidget) MouseUp(e *desktop.MouseEvent) {}

func (h *HeaderWidget) HoverIn(e *event.HoverEvent) {
	h.nearBorder = h.Grid.HeaderBorderCol(e.Position.X) >= 0
}

func (h *HeaderWidget) HoverMoved(e *event.HoverEvent) {
	h.nearBorder = h.Grid.HeaderBorderCol(e.Position.X) >= 0
}

func (h *HeaderWidget) HoverOut() {
	h.nearBorder = false
}

func (h *HeaderWidget) Cursor() desktop.Cursor {
	if h.nearBorder || h.resizing {
		return desktop.HResizeCursor
	}
	return desktop.DefaultCursor
}

func (h *HeaderWidget) Dragged(e *event.DragEvent) {
	if h.resizing {
		h.Grid.OnResizeDrag(e.Position.X)
	}
}

func (h *HeaderWidget) DragEnd() {
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
	sortArrows []*canvas.Text   // "▲" or "▼" indicators
	filterDots []*canvas.Circle // small dot when filter active

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
	tc := ResolveThemeColors()
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
		return dst
	}

	// The full-width background sits behind everything; the shared helper layers
	// the clipped main band and the fixed pinned bands on top.
	r.objects = r.bands.layout(grid, size, placeCol, r.bg)
}
