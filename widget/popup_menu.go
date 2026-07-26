package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
)

var (
	_ core.Widget     = (*PopUpMenu)(nil)
	_ event.Focusable = (*PopUpMenu)(nil)
	_ core.Accessible = (*PopUpMenu)(nil)
)

// AccessibleRole returns [core.RoleMenu].
func (p *PopUpMenu) AccessibleRole() core.AccessibleRole { return core.RoleMenu }

// AccessibleLabel returns an empty string.
func (p *PopUpMenu) AccessibleLabel() string { return "" }

// PopUpMenu is a Menu which displays itself in an OverlayContainer.
type PopUpMenu struct {
	*Menu
	canvas  core.Canvas
	overlay *widget.OverlayContainer
}

// NewPopUpMenu creates a new, reusable popup menu. You can show it using ShowAtPosition.
//
// Since: 2.0
func NewPopUpMenu(menu *bellui.Menu, c core.Canvas) *PopUpMenu {
	m := &Menu{}
	m.setMenu(menu)
	p := &PopUpMenu{Menu: m, canvas: c}
	p.ExtendBaseWidget(p)
	p.Menu.Resize(p.Menu.MinSize())
	p.Menu.customSized = true
	o := widget.NewOverlayContainer(p, c, p.Dismiss)
	o.Resize(o.MinSize())
	p.overlay = o
	p.OnDismiss = func() {
		p.Hide()
	}
	return p
}

// ShowPopUpMenuAtPosition creates a PopUp menu populated with items from the passed menu structure.
// It will automatically be positioned at the provided location and shown as an overlay on the specified canvas.
func ShowPopUpMenuAtPosition(menu *bellui.Menu, c core.Canvas, pos geo.Position) {
	m := NewPopUpMenu(menu, c)
	m.ShowAtPosition(pos)
}

// ShowPopUpMenuAtRelativePosition creates a PopUp menu populated with menu items from the passed menu structure.
// It will automatically be positioned at the given position relative to stated object and shown as an overlay on the specified canvas.
//
// Since 2.4
func ShowPopUpMenuAtRelativePosition(menu *bellui.Menu, c core.Canvas, rel geo.Position, to core.CanvasObject) {
	withRelativePosition(rel, to, func(pos geo.Position) {
		ShowPopUpMenuAtPosition(menu, c, pos)
	})
}

// FocusGained is triggered when the object gained focus. For the pop-up menu it does nothing.
func (p *PopUpMenu) FocusGained() {}

// FocusLost is triggered when the object lost focus. For the pop-up menu it does nothing.
func (p *PopUpMenu) FocusLost() {}

// Hide hides the pop-up menu.
func (p *PopUpMenu) Hide() {
	p.overlay.Hide()
	p.Menu.Hide()
}

// Move moves the pop-up menu.
// The position is absolute because pop-up menus are shown in an overlay which covers the whole canvas.
func (p *PopUpMenu) Move(pos geo.Position) {
	p.BaseWidget.Move(p.adjustedPosition(pos, p.Size()))
}

// Resize changes the size of the pop-up menu.
func (p *PopUpMenu) Resize(size geo.Size) {
	p.BaseWidget.Move(p.adjustedPosition(p.Position(), size))
	p.Menu.Resize(size)
}

// Show makes the pop-up menu visible.
func (p *PopUpMenu) Show() {
	p.Menu.alignment = p.alignment
	p.Menu.Refresh()

	p.overlay.Show()
	p.Menu.Show()
	if !bellui.CurrentDevice().IsMobile() {
		p.canvas.Focus(p)
	}
}

// ShowAtPosition shows the pop-up menu at the specified position.
func (p *PopUpMenu) ShowAtPosition(pos geo.Position) {
	p.Move(pos)
	p.Show()
}

// ShowAtRelativePosition shows the pop-up menu at the position relative to given object.
//
// Since 2.4
func (p *PopUpMenu) ShowAtRelativePosition(rel geo.Position, to core.CanvasObject) {
	withRelativePosition(rel, to, p.ShowAtPosition)
}

// TypedKey handles key events. It allows keyboard control of the pop-up menu.
func (p *PopUpMenu) TypedKey(e *event.KeyEvent) {
	switch e.Name {
	case event.KeyDown:
		p.ActivateNext()
	case event.KeyEnter, event.KeyReturn, event.KeySpace:
		p.TriggerLast()
	case event.KeyEscape:
		p.Dismiss()
	case event.KeyLeft:
		p.DeactivateLastSubmenu()
	case event.KeyRight:
		p.ActivateLastSubmenu()
	case event.KeyUp:
		p.ActivatePrevious()
	}
}

// TypedRune handles text events. For pop-up menus this does nothing.
func (p *PopUpMenu) TypedRune(rune) {}

func (p *PopUpMenu) adjustedPosition(pos geo.Position, size geo.Size) geo.Position {
	x := pos.X
	y := pos.Y
	_, areaSize := p.canvas.InteractiveArea()
	if x+size.Width > areaSize.Width {
		x = areaSize.Width - size.Width
		if x < 0 {
			x = 0 // wider than the canvas: pin to the left; the menu scrolls horizontally
		}
	}
	if y+size.Height > areaSize.Height {
		y = areaSize.Height - size.Height
		if y < 0 {
			y = 0 // taller than the canvas: pin to the top; the menu scrolls vertically
		}
	}
	return geo.NewPos(x, y)
}
