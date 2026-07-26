package widget

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestNewList(t *testing.T) {
	list := createList(1000)

	content := &bellui.Container{
		Layout: layout.NewHBoxLayout(), Objects: []core.CanvasObject{
			NewIcon(theme.DocumentIcon()),
			NewLabel("Template Object"),
		},
	}
	template := newListItem(content, nil)

	assert.Equal(t, 1000, list.Length())
	assert.GreaterOrEqual(t, list.MinSize().Width, template.MinSize().Width)
	assert.Equal(t, list.MinSize(), template.MinSize().Max(test.TempWidgetRenderer(t, list).(*listRenderer).scroller.MinSize()))
	assert.Equal(t, float32(0), list.offsetY)
}

func TestNewListWithData(t *testing.T) {
	data := binding.NewStringList()
	for i := 0; i < 1000; i++ {
		data.Append(fmt.Sprintf("Test Item %d", i))
	}

	list := NewListWithData(
		data,
		func() core.CanvasObject {
			return NewLabel("Template Object")
		},
		func(data binding.DataItem, item core.CanvasObject) {
			item.(*Label).Bind(data.(binding.String))
		},
	)

	template := NewLabel("Template Object")

	assert.Equal(t, 1000, list.Length())
	assert.GreaterOrEqual(t, list.MinSize().Width, template.MinSize().Width)
	assert.Equal(t, list.MinSize(), template.MinSize().Max(test.TempWidgetRenderer(t, list).(*listRenderer).scroller.MinSize()))
	assert.Equal(t, float32(0), list.offsetY)
}

func TestList_MinSize(t *testing.T) {
	for name, tt := range map[string]struct {
		cellSize        geo.Size
		expectedMinSize geo.Size
	}{
		"small": {
			geo.NewSize(1, 1),
			geo.NewSize(float32(32), float32(32)),
		},
		"large": {
			geo.NewSize(100, 100),
			geo.NewSize(100, 100),
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expectedMinSize, NewList(
				func() int { return 5 },
				func() core.CanvasObject {
					r := canvas.NewRectangle(color.Black)
					r.SetMinSize(tt.cellSize)
					r.Resize(tt.cellSize)
					return r
				},
				func(ListItemID, core.CanvasObject) {},
			).MinSize())
		})
	}
}

func TestList_Resize(t *testing.T) {
	test.NewTempApp(t)
	list, w := setupList(t)

	assert.Equal(t, float32(0), list.offsetY)

	w.Resize(geo.NewSize(200, 600))

	assert.Equal(t, float32(0), list.offsetY)
	test.AssertRendersToMarkup(t, "list/resized.xml", w.Canvas())

	// and check empty too
	list = NewList(
		func() int {
			return 0
		},
		func() core.CanvasObject {
			return NewButton("", func() {})
		},
		func(ListItemID, core.CanvasObject) {
		},
	)
	list.Resize(list.Size())
}

func TestList_SetItemHeight(t *testing.T) {
	list := NewList(
		func() int { return 5 },
		func() core.CanvasObject {
			r := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0x33})
			r.SetMinSize(geo.NewSize(10, 10))
			return r
		},
		func(ListItemID, core.CanvasObject) {
		},
	)

	lay := test.TempWidgetRenderer(t, list).(*listRenderer).layout
	assert.Equal(t, geo.NewSize(32, 32), list.MinSize())
	assert.Equal(t, geo.NewSize(10, 10*5+(4*theme.Padding())), lay.MinSize())

	list.SetItemHeight(2, 50)
	assert.Equal(t, geo.NewSize(10, 10*5+(4*theme.Padding())+40), lay.MinSize())

	list.Select(2)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 200))
	test.AssertImageMatches(t, "list/list_item_height.png", w.Canvas().Capture())
}

func TestList_SetItemHeight_InUpdate(t *testing.T) {
	var list *List
	list = NewList(
		func() int { return 5 },
		func() core.CanvasObject {
			r := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0x33})
			r.SetMinSize(geo.NewSize(10, 10))
			return r
		},
		func(id ListItemID, o core.CanvasObject) {
			list.SetItemHeight(id, 32)
		},
	)

	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			assert.Fail(t, "Timed out waiting for list to complete refresh")
		}
	}()
	list.Refresh() // could block
	done <- struct{}{}
}

func TestList_OffsetChange(t *testing.T) {
	test.NewTempApp(t)

	list := createList(1000)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 400))

	assert.Equal(t, float32(0), list.offsetY)

	scroll := test.TempWidgetRenderer(t, list).(*listRenderer).scroller
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -280)})

	assert.NotEqual(t, 0, list.offsetY)
	test.AssertRendersToMarkup(t, "list/offset_changed.xml", w.Canvas())
}

func TestList_Hover(t *testing.T) {
	list := createList(1000)
	children := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children

	onSurface := theme.Color(theme.ColorNameOnSurface)
	hoverColor := theme.StateLayerHover(onSurface)
	for i := 0; i < 2; i++ {
		assert.False(t, children[i].(*listItem).background.Visible())
		children[i].(*listItem).HoverIn(&event.HoverEvent{})
		assert.Equal(t, children[i].(*listItem).background.FillColor, hoverColor)
		children[i].(*listItem).HoverOut()
		assert.False(t, children[i].(*listItem).background.Visible())
	}
}

func TestList_ScrollTo(t *testing.T) {
	list := createList(1000)

	// Compute item stride and scroller height dynamically from theme.
	sep := list.Theme().Size(theme.SizeNamePadding)
	itemH := list.itemMin.Height
	scrollerH := list.scroller.Size().Height

	// yForItem returns the Y coordinate of the top of item id.
	yForItem := func(id int) float32 {
		return (float32(id) * itemH) + (float32(id) * sep)
	}
	// expectOffsetBelow returns offset when scrolling down to an item below viewport.
	expectOffsetBelow := func(id int) int {
		return int(yForItem(id) + itemH - scrollerH)
	}
	// expectOffsetAbove returns offset when scrolling up to an item above viewport.
	expectOffsetAbove := func(id int) int {
		return int(yForItem(id))
	}

	offset := 0
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	list.ScrollTo(20)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	offset = expectOffsetBelow(200)
	list.ScrollTo(200)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	offset = expectOffsetBelow(999)
	list.ScrollTo(999)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	offset = expectOffsetAbove(500)
	list.ScrollTo(500)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	list.ScrollTo(1000)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	offset = expectOffsetAbove(1)
	list.ScrollTo(1)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))
}

func TestList_ScrollToBottom(t *testing.T) {
	list := createList(1000)

	// Compute expected offset: total content height minus scroller viewport height.
	sep := list.Theme().Size(theme.SizeNamePadding)
	itemH := list.itemMin.Height
	totalH := float32(1000)*itemH + float32(999)*sep // 1000 items, 999 separators
	scrollerH := list.scroller.Size().Height
	expectedOffset := int(totalH - scrollerH)

	list.ScrollToBottom()
	assert.Equal(t, expectedOffset, int(list.offsetY))
	assert.Equal(t, expectedOffset, int(list.scroller.Offset.Y))
}

func TestList_ScrollToTop(t *testing.T) {
	list := createList(1000)

	tmpOffset := float32(50)
	list.ScrollToOffset(tmpOffset)
	assert.Equal(t, tmpOffset, list.offsetY)
	assert.Equal(t, tmpOffset, list.scroller.Offset.Y)

	offset := float32(0)
	list.ScrollToTop()
	assert.Equal(t, offset, list.offsetY)
	assert.Equal(t, offset, list.scroller.Offset.Y)
}

func TestList_ScrollOffset(t *testing.T) {
	list := createList(10)
	list.Resize(geo.NewSize(20, 15))

	offset := float32(25)
	list.ScrollToOffset(25)
	assert.Equal(t, offset, list.GetScrollOffset())

	list.ScrollToOffset(-2)
	assert.Equal(t, float32(0), list.GetScrollOffset())

	list.ScrollToOffset(1000)
	assert.LessOrEqual(t, list.GetScrollOffset(), float32(500) /*upper bound on content height*/)

	// list viewport is larger than content size
	list.Resize(geo.NewSize(100, 500))
	list.ScrollToOffset(20)
	assert.Equal(t, float32(0), list.GetScrollOffset()) // doesn't scroll
}

func TestList_Selection(t *testing.T) {
	list := createList(1000)
	children := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children

	assert.False(t, children[0].(*listItem).background.Visible())
	children[0].(*listItem).Tapped(&event.PointEvent{})
	assert.Equal(t, children[0].(*listItem).background.FillColor, theme.Color(theme.ColorNameSelection))
	assert.True(t, children[0].(*listItem).background.Visible())
	assert.Len(t, list.selected, 1)
	assert.Equal(t, 0, list.selected[0])
	children[1].(*listItem).Tapped(&event.PointEvent{})
	assert.Equal(t, children[1].(*listItem).background.FillColor, theme.Color(theme.ColorNameSelection))
	assert.True(t, children[1].(*listItem).background.Visible())
	assert.Len(t, list.selected, 1)
	assert.Equal(t, 1, list.selected[0])
	assert.False(t, children[0].(*listItem).background.Visible())

	offset := 0
	list.SetItemHeight(2, 220)
	list.SetItemHeight(3, 220)
	assert.Equal(t, offset, int(list.offsetY))
	assert.Equal(t, offset, int(list.scroller.Offset.Y))

	list.Select(200)
	// Compute expected offset dynamically: items 2 and 3 are 220px, rest are itemMin.Height.
	sep := list.Theme().Size(theme.SizeNamePadding)
	itemH := list.itemMin.Height
	scrollerH := list.scroller.Size().Height
	// y for item 200: sum of heights for items 0..199 + separators
	y := float32(200)*itemH + float32(200)*sep + float32(2)*(220-itemH)
	expectedOffset := int(y + itemH - scrollerH)
	assert.Equal(t, expectedOffset, int(list.offsetY))
	assert.Equal(t, expectedOffset, int(list.scroller.Offset.Y))
}

func TestList_Select(t *testing.T) {
	list := createList(1000)

	sep := list.Theme().Size(theme.SizeNamePadding)
	itemH := list.itemMin.Height
	scrollerH := list.scroller.Size().Height

	yForItem := func(id int) float32 {
		return (float32(id) * itemH) + (float32(id) * sep)
	}

	assert.Equal(t, float32(0), list.offsetY)
	list.Select(50)
	// Scrolling down to item 50 (below viewport).
	expectedOffset50 := int(yForItem(50) + itemH - scrollerH)
	assert.Equal(t, expectedOffset50, int(list.offsetY))
	lo := list.scroller.Content.(*bellui.Container).Layout.(*listLayout)
	visible50, _ := lo.searchVisible(lo.visible, 50)
	assert.Equal(t, visible50.background.FillColor, theme.Color(theme.ColorNameSelection))
	assert.True(t, visible50.background.Visible())

	list.Select(5)
	// Scrolling up to item 5 (above viewport).
	expectedOffset5 := int(yForItem(5))
	assert.Equal(t, expectedOffset5, int(list.offsetY))
	visible5, _ := lo.searchVisible(lo.visible, 5)
	assert.Equal(t, visible5.background.FillColor, theme.Color(theme.ColorNameSelection))
	assert.True(t, visible5.background.Visible())

	list.Select(6)
	// Item 6 is already visible, offset unchanged.
	assert.Equal(t, expectedOffset5, int(list.offsetY))
	visible5, _ = lo.searchVisible(lo.visible, 5)
	visible6, _ := lo.searchVisible(lo.visible, 6)
	assert.False(t, visible5.background.Visible())
	assert.Equal(t, visible6.background.FillColor, theme.Color(theme.ColorNameSelection))
	assert.True(t, visible6.background.Visible())
}

func TestList_Unselect(t *testing.T) {
	list := createList(1000)
	var unselected ListItemID
	list.OnUnselected = func(id ListItemID) {
		unselected = id
	}

	list.Select(10)
	children := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.Equal(t, children[10].(*listItem).background.FillColor, theme.Color(theme.ColorNameSelection))
	assert.True(t, children[10].(*listItem).background.Visible())

	list.Unselect(10)
	children = list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.False(t, children[10].(*listItem).background.Visible())
	assert.Nil(t, list.selected)
	assert.Equal(t, 10, unselected)

	unselected = -1
	list.Select(11)
	list.Unselect(9)
	assert.Len(t, list.selected, 1)
	assert.Equal(t, -1, unselected)

	list.UnselectAll()
	assert.Nil(t, list.selected)
	assert.Equal(t, 11, unselected)
}

func TestList_DataChange(t *testing.T) {
	test.NewTempApp(t)

	list, w := setupList(t)
	children := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children

	assert.Equal(t, "Test Item 0", children[0].(*listItem).child.(*bellui.Container).Objects[1].(*Label).Text)
	changeData(list)
	list.Refresh()
	children = list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.Equal(t, "a", children[0].(*listItem).child.(*bellui.Container).Objects[1].(*Label).Text)
	test.AssertRendersToMarkup(t, "list/new_data.xml", w.Canvas())
}

func TestList_ItemDataChange(t *testing.T) {
	test.NewTempApp(t)

	list, _ := setupList(t)
	children := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.Equal(t, "Test Item 0", children[0].(*listItem).child.(*bellui.Container).Objects[1].(*Label).Text)
	changeData(list)
	list.RefreshItem(0)
	children = list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.Equal(t, "a", children[0].(*listItem).child.(*bellui.Container).Objects[1].(*Label).Text)
}

func TestList_SmallList(t *testing.T) {
	test.NewTempApp(t)

	var data []string
	data = append(data, "Test Item 0")

	list := NewList(
		func() int {
			return len(data)
		},
		func() core.CanvasObject {
			return &bellui.Container{
				Layout: layout.NewHBoxLayout(), Objects: []core.CanvasObject{
					NewIcon(theme.DocumentIcon()),
					NewLabel("Template Object"),
				},
			}
		},
		func(id ListItemID, item core.CanvasObject) {
			item.(*bellui.Container).Objects[1].(*Label).SetText(data[id])
		},
	)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 400))

	visibleCount := len(list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children)
	assert.Equal(t, 1, visibleCount)

	data = append(data, "Test Item 1")
	list.Refresh()

	visibleCount = len(list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children)
	assert.Equal(t, 2, visibleCount)

	test.AssertRendersToMarkup(t, "list/small.xml", w.Canvas())
}

func TestList_ClearList(t *testing.T) {
	test.NewTempApp(t)
	list, w := setupList(t)
	assert.Equal(t, 1000, list.Length())

	list.Length = func() int {
		return 0
	}
	list.Refresh()

	visibleCount := len(list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children)

	assert.Equal(t, 0, visibleCount)
	test.AssertRendersToMarkup(t, "list/cleared.xml", w.Canvas())
}

func TestList_RemoveItem(t *testing.T) {
	test.NewTempApp(t)

	data := []string{"Test Item 0", "Test Item 1", "Test Item 2"}

	list := NewList(
		func() int {
			return len(data)
		},
		func() core.CanvasObject {
			return &bellui.Container{
				Layout: layout.NewHBoxLayout(), Objects: []core.CanvasObject{
					NewIcon(theme.DocumentIcon()),
					NewLabel("Template Object"),
				},
			}
		},
		func(id ListItemID, item core.CanvasObject) {
			item.(*bellui.Container).Objects[1].(*Label).SetText(data[id])
		},
	)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 400))

	visibleCount := len(list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children)
	assert.Equal(t, 3, visibleCount)

	data = data[:len(data)-1]
	list.Refresh()

	visibleCount = len(list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children)
	assert.Equal(t, 2, visibleCount)
	test.AssertRendersToMarkup(t, "list/item_removed.xml", w.Canvas())
}

func TestList_ScrollThenShrink(t *testing.T) {
	test.NewTempApp(t)

	data := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		data = append(data, fmt.Sprintf("Data %d", i))
	}

	list := NewList(
		func() int {
			return len(data)
		},
		func() core.CanvasObject {
			return NewLabel("TEMPLATE")
		},
		func(id ListItemID, item core.CanvasObject) {
			item.(*Label).SetText(data[id])
		},
	)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(300, 300))

	visibles := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	visibleCount := len(visibles)
	assert.Greater(t, visibleCount, 0)

	list.scroller.ScrollToBottom()
	visibles = list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.Equal(t, "Data 19", visibles[len(visibles)-1].(*listItem).child.(*Label).Text)

	data = data[:1]
	assert.NotPanics(t, func() { list.Refresh() })

	visibles = list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	visibleCount = len(visibles)
	assert.Equal(t, 1, visibleCount)
	assert.Equal(t, "Data 0", visibles[0].(*listItem).child.(*Label).Text)
}

func TestList_ScrollThenResizeWindow(t *testing.T) {
	test.NewTempApp(t)

	data := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		data = append(data, fmt.Sprintf("Data %d", i))
	}

	list := NewList(
		func() int {
			return len(data)
		},
		func() core.CanvasObject {
			return NewLabel("TEMPLATE")
		},
		func(id ListItemID, item core.CanvasObject) {
			item.(*Label).SetText(data[id])
		},
	)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(300, 300))

	list.scroller.ScrollToBottom()

	// increase window size enough so that all elements are visible
	w.Resize(geo.NewSize(300, 1000))

	visibles := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	visibleCount := len(visibles)
	assert.Equal(t, 20, visibleCount)
	assert.Equal(t, "Data 0", visibles[0].(*listItem).child.(*Label).Text)
}

func TestList_NoFunctionsSet(t *testing.T) {
	list := &List{}
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 400))
	list.Refresh()
}

func TestList_Focus(t *testing.T) {
	test.NewTempApp(t)
	list := createList(10)
	window := test.NewWindow(list)
	defer window.Close()
	window.Resize(list.MinSize().Max(geo.NewSize(150, 200)))

	canvas := window.Canvas().(test.WindowlessCanvas)
	assert.Nil(t, canvas.Focused())

	canvas.FocusNext()
	assert.NotNil(t, canvas.Focused())
	assert.Equal(t, 0, canvas.Focused().(*List).currentFocus)

	children := list.scroller.Content.(*bellui.Container).Layout.(*listLayout).children
	assert.True(t, children[0].(*listItem).hovered)
	assert.False(t, children[1].(*listItem).hovered)
	assert.False(t, children[2].(*listItem).hovered)

	list.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.False(t, children[0].(*listItem).hovered)
	assert.True(t, children[1].(*listItem).hovered)
	assert.False(t, children[2].(*listItem).hovered)

	list.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.True(t, children[0].(*listItem).hovered)
	assert.False(t, children[1].(*listItem).hovered)
	assert.False(t, children[2].(*listItem).hovered)

	canvas.Focused().TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.True(t, children[0].(*listItem).selected)
}

func createList(items int) *List {
	var data []string
	for i := 0; i < items; i++ {
		data = append(data, fmt.Sprintf("Test Item %d", i))
	}

	list := NewList(
		func() int {
			return len(data)
		},
		func() core.CanvasObject {
			icon := NewIcon(theme.DocumentIcon())
			return &bellui.Container{Layout: layout.NewBorderLayout(nil, nil, icon, nil), Objects: []core.CanvasObject{icon, NewLabel("Template Object")}}
		},
		func(id ListItemID, item core.CanvasObject) {
			item.(*bellui.Container).Objects[1].(*Label).SetText(data[id])
		},
	)
	list.Resize(geo.NewSize(200, 1000))
	return list
}

func changeData(list *List) {
	data := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	list.Length = func() int {
		return len(data)
	}
	list.UpdateItem = func(id ListItemID, item core.CanvasObject) {
		item.(*bellui.Container).Objects[1].(*Label).SetText(data[id])
	}
}

func setupList(t *testing.T) (*List, bellui.Window) {
	test.NewApp()
	list := createList(1000)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 400))
	test.AssertRendersToMarkup(t, "list/initial.xml", w.Canvas())
	return list, w
}

func TestList_LimitUpdateItem(t *testing.T) {
	app := test.NewApp()
	w := app.NewWindow("")
	defer w.Close()
	printOut := ""
	list := NewList(
		func() int {
			return 5
		},
		func() core.CanvasObject {
			return NewLabel("")
		},
		func(id ListItemID, item core.CanvasObject) {
			printOut += fmt.Sprintf("%d.", id)
		},
	)
	w.SetContent(list)
	bellui.ShowAndRun(w)
	assert.Equal(t, "0.1.", printOut)
	list.scrollTo(1)
	assert.Equal(t, "0.1.2.", printOut)
	list.scrollTo(2)
	assert.Equal(t, "0.1.2.3.", printOut)
}

func TestList_RefreshUpdatesAllItems(t *testing.T) {
	app := test.NewApp()
	w := app.NewWindow("")
	defer w.Close()
	printOut := ""
	list := NewList(
		func() int {
			return 1
		},
		func() core.CanvasObject {
			return NewLabel("Test")
		},
		func(id ListItemID, item core.CanvasObject) {
			printOut += fmt.Sprintf("%d.", id)
		},
	)
	w.SetContent(list)
	bellui.ShowAndRun(w)
	assert.Equal(t, "0.", printOut)

	list.Refresh()
	assert.Equal(t, "0.0.", printOut)
}

func TestList_ScrollToLargeItem(t *testing.T) {
	list := NewList(
		func() int {
			return 10
		},
		func() core.CanvasObject {
			return NewLabel("Row")
		},
		func(id ListItemID, item core.CanvasObject) {
		},
	)
	list.SetItemHeight(9, 50)
	w := test.NewTempWindow(t, list)

	w.SetContent(list)
	list.scrollTo(9)
	assert.Equal(t, list.scroller.Content.MinSize().Height-list.Size().Height, list.scroller.Offset.Y)
}

var minSize geo.Size

func BenchmarkContentMinSize(b *testing.B) {
	b.StopTimer()

	l := NewList(
		func() int { return 1000000 },
		func() core.CanvasObject {
			return NewLabel("Test")
		},
		func(id ListItemID, item core.CanvasObject) {
			item.(*Label).SetText(fmt.Sprintf("%d", id))
		},
	)
	l.SetItemHeight(10, 55)
	l.SetItemHeight(12345, 2)

	min := geo.Size{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		min = l.contentMinSize()
	}

	minSize = min
}
