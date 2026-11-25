package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// overflowBar shows a 64 dp high bar with a navigation button and actions in
// a window of the given width, laid out in dir, and returns its renderer.
func overflowBar(t *testing.T, dir bellui.Direction, width float32, actions ...TopAppBarAction) (*TopAppBar, *topAppBarRenderer) {
	t.Helper()
	test.NewTempApp(t)
	bellui.SetLayoutDirection(dir)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bar := NewTopAppBar("Communication", theme.NavigateBackIcon(), nil)
	bar.Actions = actions
	// The bar at the top of a phone-high window, so its menu has room below.
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{bar}})
	w.SetPadded(false)
	w.Resize(geo.NewSize(width, 640))
	require.Equal(t, geo.NewSize(width, 64), bar.Size())
	return bar, test.TempWidgetRenderer(t, bar).(*topAppBarRenderer)
}

// labelled returns n labelled actions, "A0".."A<n-1>", each counting its taps
// in taps[i].
func labelled(n int, taps []int) []TopAppBarAction {
	actions := make([]TopAppBarAction, n)
	for i := range actions {
		actions[i] = TopAppBarAction{
			Icon:     theme.SearchIcon(),
			Label:    "A" + string(rune('0'+i)),
			OnTapped: func() { taps[i]++ },
		}
	}
	return actions
}

// onBar returns the buttons in the renderer's objects after the navigation
// button: what the bar draws, in object order.
func onBar(r *topAppBarRenderer) (buttons []*IconButton) {
	for _, o := range r.Objects() {
		if b, ok := o.(*IconButton); ok && b != r.navBtn {
			buttons = append(buttons, b)
		}
	}
	return buttons
}

// assertNoOverlap checks that the title and every button on the bar keep
// apart, and that the title has the three button widths folding keeps for it.
func assertNoOverlap(t *testing.T, r *topAppBarRenderer, wantRoom bool) {
	t.Helper()
	objs := []core.CanvasObject{r.title, r.navBtn}
	for _, b := range onBar(r) {
		objs = append(objs, b)
	}
	for i, a := range objs {
		for _, b := range objs[i+1:] {
			apart := a.Position().X+a.Size().Width <= b.Position().X || b.Position().X+b.Size().Width <= a.Position().X
			assert.True(t, apart, "%T at %v (%v) overlaps %T at %v (%v)", a, a.Position(), a.Size(), b, b.Position(), b.Size())
		}
	}
	if wantRoom {
		// The room the title had: between the nav button and the first
		// button on the bar, which is the one nearest the nav button in
		// either direction.
		btnSize := r.bar.Theme().Size(theme.SizeNameIconButtonSize)
		nav, first := r.navBtn, onBar(r)[0]
		room := first.Position().X - (nav.Position().X + nav.Size().Width) // LTR
		if nav.Position().X > first.Position().X {
			room = nav.Position().X - (first.Position().X + first.Size().Width) // RTL
		}
		// The gaps either side of the title, padding*0.5 each, are not the
		// title's: 0.4 of a button each side.
		assert.GreaterOrEqual(t, room-2*0.2*btnSize, 3*btnSize, "the title keeps three button widths")
	}
}

func TestTopAppBar_Overflow_FoldsAtPhoneWidth(t *testing.T) {
	taps := make([]int, 5)
	_, r := overflowBar(t, bellui.DirectionLTR, 360, labelled(5, taps)...)

	assert.Equal(t, []int{2, 3, 4}, r.folded, "three fold, leaving two actions and the overflow button")
	buttons := onBar(r)
	require.Len(t, buttons, 3)
	assert.Same(t, r.actionBtns[0], buttons[0])
	assert.Same(t, r.actionBtns[1], buttons[1])
	assert.Same(t, r.overflowBtn, buttons[2], "the overflow button is the last")
	assert.Greater(t, buttons[2].Position().X, buttons[1].Position().X)
	assertNoOverlap(t, r, true)
}

func TestTopAppBar_Overflow_UnfoldsAndFoldsOnResize(t *testing.T) {
	taps := make([]int, 5)
	bar, r := overflowBar(t, bellui.DirectionLTR, 360, labelled(5, taps)...)
	require.NotEmpty(t, r.folded)

	bar.Resize(geo.NewSize(600, 64))
	assert.Empty(t, r.folded, "every action fits at 600")
	assert.Equal(t, r.actionBtns, onBar(r), "the bar shows every action and no overflow button")
	assert.Equal(t, "Communication", r.title.Text)
	assertNoOverlap(t, r, true)

	bar.Resize(geo.NewSize(360, 64))
	assert.Equal(t, []int{2, 3, 4}, r.folded)
	assert.Contains(t, onBar(r), r.overflowBtn)
	assertNoOverlap(t, r, true)
}

func TestTopAppBar_Overflow_RTL(t *testing.T) {
	taps := make([]int, 5)
	_, r := overflowBar(t, bellui.DirectionRTL, 360, labelled(5, taps)...)

	assert.Equal(t, []int{2, 3, 4}, r.folded)
	buttons := onBar(r)
	require.Len(t, buttons, 3)
	// Mirrored: the overflow button at the left end, then the two actions,
	// then the title and the navigation button at the right.
	assert.Less(t, r.overflowBtn.Position().X, r.actionBtns[1].Position().X)
	assert.Less(t, r.actionBtns[1].Position().X, r.actionBtns[0].Position().X)
	assert.Less(t, r.actionBtns[0].Position().X, r.title.Position().X)
	assert.Less(t, r.title.Position().X, r.navBtn.Position().X)
	assertNoOverlap(t, r, true)
}

func TestTopAppBar_Overflow_UnlabelledNeverFolds(t *testing.T) {
	actions := labelled(5, make([]int, 5))
	for i := range actions {
		actions[i].Label = ""
	}
	_, r := overflowBar(t, bellui.DirectionLTR, 360, actions...)

	assert.Empty(t, r.folded)
	assert.Equal(t, r.actionBtns, onBar(r), "no overflow button: as before #1456")
	assertNoOverlap(t, r, false) // the title is cut instead (#1454)
}

func TestTopAppBar_Overflow_PinnedStays(t *testing.T) {
	actions := labelled(5, make([]int, 5))
	actions[3].Pinned = true
	_, r := overflowBar(t, bellui.DirectionLTR, 360, actions...)

	// From the end, past the pinned A3: A4, A2, then A1 to reach the room.
	assert.Equal(t, []int{1, 2, 4}, r.folded)
	buttons := onBar(r)
	require.Len(t, buttons, 3)
	assert.Same(t, r.actionBtns[0], buttons[0])
	assert.Same(t, r.actionBtns[3], buttons[1], "the pinned action stays, in its place")
	assert.Same(t, r.overflowBtn, buttons[2])
	assertNoOverlap(t, r, true)
}

func TestTopAppBar_Overflow_NeverFoldsOneAlone(t *testing.T) {
	// Only the last action may fold. Folding it would put the overflow
	// button in its place and gain nothing.
	actions := labelled(5, make([]int, 5))
	for i := range 4 {
		actions[i].Label = ""
	}
	_, r := overflowBar(t, bellui.DirectionLTR, 360, actions...)

	assert.Empty(t, r.folded)
	assert.NotContains(t, onBar(r), r.overflowBtn)
}

// menuItems returns the items of the overflow menu the renderer last opened.
func menuItems(t *testing.T, r *topAppBarRenderer) []*menuItem {
	t.Helper()
	require.NotNil(t, r.menu, "the overflow menu opened")
	var items []*menuItem
	for _, o := range r.menu.Items {
		if it, ok := o.(*menuItem); ok {
			items = append(items, it)
		}
	}
	return items
}

func TestTopAppBar_Overflow_MenuTaps(t *testing.T) {
	taps := make([]int, 5)
	_, r := overflowBar(t, bellui.DirectionLTR, 360, labelled(5, taps)...)

	test.Tap(r.overflowBtn)
	items := menuItems(t, r)
	require.Len(t, items, 3)
	for i, want := range []string{"A2", "A3", "A4"} {
		assert.Equal(t, want, items[i].Item.Label)
		assert.Equal(t, theme.SearchIcon(), items[i].Item.Icon, "the item shows the action's icon")
		assert.False(t, items[i].Item.Disabled)
	}
	assert.True(t, r.menu.Visible())

	test.Tap(items[1])
	assert.Equal(t, []int{0, 0, 0, 1, 0}, taps, "choosing A3 calls its OnTapped, and only that")
	assert.False(t, r.menu.Visible(), "choosing an item closes the menu")

	// The menu opens under the overflow button, its right edge at the
	// button's right edge.
	test.Tap(r.overflowBtn)
	btn := bellui.CurrentApp().Driver().AbsolutePositionForObject(r.overflowBtn)
	assert.InDelta(t, btn.Y+r.overflowBtn.Size().Height, r.menu.Position().Y, 0.5)
	assert.InDelta(t, btn.X+r.overflowBtn.Size().Width, r.menu.Position().X+r.menu.MinSize().Width, 0.5)
}

func TestTopAppBar_Overflow_MenuOpensFromLeftUnderRTL(t *testing.T) {
	_, r := overflowBar(t, bellui.DirectionRTL, 360, labelled(5, make([]int, 5))...)

	test.Tap(r.overflowBtn)
	require.NotNil(t, r.menu)
	btn := bellui.CurrentApp().Driver().AbsolutePositionForObject(r.overflowBtn)
	assert.InDelta(t, btn.X, r.menu.Position().X, 0.5, "the menu's left edge at the button's left edge")
	assert.InDelta(t, btn.Y+r.overflowBtn.Size().Height, r.menu.Position().Y, 0.5)
}

func TestTopAppBar_Overflow_DisabledBar(t *testing.T) {
	taps := make([]int, 5)
	bar, r := overflowBar(t, bellui.DirectionLTR, 360, labelled(5, taps)...)
	bar.Disable()

	assert.False(t, r.overflowBtn.Disabled(), "the overflow button stays pressable")
	assert.True(t, r.actionBtns[0].Disabled(), "the other buttons are disabled (#1251)")

	test.Tap(r.overflowBtn)
	items := menuItems(t, r)
	require.Len(t, items, 3)
	for _, it := range items {
		assert.True(t, it.Item.Disabled, "%s is disabled", it.Item.Label)
	}
	test.Tap(items[0])
	assert.Equal(t, []int{0, 0, 0, 0, 0}, taps, "a disabled item calls nothing")

	// An item reached some other way, such as a menu left open while the
	// bar was disabled, still calls nothing.
	items[0].Item.Action()
	assert.Equal(t, []int{0, 0, 0, 0, 0}, taps)

	bar.Enable()
	test.Tap(r.overflowBtn)
	items = menuItems(t, r)
	assert.False(t, items[0].Item.Disabled)
	test.Tap(items[0])
	assert.Equal(t, []int{0, 0, 1, 0, 0}, taps)
}

func TestTopAppBar_Overflow_AccessibleNames(t *testing.T) {
	_, r := overflowBar(t, bellui.DirectionLTR, 360, labelled(5, make([]int, 5))...)

	for i, btn := range r.actionBtns {
		node, ok := core.SnapshotAccessible(btn)
		require.True(t, ok)
		assert.Equal(t, r.actions[i].Label, node.Label, "an action button is named by its label")
	}
	node, _ := core.SnapshotAccessible(r.overflowBtn)
	assert.Equal(t, "More options", node.Label)
}
