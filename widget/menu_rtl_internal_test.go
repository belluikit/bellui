package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// inRTL sets the global layout direction to right-to-left for the test.
func inRTL(t *testing.T) {
	t.Helper()
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
}

// rtlTestItem is a checked item with an icon, a Ctrl+S shortcut and a
// submenu, the case the item layout has to mirror whole (#769).
func rtlTestItem() *bellui.MenuItem {
	item := bellui.NewMenuItem("Save", nil)
	item.Checked = true
	item.Icon = theme.DocumentSaveIcon()
	item.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyS, Modifier: event.KeyModifierControl}
	item.ChildMenu = bellui.NewMenu("", bellui.NewMenuItem("Sub", nil))
	return item
}

// laidOutItem lays out a menu holding item at a fixed width and returns its
// renderer.
func laidOutItem(t *testing.T, item *bellui.MenuItem) *menuItemRenderer {
	t.Helper()
	m := NewMenu(bellui.NewMenu("", item))
	mi := m.Items[0].(*menuItem)
	r := test.TempWidgetRenderer(t, mi).(*menuItemRenderer)
	size := geo.NewSize(r.MinSize().Width+100, r.MinSize().Height)
	mi.Resize(size)
	r.Layout(size)
	return r
}

// Under RTL every part of the item sits where its LTR position mirrors to,
// and the shortcut keeps its modifier before its key as one block (#769).
func TestMenuItem_LayoutMirrorsUnderRTL(t *testing.T) {
	test.NewTempApp(t)
	ltr := laidOutItem(t, rtlTestItem())
	require.Len(t, ltr.shortcutTexts, 2, "a modifier and a key")
	width := ltr.i.Size().Width

	inRTL(t)
	rtl := laidOutItem(t, rtlTestItem())

	mirrored := func(x, w float32) float32 { return width - x - w }
	for name, pair := range map[string][2]*geo.Position{
		"check": {ptr(ltr.checkIcon.Position()), ptr(rtl.checkIcon.Position())},
		"icon":  {ptr(ltr.icon.Position()), ptr(rtl.icon.Position())},
		"text":  {ptr(ltr.text.Position()), ptr(rtl.text.Position())},
		"arrow": {ptr(ltr.expandIcon.Position()), ptr(rtl.expandIcon.Position())},
	} {
		var w float32
		switch name {
		case "text":
			w = ltr.text.Size().Width
		default:
			w = theme.IconInlineSize()
		}
		assert.Equal(t, mirrored(pair[0].X, w), pair[1].X, name)
		assert.Equal(t, pair[0].Y, pair[1].Y, name)
	}
	assert.Equal(t, float32(0), rtl.expandIcon.Position().X, "the arrow is at the left edge")
	assert.Greater(t, rtl.checkIcon.Position().X, rtl.icon.Position().X, "the check is right of the icon")
	assert.Greater(t, rtl.icon.Position().X, rtl.text.Position().X, "the icon is right of the label")

	mod, key := rtl.shortcutTexts[0], rtl.shortcutTexts[1]
	assert.Equal(t, mod.Position().X+mod.Size().Width, key.Position().X, "the key follows its modifier")
	ltrBlockEnd := ltr.shortcutTexts[1].Position().X + ltr.shortcutTexts[1].Size().Width
	assert.Equal(t, mirrored(ltr.shortcutTexts[0].Position().X, ltrBlockEnd-ltr.shortcutTexts[0].Position().X), mod.Position().X,
		"the shortcut block is mirrored whole")
	assert.Less(t, key.Position().X+key.Size().Width, rtl.text.Position().X, "the shortcut is left of the label")
}

// An item checked after it was laid out shows its check on the right under
// RTL: Refresh shows the hidden check without laying the item out again.
func TestMenuItem_CheckShownLaterIsMirroredUnderRTL(t *testing.T) {
	test.NewTempApp(t)
	inRTL(t)
	item := rtlTestItem()
	item.Checked = false
	r := laidOutItem(t, item)
	require.False(t, r.checkIcon.Visible())

	item.Checked = true
	r.Refresh()
	assert.True(t, r.checkIcon.Visible())
	width := r.i.Size().Width
	assert.Equal(t, width-theme.InnerPadding()-theme.IconInlineSize(), r.checkIcon.Position().X)
}

// rtlSubmenuTest shows a pop-up menu whose first item has a submenu at x in
// a 400x300 window, and opens that submenu.
func rtlSubmenuTest(t *testing.T, x float32) (*PopUpMenu, *menuItem) {
	t.Helper()
	test.NewTempApp(t)
	sub := bellui.NewMenu("", bellui.NewMenuItem("Sub one", nil), bellui.NewMenuItem("Sub two", nil))
	parent := bellui.NewMenuItem("Parent", nil)
	parent.ChildMenu = sub
	w := test.NewTempWindow(t, NewLabel(""))
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))
	m := NewPopUpMenu(bellui.NewMenu("", parent, bellui.NewMenuItem("Other", nil)), w.Canvas())
	m.ShowAtPosition(geo.NewPos(x, 10))
	m.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	return m, m.Items[0].(*menuItem)
}

// Under RTL a submenu opens to the left of its item (#769).
func TestPopUpMenu_SubmenuOpensLeftUnderRTL(t *testing.T) {
	inRTL(t)
	m, item := rtlSubmenuTest(t, 250)
	m.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	require.True(t, item.isSubmenuOpen(), "Left opens the submenu under RTL")
	assert.Equal(t, -item.Child().Size().Width, item.Child().Position().X)
}

// With no room on the left, an RTL submenu flips to the right.
func TestPopUpMenu_SubmenuFlipsRightUnderRTLWithoutRoom(t *testing.T) {
	inRTL(t)
	m, item := rtlSubmenuTest(t, 0)
	m.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	require.True(t, item.isSubmenuOpen())
	assert.Equal(t, item.Size().Width, item.Child().Position().X)
}

// Left opens a submenu and Right closes it under RTL; the reverse in LTR.
func TestPopUpMenu_LeftRightSwapUnderRTL(t *testing.T) {
	t.Run("ltr", func(t *testing.T) {
		m, item := rtlSubmenuTest(t, 10)
		m.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
		assert.False(t, item.isSubmenuOpen(), "Left does not open in LTR")
		m.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		assert.True(t, item.isSubmenuOpen(), "Right opens in LTR")
		assert.Equal(t, item.Size().Width, item.Child().Position().X, "to the right")
		m.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
		assert.False(t, item.isSubmenuOpen(), "Left closes in LTR")
	})
	t.Run("rtl", func(t *testing.T) {
		inRTL(t)
		m, item := rtlSubmenuTest(t, 250)
		m.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		assert.False(t, item.isSubmenuOpen(), "Right does not open under RTL")
		m.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
		assert.True(t, item.isSubmenuOpen(), "Left opens under RTL")
		m.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		assert.False(t, item.isSubmenuOpen(), "Right closes under RTL")
	})
}

func ptr(p geo.Position) *geo.Position { return &p }

// A pop-up on a canvas with its own RTL direction, in an LTR app, mirrors its
// items once shown, though they were laid out before it was on the canvas,
// and opens its submenus to the left (#769).
func TestPopUpMenu_ItemsMirrorOnRTLCanvas(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, NewLabel(""))
	w.SetPadded(false)
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	w.Resize(geo.NewSize(400, 300))
	m := NewPopUpMenu(bellui.NewMenu("", rtlTestItem(), bellui.NewMenuItem("Other", nil)), w.Canvas())
	m.ShowAtPosition(geo.NewPos(250, 10))

	item := m.Items[0].(*menuItem)
	r := test.TempWidgetRenderer(t, item).(*menuItemRenderer)
	width := item.Size().Width
	assert.Equal(t, width-theme.InnerPadding()-theme.IconInlineSize(), r.checkIcon.Position().X, "the check is at the right")
	assert.Equal(t, float32(0), r.expandIcon.Position().X, "the arrow is at the left edge")

	m.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	m.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	require.True(t, item.isSubmenuOpen())
	assert.Equal(t, -item.Child().Size().Width, item.Child().Position().X)
}
