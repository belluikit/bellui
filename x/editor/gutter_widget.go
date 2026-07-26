package editor

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// gutterWidget is an internal Fyne widget that renders gutter content.
// It lives outside the text scroll viewport and only scrolls vertically,
// keeping line numbers and markers pinned when the text scrolls horizontally.
type gutterWidget struct {
	widget.BaseWidget

	provider  GutterProvider
	scrollY   float32 // vertical scroll offset, synced from editor
	topOffset float32 // Y of first row, from Entry.RowMetrics()
	stride    float32 // row-to-row Y distance, from Entry.RowMetrics()

	// getLineCount returns the document's live logical-line count (the fork's
	// O(1) Entry.LineCount, #149) — replaces counting a pushed text copy,
	// which was an O(document) scan per repaint and went stale between
	// DeferTextSync sync points.
	getLineCount func() int

	// getRowPositions returns exact row geometry from the Entry.
	// Called during Refresh to always get the latest layout data.
	getRowPositions func() ([]float32, []float32)

	// getRowWindow returns only the rows visible in [scrollY, scrollY+height],
	// each with its content-space top and height, in O(viewport). It returns a
	// non-nil (possibly empty) slice on large documents and nil to defer to the
	// exact getRowPositions path. Keeps the gutter viewport-scaled on big files.
	getRowWindow func(scrollY, viewportH float32) (rows []int, tops []float32, heights []float32)

	// getRowAtContentY maps a content-space Y to its logical row analytically
	// (large documents), or reports ok=false to fall back to the position scan.
	getRowAtContentY func(contentY float32) (row int, ok bool)
}

func newGutterWidget(provider GutterProvider) *gutterWidget {
	g := &gutterWidget{provider: provider}
	g.ExtendBaseWidget(g)
	return g
}

func (g *gutterWidget) rowCount() int {
	if g.getLineCount == nil {
		return 1
	}
	return g.getLineCount()
}

func (g *gutterWidget) gutterWidth() float32 {
	if g.provider == nil {
		return 0
	}
	return g.provider.Width(g.rowCount(), editorTextSize())
}

// CreateRenderer implements core.Widget.
func (g *gutterWidget) CreateRenderer() core.WidgetRenderer {
	return &gutterWidgetRenderer{gutter: g}
}

// MinSize returns the gutter's minimum size: provider width x one row.
func (g *gutterWidget) MinSize() geo.Size {
	w := g.gutterWidth()
	s := g.stride
	if s <= 0 {
		s = editorCharHeight() // fallback before metrics are set
	}
	return geo.NewSize(w, s)
}

// Refresh triggers a full repaint of the gutter widget.
// Overrides BaseWidget.Refresh to also notify the canvas, since the
// base implementation only refreshes the renderer without triggering a
// canvas repaint.
func (g *gutterWidget) Refresh() {
	g.BaseWidget.Refresh()
	canvas.Refresh(g)
}

// Tapped handles tap events on the gutter, dispatching to InteractiveGutter
// providers.
func (g *gutterWidget) Tapped(ev *event.PointEvent) {
	ig, ok := g.provider.(InteractiveGutter)
	if !ok {
		return
	}
	clickY := ev.Position.Y + g.scrollY
	row := -1
	// Large-document fast path: map the click analytically, avoiding an
	// O(document) RowPositions() scan.
	if g.getRowAtContentY != nil {
		if lr, ok := g.getRowAtContentY(clickY); ok {
			row = lr
		}
	}
	if row < 0 {
		// Exact path: prefer the Entry's row positions, else stride-based mapping.
		var rowYPos, rowH []float32
		if g.getRowPositions != nil {
			rowYPos, rowH = g.getRowPositions()
		}
		if len(rowYPos) > 0 {
			for i, ry := range rowYPos {
				rh := g.stride
				if i < len(rowH) {
					rh = rowH[i]
				}
				if clickY < ry+rh {
					row = i
					break
				}
			}
		} else {
			stride := g.stride
			if stride <= 0 {
				stride = editorCharHeight()
			}
			if stride > 0 {
				row = int((clickY - g.topOffset) / stride)
			}
		}
	}
	if row >= 0 && row < g.rowCount() {
		// Prefer column-aware routing (CompositeGutter) so the tap reaches only the
		// clicked column; the gutter does not scroll horizontally, so the event's X
		// is the offset from the gutter's left edge.
		if ct, ok := g.provider.(columnTappable); ok {
			ct.TappedAtX(row+1, ev.Position.X)
		} else {
			ig.Tapped(row + 1)
		}
		g.Refresh()
	}
}

// gutterWidgetRenderer renders the gutter content.
type gutterWidgetRenderer struct {
	gutter            *gutterWidget
	objects           []core.CanvasObject
	bg                *canvas.Rectangle
	lastSize          geo.Size
	hasExactPositions bool
}

func (r *gutterWidgetRenderer) Destroy() {}

func (r *gutterWidgetRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *gutterWidgetRenderer) MinSize() geo.Size {
	return r.gutter.MinSize()
}

func (r *gutterWidgetRenderer) Layout(size geo.Size) {
	r.lastSize = size
	r.Refresh()
	// On the first few frames, exact row positions may not be available
	// yet (Entry layout hasn't completed). Re-render once they arrive.
	if !r.hasExactPositions && r.gutter.getRowPositions != nil {
		if yPos, _ := r.gutter.getRowPositions(); yPos != nil {
			r.hasExactPositions = true
		}
	}
}

func (r *gutterWidgetRenderer) Refresh() {
	g := r.gutter
	r.objects = nil

	if g.provider == nil {
		return
	}

	// Use the size from the last Layout() call — g.Size() may be stale
	// when Refresh is triggered before the layout pass completes.
	size := r.lastSize
	if size.IsZero() {
		size = g.Size()
	}
	textSize := editorTextSize()
	gutterW := g.gutterWidth()
	stride := g.stride
	if stride <= 0 {
		stride = editorCharHeight()
	}

	// Background.
	bgColor := themeColor(ColorNameGutterBackground)
	if bgColor == nil {
		bgColor = color.Transparent
	}
	r.bg = canvas.NewRectangle(bgColor)
	r.bg.Resize(size)
	r.objects = append(r.objects, r.bg)

	// Large-document fast path: iterate only the rows in the viewport band,
	// computed analytically in O(viewport) — never touching an O(document)
	// RowPositions() array.
	if g.getRowWindow != nil {
		if rows, tops, heights := g.getRowWindow(g.scrollY, size.Height); rows != nil {
			for i, row := range rows {
				y := tops[i] - g.scrollY
				rh := heights[i]
				if y+rh < 0 {
					continue
				}
				if y > size.Height {
					break
				}
				r.emitRow(g, row, y, rh, gutterW, textSize)
			}
			return
		}
	}

	// Exact path (small documents): pull the full row positions from the Entry
	// (may be nil before first layout) and fill any blank space below the content
	// with stride-based phantom rows so the gutter spans the whole editor height.
	topPad := g.topOffset
	var rowYPos, rowH []float32
	if g.getRowPositions != nil {
		rowYPos, rowH = g.getRowPositions()
	}

	contentRows := g.rowCount()
	if len(rowYPos) > 0 {
		contentRows = len(rowYPos)
	}
	visibleRows := contentRows
	if stride > 0 {
		fitRows := int((size.Height-topPad)/stride) + 1
		if fitRows > visibleRows {
			visibleRows = fitRows
		}
	}

	for row := 0; row < visibleRows; row++ {
		var y, rh float32
		if row < len(rowYPos) && row < len(rowH) {
			y = rowYPos[row] - g.scrollY
			rh = rowH[row]
		} else {
			y = topPad + float32(row)*stride - g.scrollY
			rh = stride
		}
		// Skip folded-away rows: a hidden row collapses to zero height and shares
		// the next visible row's Y, so drawing its line number here stacks the
		// hidden numbers into a smudge at the fold point (#116 follow-up).
		if rh <= 0 {
			continue
		}
		// Skip rows fully above the visible area.
		if y+rh < 0 {
			continue
		}
		if y > size.Height {
			break
		}
		r.emitRow(g, row, y, rh, gutterW, textSize)
	}
}

// emitRow renders the gutter provider's objects for a single row at viewport-space
// top y and height rh, offsetting each object by y. row is the 0-based logical row.
func (r *gutterWidgetRenderer) emitRow(g *gutterWidget, row int, y, rh, gutterW, textSize float32) {
	cellSize := geo.NewSize(gutterW, rh)
	objs := g.provider.Render(row+1, cellSize, textSize)
	for _, obj := range objs {
		pos := obj.Position()
		obj.Move(geo.NewPos(pos.X, pos.Y+y))
		r.objects = append(r.objects, obj)
	}
}
