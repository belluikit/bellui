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

// NavigationBarItem represents a destination in a NavigationBar.
type NavigationBarItem struct {
	Label string
	Icon  bellui.Resource
}

var _ core.Accessible = (*NavigationBar)(nil)

// AccessibleRole returns [core.RoleNavigation].
func (n *NavigationBar) AccessibleRole() core.AccessibleRole { return core.RoleNavigation }

// AccessibleLabel returns an empty string.
func (n *NavigationBar) AccessibleLabel() string { return "" }

// NavigationBar is an MD3 bottom navigation bar with 3-5 destinations.
type NavigationBar struct {
	BaseWidget
	Items     []*NavigationBarItem
	Selected  int       // index of the selected destination; out of range selects nothing
	OnChanged func(int) `json:"-"`
}

// NewNavigationBar creates a bottom navigation bar.
func NewNavigationBar(items []*NavigationBarItem, changed func(int)) *NavigationBar {
	n := &NavigationBar{Items: items, OnChanged: changed}
	n.ExtendBaseWidget(n)
	return n
}

func (n *NavigationBar) MinSize() geo.Size {
	n.ExtendBaseWidget(n)
	return n.BaseWidget.MinSize()
}

func (n *NavigationBar) CreateRenderer() core.WidgetRenderer {
	n.ExtendBaseWidget(n)
	bg := canvas.NewRectangle(color.Transparent)

	r := &navBarRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{bg}),
		bg:           bg,
		bar:          n,
	}
	r.Refresh() // populates the item widgets from Items
	return r
}

// --- internal item widget ---

type navBarItemWidget struct {
	BaseWidget
	item     *NavigationBarItem
	selected bool
	onTap    func()
}

func newNavBarItem(item *NavigationBarItem, selected bool, onTap func()) *navBarItemWidget {
	w := &navBarItemWidget{item: item, selected: selected, onTap: onTap}
	w.ExtendBaseWidget(w)
	return w
}

func (w *navBarItemWidget) Tapped(*event.PointEvent) {
	if w.onTap != nil {
		w.onTap()
	}
}

// AccessibleRole returns [core.RoleTab]; the bar hosts one selected destination of N.
func (w *navBarItemWidget) AccessibleRole() core.AccessibleRole { return core.RoleTab }

// AccessibleLabel returns the destination's label.
func (w *navBarItemWidget) AccessibleLabel() string { return w.item.Label }

// AccessibleState reports whether the destination is selected.
func (w *navBarItemWidget) AccessibleState() core.AccessibleState {
	if w.selected {
		return core.StateSelected
	}
	return 0
}

// AccessibleActions returns the "select" action.
func (w *navBarItemWidget) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this destination"}}
}

// PerformAccessibleAction selects the destination like a tap would.
func (w *navBarItemWidget) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		w.Tapped(nil)
	}
}

func (w *navBarItemWidget) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	indicator := canvas.NewRectangle(color.Transparent)
	icon := canvas.NewImageFromResource(w.item.Icon)
	icon.FillMode = canvas.ImageFillContain
	label := canvas.NewText(w.item.Label, color.White)
	label.Alignment = bellui.TextAlignCenter

	r := &navBarItemRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{indicator, icon, label}),
		indicator:    indicator,
		icon:         icon,
		label:        label,
		item:         w,
	}
	r.Refresh()
	return r
}

type navBarItemRenderer struct {
	internalWidget.BaseRenderer
	indicator *canvas.Rectangle
	icon      *canvas.Image
	label     *canvas.Text
	item      *navBarItemWidget
}

func (r *navBarItemRenderer) MinSize() geo.Size {
	th := r.item.Theme()
	barH := th.Size(theme.SizeNameNavBarHeight)
	return geo.NewSize(barH*0.8, barH)
}

func (r *navBarItemRenderer) Layout(size geo.Size) {
	th := r.item.Theme()
	barH := th.Size(theme.SizeNameNavBarHeight)
	iconSize := th.Size(theme.SizeNameIconSize)
	indicatorW := barH * 0.8
	indicatorH := barH * 0.4
	cx := size.Width / 2

	r.indicator.CornerRadius = indicatorH / 2
	r.indicator.Resize(geo.NewSize(indicatorW, indicatorH))
	r.indicator.Move(geo.NewPos(cx-indicatorW/2, barH*0.15))

	r.icon.Resize(geo.NewSize(iconSize, iconSize))
	r.icon.Move(geo.NewPos(cx-iconSize/2, barH*0.2))

	r.label.TextSize = th.Size(theme.SizeNameLabelMedium)
	// Give label full width and let TextAlignCenter handle centering.
	labelH := r.label.MinSize().Height
	r.label.Resize(geo.NewSize(size.Width, labelH))
	r.label.Move(geo.NewPos(0, barH*0.625))
}

func (r *navBarItemRenderer) Refresh() {
	th := r.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	// The widget may have been re-pointed at a different item by
	// navBarRenderer.updateItems, so the label text re-syncs like the icon.
	r.label.Text = r.item.item.Label

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

// --- bar renderer ---

type navBarRenderer struct {
	internalWidget.BaseRenderer
	bg    *canvas.Rectangle
	items []*navBarItemWidget
	bar   *NavigationBar
}

func (r *navBarRenderer) MinSize() geo.Size {
	th := r.bar.Theme()
	barH := th.Size(theme.SizeNameNavBarHeight)
	w := float32(0)
	for _, item := range r.items {
		w += item.MinSize().Width
	}
	return geo.NewSize(w, barH)
}

func (r *navBarRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	if len(r.items) == 0 {
		return
	}
	itemWidth := size.Width / float32(len(r.items))
	for i, item := range r.items {
		item.Move(geo.NewPos(float32(i)*itemWidth, 0))
		item.Resize(geo.NewSize(itemWidth, size.Height))
	}

	// Under RTL reflect the item row so the first item sits on the right; each
	// item centres its own icon/label so its internals need no mirroring.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *navBarRenderer) Refresh() {
	r.updateItems()
	th := r.bar.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerMedium, v)
	r.bg.Refresh()

	for i, item := range r.items {
		item.selected = i == r.bar.Selected
		item.Refresh()
	}
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar.super())
}

// updateItems re-syncs the drawn item widgets with the bar's current Items,
// the radioGroupRenderer.updateItems pattern (#203, #234): Items is a public
// slice, so it can be appended to, replaced or truncated at any time.
// Surviving widgets are re-pointed at the live item; the tap closures capture
// their index, so a widget never has to move within the row.
func (r *navBarRenderer) updateItems() {
	if len(r.items) > len(r.bar.Items) {
		clear(r.items[len(r.bar.Items):]) // drop the tail's references too
		r.items = r.items[:len(r.bar.Items)]
	}
	for i, item := range r.bar.Items {
		if i == len(r.items) {
			bar, idx := r.bar, i
			w := newNavBarItem(item, idx == bar.Selected, func() {
				bar.Selected = idx
				if bar.OnChanged != nil {
					bar.OnChanged(idx)
				}
				bar.Refresh()
			})
			r.items = append(r.items, w)
			continue
		}
		r.items[i].item = item
	}

	objects := make([]core.CanvasObject, 0, len(r.items)+1)
	objects = append(objects, r.bg)
	for _, item := range r.items {
		objects = append(objects, item)
	}
	r.SetObjects(objects)
}
