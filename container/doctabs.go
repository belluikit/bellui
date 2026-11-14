package container

import (
	"image/color"

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
	_ core.Widget     = (*DocTabs)(nil)
	_ core.Accessible = (*DocTabs)(nil)
)

// AccessibleRole returns [core.RoleTabList].
func (t *DocTabs) AccessibleRole() core.AccessibleRole { return core.RoleTabList }

// AccessibleLabel returns an empty string; tab containers have no intrinsic label.
func (t *DocTabs) AccessibleLabel() string { return "" }

// DocTabs container is used to display various pieces of content identified by tabs.
// The tabs contain text and/or an icon and allow the user to switch between the content specified in each TabItem.
// Each item is represented by a button at the edge of the container.
//
// Since: 1.0
type DocTabs struct {
	widget.BaseWidget

	Items []*TabItem

	CreateTab      func() *TabItem `json:"-"`
	CloseIntercept func(*TabItem)  `json:"-"`
	OnClosed       func(*TabItem)  `json:"-"`
	OnSelected     func(*TabItem)  `json:"-"`
	OnUnselected   func(*TabItem)  `json:"-"`

	current         int
	location        TabLocation
	isTransitioning bool

	popUpMenu *widget.PopUpMenu
}

// NewDocTabs creates a new tab container that allows the user to choose between various pieces of content.
//
// Since: 1.0
func NewDocTabs(items ...*TabItem) *DocTabs {
	tabs := &DocTabs{Items: items}
	tabs.ExtendBaseWidget(tabs)
	return tabs
}

// Append adds a new TabItem to the end of the tab bar.
func (t *DocTabs) Append(item *TabItem) {
	t.SetItems(append(t.Items, item))
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (t *DocTabs) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	th := t.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r := &docTabsRenderer{
		baseTabsRenderer: baseTabsRenderer{
			bar:       &bellui.Container{},
			divider:   canvas.NewRectangle(th.Color(theme.ColorNameShadow, v)),
			indicator: canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v)),
		},
		docTabs:  t,
		scroller: NewScroll(&bellui.Container{}),
	}
	r.action = r.buildAllTabsButton()
	r.create = r.buildCreateTabsButton()
	r.tabs = t

	r.box = NewHBox(r.create, r.action)
	r.scroller.OnScrolled = func(offset geo.Position) {
		r.updateIndicator(false)
	}
	r.updateAllTabs()
	r.updateCreateTab()
	r.updateTabs()
	r.updateIndicator(false)
	r.applyTheme(t)
	return r
}

// DisableIndex disables the TabItem at the specified index.
//
// Since: 1.0
func (t *DocTabs) DisableIndex(i int) {
	disableIndex(t, i)
}

// DisableItem disables the specified TabItem.
//
// Since: 1.0
func (t *DocTabs) DisableItem(item *TabItem) {
	disableItem(t, item)
}

// EnableIndex enables the TabItem at the specified index.
//
// Since: 1.0
func (t *DocTabs) EnableIndex(i int) {
	enableIndex(t, i)
}

// EnableItem enables the specified TabItem.
//
// Since: 1.0
func (t *DocTabs) EnableItem(item *TabItem) {
	enableItem(t, item)
}

// Hide hides the widget.
func (t *DocTabs) Hide() {
	if t.popUpMenu != nil {
		t.popUpMenu.Hide()
		t.popUpMenu = nil
	}
	t.BaseWidget.Hide()
}

// MinSize returns the size that this widget should not shrink below
func (t *DocTabs) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

// Remove tab by value.
// Removing the selected tab selects the tab after it, or the one before when
// it was last, and reports both through OnUnselected and OnSelected.
func (t *DocTabs) Remove(item *TabItem) {
	removeItem(t, item)
	t.Refresh()
}

// RemoveIndex removes tab by index.
// Removing the selected tab selects the tab after it, or the one before when
// it was last, and reports both through OnUnselected and OnSelected.
func (t *DocTabs) RemoveIndex(index int) {
	removeIndex(t, index)
	t.Refresh()
}

// Select sets the specified TabItem to be selected and its content visible.
func (t *DocTabs) Select(item *TabItem) {
	selectItem(t, item)
	t.Refresh()
}

// SelectIndex sets the TabItem at the specific index to be selected and its content visible.
func (t *DocTabs) SelectIndex(index int) {
	selectIndex(t, index)
}

// Selected returns the currently selected TabItem.
func (t *DocTabs) Selected() *TabItem {
	return selected(t)
}

// SelectedIndex returns the index of the currently selected TabItem.
func (t *DocTabs) SelectedIndex() int {
	return t.selected()
}

// SetItems sets the containers items and refreshes.
func (t *DocTabs) SetItems(items []*TabItem) {
	setItems(t, items)
	t.Refresh()
}

// SetTabLocation sets the location of the tab bar
func (t *DocTabs) SetTabLocation(l TabLocation) {
	t.location = tabsAdjustedLocation(l, t)
	t.Refresh()
}

// Show this widget, if it was previously hidden
func (t *DocTabs) Show() {
	t.BaseWidget.Show()
	t.SelectIndex(t.current)
}

func (t *DocTabs) close(item *TabItem) {
	if f := t.CloseIntercept; f != nil {
		f(item)
	} else {
		t.Remove(item)
		if f := t.OnClosed; f != nil {
			f(item)
		}
	}
}

func (t *DocTabs) onUnselected() func(*TabItem) {
	return t.OnUnselected
}

func (t *DocTabs) onSelected() func(*TabItem) {
	return t.OnSelected
}

func (t *DocTabs) items() []*TabItem {
	return t.Items
}

func (t *DocTabs) selected() int {
	if len(t.Items) == 0 {
		return -1
	}
	return t.current
}

func (t *DocTabs) setItems(items []*TabItem) {
	t.Items = items
}

func (t *DocTabs) setSelected(selected int) {
	t.current = selected
}

func (t *DocTabs) setTransitioning(transitioning bool) {
	t.isTransitioning = transitioning
}

func (t *DocTabs) tabLocation() TabLocation {
	return t.location
}

func (t *DocTabs) transitioning() bool {
	return t.isTransitioning
}

// Declare conformity with WidgetRenderer interface.
var _ core.WidgetRenderer = (*docTabsRenderer)(nil)

type docTabsRenderer struct {
	baseTabsRenderer
	docTabs      *DocTabs
	scroller     *Scroll
	box          *bellui.Container
	create       *widget.Button
	lastSelected int

	// barLocation is the location the bar was last built for, and barBuilt
	// whether it has been. The bar is refreshed only when it or the buttons
	// shown change, so a Layout with the same tabs refreshes nothing (#1059).
	barLocation TabLocation
	barBuilt    bool
	// buttonsChanged records that a button's text, icon or state changed
	// since the bar was last laid out.
	buttonsChanged bool
}

func (r *docTabsRenderer) Layout(size geo.Size) {
	r.updateAllTabs()
	r.updateCreateTab()
	r.updateTabs()
	r.layout(r.docTabs, size)

	// lay out buttons before updating indicator, which is relative to their position
	buttons := r.scroller.Content.(*bellui.Container)
	buttons.Layout.Layout(buttons.Objects, buttons.Size())
	r.updateIndicator(r.docTabs.transitioning())

	if r.docTabs.transitioning() {
		r.docTabs.setTransitioning(false)
	}
}

func (r *docTabsRenderer) MinSize() geo.Size {
	return r.minSize(r.docTabs)
}

func (r *docTabsRenderer) Objects() []core.CanvasObject {
	return r.objects(r.docTabs)
}

func (r *docTabsRenderer) Refresh() {
	r.Layout(r.docTabs.Size())

	if c := r.docTabs.current; c != r.lastSelected {
		if c >= 0 && c < len(r.docTabs.Items) {
			r.scrollToSelected()
		}
		r.lastSelected = c
	}

	r.refresh(r.docTabs)

	canvas.Refresh(r.docTabs)
}

func (r *docTabsRenderer) buildAllTabsButton() (all *widget.Button) {
	all = &widget.Button{Importance: widget.LowImportance, OnTapped: func() {
		// Show pop up containing all tabs

		items := make([]*bellui.MenuItem, len(r.docTabs.Items))
		for i := 0; i < len(r.docTabs.Items); i++ {
			index := i // capture
			// FIXME MenuItem doesn't support icons (pre-rebrand tracker #1752)
			items[i] = bellui.NewMenuItem(r.docTabs.Items[i].Text, func() {
				r.docTabs.SelectIndex(index)
				if r.docTabs.popUpMenu != nil {
					r.docTabs.popUpMenu.Hide()
					r.docTabs.popUpMenu = nil
				}
			})
			items[i].Checked = index == r.docTabs.current
		}

		r.docTabs.popUpMenu = buildPopUpMenu(r.docTabs, all, items)
	}}

	// Extend before the renderer Hide()/Show()/Refresh()/SetIcon()s it during setup —
	// otherwise those calls hit BaseWidget.super() with a nil impl and log a spurious
	// "ExtendBaseWidget not called" warning (pre-rebrand tracker #388). Mirrors the tab/close buttons.
	all.ExtendBaseWidget(all)
	return all
}

func (r *docTabsRenderer) buildCreateTabsButton() *widget.Button {
	create := widget.NewButton("", func() {
		if f := r.docTabs.CreateTab; f != nil {
			if tab := f(); tab != nil {
				r.docTabs.Append(tab)
				r.docTabs.SelectIndex(len(r.docTabs.Items) - 1)
			}
		}
	})
	create.Importance = widget.LowImportance
	return create
}

// buildTabButtons brings the first count tab buttons up to date with their
// items, refreshing only the buttons that changed. It reports whether the
// buttons shown, or their layout, changed.
func (r *docTabsRenderer) buildTabButtons(count int, buttons *bellui.Container) (structural bool) {
	vertical := r.docTabs.location == TabLocationLeading || r.docTabs.location == TabLocationTrailing
	iconPos := buttonIconInline
	if vertical {
		iconPos = buttonIconTop
	}
	if !r.barBuilt || r.barLocation != r.docTabs.location {
		if vertical {
			buttons.Layout = layout.NewVBoxLayout()
		} else {
			buttons.Layout = layout.NewHBoxLayout()
		}
		structural = true
	}
	structural = structural || len(buttons.Objects) != count

	for i := 0; i < count; i++ {
		item := r.docTabs.Items[i]
		changed := false
		if item.button == nil {
			item.button = &tabButton{
				onTapped: func() { r.docTabs.Select(item) },
				onClosed: func() { r.docTabs.close(item) },
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
		if i == r.docTabs.current {
			importance = widget.HighImportance
		}
		if button.icon != item.Icon || button.iconPosition != iconPos || button.importance != importance ||
			button.text != item.Text || button.textAlignment != bellui.TextAlignLeading {
			button.icon = item.Icon
			button.iconPosition = iconPos
			button.importance = importance
			button.text = item.Text
			button.textAlignment = bellui.TextAlignLeading
			changed = true
		}
		if changed {
			button.Refresh()
			r.buttonsChanged = true
		}
		structural = structural || buttons.Objects[i] != button
	}

	if structural {
		buttons.Objects = make([]core.CanvasObject, 0, count)
		for _, item := range r.docTabs.Items[:count] {
			buttons.Objects = append(buttons.Objects, item.button)
		}
	}
	return structural
}

func (r *docTabsRenderer) scrollToSelected() {
	buttons := r.scroller.Content.(*bellui.Container)

	// https://github.com/fyne-io/fyne/issues/3909
	// very dirty temporary fix to this crash!
	if r.docTabs.current < 0 || r.docTabs.current >= len(buttons.Objects) {
		return
	}

	button := buttons.Objects[r.docTabs.current]
	pos := button.Position()
	size := button.Size()
	offset := r.scroller.Offset
	viewport := r.scroller.Size()
	if r.docTabs.location == TabLocationLeading || r.docTabs.location == TabLocationTrailing {
		if pos.Y < offset.Y {
			offset.Y = pos.Y
		} else if pos.Y+size.Height > offset.Y+viewport.Height {
			offset.Y = pos.Y + size.Height - viewport.Height
		}
	} else {
		if pos.X < offset.X {
			offset.X = pos.X
		} else if pos.X+size.Width > offset.X+viewport.Width {
			offset.X = pos.X + size.Width - viewport.Width
		}
	}
	r.scroller.Offset = offset
	r.updateIndicator(false)
}

func (r *docTabsRenderer) updateIndicator(animate bool) {
	th := r.docTabs.Theme()
	if r.docTabs.current < 0 {
		r.indicator.FillColor = color.Transparent
		r.moveIndicator(geo.NewPos(0, 0), geo.NewSize(0, 0), th, animate)
		return
	}

	vertical := r.docTabs.location == TabLocationLeading || r.docTabs.location == TabLocationTrailing
	along := func(p geo.Position) float32 {
		if vertical {
			return p.Y
		}
		return p.X
	}
	alongSize := func(s geo.Size) float32 {
		if vertical {
			return s.Height
		}
		return s.Width
	}

	// Measured along the bar, in the renderer's coordinates. The tab buttons
	// scroll in a viewport that starts where the scroller sits in the bar:
	// in a right-to-left layout that is after the create and overflow
	// buttons, which are mirrored to the leading edge (#1078).
	var start, length float32
	buttons := r.scroller.Content.(*bellui.Container).Objects
	if r.docTabs.current >= len(buttons) {
		a := r.action
		if a == nil {
			return
		}
		start = along(r.bar.Position().Add(r.box.Position()).Add(a.Position()))
		size := a.Size()
		if minSize := a.MinSize(); minSize.Width > size.Width {
			size = minSize
		}
		length = alongSize(size)
	} else {
		selected := buttons[r.docTabs.current]
		viewStart := along(r.bar.Position().Add(r.scroller.Position()))
		viewEnd := viewStart + alongSize(r.scroller.Size())
		start = viewStart + along(selected.Position()) - along(r.scroller.Offset)
		size := selected.Size()
		if minSize := selected.MinSize(); minSize.Width > size.Width {
			size = minSize
		}
		end := geo.Min(start+alongSize(size), viewEnd)
		start = geo.Max(start, viewStart)
		length = end - start
	}
	if length < 0 { // the selected tab is scrolled out of view
		r.indicator.FillColor = color.Transparent
		r.indicator.Refresh()
		return
	}

	pad := th.Size(theme.SizeNamePadding)
	var indicatorPos geo.Position
	var indicatorSize geo.Size
	switch r.docTabs.location {
	case TabLocationLeading:
		indicatorPos = geo.NewPos(r.bar.MinSize().Width, start)
		indicatorSize = geo.NewSize(pad, length)
	case TabLocationBottom:
		indicatorPos = geo.NewPos(start, r.bar.Position().Y-pad)
		indicatorSize = geo.NewSize(length, pad)
	case TabLocationTrailing:
		indicatorPos = geo.NewPos(r.bar.Position().X-pad, start)
		indicatorSize = geo.NewSize(pad, length)
	default: // TabLocationTop
		indicatorPos = geo.NewPos(start, r.bar.MinSize().Height)
		indicatorSize = geo.NewSize(length, pad)
	}

	r.moveIndicator(indicatorPos, indicatorSize, th, animate)
}

func (r *docTabsRenderer) updateAllTabs() {
	if len(r.docTabs.Items) > 0 {
		r.action.Show()
	} else {
		r.action.Hide()
	}
}

func (r *docTabsRenderer) updateCreateTab() {
	if r.docTabs.CreateTab != nil {
		if icon := theme.ContentAddIcon(); r.create.Icon != icon {
			r.create.SetIcon(icon) // SetIcon refreshes even when the icon is the same
		}
		r.create.Show()
	} else {
		r.create.Hide()
	}
}

func (r *docTabsRenderer) updateTabs() {
	tabCount := len(r.docTabs.Items)
	structural := r.buildTabButtons(tabCount, r.scroller.Content.(*bellui.Container))
	if !structural {
		if r.buttonsChanged {
			// The scroller re-measures and resizes the buttons without
			// refreshing them.
			r.buttonsChanged = false
			r.scroller.Refresh()
		}
		return
	}
	r.barBuilt = true
	r.barLocation = r.docTabs.location
	r.buttonsChanged = false

	// Set layout of tab bar containing tab buttons and overflow action
	if r.docTabs.location == TabLocationLeading || r.docTabs.location == TabLocationTrailing {
		r.bar.Layout = layout.NewBorderLayout(nil, r.box, nil, nil)
		r.scroller.Direction = ScrollVerticalOnly
	} else {
		r.bar.Layout = layout.NewBorderLayout(nil, nil, nil, r.box)
		r.scroller.Direction = ScrollHorizontalOnly
	}

	r.bar.Objects = []core.CanvasObject{r.scroller, r.box}
	r.bar.Refresh()
}
