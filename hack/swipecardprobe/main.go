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
// see how a card made of several objects tilts (#997). With -auto it also paints
// the window with the software painter, the reference for where each object
// goes, and prints how far the label's glyphs are from where it draws them:
//
//	go run ./hack/swipecardprobe -auto -composite -dx 150
//	go run -tags gl ./hack/swipecardprobe -auto -composite -dx 150
package main

import (
	"flag"
	"fmt"
	"image"
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
	"github.com/belluikit/bellui/internal/painter/software"
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
				if *composite {
					ref := software.NewPainter().Paint(w.Canvas())
					// Below the status bar, where only the card draws.
					top := int((status.MinSize().Height + 4) * w.Canvas().Scale())
					gx, gy, gok := glyphCentre(img, top)
					rx, ry, rok := glyphCentre(ref, top)
					px := float32(img.Bounds().Dx()) / w.Canvas().Size().Width
					rpx := float32(ref.Bounds().Dx()) / w.Canvas().Size().Width
					if gok && rok {
						fmt.Printf("label glyphs: painter (%.1f, %.1f) pt, software (%.1f, %.1f) pt, off by (%.1f, %.1f) pt\n",
							gx/px, gy/px, rx/rpx, ry/rpx, gx/px-rx/rpx, gy/px-ry/rpx)
					} else {
						fmt.Printf("label glyphs not found: painter %v, software %v\n", gok, rok)
					}
				}
			})
			bellui.DoAndWait(a.Quit)
		}()
	}
	bellui.ShowAndRun(w)
}

// glyphCentre is the centre of the bounding box of the label's glyph pixels
// below row top: grey pixels (the theme's text colour, or its blend with the
// background) far from the background, which it reads at the bottom-left
// corner. The red block and its anti-aliased edges are not grey.
func glyphCentre(img image.Image, top int) (x, y float32, ok bool) {
	b := img.Bounds()
	br, bg, bb, _ := img.At(b.Min.X, b.Max.Y-1).RGBA()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for py := b.Min.Y + top; py < b.Max.Y; py++ {
		for px := b.Min.X; px < b.Max.X; px++ {
			r, g, bl, _ := img.At(px, py).RGBA()
			r, g, bl = r>>8, g>>8, bl>>8
			if diff(r, g) > 30 || diff(g, bl) > 30 {
				continue
			}
			if diff(r, br>>8)+diff(g, bg>>8)+diff(bl, bb>>8) < 150 {
				continue
			}
			ok = true
			minX, maxX = min(minX, px), max(maxX, px)
			minY, maxY = min(minY, py), max(maxY, py)
		}
	}
	return float32(minX+maxX+1)/2 - float32(b.Min.X), float32(minY+maxY+1)/2 - float32(b.Min.Y), ok
}

func diff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
