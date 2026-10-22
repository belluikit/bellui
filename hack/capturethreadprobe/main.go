// Command capturethreadprobe stresses Canvas.Capture from goroutines on a
// mobile device or simulator (#573). Each step recolours the screen, then a
// goroutine captures and checks the centre pixel shows the new colour: a
// capture that read the last frame drawn, not a fresh one, shows the old
// colour. A second goroutine captures without pause while an animation keeps
// the paint loop busy, so a read that raced the paint would crash or tear.
//
// Each stale or nil capture logs a line, and the run exits after
//
//	CAPTURE_DONE steps=N fresh=F stale=S nil=Z background=B
package main

import (
	"image/color"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
)

const steps = 200

var palette = []color.NRGBA{
	{R: 0xe0, G: 0x20, B: 0x20, A: 0xff},
	{R: 0x20, G: 0xc0, B: 0x20, A: 0xff},
	{R: 0x20, G: 0x40, B: 0xe0, A: 0xff},
	{R: 0xe0, G: 0xc0, B: 0x20, A: 0xff},
}

func main() {
	a := app.NewWithID("com.trendvidia.capturethreadprobe")
	w := a.NewWindow("Capture thread probe")
	w.SetPadded(false)

	bg := canvas.NewRectangle(palette[0])
	spinner := canvas.NewRectangle(color.White)
	spinner.SetMinSize(geo.NewSquareSize(40))
	w.SetContent(container.NewStack(bg, container.NewWithoutLayout(spinner)))

	anim := canvas.NewPositionAnimation(geo.NewPos(0, 0), geo.NewPos(200, 0), time.Second, func(p geo.Position) {
		spinner.Move(p)
		spinner.Resize(geo.NewSquareSize(40))
	})
	anim.AutoReverse = true
	anim.RepeatCount = bellui.AnimationRepeatForever

	var done atomic.Bool
	var background atomic.Int32
	go func() {
		time.Sleep(2 * time.Second) // let the first frames reach the screen
		bellui.Do(anim.Start)
		go func() {
			for !done.Load() {
				if w.Canvas().Capture() != nil {
					background.Add(1)
				}
			}
		}()

		fresh, stale, none := 0, 0, 0
		for i := 1; i <= steps; i++ {
			want := palette[i%len(palette)]
			bellui.DoAndWait(func() {
				bg.FillColor = want
				bg.Refresh()
			})
			img := w.Canvas().Capture()
			if img == nil {
				none++
				log.Printf("CAPTURE step=%d nil", i)
				continue
			}
			b := img.Bounds()
			// The spinner runs along the top; sample below it.
			got := color.NRGBAModel.Convert(img.At(b.Dx()/2, b.Dy()*2/3)).(color.NRGBA)
			if near(got, want) {
				fresh++
			} else {
				stale++
				log.Printf("CAPTURE step=%d stale want=%v got=%v size=%v", i, want, got, b.Size())
			}
		}
		done.Store(true)
		log.Printf("CAPTURE_DONE steps=%d fresh=%d stale=%d nil=%d background=%d",
			steps, fresh, stale, none, background.Load())
		time.Sleep(time.Second) // let the line reach the console
		os.Exit(0)
	}()

	bellui.ShowAndRun(w)
}

func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) < 12 && d(a.G, b.G) < 12 && d(a.B, b.B) < 12
}
