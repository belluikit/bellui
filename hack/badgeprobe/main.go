package main

import (
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// #706 probe: shows a window and sets a taskbar badge. Run with
// BELLUI_TASKBAR_DEBUG=1 to log the SetOverlayIcon HRESULT.
func main() {
	a := app.NewWithID("com.trendvidia.badgeprobe")
	w := a.NewWindow("badge probe")
	w.SetContent(widget.NewLabel("taskbar badge = 5"))
	w.Resize(geo.NewSize(320, 200))

	go func() {
		time.Sleep(2 * time.Second)
		ba, ok := a.(bellui.BadgeApp)
		if !ok {
			log.Println("app is not a BadgeApp")
			return
		}
		ba.SetBadge(5)
		log.Println("SetBadge(5) called")
		time.Sleep(4 * time.Second)
		ba.SetBadge(0) // clear
		log.Println("SetBadge(0) called")
	}()

	bellui.ShowAndRun(w)
}
