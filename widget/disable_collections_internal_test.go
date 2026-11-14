package widget

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// The tests in this file pin #1215: a disabled List, Tree (and so FileTree),
// TreeView, Carousel or SwipeArea is visible but inert. Each one disables the
// widget before it is drawn, checks that a tap, a drag, a scroll and a key
// change nothing and fire no callback, that Tab skips it, that the disabled
// look is on, and then that Enable brings input and the look back. The parts
// the app supplies (row templates, pages, content) are not touched.

// disabledTestWindow shows w below an Entry, in an unpadded window, so that
// Tab has somewhere else to go and absolute positions equal the content's.
func disabledTestWindow(t *testing.T, w core.CanvasObject, size geo.Size) (bellui.Window, *Entry) {
	t.Helper()
	other := NewEntry()
	win := test.NewTempWindow(t, &bellui.Container{
		Layout:  layout.NewBorderLayout(other, nil, nil, nil),
		Objects: []core.CanvasObject{other, w},
	})
	win.SetPadded(false)
	win.Resize(size)
	return win, other
}

// assertTabSkips checks that Tab from nothing lands on other and that the
// next Tab comes back to it, past the disabled widget and its parts.
func assertTabSkips(t *testing.T, c core.Canvas, other *Entry) {
	t.Helper()
	c.Unfocus()
	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab went to the disabled widget or a part of it")
	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab went to the disabled widget or a part of it")
	c.Unfocus()
}

// inside returns a point inside o, in canvas coordinates.
func inside(o core.CanvasObject, at geo.Position) geo.Position {
	return bellui.CurrentApp().Driver().AbsolutePositionForObject(o).Add(at)
}

func themeColor(w interface{ Theme() bellui.Theme }, name bellui.ThemeColorName) color.Color {
	return w.Theme().Color(name, bellui.CurrentApp().Settings().ThemeVariant())
}

func listRow(t *testing.T, l *List, id ListItemID) *listItem {
	t.Helper()
	lo := l.scroller.Content.(*bellui.Container).Layout.(*listLayout)
	row, ok := lo.searchVisible(lo.visible, id)
	require.True(t, ok, "row %d is not shown", id)
	return row
}

func TestList_Disable(t *testing.T) {
	test.NewTempApp(t)
	selected := 0
	l := NewList(
		func() int { return 100 },
		func() core.CanvasObject { return NewLabel("row") },
		func(id ListItemID, o core.CanvasObject) { o.(*Label).SetText(fmt.Sprint(id)) },
	)
	l.OnSelected = func(ListItemID) { selected++ }

	l.Disable() // before the first draw: no renderer, no window
	assert.True(t, l.Disabled())
	w, other := disabledTestWindow(t, l, geo.NewSize(200, 300))
	c := w.Canvas()
	assert.True(t, l.Disabled())

	row := listRow(t, l, 1)
	pos := inside(l, geo.NewPos(20, 60))

	// Taps, the accessible select, hover and the pressed state.
	test.Tap(row)
	row.PerformAccessibleAction(core.ActionSelect)
	row.HoverIn(&event.HoverEvent{})
	_, pressable := row.pressShape()
	assert.False(t, row.hovered, "a disabled list's row took a hover")
	assert.False(t, pressable, "a disabled list's row shows a press")

	// The wheel, a touch drag and keys.
	test.Scroll(c, pos, 0, -200)
	test.Drag(c, pos, 0, -200)
	assert.False(t, l.scroller.TryScrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -200}}),
		"a disabled list consumed a scroll: the scroll around it would not get it")
	// ClaimsDrag is built only for touch (ci, android, ios, mobile), not wasm.
	if claimer, ok := any(l.scroller).(interface{ ClaimsDrag(geo.Delta) bool }); ok {
		assert.False(t, claimer.ClaimsDrag(geo.Delta{DY: -20}), "a disabled list claimed a touch drag")
	}
	l.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	l.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	assert.Zero(t, selected, "OnSelected fired")
	assert.Empty(t, l.selected)
	assert.Zero(t, l.scroller.Offset.Y, "the list scrolled")
	assert.Zero(t, l.currentFocus, "a key moved the list's focus row")
	assertTabSkips(t, c, other)

	// The look: a row selected from code shows in the disabled colours.
	l.Select(1)
	selected = 0
	row = listRow(t, l, 1)
	assert.Equal(t, themeColor(l, theme.ColorNameDisabledButton), row.background.FillColor)

	l.Enable()
	assert.False(t, l.Disabled())
	assert.Equal(t, themeColor(l, theme.ColorNameSelection), listRow(t, l, 1).background.FillColor)

	// Enabled, the same input works.
	test.Tap(listRow(t, l, 2))
	assert.Equal(t, 1, selected, "an enabled list's row did not select on a tap")
	test.Scroll(c, pos, 0, -50)
	assert.NotZero(t, l.scroller.Offset.Y, "an enabled list did not scroll")

	// Disabled after the first draw, with the focus on the list.
	c.Focus(l)
	require.Equal(t, l, c.Focused())
	l.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	focusRow := l.currentFocus
	l.Disable()
	assert.Nil(t, c.Focused(), "a disabled list kept the focus")
	assert.Equal(t, themeColor(l, theme.ColorNameDisabledButton), listRow(t, l, 2).background.FillColor)
	l.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, focusRow, l.currentFocus, "a key moved the disabled list's focus row")
	assertTabSkips(t, c, other)
}

// treeNodes returns the tree's shown rows by node.
func treeNodes(t *testing.T, tr *Tree) (map[string]*branch, map[string]*leaf) {
	t.Helper()
	r, ok := cache.Renderer(tr.scroller.Content.(*treeContent)).(*treeContentRenderer)
	require.True(t, ok)
	return r.branches, r.leaves
}

// checkTreeDisable runs the #1215 checks on a tree whose root has branch a
// (closed, holding something) and leaf b, both shown, and enough rows after
// them to scroll. d is the widget as the
// app holds it: the Tree, or the FileTree around it.
func checkTreeDisable(t *testing.T, d core.Disableable, tr *Tree, a, b string) {
	t.Helper()
	selected, opened := 0, 0
	tr.OnSelected = func(TreeNodeID) { selected++ }
	wasOpened := tr.OnBranchOpened // a FileTree watches through it
	tr.OnBranchOpened = func(id TreeNodeID) {
		if id == a {
			opened++
		}
		if wasOpened != nil {
			wasOpened(id)
		}
	}

	d.Disable() // before the first draw
	assert.True(t, d.Disabled())
	w, other := disabledTestWindow(t, d.(core.CanvasObject), geo.NewSize(200, 300))
	c := w.Canvas()

	branches, leaves := treeNodes(t, tr)
	require.Contains(t, branches, a)
	require.Contains(t, leaves, b)
	icon := branches[a].icon.(*branchIcon)
	pos := inside(d.(core.CanvasObject), geo.NewPos(20, 30))

	test.Tap(leaves[b])
	test.Tap(icon)
	leaves[b].PerformAccessibleAction(core.ActionSelect)
	branches[a].PerformAccessibleAction(core.ActionExpand)
	leaves[b].HoverIn(&event.HoverEvent{})
	_, pressable := leaves[b].pressShape()
	assert.False(t, leaves[b].hovered, "a disabled tree's row took a hover")
	assert.False(t, pressable, "a disabled tree's row shows a press")

	test.Scroll(c, pos, 0, -200)
	test.Drag(c, pos, 0, -200)
	assert.False(t, tr.scroller.TryScrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -200}}))
	tr.currentFocus = a
	tr.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	tr.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	assert.Zero(t, selected, "OnSelected fired")
	assert.Zero(t, opened, "OnBranchOpened fired")
	assert.False(t, tr.IsBranchOpen(a))
	assert.Empty(t, tr.selected)
	assert.Zero(t, tr.scroller.Offset.Y, "the tree scrolled")
	assertTabSkips(t, c, other)

	// The look: the expand icon and a row selected from code.
	assert.IsType(t, &theme.DisabledResource{}, icon.Resource)
	tr.Select(b)
	selected = 0
	bg := cache.Renderer(leaves[b]).(*treeNodeRenderer).background
	assert.Equal(t, themeColor(tr, theme.ColorNameDisabledButton), bg.FillColor)

	d.Enable()
	assert.False(t, d.Disabled())
	assert.NotEqual(t, "*theme.DisabledResource", fmt.Sprintf("%T", icon.Resource))
	assert.Equal(t, themeColor(tr, theme.ColorNameSelection), bg.FillColor)

	// Enabled, the same input works.
	test.Tap(icon)
	assert.Equal(t, 1, opened, "an enabled tree's expand icon did not open the branch")
	branches, _ = treeNodes(t, tr)
	test.Tap(branches[a])
	assert.Equal(t, 1, selected, "an enabled tree's row did not select on a tap")
	test.Scroll(c, pos, 0, -50)
	assert.NotZero(t, tr.scroller.Offset.Y, "an enabled tree did not scroll")

	// Disabled after the first draw, with the focus on the tree.
	c.Focus(d.(event.Focusable))
	require.NotNil(t, c.Focused())
	d.Disable()
	assert.Nil(t, c.Focused(), "a disabled tree kept the focus")
	assertTabSkips(t, c, other)
}

func TestTree_Disable(t *testing.T) {
	test.NewTempApp(t)
	data := map[string][]string{"": {"a", "b"}, "a": {"a1"}}
	for i := range 30 {
		data[""] = append(data[""], fmt.Sprintf("f%d", i))
	}
	tr := NewTreeWithStrings(data)
	checkTreeDisable(t, tr, tr, "a", "b")
}

// FileTree is a Tree, and disables as one. Disable and Enable redraw it
// without listing the directories again, as Refresh would.
func TestFileTree_Disable(t *testing.T) {
	test.NewTempApp(t)
	tr, root := newTestFileTree(t)
	// A file at the top, beside the directories A and B.
	require.NoError(t, os.WriteFile(filepath.Join(root.Path(), "top.txt"), nil, 0o600))
	for i := range 30 { // rows enough to scroll, listed after the others
		require.NoError(t, os.WriteFile(filepath.Join(root.Path(), fmt.Sprintf("z%02d.txt", i)), nil, 0o600))
	}
	b, _ := storage.Child(root, "B")
	leaf, _ := storage.Child(root, "top.txt")

	checkTreeDisable(t, tr, &tr.Tree, b.String(), leaf.String())
	assert.Empty(t, tr.stale, "Disable or Enable listed the tree again")
}

func treeViewRow(t *testing.T, tv *TreeView, id TreeViewNodeID) (*treeViewItem, *treeViewItemRenderer) {
	t.Helper()
	item := tv.items()[id]
	require.NotNil(t, item, "row %s is not shown", id)
	return item, cache.Renderer(item).(*treeViewItemRenderer)
}

func TestTreeView_Disable(t *testing.T) {
	test.NewTempApp(t)
	tv := NewTreeViewWithNodes([]TreeViewNode{
		{ID: "a", Text: "Alpha", Icon: theme.FolderIcon()},
		{ID: "b", Text: "Beta"},
		{ID: "c", Text: "Charlie"},
	})
	require.NoError(t, tv.AddNode("a", TreeViewNode{ID: "a1", Text: "One"}))
	for i := range 30 { // rows enough to scroll
		require.NoError(t, tv.AddNode("", TreeViewNode{ID: fmt.Sprintf("n%d", i), Text: "More"}))
	}
	tv.AllowReorder, tv.AllowRename, tv.AllowDelete = true, true, true
	selected, opened, moved, deleted := 0, 0, 0, 0
	tv.OnSelected = func(TreeViewNodeID) { selected++ }
	tv.OnBranchOpened = func(TreeViewNodeID) { opened++ }
	tv.OnMoved = func(TreeViewNodeID, TreeViewNodeID, int) { moved++ }
	tv.OnDeleted = func(TreeViewNodeID) { deleted++ }

	tv.Disable() // before the first draw
	assert.True(t, tv.Disabled())
	w, other := disabledTestWindow(t, tv, geo.NewSize(300, 300))
	c := w.Canvas()

	a, ra := treeViewRow(t, tv, "a")
	cRow, _ := treeViewRow(t, tv, "c")
	pos := inside(tv, geo.NewPos(20, 10))
	// A drag of c onto row a, which moves c when enabled.
	dragC := func() {
		onA := inside(tv.items()["a"], geo.NewPos(20, 2))
		cRow.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: onA}, Dragged: geo.Delta{DY: -40}})
		cRow.DragEnd()
	}

	test.Tap(a)
	test.DoubleTap(a)
	test.TapSecondary(a)
	test.Tap(ra.chevron)
	a.PerformAccessibleAction(core.ActionSelect)
	a.PerformAccessibleAction(core.ActionExpand)
	a.PerformAccessibleAction(core.ActionRename)
	a.PerformAccessibleAction(core.ActionDelete)
	tv.BeginRename("b")
	a.HoverIn(&event.HoverEvent{})
	_, pressable := a.pressShape()
	assert.False(t, a.hovered, "a disabled tree's row took a hover")
	assert.False(t, ra.renameBtn.Visible() || ra.deleteBtn.Visible(), "a disabled tree's row shows its hover buttons")
	assert.False(t, pressable, "a disabled tree's row shows a press")
	assert.Nil(t, c.Overlays().Top(), "a disabled tree opened a context menu")
	dragC()
	test.Scroll(c, pos, 0, -200)
	test.Drag(c, pos, 0, -200)
	assert.False(t, tv.scroller.TryScrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -200}}))
	tv.currentFocus = "a"
	tv.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	tv.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	tv.TypedKey(&event.KeyEvent{Name: event.KeyDelete})

	assert.Zero(t, selected, "OnSelected fired")
	assert.Zero(t, opened, "OnBranchOpened fired")
	assert.Zero(t, moved, "OnMoved fired")
	assert.Zero(t, deleted, "OnDeleted fired")
	assert.False(t, tv.IsBranchOpen("a"))
	assert.Equal(t, []TreeViewNodeID{"a", "b", "c"}, tv.ChildIDs("")[:3])
	assert.Equal(t, "", tv.Selected())
	for _, id := range []TreeViewNodeID{"a", "b", "c"} {
		item, _ := treeViewRow(t, tv, id)
		assert.False(t, item.renaming, "a disabled tree opened a rename editor on %s", id)
	}
	assert.Zero(t, tv.scroller.Offset.Y, "the tree scrolled")
	assertTabSkips(t, c, other)

	// The look: buttons, node icon, default label, and a row selected from code.
	assert.True(t, ra.chevron.Disabled(), "the expand button is not disabled")
	assert.True(t, ra.renameBtn.Disabled() && ra.deleteBtn.Disabled(), "the row buttons are not disabled")
	assert.IsType(t, &theme.DisabledResource{}, ra.icon.Resource)
	assert.Equal(t, LowImportance, ra.content.(*Label).Importance)
	tv.Select("a")
	selected = 0
	assert.Equal(t, themeColor(tv, theme.ColorNameDisabledButton), ra.bg.FillColor)

	tv.Enable()
	assert.False(t, tv.Disabled())
	_, ra = treeViewRow(t, tv, "a")
	assert.False(t, ra.chevron.Disabled())
	assert.Equal(t, theme.FolderIcon(), ra.icon.Resource)
	assert.Equal(t, MediumImportance, ra.content.(*Label).Importance)
	assert.Equal(t, themeColor(tv, theme.ColorNameSecondaryContainer), ra.bg.FillColor)

	// Enabled, the same input works.
	test.Tap(ra.chevron)
	assert.Equal(t, 1, opened, "an enabled tree's expand button did not open the branch")
	b, _ := treeViewRow(t, tv, "b")
	test.Tap(b)
	assert.Equal(t, 1, selected, "an enabled tree's row did not select on a tap")
	cRow, _ = treeViewRow(t, tv, "c")
	dragC()
	assert.Equal(t, 1, moved, "an enabled tree's row did not move on a drag")
	test.Scroll(c, pos, 0, -50)
	assert.NotZero(t, tv.scroller.Offset.Y, "an enabled tree did not scroll")

	// Disabled after the first draw, mid-rename and with the focus in the
	// editor: the editor closes and the focus goes.
	b, rb := treeViewRow(t, tv, "b")
	tv.BeginRename("b")
	require.True(t, b.renaming)
	require.Equal(t, rb.renameEntry, c.Focused())
	tv.Disable()
	assert.False(t, b.renaming, "a disabled tree kept its rename editor open")
	assert.Nil(t, c.Focused(), "a disabled tree kept the focus")
	assertTabSkips(t, c, other)
}

func TestCarousel_Disable(t *testing.T) {
	test.NewTempApp(t)
	changed := 0
	car := NewCarousel(NewLabel("1"), NewLabel("2"), NewLabel("3"))
	car.OnChanged = func(int) { changed++ }
	car.CurrentItem = 1 // both buttons shown

	car.Disable() // before the first draw
	assert.True(t, car.Disabled())
	w, other := disabledTestWindow(t, car, geo.NewSize(300, 240))
	c := w.Canvas()
	r := cache.Renderer(car).(*carouselRenderer)
	mid := inside(car, geo.NewPos(150, 80))

	test.Tap(r.nextBtn)
	test.Tap(r.prevBtn)
	test.Drag(c, mid, -150, 0)
	test.DragSteps(c, mid, 150, 0, 5, nil)
	assert.False(t, car.ClaimsDrag(geo.Delta{DX: -20}), "a disabled carousel claimed a swipe")
	// The carousel has no scroll, key or focus of its own, and its buttons
	// (IconButton) take no focus either: Tab has nothing here to skip, which
	// assertTabSkips below still checks.

	assert.Zero(t, changed, "OnChanged fired")
	assert.Equal(t, 1, car.CurrentItem)
	assert.Zero(t, car.offset, "the content moved")
	assertTabSkips(t, c, other)

	// The look: the buttons and the dots.
	assert.True(t, r.prevBtn.Disabled() && r.nextBtn.Disabled(), "the buttons are not disabled")
	assert.Equal(t, themeColor(car, theme.ColorNameDisabled), r.dots[1].FillColor)
	assert.Equal(t, themeColor(car, theme.ColorNameDisabledButton), r.dots[0].FillColor)

	car.Enable()
	assert.False(t, car.Disabled())
	assert.False(t, r.prevBtn.Disabled() || r.nextBtn.Disabled())
	assert.Equal(t, themeColor(car, theme.ColorNamePrimary), r.dots[1].FillColor)

	// Enabled, the same input works.
	test.Drag(c, mid, -150, 0)
	assert.Equal(t, 2, car.CurrentItem, "an enabled carousel did not page on a swipe")
	test.Tap(r.prevBtn)
	assert.Equal(t, 1, car.CurrentItem)
	assert.Equal(t, 2, changed)

	// Disabled after the first draw, mid-swipe: the swipe ends on the
	// current item.
	car.Dragged(&event.DragEvent{Dragged: geo.Delta{DX: -100}})
	require.NotZero(t, car.offset)
	car.Disable()
	car.DragEnd()
	assert.Zero(t, car.offset, "the swipe did not settle back") // animations are off in the test app
	assert.Equal(t, 1, car.CurrentItem)
	assert.Equal(t, 2, changed)
	assertTabSkips(t, c, other)
}

func TestSwipeArea_Disable(t *testing.T) {
	for _, follow := range []bool{false, true} {
		t.Run(fmt.Sprintf("Follow=%v", follow), func(t *testing.T) {
			test.NewTempApp(t)
			swiped, progress, taps := 0, 0, 0
			content := NewButton("x", func() { taps++ })
			area := NewSwipeArea(content, func(SwipeDirection) { swiped++ })
			area.Follow = follow
			area.OnSwipeProgress = func(SwipeDirection, float32) { progress++ }

			area.Disable() // before the first draw
			assert.True(t, area.Disabled())
			w, _ := disabledTestWindow(t, area, geo.NewSize(300, 300))
			c := w.Canvas()
			mid := inside(area, geo.NewPos(150, 150))

			area.Dragged(&event.DragEvent{Dragged: geo.Delta{DX: 150}})
			area.DragEnd()
			test.DragSteps(c, mid, -150, 0, 5, nil)
			assert.False(t, area.ClaimsDrag(geo.Delta{DX: 20}), "a disabled area claimed a swipe")
			// A SwipeArea has no scroll, key or focus of its own, and draws
			// nothing of its own to give a disabled look. Its content is the
			// app's: a tap still reaches it.
			test.Tap(area)
			assert.Zero(t, swiped, "OnSwiped fired")
			assert.Zero(t, progress, "OnSwipeProgress fired")
			assert.Equal(t, 1, taps, "the content's tap did not reach it")
			assert.False(t, content.Disabled(), "the app's content was disabled")

			area.Enable()
			assert.False(t, area.Disabled())
			assert.True(t, area.ClaimsDrag(geo.Delta{DX: 20}))
			test.DragSteps(c, mid, -150, 0, 5, nil)
			assert.Equal(t, 1, swiped, "an enabled area did not report a swipe")

			// Disabled after the first draw, mid-drag: the drag ends unreported.
			swiped = 0
			area.Dragged(&event.DragEvent{Dragged: geo.Delta{DX: 150}})
			area.Disable()
			area.DragEnd()
			assert.Zero(t, swiped, "a swipe begun before Disable was reported")
			assert.Zero(t, area.offset, "the card did not spring back") // animations are off in the test app
		})
	}
}
