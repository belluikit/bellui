package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/dirhook"
	"github.com/belluikit/bellui/internal/paintgen"
)

// Base provides a helper that handles basic widget behaviours.
type Base struct {
	hidden   bool
	position geo.Position
	size     geo.Size
	impl     core.Widget
}

// ExtendBaseWidget is used by an extending widget to make use of BaseWidget functionality.
func (w *Base) ExtendBaseWidget(wid core.Widget) {
	impl := w.super()
	if impl != nil {
		return
	}

	w.impl = wid
}

// Size gets the current size of this widget.
func (w *Base) Size() geo.Size {
	return w.size
}

// Resize sets a new size for a widget.
// Note this should not be used if the widget is being managed by a Layout within a Container.
func (w *Base) Resize(size geo.Size) {
	if size == w.Size() {
		return
	}

	w.size = size

	impl := w.super()
	if impl == nil {
		return
	}
	r := cache.Renderer(impl)
	if dirhook.InUse() {
		dirhook.Run(impl, func() { r.Layout(size) }) // in impl's direction (#749)
	} else {
		r.Layout(size)
	}
	Repaint(impl)
}

// Position gets the current position of this widget, relative to its parent.
func (w *Base) Position() geo.Position {
	return w.position
}

// Move the widget to a new position, relative to its parent.
// Note this should not be used if the widget is being managed by a Layout within a Container.
func (w *Base) Move(pos geo.Position) {
	if w.Position() == pos {
		return
	}

	w.position = pos

	Repaint(w.super())
}

// MinSize for the widget - it should never be resized below this value.
func (w *Base) MinSize() geo.Size {
	impl := w.super()

	r := cache.Renderer(impl)
	if r == nil {
		return geo.NewSize(0, 0)
	}

	return r.MinSize()
}

// Visible returns whether or not this widget should be visible.
// Note that this may not mean it is currently visible if a parent has been hidden.
func (w *Base) Visible() bool {
	return !w.hidden
}

// Show this widget so it becomes visible
func (w *Base) Show() {
	if !w.hidden {
		return // Visible already
	}

	w.hidden = false
	paintgen.Changed() // a hidden widget has no node to refresh (#599)

	impl := w.super()
	if impl == nil {
		return
	}
	impl.Refresh()
}

// Hide this widget so it is no longer visible
func (w *Base) Hide() {
	if w.hidden {
		return // Hidden already
	}

	w.hidden = true

	impl := w.super()
	if impl == nil {
		return
	}
	canvas.Refresh(impl)
}

// Refresh causes this widget to be redrawn in it's current state
func (w *Base) Refresh() {
	impl := w.super()
	if impl == nil {
		return
	}

	r := cache.Renderer(impl)
	if dirhook.InUse() {
		dirhook.Run(impl, r.Refresh) // in impl's direction (#749)
		return
	}
	r.Refresh()
}

// super will return the actual object that this represents.
// If extended then this is the extending widget, otherwise it is nil.
func (w *Base) super() core.Widget {
	return w.impl
}

// Repaint instructs the containing canvas to redraw, even if nothing changed.
// Callers have moved or resized obj, and the canvas's partial repaint learns
// where from this alone (#599).
// This method is a duplicate of what is in `canvas/canvas.go` to avoid a dependency loop or public API.
func Repaint(obj core.CanvasObject) {
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}

	c := app.Driver().CanvasForObject(obj)
	if c != nil {
		if m, ok := c.(interface{ MarkPlacementDirty(core.CanvasObject) }); ok {
			m.MarkPlacementDirty(obj)
		}
		if paint, ok := c.(interface{ SetDirty() }); ok {
			paint.SetDirty()
		}
	}
}
