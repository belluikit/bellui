// Command recreateprobe live-verifies in-process Android activity recreation
// (#283): a configuration change the manifest does not declare makes the
// framework destroy and recreate the activity inside the surviving process,
// and the toolkit must come back painting instead of hanging the Java main
// thread in the redraw handshake (ANR).
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
// The original #283 trigger — an Android 13+ per-app locale override — stopped
// recreating once the packaged manifest declared locale|layoutDirection
// (#287): it now arrives as an in-place onConfigurationChanged, so on a
// current build that command must produce NO lifecycle lines at all (the #288
// half of the check; deviceinfoprobe shows the locale was still picked up).
// Exception: ColorOS 16 (OPD2403) relaunches anyway — its
// ActivityRecordExtImpl.hookShouldRelaunchLocked adds CONFIG_SCREEN_LAYOUT to
// the change mask, which the manifest does not declare — so on that device
// expect the recreation lines and the locale pickup on the re-cross.
// To force a recreation use a config change the manifest does not declare: a
// font-scale change recreates promptly and reliably; `wm density` proved
// unreliable on the goldfish AVD (2026-08-05: sometimes no relaunch at all,
// and a relaunch on a cold-booted emulator ANRed in host-side paging).
//
// Repro on the pixel143 AVD (app id com.trendvidia.recreateprobe):
//
//	bellui package --target android/arm64 --app-id com.trendvidia.recreateprobe
//	adb install recreateprobe.apk && adb shell am start com.trendvidia.recreateprobe/org.golang.app.GoNativeActivity
//	adb shell settings put system font_scale 1.3   # recreation trigger (1.0 to restore)
//	adb shell cmd locale set-app-locales com.trendvidia.recreateprobe --user 0 --locales pt-BR   # in-place since #287/#288: no lifecycle lines
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
