// Command insetsprobe live-verifies that the page follows the keyboard on
// Android (#894): it shrinks when a keyboard comes up and grows back when it
// leaves, including a keyboard that another app left up at launch.
//
// The page is one solid colour, so a screenshot shows where it ends. Each
// size the page is given is logged to logcat (tag Bellui) as
//
//	insetsprobe size=<w>x<h>
//
// Launch it from the launcher's search with the keyboard up, then wait for
// that keyboard to leave: the last size logged must match a launch without
// one. Tap the Entry to raise this app's keyboard and close it with Back: the
// size must shrink and then come back.
package main

import (
	"image/color"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// sizeLog is a stack layout that logs each size it is given.
type sizeLog struct{}

func (sizeLog) Layout(objects []core.CanvasObject, size geo.Size) {
	log.Printf("insetsprobe size=%vx%v", size.Width, size.Height)
	for _, o := range objects {
		o.Move(geo.NewPos(0, 0))
		o.Resize(size)
	}
}

func (sizeLog) MinSize(objects []core.CanvasObject) geo.Size {
	var min geo.Size
	for _, o := range objects {
		min = min.Max(o.MinSize())
	}
	return min
}

func main() {
	a := app.NewWithID("io.bellui.insetsprobe")
	w := a.NewWindow("insetsprobe")

	e := widget.NewEntry()
	e.SetPlaceHolder("Tap to raise the keyboard")
	page := canvas.NewRectangle(color.NRGBA{R: 0x20, G: 0xa0, B: 0x60, A: 0xff})

	w.SetContent(container.New(sizeLog{}, page, container.NewVBox(e)))
	log.Print("insetsprobe: ready")
	bellui.ShowAndRun(w)
}
