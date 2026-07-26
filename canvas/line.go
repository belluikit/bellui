package canvas

import (
	"image/color"
	"math"
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Line)(nil)

// Line describes a colored line primitive in a Bellui canvas.
// Lines are special as they can have a negative width or height to indicate
// an inverse slope (i.e. slope up vs down).
type Line struct {
	Position1 geo.Position // The current top-left position of the Line
	Position2 geo.Position // The current bottom-right position of the Line
	Hidden    bool         // Is this Line currently hidden

	StrokeColor color.Color // The line stroke color
	StrokeWidth float32     // The stroke width of the line

	propMu    sync.RWMutex
	transform geo.Matrix
}

// Size returns the current size of bounding box for this line object
func (l *Line) Size() geo.Size {
	l.propMu.RLock()
	s := l.sizeLocked()
	l.propMu.RUnlock()
	return s
}

func (l *Line) sizeLocked() geo.Size {
	return geo.NewSize(
		float32(math.Abs(float64(l.Position2.X)-float64(l.Position1.X))),
		float32(math.Abs(float64(l.Position2.Y)-float64(l.Position1.Y))),
	)
}

// Resize sets a new bottom-right position for the line object, then it will then be refreshed.
func (l *Line) Resize(size geo.Size) {
	l.propMu.Lock()
	if size == l.sizeLocked() {
		l.propMu.Unlock()
		return
	}

	if l.Position1.X <= l.Position2.X {
		l.Position2.X = l.Position1.X + size.Width
	} else {
		l.Position1.X = l.Position2.X + size.Width
	}
	if l.Position1.Y <= l.Position2.Y {
		l.Position2.Y = l.Position1.Y + size.Height
	} else {
		l.Position1.Y = l.Position2.Y + size.Height
	}
	l.propMu.Unlock()
	Refresh(l)
}

// Position gets the current top-left position of this line object, relative to its parent / canvas
func (l *Line) Position() geo.Position {
	l.propMu.RLock()
	pos := l.positionLocked()
	l.propMu.RUnlock()
	return pos
}

func (l *Line) positionLocked() geo.Position {
	return geo.NewPos(geo.Min(l.Position1.X, l.Position2.X), geo.Min(l.Position1.Y, l.Position2.Y))
}

// Move the line object to a new position, relative to its parent / canvas
func (l *Line) Move(pos geo.Position) {
	l.propMu.Lock()
	oldPos := l.positionLocked()
	if oldPos == pos {
		l.propMu.Unlock()
		return
	}

	deltaX := pos.X - oldPos.X
	deltaY := pos.Y - oldPos.Y

	l.Position1 = l.Position1.AddXY(deltaX, deltaY)
	l.Position2 = l.Position2.AddXY(deltaX, deltaY)
	l.propMu.Unlock()
	repaint(l)
}

// MinSize for a Line simply returns Size{1, 1} as there is no
// explicit content
func (l *Line) MinSize() geo.Size {
	return geo.NewSize(1, 1)
}

// Transform returns the local affine transform of this line.
// If no transform has been set, it returns the identity matrix.
func (l *Line) Transform() geo.Matrix {
	l.propMu.RLock()
	t := l.transform
	l.propMu.RUnlock()
	if t == (geo.Matrix{}) {
		return geo.Identity()
	}
	return t
}

// SetTransform sets the local affine transform for this line.
// Use [geo.Identity] to clear the transform.
func (l *Line) SetTransform(m geo.Matrix) {
	l.propMu.Lock()
	l.transform = m
	l.propMu.Unlock()
}

// Visible returns true if this line is visible, false otherwise
func (l *Line) Visible() bool {
	l.propMu.RLock()
	h := l.Hidden
	l.propMu.RUnlock()
	return !h
}

// Show will set this line to be visible
func (l *Line) Show() {
	l.propMu.Lock()
	l.Hidden = false
	l.propMu.Unlock()

	l.Refresh()
}

// Hide will set this line to not be visible
func (l *Line) Hide() {
	l.propMu.Lock()
	l.Hidden = true
	l.propMu.Unlock()

	repaint(l)
}

// Refresh causes this line to be redrawn with its configured state.
func (l *Line) Refresh() {
	Refresh(l)
}

// NewLine returns a new Line instance
func NewLine(color color.Color) *Line {
	return &Line{
		StrokeColor: color,
		StrokeWidth: 1,
	}
}
