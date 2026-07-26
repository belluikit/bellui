package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var _ core.Widget = (*MenuBar)(nil)

// MenuBar is a widget for displaying a bellui.MainMenu in a bar.
type MenuBar struct {
	widget.Base
	Items []core.CanvasObject

	active     bool
	activeItem *menuBarItem
	canvas     core.Canvas
}

// NewMenuBar creates a menu bar populated with items from the passed main menu structure.
func NewMenuBar(mainMenu *bellui.MainMenu, canvas core.Canvas) *MenuBar {
	items := make([]core.CanvasObject, len(mainMenu.Items))
	b := &MenuBar{Items: items, canvas: canvas}
	b.ExtendBaseWidget(b)
	for i, menu := range mainMenu.Items {
		barItem := &menuBarItem{Menu: menu, Parent: b}
		barItem.ExtendBaseWidget(barItem)
		items[i] = barItem
	}
	return b
}

// CreateRenderer returns a new renderer for the menu bar.
func (b *MenuBar) CreateRenderer() core.WidgetRenderer {
	cont := container.NewHBox(b.Items...)
	background := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	underlay := &menuBarUnderlay{action: b.deactivate}
	underlay.ExtendBaseWidget(underlay)
	objects := []core.CanvasObject{underlay, background, cont}
	for _, item := range b.Items {
		objects = append(objects, item.(*menuBarItem).Child())
	}
	return &menuBarRenderer{
		widget.NewShadowingRenderer(objects, widget.MenuLevel),
		b,
		background,
		underlay,
		cont,
	}
}

// IsActive returns whether the menu bar is active or not.
// An active menu bar shows the current selected menu and should have the focus.
func (b *MenuBar) IsActive() bool {
	return b.active
}

// Toggle changes the activation state of the menu bar.
// On activation, the first item will become active.
func (b *MenuBar) Toggle() {
	b.toggle(b.Items[0].(*menuBarItem))
}

func (b *MenuBar) activateChild(item *menuBarItem) {
	b.active = true
	if item.Child() != nil {
		item.Child().DeactivateChild()
	}
	if b.activeItem == item {
		return
	}

	if b.activeItem != nil {
		if c := b.activeItem.Child(); c != nil {
			c.Hide()
		}
		b.activeItem.Refresh()
	}
	b.activeItem = item
	if item == nil {
		return
	}

	item.Refresh()
	item.Child().Show()
	b.Refresh()
}

func (b *MenuBar) deactivate() {
	if !b.active {
		return
	}

	b.active = false
	if b.activeItem != nil {
		if c := b.activeItem.Child(); c != nil {
			defer c.Dismiss()
			c.Hide()
		}
		b.activeItem.Refresh()
		b.activeItem = nil
	}
	b.Refresh()
}

func (b *MenuBar) toggle(item *menuBarItem) {
	if b.active {
		b.canvas.Unfocus()
		b.deactivate()
	} else {
		b.activateChild(item)
		b.canvas.Focus(item)
	}
}

type menuBarRenderer struct {
	*widget.ShadowingRenderer
	b          *MenuBar
	background *canvas.Rectangle
	underlay   *menuBarUnderlay
	cont       *bellui.Container
}

func (r *menuBarRenderer) Layout(size geo.Size) {
	r.LayoutShadow(size, geo.NewPos(0, 0))
	minSize := r.MinSize()
	if size.Height != minSize.Height || size.Width < minSize.Width {
		r.b.Resize(geo.NewSize(geo.Max(size.Width, minSize.Width), minSize.Height))
		return
	}

	if r.b.active {
		r.underlay.Resize(r.b.canvas.Size())
	} else {
		r.underlay.Resize(geo.NewSize(0, 0))
	}
	innerPadding := theme.InnerPadding()
	r.cont.Resize(geo.NewSize(size.Width-2*innerPadding, size.Height))
	r.cont.Move(geo.NewPos(innerPadding, 0))
	if item := r.b.activeItem; item != nil {
		if item.Child().Size().IsZero() {
			item.Child().Resize(item.Child().MinSize())
		}
		item.Child().Move(geo.NewPos(item.Position().X+innerPadding, item.Size().Height))
	}
	r.background.Resize(size)
}

func (r *menuBarRenderer) MinSize() geo.Size {
	return r.cont.MinSize().Add(geo.NewSize(theme.InnerPadding()*2, 0))
}

func (r *menuBarRenderer) Refresh() {
	r.Layout(r.b.Size())
	r.background.FillColor = theme.Color(theme.ColorNameBackground)
	r.background.Refresh()
	r.ShadowingRenderer.RefreshShadow()
	canvas.Refresh(r.b)
}

// Transparent underlay shown as soon as menu is active.
// It catches mouse events outside the menu's objects.
type menuBarUnderlay struct {
	widget.Base
	action func()
}

var (
	_ core.Widget     = (*menuBarUnderlay)(nil)
	_ event.Tappable  = (*menuBarUnderlay)(nil) // deactivate menu on click outside
	_ event.Hoverable = (*menuBarUnderlay)(nil) // block hover events on main content
)

func (u *menuBarUnderlay) CreateRenderer() core.WidgetRenderer {
	return &menuUnderlayRenderer{}
}

func (u *menuBarUnderlay) HoverIn(*event.HoverEvent) {
}

func (u *menuBarUnderlay) HoverOut() {
}

func (u *menuBarUnderlay) HoverMoved(*event.HoverEvent) {
}

func (u *menuBarUnderlay) Tapped(*event.PointEvent) {
	u.action()
}

type menuUnderlayRenderer struct {
	widget.BaseRenderer
}

var _ core.WidgetRenderer = (*menuUnderlayRenderer)(nil)

func (r *menuUnderlayRenderer) Layout(geo.Size) {
}

func (r *menuUnderlayRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *menuUnderlayRenderer) Refresh() {
}
