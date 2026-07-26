package canvas

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Path)(nil)

// LineCap defines the shape at the endpoints of an open path stroke.
type LineCap uint8

const (
	// LineCapButt ends the stroke flush at the endpoint (default).
	LineCapButt LineCap = iota
	// LineCapRound ends the stroke with a semicircle at the endpoint.
	LineCapRound
	// LineCapSquare ends the stroke with a half-square at the endpoint.
	LineCapSquare
)

// LineJoin defines the shape at the junction of two path segments.
type LineJoin uint8

const (
	// LineJoinMiter joins segments with a sharp point (default).
	LineJoinMiter LineJoin = iota
	// LineJoinRound joins segments with a circular arc.
	LineJoinRound
	// LineJoinBevel joins segments with a flat cut.
	LineJoinBevel
)

// FillRule defines how the interior of a path is determined.
type FillRule uint8

const (
	// FillRuleWinding uses the non-zero winding rule (default).
	FillRuleWinding FillRule = iota
	// FillRuleEvenOdd uses the even-odd rule.
	FillRuleEvenOdd
)

// PathCommand identifies a path drawing command.
type PathCommand uint8

const (
	// PathMoveTo begins a new subpath at the given point.
	PathMoveTo PathCommand = iota
	// PathLineTo adds a straight line from the current point.
	PathLineTo
	// PathQuadTo adds a quadratic Bezier curve from the current point.
	PathQuadTo
	// PathCubicTo adds a cubic Bezier curve from the current point.
	PathCubicTo
	// PathClose closes the current subpath with a straight line back to the start.
	PathClose
)

// PathElement stores a single path command with its control points.
type PathElement struct {
	cmd    PathCommand
	points [3]geo.Position // MoveTo/LineTo: [0], QuadTo: [0]=ctrl [1]=end, CubicTo: [0]=ctrl1 [1]=ctrl2 [2]=end
}

// Cmd returns the path command.
func (e PathElement) Cmd() PathCommand { return e.cmd }

// Point returns the i-th control point (0, 1, or 2).
func (e PathElement) Point(i int) geo.Position { return e.points[i] }

// Path describes an arbitrary 2D vector path in a Bellui canvas.
// It supports fill, stroke with configurable width/caps/joins/dashes,
// and is rendered through the GPU pipeline.
type Path struct {
	baseObject

	FillColor   color.Color // The path fill color
	FillRule    FillRule    // The fill rule (even-odd or winding)
	StrokeColor color.Color // The path stroke color
	StrokeWidth float32     // The stroke width
	LineCap     LineCap     // The stroke cap style
	LineJoin    LineJoin    // The stroke join style
	MiterLimit  float32     // The miter limit for miter joins (0 uses default of 4)
	DashPattern []float32   // The stroke dash pattern (alternating dash/gap lengths)
	DashOffset  float32     // The starting offset into the dash pattern

	elements []PathElement
	bounds   [4]float32 // cached bounding box: minX, minY, maxX, maxY
}

// MoveTo begins a new subpath at the given point.
func (p *Path) MoveTo(x, y float32) {
	p.elements = append(p.elements, PathElement{
		cmd:    PathMoveTo,
		points: [3]geo.Position{geo.NewPos(x, y)},
	})
	p.updateBoundsForPoint(x, y)
}

// LineTo adds a straight line segment from the current point to (x, y).
func (p *Path) LineTo(x, y float32) {
	p.elements = append(p.elements, PathElement{
		cmd:    PathLineTo,
		points: [3]geo.Position{geo.NewPos(x, y)},
	})
	p.updateBoundsForPoint(x, y)
}

// QuadTo adds a quadratic Bezier curve from the current point,
// with control point (cx, cy) and endpoint (x, y).
func (p *Path) QuadTo(cx, cy, x, y float32) {
	p.elements = append(p.elements, PathElement{
		cmd:    PathQuadTo,
		points: [3]geo.Position{geo.NewPos(cx, cy), geo.NewPos(x, y)},
	})
	p.updateBoundsForPoint(cx, cy)
	p.updateBoundsForPoint(x, y)
}

// CubicTo adds a cubic Bezier curve from the current point,
// with control points (cx1, cy1), (cx2, cy2) and endpoint (x, y).
func (p *Path) CubicTo(cx1, cy1, cx2, cy2, x, y float32) {
	p.elements = append(p.elements, PathElement{
		cmd:    PathCubicTo,
		points: [3]geo.Position{geo.NewPos(cx1, cy1), geo.NewPos(cx2, cy2), geo.NewPos(x, y)},
	})
	p.updateBoundsForPoint(cx1, cy1)
	p.updateBoundsForPoint(cx2, cy2)
	p.updateBoundsForPoint(x, y)
}

// Close closes the current subpath by drawing a straight line back to the
// starting point of the subpath.
func (p *Path) Close() {
	p.elements = append(p.elements, PathElement{cmd: PathClose})
}

// Reset clears all path commands, restoring the path to an empty state.
func (p *Path) Reset() {
	p.elements = p.elements[:0]
	p.bounds = [4]float32{math.MaxFloat32, math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
}

// Elements returns the path elements. The returned slice must not be modified.
func (p *Path) Elements() []PathElement {
	return p.elements
}

// Bounds returns the axis-aligned bounding box of the path's control points
// as (minX, minY, maxX, maxY) in path-local coordinates.
func (p *Path) Bounds() (minX, minY, maxX, maxY float32) {
	if len(p.elements) == 0 {
		return 0, 0, 0, 0
	}
	return p.bounds[0], p.bounds[1], p.bounds[2], p.bounds[3]
}

// Hide will set this path to not be visible.
func (p *Path) Hide() {
	p.baseObject.Hide()
	repaint(p)
}

// Move the path to a new position, relative to its parent / canvas.
func (p *Path) Move(pos geo.Position) {
	if p.Position() == pos {
		return
	}
	p.baseObject.Move(pos)
	repaint(p)
}

// Refresh causes this path to be redrawn with its configured state.
func (p *Path) Refresh() {
	Refresh(p)
}

// Resize on a path updates the size of this object.
func (p *Path) Resize(s geo.Size) {
	if s == p.Size() {
		return
	}
	p.baseObject.Resize(s)
	Refresh(p)
}

func (p *Path) updateBoundsForPoint(x, y float32) {
	if x < p.bounds[0] {
		p.bounds[0] = x
	}
	if y < p.bounds[1] {
		p.bounds[1] = y
	}
	if x > p.bounds[2] {
		p.bounds[2] = x
	}
	if y > p.bounds[3] {
		p.bounds[3] = y
	}
}

// NewPath creates a new Path and calls the build function to construct it.
func NewPath(build func(*Path)) *Path {
	p := &Path{
		bounds: [4]float32{math.MaxFloat32, math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32},
	}
	if build != nil {
		build(p)
	}
	return p
}
