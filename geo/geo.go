// Package geo defines the fundamental geometry types used throughout Bellui:
// [Size], [Position], and [Delta].
//
// These types have no dependencies on the rest of the Bellui framework, making
// them safe to import from any package without circular dependency concerns.
//
//	import "github.com/belluikit/bellui/geo"
//
//	size := geo.NewSize(200, 100)
//	pos := geo.NewPos(10, 20)
//	delta := geo.NewDelta(5, 5)
package geo

// Vector2 marks geometry types that can operate as a coordinate vector.
type Vector2 interface {
	Components() (float32, float32)
	IsZero() bool
}

// --- Delta ---

// Delta is a generic X, Y movement or offset representation.
type Delta struct {
	DX, DY float32
}

// NewDelta returns a newly allocated Delta representing a movement in the X and Y axis.
func NewDelta(dx float32, dy float32) Delta {
	return Delta{DX: dx, DY: dy}
}

func (v Delta) Components() (float32, float32) { return v.DX, v.DY }
func (v Delta) IsZero() bool                   { return v.DX == 0.0 && v.DY == 0.0 }

// --- Position ---

// Position describes a generic X, Y coordinate relative to a parent canvas or object.
type Position struct {
	X float32 // The position from the parent's left edge
	Y float32 // The position from the parent's top edge
}

// NewPos returns a newly allocated Position.
func NewPos(x float32, y float32) Position {
	return Position{x, y}
}

// NewSquareOffsetPos returns a Position with the same x and y.
func NewSquareOffsetPos(length float32) Position {
	return Position{length, length}
}

func (p Position) Add(v Vector2) Position {
	x, y := v.Components()
	return Position{p.X + x, p.Y + y}
}

func (p Position) AddXY(x, y float32) Position {
	return Position{p.X + x, p.Y + y}
}

func (p Position) Components() (float32, float32) { return p.X, p.Y }
func (p Position) IsZero() bool                   { return p.X == 0.0 && p.Y == 0.0 }

func (p Position) Subtract(v Vector2) Position {
	x, y := v.Components()
	return Position{p.X - x, p.Y - y}
}

func (p Position) SubtractXY(x, y float32) Position {
	return Position{p.X - x, p.Y - y}
}

// --- Size ---

// Size describes something with width and height.
type Size struct {
	Width  float32 // The number of units along the X axis.
	Height float32 // The number of units along the Y axis.
}

// NewSize returns a newly allocated Size of the specified dimensions.
func NewSize(w float32, h float32) Size {
	return Size{w, h}
}

// NewSquareSize returns a Size with the same width and height.
func NewSquareSize(side float32) Size {
	return Size{side, side}
}

func (s Size) Add(v Vector2) Size {
	w, h := v.Components()
	return Size{s.Width + w, s.Height + h}
}

func (s Size) AddWidthHeight(width, height float32) Size {
	return Size{s.Width + width, s.Height + height}
}

func (s Size) IsZero() bool { return s.Width == 0.0 && s.Height == 0.0 }

func (s Size) Max(v Vector2) Size {
	x, y := v.Components()
	return NewSize(Max(s.Width, x), Max(s.Height, y))
}

func (s Size) Min(v Vector2) Size {
	x, y := v.Components()
	return NewSize(Min(s.Width, x), Min(s.Height, y))
}

func (s Size) Components() (float32, float32) { return s.Width, s.Height }

func (s Size) Subtract(v Vector2) Size {
	w, h := v.Components()
	return Size{s.Width - w, s.Height - h}
}

func (s Size) SubtractWidthHeight(width, height float32) Size {
	return Size{s.Width - width, s.Height - height}
}

// Rectangle describes an axis-aligned rectangle by its top-left position and
// its size. The zero Rectangle (zero Pos and zero Size) is used by callers to
// signal "no/invalid rectangle" — e.g. a caret position that is out of range
// or scrolled out of view.
//
// Since: 2.9
type Rectangle struct {
	Pos  Position
	Size Size
}

// NewRectangle returns a Rectangle at (x, y) with the given width and height.
//
// Since: 2.9
func NewRectangle(x, y, w, h float32) Rectangle {
	return Rectangle{Pos: Position{x, y}, Size: Size{w, h}}
}

// IsZero reports whether the rectangle has both a zero position and zero size,
// the conventional "no rectangle" sentinel.
//
// Since: 2.9
func (r Rectangle) IsZero() bool { return r.Pos.IsZero() && r.Size.IsZero() }

// --- Math helpers ---

// Min returns the smaller of the passed values.
func Min(x, y float32) float32 {
	if x < y {
		return x
	}
	return y
}

// Max returns the larger of the passed values.
func Max(x, y float32) float32 {
	if x > y {
		return x
	}
	return y
}

// Compile-time interface checks.
var (
	_ Vector2 = (*Delta)(nil)
	_ Vector2 = (*Position)(nil)
	_ Vector2 = (*Size)(nil)
)
