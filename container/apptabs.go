package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Declare conformity with Widget interface.
var (
	_ core.Widget     = (*AppTabs)(nil)
	_ core.Accessible = (*AppTabs)(nil)
)

// AccessibleRole returns [core.RoleTabList].
func (t *AppTabs) AccessibleRole() core.AccessibleRole { return core.RoleTabList }

// AccessibleLabel returns an empty string; tab containers have no intrinsic label.
func (t *AppTabs) AccessibleLabel() string { return "" }

// AppTabs container is used to split your application into various different areas identified by tabs.
// The tabs contain text and/or an icon and allow the user to switch between the content specified in each TabItem.
// Each item is represented by a button at the edge of the container.
//
// Since: 1.0
type AppTabs struct {
	widget.BaseWidget

	Items []*TabItem

	OnSelected   func(*TabItem) `json:"-"`
	OnUnselected func(*TabItem) `json:"-"`

	// DragToSelect lets the user drag along the tab titles to move the
	// selection indicator. The indicator follows the pointer, and on release
	// the nearest enabled tab is selected. Tapping a title still selects it.
	// Call Refresh after changing it.
	//
	// Since: 1.3
	DragToSelect bool

	current         int
	location        TabLocation
	isTransitioning bool

	popUpMenu *widget.PopUpMenu
}

// NewAppTabs creates a new tab container that allows the user to choose between different areas of an app.
//
// Since: 1.0
func NewAppTabs(items ...*TabItem) *AppTabs {
	tabs := &AppTabs{Items: items}
	tabs.BaseWidget.ExtendBaseWidget(tabs)
	return tabs
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (t *AppTabs) CreateRenderer() core.WidgetRenderer {
	t.BaseWidget.ExtendBaseWidget(t)
	th := t.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r := &appTabsRenderer{
		baseTabsRenderer: baseTabsRenderer{
			bar:       &bellui.Container{},
			divider:   canvas.NewRectangle(th.Color(theme.ColorNameShadow, v)),
			indicator: canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v)),
		},
		appTabs: t,
	}
	r.action = r.buildOverflowTabsButton()
	r.dragSurface = newTabDragSurface(r)
	r.tabs = t

	// Initially setup the tab bar to only show one tab, all others will be in overflow.
	// When the widget is laid out, and we know the size, the tab bar will be updated to show as many as can fit.
	r.updateTabs(1)
	r.updateIndicator(false)
	r.applyTheme(t)
	return r
}

// Append adds a new TabItem to the end of the tab bar.
func (t *AppTabs) Append(item *TabItem) {
	t.SetItems(append(t.Items, item))
}

// DisableIndex disables the TabItem at the specified index.
//
// Since: 1.0
func (t *AppTabs) DisableIndex(i int) {
	disableIndex(t, i)
}

// DisableItem disables the specified TabItem.
//
// Since: 1.0
func (t *AppTabs) DisableItem(item *TabItem) {
	disableItem(t, item)
}

// EnableIndex enables the TabItem at the specified index.
//
// Since: 1.0
func (t *AppTabs) EnableIndex(i int) {
	enableIndex(t, i)
}

// EnableItem enables the specified TabItem.
//
// Since: 1.0
func (t *AppTabs) EnableItem(item *TabItem) {
	enableItem(t, item)
}

// Hide hides the widget.
func (t *AppTabs) Hide() {
	if t.popUpMenu != nil {
		t.popUpMenu.Hide()
		t.popUpMenu = nil
	}
	t.BaseWidget.Hide()
}

// MinSize returns the size that this widget should not shrink below
func (t *AppTabs) MinSize() geo.Size {
	t.BaseWidget.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

// Remove tab by value.
func (t *AppTabs) Remove(item *TabItem) {
	removeItem(t, item)
	t.Refresh()
}

// RemoveIndex removes tab by index.
func (t *AppTabs) RemoveIndex(index int) {
	removeIndex(t, index)
	t.Refresh()
}

// Select sets the specified TabItem to be selected and its content visible.
func (t *AppTabs) Select(item *TabItem) {
	selectItem(t, item)
}

// SelectIndex sets the TabItem at the specific index to be selected and its content visible.
func (t *AppTabs) SelectIndex(index int) {
	selectIndex(t, index)
}

// Selected returns the currently selected TabItem.
func (t *AppTabs) Selected() *TabItem {
	return selected(t)
}

// SelectedIndex returns the index of the currently selected TabItem.
func (t *AppTabs) SelectedIndex() int {
	return t.selected()
}

// SetItems sets the containers items and refreshes.
func (t *AppTabs) SetItems(items []*TabItem) {
	setItems(t, items)
	t.Refresh()
}

// SetTabLocation sets the location of the tab bar
func (t *AppTabs) SetTabLocation(l TabLocation) {
	t.location = tabsAdjustedLocation(l, t)
	t.Refresh()
}

// Show this widget, if it was previously hidden
func (t *AppTabs) Show() {
	t.BaseWidget.Show()
	t.SelectIndex(t.current)
}

func (t *AppTabs) onUnselected() func(*TabItem) {
	return t.OnUnselected
}

func (t *AppTabs) onSelected() func(*TabItem) {
	return func(tab *TabItem) {
		if f := t.OnSelected; f != nil {
			f(tab)
		}
	}
}

func (t *AppTabs) items() []*TabItem {
	return t.Items
}

func (t *AppTabs) selected() int {
	if len(t.Items) == 0 {
		return -1
	}
	return t.current
}

func (t *AppTabs) setItems(items []*TabItem) {
	t.Items = items
}

func (t *AppTabs) setSelected(selected int) {
	t.current = selected
}

func (t *AppTabs) setTransitioning(transitioning bool) {
	t.isTransitioning = transitioning
}

func (t *AppTabs) tabLocation() TabLocation {
	return t.location
}

func (t *AppTabs) transitioning() bool {
	return t.isTransitioning
}

// Declare conformity with WidgetRenderer interface.
var _ core.WidgetRenderer = (*appTabsRenderer)(nil)

type appTabsRenderer struct {
	baseTabsRenderer
	appTabs *AppTabs

	dragSurface *tabDragSurface
	drag        tabDrag
}

func (r *appTabsRenderer) Layout(size geo.Size) {
	// Try render as many tabs as will fit, others will appear in the overflow
	if len(r.appTabs.Items) == 0 {
		r.updateTabs(0)
	} else {
		for i := len(r.appTabs.Items); i > 0; i-- {
			r.updateTabs(i)
			barMin := r.bar.MinSize()
			if r.appTabs.location == TabLocationLeading || r.appTabs.location == TabLocationTrailing {
				if barMin.Height <= size.Height {
					// Tab bar is short enough to fit
					break
				}
			} else {
				if barMin.Width <= size.Width {
					// Tab bar is thin enough to fit
					break
				}
			}
		}
	}

	r.layout(r.appTabs, size)
	r.layoutDragSurface()
	if r.drag.active {
		r.placeDragIndicator()
	} else {
		r.updateIndicator(r.appTabs.transitioning())
	}
	if r.appTabs.transitioning() {
		r.appTabs.setTransitioning(false)
	}
}

func (r *appTabsRenderer) MinSize() geo.Size {
	return r.minSize(r.appTabs)
}

func (r *appTabsRenderer) Objects() []core.CanvasObject {
	objects := r.objects(r.appTabs)
	if r.appTabs.DragToSelect {
		// First, so it is under the tab buttons: a tap passes it to the button.
		objects = append([]core.CanvasObject{r.dragSurface}, objects...)
	}
	return objects
}

func (r *appTabsRenderer) Refresh() {
	r.Layout(r.appTabs.Size())

	r.refresh(r.appTabs)

	canvas.Refresh(r.appTabs)
}

func (r *appTabsRenderer) buildOverflowTabsButton() (overflow *widget.Button) {
	overflow = &widget.Button{Icon: moreIcon(r.appTabs), Importance: widget.LowImportance, OnTapped: func() {
		// Show pop up containing all tabs which did not fit in the tab bar

		itemLen, objLen := len(r.appTabs.Items), len(r.bar.Objects[0].(*bellui.Container).Objects)
		items := make([]*bellui.MenuItem, 0, itemLen-objLen)
		for i := objLen; i < itemLen; i++ {
			index := i // capture
			// FIXME MenuItem doesn't support icons (pre-rebrand tracker #1752)
			// FIXME MenuItem can't show if it is the currently selected tab (pre-rebrand tracker #1753)
			ti := r.appTabs.Items[i]
			mi := bellui.NewMenuItem(ti.Text, func() {
				r.appTabs.SelectIndex(index)
				if r.appTabs.popUpMenu != nil {
					r.appTabs.popUpMenu.Hide()
					r.appTabs.popUpMenu = nil
				}
			})
			if ti.Disabled() {
				mi.Disabled = true
			}
			items = append(items, mi)
		}

		r.appTabs.popUpMenu = buildPopUpMenu(r.appTabs, overflow, items)
	}}

	// Extend before the renderer Hide()/Show()/Refresh()/SetIcon()s it during setup —
	// otherwise those calls hit BaseWidget.super() with a nil impl and log a spurious
	// "ExtendBaseWidget not called" warning (pre-rebrand tracker #388). Mirrors the tab/close buttons.
	overflow.ExtendBaseWidget(overflow)
	return overflow
}

func (r *appTabsRenderer) buildTabButtons(count int) *bellui.Container {
	buttons := &bellui.Container{}

	var iconPos buttonIconPosition
	if isMobile(r.tabs) {
		cells := count
		if cells == 0 {
			cells = 1
		}
		if r.appTabs.location == TabLocationTop || r.appTabs.location == TabLocationBottom {
			buttons.Layout = layout.NewGridLayoutWithColumns(cells)
		} else {
			buttons.Layout = layout.NewGridLayoutWithRows(cells)
		}
		iconPos = buttonIconTop
	} else if r.appTabs.location == TabLocationLeading || r.appTabs.location == TabLocationTrailing {
		buttons.Layout = layout.NewVBoxLayout()
		iconPos = buttonIconTop
	} else {
		buttons.Layout = layout.NewHBoxLayout()
		iconPos = buttonIconInline
	}

	for i := 0; i < count; i++ {
		item := r.appTabs.Items[i]
		if item.button == nil {
			item.button = &tabButton{
				onTapped: func() { r.appTabs.Select(item) },
				tabs:     r.tabs,
			}
			item.button.ExtendBaseWidget(item.button)
			if item.disabled {
				item.button.Disable()
			}
		}
		button := item.button
		button.icon = item.Icon
		button.iconPosition = iconPos
		if i == r.appTabs.current {
			button.importance = widget.HighImportance
		} else {
			button.importance = widget.MediumImportance
		}
		button.text = item.Text
		button.textAlignment = bellui.TextAlignCenter
		button.Refresh()
		buttons.Objects = append(buttons.Objects, button)
	}
	return buttons
}

func (r *appTabsRenderer) updateIndicator(animate bool) {
	if len(r.appTabs.Items) == 0 || r.appTabs.current < 0 {
		r.indicator.Hide()
		return
	}
	r.indicator.Show()

	var selectedPos geo.Position
	var selectedSize geo.Size

	// Positions are taken through the bar and the buttons' container: in a
	// right-to-left layout the overflow button comes before the tab buttons.
	buttons := r.bar.Objects[0].(*bellui.Container)
	if r.appTabs.current >= len(buttons.Objects) {
		if a := r.action; a != nil {
			selectedPos = r.bar.Position().Add(a.Position())
			selectedSize = a.Size()
		}
	} else {
		selected := buttons.Objects[r.appTabs.current]
		selectedPos = r.bar.Position().Add(buttons.Position()).Add(selected.Position())
		selectedSize = selected.Size()
	}

	start, length := selectedPos.X, selectedSize.Width
	if r.verticalBar() {
		start, length = selectedPos.Y, selectedSize.Height
	}
	indicatorPos, indicatorSize := r.indicatorRect(start, length)
	r.moveIndicator(indicatorPos, indicatorSize, r.appTabs.Theme(), animate)
}

// indicatorRect places the indicator along the tab bar: start and length are
// measured along the bar's axis, and the indicator sits on the content side.
func (r *appTabsRenderer) indicatorRect(start, length float32) (geo.Position, geo.Size) {
	pad := r.appTabs.Theme().Size(theme.SizeNamePadding)
	switch r.appTabs.location {
	case TabLocationLeading:
		return geo.NewPos(r.bar.MinSize().Width, start), geo.NewSize(pad, length)
	case TabLocationBottom:
		return geo.NewPos(start, r.bar.Position().Y-pad), geo.NewSize(length, pad)
	case TabLocationTrailing:
		return geo.NewPos(r.bar.Position().X-pad, start), geo.NewSize(pad, length)
	default: // TabLocationTop
		return geo.NewPos(start, r.bar.MinSize().Height), geo.NewSize(length, pad)
	}
}

func (r *appTabsRenderer) verticalBar() bool {
	return r.appTabs.location == TabLocationLeading || r.appTabs.location == TabLocationTrailing
}

func (r *appTabsRenderer) updateTabs(max int) {
	tabCount := len(r.appTabs.Items)

	// Set overflow action
	if tabCount <= max {
		r.action.Hide()
		r.bar.Layout = layout.NewStackLayout()
	} else {
		tabCount = max
		r.action.Show()

		// Set layout of tab bar containing tab buttons and overflow action
		if r.appTabs.location == TabLocationLeading || r.appTabs.location == TabLocationTrailing {
			r.bar.Layout = layout.NewBorderLayout(nil, r.action, nil, nil)
		} else {
			r.bar.Layout = layout.NewBorderLayout(nil, nil, nil, r.action)
		}
	}

	buttons := r.buildTabButtons(tabCount)

	r.bar.Objects = []core.CanvasObject{buttons}
	if a := r.action; a != nil {
		r.bar.Objects = append(r.bar.Objects, a)
	}

	r.bar.Refresh()
}
