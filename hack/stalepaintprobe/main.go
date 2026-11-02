// Command stalepaintprobe checks #864: a change made just after a window's
// content is built, before or around its first paint, must reach the screen.
//
// It builds a window with a red rectangle and a short label, and from a
// goroutine started at build turns the rectangle green and lengthens the
// label. 2.5 s later it reads back the last frame the window presented, armed
// before the first paint so nothing forces a repaint, and prints
//
//	CHANGE at=... rect-canvas=... label-canvas=... frames=N
//	RESULT ... rect=green(ok)|red(STALE) label-ink-width=W frames=N
//
// label-ink-width is how far right the label's text reaches: 154 for the long
// text on macOS at 1x, 85 for the short one. Run with no change at all
// (DELAY_MS=100000) as the control: it must print red(STALE) and the short
// width, which shows the read-back is the presented frame.
//
// Environment: DELAY_MS before the change; HOP=runonmain to hop with
// bellui.RunOnMain instead of bellui.Do; MODE=swap to build into a window
// already shown; SHAPE=stack for Bellui Player's AudioPlayer shape (a widget
// over a Stack whose child is replaced, then both refreshed).
//
// HIDE=1 (macOS) is the case that lost the change (#864): the app is hidden
// 300 ms after the start, so the window can't paint, and brought back to the
// front at 3 s. Give the change a DELAY_MS between the two, such as 1500. The
// read-back then waits until 5.5 s. It needs System Events (Accessibility)
// and does not raise the window if another app takes the front meanwhile, so
// the probe then forces a repaint after the read-back and prints whether a
// frame began: repaint-began=false means the window was still covered and the
// run says nothing.
//
//	go run ./hack/stalepaintprobe
//	HIDE=1 DELAY_MS=1500 go run ./hack/stalepaintprobe
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/widget"
)

var (
	delay, _ = strconv.Atoi(os.Getenv("DELAY_MS"))
	mode     = os.Getenv("MODE") // "show" builds before Show; "swap" replaces the content of a shown window
	hide     = os.Getenv("HIDE") == "1"
	red      = color.NRGBA{255, 0, 0, 255}
	green    = color.NRGBA{0, 255, 0, 255}
)

func main() {
	a := app.NewWithID("com.trendvidia.stalepaintprobe")
	w := a.NewWindow("stalepaintprobe")
	w.Resize(geo.NewSize(300, 200))
	start := time.Now()
	var rect *canvas.Rectangle
	var label *widget.Label
	build := func() {
		rect = canvas.NewRectangle(red)
		label = widget.NewLabel("AAAAAAAA")
		var top core.CanvasObject = label
		var swap func()
		if os.Getenv("SHAPE") == "stack" {
			// AudioPlayer's shape: a widget over a Stack whose child is
			// replaced, then the Stack and the widget refreshed.
			status := widget.NewLabel("No audio loaded")
			stack := container.NewStack(status)
			holder := newHolder(stack)
			top = holder
			swap = func() {
				stack.Objects = []core.CanvasObject{label}
				stack.Refresh()
				holder.Refresh()
			}
		}
		w.SetContent(container.NewBorder(top, nil, nil, nil, rect))
		go func() {
			if delay > 0 {
				time.Sleep(time.Duration(delay) * time.Millisecond)
			}
			hop := bellui.Do
			if os.Getenv("HOP") == "runonmain" {
				hop = bellui.RunOnMain
			}
			hop(func() {
				if swap != nil {
					swap()
				}
				fmt.Printf("CHANGE at=%v rect-canvas=%v label-canvas=%v frames=%d\n", time.Since(start).Round(time.Millisecond),
					a.Driver().CanvasForObject(rect) != nil, a.Driver().CanvasForObject(label) != nil, frames(w.Canvas()))
				rect.FillColor = green
				rect.Refresh()
				label.SetText("BBBBBBBBBBBBBBBB")
			})
		}()
	}
	if mode == "swap" {
		w.SetContent(widget.NewLabel("placeholder"))
		go func() {
			time.Sleep(500 * time.Millisecond)
			bellui.Do(build)
		}()
	} else {
		build()
	}
	go func() {
		for i := 0; i < 2000; i++ {
			done := false
			bellui.DoAndWait(func() {
				if p, ok := painterOf(w.Canvas()).(interface{ EnableCapture() }); ok {
					p.EnableCapture()
					done = true
				}
			})
			if done {
				break
			}
			time.Sleep(time.Millisecond)
		}
		readAfter := 2500 * time.Millisecond
		if hide {
			readAfter = 5500 * time.Millisecond
			go setHidden(start, 300*time.Millisecond, true)
			go setHidden(start, 3*time.Second, false)
		}
		time.Sleep(time.Until(start.Add(readAfter)))
		bellui.DoAndWait(func() {
			img := painterOf(w.Canvas()).Capture(w.Canvas())
			b := img.Bounds()
			r, g, _, _ := img.At(b.Dx()/2, b.Dy()*3/4).RGBA()
			shown := "other"
			switch {
			case r>>8 > 200 && g>>8 < 50:
				shown = "red(STALE)"
			case g>>8 > 200 && r>>8 < 50:
				shown = "green(ok)"
			}
			fmt.Printf("RESULT mode=%s hide=%v delay=%d rect=%s label-ink-width=%d frames=%d\n", mode, hide, delay, shown, inkWidth(img, b.Dy()/10), frames(w.Canvas()))
		})
		var before uint64
		bellui.DoAndWait(func() {
			before = frames(w.Canvas())
			w.Canvas().Refresh(w.Content())
		})
		time.Sleep(500 * time.Millisecond)
		bellui.DoAndWait(func() { fmt.Printf("CHECK repaint-began=%v\n", frames(w.Canvas()) != before) })
		a.Quit()
	}()
	bellui.ShowAndRun(w)
}

// setHidden hides or shows this process' app through System Events at at
// after start, and brings it to the front when showing it. Hidden, its window
// is occluded and cannot paint.
func setHidden(start time.Time, at time.Duration, hidden bool) {
	time.Sleep(time.Until(start.Add(at)))
	name := filepath.Base(os.Args[0])
	script := fmt.Sprintf(`tell application "System Events" to set visible of process %q to %v`, name, !hidden)
	if !hidden {
		script += fmt.Sprintf("\n"+`tell application "System Events" to set frontmost of process %q to true`, name)
	}
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	fmt.Printf("HIDE hidden=%v at=%v err=%v %s\n", hidden, time.Since(start).Round(time.Millisecond), err, out)
}

// inkWidth is how far right the label's text reaches on row y: longer text
// reaches further.
func inkWidth(img image.Image, y int) int {
	right := 0
	bg := img.At(2, y)
	for x := 0; x < img.Bounds().Dx(); x++ {
		if img.At(x, y) != bg {
			right = x
		}
	}
	return right
}

func painterOf(c core.Canvas) gpu.Painter {
	return c.(interface{ Painter() gpu.Painter }).Painter()
}

func frames(c core.Canvas) uint64 {
	if f, ok := painterOf(c).(interface{ FramesBegun() uint64 }); ok {
		return f.FramesBegun()
	}
	return 0
}

// holder is a widget over one object, as AudioPlayer is.
type holder struct {
	widget.BaseWidget
	content core.CanvasObject
}

func newHolder(c core.CanvasObject) *holder {
	h := &holder{content: c}
	h.ExtendBaseWidget(h)
	return h
}

func (h *holder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(h.content)
}
