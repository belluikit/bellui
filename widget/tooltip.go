package widget

import (
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

const tooltipDelay = 500 * time.Millisecond

var (
	_ core.Widget     = (*Tooltip)(nil)
	_ event.Hoverable = (*Tooltip)(nil)
	_ event.Tappable  = (*Tooltip)(nil)
	_ core.Accessible = (*Tooltip)(nil)
)

// AccessibleRole returns [core.RoleTooltip].
func (t *Tooltip) AccessibleRole() core.AccessibleRole { return core.RoleTooltip }

// AccessibleLabel returns the tooltip text.
func (t *Tooltip) AccessibleLabel() string { return t.Text }

var _ core.ToolTipable = (*Tooltip)(nil)

// ToolTip returns the tooltip text, so the driver-level tooltip path (#800)
// shows this wrapper's text on the glfw desktop driver where the internal
// hover-catcher overlay never receives hover events.
func (t *Tooltip) ToolTip() string { return t.Text }

// newToolTipBubble builds the tooltip bubble content: a rounded background rect
// filled with the foreground colour and inverted (background-coloured) caption
// text, padded by the theme padding. Shared by the overlay-based [Tooltip] and
// the driver-level tooltip controller.
func newToolTipBubble(th bellui.Theme, v bellui.ThemeVariant, text string) core.CanvasObject {
	bg := canvas.NewRectangle(th.Color(theme.ColorNameForeground, v))
	bg.CornerRadius = th.Size(theme.SizeNameInputRadius)

	// RichText gives multiline support with an explicit inverted colour.
	fg := NewRichText(&TextSegment{
		Text: text,
		Style: RichTextStyle{
			ColorName: theme.ColorNameBackground,
			SizeName:  theme.SizeNameCaptionText,
		},
	})

	pad := th.Size(theme.SizeNamePadding)
	return &bellui.Container{
		Objects: []core.CanvasObject{bg, fg},
		Layout:  &tooltipContentLayout{padding: pad},
	}
}

// Tooltip wraps any CanvasObject and shows hover text after a short delay.
// It works by placing a transparent hover-intercepting overlay on top of the
// content, ensuring the tooltip receives hover events even when the content
// widget (e.g. Button) handles hover internally.
//
// Since: 2.7
type Tooltip struct {
	BaseWidget

	Content core.CanvasObject // The wrapped widget
	Text    string            // The tooltip text

	mu        sync.Mutex
	tip       *tooltipOverlay
	tipCanvas core.Canvas
	showTimer *time.Timer
	overlay   *hoverOverlay
}

// NewTooltip creates a new tooltip wrapper around the given content.
//
// Since: 2.7
func NewTooltip(content core.CanvasObject, text string) *Tooltip {
	t := &Tooltip{Content: content, Text: text}
	t.overlay = newHoverOverlay(t)
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer returns the renderer for this tooltip.
func (t *Tooltip) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	if t.overlay == nil {
		t.overlay = newHoverOverlay(t)
	}
	return &tooltipRenderer{tooltip: t}
}

// Tapped delegates to the wrapped content and hides any visible tooltip.
func (t *Tooltip) Tapped(e *event.PointEvent) {
	t.cancelTimer()
	t.hidePopUp()

	if tappable, ok := t.Content.(event.Tappable); ok {
		tappable.Tapped(e)
	}
}

// HoverIn is implemented for the Hoverable interface but hover is handled
// by the internal overlay. This is called only for non-widget content.
func (t *Tooltip) HoverIn(e *event.HoverEvent) { t.handleHoverIn(e) }

// HoverMoved is implemented for the [event.Hoverable] interface; pointer
// movement within the content does not affect the tooltip, so it is a no-op.
func (t *Tooltip) HoverMoved(e *event.HoverEvent) {}

// HoverOut is implemented for the [event.Hoverable] interface and cancels any
// pending tooltip and hides a visible one when the pointer leaves the content.
func (t *Tooltip) HoverOut() { t.handleMouseOut() }

// MinSize returns the wrapped content's minimum size.
func (t *Tooltip) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

func (t *Tooltip) handleHoverIn(e *event.HoverEvent) {
	t.startTimer()
}

func (t *Tooltip) handleMouseOut() {
	t.cancelTimer()
	t.hidePopUp()
}

func (t *Tooltip) startTimer() {
	t.cancelTimer()

	t.mu.Lock()
	t.showTimer = time.AfterFunc(tooltipDelay, func() {
		bellui.Do(func() {
			t.showPopUp()
		})
	})
	t.mu.Unlock()
}

func (t *Tooltip) cancelTimer() {
	t.mu.Lock()
	if t.showTimer != nil {
		t.showTimer.Stop()
		t.showTimer = nil
	}
	t.mu.Unlock()
}

func (t *Tooltip) showPopUp() {
	if t.Text == "" {
		return
	}
	c := bellui.CurrentApp().Driver().CanvasForObject(t)
	if c == nil {
		return
	}

	t.hidePopUp()

	th := t.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	content := newToolTipBubble(th, v, t.Text)

	t.mu.Lock()
	t.tip = newTooltipOverlay(content, t.tooltipPos())
	t.tipCanvas = c
	t.mu.Unlock()

	c.Overlays().Add(t.tip)
}

func (t *Tooltip) hidePopUp() {
	t.mu.Lock()
	tip, c := t.tip, t.tipCanvas
	t.tip = nil
	t.tipCanvas = nil
	t.mu.Unlock()

	if tip != nil && c != nil {
		c.Overlays().Remove(tip)
	}
}

// tooltipOverlay is the overlay root that hosts a tooltip's bubble. It
// is an input-pass-through overlay: it contains nothing tappable or
// hoverable, and it reports PassesInputThrough so the driver's
// hit-testing falls through to the UI beneath — a visible tooltip must
// never consume the click aimed at the widget under the pointer.
type tooltipOverlay struct {
	BaseWidget

	content core.CanvasObject
	pos     geo.Position
}

func newTooltipOverlay(content core.CanvasObject, pos geo.Position) *tooltipOverlay {
	o := &tooltipOverlay{content: content, pos: pos}
	o.ExtendBaseWidget(o)
	return o
}

// PassesInputThrough marks this overlay as input-transparent for the
// driver's hit-testing (internal/driver.InputPassThrough).
func (o *tooltipOverlay) PassesInputThrough() bool { return true }

// CreateRenderer places the bubble at its fixed position; the overlay
// root itself spans the canvas but draws nothing.
func (o *tooltipOverlay) CreateRenderer() core.WidgetRenderer {
	o.ExtendBaseWidget(o)
	return &tooltipOverlayRenderer{overlay: o}
}

type tooltipOverlayRenderer struct {
	overlay *tooltipOverlay
}

func (r *tooltipOverlayRenderer) Layout(geo.Size) {
	r.overlay.content.Resize(r.overlay.content.MinSize())
	r.overlay.content.Move(r.overlay.pos)
}

func (r *tooltipOverlayRenderer) MinSize() geo.Size { return geo.Size{} }

func (r *tooltipOverlayRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.overlay.content}
}

func (r *tooltipOverlayRenderer) Refresh() { canvas.Refresh(r.overlay) }
func (r *tooltipOverlayRenderer) Destroy() {}

func (t *Tooltip) tooltipPos() geo.Position {
	absPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(t)
	return absPos.Add(geo.NewPos(0, t.Size().Height))
}

// --- hoverOverlay: transparent widget that intercepts hover events ---
// Placed after Content in the renderer's Objects() so the tree walker
// finds it last (deepest match wins), beating any Hoverable inside Content.

type hoverOverlay struct {
	BaseWidget
	tooltip *Tooltip
}

var _ event.Hoverable = (*hoverOverlay)(nil)

func newHoverOverlay(t *Tooltip) *hoverOverlay {
	o := &hoverOverlay{tooltip: t}
	o.ExtendBaseWidget(o)
	return o
}

func (o *hoverOverlay) CreateRenderer() core.WidgetRenderer {
	o.ExtendBaseWidget(o)
	// Fully transparent — no visual
	return NewSimpleRenderer(canvas.NewRectangle(nil))
}

func (o *hoverOverlay) HoverIn(e *event.HoverEvent) {
	o.tooltip.handleHoverIn(e)
}

func (o *hoverOverlay) HoverMoved(_ *event.HoverEvent) {}

func (o *hoverOverlay) HoverOut() {
	o.tooltip.handleMouseOut()
}

func (o *hoverOverlay) Tapped(e *event.PointEvent) {
	// Forward tap to the content underneath
	if tappable, ok := o.tooltip.Content.(event.Tappable); ok {
		o.tooltip.cancelTimer()
		o.tooltip.hidePopUp()
		tappable.Tapped(e)
	}
}

// --- tooltipRenderer ---

type tooltipRenderer struct {
	tooltip *Tooltip
}

func (r *tooltipRenderer) Destroy() {
	r.tooltip.cancelTimer()
	r.tooltip.hidePopUp()
}

func (r *tooltipRenderer) Layout(size geo.Size) {
	if r.tooltip.Content != nil {
		r.tooltip.Content.Resize(size)
		r.tooltip.Content.Move(geo.NewPos(0, 0))
	}
	if r.tooltip.overlay != nil {
		r.tooltip.overlay.Resize(size)
		r.tooltip.overlay.Move(geo.NewPos(0, 0))
	}
}

func (r *tooltipRenderer) MinSize() geo.Size {
	if r.tooltip.Content != nil {
		return r.tooltip.Content.MinSize()
	}
	return geo.NewSize(0, 0)
}

func (r *tooltipRenderer) Objects() []core.CanvasObject {
	var objs []core.CanvasObject
	if r.tooltip.Content != nil {
		objs = append(objs, r.tooltip.Content)
	}
	if r.tooltip.overlay != nil {
		objs = append(objs, r.tooltip.overlay)
	}
	return objs
}

func (r *tooltipRenderer) SetObjects(objs []core.CanvasObject) {}

func (r *tooltipRenderer) Refresh() {
	if r.tooltip.Content != nil {
		r.tooltip.Content.Refresh()
	}
}

// --- tooltipContentLayout: background fills area, text is padded ---

type tooltipContentLayout struct {
	padding float32
}

func (l *tooltipContentLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	for i, o := range objects {
		if i == 0 && len(objects) > 1 {
			// First object is background — fills entire area.
			o.Move(geo.NewPos(0, 0))
			o.Resize(size)
			continue
		}
		// Text (or only) object is padded.
		o.Move(geo.NewPos(l.padding, l.padding))
		o.Resize(geo.NewSize(size.Width-l.padding*2, size.Height-l.padding*2))
	}
}

func (l *tooltipContentLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if len(objects) == 0 {
		return geo.NewSize(0, 0)
	}
	// The last object is always the text.
	textMin := objects[len(objects)-1].MinSize()
	return geo.NewSize(textMin.Width+l.padding*2, textMin.Height+l.padding*2)
}
