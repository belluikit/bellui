// Command wheelprobe live-verifies widget.WheelPicker on a touch device
// (#977): a drag spins the wheel and it settles with an item centred, a flick
// coasts, and items away from the centre shrink and fade through
// core.OpacityGroup on the device's real painter.
//
// The window holds one vertical wheel of seven white squares, five visible, on
// a flat black background, so a screenshot gives each item's opacity directly:
// a square's pixel value over black is 255 times its opacity. With FadeTo 0.3
// they read 1 at the centre, 0.72 one slot out and 0.44 two out.
//
// It prints machine-greppable lines to stdout (logcat tag Bellui on Android):
//
//	wheelprobe ready scale=2.62 wheel=155,40+100x400pt selected=3
//	wheelprobe changed selected=5 t=4321
//
// Drive it with adb (pixels: the ready line's points times scale). With a
// pitch of 80pt:
//
//	adb shell input swipe 540 1500 540 1080 1500  # 2 pitches up, slow: +2
//	adb shell input swipe 540 1300 540 1100 60    # a flick up: coasts further
//
// It quits by itself after ten minutes.
//
// Android: package with `bellui package --target android/arm64 --app-id
// com.trendvidia.wheelprobe` from this directory.
package main

import (
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/widget"
)

func main() {
	start := time.Now()
	a := app.New()
	w := a.NewWindow("wheelprobe")

	items := make([]core.CanvasObject, 7)
	for i := range items {
		r := canvas.NewRectangle(color.White)
		r.SetMinSize(geo.NewSize(60, 60))
		items[i] = r
	}
	ms := func() int64 { return time.Since(start).Milliseconds() }
	wheel := widget.NewWheelPicker(items, func(i int) { fmt.Printf("wheelprobe changed selected=%d t=%d\n", i, ms()) })
	wheel.SetSelectedSilently(3)

	box := container.New(layout.NewGridWrapLayout(geo.NewSize(100, 400)), wheel)
	bg := canvas.NewRectangle(color.Black)
	w.SetContent(container.NewStack(bg, container.NewVBox(layout.NewSpacer(), container.NewCenter(box), layout.NewSpacer())))

	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(func() {
			p := bellui.CurrentApp().Driver().AbsolutePositionForObject(wheel)
			s := wheel.Size()
			fmt.Printf("wheelprobe ready scale=%.3f wheel=%.0f,%.0f+%.0fx%.0fpt selected=%d\n",
				w.Canvas().Scale(), p.X, p.Y, s.Width, s.Height, wheel.Selected)
		})
	}()
	// App.Quit does nothing on a phone, whose guidelines forbid an app
	// quitting itself, so the self-stop exits the process (#1045).
	go func() {
		time.Sleep(10 * time.Minute)
		fmt.Println("wheelprobe done")
		os.Exit(0)
	}()

	bellui.ShowAndRun(w)
}
