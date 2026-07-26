//go:build !windows || !ci

package mobile

import (
	"testing"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestMobileCanvas_DismissBar(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(belluicanvas.NewRectangle(theme.Color(theme.ColorNameBackground)))
	menu := bellui.NewMainMenu(
		bellui.NewMenu("Test"),
	)
	c.showMenu(menu)
	c.Resize(geo.NewSize(100, 100))

	assert.NotNil(t, c.menu)
	// simulate tap as the test util does not know about our menu...
	c.TapDown(geo.NewPos(80, 20), 1)
	c.TapUp(geo.NewPos(80, 20), 1, nil, nil, nil, nil, nil)
	assert.Nil(t, c.menu)
}

func TestMobileCanvas_DismissMenu(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(belluicanvas.NewRectangle(theme.Color(theme.ColorNameBackground)))
	menu := bellui.NewMainMenu(
		bellui.NewMenu("Test", bellui.NewMenuItem("TapMe", func() {})),
	)
	c.showMenu(menu)
	c.Resize(geo.NewSize(100, 100))

	assert.NotNil(t, c.menu)
	menuObj := c.menu.(*bellui.Container).Objects[1].(*bellui.Container).Objects[1].(*menuLabel)
	point := &event.PointEvent{Position: geo.NewPos(10, 10)}
	menuObj.Tapped(point)

	tapMeItem := c.Overlays().Top().(*internalWidget.OverlayContainer).Content.(*widget.PopUpMenu).Items[0].(event.Tappable)
	tapMeItem.Tapped(point)
	assert.Nil(t, c.menu)
}

func TestMobileCanvas_Menu(t *testing.T) {
	c := &canvas{}
	labels := []string{"File", "Edit"}
	menu := bellui.NewMainMenu(
		bellui.NewMenu(labels[0]),
		bellui.NewMenu(labels[1]),
	)

	c.showMenu(menu)
	menuObjects := c.menu.(*bellui.Container).Objects[1].(*bellui.Container)
	assert.Equal(t, 3, len(menuObjects.Objects))
	header, ok := menuObjects.Objects[0].(*bellui.Container)
	assert.True(t, ok)
	closed, ok := header.Objects[0].(*widget.Button)
	assert.True(t, ok)
	assert.Equal(t, theme.CancelIcon().Name(), closed.Icon.Name())

	for i := 1; i < 3; i++ {
		item, ok := menuObjects.Objects[i].(*menuLabel)
		assert.True(t, ok)
		assert.Equal(t, labels[i-1], item.menu.Label)
	}
}

func TestMobileCanvas_MenuChild(t *testing.T) {
	c := &canvas{}
	c.Initialize(c, nil)
	c.Resize(geo.NewSize(100, 200))

	child := bellui.NewMenu("Child", bellui.NewMenuItem("One", func() {}))
	parent := bellui.NewMenuItem("Parent", func() {})
	parent.ChildMenu = child
	menu := bellui.NewMainMenu(
		bellui.NewMenu("Top", parent),
	)

	c.showMenu(menu)
	topObj := c.menu.(*bellui.Container).Objects[1].(*bellui.Container).Objects[1].(*menuLabel)
	assert.Equal(t, "Top", topObj.menu.Label)

	test.Tap(topObj)
	assert.Equal(t, 1, len(c.Overlays().List()))
	rootOverlay := c.Overlays().Top()
	parentItem := rootOverlay.(*internalWidget.OverlayContainer).Content.(*widget.PopUpMenu).Items[0]
	parentDetails := test.WidgetRenderer(parentItem.(core.Widget))
	text := parentDetails.Objects()[1].(*belluicanvas.Text)
	assert.Equal(t, "Parent", text.Text)

	test.Tap(parentItem.(event.Tappable))
	assert.NotNil(t, c.menu) // still visible

	childMenu := parentItem.(interface{ Child() *widget.Menu }).Child()
	assert.NotNil(t, childMenu)
	assert.True(t, childMenu.Visible())
	assert.Equal(t, 1, len(childMenu.Items))

	childDetails := test.WidgetRenderer(childMenu.Items[0].(core.Widget))
	text = childDetails.Objects()[1].(*belluicanvas.Text)
	assert.Equal(t, "One", text.Text)
}

func dummyWin(d *driver, title string) *window {
	ret := &window{title: title}
	d.windows = append(d.windows, ret)

	return ret
}

func TestMobileDriver_FindMenu(t *testing.T) {
	m1 := bellui.NewMainMenu(bellui.NewMenu("1"))
	m2 := bellui.NewMainMenu(bellui.NewMenu("2"))

	d := NewGoMobileDriver().(*driver)
	w1 := dummyWin(d, "top")
	w1.SetMainMenu(m1)
	assert.Equal(t, m1, d.findMenu(w1))

	w2 := dummyWin(d, "child")
	assert.Equal(t, m1, d.findMenu(w2))

	w2.SetMainMenu(m2)
	assert.Equal(t, m2, d.findMenu(w2))
}
