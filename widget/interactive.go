package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// Interactive wraps any CanvasObject and adds event handling via callbacks.
// This bridges the gap between canvas primitives (which can only draw) and
// widgets (which can handle events), without the full CreateRenderer ceremony.
//
// Usage:
//
//	rect := canvas.NewRectangle(color.NRGBA{R: 200, A: 255})
//	btn := widget.NewInteractive(rect)
//	btn.OnTapped = func(*event.PointEvent) {
//	    rect.FillColor = color.NRGBA{G: 200, A: 255}
//	    canvas.Refresh(rect)
//	}
//
// Interactive implements Tappable, DoubleTappable, SecondaryTappable,
// Draggable, Scrollable, event.Hoverable, and desktop.Cursorable
// — but only dispatches events when the corresponding callback is set.
type Interactive struct {
	BaseWidget

	// Content is the wrapped CanvasObject.
	Content core.CanvasObject

	// Pointer events.
	OnTapped          func(*event.PointEvent)
	OnDoubleTapped    func(*event.PointEvent)
	OnTappedSecondary func(*event.PointEvent)

	// Drag events.
	OnDragged func(*event.DragEvent)
	OnDragEnd func()

	// Scroll events.
	OnScrolled func(*event.ScrollEvent)

	// Hover events.
	OnHoverIn    func(*event.HoverEvent)
	OnHoverMoved func(*event.HoverEvent)
	OnHoverOut   func()

	// CursorStyle override (desktop only). Nil means default cursor.
	CursorStyle desktop.Cursor
}

// NewInteractive wraps a CanvasObject to make it interactive.
func NewInteractive(content core.CanvasObject) *Interactive {
	w := &Interactive{Content: content}
	w.ExtendBaseWidget(w)
	return w
}

// CreateRenderer returns the renderer for this interactive wrapper.
func (i *Interactive) CreateRenderer() core.WidgetRenderer {
	return NewRenderer(i.Content).Build()
}

// --- Tappable ---

func (i *Interactive) Tapped(ev *event.PointEvent) {
	if i.OnTapped != nil {
		i.OnTapped(ev)
	}
}

// --- DoubleTappable ---

func (i *Interactive) DoubleTapped(ev *event.PointEvent) {
	if i.OnDoubleTapped != nil {
		i.OnDoubleTapped(ev)
	}
}

// --- SecondaryTappable ---

func (i *Interactive) TappedSecondary(ev *event.PointEvent) {
	if i.OnTappedSecondary != nil {
		i.OnTappedSecondary(ev)
	}
}

// --- Draggable ---

func (i *Interactive) Dragged(ev *event.DragEvent) {
	if i.OnDragged != nil {
		i.OnDragged(ev)
	}
}

func (i *Interactive) DragEnd() {
	if i.OnDragEnd != nil {
		i.OnDragEnd()
	}
}

// --- Scrollable ---

func (i *Interactive) Scrolled(ev *event.ScrollEvent) {
	if i.OnScrolled != nil {
		i.OnScrolled(ev)
	}
}

// --- event.Hoverable ---

func (i *Interactive) HoverIn(ev *event.HoverEvent) {
	if i.OnHoverIn != nil {
		i.OnHoverIn(ev)
	}
}

func (i *Interactive) HoverMoved(ev *event.HoverEvent) {
	if i.OnHoverMoved != nil {
		i.OnHoverMoved(ev)
	}
}

func (i *Interactive) HoverOut() {
	if i.OnHoverOut != nil {
		i.OnHoverOut()
	}
}

// --- desktop.Cursorable ---

func (i *Interactive) Cursor() desktop.Cursor {
	if i.CursorStyle != nil {
		return i.CursorStyle
	}
	return desktop.DefaultCursor
}

// Compile-time interface checks.
var (
	_ event.Tappable          = (*Interactive)(nil)
	_ event.DoubleTappable    = (*Interactive)(nil)
	_ event.SecondaryTappable = (*Interactive)(nil)
	_ event.Draggable         = (*Interactive)(nil)
	_ event.Scrollable        = (*Interactive)(nil)
	_ event.Hoverable         = (*Interactive)(nil)
	_ desktop.Cursorable      = (*Interactive)(nil)
)
