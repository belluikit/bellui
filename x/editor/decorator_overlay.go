package editor

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// decoratorOverlay is an internal widget that renders RowDecorator output
// as a transparent overlay. It sits in a Stack behind (or over) the editor
// text and syncs its vertical scroll offset with the Entry.
type decoratorOverlay struct {
	widget.BaseWidget

	decorators     []RowDecorator
	spanDecorators []SpanDecorator
	scrollY        float32 // vertical scroll offset, synced from editor
	topOffset      float32 // Y of first row, from Entry.RowMetrics()
	stride         float32 // row-to-row Y distance, from Entry.RowMetrics()

	// getLineCount returns the document's live logical-line count (the fork's
	// O(1) Entry.LineCount, #149) — replaces counting a pushed text copy,
	// which was an O(document) scan per repaint and went stale between
	// DeferTextSync sync points.
	getLineCount func() int

	// getRowPositions returns exact row geometry from the Entry.
	getRowPositions func() ([]float32, []float32)
	// getRangeRects maps a TextRange to viewport-space rectangles (one per
	// visual row), used to place span decorations.
	getRangeRects func(TextRange) []geo.Rectangle
	// foldPlaceholders returns the collapsed-region markers to draw inline; nil
	// when folding is unused.
	foldPlaceholders func() []foldPlaceholder
	// getCaretRect maps a (row, col) to its viewport-space cell rectangle (the
	// zero rectangle when off-screen), used to place fold placeholders.
	getCaretRect func(row, col int) geo.Rectangle
	// getRowWindow returns only the rows visible in [scrollY, scrollY+height],
	// each with its content-space top and height, in O(viewport). Non-nil (possibly
	// empty) on large documents, nil to defer to the exact getRowPositions path.
	getRowWindow func(scrollY, viewportH float32) (rows []int, tops []float32, heights []float32)
}

func newDecoratorOverlay(decorators []RowDecorator, spanDecorators []SpanDecorator) *decoratorOverlay {
	d := &decoratorOverlay{decorators: decorators, spanDecorators: spanDecorators}
	d.ExtendBaseWidget(d)
	return d
}

func (d *decoratorOverlay) rowCount() int {
	if d.getLineCount == nil {
		return 1
	}
	return d.getLineCount()
}

// Refresh triggers a full repaint of the decorator overlay.
func (d *decoratorOverlay) Refresh() {
	d.BaseWidget.Refresh()
	canvas.Refresh(d)
}

func (d *decoratorOverlay) CreateRenderer() core.WidgetRenderer {
	return &decoratorOverlayRenderer{overlay: d}
}

type decoratorOverlayRenderer struct {
	overlay           *decoratorOverlay
	objects           []core.CanvasObject
	lastSize          geo.Size
	hasExactPositions bool
}

func (r *decoratorOverlayRenderer) Destroy() {}

// IsClip marks the overlay's content as clipped to its own bounds. The overlay
// emits a full-row rect for an edge row that is only partially visible (y <
// height < y+rowHeight); fyne does not auto-clip widget content, so without this
// that overhang draws past the editor — over the gutter or a neighbouring status
// bar — on every backend (this, not a WebGPU-painter difference, was the real
// cause of trendvidia/fyne#341). The marker is the no-arg method the fyne paint
// walk detects via internal/driver.IsClip.
//
// Clipping here (the overlay subtree) rather than at the editor level is
// deliberate: the editor's floatLayer popovers intentionally overflow the editor
// into the window (compact-editor completion/hover, #19/#69), so an editor-wide
// clip would break them.
func (r *decoratorOverlayRenderer) IsClip() {}

func (r *decoratorOverlayRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *decoratorOverlayRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *decoratorOverlayRenderer) Layout(size geo.Size) {
	if size != r.lastSize {
		r.lastSize = size
		r.Refresh()
		return
	}
	if !r.hasExactPositions && r.overlay.getRowPositions != nil {
		if yPos, _ := r.overlay.getRowPositions(); yPos != nil {
			r.hasExactPositions = true
			r.Refresh()
		}
	}
}

func (r *decoratorOverlayRenderer) Refresh() {
	d := r.overlay
	r.objects = nil

	if len(d.decorators) == 0 && len(d.spanDecorators) == 0 && d.foldPlaceholders == nil {
		return
	}

	r.renderRows()
	r.renderSpans()
	r.renderFoldPlaceholders()
}

// renderFoldPlaceholders draws the inline ellipsis/summary after each collapsed
// region's header line. Each placeholder is positioned via getCaretRect at the
// column just past the header line's text; off-screen placeholders (zero rect)
// are skipped.
func (r *decoratorOverlayRenderer) renderFoldPlaceholders() {
	d := r.overlay
	if d.foldPlaceholders == nil || d.getCaretRect == nil {
		return
	}
	col := themeColor(ColorNameFoldPlaceholder)
	for _, p := range d.foldPlaceholders() {
		rect := d.getCaretRect(p.row, p.col)
		if rect.IsZero() {
			continue
		}
		t := canvas.NewText(p.text, col)
		t.TextStyle = bellui.TextStyle{Monospace: true}
		t.TextSize = editorTextSize()
		// Left-align the marker to the caret cell and vertically center it in the row.
		ts := t.MinSize()
		t.Move(geo.NewPos(rect.Pos.X, rect.Pos.Y+(rect.Size.Height-ts.Height)/2))
		r.objects = append(r.objects, t)
	}
}

// renderRows draws the full-width RowDecorator output for each visible row.
func (r *decoratorOverlayRenderer) renderRows() {
	d := r.overlay
	if len(d.decorators) == 0 {
		return
	}

	size := d.Size()
	stride := d.stride
	topPad := d.topOffset
	if stride <= 0 {
		stride = editorCharHeight()
	}
	if stride <= 0 {
		return
	}

	// Large-document fast path: iterate only the rows in the viewport band.
	if d.getRowWindow != nil {
		if rows, tops, heights := d.getRowWindow(d.scrollY, size.Height); rows != nil {
			for i, row := range rows {
				y := tops[i] - d.scrollY
				rh := heights[i]
				if y+rh < 0 {
					continue
				}
				if y > size.Height {
					break
				}
				r.emitRowDecorations(d, row, y, rh, size.Width)
			}
			return
		}
	}

	// Exact path (small documents): pull the full row positions from the Entry.
	var rowYPos, rowH []float32
	if d.getRowPositions != nil {
		rowYPos, rowH = d.getRowPositions()
	}

	rowCount := d.rowCount()
	if len(rowYPos) > 0 {
		rowCount = len(rowYPos)
	}

	for row := 0; row < rowCount; row++ {
		var y, rh float32
		if row < len(rowYPos) && row < len(rowH) {
			y = rowYPos[row] - d.scrollY
			rh = rowH[row]
		} else {
			y = topPad + float32(row)*stride - d.scrollY
			rh = stride
		}
		// Folded-away rows collapse to zero height; skip them so a row decoration
		// isn't emitted as a degenerate zero-height rect at the fold point.
		if rh <= 0 {
			continue
		}
		if y+rh < 0 {
			continue
		}
		if y > size.Height {
			break
		}
		r.emitRowDecorations(d, row, y, rh, size.Width)
	}
}

// emitRowDecorations runs every RowDecorator for logical row (0-based) at
// viewport-space top y and height rh, spanning the full overlay width.
func (r *decoratorOverlayRenderer) emitRowDecorations(d *decoratorOverlay, row int, y, rh, width float32) {
	rowPos := geo.NewPos(0, y)
	rowSize := geo.NewSize(width, rh)
	for _, dec := range d.decorators {
		r.objects = append(r.objects, dec.Decorate(row+1, rowPos, rowSize)...)
	}
}

// renderSpans draws character-range (span) decorations over the rows. Each
// span maps to one rectangle per visual row via getRangeRects (already in
// viewport coordinates); off-screen rectangles are clipped by the widget.
func (r *decoratorOverlayRenderer) renderSpans() {
	d := r.overlay
	if len(d.spanDecorators) == 0 || d.getRangeRects == nil {
		return
	}
	for _, sd := range d.spanDecorators {
		for _, span := range sd.Spans() {
			for _, rect := range d.getRangeRects(span.Range) {
				r.objects = append(r.objects, spanObjects(span, rect)...)
			}
		}
	}
}
