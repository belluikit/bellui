package main

import (
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// pre-rebrand tracker #721 probe: drives desktop.ProgressWindow.SetProgress so the Unity
// LauncherEntry progress D-Bus signal can be observed on the session bus.
func main() {
	a := app.NewWithID("com.trendvidia.progressprobe")
	w := a.NewWindow("progress probe")
	w.SetContent(widget.NewLabel("progress probe"))
	w.Resize(geo.NewSize(300, 200))

	go func() {
		time.Sleep(2 * time.Second)
		pw, ok := w.(desktop.ProgressWindow)
		if !ok {
			log.Println("window is not a ProgressWindow")
			return
		}
		for _, v := range []float64{0.25, 0.5, 0.75, 1.0} {
			val := v
			bellui.Do(func() { pw.SetProgress(val, false) })
			log.Printf("SetProgress(%.2f)", val)
			time.Sleep(600 * time.Millisecond)
		}
		bellui.Do(func() { pw.SetProgress(-1, false) }) // clear
		log.Println("SetProgress(clear)")
	}()

	bellui.ShowAndRun(w)
}
