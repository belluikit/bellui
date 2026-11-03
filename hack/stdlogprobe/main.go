// Command stdlogprobe live-checks that what an iOS app writes to stdout and
// stderr reaches the device log when the app is started without Xcode, as it
// is from the home screen (#914). Android already sends both to logcat.
//
// Once a second for 30 s it writes one marked line to each of stdout, stderr
// and package log:
//
//	stdlogprobe stdout n=3
//	stdlogprobe stderr n=3
//	stdlogprobe log n=3
//
// At start it also writes a 5000-byte stderr line, which reaches the log in
// pieces, and 1 MiB of stderr in one go, then logs "stdlogprobe burst
// done": that line proves a writer is never blocked by the pipe behind
// stderr. It then exits by itself.
//
// Package from this directory, install, and launch WITHOUT --console (with
// it, stdout and stderr are the console, and are left alone):
//
//	bellui package -os ios --app-id com.trendvidia.stdlogprobe \
//	    --cert "Apple Development: Benedicto Franco Jr. (D68Z8W925S)"
//	xcrun devicectl device install app --device <UDID> stdlogprobe.app
//	pymobiledevice3 syslog live --udid <UDID> -m stdlogprobe   # in another shell
//	xcrun devicectl device process launch --device <UDID> com.trendvidia.stdlogprobe
//
// All three kinds of line must appear in the log, each with its n, and each
// once: stdout at the default level, stderr at the error level, log at the
// default level with no second NSLog header.
//
// Measured on an iPad (10th gen, 2026-11-03), launched by devicectl without
// --console: before the fix no line of any kind arrived, package log
// included, though the app ran its 30 s and exited; after it all 30 of each,
// the burst finished, and the long line arrived in 1 KiB pieces.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("stdlogprobe")
	status := widget.NewLabel("writing to stdout and stderr for 30 s")
	w.SetContent(status)

	go func() {
		fmt.Fprintln(os.Stderr, "stdlogprobe long "+strings.Repeat("x", 5000))
		burst := strings.Repeat(strings.Repeat("b", 1023)+"\n", 1024)
		fmt.Fprint(os.Stderr, burst)
		log.Println("stdlogprobe burst done")

		for n := 1; n <= 30; n++ {
			fmt.Fprintf(os.Stdout, "stdlogprobe stdout n=%d\n", n)
			fmt.Fprintf(os.Stderr, "stdlogprobe stderr n=%d\n", n)
			log.Printf("stdlogprobe log n=%d", n)
			time.Sleep(time.Second)
		}
		log.Println("stdlogprobe done")
		bellui.Do(func() { status.SetText("done") })
		time.Sleep(time.Second)
		os.Exit(0)
	}()
	bellui.ShowAndRun(w)
}
