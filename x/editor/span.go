package editor

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// SpanKind selects how a [DecoratedSpan] is rendered.
type SpanKind int

const (
	// SpanUnderline draws an underline beneath the span (see UnderlineStyle for
	// solid links vs. wavy/dotted diagnostics).
	SpanUnderline SpanKind = iota
	// SpanBackground fills the span's cells with a (usually translucent) wash.
	SpanBackground
	// SpanBox strokes a rectangle around the span without filling it.
	SpanBox
	// SpanStrikeThrough draws a line through the middle of the span.
	SpanStrikeThrough
)

// DecoratedSpan is a single character-range decoration: a range plus a render
// style. It is the unit returned by a [SpanDecorator] and the building block on
// which diagnostics squiggles (Phase 3) and redaction overlays (Phase 4) sit.
type DecoratedSpan struct {
	// Range is the character range to decorate.
	Range TextRange
	// Kind selects the shape (underline, background, box, strike-through).
	Kind SpanKind
	// Color is the decoration color. A nil color falls back to a diagnostic red.
	Color color.Color
	// UnderlineStyle selects solid vs. wavy/dotted for SpanUnderline. Ignored
	// for other kinds.
	UnderlineStyle UnderlineStyle
	// Opaque, when true with SpanBackground, paints a fully opaque rectangle
	// that covers the text (redaction) instead of a translucent wash.
	Opaque bool
}

// SpanDecorator supplies character-range decorations for the editor overlay.
//
// Spans() is called on every text change and scroll, so implementations should
// be cheap to recompute (return a precomputed slice, or derive it quickly).
type SpanDecorator interface {
	// Spans returns the ranges this decorator wants to draw, each with a render
	// style. Return nil for nothing to draw.
	Spans() []DecoratedSpan
}

// StaticSpanDecorator renders a fixed set of spans supplied by the host. It is
// the building block diagnostics (Phase 3) and redaction (Phase 4) layer on.
type StaticSpanDecorator struct {
	// SpansList is the set of spans to draw.
	SpansList []DecoratedSpan
}

// Spans implements [SpanDecorator].
func (d *StaticSpanDecorator) Spans() []DecoratedSpan {
	return d.SpansList
}

// SearchMatchSpanDecorator highlights search matches as character-range
// backgrounds — the span-level upgrade of the row-level
// [SearchHighlightDecorator]. The active match (the one the find bar has
// navigated to) is drawn in a stronger color so it stands out from the rest.
type SearchMatchSpanDecorator struct {
	// Matches is the set of matched ranges to highlight.
	Matches []TextRange
	// Active is the index into Matches of the current/active match, drawn in
	// ActiveColor. An out-of-range index (e.g. -1) highlights none as active.
	// The zero value highlights Matches[0]; the find bar always sets this
	// explicitly.
	Active int
	// Color is the highlight color for non-active matches. Defaults to a
	// translucent orange when nil.
	Color color.Color
	// ActiveColor is the highlight color for the active match. Defaults to a
	// stronger orange when nil.
	ActiveColor color.Color
}

// Spans implements [SpanDecorator].
func (d *SearchMatchSpanDecorator) Spans() []DecoratedSpan {
	c := d.Color
	if c == nil {
		c = color.NRGBA{R: 255, G: 180, B: 50, A: 70}
	}
	ac := d.ActiveColor
	if ac == nil {
		ac = color.NRGBA{R: 255, G: 150, B: 0, A: 140}
	}
	spans := make([]DecoratedSpan, 0, len(d.Matches))
	for i, m := range d.Matches {
		col := c
		if i == d.Active {
			col = ac
		}
		spans = append(spans, DecoratedSpan{Range: m, Kind: SpanBackground, Color: col})
	}
	return spans
}

// defaultSpanColor is used when a DecoratedSpan leaves Color nil.
var defaultSpanColor = color.NRGBA{R: 220, G: 50, B: 50, A: 255}

// spanObjects renders a single DecoratedSpan over one of its per-row
// rectangles (already in viewport coordinates).
func spanObjects(span DecoratedSpan, rect geo.Rectangle) []core.CanvasObject {
	col := span.Color
	if col == nil {
		col = defaultSpanColor
	}

	switch span.Kind {
	case SpanBackground:
		if span.Opaque {
			col = toOpaque(col)
		}
		r := canvas.NewRectangle(col)
		r.Move(rect.Pos)
		r.Resize(rect.Size)
		return []core.CanvasObject{r}

	case SpanBox:
		r := canvas.NewRectangle(color.Transparent)
		r.StrokeColor = col
		r.StrokeWidth = 1
		r.Move(rect.Pos)
		r.Resize(rect.Size)
		return []core.CanvasObject{r}

	case SpanStrikeThrough:
		y := rect.Pos.Y + rect.Size.Height/2
		ln := canvas.NewLine(col)
		ln.StrokeWidth = max(float32(1), rect.Size.Height/14)
		ln.Position1 = geo.NewPos(rect.Pos.X, y)
		ln.Position2 = geo.NewPos(rect.Pos.X+rect.Size.Width, y)
		return []core.CanvasObject{ln}

	default: // SpanUnderline
		return underlineObjects(rect.Pos.X, rect.Pos.X+rect.Size.Width,
			rect.Pos.Y+rect.Size.Height, rect.Size.Height, span.UnderlineStyle, col)
	}
}

// underlineObjects builds the canvas objects for an underline spanning
// [x0, x1] just below bottomY, sized relative to rowHeight. Wavy underlines are
// a polyline approximating a sine wave; intended for short ranges.
func underlineObjects(x0, x1, bottomY, rowHeight float32, style UnderlineStyle, col color.Color) []core.CanvasObject {
	if x1 <= x0 {
		return nil
	}
	thickness := max(float32(1), rowHeight/14)

	switch style {
	case UnderlineWavy:
		amp := max(float32(1), thickness)
		wavelen := max(float32(4), thickness*4)
		center := bottomY - thickness - amp
		step := wavelen / 6

		var objs []core.CanvasObject
		prev := geo.NewPos(x0, center)
		for x := x0 + step; ; x += step {
			if x > x1 {
				x = x1
			}
			y := center + amp*float32(math.Sin(float64(2*math.Pi*(x-x0)/wavelen)))
			cur := geo.NewPos(x, y)
			seg := canvas.NewLine(col)
			seg.StrokeWidth = thickness
			seg.Position1 = prev
			seg.Position2 = cur
			objs = append(objs, seg)
			prev = cur
			if x >= x1 {
				break
			}
		}
		return objs

	case UnderlineDotted:
		y := bottomY - thickness
		step := thickness * 2
		var objs []core.CanvasObject
		for x := x0; x < x1; x += step {
			w := thickness
			if x+w > x1 {
				w = x1 - x
			}
			dot := canvas.NewRectangle(col)
			dot.Move(geo.NewPos(x, y))
			dot.Resize(geo.NewSize(w, thickness))
			objs = append(objs, dot)
		}
		return objs

	default: // UnderlineSolid
		y := bottomY - thickness
		ln := canvas.NewLine(col)
		ln.StrokeWidth = thickness
		ln.Position1 = geo.NewPos(x0, y)
		ln.Position2 = geo.NewPos(x1, y)
		return []core.CanvasObject{ln}
	}
}

// toOpaque returns col with its alpha forced to fully opaque, for redaction
// fills.
func toOpaque(col color.Color) color.Color {
	r, g, b, _ := col.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
}
