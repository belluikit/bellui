package widget

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
)

// ButtonGroup is an MD3 connected group of buttons sharing borders.
type ButtonGroup struct {
	BaseWidget
	Buttons []*Button
}

// NewButtonGroup creates a connected button group.
func NewButtonGroup(buttons ...*Button) *ButtonGroup {
	for _, btn := range buttons {
		btn.ExtendBaseWidget(btn)
	}
	g := &ButtonGroup{Buttons: buttons}
	g.ExtendBaseWidget(g)
	return g
}

func (g *ButtonGroup) MinSize() geo.Size {
	g.ExtendBaseWidget(g)
	return g.BaseWidget.MinSize()
}

func (g *ButtonGroup) CreateRenderer() core.WidgetRenderer {
	g.ExtendBaseWidget(g)
	var objects []core.CanvasObject
	for _, btn := range g.Buttons {
		objects = append(objects, btn)
	}

	r := &buttonGroupRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		group:        g,
	}
	r.Refresh()
	return r
}

type buttonGroupRenderer struct {
	internalWidget.BaseRenderer
	group *ButtonGroup
}

func (r *buttonGroupRenderer) MinSize() geo.Size {
	w := float32(0)
	h := float32(0)
	for _, btn := range r.group.Buttons {
		min := btn.MinSize()
		w += min.Width
		if min.Height > h {
			h = min.Height
		}
	}
	return geo.NewSize(w, h)
}

func (r *buttonGroupRenderer) Layout(size geo.Size) {
	n := len(r.group.Buttons)
	if n == 0 {
		return
	}
	btnW := size.Width / float32(n)
	for i, btn := range r.group.Buttons {
		btn.Move(geo.NewPos(float32(i)*btnW, 0))
		btn.Resize(geo.NewSize(btnW, size.Height))
	}

	// Under RTL reflect the button row so the first button sits on the right.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *buttonGroupRenderer) Refresh() {
	for _, btn := range r.group.Buttons {
		btn.Refresh()
	}
	r.Layout(r.group.Size())
	canvas.Refresh(r.group.super())
}
