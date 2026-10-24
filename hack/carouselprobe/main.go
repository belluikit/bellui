// Command carouselprobe live-verifies widget.Carousel's swipe on a touch
// device (#686): the current item follows the finger with the neighbour
// sliding in, the release animates to the snapped item or back, and a fast
// flick pages where a slow drag of the same length does not.
//
// The window holds one three-page carousel. The probe prints machine-greppable
// lines to stdout (logcat tag Bellui on Android):
//
//	carouselprobe ready scale=2.62 carousel=0,0+411x891pt
//	carouselprobe pos t=1234 cur=0 x=[-57 167 off]
//	carouselprobe changed cur=1 t=1300
//
// pos lines are sampled every frame-ish (8ms) and printed only when an item
// moved: x lists each item's position on the track in points, "off" for a
// hidden item. t is milliseconds since launch. A tracking drag shows a run of
// pos lines with two items on the track before any release; a settle shows
// the run continuing after the finger lifts and ending at x=0 for the current
// item.
//
// Drive it with adb (coordinates in pixels; the ready line gives the
// carousel's bounds in points, times scale for pixels):
//
//	adb shell input swipe 800 1000 700 1000 60    # 100px in 60ms: flick, pages
//	adb shell input swipe 800 1000 700 1000 1500  # same length, slow: snaps back
//	adb shell input swipe 900 1000 200 1000 800   # long drag: pages, tracks first
//
// It quits by itself after ten minutes.
//
// Android: package with `bellui package --target android/arm64 --app-id
// com.trendvidia.carouselprobe` from this directory, install, launch, then
// `adb logcat -s Bellui | grep carouselprobe`.
package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

func page(name string, c color.Color) core.CanvasObject {
	bg := canvas.NewRectangle(c)
	t := canvas.NewText(name, color.White)
	t.TextSize = 48
	t.Alignment = bellui.TextAlignCenter
	return container.NewStack(bg, container.NewCenter(t))
}

func main() {
	start := time.Now()
	a := app.New()
	w := a.NewWindow("carouselprobe")

	c := widget.NewCarousel(
		page("one", color.NRGBA{R: 0xb3, G: 0x26, B: 0x1e, A: 0xff}),
		page("two", color.NRGBA{R: 0x1e, G: 0x6b, B: 0xb3, A: 0xff}),
		page("three", color.NRGBA{R: 0x2e, G: 0x7d, B: 0x32, A: 0xff}),
	)
	ms := func() int64 { return time.Since(start).Milliseconds() }
	c.OnChanged = func(i int) { fmt.Printf("carouselprobe changed cur=%d t=%d\n", i, ms()) }
	w.SetContent(c)

	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(func() {
			p := c.Position()
			s := c.Size()
			fmt.Printf("carouselprobe ready scale=%.3f carousel=%.0f,%.0f+%.0fx%.0fpt\n",
				w.Canvas().Scale(), p.X, p.Y, s.Width, s.Height)
		})
		last := ""
		tick := time.NewTicker(8 * time.Millisecond)
		defer tick.Stop()
		for range tick.C {
			bellui.Do(func() {
				xs := make([]string, len(c.Items))
				for i, it := range c.Items {
					if it.Visible() {
						xs[i] = fmt.Sprintf("%.0f", it.Position().X)
					} else {
						xs[i] = "off"
					}
				}
				line := fmt.Sprintf("cur=%d x=[%s]", c.CurrentItem, strings.Join(xs, " "))
				if line != last {
					last = line
					fmt.Printf("carouselprobe pos t=%d %s\n", ms(), line)
				}
			})
		}
	}()
	go func() {
		time.Sleep(10 * time.Minute)
		bellui.Do(a.Quit)
	}()

	bellui.ShowAndRun(w)
}
