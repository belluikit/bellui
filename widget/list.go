package widget

import (
	"fmt"
	"math"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// ListItemID uniquely identifies an item within a list.
type ListItemID = int

// Declare conformity with interfaces.
var (
	_ core.Widget     = (*List)(nil)
	_ event.Focusable = (*List)(nil)
	_ core.Accessible = (*List)(nil)
)

// AccessibleRole returns [core.RoleList].
func (l *List) AccessibleRole() core.AccessibleRole { return core.RoleList }

// AccessibleLabel returns an empty string; lists have no intrinsic label.
func (l *List) AccessibleLabel() string { return "" }

// List is a widget that pools list items for performance and
// lays the items out in a vertical direction inside of a scroller.
// By default, List requires that all items are the same size, but specific
// rows can have their heights set with SetItemHeight.
//
// Since: 1.0
type List struct {
	BaseWidget

	// Length is a callback for returning the number of items in the list.
	Length func() int `json:"-"`

	// CreateItem is a callback invoked to create a new widget to render
	// a row in the list.
	CreateItem func() core.CanvasObject `json:"-"`

	// UpdateItem is a callback invoked to update a list row widget
	// to display a new row in the list. The UpdateItem callback should
	// only update the given item, it should not invoke APIs that would
	// change other properties of the list itself.
	UpdateItem func(id ListItemID, item core.CanvasObject) `json:"-"`

	// OnSelected is a callback to be notified when a given item
	// in the list has been selected.
	OnSelected func(id ListItemID) `json:"-"`

	// OnUnselected is a callback to be notified when a given item
	// in the list has been unselected.
	OnUnselected func(id ListItemID) `json:"-"`

	// HideSeparators hides the separators between list rows
	//
	// Since: 1.0
	HideSeparators bool

	currentFocus  ListItemID
	focused       bool
	scroller      *widget.Scroll
	selected      []ListItemID
	itemMin       geo.Size
	itemHeights   map[ListItemID]float32
	offsetY       float32
	offsetUpdated func(geo.Position)
	minSizeCache  geo.Size
}

// NewList creates and returns a list widget for displaying items in
// a vertical layout with scrolling and caching for performance.
//
// Since: 1.0
func NewList(length func() int, createItem func() core.CanvasObject, updateItem func(ListItemID, core.CanvasObject)) *List {
	list := &List{Length: length, CreateItem: createItem, UpdateItem: updateItem}
	list.ExtendBaseWidget(list)
	return list
}

// NewListWithData creates a new list widget that will display the contents of the provided data.
//
// Since: 1.0
func NewListWithData(data binding.DataList, createItem func() core.CanvasObject, updateItem func(binding.DataItem, core.CanvasObject)) *List {
	l := NewList(
		data.Length,
		createItem,
		func(i ListItemID, o core.CanvasObject) {
			item, err := data.GetItem(i)
			if err != nil {
				bellui.LogError(fmt.Sprintf("Error getting data item %d", i), err)
				return
			}
			updateItem(item, o)
		},
	)

	data.AddListener(binding.NewDataListener(l.Refresh))
	return l
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (l *List) CreateRenderer() core.WidgetRenderer {
	l.ExtendBaseWidget(l)

	if f := l.CreateItem; f != nil && l.itemMin.IsZero() {
		l.itemMin = measureTemplate(createItemAndApplyThemeScope(f, l))
	}

	layout := &bellui.Container{Layout: newListLayout(l)}
	l.scroller = widget.NewVScroll(layout)
	layout.Resize(layout.MinSize())
	objects := []core.CanvasObject{l.scroller}
	return newListRenderer(objects, l, l.scroller, layout)
}

// FocusGained is called after this List has gained focus.
func (l *List) FocusGained() {
	l.focused = true
	l.RefreshItem(l.currentFocus)
}

// FocusLost is called after this List has lost focus.
func (l *List) FocusLost() {
	l.focused = false
	l.RefreshItem(l.currentFocus)
}

// MinSize returns the size that this widget should not shrink below.
func (l *List) MinSize() geo.Size {
	l.ExtendBaseWidget(l)
	return l.BaseWidget.MinSize()
}

// RefreshItem refreshes a single item, specified by the item ID passed in.
//
// Since: 1.0
func (l *List) RefreshItem(id ListItemID) {
	l.minSizeCache = geo.Size{}
	if l.scroller == nil {
		return
	}
	l.BaseWidget.Refresh()
	lo := l.scroller.Content.(*bellui.Container).Layout.(*listLayout)
	item, ok := lo.searchVisible(lo.visible, id)
	if ok {
		lo.setupListItem(item, id, l.focused && l.currentFocus == id)
	}
}

// focusedItemObject returns the realized listItem for the currently focused row,
// or nil when it is not currently laid out. It lets keyboard activation report
// the same item object the pointer path hit-tests (pre-rebrand tracker #834).
func (l *List) focusedItemObject() core.CanvasObject {
	if l.scroller == nil {
		return nil
	}
	cont, ok := l.scroller.Content.(*bellui.Container)
	if !ok {
		return nil
	}
	lo, ok := cont.Layout.(*listLayout)
	if !ok {
		return nil
	}
	if item, ok := lo.searchVisible(lo.visible, l.currentFocus); ok {
		return item
	}
	return nil
}

// SetItemHeight supports changing the height of the specified list item. Items normally take the height of the template
// returned from the CreateItem callback. The height parameter uses the same units as a geo.Size type and refers
// to the internal content height not including the divider size.
//
// Since: 1.0
func (l *List) SetItemHeight(id ListItemID, height float32) {
	if l.itemHeights == nil {
		l.itemHeights = make(map[ListItemID]float32)
	}

	refresh := l.itemHeights[id] != height
	l.itemHeights[id] = height

	if refresh {
		l.RefreshItem(id)
	}
}

func (l *List) scrollTo(id ListItemID) {
	if l.scroller == nil {
		return
	}

	separatorThickness := l.Theme().Size(theme.SizeNamePadding)
	y := float32(0)
	lastItemHeight := l.itemMin.Height

	if len(l.itemHeights) == 0 {
		y = (float32(id) * l.itemMin.Height) + (float32(id) * separatorThickness)
	} else {
		i := 0
		for ; i < id; i++ {
			height := l.itemMin.Height
			if h, ok := l.itemHeights[i]; ok {
				height = h
			}

			y += height + separatorThickness
		}
		lastItemHeight = l.itemMin.Height
		if h, ok := l.itemHeights[i]; ok {
			lastItemHeight = h
		}
	}
	if y < l.scroller.Offset.Y {
		l.scroller.Offset.Y = y
	} else if y+l.itemMin.Height > l.scroller.Offset.Y+l.scroller.Size().Height {
		l.scroller.Offset.Y = y + lastItemHeight - l.scroller.Size().Height
	}
	l.offsetUpdated(l.scroller.Offset)
}

// Resize is called when this list should change size. We refresh to ensure invisible items are drawn.
func (l *List) Resize(s geo.Size) {
	l.BaseWidget.Resize(s)
	if l.scroller == nil {
		return
	}

	l.offsetUpdated(l.scroller.Offset)
	l.scroller.Content.(*bellui.Container).Layout.(*listLayout).updateList(true)
}

// Select add the item identified by the given ID to the selection.
func (l *List) Select(id ListItemID) {
	if len(l.selected) > 0 && id == l.selected[0] {
		return
	}
	length := 0
	if f := l.Length; f != nil {
		length = f()
	}
	if id < 0 || id >= length {
		return
	}
	old := l.selected
	l.selected = []ListItemID{id}
	defer func() {
		if f := l.OnUnselected; f != nil && len(old) > 0 {
			f(old[0])
		}
		if f := l.OnSelected; f != nil {
			f(id)
		}
	}()
	l.scrollTo(id)
	l.Refresh()
}

// ScrollTo scrolls to the item represented by id
//
// Since: 1.0
func (l *List) ScrollTo(id ListItemID) {
	length := 0
	if f := l.Length; f != nil {
		length = f()
	}
	if id < 0 || id >= length {
		return
	}
	l.scrollTo(id)
	l.Refresh()
}

// ScrollToBottom scrolls to the end of the list
//
// Since: 1.0
func (l *List) ScrollToBottom() {
	l.scroller.ScrollToBottom()
	l.offsetUpdated(l.scroller.Offset)
}

// ScrollToTop scrolls to the start of the list
//
// Since: 1.0
func (l *List) ScrollToTop() {
	l.scroller.ScrollToTop()
	l.offsetUpdated(l.scroller.Offset)
}

// ScrollToOffset scrolls the list to the given offset position.
//
// Since: 1.0
func (l *List) ScrollToOffset(offset float32) {
	if l.scroller == nil {
		return
	}
	if offset < 0 {
		offset = 0
	}
	contentHeight := l.contentMinSize().Height
	if l.Size().Height >= contentHeight {
		return // content fully visible - no need to scroll
	}
	if offset > contentHeight {
		offset = contentHeight
	}
	l.scroller.ScrollToOffset(geo.NewPos(0, offset))
	l.offsetUpdated(l.scroller.Offset)
}

// GetScrollOffset returns the current scroll offset position
//
// Since: 1.0
func (l *List) GetScrollOffset() float32 {
	return l.offsetY
}

// TypedKey is called if a key event happens while this List is focused.
func (l *List) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeySpace:
		l.Select(l.currentFocus)
		// Keyboard activation targets the focused item; report the realized
		// item object (not the List) so autocapture sees RoleListItem, matching
		// the pointer path which hit-tests the item (pre-rebrand tracker #834, pre-rebrand tracker #700).
		if item := l.focusedItemObject(); item != nil {
			core.NotifyActivation(item, true)
		}
	case event.KeyDown:
		if f := l.Length; f != nil && l.currentFocus >= f()-1 {
			return
		}
		l.RefreshItem(l.currentFocus)
		l.currentFocus++
		l.scrollTo(l.currentFocus)
		l.RefreshItem(l.currentFocus)
	case event.KeyUp:
		if l.currentFocus <= 0 {
			return
		}
		l.RefreshItem(l.currentFocus)
		l.currentFocus--
		l.scrollTo(l.currentFocus)
		l.RefreshItem(l.currentFocus)
	}
}

// TypedRune is called if a text event happens while this List is focused.
func (l *List) TypedRune(_ rune) {
	// intentionally left blank
}

// Unselect removes the item identified by the given ID from the selection.
func (l *List) Unselect(id ListItemID) {
	if len(l.selected) == 0 || l.selected[0] != id {
		return
	}

	l.selected = nil
	l.Refresh()
	if f := l.OnUnselected; f != nil {
		f(id)
	}
}

// UnselectAll removes all items from the selection.
//
// Since: 1.0
func (l *List) UnselectAll() {
	if len(l.selected) == 0 {
		return
	}

	selected := l.selected
	l.selected = nil
	l.Refresh()
	if f := l.OnUnselected; f != nil {
		for _, id := range selected {
			f(id)
		}
	}
}

// Refresh causes this List to be redrawn in its current state
func (l *List) Refresh() {
	l.cleanItemHeights()
	l.minSizeCache = geo.Size{}
	l.BaseWidget.Refresh()
}

// cleanItemHeights removes stale entries from itemHeights for IDs that
// are no longer within the current list length.
func (l *List) cleanItemHeights() {
	if l.itemHeights == nil || l.Length == nil {
		return
	}
	length := l.Length()
	for id := range l.itemHeights {
		if id >= length {
			delete(l.itemHeights, id)
		}
	}
}

func (l *List) contentMinSize() geo.Size {
	if !l.minSizeCache.IsZero() {
		return l.minSizeCache
	}

	separatorThickness := l.Theme().Size(theme.SizeNamePadding)
	if l.Length == nil {
		return geo.NewSize(0, 0)
	}
	items := l.Length()

	if len(l.itemHeights) == 0 {
		return geo.NewSize(l.itemMin.Width,
			(l.itemMin.Height+separatorThickness)*float32(items)-separatorThickness)
	}

	height := float32(0)
	totalCustom := 0
	templateHeight := l.itemMin.Height
	for id, itemHeight := range l.itemHeights {
		if id < items {
			totalCustom++
			height += itemHeight
		}
	}
	height += float32(items-totalCustom) * templateHeight

	size := geo.NewSize(l.itemMin.Width, height+separatorThickness*float32(items-1))
	l.minSizeCache = size
	return size
}

// fills l.visibleRowHeights and also returns offY and minRow
func (l *listLayout) calculateVisibleRowHeights(itemHeight float32, length int, th bellui.Theme) (offY float32, minRow int) {
	rowOffset := float32(0)
	isVisible := false
	l.visibleRowHeights = l.visibleRowHeights[:0]

	if l.list.scroller.Size().Height <= 0 {
		return offY, minRow
	}

	padding := th.Size(theme.SizeNamePadding)

	if len(l.list.itemHeights) == 0 {
		paddedItemHeight := itemHeight + padding

		offY = float32(math.Floor(float64(l.list.offsetY/paddedItemHeight))) * paddedItemHeight
		minRow = int(math.Floor(float64(offY / paddedItemHeight)))
		maxRow := int(math.Ceil(float64((offY + l.list.scroller.Size().Height) / paddedItemHeight)))

		if minRow > length-1 {
			minRow = length - 1
		}
		if minRow < 0 {
			minRow = 0
			offY = 0
		}

		if maxRow > length-1 {
			maxRow = length - 1
		}

		for i := 0; i <= maxRow-minRow; i++ {
			l.visibleRowHeights = append(l.visibleRowHeights, itemHeight)
		}
		return offY, minRow
	}

	for i := 0; i < length; i++ {
		height := itemHeight
		if h, ok := l.list.itemHeights[i]; ok {
			height = h
		}

		if rowOffset <= l.list.offsetY-height-padding {
			// before scroll
		} else if rowOffset <= l.list.offsetY {
			minRow = i
			offY = rowOffset
			isVisible = true
		}
		if rowOffset >= l.list.offsetY+l.list.scroller.Size().Height {
			break
		}

		rowOffset += height + padding
		if isVisible {
			l.visibleRowHeights = append(l.visibleRowHeights, height)
		}
	}
	return offY, minRow
}

// Declare conformity with WidgetRenderer interface.
var _ core.WidgetRenderer = (*listRenderer)(nil)

type listRenderer struct {
	widget.BaseRenderer

	list     *List
	scroller *widget.Scroll
	layout   *bellui.Container
}

func newListRenderer(objects []core.CanvasObject, l *List, scroller *widget.Scroll, layout *bellui.Container) *listRenderer {
	lr := &listRenderer{BaseRenderer: widget.NewBaseRenderer(objects), list: l, scroller: scroller, layout: layout}
	lr.scroller.OnScrolled = l.offsetUpdated
	return lr
}

func (l *listRenderer) Layout(size geo.Size) {
	l.scroller.Resize(size)
}

func (l *listRenderer) MinSize() geo.Size {
	return l.scroller.MinSize().Max(l.list.itemMin)
}

func (l *listRenderer) Refresh() {
	l.list.minSizeCache = geo.Size{}
	if f := l.list.CreateItem; f != nil {
		l.list.itemMin = measureTemplate(createItemAndApplyThemeScope(f, l.list))
	}
	l.Layout(l.list.Size())
	l.scroller.Refresh()
	layout := l.layout.Layout.(*listLayout)
	layout.updateList(false)

	for _, s := range layout.separators {
		s.Refresh()
	}
	canvas.Refresh(l.list.super())
}

// Declare conformity with interfaces.
var (
	_ core.Widget     = (*listItem)(nil)
	_ event.Tappable  = (*listItem)(nil)
	_ event.Hoverable = (*listItem)(nil)
)

type listItem struct {
	BaseWidget

	onTapped          func()
	background        *canvas.Rectangle
	child             core.CanvasObject
	hovered, selected bool
}

func newListItem(child core.CanvasObject, tapped func()) *listItem {
	li := &listItem{
		child:    child,
		onTapped: tapped,
	}

	li.ExtendBaseWidget(li)
	return li
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (li *listItem) CreateRenderer() core.WidgetRenderer {
	li.ExtendBaseWidget(li)
	th := li.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	li.background = canvas.NewRectangle(th.Color(theme.ColorNameHover, v))
	li.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	li.background.Hide()

	accent := canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v))
	accent.Hide()

	objects := []core.CanvasObject{li.background, accent, li.child}

	return &listItemRenderer{BaseRenderer: widget.NewBaseRenderer(objects), item: li, accent: accent}
}

// MinSize returns the size that this widget should not shrink below.
func (li *listItem) MinSize() geo.Size {
	li.ExtendBaseWidget(li)
	return li.BaseWidget.MinSize()
}

// HoverIn is called when a desktop pointer enters the widget.
func (li *listItem) HoverIn(*event.HoverEvent) {
	li.hovered = true
	li.Refresh()
}

// HoverMoved is called when a desktop pointer hovers over the widget.
func (li *listItem) HoverMoved(*event.HoverEvent) {
}

// HoverOut is called when a desktop pointer exits the widget.
func (li *listItem) HoverOut() {
	li.hovered = false
	li.Refresh()
}

// Tapped is called when a pointer tapped event is captured and triggers any tap handler.
func (li *listItem) Tapped(*event.PointEvent) {
	if li.onTapped != nil {
		li.selected = true
		li.Refresh()
		li.onTapped()
	}
}

// AccessibleRole returns [core.RoleListItem].
func (li *listItem) AccessibleRole() core.AccessibleRole { return core.RoleListItem }

// AccessibleLabel returns an empty string; the item's template content
// provides its own semantics as child nodes.
func (li *listItem) AccessibleLabel() string { return "" }

// AccessibleState reports whether the row is selected.
func (li *listItem) AccessibleState() core.AccessibleState {
	if li.selected {
		return core.StateSelected
	}
	return 0
}

// AccessibleActions returns the "select" action.
func (li *listItem) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this item"}}
}

// PerformAccessibleAction selects the row like a tap would.
func (li *listItem) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		li.Tapped(nil)
	}
}

// Declare conformity with the WidgetRenderer interface.
var _ core.WidgetRenderer = (*listItemRenderer)(nil)

type listItemRenderer struct {
	widget.BaseRenderer

	item   *listItem
	accent *canvas.Rectangle
}

// MinSize calculates the minimum size of a listItem.
// This is based on the size of the status indicator and the size of the child object.
func (li *listItemRenderer) MinSize() geo.Size {
	return li.item.child.MinSize()
}

// Layout the components of the listItem widget.
func (li *listItemRenderer) Layout(size geo.Size) {
	li.item.background.Resize(size)
	if li.accent != nil {
		accentWidth := float32(3)
		li.accent.Resize(geo.NewSize(accentWidth, size.Height*0.6))
		li.accent.Move(geo.NewPos(0, size.Height*0.2))
	}
	li.item.child.Resize(size)
}

func (li *listItemRenderer) Refresh() {
	th := li.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	style := internaltheme.RenderStyleForWidget(li.item)

	li.item.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	if li.item.selected {
		li.item.background.FillColor = th.Color(theme.ColorNameSelection, v)
		li.item.background.Show()
	} else if li.item.hovered {
		onSurface := th.Color(theme.ColorNameOnSurface, v)
		li.item.background.FillColor = theme.StateLayerHover(onSurface)
		li.item.background.Show()
	} else {
		li.item.background.Hide()
	}
	li.item.background.Refresh()

	// Show left accent bar on selected items in modern mode.
	if li.accent != nil {
		if li.item.selected && style == internaltheme.RenderStyleModern {
			li.accent.FillColor = th.Color(theme.ColorNamePrimary, v)
			li.accent.CornerRadius = 1.5
			li.accent.Show()
			li.accent.Refresh()
		} else {
			li.accent.Hide()
		}
	}

	canvas.Refresh(li.item.super())
}

// Declare conformity with Layout interface.
var _ core.Layout = (*listLayout)(nil)

type listItemAndID struct {
	item *listItem
	id   ListItemID
}

type listLayout struct {
	list       *List
	separators []core.CanvasObject
	children   []core.CanvasObject

	itemPool          async.Pool[core.CanvasObject]
	visible           []listItemAndID
	wasVisible        []listItemAndID
	visibleRowHeights []float32
}

func newListLayout(list *List) core.Layout {
	l := &listLayout{list: list}
	list.offsetUpdated = l.offsetUpdated
	return l
}

func (l *listLayout) Layout([]core.CanvasObject, geo.Size) {
	l.updateList(true)
}

func (l *listLayout) MinSize([]core.CanvasObject) geo.Size {
	return l.list.contentMinSize()
}

func (l *listLayout) getItem() *listItem {
	item := l.itemPool.Get()
	if item == nil {
		if f := l.list.CreateItem; f != nil {
			item2 := createItemAndApplyThemeScope(f, l.list)

			item = newListItem(item2, nil)
		}
	}
	return item.(*listItem)
}

func (l *listLayout) offsetUpdated(pos geo.Position) {
	if l.list.offsetY == pos.Y {
		return
	}
	l.list.offsetY = pos.Y
	l.updateList(true)
}

func (l *listLayout) setupListItem(li *listItem, id ListItemID, focus bool) {
	previousIndicator := li.selected
	li.selected = false
	for _, s := range l.list.selected {
		if id == s {
			li.selected = true
			break
		}
	}
	if focus {
		li.hovered = true
		li.Refresh()
	} else if previousIndicator != li.selected || li.hovered {
		li.hovered = false
		li.Refresh()
	}
	if f := l.list.UpdateItem; f != nil {
		f(id, li.child)
	}
	li.onTapped = func() {
		if !bellui.CurrentDevice().IsMobile() {
			canvas := bellui.CurrentApp().Driver().CanvasForObject(l.list.super())
			if canvas != nil {
				canvas.Focus(l.list.super().(event.Focusable))
			}

			l.list.currentFocus = id
		}

		l.list.Select(id)
	}
}

func (l *listLayout) updateList(newOnly bool) {
	th := l.list.Theme()
	separatorThickness := th.Size(theme.SizeNamePadding)
	width := l.list.Size().Width
	length := 0
	if f := l.list.Length; f != nil {
		length = f()
	}
	if l.list.UpdateItem == nil {
		bellui.LogError("Missing UpdateCell callback required for List", nil)
	}

	// l.wasVisible now represents the currently visible items, while
	// l.visible will be updated to represent what is visible *after* the update
	l.wasVisible = append(l.wasVisible, l.visible...)
	l.visible = l.visible[:0]

	offY, minRow := l.calculateVisibleRowHeights(l.list.itemMin.Height, length, th)
	if len(l.visibleRowHeights) == 0 && length > 0 { // we can't show anything until we have some dimensions
		return
	}

	oldChildrenLen := len(l.children)
	l.children = l.children[:0]

	y := offY
	for index, itemHeight := range l.visibleRowHeights {
		row := index + minRow
		size := geo.NewSize(width, itemHeight)

		c, ok := l.searchVisible(l.wasVisible, row)
		if !ok {
			c = l.getItem()
			if c == nil {
				continue
			}
			c.Resize(size)
		}

		c.Move(geo.NewPos(0, y))
		c.Resize(size)

		y += itemHeight + separatorThickness
		l.visible = append(l.visible, listItemAndID{id: row, item: c})
		l.children = append(l.children, c)
	}
	l.nilOldSliceData(l.children, len(l.children), oldChildrenLen)

	for _, wasVis := range l.wasVisible {
		if _, ok := l.searchVisible(l.visible, wasVis.id); !ok {
			l.itemPool.Put(wasVis.item)
		}
	}

	l.updateSeparators()

	c := l.list.scroller.Content.(*bellui.Container)
	oldObjLen := len(c.Objects)
	c.Objects = c.Objects[:0]
	c.Objects = append(c.Objects, l.children...)
	c.Objects = append(c.Objects, l.separators...)
	l.nilOldSliceData(c.Objects, len(c.Objects), oldObjLen)

	if newOnly {
		for _, vis := range l.visible {
			if _, ok := l.searchVisible(l.wasVisible, vis.id); !ok {
				l.setupListItem(vis.item, vis.id, l.list.focused && l.list.currentFocus == vis.id)
			}
		}
	} else {
		for _, vis := range l.visible {
			l.setupListItem(vis.item, vis.id, l.list.focused && l.list.currentFocus == vis.id)
		}

		// a full refresh may change theme, we should drain the pool of unused items instead of refreshing them.
		// A drained item is dropped, so its renderers go with it (#722).
		for item := l.itemPool.Get(); item != nil; item = l.itemPool.Get() {
			intdriver.DestroyRenderers(item)
		}
	}

	// we don't need wasVisible now until next call to update
	// nil out all references before truncating slice
	for i := 0; i < len(l.wasVisible); i++ {
		l.wasVisible[i].item = nil
	}
	l.wasVisible = l.wasVisible[:0]
}

func (l *listLayout) updateSeparators() {
	if l.list.HideSeparators {
		l.separators = dropObjects(l.separators, 0)
		return
	}
	if lenChildren := len(l.children); lenChildren > 1 {
		if lenSep := len(l.separators); lenSep > lenChildren {
			l.separators = dropObjects(l.separators, lenChildren)
		} else {
			for i := lenSep; i < lenChildren; i++ {

				sep := NewSeparator()
				if cache.OverrideThemeMatchingScope(sep, l.list) {
					sep.Refresh()
				}

				l.separators = append(l.separators, sep)
			}
		}
	} else {
		l.separators = dropObjects(l.separators, 0)
	}

	th := l.list.Theme()
	separatorThickness := th.Size(theme.SizeNameSeparatorThickness)
	dividerOff := (th.Size(theme.SizeNamePadding) + separatorThickness) / 2
	for i, child := range l.children {
		if i == 0 {
			continue
		}
		l.separators[i].Move(geo.NewPos(0, child.Position().Y-dividerOff))
		l.separators[i].Resize(geo.NewSize(l.list.Size().Width, separatorThickness))
		l.separators[i].Show()
	}
}

// invariant: visible is in ascending order of IDs
func (l *listLayout) searchVisible(visible []listItemAndID, id ListItemID) (*listItem, bool) {
	ln := len(visible)
	idx := sort.Search(ln, func(i int) bool { return visible[i].id >= id })
	if idx < ln && visible[idx].id == id {
		return visible[idx].item, true
	}
	return nil, false
}

func (l *listLayout) nilOldSliceData(objs []core.CanvasObject, len, oldLen int) {
	if oldLen > len {
		objs = objs[:oldLen] // gain view into old data
		for i := len; i < oldLen; i++ {
			objs[i] = nil
		}
	}
}

// measureTemplate measures an item template that is never shown, then
// destroys the renderers measuring it cached, for the whole tree under it. No
// window holds the template, so nothing else releases them before the cache
// expiry, and without a driver nothing ever does (#722).
func measureTemplate(template core.CanvasObject) geo.Size {
	size := template.MinSize()
	intdriver.DestroyRenderers(template)
	return size
}

// releaseRenderer destroys the cached renderer of an object a widget stops
// holding, such as a separator it no longer needs: once the object is out of
// the tree, nothing else releases it before the cache expiry (#722).
func releaseRenderer(o core.CanvasObject) {
	if w, ok := o.(core.Widget); ok {
		cache.DestroyRenderer(w)
	}
}

// dropObjects keeps the first n objects of s and releases the renderers of
// the rest, which leave the tree (#722).
func dropObjects(s []core.CanvasObject, n int) []core.CanvasObject {
	if n >= len(s) {
		return s
	}
	for _, o := range s[n:] {
		releaseRenderer(o)
	}
	clear(s[n:])
	return s[:n]
}

func createItemAndApplyThemeScope(f func() core.CanvasObject, scope core.Widget) core.CanvasObject {
	item := f()
	if !cache.OverrideThemeMatchingScope(item, scope) {
		return item
	}

	item.Refresh()
	return item
}
