package widget

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// newUnderlineDecoration builds the canvas objects that draw a StyleRun
// underline spanning [x0, x1] just below bottomY (the bottom of the glyph box),
// in the given style and color. Geometry is derived from textSize so the
// decoration scales with the font.
//
// Solid underlines are a single stroked line. Dotted underlines are a row of
// small square marks. Wavy underlines (diagnostic squiggles) are a polyline
// approximating a sine wave; the segment count scales with the span width, so
// they are intended for short ranges (a token, not a whole file).
func newUnderlineDecoration(x0, x1, bottomY, textSize float32, style UnderlineStyle, col color.Color) []core.CanvasObject {
	if x1 <= x0 {
		return nil
	}
	thickness := max(float32(1), textSize/12)

	switch style {
	case UnderlineWavy:
		// Amplitude and wavelength scale with the stroke so the squiggle reads
		// at any text size. Center the wave band above the glyph-box bottom so
		// it stays within the row.
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

// newStrikeDecoration builds the canvas object that draws a StyleRun
// strike-through: a single solid line spanning [x0, x1] at midY (the vertical
// middle of the glyph box, ≈ the x-height center), in the given color. The
// stroke thickness scales with textSize, mirroring newUnderlineDecoration.
func newStrikeDecoration(x0, x1, midY, textSize float32, col color.Color) []core.CanvasObject {
	if x1 <= x0 {
		return nil
	}
	thickness := max(float32(1), textSize/12)
	ln := canvas.NewLine(col)
	ln.StrokeWidth = thickness
	ln.Position1 = geo.NewPos(x0, midY)
	ln.Position2 = geo.NewPos(x1, midY)
	return []core.CanvasObject{ln}
}
