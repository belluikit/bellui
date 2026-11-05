package main

import (
	"fmt"
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("framebeginprobe")
	tick := widget.NewLabel("tick: 0")
	w.SetContent(container.NewVBox(widget.NewLabel("framebeginprobe (#907)"), tick))
	lc := a.Lifecycle()
	lc.SetOnStarted(func() { log.Println("framebeginprobe lifecycle=started") })
	lc.SetOnStopped(func() { log.Println("framebeginprobe lifecycle=stopped") })
	lc.SetOnEnteredForeground(func() { log.Println("framebeginprobe lifecycle=foreground") })
	lc.SetOnExitedForeground(func() { log.Println("framebeginprobe lifecycle=background") })
	go func() {
		for i := 1; i < 6000; i++ { // self-stop after ~10 min
			time.Sleep(100 * time.Millisecond)
			n := i
			bellui.Do(func() { tick.SetText(fmt.Sprintf("tick: %d", n)) })
		}
		log.Println("framebeginprobe done")
		bellui.Do(a.Quit)
	}()
	bellui.ShowAndRun(w)
}
