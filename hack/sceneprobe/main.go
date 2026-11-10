// Command sceneprobe live-checks the iOS scene life cycle (#1117): an app built
// with the iOS 27 SDK must declare UIApplicationSceneManifest, or UIKit traps
// in UIApplicationMain before anything draws. It logs, to the console:
//
//	sceneprobe lifecycle=started|foreground|background|stopped
//	sceneprobe drawn n=3 size=1640x2360 centre=rgba(230,40,90,255) want=rgba(230,40,90,255) ok=true
//	sceneprobe url=sceneprobe://cold?x=1
//
// The lines are:
//
//   - lifecycle: the app's lifecycle callbacks, which the scene delegate's
//     sceneDidBecomeActive / sceneDidEnterBackground now drive.
//   - drawn: once a second while in the foreground, the centre pixel of a
//     capture of the window, which is filled with one colour. ok=true means the
//     window the scene delegate made is the one bellui paints.
//   - url: every URL the app is opened with. A URL that launches the app comes
//     through the scene's connection options; a URL that arrives while it runs
//     comes through scene:openURLContexts:. Four seconds after start the probe
//     opens sceneprobe://warm itself to exercise the second path.
//
// The probe exits by itself after a minute, so a run left behind stops.
//
// Package from this directory and run on a device:
//
//	bellui package -os ios --app-id com.trendvidia.sceneprobe \
//	    --cert "Apple Development: Benedicto Franco Jr. (D68Z8W925S)"
//	xcrun devicectl device install app --device <UDID> sceneprobe.app
//	xcrun devicectl device process launch --console --device <UDID> \
//	    --payload-url 'sceneprobe://cold?x=1' com.trendvidia.sceneprobe
//
// Then send it to the background and back: launch another app (for example
// `devicectl device process launch --device <UDID> com.apple.Preferences`),
// and launch the probe again without --terminate-existing.
package main

import (
	"image/color"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
)

// fill is the window's one colour, the value the drawn line checks for.
var fill = color.NRGBA{R: 230, G: 40, B: 90, A: 255}

// runFor is how long the probe runs before it exits by itself.
const runFor = time.Minute

func main() {
	a := app.NewWithID("com.trendvidia.sceneprobe")
	w := a.NewWindow("sceneprobe")
	w.SetContent(canvas.NewRectangle(fill))

	if oh, ok := a.(bellui.OpenHandlerApp); ok {
		oh.SetOnURLOpened(func(u *url.URL) { log.Printf("sceneprobe url=%s", u) })
	} else {
		log.Println("sceneprobe url=unsupported (no OpenHandlerApp)")
	}

	foreground := false
	lc := a.Lifecycle()
	lc.SetOnStarted(func() { log.Println("sceneprobe lifecycle=started") })
	lc.SetOnStopped(func() { log.Println("sceneprobe lifecycle=stopped") })
	lc.SetOnEnteredForeground(func() {
		foreground = true
		log.Println("sceneprobe lifecycle=foreground")
	})
	lc.SetOnExitedForeground(func() {
		foreground = false
		log.Println("sceneprobe lifecycle=background")
	})

	go func() {
		for n := 1; ; n++ {
			time.Sleep(time.Second)
			bellui.Do(func() {
				if foreground {
					logDrawn(w, n)
				}
			})
		}
	}()
	time.AfterFunc(4*time.Second, func() {
		u, _ := url.Parse("sceneprobe://warm")
		bellui.Do(func() {
			if err := bellui.OpenURL(u); err != nil {
				log.Printf("sceneprobe openurl error=%v", err)
			}
		})
	})
	time.AfterFunc(runFor, func() {
		log.Printf("sceneprobe exit after %v", runFor)
		os.Exit(0)
	})

	bellui.ShowAndRun(w)
}

// logDrawn logs the centre pixel of a capture of w. Capture runs on the main
// goroutine, where bellui.Do runs this.
func logDrawn(w bellui.Window, n int) {
	img := w.Canvas().Capture()
	b := img.Bounds()
	got := color.NRGBAModel.Convert(img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2)).(color.NRGBA)
	log.Printf("sceneprobe drawn n=%d size=%dx%d centre=rgba(%d,%d,%d,%d) want=rgba(%d,%d,%d,%d) ok=%v",
		n, b.Dx(), b.Dy(), got.R, got.G, got.B, got.A, fill.R, fill.G, fill.B, fill.A, got == fill)
}
