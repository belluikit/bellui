// Command recreateprobe live-verifies in-process Android activity recreation
// (#283): a configuration change the manifest does not declare (the guaranteed
// trigger is a per-app locale override on Android 13+) makes the framework
// destroy and recreate the activity inside the surviving process, and the
// toolkit must come back painting instead of hanging the Java main thread in
// the redraw handshake (ANR).
//
// It logs one machine-greppable line per lifecycle hook and two heartbeats:
//
//	recreateprobe alive=N   — from a plain goroutine; proves the process runs
//	recreateprobe tick=N    — through bellui.Do; proves the driver loop drains
//
// At a #283 hang, alive= keeps counting while tick= stops. After the fix both
// keep counting across the recreation and the lifecycle lines show the stage
// drop and re-entry.
//
// Repro on the pixel143 AVD (app id com.trendvidia.recreateprobe):
//
//	bellui package --target android/arm64 --app-id com.trendvidia.recreateprobe
//	adb install recreateprobe.apk && adb shell am start com.trendvidia.recreateprobe/org.golang.app.GoNativeActivity
//	adb shell cmd locale set-app-locales com.trendvidia.recreateprobe --user 0 --locales pt-BR
//	adb logcat -s Bellui | grep recreateprobe
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
	w := a.NewWindow("recreateprobe")

	tick := widget.NewLabel("tick: 0")
	w.SetContent(container.NewVBox(widget.NewLabel("recreateprobe (#283)"), tick))

	lc := a.Lifecycle()
	lc.SetOnStarted(func() { log.Println("recreateprobe lifecycle=started") })
	lc.SetOnStopped(func() { log.Println("recreateprobe lifecycle=stopped") })
	lc.SetOnEnteredForeground(func() { log.Println("recreateprobe lifecycle=foreground") })
	lc.SetOnExitedForeground(func() { log.Println("recreateprobe lifecycle=background") })

	go func() {
		for i := 1; ; i++ {
			time.Sleep(time.Second)
			log.Printf("recreateprobe alive=%d", i)
			n := i
			bellui.Do(func() {
				tick.SetText(fmt.Sprintf("tick: %d", n))
				log.Printf("recreateprobe tick=%d", n)
			})
		}
	}()

	bellui.ShowAndRun(w)
}
