// Package dialog defines standard dialog windows for application GUIs.
package dialog // import "github.com/belluikit/bellui/dialog"

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	col "github.com/belluikit/bellui/internal/color"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

const (
	padWidth  = 32
	padHeight = 16
)

// Dialog is the common API for any dialog window with a single dismiss button
type Dialog interface {
	Show()
	Hide()
	SetDismissText(label string)
	SetOnClosed(closed func())
	Refresh()
	Resize(size geo.Size)

	// MinSize returns the size that this dialog should not shrink below.
	//
	// Since: 2.1
	MinSize() geo.Size

	// Dismiss instructs the dialog to close without any affirmative action.
	//
	// Since: 2.6
	Dismiss()

	// SetWindowed controls whether the dialog is hosted in a window frame
	// that the user can drag by its title bar and resize from its corners.
	// Windowed is the default; pass false for a fixed, centered dialog.
	// Call before Show.
	SetWindowed(windowed bool)

	// SetWindowHosted controls whether Show opens this dialog in its own
	// native window instead of an overlay inside the parent window's canvas.
	// While the dialog window is open the parent window is blocked by a
	// modal scrim; other application windows stay interactive
	// (window-modal). On single-window platforms (mobile, web) the mode is
	// ignored and the dialog falls back to the standard overlay
	// presentation. Call before Show.
	//
	// Since: 2.16
	SetWindowHosted(hosted bool)
}

// Declare conformity to Dialog interface
var _ Dialog = (*dialog)(nil)

type dialog struct {
	callback     func(bool)
	title        string
	icon         bellui.Resource
	desiredSize  geo.Size
	windowed     bool
	windowHosted bool

	win     *widget.PopUp
	body    *bellui.Container // the dialog layout container inside the popup
	content core.CanvasObject
	dismiss *widget.Button
	parent  bellui.Window

	// set while the dialog body is shown in its own native window (#498)
	hostWin bellui.Window
	scrim   *modalScrim

	// allows derived dialogs to inject logic that runs before Show()
	beforeShowHook func()
}

func (d *dialog) Dismiss() {
	d.Hide()
}

func (d *dialog) Hide() {
	d.hideWithResponse(false)
}

// MinSize returns the size that this dialog should not shrink below.
//
// Since: 2.1
func (d *dialog) MinSize() geo.Size {
	if d.hostWin != nil {
		return d.body.MinSize()
	}
	return d.win.MinSize()
}

func (d *dialog) Show() {
	if d.beforeShowHook != nil {
		d.beforeShowHook()
	}
	if d.windowHosted && windowHostSupported {
		d.showHostWindow()
		return
	}
	if !d.desiredSize.IsZero() {
		d.win.Resize(d.desiredSize)
	}
	d.win.Show()
}

func (d *dialog) Refresh() {
	d.refreshHost()
}

// Resize dialog, call this function after dialog show
func (d *dialog) Resize(size geo.Size) {
	d.desiredSize = size
	if d.hostWin != nil {
		d.hostWin.Resize(size)
		return
	}
	if d.win != nil { // could be called before popup is created!
		d.win.Resize(size)
	}
}

// SetDismissText allows custom text to be set in the dismiss button
// This is a no-op for dialogs without dismiss buttons.
func (d *dialog) SetDismissText(label string) {
	if d.dismiss == nil {
		return
	}

	d.dismiss.SetText(label)
	d.refreshHost()
}

// SetOnClosed allows to set a callback function that is called when
// the dialog is closed
func (d *dialog) SetOnClosed(closed func()) {
	// if there is already a callback set, remember it and call both
	originalCallback := d.callback

	d.callback = func(response bool) {
		if originalCallback != nil {
			originalCallback(response)
		}
		closed()
	}
}

func (d *dialog) hideWithResponse(resp bool) {
	d.hideHost()
	if d.callback != nil {
		d.callback(resp)
	}
}

// SetWindowed controls whether the dialog is hosted in a window frame that
// the user can drag by its title bar and resize from its corners. Windowed
// is the default; pass false for a fixed, centered dialog. Call before Show.
func (d *dialog) SetWindowed(windowed bool) {
	if d.windowed == windowed {
		return
	}
	d.windowed = windowed
	if d.win != nil && d.body != nil {
		// The popup is built eagerly by the dialog constructors, so a
		// change of mode rebuilds it around the existing body.
		d.win.Hide()
		d.win = wrapModal(d.body, d.parent.Canvas(), windowed, d.Dismiss)
	}
}

func (d *dialog) create(buttons core.CanvasObject) {
	label := widget.NewLabelWithStyle(d.title, bellui.TextAlignLeading, bellui.TextStyle{Bold: true})

	var image core.CanvasObject
	if d.icon != nil {
		image = &canvas.Image{Resource: d.icon}
	} else {
		image = &layout.Spacer{}
	}

	content := container.New(
		&dialogLayout{d: d},
		image,
		newThemedBackground(),
		d.content,
		buttons,
		label,
	)

	d.body = content
	d.win = wrapModal(content, d.parent.Canvas(), d.windowed, d.Dismiss)
	if d.hostWin != nil {
		// rebuilt while shown in a host window (e.g. ColorPickerDialog
		// updateUI) — re-seat the fresh body into the window
		d.hostWin.SetContent(content)
	}
}

// wrapModal hosts the dialog body in a modal popup. In windowed mode the body
// is framed by an InnerWindow that the user can drag by its title bar and
// resize from its corners; the maximize button toggles between the current
// and the full-canvas size.
func wrapModal(body core.CanvasObject, cv core.Canvas, windowed bool, onClose func()) *widget.PopUp {
	if !windowed {
		return widget.NewModalPopUp(body, cv)
	}

	win := container.NewInnerWindow("", body)
	// Pin the frame buttons to one side: InnerWindow's default placement
	// is OS-dependent, which would make dialog rendering differ per platform.
	win.Alignment = widget.ButtonAlignTrailing
	pop := widget.NewModalPopUp(win, cv)
	// The InnerWindow draws its own frame; popup padding would offset the
	// drag/resize math and draw a second chrome ring around it.
	pop.SetPadded(false)

	win.CloseIntercept = onClose
	win.OnDragged = func(ev *event.DragEvent) {
		pop.Move(win.Position().Add(ev.Dragged))
	}
	win.OnResized = func(ev *event.DragEvent) {
		pop.Resize(win.Size().Add(geo.NewSize(ev.Dragged.DX, ev.Dragged.DY)).Max(win.MinSize()))
	}

	var restoreSize geo.Size
	var restorePos geo.Position
	maximized := false
	win.OnMaximized = func() {
		if maximized {
			pop.Resize(restoreSize)
			pop.Move(restorePos)
		} else {
			restoreSize = win.Size()
			restorePos = win.Position()
			pop.Move(geo.Position{})
			pop.Resize(cv.Size())
		}
		maximized = !maximized
		win.SetMaximized(maximized)
	}

	return pop
}

func (d *dialog) setButtons(buttons core.CanvasObject) {
	d.body.Objects[3] = buttons
	d.refreshHost()
}

func (d *dialog) setIcon(icon bellui.Resource) {
	if icon == nil {
		d.body.Objects[0] = &layout.Spacer{}
		d.refreshHost()
		return
	}
	d.body.Objects[0] = &canvas.Image{Resource: icon}
	d.refreshHost()
}

// The method .create() needs to be called before the dialog can be shown.
func newDialog(title, message string, icon bellui.Resource, callback func(bool), parent bellui.Window) *dialog {
	d := &dialog{content: newCenterWrappedLabel(message), title: title, icon: icon, parent: parent, windowed: true}
	d.callback = callback

	return d
}

// ===============================================================
// ThemedBackground
// ===============================================================

type themedBackground struct {
	widget.BaseWidget
}

func newThemedBackground() *themedBackground {
	t := &themedBackground{}
	t.ExtendBaseWidget(t)
	return t
}

func (t *themedBackground) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	rect := canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground))
	return &themedBackgroundRenderer{rect, []core.CanvasObject{rect}}
}

type themedBackgroundRenderer struct {
	rect    *canvas.Rectangle
	objects []core.CanvasObject
}

func (renderer *themedBackgroundRenderer) Destroy() {
}

func (renderer *themedBackgroundRenderer) Layout(size geo.Size) {
	renderer.rect.Resize(size)
}

func (renderer *themedBackgroundRenderer) MinSize() geo.Size {
	return renderer.rect.MinSize()
}

func (renderer *themedBackgroundRenderer) Objects() []core.CanvasObject {
	return renderer.objects
}

func (renderer *themedBackgroundRenderer) Refresh() {
	r, g, b, _ := col.ToNRGBA(theme.Color(theme.ColorNameOverlayBackground))
	bg := &color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 230}
	renderer.rect.FillColor = bg
}

// ===============================================================
// DialogLayout
// ===============================================================

type dialogLayout struct {
	d *dialog
}

func (l *dialogLayout) Layout(obj []core.CanvasObject, size geo.Size) {
	btnMin := obj[3].MinSize()
	labelMin := obj[4].MinSize()

	// icon
	iconHeight := padHeight*2 + labelMin.Height*2 - theme.Padding()
	obj[0].Resize(geo.NewSize(iconHeight, iconHeight))
	obj[0].Move(geo.NewPos(size.Width-iconHeight+theme.Padding(), -theme.Padding()))

	// background
	obj[1].Move(geo.NewPos(0, 0))
	obj[1].Resize(size)

	// content
	contentStart := obj[4].Position().Y + labelMin.Height + padHeight
	contentEnd := obj[3].Position().Y - theme.Padding()
	obj[2].Move(geo.NewPos(padWidth/2, labelMin.Height+padHeight))
	obj[2].Resize(geo.NewSize(size.Width-padWidth, contentEnd-contentStart))

	// buttons
	obj[3].Resize(btnMin)
	obj[3].Move(geo.NewPos(size.Width/2-(btnMin.Width/2), size.Height-padHeight-btnMin.Height))
}

func (l *dialogLayout) MinSize(obj []core.CanvasObject) geo.Size {
	contentMin := obj[2].MinSize()
	btnMin := obj[3].MinSize()
	labelMin := obj[4].MinSize()

	width := geo.Max(geo.Max(contentMin.Width, btnMin.Width), labelMin.Width) + padWidth
	height := contentMin.Height + btnMin.Height + labelMin.Height + theme.Padding() + padHeight*2

	return geo.NewSize(width, height)
}

// popUpBody returns the dialog layout container hosted in pop, unwrapping
// the window frame when the dialog is windowed.
func popUpBody(pop *widget.PopUp) *bellui.Container {
	if win, ok := pop.Content.(*container.InnerWindow); ok {
		return win.Content().(*bellui.Container)
	}
	return pop.Content.(*bellui.Container)
}
