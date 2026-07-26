// Command dragghostprobe live-verifies the in-canvas drag affordances (#819
// PR-1): a grab/grabbing cursor for the duration of a drag and a ghost image
// following the pointer. Drag the coloured box around.
package main

import (
	"image/color"

	bellui "github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// paletteItem is a minimal drag source: Draggable + DragImageProvider (#819).
type paletteItem struct {
	widget.BaseWidget
}

func newPaletteItem() *paletteItem {
	p := &paletteItem{}
	p.ExtendBaseWidget(p)
	return p
}

func (p *paletteItem) CreateRenderer() core.WidgetRenderer {
	r := canvas.NewRectangle(color.NRGBA{R: 0x3f, G: 0x6f, B: 0xff, A: 0xff})
	r.SetMinSize(geo.NewSize(120, 60))
	return widget.NewSimpleRenderer(r)
}

func (p *paletteItem) Dragged(*event.DragEvent) {} // position handled by the ghost
func (p *paletteItem) DragEnd()                 {}

// DragImage returns the ghost painted under the pointer during the drag.
func (p *paletteItem) DragImage() core.CanvasObject {
	g := canvas.NewRectangle(color.NRGBA{R: 0x3f, G: 0x6f, B: 0xff, A: 0xaa})
	g.SetMinSize(geo.NewSize(120, 60))
	g.CornerRadius = 8
	return g
}

var (
	_ event.Draggable        = (*paletteItem)(nil)
	_ core.DragImageProvider = (*paletteItem)(nil)
)

func main() {
	a := app.New()
	w := a.NewWindow("drag ghost probe")
	item := newPaletteItem()
	w.SetContent(container.NewVBox(
		widget.NewLabel("drag the blue box — ghost should follow, cursor = move"),
		container.NewCenter(item),
	))
	w.Resize(geo.NewSize(400, 300))
	bellui.ShowAndRun(w)
}
