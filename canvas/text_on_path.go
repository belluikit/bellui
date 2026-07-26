package canvas

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*TextOnPath)(nil)

// TextOnPath describes text that follows a path, with each glyph
// positioned and rotated along the path's arc length.
//
// Since: 2.7
type TextOnPath struct {
	baseObject

	Color      color.Color      // The text draw color
	Text       string           // The string content
	TextSize   float32          // Font size
	TextStyle  bellui.TextStyle // Style for the text
	FontSource bellui.Resource  // Custom font resource; nil means use theme font

	// Offset is the starting distance along the path where text begins.
	// Default 0 means text starts at the path's first point.
	Offset float32

	// BaselineShift moves glyphs perpendicular to the path direction.
	// Positive values shift upward (away from the path's convex side).
	// Default 0 places the text baseline on the path.
	BaselineShift float32

	elements []PathElement
	bounds   [4]float32 // cached bounding box: minX, minY, maxX, maxY
}

// MoveTo begins a new subpath at the given point.
func (t *TextOnPath) MoveTo(x, y float32) {
	t.elements = append(t.elements, PathElement{
		cmd:    PathMoveTo,
		points: [3]geo.Position{geo.NewPos(x, y)},
	})
	t.updateBoundsForPoint(x, y)
}

// LineTo adds a straight line segment from the current point to (x, y).
func (t *TextOnPath) LineTo(x, y float32) {
	t.elements = append(t.elements, PathElement{
		cmd:    PathLineTo,
		points: [3]geo.Position{geo.NewPos(x, y)},
	})
	t.updateBoundsForPoint(x, y)
}

// QuadTo adds a quadratic Bezier curve from the current point,
// with control point (cx, cy) and endpoint (x, y).
func (t *TextOnPath) QuadTo(cx, cy, x, y float32) {
	t.elements = append(t.elements, PathElement{
		cmd:    PathQuadTo,
		points: [3]geo.Position{geo.NewPos(cx, cy), geo.NewPos(x, y)},
	})
	t.updateBoundsForPoint(cx, cy)
	t.updateBoundsForPoint(x, y)
}

// CubicTo adds a cubic Bezier curve from the current point,
// with control points (cx1, cy1), (cx2, cy2) and endpoint (x, y).
func (t *TextOnPath) CubicTo(cx1, cy1, cx2, cy2, x, y float32) {
	t.elements = append(t.elements, PathElement{
		cmd:    PathCubicTo,
		points: [3]geo.Position{geo.NewPos(cx1, cy1), geo.NewPos(cx2, cy2), geo.NewPos(x, y)},
	})
	t.updateBoundsForPoint(cx1, cy1)
	t.updateBoundsForPoint(cx2, cy2)
	t.updateBoundsForPoint(x, y)
}

// Close closes the current subpath by drawing a straight line back to the
// starting point of the subpath.
func (t *TextOnPath) Close() {
	t.elements = append(t.elements, PathElement{cmd: PathClose})
}

// Reset clears all path commands.
func (t *TextOnPath) Reset() {
	t.elements = t.elements[:0]
	t.bounds = [4]float32{math.MaxFloat32, math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
}

// Elements returns the path elements. The returned slice must not be modified.
func (t *TextOnPath) Elements() []PathElement {
	return t.elements
}

// Hide will set this text to not be visible.
func (t *TextOnPath) Hide() {
	t.baseObject.Hide()
	repaint(t)
}

// MinSize returns the minimum size based on the path bounds expanded by
// the text height (glyphs can extend beyond the path due to rotation).
func (t *TextOnPath) MinSize() geo.Size {
	if len(t.elements) == 0 || t.Text == "" {
		return geo.Size{}
	}

	textSize, _ := bellui.MeasureTextFull(t.Text, t.TextSize, t.TextStyle, t.FontSource)
	pad := textSize.Height // glyphs can rotate, extending beyond path bounds

	minX, minY := t.bounds[0], t.bounds[1]
	maxX, maxY := t.bounds[2], t.bounds[3]
	if minX > maxX { // no points added
		return textSize
	}

	w := maxX - minX + pad*2
	h := maxY - minY + pad*2
	return geo.NewSize(w, h)
}

// Move the text on path to a new position, relative to its parent / canvas.
func (t *TextOnPath) Move(pos geo.Position) {
	if t.Position() == pos {
		return
	}
	t.baseObject.Move(pos)
	repaint(t)
}

// Resize updates the size of this object.
func (t *TextOnPath) Resize(s geo.Size) {
	if s == t.Size() {
		return
	}
	t.baseObject.Resize(s)
	Refresh(t)
}

// SetMinSize has no effect as the smallest size is based on the path and text content.
func (t *TextOnPath) SetMinSize(geo.Size) {
	// no-op
}

// Refresh causes this text on path to be redrawn with its configured state.
func (t *TextOnPath) Refresh() {
	Refresh(t)
}

func (t *TextOnPath) updateBoundsForPoint(x, y float32) {
	if x < t.bounds[0] {
		t.bounds[0] = x
	}
	if y < t.bounds[1] {
		t.bounds[1] = y
	}
	if x > t.bounds[2] {
		t.bounds[2] = x
	}
	if y > t.bounds[3] {
		t.bounds[3] = y
	}
}

// NewTextOnPath creates a new TextOnPath and calls the build function to construct the path.
//
// Since: 2.7
func NewTextOnPath(text string, color color.Color, buildPath func(*TextOnPath)) *TextOnPath {
	t := &TextOnPath{
		Color:    color,
		Text:     text,
		TextSize: theme.TextSize(),
		bounds:   [4]float32{math.MaxFloat32, math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32},
	}
	if buildPath != nil {
		buildPath(t)
	}
	return t
}
