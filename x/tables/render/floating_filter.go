package render

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// FloatingFilterAccess is the interface the FloatingFilterRow needs from the
// DataGrid. Like the header, the row renders ALL visible columns (pinned + main
// + right) and shares the body's region geometry (RegionLayoutAccess) rather
// than the main-only ColumnCount/ColumnWidths.
type FloatingFilterAccess interface {
	RegionLayoutAccess
	// OnFloatingFilterChanged receives an absolute display-column index.
	OnFloatingFilterChanged(col int, text string)
}

// FloatingFilterRow renders inline filter Entry widgets below the column headers.
// Its columns share the body's region layout: the main band scrolls horizontally
// with the content while the pinned-left/right bands stay fixed.
type FloatingFilterRow struct {
	widget.BaseWidget

	Grid     FloatingFilterAccess
	viewport *Viewport
	entries  []*widget.Entry
}

// NewFloatingFilterRow creates a floating filter row.
func NewFloatingFilterRow(grid FloatingFilterAccess, viewport *Viewport) *FloatingFilterRow {
	f := &FloatingFilterRow{
		Grid:     grid,
		viewport: viewport,
	}
	f.ExtendBaseWidget(f)
	return f
}

func (f *FloatingFilterRow) CreateRenderer() core.WidgetRenderer {
	f.ExtendBaseWidget(f)
	return &floatingFilterRenderer{row: f}
}

type floatingFilterRenderer struct {
	row     *FloatingFilterRow
	objects []core.CanvasObject

	// bands lays the entries out in the body's three column bands, clipping the
	// scrolling main band so its entries cannot render — or receive pointer
	// events — outside the main region (shared with the header renderer).
	bands regionBands
}

func (r *floatingFilterRenderer) Layout(size geo.Size) {
	r.rebuild(size)
}

func (r *floatingFilterRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 30)
}

func (r *floatingFilterRenderer) Refresh() {
	r.rebuild(r.row.Size())
}

func (r *floatingFilterRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *floatingFilterRenderer) Destroy() {}

func (r *floatingFilterRenderer) rebuild(size geo.Size) {
	grid := r.row.Grid
	colCount := grid.HeaderColumnCount()

	// Ensure one entry per visible column. Each entry's OnChanged reports its own
	// absolute display-column index (stable for the entry's lifetime), which is
	// what OnFloatingFilterChanged expects.
	for len(r.row.entries) < colCount {
		absCol := len(r.row.entries)
		e := widget.NewEntry()
		e.PlaceHolder = "Filter..."
		e.OnChanged = func(text string) {
			grid.OnFloatingFilterChanged(absCol, text)
		}
		r.row.entries = append(r.row.entries, e)
	}

	// Lay the entries out in the three bands (pinned-left / clipped main /
	// pinned-right); the shared helper owns the geometry and the clip.
	place := func(absCol int, x, w float32, dst []core.CanvasObject) []core.CanvasObject {
		entry := r.row.entries[absCol]
		entry.Move(geo.NewPos(x, 0))
		entry.Resize(geo.NewSize(w, size.Height))
		return append(dst, entry)
	}
	r.objects = r.bands.layout(grid, size, place)
}
