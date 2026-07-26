// Command dropprobe is a live-verify harness for #708 (drag-motion hover
// feedback for drop targets). It shows a drop zone implementing
// bellui.DropHoverable: drag a file from the desktop file manager over the window
// and the zone should highlight on DragEnter, track the pointer on DragOver, and
// un-highlight on DragLeave — before you even release. Dropping lists the files.
//
// Drag-motion is delivered on X11 and macOS (glfw #708); on Windows the zone
// still receives the drop but no hover phases (WM_DROPFILES has no motion).
//
//	go run ./hack/dropprobe
package main

import (
	"fmt"
	"image/color"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// dropZone is a bellui.DropHoverable drop target that highlights while a drag
// hovers over it and shows what was dropped.
type dropZone struct {
	widget.BaseWidget
	bg    *canvas.Rectangle
	label *widget.Label
	hover bool
}

func newDropZone() *dropZone {
	z := &dropZone{
		bg:    canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground)),
		label: widget.NewLabel("Drag files here"),
	}
	z.bg.CornerRadius = theme.Padding()
	z.label.Alignment = bellui.TextAlignCenter
	z.ExtendBaseWidget(z)
	return z
}

func (z *dropZone) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(z.bg, container.NewCenter(z.label)))
}

func (z *dropZone) refresh() {
	if z.hover {
		z.bg.FillColor = theme.Color(theme.ColorNamePrimary)
		z.bg.StrokeColor = theme.Color(theme.ColorNameFocus)
		z.bg.StrokeWidth = 3
	} else {
		z.bg.FillColor = theme.Color(theme.ColorNameInputBackground)
		z.bg.StrokeWidth = 0
	}
	z.bg.Refresh()
}

func (z *dropZone) DragEnter(ev *bellui.DragHoverEvent) {
	z.hover = true
	z.label.SetText(fmt.Sprintf("DragEnter @ %.0f,%.0f", ev.Position.X, ev.Position.Y))
	z.refresh()
	log.Printf("DragEnter abs=%v rel=%v", ev.AbsolutePosition, ev.Position)
}

func (z *dropZone) DragOver(ev *bellui.DragHoverEvent) {
	z.label.SetText(fmt.Sprintf("DragOver @ %.0f,%.0f", ev.Position.X, ev.Position.Y))
}

func (z *dropZone) DragLeave() {
	z.hover = false
	z.label.SetText("Drag files here")
	z.refresh()
	log.Print("DragLeave")
}

func (z *dropZone) Dropped(ev *bellui.DropEvent) {
	names := make([]string, len(ev.URIs))
	for i, u := range ev.URIs {
		names[i] = u.Name()
	}
	z.label.SetText(fmt.Sprintf("Dropped %d: %v", len(ev.URIs), names))
	log.Printf("Dropped %d items at %v: %v", len(ev.URIs), ev.Position, ev.URIs)
}

func main() {
	a := app.New()
	w := a.NewWindow("dropprobe (#708)")

	zone := newDropZone()
	hint := canvas.NewText("Hover a dragged file over the zone — it highlights BEFORE the drop (X11/macOS).", color.Gray{Y: 0x88})
	hint.TextSize = 11
	w.SetContent(container.NewBorder(nil, hint, nil, nil, zone))
	w.Resize(geo.NewSize(420, 300))
	bellui.ShowAndRun(w)
}
