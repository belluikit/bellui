package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// NavigationRailItem represents a destination in a NavigationRail.
type NavigationRailItem struct {
	Label string
	Icon  bellui.Resource
}

var _ core.Accessible = (*NavigationRail)(nil)

// AccessibleRole returns [core.RoleNavigation].
func (n *NavigationRail) AccessibleRole() core.AccessibleRole { return core.RoleNavigation }

// AccessibleLabel returns an empty string.
func (n *NavigationRail) AccessibleLabel() string { return "" }

// NavigationRail is an MD3 side navigation rail with 3-7 destinations.
type NavigationRail struct {
	BaseWidget
	Items      []*NavigationRailItem
	Selected   int
	ShowLabels bool      // show labels below icons
	OnChanged  func(int) `json:"-"`
}

// NewNavigationRail creates a side navigation rail.
func NewNavigationRail(items []*NavigationRailItem, changed func(int)) *NavigationRail {
	n := &NavigationRail{Items: items, ShowLabels: true, OnChanged: changed}
	n.ExtendBaseWidget(n)
	return n
}

func (n *NavigationRail) MinSize() geo.Size {
	n.ExtendBaseWidget(n)
	return n.BaseWidget.MinSize()
}

func (n *NavigationRail) CreateRenderer() core.WidgetRenderer {
	n.ExtendBaseWidget(n)
	bg := canvas.NewRectangle(color.Transparent)

	var objects []core.CanvasObject
	objects = append(objects, bg)

	var items []*navRailItemWidget
	for i, item := range n.Items {
		idx := i
		w := &navRailItemWidget{
			item: item, selected: i == n.Selected, showLabel: n.ShowLabels,
			onTap: func() {
				n.Selected = idx
				if n.OnChanged != nil {
					n.OnChanged(idx)
				}
				n.Refresh()
			},
		}
		w.ExtendBaseWidget(w)
		items = append(items, w)
		objects = append(objects, w)
	}

	r := &navRailRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		items:        items,
		rail:         n,
	}
	r.Refresh()
	return r
}

type navRailItemWidget struct {
	BaseWidget
	item      *NavigationRailItem
	selected  bool
	showLabel bool
	onTap     func()
}

func (w *navRailItemWidget) Tapped(*event.PointEvent) {
	if w.onTap != nil {
		w.onTap()
	}
}

// AccessibleRole returns [core.RoleTab]; the rail hosts one selected destination of N.
func (w *navRailItemWidget) AccessibleRole() core.AccessibleRole { return core.RoleTab }

// AccessibleLabel returns the destination's label.
func (w *navRailItemWidget) AccessibleLabel() string { return w.item.Label }

// AccessibleState reports whether the destination is selected.
func (w *navRailItemWidget) AccessibleState() core.AccessibleState {
	if w.selected {
		return core.StateSelected
	}
	return 0
}

// AccessibleActions returns the "select" action.
func (w *navRailItemWidget) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this destination"}}
}

// PerformAccessibleAction selects the destination like a tap would.
func (w *navRailItemWidget) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		w.Tapped(nil)
	}
}

func (w *navRailItemWidget) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	indicator := canvas.NewRectangle(color.Transparent)
	icon := canvas.NewImageFromResource(w.item.Icon)
	icon.FillMode = canvas.ImageFillContain
	label := canvas.NewText(w.item.Label, color.White)
	label.Alignment = bellui.TextAlignCenter

	r := &navRailItemRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{indicator, icon, label}),
		indicator:    indicator,
		icon:         icon,
		label:        label,
		item:         w,
	}
	r.Refresh()
	return r
}

type navRailItemRenderer struct {
	internalWidget.BaseRenderer
	indicator *canvas.Rectangle
	icon      *canvas.Image
	label     *canvas.Text
	item      *navRailItemWidget
}

func (r *navRailItemRenderer) MinSize() geo.Size {
	th := r.item.Theme()
	railW := th.Size(theme.SizeNameNavRailWidth)
	if r.item.showLabel {
		return geo.NewSize(railW, railW*0.7)
	}
	return geo.NewSize(railW, railW*0.6)
}

func (r *navRailItemRenderer) Layout(size geo.Size) {
	th := r.item.Theme()
	railW := th.Size(theme.SizeNameNavRailWidth)
	iconSize := th.Size(theme.SizeNameIconSize)
	indicatorW := railW * 0.7
	indicatorH := railW * 0.4
	cx := size.Width / 2

	r.indicator.CornerRadius = indicatorH / 2
	r.indicator.Resize(geo.NewSize(indicatorW, indicatorH))
	r.indicator.Move(geo.NewPos(cx-indicatorW/2, railW*0.05))

	r.icon.Resize(geo.NewSize(iconSize, iconSize))
	r.icon.Move(geo.NewPos(cx-iconSize/2, railW*0.1))

	r.label.TextSize = th.Size(theme.SizeNameLabelMedium)
	if r.item.showLabel {
		labelH := r.label.MinSize().Height
		r.label.Resize(geo.NewSize(size.Width, labelH))
		r.label.Move(geo.NewPos(0, railW*0.475))
		r.label.Show()
	} else {
		r.label.Hide()
	}
}

func (r *navRailItemRenderer) Refresh() {
	th := r.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	if r.item.selected {
		r.indicator.FillColor = th.Color(theme.ColorNameSecondaryContainer, v)
		r.icon.Resource = theme.NewColoredResource(r.item.item.Icon, theme.ColorNameOnSecondaryContainer)
		r.label.Color = th.Color(theme.ColorNameOnSurface, v)
		r.label.TextStyle.Bold = true
	} else {
		r.indicator.FillColor = color.Transparent
		r.icon.Resource = theme.NewColoredResource(r.item.item.Icon, theme.ColorNameOnSurfaceVariant)
		r.label.Color = th.Color(theme.ColorNameOnSurfaceVariant, v)
		r.label.TextStyle.Bold = false
	}

	r.indicator.Refresh()
	r.icon.Refresh()
	r.label.Refresh()
	r.Layout(r.item.Size())
	canvas.Refresh(r.item.super())
}

type navRailRenderer struct {
	internalWidget.BaseRenderer
	bg    *canvas.Rectangle
	items []*navRailItemWidget
	rail  *NavigationRail
}

func (r *navRailRenderer) MinSize() geo.Size {
	th := r.rail.Theme()
	railW := th.Size(theme.SizeNameNavRailWidth)
	itemH := railW * 0.7
	if !r.rail.ShowLabels {
		itemH = railW * 0.6
	}
	return geo.NewSize(railW, float32(len(r.items))*itemH+railW*0.2)
}

func (r *navRailRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	th := r.rail.Theme()
	railW := th.Size(theme.SizeNameNavRailWidth)
	itemH := railW * 0.7
	if !r.rail.ShowLabels {
		itemH = railW * 0.6
	}
	y := railW * 0.15
	for _, item := range r.items {
		item.Move(geo.NewPos(0, y))
		item.Resize(geo.NewSize(size.Width, itemH))
		y += itemH
	}
}

func (r *navRailRenderer) Refresh() {
	th := r.rail.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerMedium, v)
	r.bg.Refresh()

	for i, item := range r.items {
		item.selected = i == r.rail.Selected
		item.showLabel = r.rail.ShowLabels
		item.Refresh()
	}
	r.Layout(r.rail.Size())
	canvas.Refresh(r.rail.super())
}
