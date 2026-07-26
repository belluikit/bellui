package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestMenu_RefreshOptions(t *testing.T) {
	test.NewTempApp(t)

	w := bellui.CurrentApp().NewWindow("")
	defer w.Close()
	w.SetPadded(false)
	c := w.Canvas()

	itemFoo := bellui.NewMenuItem("Foo", nil)
	itemBar := bellui.NewMenuItem("Bar", nil)
	itemBar.ChildMenu = bellui.NewMenu("", bellui.NewMenuItem("Sub", nil))
	itemBar.Icon = theme.AccountIcon()
	itemBaz := bellui.NewMenuItem("Baz", nil)

	m := widget.NewMenu(bellui.NewMenu(
		"",
		itemFoo,
		bellui.NewMenuItemSeparator(),
		itemBar,
		bellui.NewMenuItemSeparator(),
		itemBaz,
	))
	w.SetContent(internalWidget.NewOverlayContainer(m, c, nil))
	// + 4,5 for canvas’ safe area
	w.Resize(m.MinSize().AddWidthHeight(4, 5))
	m.Resize(m.MinSize())
	test.AssertRendersToMarkup(t, "menu/refresh_initial.xml", c)

	itemBar.Disabled = true
	m.Refresh()

	test.AssertRendersToMarkup(t, "menu/refresh_disabled.xml", c)

	itemBaz.Checked = true
	m.Refresh()

	test.AssertRendersToMarkup(t, "menu/refresh_checkmark.xml", c)

	itemBar.Checked = true
	m.Refresh()

	test.AssertRendersToMarkup(t, "menu/refresh_2nd_checkmark.xml", c)

	itemBar.Checked = false
	itemBar.Disabled = false
	m.Refresh()

	itemBaz.Checked = false
	m.Refresh()

	test.AssertRendersToMarkup(t, "menu/refresh_initial.xml", c)
}

func TestMenu_TappedPaddingOrSeparator(t *testing.T) {
	test.NewTempApp(t)

	w := bellui.CurrentApp().NewWindow("")
	defer w.Close()
	w.SetPadded(false)
	c := w.Canvas()

	var item1Hit, item2Hit, overlayContainerHit bool
	m := widget.NewMenu(bellui.NewMenu(
		"",
		bellui.NewMenuItem("Foo", func() { item1Hit = true }),
		bellui.NewMenuItemSeparator(),
		bellui.NewMenuItem("Bar", func() { item2Hit = true }),
	))
	size := m.MinSize()
	w.Resize(size.Add(geo.NewSize(10, 10)))
	m.Resize(size)
	o := internalWidget.NewOverlayContainer(m, c, func() { overlayContainerHit = true })
	w.SetContent(o)

	// tap on separator
	p := geo.NewPos(5, size.Height/2)
	if test.AssertCanvasTappableAt(t, c, p) {
		test.TapCanvas(c, p)
		assert.False(t, item1Hit, "item 1 should not be hit")
		assert.False(t, item2Hit, "item 2 should not be hit")
		assert.False(t, overlayContainerHit, "the overlay container should not be hit")
	}

	// verify test setup: we can hit the items and the container
	test.TapCanvas(c, geo.NewPos(5, size.Height/4))
	assert.True(t, item1Hit, "hit item 1")
	assert.False(t, item2Hit, "item 2 should not be hit")
	assert.False(t, overlayContainerHit, "the overlay container should not be hit")
	test.TapCanvas(c, geo.NewPos(5, 3*size.Height/4))
	assert.True(t, item2Hit, "hit item 2")
	assert.False(t, overlayContainerHit, "the overlay container should not be hit")
	test.TapCanvas(c, geo.NewPos(size.Width+1, size.Height+1))
	assert.True(t, overlayContainerHit, "hit the overlay container")
}
