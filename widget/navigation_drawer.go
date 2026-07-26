package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// NavigationDrawerItem represents a destination in a NavigationDrawer.
type NavigationDrawerItem struct {
	Label     string
	Icon      bellui.Resource
	IsDivider bool // true = render as separator
}

// NavigationDrawer is an MD3 slide-out side panel with destinations.
type NavigationDrawer struct {
	Items     []*NavigationDrawerItem
	Header    core.CanvasObject // optional header content
	Selected  int
	OnChanged func(int) `json:"-"`
	Modal     bool

	canvas core.Canvas
	popUp  *PopUp
	open   bool
}

// NewNavigationDrawer creates a navigation drawer.
func NewNavigationDrawer(items []*NavigationDrawerItem, changed func(int)) *NavigationDrawer {
	return &NavigationDrawer{Items: items, OnChanged: changed, Modal: true}
}

// Show displays the drawer on the given canvas.
func (d *NavigationDrawer) Show(c core.Canvas) {
	d.canvas = c
	d.Hide()

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	var objects []core.CanvasObject

	if d.Header != nil {
		objects = append(objects, d.Header)
	}

	for i, item := range d.Items {
		if item.IsDivider {
			sep := NewSeparator()
			objects = append(objects, sep)
			continue
		}

		idx := i
		bg := canvas.NewRectangle(color.Transparent)
		bg.CornerRadius = 28

		icon := canvas.NewImageFromResource(item.Icon)
		icon.FillMode = canvas.ImageFillContain
		icon.Resize(geo.NewSize(24, 24))

		label := canvas.NewText(item.Label, th.Color(theme.ColorNameOnSurface, v))
		label.TextSize = 14

		if idx == d.Selected {
			bg.FillColor = th.Color(theme.ColorNameSecondaryContainer, v)
			icon.Resource = theme.NewColoredResource(item.Icon, theme.ColorNameOnSecondaryContainer)
			label.Color = th.Color(theme.ColorNameOnSecondaryContainer, v)
			label.TextStyle.Bold = true
		} else {
			icon.Resource = theme.NewColoredResource(item.Icon, theme.ColorNameOnSurfaceVariant)
			label.Color = th.Color(theme.ColorNameOnSurfaceVariant, v)
		}

		row := &drawerItemWidget{
			bg: bg, icon: icon, label: label,
			selected: idx == d.Selected,
			onTap: func() {
				d.Selected = idx
				if d.OnChanged != nil {
					d.OnChanged(idx)
				}
				d.Hide()
			},
		}
		row.ExtendBaseWidget(row)
		objects = append(objects, row)
	}

	drawerWidth := float32(360)
	content := &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: objects}
	padded := &bellui.Container{Layout: layout.NewPaddedLayout(), Objects: []core.CanvasObject{content}}
	// Scroll so destination lists taller than the canvas remain reachable.
	scroll := internalWidget.NewScroll(padded)

	if d.Modal {
		d.popUp = NewModalPopUp(scroll, c)
		d.popUp.modalDismissOnTap = true
	} else {
		d.popUp = NewPopUp(scroll, c)
	}

	canvasSize := c.Size()
	height := canvasSize.Height
	d.popUp.Resize(geo.NewSize(drawerWidth, height))
	d.popUp.Show()
	d.popUp.Move(geo.NewPos(0, 0))
	d.open = true
}

// Hide dismisses the drawer.
func (d *NavigationDrawer) Hide() {
	if d.popUp != nil {
		d.popUp.Hide()
		d.popUp = nil
	}
	d.open = false
}

// IsOpen returns whether the drawer is showing.
func (d *NavigationDrawer) IsOpen() bool {
	if d.open && d.popUp != nil && !d.popUp.Visible() {
		d.open = false
		d.popUp = nil
	}
	return d.open
}

// Toggle opens or closes the drawer.
func (d *NavigationDrawer) Toggle(c core.Canvas) {
	if d.IsOpen() {
		d.Hide()
	} else {
		d.Show(c)
	}
}

// --- internal drawer item ---

type drawerItemWidget struct {
	BaseWidget
	bg       *canvas.Rectangle
	icon     *canvas.Image
	label    *canvas.Text
	selected bool
	onTap    func()
}

func (w *drawerItemWidget) Tapped(*event.PointEvent) {
	if w.onTap != nil {
		w.onTap()
	}
}

// AccessibleRole returns [core.RoleMenuItem]; the drawer is a menu-like
// list of destinations shown in a popup.
func (w *drawerItemWidget) AccessibleRole() core.AccessibleRole { return core.RoleMenuItem }

// AccessibleLabel returns the destination's label.
func (w *drawerItemWidget) AccessibleLabel() string { return w.label.Text }

// AccessibleState reports whether the destination is selected.
func (w *drawerItemWidget) AccessibleState() core.AccessibleState {
	if w.selected {
		return core.StateSelected
	}
	return 0
}

// AccessibleActions returns the "select" action.
func (w *drawerItemWidget) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this destination"}}
}

// PerformAccessibleAction selects the destination like a tap would.
func (w *drawerItemWidget) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		w.Tapped(nil)
	}
}

func (w *drawerItemWidget) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	return &drawerItemRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{w.bg, w.icon, w.label}),
		item:         w,
	}
}

type drawerItemRenderer struct {
	internalWidget.BaseRenderer
	item *drawerItemWidget
}

func (r *drawerItemRenderer) MinSize() geo.Size {
	return geo.NewSize(280, 56)
}

func (r *drawerItemRenderer) Layout(size geo.Size) {
	r.item.bg.Resize(size)
	r.item.icon.Move(geo.NewPos(16, (size.Height-24)/2))
	r.item.icon.Resize(geo.NewSize(24, 24))
	labelSize := r.item.label.MinSize()
	r.item.label.Move(geo.NewPos(52, (size.Height-labelSize.Height)/2))
}

func (r *drawerItemRenderer) Refresh() {
	r.Layout(r.item.Size())
	canvas.Refresh(r.item.super())
}
