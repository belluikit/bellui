// Command tzprobe live-verifies that time.Local is the device's time zone on
// a real device (#944). Go's time package leaves it at UTC on iOS, so the
// mobile driver sets it at startup from the zone Foundation reports.
//
// Two seconds after launch it prints one line to stdout and shows it in a
// label:
//
//	tzprobe local=Local now=05:12:03 PST jan=04:00 PST jul=05:00 PDT
//
// now must read the hour the status bar shows. jan and jul are 12:00 UTC on
// 15 January and 15 July 2026 in the device zone: in a zone with daylight
// saving they differ by an hour, which a fixed offset cannot do. With -auto
// it exits after printing, so a scripted run leaves nothing running.
//
// iOS: package with `bellui package -os ios` from this directory, install,
// then `xcrun devicectl device process launch --console <device> <app-id>
// -auto` and read the tzprobe line.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func report() string {
	jan := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC).Local()
	jul := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC).Local()
	return fmt.Sprintf("tzprobe local=%s now=%s jan=%s jul=%s",
		time.Local, time.Now().Format("15:04:05 MST"),
		jan.Format("15:04 MST"), jul.Format("15:04 MST"))
}

func main() {
	auto := flag.Bool("auto", false, "exit after printing the report")
	flag.Parse()

	a := app.NewWithID("com.trendvidia.tzprobe")
	w := a.NewWindow("tzprobe")
	label := widget.NewLabel("waiting…")
	w.SetContent(label)

	go func() {
		time.Sleep(2 * time.Second)
		line := report()
		fmt.Println(line)
		bellui.Do(func() { label.SetText(line) })
		if *auto {
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}
	}()
	bellui.ShowAndRun(w)
}
