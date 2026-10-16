// Command dndprobe live-verifies per-widget drop targets (pre-rebrand tracker #515 Part A).
//
// It shows a window split between a Droppable widget (which logs the files
// dropped onto it and their position) and a plain area covered by the
// window-wide SetOnDropped fallback. Drag files from the desktop file manager
// onto each region and watch stdout.
//
//	go run ./hack/dndprobe
package main

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// dropZone is a Droppable widget that reports what lands on it.
type dropZone struct {
	widget.BaseWidget
	label *widget.Label
}

func newDropZone() *dropZone {
	z := &dropZone{label: widget.NewLabel("Drop files HERE (widget target)")}
	z.ExtendBaseWidget(z)
	return z
}

func (z *dropZone) CreateRenderer() core.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	return widget.NewSimpleRenderer(container.NewStack(bg, z.label))
}

func (z *dropZone) Dropped(ev *bellui.DropEvent) {
	fmt.Printf("WIDGET drop @ %v (abs %v): %d file(s)\n", ev.Position, ev.AbsolutePosition, len(ev.URIs))
	for _, u := range ev.URIs {
		fmt.Println("   ", u)
	}
	z.label.SetText(fmt.Sprintf("widget got %d file(s)", len(ev.URIs)))
}

func main() {
	a := app.New()
	w := a.NewWindow("Drag-and-Drop Probe (#515)")

	zone := newDropZone()
	outside := widget.NewLabel("Drop files here for the window-wide fallback")
	w.SetContent(container.NewGridWithColumns(2, zone, outside))
	w.Resize(geo.NewSize(560, 220))

	if dw, ok := w.(bellui.DroppableWindow); ok {
		dw.SetOnDropped(func(pos geo.Position, items []bellui.URI) {
			fmt.Printf("WINDOW fallback drop @ %v: %d file(s)\n", pos, len(items))
			for _, u := range items {
				fmt.Println("   ", u)
			}
		})
	}

	w.Show()
	a.Run()
}
