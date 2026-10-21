package canvas

import (
	"image/color"

	"github.com/belluikit/bellui/geo"
)

// Ripple is a canvas object that renders a radial ripple effect from a center
// point, clipped to a rounded rectangle shape. It is used for MD3-style touch
// feedback on interactive widgets.
type Ripple struct {
	baseObject

	// Center is the ripple origin relative to the widget's top-left.
	Center geo.Position

	// Radius is the current ripple radius in logical pixels (animated).
	Radius float32

	// Color is the ripple overlay color (typically the widget's content color).
	Color color.Color

	// Opacity is the current ripple opacity (0.0–1.0, animated).
	Opacity float32

	// CornerRadius is the corner radius for SDF clipping to the parent shape.
	CornerRadius float32
}

// NewRipple creates a new ripple canvas object.
func NewRipple(c color.Color) *Ripple {
	return &Ripple{Color: c}
}

// Refresh causes this ripple to be redrawn.
func (r *Ripple) Refresh() {
	Refresh(r)
}

// Hide will set this ripple to not be visible.
func (r *Ripple) Hide() {
	r.baseObject.Hide()

	repaint(r)
}

// Move the ripple to a new position, relative to its parent.
func (r *Ripple) Move(pos geo.Position) {
	if r.Position() == pos {
		return
	}

	r.baseObject.Move(pos)

	repaint(r)
}

// Resize sets a new size for the ripple.
func (r *Ripple) Resize(s geo.Size) {
	if s == r.Size() {
		return
	}

	r.baseObject.Resize(s)

	repaint(r)
}
