package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	col "github.com/belluikit/bellui/internal/color"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var _ core.Widget = (*PopUp)(nil)

// PopUp is a widget that can float above the user interface.
// It wraps any standard elements with padding and a shadow.
// If it is modal then the shadow will cover the entire canvas it hovers over and block interactions.
type PopUp struct {
	BaseWidget

	Content core.CanvasObject
	Canvas  core.Canvas

	// OnDismissed, when set, replaces hiding as the response to the user
	// dismissing this pop-up: pressing Escape (see DismissOnEscape), tapping or
	// secondary-tapping outside the content (when the pop-up dismisses on tap),
	// or invoking the accessibility dismiss action. A host that has its own
	// close path — a dialog with a result to report — sets it so that path runs
	// whatever the user dismissed with; it is then responsible for hiding the
	// pop-up. Calling Hide from code does not run it.
	//
	// Since: 1.0
	OnDismissed func() `json:"-"`

	innerPos          geo.Position
	innerPosMoved     bool // true if Move was called explicitly
	innerSize         geo.Size
	modal             bool
	modalDismissOnTap bool // when true, modal popups dismiss on outside tap (for drawers)
	passThrough       bool // when true, input outside the content passes to the UI beneath (pre-rebrand tracker #569)
	overlayShown      bool
	bgColor           color.Color // custom background color; nil = use overlay background
	noPadding         bool        // when true, skip inner padding (used for tooltips)
	noDismissOnTap    bool        // when true, outside taps don't dismiss (for snackbars)
	isHiding          bool        // true during hide animation — absorb taps, don't act
	showAnim          *bellui.Animation
	hideAnim          *bellui.Animation
	opacity           float32 // 0.0–1.0 for animation; 1.0 when fully visible
}

// Hide this widget, if it was previously visible
func (p *PopUp) Hide() {
	if p.showAnim != nil {
		p.showAnim.Stop()
	}

	p.isHiding = true

	if p.overlayShown && bellui.CurrentApp().Settings().ShowAnimations() {
		if p.hideAnim != nil {
			p.hideAnim.Stop()
		}
		p.hideAnim = bellui.NewAnimation(canvas.DurationShort2, func(done float32) {
			p.opacity = 1 - done
			p.Refresh()
			if done >= 1.0 {
				p.finishHide()
			}
		})
		p.hideAnim.Curve = bellui.AnimationEaseStandardAccelerate
		p.hideAnim.Start()
		return
	}

	p.finishHide()
}

// PassesInputThrough marks pass-through pop-ups (transient surfaces like
// Toast and Snackbar, pre-rebrand tracker #569) as input-transparent for the driver hit tests
// and the canvas focus routing: taps, scrolls and keyboard input outside
// the content reach the UI beneath as if the pop-up were not there, per the
// MD3 rule that transient notifications never block interaction. Taps on
// the content itself (a Snackbar's action button) still work.
func (p *PopUp) PassesInputThrough() bool {
	return p.passThrough
}

func (p *PopUp) finishHide() {
	if p.overlayShown {
		p.Canvas.Overlays().Remove(p)
		p.overlayShown = false
	}
	p.isHiding = false
	p.opacity = 0
	p.BaseWidget.Hide()
}

// Move the widget to a new position. A PopUp position is absolute to the top, left of its canvas.
// For PopUp this actually moves the content so checking Position() will not return the same value as is set here.
func (p *PopUp) Move(pos geo.Position) {
	p.innerPos = pos
	p.innerPosMoved = true
	if p.overlayShown {
		p.Refresh()
	}
}

// Resize changes the size of the PopUp's content.
// PopUps always have the size of their canvas, but this call updates the
// size of the content portion.
func (p *PopUp) Resize(size geo.Size) {
	p.innerSize = size
	// The canvas size might not have changed and therefore the Resize won't trigger a layout.
	// Until we have a widget.Relayout() or similar, the renderer's refresh will do the re-layout.
	p.Refresh()
}

// Show this pop-up as overlay if not already shown.
func (p *PopUp) Show() {
	if p.hideAnim != nil {
		p.hideAnim.Stop()
	}

	if !p.overlayShown {
		p.Canvas.Overlays().Add(p)
		p.overlayShown = true
	}

	if bellui.CurrentApp().Settings().ShowAnimations() {
		p.opacity = 0
		p.BaseWidget.Show()
		p.Refresh()
		if p.showAnim != nil {
			p.showAnim.Stop()
		}
		p.showAnim = bellui.NewAnimation(canvas.DurationShort3, func(done float32) {
			p.opacity = done
			p.Refresh()
		})
		p.showAnim.Curve = bellui.AnimationEaseStandardDecelerate
		p.showAnim.Start()
	} else {
		p.opacity = 1
		p.Refresh()
		p.BaseWidget.Show()
	}
}

// ShowAtPosition shows this pop-up at the given position.
func (p *PopUp) ShowAtPosition(pos geo.Position) {
	p.Move(pos)
	p.Show()
}

// ShowAtRelativePosition shows this pop-up at the given position relative to stated object.
//
// Since: 1.0
func (p *PopUp) ShowAtRelativePosition(rel geo.Position, to core.CanvasObject) {
	withRelativePosition(rel, to, p.ShowAtPosition)
}

// AccessibleRole returns [core.RoleDialog] for modal pop-ups and
// [core.RoleGroup] otherwise. Dialogs, sheets and drawers are all built on
// modal pop-ups, so this gives each of them a dialog container node; the
// pop-up's content provides its own semantics as child nodes.
func (p *PopUp) AccessibleRole() core.AccessibleRole {
	if p.modal {
		return core.RoleDialog
	}
	return core.RoleGroup
}

// AccessibleLabel returns an empty string; the content names itself.
func (p *PopUp) AccessibleLabel() string { return "" }

// AccessibleState reports the modal state.
func (p *PopUp) AccessibleState() core.AccessibleState {
	if p.modal {
		return core.StateModal
	}
	return 0
}

// AccessibleActions returns the "dismiss" action for pop-ups that dismiss
// on an outside tap.
func (p *PopUp) AccessibleActions() []core.AccessibleAction {
	if p.noDismissOnTap || (p.modal && !p.modalDismissOnTap) {
		return nil
	}
	return []core.AccessibleAction{{Name: core.ActionDismiss, Description: "Dismiss the pop-up"}}
}

// PerformAccessibleAction dismisses the pop-up, mirroring an outside tap.
func (p *PopUp) PerformAccessibleAction(name string) {
	if name != core.ActionDismiss || p.isHiding {
		return
	}
	if p.noDismissOnTap || (p.modal && !p.modalDismissOnTap) {
		return
	}
	p.dismiss()
}

// dismiss responds to a user-initiated dismissal: the host's OnDismissed when
// set, otherwise a plain Hide.
func (p *PopUp) dismiss() {
	if p.OnDismissed != nil {
		p.OnDismissed()
		return
	}
	p.Hide()
}

// DismissOnEscape closes the pop-up because the user pressed Escape, reporting
// whether it did.
//
// Unlike an outside tap this is not blocked by being modal — Escape is how a
// user cancels a dialog, and every canvas dialog is a modal pop-up. What does
// block it is a pop-up that manages its own dismissal (a snackbar) or one that
// is not an input surface at all (a toast, a tooltip): those keep counting down
// and leave the key to whatever is beneath them.
//
// Since: 1.0
func (p *PopUp) DismissOnEscape() bool {
	if p.isHiding || p.noDismissOnTap || p.passThrough || !p.Visible() {
		return false
	}

	// A host with OnDismissed owns the close path — a dialog has a result to
	// report.
	p.dismiss()
	return true
}

// Tapped is called when the user taps the popUp.
// If not modal and the tap is outside the content area, then dismiss this widget
func (p *PopUp) Tapped(e *event.PointEvent) {
	if p.isHiding {
		return // absorb tap during hide animation, don't pass through
	}
	if p.noDismissOnTap || p.isInsideContent(e.Position) {
		return
	}
	if !p.modal || p.modalDismissOnTap {
		p.dismiss()
	}
}

// TappedSecondary is called when the user right/alt taps the popUp.
// If not modal and the tap is outside the content area, then dismiss this widget
func (p *PopUp) TappedSecondary(e *event.PointEvent) {
	if p.isHiding {
		return
	}
	if p.noDismissOnTap || p.isInsideContent(e.Position) {
		return
	}
	if !p.modal || p.modalDismissOnTap {
		p.dismiss()
	}
}

// MinSize returns the size that this widget should not shrink below
func (p *PopUp) MinSize() geo.Size {
	p.ExtendBaseWidget(p)
	return p.BaseWidget.MinSize()
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (p *PopUp) CreateRenderer() core.WidgetRenderer {
	th := p.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	p.ExtendBaseWidget(p)
	bgCol := th.Color(theme.ColorNameSurfaceContainerLow, v)
	if p.bgColor != nil {
		bgCol = p.bgColor
	}
	background := canvas.NewRectangle(bgCol)
	background.CornerRadius = th.Size(theme.SizeNameShapeLarge)
	if p.modal {
		underlay := canvas.NewRectangle(th.Color(theme.ColorNameScrim, v))
		objects := []core.CanvasObject{underlay, background, p.Content}
		return &modalPopUpRenderer{
			widget.NewShadowingRenderer(objects, widget.DialogLevel),
			popUpBaseRenderer{popUp: p, background: background},
			underlay,
		}
	}
	objects := []core.CanvasObject{background, p.Content}
	return &popUpRenderer{
		widget.NewShadowingRenderer(objects, widget.PopUpLevel),
		popUpBaseRenderer{popUp: p, background: background},
	}
}

func (p *PopUp) isInsideContent(pos geo.Position) bool {
	return pos.X >= p.innerPos.X && pos.Y >= p.innerPos.Y &&
		pos.X <= p.innerPos.X+p.innerSize.Width &&
		pos.Y <= p.innerPos.Y+p.innerSize.Height
}

// ShowPopUpAtPosition creates a new popUp for the specified content at the specified absolute position.
// It will then display the popup on the passed canvas.
func ShowPopUpAtPosition(content core.CanvasObject, canvas core.Canvas, pos geo.Position) {
	newPopUp(content, canvas).ShowAtPosition(pos)
}

// ShowPopUpAtRelativePosition shows a new popUp for the specified content at the given position relative to stated object.
// It will then display the popup on the passed canvas.
//
// Since: 1.0
func ShowPopUpAtRelativePosition(content core.CanvasObject, canvas core.Canvas, rel geo.Position, to core.CanvasObject) {
	withRelativePosition(rel, to, func(pos geo.Position) {
		ShowPopUpAtPosition(content, canvas, pos)
	})
}

func newPopUp(content core.CanvasObject, canvas core.Canvas) *PopUp {
	ret := &PopUp{Content: content, Canvas: canvas, modal: false}
	ret.ExtendBaseWidget(ret)
	return ret
}

func newPopUpWithBgColor(content core.CanvasObject, canvas core.Canvas, bg color.Color) *PopUp {
	ret := &PopUp{Content: content, Canvas: canvas, modal: false, bgColor: bg, noPadding: true}
	ret.ExtendBaseWidget(ret)
	return ret
}

// NewPopUp creates a new popUp for the specified content on the passed canvas.
// It is not shown until Show is called; use ShowPopUp to create and show in one call.
func NewPopUp(content core.CanvasObject, canvas core.Canvas) *PopUp {
	return newPopUp(content, canvas)
}

// ShowPopUp creates a new popUp for the specified content and displays it on the passed canvas.
func ShowPopUp(content core.CanvasObject, canvas core.Canvas) {
	newPopUp(content, canvas).Show()
}

func newModalPopUp(content core.CanvasObject, canvas core.Canvas) *PopUp {
	p := &PopUp{Content: content, Canvas: canvas, modal: true}
	p.ExtendBaseWidget(p)
	return p
}

// NewModalPopUp creates a new modal popUp for the specified content on the passed canvas.
// It is not shown until Show is called; use ShowModalPopUp to create and show in one call.
// A modal PopUp blocks interactions with underlying elements, covered with a semi-transparent overlay.
func NewModalPopUp(content core.CanvasObject, canvas core.Canvas) *PopUp {
	return newModalPopUp(content, canvas)
}

// ShowModalPopUp creates a new popUp for the specified content and displays it on the passed canvas.
// A modal PopUp blocks interactions with underlying elements, covered with a semi-transparent overlay.
func ShowModalPopUp(content core.CanvasObject, canvas core.Canvas) {
	p := newModalPopUp(content, canvas)
	p.Show()
}

type popUpBaseRenderer struct {
	popUp      *PopUp
	background *canvas.Rectangle
}

func (r *popUpBaseRenderer) padding() geo.Size {
	if r.popUp.noPadding {
		return geo.NewSquareSize(0)
	}
	th := r.popUp.Theme()
	return geo.NewSquareSize(th.Size(theme.SizeNameInnerPadding))
}

func (r *popUpBaseRenderer) offset() geo.Position {
	if r.popUp.noPadding {
		return geo.NewPos(0, 0)
	}
	th := r.popUp.Theme()
	return geo.NewSquareOffsetPos(th.Size(theme.SizeNameInnerPadding) / 2)
}

type popUpRenderer struct {
	*widget.ShadowingRenderer
	popUpBaseRenderer
}

func (r *popUpRenderer) Layout(_ geo.Size) {
	innerSize := r.popUp.innerSize.Max(r.popUp.MinSize())
	canvasSize := r.popUp.Canvas.Size()
	pad := r.padding()

	// Clamp popup size to canvas bounds so content doesn't overflow.
	if innerSize.Width > canvasSize.Width {
		innerSize.Width = canvasSize.Width
	}
	if innerSize.Height > canvasSize.Height {
		innerSize.Height = canvasSize.Height
	}

	r.popUp.Content.Resize(innerSize.Subtract(pad))

	innerPos := r.popUp.innerPos
	if innerPos.X+innerSize.Width > canvasSize.Width {
		innerPos.X = canvasSize.Width - innerSize.Width
		if innerPos.X < 0 {
			innerPos.X = 0
		}
	}
	if innerPos.Y+innerSize.Height > canvasSize.Height {
		innerPos.Y = canvasSize.Height - innerSize.Height
		if innerPos.Y < 0 {
			innerPos.Y = 0
		}
	}
	r.popUp.Content.Move(innerPos.Add(r.offset()))

	r.background.Resize(innerSize)
	r.background.Move(innerPos)
	r.LayoutShadow(innerSize, innerPos)
}

func (r *popUpRenderer) MinSize() geo.Size {
	return r.popUp.Content.MinSize().Add(r.padding())
}

func (r *popUpRenderer) Refresh() {
	th := r.popUp.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	var bgColor color.Color
	if r.popUp.bgColor != nil {
		bgColor = r.popUp.bgColor
	} else {
		bgColor = th.Color(theme.ColorNameSurfaceContainerLow, v)
	}
	// Apply popup opacity for entrance/exit animation.
	if r.popUp.opacity < 1 && r.popUp.opacity >= 0 {
		nr, ng, nb, na := col.ToNRGBA(bgColor)
		a := float32(na) * r.popUp.opacity
		bgColor = color.NRGBA{R: uint8(nr), G: uint8(ng), B: uint8(nb), A: uint8(a)}
	}
	r.background.FillColor = bgColor
	r.background.CornerRadius = th.Size(theme.SizeNameShapeLarge)
	expectedContentSize := r.popUp.innerSize.Max(r.popUp.MinSize()).Subtract(r.padding())
	shouldRelayout := r.popUp.Content.Size() != expectedContentSize

	if r.background.Size() != r.popUp.innerSize || r.background.Position() != r.popUp.innerPos || shouldRelayout {
		r.Layout(r.popUp.Size())
	}
	if r.popUp.Canvas.Size() != r.popUp.BaseWidget.Size() {
		r.popUp.BaseWidget.Resize(r.popUp.Canvas.Size())
	}
	r.popUp.Content.Refresh()
	r.background.Refresh()
	r.ShadowingRenderer.RefreshShadow()
}

type modalPopUpRenderer struct {
	*widget.ShadowingRenderer
	popUpBaseRenderer
	underlay *canvas.Rectangle
}

func (r *modalPopUpRenderer) Layout(canvasSize geo.Size) {
	r.underlay.Resize(canvasSize)

	padding := r.padding()
	innerSize := r.popUp.innerSize.Max(r.popUp.Content.MinSize().Add(padding))

	requestedSize := innerSize.Subtract(padding)
	size := r.popUp.Content.MinSize().Max(requestedSize)
	size = size.Min(canvasSize.Subtract(padding))

	var pos geo.Position
	if r.popUp.innerPosMoved {
		// Explicit position requested via Move/ShowAtPosition.
		pos = r.popUp.innerPos.Add(r.offset())
	} else {
		// Default: center the content.
		pos = geo.NewPos((canvasSize.Width-size.Width)/2, (canvasSize.Height-size.Height)/2)
	}
	r.popUp.Content.Move(pos)
	r.popUp.Content.Resize(size)

	innerPos := pos.Subtract(r.offset())
	r.background.Move(innerPos)
	r.background.Resize(size.Add(padding))
	r.LayoutShadow(innerSize, innerPos)
}

func (r *modalPopUpRenderer) MinSize() geo.Size {
	return r.popUp.Content.MinSize().Add(r.padding())
}

func (r *modalPopUpRenderer) Refresh() {
	th := r.popUp.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	// Apply popup opacity for entrance/exit animation.
	scrimColor := th.Color(theme.ColorNameScrim, v)
	bgColor := th.Color(theme.ColorNameSurfaceContainerLow, v)
	if r.popUp.opacity < 1 && r.popUp.opacity >= 0 {
		sr, sg, sb, sa := col.ToNRGBA(scrimColor)
		scrimColor = color.NRGBA{R: uint8(sr), G: uint8(sg), B: uint8(sb), A: uint8(float32(sa) * r.popUp.opacity)}
		br, bg2, bb, ba := col.ToNRGBA(bgColor)
		bgColor = color.NRGBA{R: uint8(br), G: uint8(bg2), B: uint8(bb), A: uint8(float32(ba) * r.popUp.opacity)}
	}
	r.underlay.FillColor = scrimColor
	r.background.FillColor = bgColor
	r.background.CornerRadius = th.Size(theme.SizeNameShapeLarge)
	expectedContentSize := r.popUp.innerSize.Max(r.popUp.MinSize()).Subtract(r.padding())
	shouldLayout := r.popUp.Content.Size() != expectedContentSize

	if r.background.Size() != r.popUp.innerSize || r.background.Position() != r.popUp.innerPos || shouldLayout {
		r.Layout(r.popUp.Size())
	}
	if r.popUp.Canvas.Size() != r.popUp.BaseWidget.Size() {
		r.popUp.BaseWidget.Resize(r.popUp.Canvas.Size())
	}
	r.popUp.Content.Refresh()
	r.background.Refresh()
	// The scrim's colour fades with the entrance and exit animations, and it
	// covers the canvas, far past the content: unrefreshed, partial repaint
	// redrew it only where the content changed (#605).
	r.underlay.Refresh()
}

func withRelativePosition(rel geo.Position, to core.CanvasObject, f func(position geo.Position)) {
	d := bellui.CurrentApp().Driver()
	c := d.CanvasForObject(to)
	if c == nil {
		bellui.LogError("Could not locate parent object to display relative to", nil)
		f(rel)
		return
	}

	pos := d.AbsolutePositionForObject(to).Add(rel)
	f(pos)
}

// SetPadded controls whether the pop-up surrounds its content with inner
// padding and positions it accordingly. Disable for content that draws its
// own chrome, such as a container.InnerWindow.
func (p *PopUp) SetPadded(padded bool) {
	p.noPadding = !padded
	p.Refresh()
}
