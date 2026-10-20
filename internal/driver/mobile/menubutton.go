//go:build !wasm

package mobile

import (
	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

type menuButton struct {
	widget.BaseWidget
	win  *window
	menu *bellui.MainMenu
}

func (w *window) newMenuButton(menu *bellui.MainMenu) *menuButton {
	b := &menuButton{win: w, menu: menu}
	b.ExtendBaseWidget(b)
	return b
}

func (m *menuButton) CreateRenderer() core.WidgetRenderer {
	return &menuButtonRenderer{btn: widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		m.win.canvas.showMenu(m.menu)
	}), bg: belluicanvas.NewRectangle(theme.Color(theme.ColorNameBackground))}
}

type menuButtonRenderer struct {
	btn *widget.Button
	bg  *belluicanvas.Rectangle
}

func (m *menuButtonRenderer) Destroy() {
}

func (m *menuButtonRenderer) Layout(size geo.Size) {
	m.bg.Move(geo.NewPos(theme.Padding()/2, theme.Padding()/2))
	m.bg.Resize(size.Subtract(geo.NewSize(theme.Padding(), theme.Padding())))
	m.btn.Resize(size)
}

func (m *menuButtonRenderer) MinSize() geo.Size {
	return m.btn.MinSize()
}

func (m *menuButtonRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{m.bg, m.btn}
}

func (m *menuButtonRenderer) Refresh() {
	m.bg.FillColor = theme.Color(theme.ColorNameBackground)
}
