// rotprobe: a label that changes every 100 ms, logging a tick each second (#909 control).
package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.rotprobe")
	w := a.NewWindow("rotprobe")
	l := widget.NewLabel("0")
	w.SetContent(l)
	go func() { log.Println(http.ListenAndServe("127.0.0.1:6060", nil)) }()
	n := 0
	go func() {
		for range time.Tick(100 * time.Millisecond) {
			n++
			v := n
			bellui.Do(func() { l.SetText(fmt.Sprint(v)) })
			if v%10 == 0 {
				log.Printf("rotprobe tick=%d", v)
			}
		}
	}()
	bellui.ShowAndRun(w)
}
