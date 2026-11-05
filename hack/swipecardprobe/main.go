// Command swipecardprobe shows a SwipeArea in card mode (#989) on a real
// painter: drag the card left or right, or up or down, and release.
//
// -auto drags the card by -dx, -dy without a finger, saves a capture to -shot
// and exits, to check the painters draw the card where the software painter
// does (the centre moved by the drag, tilted clockwise when dragged right):
//
//	go run ./hack/swipecardprobe -auto -dx 150 -shot /tmp/webgpu.png
//	go run -tags gl ./hack/swipecardprobe -auto -dx 150 -shot /tmp/gl.png
//
// -composite puts a label below a coloured block instead of one rectangle, to
// see how a card made of several objects tilts (#997).
package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	auto := flag.Bool("auto", false, "drag by -dx, -dy, capture to -shot and exit")
	dx := flag.Float64("dx", 150, "drag across, in points, for -auto")
	dy := flag.Float64("dy", 0, "drag down, in points, for -auto")
	shot := flag.String("shot", "", "where -auto saves its capture")
	composite := flag.Bool("composite", false, "a card of a block and a label")
	flag.Parse()

	a := app.New()
	w := a.NewWindow("swipecardprobe")
	status := widget.NewLabel("swipe the card")

	var content core.CanvasObject = canvas.NewRectangle(color.NRGBA{R: 0xff, A: 0xff})
	if *composite {
		content = container.NewBorder(nil, widget.NewLabel("a label at the bottom"), nil, nil,
			canvas.NewRectangle(color.NRGBA{R: 0xff, A: 0xff}))
	}
	n := 0
	area := widget.NewSwipeArea(content, func(d widget.SwipeDirection) {
		n++
		status.SetText(fmt.Sprintf("swiped %s (%d)", d, n))
	})
	area.Follow = true
	area.OnSwipeProgress = func(d widget.SwipeDirection, p float32) {
		if p > 0 {
			status.SetText(fmt.Sprintf("%s %.0f%%", d, p*100))
		}
	}
	// The card sits in the middle third of the window, with room to tilt.
	card := container.NewGridWrap(geo.NewSize(300, 300), area)
	w.SetContent(container.NewBorder(status, nil, nil, nil, container.NewCenter(card)))
	w.SetPadded(false)
	w.Resize(geo.NewSize(900, 500))

	if *auto {
		go func() {
			time.Sleep(time.Second)
			bellui.DoAndWait(func() {
				area.Dragged(&event.DragEvent{Dragged: geo.Delta{DX: float32(*dx), DY: float32(*dy)}})
			})
			time.Sleep(500 * time.Millisecond)
			bellui.DoAndWait(func() {
				img := w.Canvas().Capture()
				if *shot != "" {
					f, err := os.Create(*shot)
					if err == nil {
						err = png.Encode(f, img)
						f.Close()
					}
					if err != nil {
						fmt.Println("shot:", err)
					}
				}
				p := area.Position()
				fmt.Printf("area at %v size %v, capture %v, scale %v\n", p, area.Size(), img.Bounds(), w.Canvas().Scale())
			})
			bellui.DoAndWait(a.Quit)
		}()
	}
	bellui.ShowAndRun(w)
}
