// Command menuprobe opens a window with a main menu and quits after a while,
// so the macOS application menu can be read from outside with System Events
// (#934: the app menu without the undeclared setAppleMenu: call).
//
// Usage:
//
//	go build -o /tmp/menuprobe ./hack/menuprobe && /tmp/menuprobe -for 20s &
//	osascript -e 'tell application "System Events" to tell process "menuprobe" to get name of every menu item of menu 1 of menu bar item 1 of menu bar 1'
package main

import (
	"flag"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	stay := flag.Duration("for", 20*time.Second, "quit after this long")
	flag.Parse()

	a := app.New()
	w := a.NewWindow("menuprobe")
	w.SetMainMenu(bellui.NewMainMenu(bellui.NewMenu("File", bellui.NewMenuItem("Probe", func() {}))))
	w.SetContent(widget.NewLabel("menuprobe"))
	go func() {
		time.Sleep(*stay)
		bellui.Do(a.Quit)
	}()
	bellui.ShowAndRun(w)
}
