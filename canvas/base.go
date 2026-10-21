// Package canvas contains all of the primitive CanvasObjects that make up a Bellui GUI.
//
// The types implemented in this package are used as building blocks in order
// to build higher order functionality. These types are designed to be
// non-interactive, by design. If additional functionality is required,
// it's usually a sign that this type should be used as part of a custom
// widget.
package canvas // import "github.com/belluikit/bellui/canvas"

import (
	"sync"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/layoutgen"
	"github.com/belluikit/bellui/internal/paintgen"
)

type baseObject struct {
	size        geo.Size     // The current size of the canvas object
	position    geo.Position // The current position of the object
	Hidden      bool         // Is this object currently hidden
	min         geo.Size     // The minimum size this object can be
	propMu      sync.RWMutex // protects size, position, Hidden, min
	transform   geo.Matrix   // Local affine transform (zero value = identity)
	transformMu sync.RWMutex
}

// Hide will set this object to not be visible.
func (o *baseObject) Hide() {
	o.propMu.Lock()
	changed := !o.Hidden
	o.Hidden = true
	o.propMu.Unlock()
	if changed {
		layoutgen.Changed()
	}
}

// MinSize returns the specified minimum size, if set, or {1, 1} otherwise.
func (o *baseObject) MinSize() geo.Size {
	o.propMu.RLock()
	min := o.min
	o.propMu.RUnlock()
	if min.IsZero() {
		return geo.Size{Width: 1, Height: 1}
	}
	return min
}

// Move the object to a new position, relative to its parent.
func (o *baseObject) Move(pos geo.Position) {
	o.propMu.Lock()
	o.position = pos
	o.propMu.Unlock()
}

// Position gets the current position of this canvas object, relative to its parent.
func (o *baseObject) Position() geo.Position {
	o.propMu.RLock()
	pos := o.position
	o.propMu.RUnlock()
	return pos
}

// Resize sets a new size for the canvas object.
func (o *baseObject) Resize(size geo.Size) {
	o.propMu.Lock()
	o.size = size
	o.propMu.Unlock()
}

// SetMinSize specifies the smallest size this object should be.
func (o *baseObject) SetMinSize(size geo.Size) {
	o.propMu.Lock()
	changed := o.min != size
	o.min = size
	o.propMu.Unlock()
	if changed {
		layoutgen.Changed()
	}
}

// Show will set this object to be visible.
func (o *baseObject) Show() {
	// Renderers call Show on every layout; only a change counts (#487).
	o.propMu.Lock()
	changed := o.Hidden
	o.Hidden = false
	o.propMu.Unlock()
	if changed {
		layoutgen.Changed()
		paintgen.Changed()
	}
}

// Size returns the current size of this canvas object.
func (o *baseObject) Size() geo.Size {
	o.propMu.RLock()
	s := o.size
	o.propMu.RUnlock()
	return s
}

// Visible returns true if this object is visible, false otherwise.
func (o *baseObject) Visible() bool {
	o.propMu.RLock()
	h := o.Hidden
	o.propMu.RUnlock()
	return !h
}

// Transform returns the local affine transform of this object.
// If no transform has been set, it returns the identity matrix.
// Transform is safe for concurrent use.
func (o *baseObject) Transform() geo.Matrix {
	o.transformMu.RLock()
	t := o.transform
	o.transformMu.RUnlock()
	if t == (geo.Matrix{}) {
		return geo.Identity()
	}
	return t
}

// SetTransform sets the local affine transform for this object.
// Use [geo.Identity] to clear the transform.
// SetTransform is safe for concurrent use.
func (o *baseObject) SetTransform(m geo.Matrix) {
	o.transformMu.Lock()
	changed := !sameTransform(o.transform, m)
	o.transform = m
	o.transformMu.Unlock()
	if changed {
		paintgen.Changed()
	}
}

// sameTransform reports whether a and b transform alike: the zero Matrix is
// the identity, as Transform returns it, so setting the identity on an object
// that never had a transform changes nothing.
func sameTransform(a, b geo.Matrix) bool {
	if a == (geo.Matrix{}) {
		a = geo.Identity()
	}
	if b == (geo.Matrix{}) {
		b = geo.Identity()
	}
	return a == b
}
