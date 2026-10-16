package dialog

// Window-hosted dialogs (pre-rebrand tracker #498, Phase 1): an opt-in mode where Show opens the
// dialog body in its own native window instead of an overlay popup inside the
// parent window's canvas. The dialog window can be moved and resized freely by
// the OS window manager and appears in the window switcher, while the parent
// window is blocked by a modal scrim overlay until the dialog closes
// (window-modal: other application windows stay interactive). This is a pure
// application-level modality — the native parent/owner window relationship
// (always-on-top of parent, minimize-with-parent) is Phase 2.

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// SetWindowHosted controls whether Show opens this dialog in its own native
// window instead of an overlay inside the parent window's canvas. While the
// dialog window is open the parent window is blocked by a modal scrim
// (tapping it focuses the dialog window); other application windows stay
// interactive. Closing the dialog window — its close button or the Escape
// key — dismisses the dialog as if the cancel button were tapped.
// On single-window platforms (mobile, web) the mode is ignored and the
// dialog falls back to the standard overlay presentation. Call before Show.
//
// Since: 1.0
func (d *dialog) SetWindowHosted(hosted bool) {
	d.windowHosted = hosted
}

// showHostWindow opens the dialog body in its own native window, blocking the
// parent window with a scrim overlay for the lifetime of the window.
func (d *dialog) showHostWindow() {
	if d.hostWin != nil {
		d.hostWin.RequestFocus()
		return
	}

	w := bellui.CurrentApp().NewWindow(d.title)
	d.hostWin = w
	w.SetContent(d.body)
	if !d.desiredSize.IsZero() {
		w.Resize(d.desiredSize)
	}
	w.SetCloseIntercept(d.Dismiss)
	w.SetOnClosed(d.hostWindowClosed)
	w.Canvas().SetOnTypedKey(func(ev *event.KeyEvent) {
		if ev.Name == event.KeyEscape {
			d.Dismiss()
		}
	})

	d.showScrim()
	adoptParentWindow(w, d.parent)
	adoptModal(w, d.parent)
	centerWindowOver(w, d.parent, d.hostSize())
	w.Show()
}

// hideHost hides the dialog — the host window when window-hosted, the overlay
// popup otherwise — without running the response callback.
func (d *dialog) hideHost() {
	if d.hostWin != nil {
		w := d.hostWin
		d.hostWin = nil
		d.hideScrim()
		w.Close()
		return
	}
	d.win.Hide()
}

// hostWindowClosed runs when the host window closes for any reason. When the
// window was closed from outside the dialog API (for example application code
// calling Close on it), the dialog cleans up and reports a dismissal.
func (d *dialog) hostWindowClosed() {
	if d.hostWin == nil { // closed through the dialog API, already handled
		return
	}
	d.hostWin = nil
	d.hideScrim()
	if d.callback != nil {
		d.callback(false)
	}
}

// refreshHost refreshes whichever surface currently hosts the dialog body.
// The overlay popup must not be refreshed while the body lives in a host
// window: its renderer would lay the shared body out to the popup's size.
func (d *dialog) refreshHost() {
	if d.hostWin != nil {
		d.body.Refresh()
		return
	}
	d.win.Refresh()
}

// hostSize is the size the host window is expected to open at, used to center
// it over the parent before it is shown.
func (d *dialog) hostSize() geo.Size {
	if !d.desiredSize.IsZero() {
		return d.desiredSize
	}
	pad := theme.Padding() * 2
	return d.body.MinSize().Add(geo.NewSize(pad, pad))
}

func (d *dialog) showScrim() {
	if d.scrim != nil {
		return
	}
	d.scrim = addScrim(d.parent, func() {
		if d.hostWin != nil {
			d.hostWin.RequestFocus()
		}
	})
}

func (d *dialog) hideScrim() {
	if d.scrim == nil {
		return
	}
	removeScrim(d.parent, d.scrim)
	d.scrim = nil
}

// addScrim adds a modal scrim overlay blocking parent's canvas and returns it.
// onTapped runs when the scrim is tapped (window-modal convention: focus the
// dialog window rather than dismiss it).
func addScrim(parent bellui.Window, onTapped func()) *modalScrim {
	s := newModalScrim(onTapped)
	cv := parent.Canvas()
	cv.Overlays().Add(s)
	s.Resize(cv.Size())
	return s
}

// removeScrim removes a scrim previously added to parent's canvas.
func removeScrim(parent bellui.Window, s *modalScrim) {
	if s == nil {
		return
	}
	parent.Canvas().Overlays().Remove(s)
}

// adoptParentWindow gives the host window a native parent/owner relationship
// with its dialog's parent when the driver supports it (pre-rebrand tracker #598, desktop.
// ChildWindow): the window manager then keeps the dialog above its parent and
// minimizes them together, complementing the pure-Go modal scrim. A no-op on
// drivers without the capability (mobile/web).
func adoptParentWindow(child, parent bellui.Window) {
	if cw, ok := child.(desktop.ChildWindow); ok {
		cw.SetParent(parent)
	}
}

// adoptModal asks the driver to enforce native window-modality of the host
// window against its dialog's parent when supported (pre-rebrand tracker #498, desktop.ModalWindow):
// the OS then blocks input to the parent while the dialog is up, matching the
// pure-Go scrim. A no-op on drivers without the capability, where the scrim is
// the only blocker. The parent must already be adopted (adoptParentWindow), and
// the child re-enables it on close, so no explicit un-set is needed here.
func adoptModal(child, parent bellui.Window) {
	if mw, ok := child.(desktop.ModalWindow); ok {
		mw.SetParent(parent)
		mw.SetModal(true)
	}
}

// centerWindowOver requests the child window be positioned centered over the
// parent window, on platforms that reveal window positions (pre-rebrand tracker #497). childSize
// is the expected child window size in device-independent pixels; the parent
// canvas scale approximates the child's, which is unknown before first show.
func centerWindowOver(child, parent bellui.Window, childSize geo.Size) {
	p, ok := parent.(desktop.PositionableWindow)
	if !ok {
		return
	}
	c, ok := child.(desktop.Window)
	if !ok {
		return
	}

	px, py := p.Position()
	cv := parent.Canvas()
	pw := scale.ToScreenCoordinate(cv, cv.Size().Width)
	ph := scale.ToScreenCoordinate(cv, cv.Size().Height)
	cw := scale.ToScreenCoordinate(cv, childSize.Width)
	ch := scale.ToScreenCoordinate(cv, childSize.Height)
	c.RequestPosition(px+(pw-cw)/2, py+(ph-ch)/2)
}

// modalScrim dims the parent window's canvas and blocks its input while a
// window-hosted dialog is open. Tapping it focuses the dialog window rather
// than dismissing the dialog, matching window-modal convention.
type modalScrim struct {
	widget.BaseWidget

	onTapped func()
}

func newModalScrim(onTapped func()) *modalScrim {
	s := &modalScrim{onTapped: onTapped}
	s.ExtendBaseWidget(s)
	return s
}

func (s *modalScrim) CreateRenderer() core.WidgetRenderer {
	rect := canvas.NewRectangle(s.scrimColor())
	return &modalScrimRenderer{scrim: s, rect: rect}
}

func (s *modalScrim) Tapped(*event.PointEvent) {
	if s.onTapped != nil {
		s.onTapped()
	}
}

func (s *modalScrim) TappedSecondary(*event.PointEvent) {
}

func (s *modalScrim) scrimColor() color.Color {
	return s.Theme().Color(theme.ColorNameScrim, bellui.CurrentApp().Settings().ThemeVariant())
}

type modalScrimRenderer struct {
	scrim *modalScrim
	rect  *canvas.Rectangle
}

func (r *modalScrimRenderer) Destroy() {
}

func (r *modalScrimRenderer) Layout(size geo.Size) {
	r.rect.Resize(size)
}

func (r *modalScrimRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *modalScrimRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.rect}
}

func (r *modalScrimRenderer) Refresh() {
	r.rect.FillColor = r.scrim.scrimColor()
	r.rect.Refresh()
}
