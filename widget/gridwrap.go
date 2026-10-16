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
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// Declare conformity with interfaces.
var (
	_ core.Widget     = (*GridWrap)(nil)
	_ event.Focusable = (*GridWrap)(nil)
)

// GridWrapItemID is the ID of an individual item in the GridWrap widget.
//
// Since: 1.0
type GridWrapItemID = int

// GridWrap is a widget with an API very similar to widget.List,
// that lays out items in a scrollable wrapping grid similar to container.NewGridWrap.
// It caches and reuses widgets for performance.
//
// Since: 1.0
type GridWrap struct {
	BaseWidget

	// Length is a callback for returning the number of items in the GridWrap.
	Length func() int `json:"-"`

	// CreateItem is a callback invoked to create a new widget to render
	// an item in the GridWrap.
	CreateItem func() core.CanvasObject `json:"-"`

	// UpdateItem is a callback invoked to update a GridWrap item widget
	// to display a new item in the list. The UpdateItem callback should
	// only update the given item, it should not invoke APIs that would
	// change other properties of the GridWrap itself.
	UpdateItem func(id GridWrapItemID, item core.CanvasObject) `json:"-"`

	// OnSelected is a callback to be notified when a given item
	// in the GridWrap has been selected.
	OnSelected func(id GridWrapItemID) `json:"-"`

	// OnUnselected is a callback to be notified when a given item
	// in the GridWrap has been unselected.
	OnUnselected func(id GridWrapItemID) `json:"-"`

	currentFocus  ListItemID
	focused       bool
	scroller      *widget.Scroll
	selected      []GridWrapItemID
	itemMin       geo.Size
	offsetY       float32
	offsetUpdated func(geo.Position)
	colCountCache int
	minSizeCache  geo.Size
}

// NewGridWrap creates and returns a GridWrap widget for displaying items in
// a wrapping grid layout with scrolling and caching for performance.
//
// Since: 1.0
func NewGridWrap(length func() int, createItem func() core.CanvasObject, updateItem func(GridWrapItemID, core.CanvasObject)) *GridWrap {
	gwList := &GridWrap{Length: length, CreateItem: createItem, UpdateItem: updateItem}
	gwList.ExtendBaseWidget(gwList)
	return gwList
}

// AccessibleRole returns [core.RoleList].
func (l *GridWrap) AccessibleRole() core.AccessibleRole { return core.RoleList }

// AccessibleLabel returns an empty string; realized items provide their own semantics.
func (l *GridWrap) AccessibleLabel() string { return "" }

// NewGridWrapWithData creates a new GridWrap widget that will display the contents of the provided data.
//
// Since: 1.0
func NewGridWrapWithData(data binding.DataList, createItem func() core.CanvasObject, updateItem func(binding.DataItem, core.CanvasObject)) *GridWrap {
	gwList := NewGridWrap(
		data.Length,
		createItem,
		func(i GridWrapItemID, o core.CanvasObject) {
			item, err := data.GetItem(i)
			if err != nil {
				bellui.LogError(fmt.Sprintf("Error getting data item %d", i), err)
				return
			}
			updateItem(item, o)
		},
	)

	data.AddListener(binding.NewDataListener(gwList.Refresh))
	return gwList
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (l *GridWrap) CreateRenderer() core.WidgetRenderer {
	l.ExtendBaseWidget(l)

	if f := l.CreateItem; f != nil && l.itemMin.IsZero() {
		item := createItemAndApplyThemeScope(f, l)

		l.itemMin = item.MinSize()
	}

	layout := &bellui.Container{Layout: newGridWrapLayout(l)}
	l.scroller = widget.NewVScroll(layout)
	layout.Resize(layout.MinSize())

	return newGridWrapRenderer([]core.CanvasObject{l.scroller}, l, l.scroller, layout)
}

// FocusGained is called after this GridWrap has gained focus.
func (l *GridWrap) FocusGained() {
	l.focused = true
	l.RefreshItem(l.currentFocus)
}

// FocusLost is called after this GridWrap has lost focus.
func (l *GridWrap) FocusLost() {
	l.focused = false
	l.RefreshItem(l.currentFocus)
}

// MinSize returns the size that this widget should not shrink below.
func (l *GridWrap) MinSize() geo.Size {
	l.ExtendBaseWidget(l)
	return l.BaseWidget.MinSize()
}

func (l *GridWrap) scrollTo(id GridWrapItemID) {
	if l.scroller == nil {
		return
	}

	pad := l.Theme().Size(theme.SizeNamePadding)
	row := math.Floor(float64(id) / float64(l.ColumnCount()))
	y := float32(row)*l.itemMin.Height + float32(row)*pad
	if y < l.scroller.Offset.Y {
		l.scroller.Offset.Y = y
	} else if size := l.scroller.Size(); y+l.itemMin.Height > l.scroller.Offset.Y+size.Height {
		l.scroller.Offset.Y = y + l.itemMin.Height - size.Height
	}
	l.offsetUpdated(l.scroller.Offset)
}

// RefreshItem refreshes a single item, specified by the item ID passed in.
//
// Since: 1.0
func (l *GridWrap) RefreshItem(id GridWrapItemID) {
	l.minSizeCache = geo.Size{}
	if l.scroller == nil {
		return
	}
	l.BaseWidget.Refresh()
	lo := l.scroller.Content.(*bellui.Container).Layout.(*gridWrapLayout)
	item, ok := lo.searchVisible(lo.visible, id)
	if ok {
		lo.setupGridItem(item, id, l.focused && l.currentFocus == id)
	}
}

// focusedItemObject returns the realized gridWrapItem for the currently focused
// cell, or nil when it is not currently laid out. It lets keyboard activation
// report the same item object the pointer path hit-tests (#834).
func (l *GridWrap) focusedItemObject() core.CanvasObject {
	if l.scroller == nil {
		return nil
	}
	cont, ok := l.scroller.Content.(*bellui.Container)
	if !ok {
		return nil
	}
	lo, ok := cont.Layout.(*gridWrapLayout)
	if !ok {
		return nil
	}
	if item, ok := lo.searchVisible(lo.visible, l.currentFocus); ok {
		return item
	}
	return nil
}

// Resize is called when this GridWrap should change size. We refresh to ensure invisible items are drawn.
func (l *GridWrap) Resize(s geo.Size) {
	oldColCount := l.ColumnCount()
	oldHeight := l.size.Height
	l.colCountCache = 0
	l.BaseWidget.Resize(s)
	newColCount := l.ColumnCount()

	if oldColCount == newColCount && oldHeight == s.Height {
		// no content update needed if resizing only horizontally and col count is unchanged
		return
	}

	if l.scroller != nil {
		l.offsetUpdated(l.scroller.Offset)
		l.scroller.Content.(*bellui.Container).Layout.(*gridWrapLayout).updateGrid(true)
	}
}

// Select adds the item identified by the given ID to the selection.
func (l *GridWrap) Select(id GridWrapItemID) {
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
	l.selected = []GridWrapItemID{id}
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
func (l *GridWrap) ScrollTo(id GridWrapItemID) {
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
func (l *GridWrap) ScrollToBottom() {
	l.scroller.ScrollToBottom()
	l.offsetUpdated(l.scroller.Offset)
}

// ScrollToTop scrolls to the start of the list
func (l *GridWrap) ScrollToTop() {
	l.scroller.ScrollToTop()
	l.offsetUpdated(l.scroller.Offset)
}

// ScrollToOffset scrolls the list to the given offset position
func (l *GridWrap) ScrollToOffset(offset float32) {
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

// TypedKey is called if a key event happens while this GridWrap is focused.
func (l *GridWrap) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeySpace:
		l.Select(l.currentFocus)
		// Report the realized item object (RoleListItem) so keyboard activation
		// matches the pointer path for autocapture (#834, #700).
		if item := l.focusedItemObject(); item != nil {
			core.NotifyActivation(item, true)
		}
	case event.KeyDown:
		count := 0
		if f := l.Length; f != nil {
			count = f()
		}
		l.RefreshItem(l.currentFocus)
		l.currentFocus += l.ColumnCount()
		if l.currentFocus >= count-1 {
			l.currentFocus = count - 1
		}
		l.scrollTo(l.currentFocus)
		l.RefreshItem(l.currentFocus)
	case event.KeyLeft:
		// Under RTL the visual reading order is reversed, so the left arrow
		// advances to the next item rather than the previous one.
		if lang.IsRTLDirection() {
			l.focusNextItem()
		} else {
			l.focusPrevItem()
		}
	case event.KeyRight:
		if lang.IsRTLDirection() {
			l.focusPrevItem()
		} else {
			l.focusNextItem()
		}
	case event.KeyUp:
		if l.currentFocus <= 0 {
			return
		}
		l.RefreshItem(l.currentFocus)
		l.currentFocus -= l.ColumnCount()
		if l.currentFocus < 0 {
			l.currentFocus = 0
		}
		l.scrollTo(l.currentFocus)
		l.RefreshItem(l.currentFocus)
	}
}

// focusPrevItem moves keyboard focus to the preceding item (the visual left
// neighbour under LTR), clamped at the first item.
func (l *GridWrap) focusPrevItem() {
	if l.currentFocus <= 0 {
		return
	}
	l.RefreshItem(l.currentFocus)
	l.currentFocus--
	l.scrollTo(l.currentFocus)
	l.RefreshItem(l.currentFocus)
}

// focusNextItem moves keyboard focus to the following item (the visual right
// neighbour under LTR), clamped at the last item.
func (l *GridWrap) focusNextItem() {
	if f := l.Length; f != nil && l.currentFocus >= f()-1 {
		return
	}
	l.RefreshItem(l.currentFocus)
	l.currentFocus++
	l.scrollTo(l.currentFocus)
	l.RefreshItem(l.currentFocus)
}

// TypedRune is called if a text event happens while this GridWrap is focused.
func (l *GridWrap) TypedRune(_ rune) {
	// intentionally left blank
}

// GetScrollOffset returns the current scroll offset position
func (l *GridWrap) GetScrollOffset() float32 {
	return l.offsetY
}

// Unselect removes the item identified by the given ID from the selection.
func (l *GridWrap) Unselect(id GridWrapItemID) {
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
func (l *GridWrap) UnselectAll() {
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

// Refresh causes this GridWrap to be redrawn in its current state.
func (l *GridWrap) Refresh() {
	l.minSizeCache = geo.Size{}
	l.BaseWidget.Refresh()
}

func (l *GridWrap) contentMinSize() geo.Size {
	if !l.minSizeCache.IsZero() {
		return l.minSizeCache
	}

	padding := l.Theme().Size(theme.SizeNamePadding)
	if l.Length == nil {
		return geo.NewSize(0, 0)
	}

	cols := l.ColumnCount()
	rows := float32(math.Ceil(float64(l.Length()) / float64(cols)))
	size := geo.NewSize(l.itemMin.Width, (l.itemMin.Height+padding)*rows-padding)
	l.minSizeCache = size
	return size
}

// Declare conformity with WidgetRenderer interface.
var _ core.WidgetRenderer = (*gridWrapRenderer)(nil)

type gridWrapRenderer struct {
	objects []core.CanvasObject

	list     *GridWrap
	scroller *widget.Scroll
	layout   *bellui.Container
}

func newGridWrapRenderer(objects []core.CanvasObject, l *GridWrap, scroller *widget.Scroll, layout *bellui.Container) *gridWrapRenderer {
	lr := &gridWrapRenderer{objects: objects, list: l, scroller: scroller, layout: layout}
	lr.scroller.OnScrolled = l.offsetUpdated
	return lr
}

func (l *gridWrapRenderer) Layout(size geo.Size) {
	l.scroller.Resize(size)
}

func (l *gridWrapRenderer) MinSize() geo.Size {
	return l.scroller.MinSize().Max(l.list.itemMin)
}

func (l *gridWrapRenderer) Refresh() {
	if f := l.list.CreateItem; f != nil {
		item := createItemAndApplyThemeScope(f, l.list)

		l.list.itemMin = item.MinSize()
	}
	l.Layout(l.list.Size())
	l.scroller.Refresh()
	l.layout.Layout.(*gridWrapLayout).updateGrid(false)
	canvas.Refresh(l.list)
}

func (l *gridWrapRenderer) Destroy() {
}

func (l *gridWrapRenderer) Objects() []core.CanvasObject {
	return l.objects
}

// Declare conformity with interfaces.
var (
	_ core.Widget     = (*gridWrapItem)(nil)
	_ event.Tappable  = (*gridWrapItem)(nil)
	_ event.Hoverable = (*gridWrapItem)(nil)
)

type gridWrapItem struct {
	BaseWidget

	onTapped          func()
	background        *canvas.Rectangle
	child             core.CanvasObject
	hovered, selected bool
}

func newGridWrapItem(child core.CanvasObject, tapped func()) *gridWrapItem {
	gw := &gridWrapItem{
		child:    child,
		onTapped: tapped,
	}

	gw.ExtendBaseWidget(gw)
	return gw
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (gw *gridWrapItem) CreateRenderer() core.WidgetRenderer {
	gw.ExtendBaseWidget(gw)
	th := gw.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	gw.background = canvas.NewRectangle(th.Color(theme.ColorNameHover, v))
	gw.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	gw.background.Hide()

	objects := []core.CanvasObject{gw.background, gw.child}

	return &gridWrapItemRenderer{widget.NewBaseRenderer(objects), gw}
}

// MinSize returns the size that this widget should not shrink below.
func (gw *gridWrapItem) MinSize() geo.Size {
	gw.ExtendBaseWidget(gw)
	return gw.BaseWidget.MinSize()
}

// HoverIn is called when a desktop pointer enters the widget.
func (gw *gridWrapItem) HoverIn(*event.HoverEvent) {
	gw.hovered = true
	gw.Refresh()
}

// HoverMoved is called when a desktop pointer hovers over the widget.
func (gw *gridWrapItem) HoverMoved(*event.HoverEvent) {
}

// HoverOut is called when a desktop pointer exits the widget.
func (gw *gridWrapItem) HoverOut() {
	gw.hovered = false
	gw.Refresh()
}

// Tapped is called when a pointer tapped event is captured and triggers any tap handler.
func (gw *gridWrapItem) Tapped(*event.PointEvent) {
	if gw.onTapped != nil {
		gw.selected = true
		gw.Refresh()
		gw.onTapped()
	}
}

// AccessibleRole returns [core.RoleListItem].
func (gw *gridWrapItem) AccessibleRole() core.AccessibleRole { return core.RoleListItem }

// AccessibleLabel returns an empty string; the item's template content
// provides its own semantics as child nodes.
func (gw *gridWrapItem) AccessibleLabel() string { return "" }

// AccessibleState reports whether the cell is selected.
func (gw *gridWrapItem) AccessibleState() core.AccessibleState {
	if gw.selected {
		return core.StateSelected
	}
	return 0
}

// AccessibleActions returns the "select" action.
func (gw *gridWrapItem) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this item"}}
}

// PerformAccessibleAction selects the cell like a tap would.
func (gw *gridWrapItem) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		gw.Tapped(nil)
	}
}

// Declare conformity with the WidgetRenderer interface.
var _ core.WidgetRenderer = (*gridWrapItemRenderer)(nil)

type gridWrapItemRenderer struct {
	widget.BaseRenderer

	item *gridWrapItem
}

// MinSize calculates the minimum size of a listItem.
// This is based on the size of the status indicator and the size of the child object.
func (gw *gridWrapItemRenderer) MinSize() geo.Size {
	return gw.item.child.MinSize()
}

// Layout the components of the listItem widget.
func (gw *gridWrapItemRenderer) Layout(size geo.Size) {
	gw.item.background.Resize(size)
	gw.item.child.Resize(size)
}

func (gw *gridWrapItemRenderer) Refresh() {
	th := gw.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	gw.item.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	if gw.item.selected {
		gw.item.background.FillColor = th.Color(theme.ColorNameSelection, v)
		gw.item.background.Show()
	} else if gw.item.hovered {
		gw.item.background.FillColor = th.Color(theme.ColorNameHover, v)
		gw.item.background.Show()
	} else {
		gw.item.background.Hide()
	}
	gw.item.background.Refresh()
	canvas.Refresh(gw.item.super())
}

// Declare conformity with Layout interface.
var _ core.Layout = (*gridWrapLayout)(nil)

type gridItemAndID struct {
	item *gridWrapItem
	id   GridWrapItemID
}

type gridWrapLayout struct {
	gw *GridWrap

	itemPool   async.Pool[core.CanvasObject]
	visible    []gridItemAndID
	wasVisible []gridItemAndID
}

func newGridWrapLayout(gw *GridWrap) core.Layout {
	gwl := &gridWrapLayout{gw: gw}
	gw.offsetUpdated = gwl.offsetUpdated
	return gwl
}

func (l *gridWrapLayout) Layout(_ []core.CanvasObject, _ geo.Size) {
	l.updateGrid(true)
}

func (l *gridWrapLayout) MinSize(_ []core.CanvasObject) geo.Size {
	return l.gw.contentMinSize()
}

func (l *gridWrapLayout) getItem() *gridWrapItem {
	item := l.itemPool.Get()
	if item == nil {
		if f := l.gw.CreateItem; f != nil {
			child := createItemAndApplyThemeScope(f, l.gw)

			item = newGridWrapItem(child, nil)
		}
	}
	return item.(*gridWrapItem)
}

func (l *gridWrapLayout) offsetUpdated(pos geo.Position) {
	if l.gw.offsetY == pos.Y {
		return
	}
	l.gw.offsetY = pos.Y
	l.updateGrid(true)
}

func (l *gridWrapLayout) setupGridItem(li *gridWrapItem, id GridWrapItemID, focus bool) {
	previousIndicator := li.selected
	li.selected = false
	for _, s := range l.gw.selected {
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
	if f := l.gw.UpdateItem; f != nil {
		f(id, li.child)
	}
	li.onTapped = func() {
		if !bellui.CurrentDevice().IsMobile() {
			l.gw.RefreshItem(l.gw.currentFocus)
			canvas := bellui.CurrentApp().Driver().CanvasForObject(l.gw.super())
			if canvas != nil {
				canvas.Focus(l.gw.super().(event.Focusable))
			}

			l.gw.currentFocus = id
		}

		l.gw.Select(id)
	}
}

// ColumnCount returns the number of columns that are/will be shown
// in this GridWrap, based on the widget's current width.
//
// Since: 1.0
func (l *GridWrap) ColumnCount() int {
	if l.colCountCache < 1 {
		padding := l.Theme().Size(theme.SizeNamePadding)
		l.colCountCache = 1
		width := l.Size().Width
		if width > l.itemMin.Width {
			l.colCountCache = int(math.Floor(float64(width+padding) / float64(l.itemMin.Width+padding)))
		}
	}
	return l.colCountCache
}

func (l *gridWrapLayout) updateGrid(newOnly bool) {
	// code here is a mashup of listLayout.updateList and gridWrapLayout.Layout
	padding := l.gw.Theme().Size(theme.SizeNamePadding)

	length := 0
	if f := l.gw.Length; f != nil {
		length = f()
	}

	colCount := l.gw.ColumnCount()
	visibleRowsCount := int(math.Ceil(float64(l.gw.scroller.Size().Height)/float64(l.gw.itemMin.Height+padding))) + 1

	offY := l.gw.offsetY - float32(math.Mod(float64(l.gw.offsetY), float64(l.gw.itemMin.Height+padding)))
	minRow := int(offY / (l.gw.itemMin.Height + padding))
	minItem := minRow * colCount
	maxRow := int(math.Min(float64(minRow+visibleRowsCount), math.Ceil(float64(length)/float64(colCount))))
	maxItem := GridWrapItemID(math.Min(float64(maxRow*colCount), float64(length-1)))

	if l.gw.UpdateItem == nil {
		bellui.LogError("Missing UpdateCell callback required for GridWrap", nil)
	}

	// l.wasVisible now represents the currently visible items, while
	// l.visible will be updated to represent what is visible *after* the update
	l.wasVisible = append(l.wasVisible, l.visible...)
	l.visible = l.visible[:0]

	c := l.gw.scroller.Content.(*bellui.Container)
	oldObjLen := len(c.Objects)
	c.Objects = c.Objects[:0]
	y := offY
	curItemID := minItem
	stride := l.gw.itemMin.Width + padding
	rtl := lang.IsRTLDirection()
	for row := minRow; row <= maxRow && curItemID <= maxItem; row++ {
		for col := 0; col < colCount && curItemID <= maxItem; col++ {
			item, ok := l.searchVisible(l.wasVisible, curItemID)
			if !ok {
				item = l.getItem()
				if item == nil {
					continue
				}
				item.Resize(l.gw.itemMin)
			}

			// Under RTL the columns flip: the first column sits on the right.
			// Rows and scrolling are unchanged.
			x := float32(col) * stride
			if rtl {
				x = float32(colCount-1-col) * stride
			}
			item.Move(geo.NewPos(x, y))
			item.Resize(l.gw.itemMin)

			l.visible = append(l.visible, gridItemAndID{item: item, id: curItemID})
			c.Objects = append(c.Objects, item)
			curItemID++
		}
		y += l.gw.itemMin.Height + padding
	}
	l.nilOldSliceData(c.Objects, len(c.Objects), oldObjLen)

	for _, old := range l.wasVisible {
		if _, ok := l.searchVisible(l.visible, old.id); !ok {
			l.itemPool.Put(old.item)
		}
	}

	if newOnly {
		for _, obj := range l.visible {
			if _, ok := l.searchVisible(l.wasVisible, obj.id); !ok {
				l.setupGridItem(obj.item, obj.id, l.gw.focused && l.gw.currentFocus == obj.id)
			}
		}
	} else {
		for _, obj := range l.visible {
			l.setupGridItem(obj.item, obj.id, l.gw.focused && l.gw.currentFocus == obj.id)
		}
	}

	// we don't need wasVisible now until next call to update
	// nil out all references before truncating slice
	for i := 0; i < len(l.wasVisible); i++ {
		l.wasVisible[i].item = nil
	}
	l.wasVisible = l.wasVisible[:0]
}

// invariant: visible is in ascending order of IDs
func (l *gridWrapLayout) searchVisible(visible []gridItemAndID, id GridWrapItemID) (*gridWrapItem, bool) {
	ln := len(visible)
	idx := sort.Search(ln, func(i int) bool { return visible[i].id >= id })
	if idx < ln && visible[idx].id == id {
		return visible[idx].item, true
	}
	return nil, false
}

func (l *gridWrapLayout) nilOldSliceData(objs []core.CanvasObject, len, oldLen int) {
	if oldLen > len {
		objs = objs[:oldLen] // gain view into old data
		for i := len; i < oldLen; i++ {
			objs[i] = nil
		}
	}
}
