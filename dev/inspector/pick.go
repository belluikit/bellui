package inspector

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

// highlightFill and highlightStroke tint the selected object's bounds. Kept
// semi-transparent so the highlighted widget stays readable underneath.
var (
	highlightFill   = color.NRGBA{R: 0, G: 150, B: 255, A: 48}
	highlightStroke = color.NRGBA{R: 0, G: 150, B: 255, A: 220}
)

// newHighlight returns a stroke-only rectangle used as a bounds marker overlay.
func newHighlight() *canvas.Rectangle {
	r := canvas.NewRectangle(highlightFill)
	r.StrokeColor = highlightStroke
	r.StrokeWidth = 2
	r.Hide()
	return r
}

// resolveObjectAt returns the deepest visible object under pos in the target
// canvas's content, ignoring overlays (so the pick surface and highlight
// marker are never selected). ok is false when nothing is hit.
func resolveObjectAt(c core.Canvas, pos geo.Position) (core.CanvasObject, bool) {
	content := c.Content()
	if content == nil {
		return nil, false
	}
	obj, _, _ := driver.FindObjectAtPositionMatching(pos, matchAny, nil, content)
	return obj, obj != nil
}

func matchAny(core.CanvasObject) bool { return true }

// pickOverlay is a transparent, full-canvas surface added on top of the target
// window while pick mode is active. It captures pointer hover and taps and
// reports the canvas position so the inspector can resolve and select the
// object beneath the cursor.
type pickOverlay struct {
	widget.BaseWidget

	onHover  func(geo.Position)
	onPick   func(geo.Position)
	onCancel func()
}

func newPickOverlay(onHover, onPick func(geo.Position), onCancel func()) *pickOverlay {
	p := &pickOverlay{onHover: onHover, onPick: onPick, onCancel: onCancel}
	p.ExtendBaseWidget(p)
	return p
}

func (p *pickOverlay) CreateRenderer() core.WidgetRenderer {
	// A faint wash so the user can see pick mode is armed, without hiding the UI.
	bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 150, B: 255, A: 12})
	return widget.NewSimpleRenderer(bg)
}

func (p *pickOverlay) HoverIn(ev *event.HoverEvent)    { p.report(ev.AbsolutePosition) }
func (p *pickOverlay) HoverMoved(ev *event.HoverEvent) { p.report(ev.AbsolutePosition) }
func (p *pickOverlay) HoverOut()                       {}

func (p *pickOverlay) report(pos geo.Position) {
	if p.onHover != nil {
		p.onHover(pos)
	}
}

func (p *pickOverlay) Tapped(ev *event.PointEvent) {
	if p.onPick != nil {
		p.onPick(ev.AbsolutePosition)
	}
}

// Focusable: the overlay grabs keyboard focus while armed so Escape cancels
// pick mode without disturbing the target app's own key handler.
func (p *pickOverlay) FocusGained()   {}
func (p *pickOverlay) FocusLost()     {}
func (p *pickOverlay) TypedRune(rune) {}
func (p *pickOverlay) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeyEscape && p.onCancel != nil {
		p.onCancel()
	}
}

var (
	_ event.Hoverable = (*pickOverlay)(nil)
	_ event.Tappable  = (*pickOverlay)(nil)
	_ event.Focusable = (*pickOverlay)(nil)
	_ core.Widget     = (*pickOverlay)(nil)
)
