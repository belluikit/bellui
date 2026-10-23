// Command bgaudioprobe live-checks that a bellui iOS app keeps playing audio
// after it leaves the screen: the use case behind belluikit/player#72, and the
// end-to-end proof that ios/Info.plist's UIBackgroundModes = [audio] (#672)
// does what it is for.
//
// At launch it sets a playback audio session and plays a quiet beep (0.5 s on,
// 0.5 s off) through AVAudioEngine. Once a second it logs:
//
//		bgaudioprobe n=12 state=background frames=529200 running=true interruptions=0 tick=12
//
//	  - frames: audio frames the engine has rendered so far. Climbing while
//	    state=background is the proof that audio plays in the background; you
//	    should also still hear the beep.
//	  - state: UIApplication's state as iOS reports it (active, inactive,
//	    background), tracked from its notifications.
//	  - n counts from a plain goroutine, tick through bellui.Do: both keep
//	    counting only while the process is not suspended.
//
// Without the background mode, iOS suspends the app a few seconds after it
// leaves the screen: the lines stop, and the beep with them.
//
// Package from this directory and run on a device:
//
//	bellui package -os ios --app-id com.trendvidia.bgaudioprobe \
//	    --cert "Apple Development: Benedicto Franco Jr. (D68Z8W925S)"
//	xcrun devicectl device install app --device <UDID> bgaudioprobe.app
//	xcrun devicectl device process launch --console --device <UDID> com.trendvidia.bgaudioprobe
//
// then press Home (or launch another app) and watch the lines continue.
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
	w := a.NewWindow("bgaudioprobe")
	status := widget.NewLabel("starting audio…")
	tick := widget.NewLabel("tick: 0")
	w.SetContent(container.NewVBox(widget.NewLabel("bgaudioprobe (player#72)"), status, tick))

	lc := a.Lifecycle()
	lc.SetOnEnteredForeground(func() { log.Println("bgaudioprobe lifecycle=foreground") })
	lc.SetOnExitedForeground(func() { log.Println("bgaudioprobe lifecycle=background") })

	// Start audio once UIKit is running: the session and engine are set up on
	// the iOS main thread, which must not be the one waiting for it.
	lc.SetOnStarted(func() {
		go func() {
			err := startAudio()
			msg := "playing a beep; press Home and keep listening"
			if err != nil {
				log.Printf("bgaudioprobe audio=error %v", err)
				msg = "audio error: " + err.Error()
			} else {
				log.Println("bgaudioprobe audio=started")
			}
			bellui.Do(func() { status.SetText(msg) })
		}()
	})

	go func() {
		var ticks int
		for n := 1; ; n++ {
			time.Sleep(time.Second)
			s := audioStats()
			log.Printf("bgaudioprobe n=%d state=%s frames=%d running=%v interruptions=%d tick=%d",
				n, s.state, s.frames, s.running, s.interruptions, ticks)
			bellui.Do(func() {
				ticks++
				tick.SetText(fmt.Sprintf("tick: %d", ticks))
			})
		}
	}()

	bellui.ShowAndRun(w)
}

type stats struct {
	state         string
	frames        uint64
	running       bool
	interruptions int
}
