// Command taskbarprobe is a live-verify harness for desktop.ProgressWindow
// (#517 taskbar progress). It shows a window and drives its taskbar progress
// indicator so the green fill (Windows), or a documented no-op elsewhere, can
// be observed. Windows is the flagship platform.
//
//	taskbarprobe                 hold 70% for a while (screenshot-friendly)
//	taskbarprobe -value 0.4      hold a specific fraction
//	taskbarprobe -indeterminate  show the marquee/busy animation
//	taskbarprobe -value -1       clear the indicator
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	value := flag.Float64("value", 0.7, "progress fraction 0..1 to display (negative clears)")
	indet := flag.Bool("indeterminate", false, "show indeterminate (marquee) progress")
	hold := flag.Duration("hold", 30*time.Second, "how long to keep the window up")
	logfile := flag.String("logfile", "", "also append log output to this file (for headless/scheduled runs)")
	flag.Parse()

	if *logfile != "" {
		if f, err := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(io.MultiWriter(os.Stderr, f))
			defer f.Close()
		}
	}

	a := app.NewWithID("com.trendvidia.taskbarprobe")
	w := a.NewWindow("Taskbar Progress Probe")

	pw, ok := w.(desktop.ProgressWindow)
	if !ok {
		log.Fatal("window does not implement desktop.ProgressWindow")
	}

	label := widget.NewLabel("starting…")
	w.SetContent(label)

	go func() {
		time.Sleep(1500 * time.Millisecond) // let the window map + get a taskbar button
		var desc string
		switch {
		case *indet:
			pw.SetProgress(0, true)
			desc = "indeterminate (marquee)"
		case *value < 0:
			pw.SetProgress(-1, false)
			desc = "cleared"
		default:
			pw.SetProgress(*value, false)
			desc = fmt.Sprintf("%.0f%%", *value*100)
		}
		log.Printf("SetProgress -> %s", desc)
		bellui.DoAndWait(func() {
			label.SetText("Taskbar progress: " + desc)
			diagWindow(w)
		})

		time.Sleep(*hold)
		log.Println("done, quitting")
		bellui.Do(a.Quit)
	}()

	w.Resize(geo.NewSize(420, 120))
	bellui.ShowAndRun(w)
}
