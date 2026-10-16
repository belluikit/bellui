package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

var (
	_ core.Widget     = (*OverlayContainer)(nil)
	_ event.Tappable  = (*OverlayContainer)(nil)
	_ event.Hoverable = (*OverlayContainer)(nil)
)

// OverlayContainer is a transparent widget containing one core.CanvasObject and meant to be used as overlay.
type OverlayContainer struct {
	Base
	Content core.CanvasObject

	// FocusTransparent, when set before Show, excludes this overlay from the
	// canvas focus routing: the focus manager of the layer beneath (usually
	// the window content) stays active, so the previously focused widget
	// keeps receiving key and shortcut events as if the overlay were not
	// there. Used by non-modal companion surfaces such as the completion
	// popup's KeepHostFocus mode, where the host widget drives the overlay.
	FocusTransparent bool

	// TapThrough, when set before Show, makes an outside tap dismiss the
	// overlay without consuming the press: the desktop driver dismisses the
	// overlay at press time and lets the same click re-resolve against the
	// layer beneath, so e.g. the click that closes a completion popup also
	// places the caret in the editor under it. A tap inside Content is
	// unaffected. Without it the outside tap is swallowed by Tapped.
	TapThrough bool

	canvas    core.Canvas
	onDismiss func()
	shown     bool
}

// NewOverlayContainer creates an OverlayContainer.
func NewOverlayContainer(c core.CanvasObject, canvas core.Canvas, onDismiss func()) *OverlayContainer {
	o := &OverlayContainer{canvas: canvas, Content: c, onDismiss: onDismiss}
	o.ExtendBaseWidget(o)
	return o
}

// CreateRenderer returns a new renderer for the overlay container.
func (o *OverlayContainer) CreateRenderer() core.WidgetRenderer {
	return &overlayRenderer{BaseRenderer{[]core.CanvasObject{o.Content}}, o}
}

// Hide hides the overlay container.
func (o *OverlayContainer) Hide() {
	if o.shown {
		o.canvas.Overlays().Remove(o)
		o.shown = false
	}
	o.Base.Hide()
}

// HoverIn catches hover-in events not handled by the container’s content. It does nothing.
func (o *OverlayContainer) HoverIn(*event.HoverEvent) {
}

// HoverMoved catches hover-moved events not handled by the container’s content. It does nothing.
func (o *OverlayContainer) HoverMoved(*event.HoverEvent) {
}

// HoverOut catches hover-out events not handled by the container’s content. It does nothing.
func (o *OverlayContainer) HoverOut() {
}

// Show makes the overlay container visible.
func (o *OverlayContainer) Show() {
	if !o.shown {
		o.canvas.Overlays().Add(o)
		o.shown = true
	}
	o.Base.Show()
}

// Tapped catches tap events not handled by the container’s content.
// It performs the overlay container’s dismiss action.
func (o *OverlayContainer) Tapped(*event.PointEvent) {
	o.Dismiss()
}

// Dismiss performs the overlay container’s dismiss action, if any. Drivers
// call it directly for TapThrough overlays, where the dismissing press never
// reaches Tapped because it is re-delivered to the layer beneath.
func (o *OverlayContainer) Dismiss() {
	if o.onDismiss != nil {
		o.onDismiss()
	}
}

// TapThroughDismiss implements the drivers' press-time pass-through seam
// (pre-rebrand tracker #472): when the overlay opted in via TapThrough and the press lands
// outside Content, the overlay is dismissed and true is returned, telling
// the driver to resolve the same press against the layer beneath. Drivers
// assert for this method structurally on the top overlay before dispatching
// a press.
func (o *OverlayContainer) TapThroughDismiss(pos geo.Position) bool {
	if !o.TapThrough || o.Content == nil {
		return false
	}
	cp, cs := o.Content.Position(), o.Content.Size()
	if pos.X >= cp.X && pos.X < cp.X+cs.Width && pos.Y >= cp.Y && pos.Y < cp.Y+cs.Height {
		return false
	}
	o.Dismiss()
	return true
}

type overlayRenderer struct {
	BaseRenderer
	o *OverlayContainer
}

var _ core.WidgetRenderer = (*overlayRenderer)(nil)

func (r *overlayRenderer) Layout(geo.Size) {
}

func (r *overlayRenderer) MinSize() geo.Size {
	return r.o.canvas.Size()
}

func (r *overlayRenderer) Refresh() {
}
