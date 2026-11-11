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

// AccessibleRole returns [core.RoleTabList], or [core.RoleNone] while
// HideTabBar is set: with no tabs shown, the widget is only its page.
func (t *AppTabs) AccessibleRole() core.AccessibleRole {
	if t.HideTabBar {
		return core.RoleNone
	}
	return core.RoleTabList
}

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

	// HideTabBar hides the tab bar, its divider and the selection
	// indicator, so the selected item's content fills the whole widget.
	// The items and the selection stay as they are; turning it off puts
	// the bar back. Call Refresh after changing it.
	//
	// Since: 1.3
	HideTabBar bool

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

	// buttons holds the tab buttons shown in the bar. It is kept across
	// updates, and the bar is refreshed only when barState changes, so a
	// Layout or Refresh with the same tabs adds nothing to the render tree
	// (#1059).
	buttons  *bellui.Container
	barState tabBarState
	// buttonsChanged records that a button's text, icon or state changed
	// since the bar was last laid out.
	buttonsChanged bool
}

// tabBarState is what decides the tab bar's layouts and objects.
type tabBarState struct {
	shown    int // tab buttons in the bar; the rest are in the overflow menu
	overflow bool
	mobile   bool
	location TabLocation
}

func (r *appTabsRenderer) Layout(size geo.Size) {
	if r.appTabs.HideTabBar {
		r.layoutBarless(size)
		return
	}
	// Try render as many tabs as will fit, others will appear in the overflow
	r.updateTabs(r.fittingTabs(size))

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
	if r.appTabs.HideTabBar {
		min := geo.NewSize(0, 0)
		for _, ti := range r.appTabs.Items {
			min = min.Max(ti.Content.MinSize())
		}
		return min
	}
	return r.minSize(r.appTabs)
}

// layoutBarless lays out the tabs while HideTabBar is set: the selected
// content fills the widget and the others are hidden, as in layout.
func (r *appTabsRenderer) layoutBarless(size geo.Size) {
	selected := r.appTabs.selected()
	for i, ti := range r.appTabs.Items {
		if i == selected {
			ti.Content.Move(geo.NewPos(0, 0))
			ti.Content.Resize(size)
			ti.Content.Show()
		} else {
			ti.Content.Hide()
		}
	}
	if r.appTabs.transitioning() {
		r.appTabs.setTransitioning(false)
	}
}

func (r *appTabsRenderer) Objects() []core.CanvasObject {
	if r.appTabs.HideTabBar {
		// Only the selected content: the bar, divider and indicator are
		// neither drawn nor tappable.
		if i, is := r.appTabs.selected(), r.appTabs.Items; i >= 0 && i < len(is) {
			return []core.CanvasObject{is[i].Content}
		}
		return nil
	}
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

// buttonsLayout lays out count tab buttons.
func (r *appTabsRenderer) buttonsLayout(count int, mobile bool) core.Layout {
	if mobile {
		cells := count
		if cells == 0 {
			cells = 1
		}
		if r.appTabs.location == TabLocationTop || r.appTabs.location == TabLocationBottom {
			return layout.NewGridLayoutWithColumns(cells)
		}
		return layout.NewGridLayoutWithRows(cells)
	} else if r.verticalBar() {
		return layout.NewVBoxLayout()
	}
	return layout.NewHBoxLayout()
}

// barLayout lays out the bar: the tab buttons alone, or with the overflow
// button after them.
func (r *appTabsRenderer) barLayout(overflow bool, action core.CanvasObject) core.Layout {
	if !overflow {
		return layout.NewStackLayout()
	} else if r.verticalBar() {
		return layout.NewBorderLayout(nil, action, nil, nil)
	}
	return layout.NewBorderLayout(nil, nil, nil, action)
}

// updateTabButtons brings every tab's button up to date with its item, and
// refreshes only the buttons that changed.
func (r *appTabsRenderer) updateTabButtons(mobile bool) {
	iconPos := buttonIconInline
	if mobile || r.verticalBar() {
		iconPos = buttonIconTop
	}

	for i, item := range r.appTabs.Items {
		changed := false
		if item.button == nil {
			item.button = &tabButton{
				onTapped: func() { r.appTabs.Select(item) },
				tabs:     r.tabs,
			}
			item.button.ExtendBaseWidget(item.button)
			if item.disabled {
				item.button.Disable()
			}
			changed = true
		}
		button := item.button
		importance := widget.MediumImportance
		if i == r.appTabs.current {
			importance = widget.HighImportance
		}
		if button.icon != item.Icon || button.iconPosition != iconPos || button.importance != importance ||
			button.text != item.Text || button.textAlignment != bellui.TextAlignCenter {
			button.icon = item.Icon
			button.iconPosition = iconPos
			button.importance = importance
			button.text = item.Text
			button.textAlignment = bellui.TextAlignCenter
			changed = true
		}
		if changed {
			button.Refresh()
			r.buttonsChanged = true
		}
	}
}

// minSizeStandIn measures as min in a layout. Unlike a rectangle's SetMinSize,
// setting min does not count as a layout change.
type minSizeStandIn struct {
	core.CanvasObject
	min geo.Size
}

func (s *minSizeStandIn) MinSize() geo.Size { return s.min }

// fittingTabs returns how many tab buttons fit in the bar at size, the others
// going to the overflow menu, and at least one when there are tabs. It
// measures with the bar's own layouts and stand-ins, leaving the bar and the
// overflow button as they are: showing or hiding a widget costs a full layout
// walk (#1059).
func (r *appTabsRenderer) fittingTabs(size geo.Size) int {
	count := len(r.appTabs.Items)
	if count == 0 {
		return 0
	}
	mobile := isMobile(r.tabs)
	r.updateTabButtons(mobile)

	all := make([]core.CanvasObject, count)
	for i, item := range r.appTabs.Items {
		all[i] = item.button
	}
	tabs := &minSizeStandIn{CanvasObject: canvas.NewRectangle(nil)}
	action := &minSizeStandIn{CanvasObject: canvas.NewRectangle(nil)}
	if r.action != nil {
		action.min = r.action.MinSize()
	}
	for i := count; i > 1; i-- {
		tabs.min = r.buttonsLayout(i, mobile).MinSize(all[:i])
		barMin := tabs.min
		if i < count {
			barMin = r.barLayout(true, action).MinSize([]core.CanvasObject{tabs, action})
		}
		if r.verticalBar() {
			if barMin.Height <= size.Height {
				return i // Tab bar is short enough to fit
			}
		} else if barMin.Width <= size.Width {
			return i // Tab bar is thin enough to fit
		}
	}
	return 1
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

// updateTabs shows the first max tab buttons in the bar and the others in the
// overflow menu. The bar is refreshed only when what it holds changes, and
// otherwise laid out again only when a button changed.
func (r *appTabsRenderer) updateTabs(max int) {
	mobile := isMobile(r.tabs)
	r.updateTabButtons(mobile)

	state := tabBarState{
		shown:    min(max, len(r.appTabs.Items)),
		overflow: max < len(r.appTabs.Items),
		mobile:   mobile,
		location: r.appTabs.location,
	}

	// Set overflow action
	if state.overflow {
		r.action.Show()
	} else {
		r.action.Hide()
	}

	structural := r.buttons == nil || state != r.barState ||
		len(r.buttons.Objects) != state.shown
	if r.buttons == nil {
		r.buttons = &bellui.Container{}
		r.bar.Objects = []core.CanvasObject{r.buttons}
		if a := r.action; a != nil {
			r.bar.Objects = append(r.bar.Objects, a)
		}
	}
	if !structural {
		for i, o := range r.buttons.Objects {
			if o != r.appTabs.Items[i].button {
				structural = true
				break
			}
		}
	}

	if structural {
		r.barState = state
		r.buttons.Layout = r.buttonsLayout(state.shown, mobile)
		r.bar.Layout = r.barLayout(state.overflow, r.action)
		r.buttons.Objects = make([]core.CanvasObject, 0, state.shown)
		for _, item := range r.appTabs.Items[:state.shown] {
			r.buttons.Objects = append(r.buttons.Objects, item.button)
		}
		r.buttonsChanged = false
		r.bar.Refresh()
	} else if r.buttonsChanged {
		r.buttonsChanged = false
		r.bar.Layout.Layout(r.bar.Objects, r.bar.Size())
		r.buttons.Layout.Layout(r.buttons.Objects, r.buttons.Size())
	}
}
