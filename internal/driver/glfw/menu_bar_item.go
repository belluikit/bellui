package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
	publicWidget "github.com/belluikit/bellui/widget"
)

var (
	_ core.Widget     = (*menuBarItem)(nil)
	_ event.Hoverable = (*menuBarItem)(nil)
	_ event.Focusable = (*menuBarItem)(nil)
)

// menuBarItem is a widget for displaying an item for a bellui.Menu in a MenuBar.
type menuBarItem struct {
	widget.Base
	Menu   *bellui.Menu
	Parent *MenuBar

	active  bool
	child   *publicWidget.Menu
	hovered bool
}

func (i *menuBarItem) Child() *publicWidget.Menu {
	if i.child == nil {
		child := publicWidget.NewMenu(i.Menu)
		child.Hide()
		child.OnDismiss = i.Parent.deactivate
		i.child = child
	}
	return i.child
}

// CreateRenderer returns a new renderer for the menu bar item.
func (i *menuBarItem) CreateRenderer() core.WidgetRenderer {
	background := canvas.NewRectangle(theme.Color(theme.ColorNameHover))
	background.CornerRadius = theme.SelectionRadiusSize()
	background.Hide()
	text := canvas.NewText(i.Menu.Label, theme.Color(theme.ColorNameForeground))
	objects := []core.CanvasObject{background, text}

	return &menuBarItemRenderer{
		widget.NewBaseRenderer(objects),
		i,
		text,
		background,
	}
}

func (i *menuBarItem) FocusGained() {
	i.active = true
	if i.Parent.active {
		i.Parent.activateChild(i)
	}
	i.Refresh()
}

func (i *menuBarItem) FocusLost() {
	i.active = false
	i.Refresh()
}

func (i *menuBarItem) Focused() bool {
	return i.active
}

// HoverIn activates the item and shows the menu if the bar is active.
// The menu that was displayed before will be hidden.
//
// If the bar is not active, the item will be hovered.
func (i *menuBarItem) HoverIn(_ *event.HoverEvent) {
	i.hovered = true
	if i.Parent.active {
		i.Parent.canvas.Focus(i)
	}
	i.Refresh()
}

// HoverMoved activates the item and shows the menu if the bar is active.
// The menu that was displayed before will be hidden.
// This might have an effect when mouse and keyboard control are mixed.
// Changing the active menu with the keyboard will make the hovered menu bar item inactive.
// On the next mouse move the hovered item is activated again.
//
// If the bar is not active, this will do nothing.
func (i *menuBarItem) HoverMoved(_ *event.HoverEvent) {
	if i.Parent.active {
		i.Parent.canvas.Focus(i)
	}
}

// HoverOut does nothing if the bar is active.
//
// IF the bar is not active, it changes the item to not be hovered.
func (i *menuBarItem) HoverOut() {
	i.hovered = false
	i.Refresh()
}

// Tapped toggles the activation state of the menu bar.
// It shows the item’s menu if the bar is activated and hides it if the bar is deactivated.
func (i *menuBarItem) Tapped(*event.PointEvent) {
	i.Parent.toggle(i)
}

func (i *menuBarItem) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeyLeft:
		if !i.Child().DeactivateLastSubmenu() {
			i.Parent.canvas.FocusPrevious()
		}
	case event.KeyRight:
		if !i.Child().ActivateLastSubmenu() {
			i.Parent.canvas.FocusNext()
		}
	case event.KeyDown:
		i.Child().ActivateNext()
	case event.KeyUp:
		i.Child().ActivatePrevious()
	case event.KeyEnter, event.KeyReturn, event.KeySpace:
		i.Child().TriggerLast()
	}
}

func (i *menuBarItem) TypedRune(_ rune) {
}

type menuBarItemRenderer struct {
	widget.BaseRenderer
	i          *menuBarItem
	text       *canvas.Text
	background *canvas.Rectangle
}

func (r *menuBarItemRenderer) Layout(size geo.Size) {
	padding := r.padding()

	r.text.TextSize = theme.TextSize()
	r.text.Color = theme.Color(theme.ColorNameForeground)
	r.text.Resize(r.text.MinSize())
	r.text.Move(geo.NewPos(padding.Width/2, padding.Height/2))

	r.background.Resize(size)
}

func (r *menuBarItemRenderer) MinSize() geo.Size {
	return r.text.MinSize().Add(r.padding())
}

func (r *menuBarItemRenderer) Refresh() {
	r.background.CornerRadius = theme.SelectionRadiusSize()
	if r.i.active && r.i.Parent.active {
		r.background.FillColor = theme.Color(theme.ColorNameFocus)
		r.background.Show()
	} else if r.i.hovered && !r.i.Parent.active {
		r.background.FillColor = theme.Color(theme.ColorNameHover)
		r.background.Show()
	} else {
		r.background.Hide()
	}
	r.background.Refresh()
	canvas.Refresh(r.i)
}

func (r *menuBarItemRenderer) padding() geo.Size {
	return geo.NewSize(theme.InnerPadding()*2, theme.InnerPadding())
}
