package series

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
)

// clipX keeps what a bar, candle, box, pictorial or scatter series drew
// within the plot's left and right edges, vp being the plot (#1396). A zoom
// moves the hidden points outside the plot, and these series drew them there
// anyway, over the axis labels and the next pane of a group; a candle or bar
// partly in view was drawn whole, and zoomed in far one candle is hundreds of
// pixels wide.
//
// A rectangle is cut at the edges and dropped when nothing is left. A line is
// cut where it crosses an edge, so a vertical one, a wick or whisker, is kept
// only when it lies inside. Anything else is a symbol or a label, which cannot
// be cut, and is kept when its middle is inside: one at an edge, such as an
// unzoomed point at the axis minimum, still overhangs by up to half its size.
// Nothing is clipped above or below, where the axis range already keeps the
// shapes. It edits and returns objects in place. A context with no viewport,
// as a caller drawing a series on its own may pass, clips nothing.
func clipX(objects []core.CanvasObject, vp coord.Rect) []core.CanvasObject {
	if vp.W <= 0 {
		return objects
	}
	minX, maxX := vp.X, vp.X+vp.W
	const slack = 0.5 // a shape on the edge is not past it
	kept := objects[:0]
	for _, o := range objects {
		switch v := o.(type) {
		case *canvas.Rectangle:
			x0 := v.Position().X
			x1 := x0 + v.Size().Width
			lo, hi := max(x0, minX), min(x1, maxX)
			if hi-lo <= 0 {
				continue
			}
			if lo-x0 > slack || x1-hi > slack {
				v.Move(geo.NewPos(lo, v.Position().Y))
				v.Resize(geo.NewSize(hi-lo, v.Size().Height))
			}
		case *canvas.Line:
			if !clipLineX(v, minX, maxX, slack) {
				continue
			}
		default:
			mid := o.Position().X + o.Size().Width/2
			if mid < minX-slack || mid > maxX+slack {
				continue
			}
		}
		kept = append(kept, o)
	}
	return kept
}

// clipLineX cuts l to the band minX..maxX and reports whether any of it is
// left.
func clipLineX(l *canvas.Line, minX, maxX, slack float32) bool {
	p1, p2 := l.Position1, l.Position2
	if p1.X > p2.X {
		p1, p2 = p2, p1
	}
	if p2.X < minX-slack || p1.X > maxX+slack {
		return false
	}
	if p1.X == p2.X {
		return true
	}
	at := func(x float32) geo.Position {
		t := (x - p1.X) / (p2.X - p1.X)
		return geo.NewPos(x, p1.Y+(p2.Y-p1.Y)*t)
	}
	if p1.X < minX-slack {
		p1 = at(minX)
	}
	if p2.X > maxX+slack {
		p2 = at(maxX)
	}
	l.Position1, l.Position2 = p1, p2
	return true
}
