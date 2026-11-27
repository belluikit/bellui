package tables

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
)

// Right to left (#779), the grid is placed as it would be left to right and
// then mirrored across its width: pinned-left goes to the right edge, the
// main band to its left with column 0 first, and pinned-right to the left
// end. Each band mirrors its own rows, in the render package's renderers, and
// pointer input is mapped back here, so the band geometry (bandAnchors and
// everything that reads it) stays left to right.
//
// The main scroll's horizontal offset, g.offset.X, is kept measured from the
// leading edge, where column 0 is. The grid converts it where it reads or sets
// the Scroll's own offset, and the render package's ScrollContainer keeps it
// when the scroll content changes width.
//
// The mirror is exact once the Scroll draws its vertical bar on its left
// under right-to-left (#1496): the bar's gutter beside pinned-right, and the
// strip the group-row overlay leaves for it, mirror with the rest.

// mirrorPlaced mirrors objects placed left to right across width, when the
// grid is laid out right to left.
func (g *DataGrid) mirrorPlaced(width float32, objs ...core.CanvasObject) {
	if g.rtl {
		layout.MirrorChildren(objs, width)
	}
}

// drawnX is where a span placed left to right at x, w wide, on a grid width
// wide, is drawn.
func (g *DataGrid) drawnX(x, w, width float32) float32 {
	if !g.rtl {
		return x
	}
	return width - x - w
}

// ltrX maps a pointer's x on the grid, as drawn, to where it is left to
// right, which is what the band geometry hit-tests in.
func (g *DataGrid) ltrX(x float32) float32 {
	if !g.rtl {
		return x
	}
	return g.Size().Width - x
}

// flipScrollX converts between the main Scroll's own horizontal offset and
// one measured from the leading edge, either way: they are the same left to
// right, and right to left each is the other's distance from the far end.
func (g *DataGrid) flipScrollX(x float32) float32 {
	if !g.rtl {
		return x
	}
	far := g.cellsWidget.Size().Width - g.scrollContent.Scroll.Size().Width
	return geo.Max(0, far-x)
}

// towardFirstColumn is the arrow key that moves the focus a column toward
// column 0: Left, or Right under right-to-left, where column 0 is at the
// right.
func (g *DataGrid) towardFirstColumn() event.KeyName {
	if g.rtl {
		return event.KeyRight
	}
	return event.KeyLeft
}

// towardLastColumn is the other horizontal arrow key.
func (g *DataGrid) towardLastColumn() event.KeyName {
	if g.rtl {
		return event.KeyLeft
	}
	return event.KeyRight
}

// resizeDelta is how much a column resize drag from startX to x widens the
// column: its trailing edge is its right one left to right, its left one
// right to left.
func (g *DataGrid) resizeDelta(startX, x float32) float32 {
	if g.rtl {
		return startX - x
	}
	return x - startX
}
