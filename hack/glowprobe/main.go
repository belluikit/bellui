// Command glowprobe measures what an animated RadialGradient costs on a device
// (#1009). It draws Daysong's cover the way Player builds it: a 300 dp square
// holding a full-size LinearGradient, two centred RadialGradients whose size
// and start colour breathe, and a glyph over them. A ticker updates the two
// glows at a chosen rate, as Player's animation cap does.
//
// Buttons above the cover pick the rate (0, 2, 10, 30 or 60 updates a second)
// and "glow only" hides the LinearGradient and the glyph, which separates the
// glows' own cost from that of what lies under them. "tiny" makes each update
// change a 4 dp square under the buttons instead of the glows: the same frames
// with almost nothing to draw, which separates a frame's fixed cost from the
// glows'. Each change prints a
// line, and once ready the probe prints where each button is, in pixels:
//
//	glowprobe ready scale=2.225 button rate0=212,480 rate2=... glowonly=...
//	glowprobe rate=30 glowonly=false
//	glowprobe updates=1800 in 60.0s
//
// Drive it with adb: `adb shell input tap <x> <y>` with a button's pixels.
// It quits by itself after thirty minutes.
//
// Android: package with `bellui package --target android/arm64 --app-id
// com.trendvidia.glowprobe` from this directory.
package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// centred lays out each object centred in the container at its own size,
// which the probe sets: a glow is not stretched to the cover.
type centred struct {
	sizes map[core.CanvasObject]geo.Size
}

func (c *centred) Layout(objs []core.CanvasObject, size geo.Size) {
	for _, o := range objs {
		s, ok := c.sizes[o]
		if !ok {
			s = size
		}
		o.Resize(s)
		o.Move(geo.NewPos((size.Width-s.Width)/2, (size.Height-s.Height)/2))
	}
}

func (c *centred) MinSize([]core.CanvasObject) geo.Size { return geo.NewSize(300, 300) }

func main() {
	a := app.New()
	w := a.NewWindow("glowprobe")

	under := canvas.NewLinearGradient(color.NRGBA{0x0E, 0x5E, 0x63, 0xFF}, color.NRGBA{0x3A, 0x2A, 0x6B, 0xFF}, 45)
	outer := canvas.NewRadialGradient(color.NRGBA{0xFF, 0xFF, 0xFF, 0x90}, color.Transparent)
	inner := canvas.NewRadialGradient(color.NRGBA{0xFF, 0xFF, 0xFF, 0x70}, color.Transparent)
	glyph := widget.NewIcon(theme.MediaMusicIcon())
	lay := &centred{sizes: map[core.CanvasObject]geo.Size{
		outer: geo.NewSize(250, 250),
		inner: geo.NewSize(220, 220),
		glyph: geo.NewSize(64, 64),
	}}
	cover := container.New(lay, under, outer, inner, glyph)

	// tiny, when on, makes each update change a 4 dp square instead of the
	// glows: the same frames at the same rate, with almost nothing to draw.
	dot := canvas.NewRectangle(color.White)
	dot.SetMinSize(geo.NewSize(4, 4))
	rate, glowOnly, tiny, updates := 0, false, false, 0
	var ticker *time.Ticker
	stop := make(chan struct{})
	start := time.Now()
	report := func() { fmt.Printf("glowprobe rate=%d glowonly=%v tiny=%v\n", rate, glowOnly, tiny) }

	// step moves both glows to where they are at this moment, so the
	// breathing runs at the same speed whatever the rate.
	step := func() {
		t := time.Since(start).Seconds()
		updates++
		if tiny {
			dot.FillColor = color.NRGBA{0xFF, uint8(t * 50), 0xFF, 0xFF}
			dot.Refresh()
			return
		}
		s := float32(math.Sin(2 * math.Pi * t / 4))
		lay.sizes[outer] = geo.NewSize(220+30*s, 220+30*s)
		lay.sizes[inner] = geo.NewSize(200-20*s, 200-20*s)
		outer.StartColor = color.NRGBA{0xFF, 0xFF, 0xFF, uint8(0x90 + 0x30*s)}
		cover.Layout.Layout(cover.Objects, cover.Size())
		outer.Refresh()
		inner.Refresh()
	}
	setRate := func(r int) {
		if ticker != nil {
			ticker.Stop()
			close(stop)
			ticker = nil
		}
		rate = r
		report()
		if r == 0 {
			return
		}
		ticker = time.NewTicker(time.Second / time.Duration(r))
		stop = make(chan struct{})
		go func(tk *time.Ticker, done chan struct{}) {
			for {
				select {
				case <-tk.C:
					bellui.Do(step)
				case <-done:
					return
				}
			}
		}(ticker, stop)
	}

	var buttons []*widget.Button
	var names []string
	for _, r := range []int{0, 2, 10, 30, 60} {
		buttons = append(buttons, widget.NewButton(fmt.Sprint(r), func() { setRate(r) }))
		names = append(names, fmt.Sprintf("rate%d", r))
	}
	only := widget.NewButton("glow only", func() {
		glowOnly = !glowOnly
		if glowOnly {
			under.Hide()
			glyph.Hide()
		} else {
			under.Show()
			glyph.Show()
		}
		report()
	})
	buttons = append(buttons, only)
	names = append(names, "glowonly")
	buttons = append(buttons, widget.NewButton("tiny", func() {
		tiny = !tiny
		report()
	}))
	names = append(names, "tiny")
	row := container.NewHBox()
	for _, b := range buttons {
		row.Add(b)
	}

	bg := canvas.NewRectangle(color.NRGBA{0x12, 0x10, 0x1C, 0xFF})
	w.SetContent(container.NewStack(bg, container.NewVBox(
		container.NewCenter(row),
		container.NewCenter(dot),
		layout.NewSpacer(),
		container.NewCenter(cover),
		layout.NewSpacer(),
	)))

	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(func() {
			scale := w.Canvas().Scale()
			// On mobile the content starts below the status bar, and
			// object positions count from the content.
			off := w.Content().Position()
			var b strings.Builder
			for i, btn := range buttons {
				p := bellui.CurrentApp().Driver().AbsolutePositionForObject(btn)
				s := btn.Size()
				fmt.Fprintf(&b, " %s=%.0f,%.0f", names[i], (off.X+p.X+s.Width/2)*scale, (off.Y+p.Y+s.Height/2)*scale)
			}
			fmt.Printf("glowprobe ready scale=%.3f button%s\n", scale, b.String())
		})
	}()
	go func() {
		for range time.Tick(time.Minute) {
			bellui.Do(func() {
				fmt.Printf("glowprobe updates=%d in 60.0s rate=%d\n", updates, rate)
				updates = 0
			})
		}
	}()
	// App.Quit does nothing on a phone, whose guidelines forbid an app
	// quitting itself, so the self-stop exits the process (#1045).
	go func() {
		time.Sleep(30 * time.Minute)
		fmt.Println("glowprobe done")
		os.Exit(0)
	}()

	bellui.ShowAndRun(w)
}
