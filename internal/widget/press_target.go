package widget

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// PressShape is where a widget's pressed state shows, and in what colour.
type PressShape struct {
	// Pos and Size place the state layer in the widget's coordinates: the
	// whole widget for a row, the indicator pill of a navigation item, the
	// circle around a checkbox.
	Pos  geo.Position
	Size geo.Size
	// CornerRadius rounds the state layer; half the size makes a circle.
	CornerRadius float32
	// Content is the colour of the widget's text or icon. The state layer is
	// that colour at the theme's pressed opacity.
	Content color.Color
	// FromCenter spreads the ripple from the shape's centre rather than from
	// the press, for a control whose target is larger than its state layer:
	// a press on a checkbox's label still shows around the box.
	FromCenter bool
}

// PressTarget gives a widget the MD3 pressed state of PressFeedback, with the
// press, release and cancel events it needs (#1091). A widget embeds it,
// which makes it a mobile.Touchable and a desktop.Mouseable, and calls
// AttachPress from CreateRenderer and TapPress from Tapped. A widget that is
// Hoverable calls CancelPress from HoverOut: a mouse released elsewhere sends
// it no MouseUp.
//
// The helpers are functions rather than methods so that a public widget that
// embeds it gains only the event methods.
type PressTarget struct {
	feedback *PressFeedback
	shape    func() (PressShape, bool)
}

// AttachPress starts the feedback of t for a new renderer and returns the
// ripple it draws on, for the renderer to place above the background and
// below the content. shape says where the state layer shows; it returns false
// while the widget takes no press, such as when it is disabled.
func AttachPress(t *PressTarget, shape func() (PressShape, bool)) *canvas.Ripple {
	CancelPress(t)
	t.feedback = NewPressFeedback()
	t.shape = shape
	return t.feedback.Ripple
}

// TapPress shows a press and its release for a tap that had no press before
// it: a key, an accessibility action, or a tap from code. A pointer tap's
// press showed already, and this does nothing for it. ev is nil for a tap
// with no position, which shows from the shape's centre.
func TapPress(t *PressTarget, ev *event.PointEvent) {
	s, ok := t.pressShape()
	if !ok {
		return
	}
	at := geo.NewPos(s.Size.Width/2, s.Size.Height/2)
	if ev != nil && !s.FromCenter {
		at = ev.Position.Subtract(s.Pos)
	}
	t.place(s)
	t.feedback.Tap(at, s.Size, s.CornerRadius, s.Content)
}

// PressFeedbackOf returns the feedback of t, or nil before its renderer
// exists. Tests read the pressed state from it.
func PressFeedbackOf(t *PressTarget) *PressFeedback { return t.feedback }

// TouchDown shows the pressed state from where the finger went down.
func (t *PressTarget) TouchDown(ev *mobile.TouchEvent) { t.pressAt(ev.Position) }

// TouchUp ends the pressed state.
func (t *PressTarget) TouchUp(*mobile.TouchEvent) { t.releasePress() }

// TouchCancel ends the pressed state with no feedback left: the finger left
// the widget, or the touch became a scroll or a drag of something else.
func (t *PressTarget) TouchCancel(*mobile.TouchEvent) { CancelPress(t) }

// MouseDown shows the pressed state from where the primary button went down.
func (t *PressTarget) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		t.pressAt(ev.Position)
	}
}

// MouseUp ends the pressed state.
func (t *PressTarget) MouseUp(*desktop.MouseEvent) { t.releasePress() }

// CancelPress ends the pressed state of t with no feedback left. A widget
// calls it from HoverOut, and wherever it stops being a target mid-press.
func CancelPress(t *PressTarget) {
	if t.feedback != nil {
		t.feedback.Cancel()
	}
}

func (t *PressTarget) pressAt(at geo.Position) {
	s, ok := t.pressShape()
	if !ok {
		return
	}
	at = at.Subtract(s.Pos)
	if s.FromCenter {
		at = geo.NewPos(s.Size.Width/2, s.Size.Height/2)
	}
	t.place(s)
	t.feedback.Press(at, s.Size, s.CornerRadius, s.Content)
}

func (t *PressTarget) releasePress() {
	if t.feedback != nil {
		t.feedback.Release()
	}
}

func (t *PressTarget) pressShape() (PressShape, bool) {
	if t.feedback == nil {
		return PressShape{}, false
	}
	return t.shape()
}

func (t *PressTarget) place(s PressShape) {
	t.feedback.Ripple.Move(s.Pos)
}
