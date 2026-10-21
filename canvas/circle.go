package canvas

import (
	"image/color"
	"math"
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/paintgen"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Circle)(nil)

// Circle describes a colored circle primitive in a Bellui canvas
type Circle struct {
	Position1 geo.Position // The current top-left position of the Circle
	Position2 geo.Position // The current bottomright position of the Circle
	Hidden    bool         // Is this circle currently hidden

	FillColor   color.Color // The circle fill color
	StrokeColor color.Color // The circle stroke color
	StrokeWidth float32     // The stroke width of the circle

	propMu    sync.RWMutex
	transform geo.Matrix
}

// NewCircle returns a new Circle instance
func NewCircle(color color.Color) *Circle {
	return &Circle{FillColor: color}
}

// Hide will set this circle to not be visible
func (c *Circle) Hide() {
	c.propMu.Lock()
	c.Hidden = true
	c.propMu.Unlock()

	repaint(c)
}

// MinSize for a Circle simply returns Size{1, 1} as there is no
// explicit content
func (c *Circle) MinSize() geo.Size {
	return geo.NewSize(1, 1)
}

// Move the circle object to a new position, relative to its parent / canvas
func (c *Circle) Move(pos geo.Position) {
	c.propMu.Lock()
	if c.Position1 == pos {
		c.propMu.Unlock()
		return
	}

	size := c.sizeLocked()
	c.Position1 = pos
	c.Position2 = c.Position1.Add(size)
	c.propMu.Unlock()

	repaint(c)
}

// Position gets the current top-left position of this circle object, relative to its parent / canvas
func (c *Circle) Position() geo.Position {
	c.propMu.RLock()
	pos := c.Position1
	c.propMu.RUnlock()
	return pos
}

// Refresh causes this object to be redrawn with its configured state.
func (c *Circle) Refresh() {
	Refresh(c)
}

// Resize sets a new bottom-right position for the circle object
// If it has a stroke width this will cause it to Refresh.
func (c *Circle) Resize(size geo.Size) {
	c.propMu.Lock()
	if size == c.sizeLocked() {
		c.propMu.Unlock()
		return
	}

	c.Position2 = c.Position1.Add(size)
	c.propMu.Unlock()

	Refresh(c)
}

// Show will set this circle to be visible
func (c *Circle) Show() {
	c.propMu.Lock()
	changed := c.Hidden
	c.Hidden = false
	c.propMu.Unlock()
	if changed {
		paintgen.Changed()
	}

	c.Refresh()
}

// Size returns the current size of bounding box for this circle object
func (c *Circle) Size() geo.Size {
	c.propMu.RLock()
	s := c.sizeLocked()
	c.propMu.RUnlock()
	return s
}

func (c *Circle) sizeLocked() geo.Size {
	return geo.NewSize(
		float32(math.Abs(float64(c.Position2.X)-float64(c.Position1.X))),
		float32(math.Abs(float64(c.Position2.Y)-float64(c.Position1.Y))),
	)
}

// Transform returns the local affine transform of this circle.
// If no transform has been set, it returns the identity matrix.
func (c *Circle) Transform() geo.Matrix {
	c.propMu.RLock()
	t := c.transform
	c.propMu.RUnlock()
	if t == (geo.Matrix{}) {
		return geo.Identity()
	}
	return t
}

// SetTransform sets the local affine transform for this circle.
// Use [geo.Identity] to clear the transform.
func (c *Circle) SetTransform(m geo.Matrix) {
	c.propMu.Lock()
	changed := c.transform != m
	c.transform = m
	c.propMu.Unlock()
	if changed {
		paintgen.Changed()
	}
}

// Visible returns true if this circle is visible, false otherwise
func (c *Circle) Visible() bool {
	c.propMu.RLock()
	h := c.Hidden
	c.propMu.RUnlock()
	return !h
}
