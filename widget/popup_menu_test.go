package widget_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestPopUpMenu_Move(t *testing.T) {
	m, w := setupPopUpMenuTest(t)
	c := w.Canvas()

	m.Show()
	test.AssertRendersToMarkup(t, "popup_menu/shown.xml", c)

	m.Move(geo.NewPos(20, 20))
	test.AssertRendersToMarkup(t, "popup_menu/moved.xml", c)

	m.Move(geo.NewPos(190, 10))
	test.AssertRendersToMarkup(t, "popup_menu/no_space_right.xml", c)

	m.Move(geo.NewPos(10, 190))
	test.AssertRendersToMarkup(t, "popup_menu/no_space_down.xml", c)
}

func TestPopUpMenu_Resize(t *testing.T) {
	m, w := setupPopUpMenuTest(t)
	c := w.Canvas()

	m.ShowAtPosition(geo.NewPos(10, 10))
	test.AssertRendersToMarkup(t, "popup_menu/shown_at_pos.xml", c)

	m.Resize(m.Size().Add(geo.NewSize(10, 10)))
	test.AssertRendersToMarkup(t, "popup_menu/grown.xml", c)

	largeSize := c.Size().Add(geo.NewSize(10, 10))
	m.Resize(largeSize)
	test.AssertRendersToMarkup(t, "popup_menu/canvas_too_small.xml", c)
	_, areaSize := c.InteractiveArea()
	assert.Equal(t, areaSize, m.Size(), "menu larger than canvas is clamped to the interactive area in both dimensions (menu scrolls, #293)")
}

func TestPopUpMenu_Show(t *testing.T) {
	m, w := setupPopUpMenuTest(t)
	c := w.Canvas()

	test.AssertRendersToMarkup(t, "popup_menu/hidden.xml", c)

	m.Show()
	test.AssertRendersToMarkup(t, "popup_menu/shown.xml", c)
}

func TestPopUpMenu_ShowAtPosition(t *testing.T) {
	m, w := setupPopUpMenuTest(t)
	c := w.Canvas()

	test.AssertRendersToMarkup(t, "popup_menu/hidden.xml", c)

	m.ShowAtPosition(geo.NewPos(10, 10))
	test.AssertRendersToMarkup(t, "popup_menu/shown_at_pos.xml", c)

	m.Hide()
	test.AssertRendersToMarkup(t, "popup_menu/hidden.xml", c)

	m.ShowAtPosition(geo.NewPos(190, 10))
	test.AssertRendersToMarkup(t, "popup_menu/no_space_right.xml", c)

	m.Hide()
	test.AssertRendersToMarkup(t, "popup_menu/hidden.xml", c)

	m.ShowAtPosition(geo.NewPos(10, 190))
	test.AssertRendersToMarkup(t, "popup_menu/no_space_down.xml", c)

	m.Hide()
	test.AssertRendersToMarkup(t, "popup_menu/hidden.xml", c)

	m.ShowAtPosition(geo.NewPos(10, 10))
	menuSize := c.Size().Add(geo.NewSize(10, 10))
	m.Resize(menuSize)
	test.AssertRendersToMarkup(t, "popup_menu/canvas_too_small.xml", c)
	_, areaSize := c.InteractiveArea()
	assert.Equal(t, areaSize, m.Size(), "menu larger than canvas is clamped to the interactive area in both dimensions (menu scrolls, #293)")
}

func setupPopUpMenuTest(t *testing.T) (*widget.PopUpMenu, bellui.Window) {
	test.NewApp()

	w := test.NewTempWindow(t, canvas.NewRectangle(color.NRGBA{G: 150, B: 150, A: 255}))
	t.Cleanup(func() { test.NewApp() })

	w.Resize(geo.NewSize(200, 200))
	m := widget.NewPopUpMenu(bellui.NewMenu(
		"",
		bellui.NewMenuItem("Option A", nil),
		bellui.NewMenuItem("Option B", nil),
	), w.Canvas())
	return m, w
}

func setupPopUpMenuWithSubmenusTest(t *testing.T, callback func(string)) (*widget.PopUpMenu, bellui.Window) {
	test.NewApp()

	w := test.NewTempWindow(t, canvas.NewRectangle(color.NRGBA{G: 150, B: 150, A: 255}))
	t.Cleanup(func() { test.NewApp() })

	w.Resize(geo.NewSize(800, 600))
	itemA := bellui.NewMenuItem("Option A", func() { callback("Option A") })
	itemB := bellui.NewMenuItem("Option B", func() { callback("Option B") })
	itemBA := bellui.NewMenuItem("Sub Option A", func() { callback("Sub Option A") })
	itemBB := bellui.NewMenuItem("Sub Option B", func() { callback("Sub Option B") })
	itemBBA := bellui.NewMenuItem("Sub Sub Option A", func() { callback("Sub Sub Option A") })
	itemBBB := bellui.NewMenuItem("Sub Sub Option B", func() { callback("Sub Sub Option B") })
	itemB.ChildMenu = bellui.NewMenu("", itemBA, itemBB)
	itemBB.ChildMenu = bellui.NewMenu("", itemBBA, itemBBB)
	m := widget.NewPopUpMenu(bellui.NewMenu("", itemA, itemB), w.Canvas())
	return m, w
}
