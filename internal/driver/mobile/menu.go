package mobile

import (
	"image/color"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

type menuLabel struct {
	widget.BaseWidget

	menu   *bellui.Menu
	bar    *bellui.Container
	canvas *canvas
}

func (m *menuLabel) Tapped(*event.PointEvent) {
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(m)
	menu := widget.NewPopUpMenu(m.menu, m.canvas)
	menu.ShowAtPosition(geo.NewPos(pos.X+m.Size().Width, pos.Y))

	menuDismiss := menu.OnDismiss // this dismisses the menu stack
	menu.OnDismiss = func() {
		menuDismiss()
		m.bar.Hide() // dismiss the overlay menu bar
		m.canvas.setMenu(nil)
	}
}

func (m *menuLabel) CreateRenderer() core.WidgetRenderer {
	label := widget.NewLabel(m.menu.Label)
	box := container.NewHBox(layout.NewSpacer(), label, layout.NewSpacer(), widget.NewIcon(theme.MenuExpandIcon()))

	return &menuLabelRenderer{menu: m, content: box}
}

func newMenuLabel(item *bellui.Menu, parent *bellui.Container, c *canvas) *menuLabel {
	l := &menuLabel{menu: item, bar: parent, canvas: c}
	l.ExtendBaseWidget(l)
	return l
}

func (c *canvas) showMenu(menu *bellui.MainMenu) {
	var panel *bellui.Container
	top := container.NewHBox(widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		panel.Hide()
		c.setMenu(nil)
	}))
	panel = container.NewVBox(top)
	for _, item := range menu.Items {
		panel.Add(newMenuLabel(item, panel, c))
	}
	if c.padded {
		panel = container.NewPadded(panel)
	}

	bg := belluicanvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	shadow := belluicanvas.NewHorizontalGradient(theme.Color(theme.ColorNameShadow), color.Transparent)

	safePos, safeSize := c.InteractiveArea()
	bg.Move(safePos)
	bg.Resize(geo.NewSize(panel.MinSize().Width+theme.Padding(), safeSize.Height))
	panel.Move(safePos)
	panel.Resize(geo.NewSize(panel.MinSize().Width+theme.Padding(), safeSize.Height))
	shadow.Resize(geo.NewSize(theme.Padding()/2, safeSize.Height))
	shadow.Move(geo.NewPos(panel.Size().Width+safePos.X, safePos.Y))

	c.setMenu(container.NewWithoutLayout(bg, panel, shadow))
}

func (d *driver) findMenu(win *window) *bellui.MainMenu {
	if win.menu != nil {
		return win.menu
	}

	matched := false
	for x := len(d.windows) - 1; x >= 0; x-- {
		w := d.windows[x]
		if !matched {
			if w == win {
				matched = true
			}
			continue
		}

		if w.(*window).menu != nil {
			return w.(*window).menu
		}
	}

	return nil
}

type menuLabelRenderer struct {
	menu    *menuLabel
	content *bellui.Container
}

func (m *menuLabelRenderer) BackgroundColor() color.Color {
	return theme.Color(theme.ColorNameBackground)
}

func (m *menuLabelRenderer) Destroy() {
}

func (m *menuLabelRenderer) Layout(size geo.Size) {
	m.content.Resize(size)
}

func (m *menuLabelRenderer) MinSize() geo.Size {
	return m.content.MinSize()
}

func (m *menuLabelRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{m.content}
}

func (m *menuLabelRenderer) Refresh() {
	m.content.Refresh()
}
