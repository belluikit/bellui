// Command backprobe live-verifies Android Back (#935): an app targeting API
// 36 on Android 16 gets no onBackPressed and no KEYCODE_BACK, so Back must
// arrive through GoNativeActivity's OnBackInvokedCallback.
//
// The probe walks four stages. Each stage logs to logcat (tag Bellui) as
//
//	backprobe stage=<n> <what is open>
//	backprobe closed=<what> by back
//
// and moves on 1.5 s after the open thing closes:
//
//  1. a dialog: Back must close it;
//  2. a pop-up: Back must hide it;
//  3. the keyboard, raised by focusing an Entry: Back must lower it;
//  4. nothing open: Back must leave the app (the activity stops).
//
// Press Back once per stage (adb shell input keyevent KEYCODE_BACK, or the
// back gesture) and read the log. A stage that does not close means Back
// never reached bellui.
package main

import (
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.backprobe")
	w := a.NewWindow("backprobe")
	status := widget.NewLabel("backprobe")
	entry := widget.NewEntry()
	w.SetContent(container.NewVBox(status, entry))

	a.Lifecycle().SetOnStopped(func() { log.Println("backprobe lifecycle=stopped") })
	a.Lifecycle().SetOnEnteredForeground(func() { log.Println("backprobe lifecycle=foreground") })

	stage := func(n int, what string) {
		status.SetText(what)
		log.Printf("backprobe stage=%d %s", n, what)
	}
	next := func(f func()) {
		time.AfterFunc(1500*time.Millisecond, func() { bellui.Do(f) })
	}

	var popup, keyboard, nothing func()
	dlg := func() {
		stage(1, "dialog")
		d := dialog.NewInformation("backprobe", "Press Back to close this dialog", w)
		d.SetOnClosed(func() {
			log.Println("backprobe closed=dialog by back")
			next(popup)
		})
		d.Show()
	}
	popup = func() {
		stage(2, "popup")
		p := widget.NewModalPopUp(widget.NewLabel("Press Back to hide this pop-up"), w.Canvas())
		p.Show()
		go func() {
			for {
				time.Sleep(100 * time.Millisecond)
				hidden := false
				bellui.DoAndWait(func() { hidden = !p.Visible() })
				if hidden {
					log.Println("backprobe closed=popup by back")
					next(keyboard)
					return
				}
			}
		}()
	}
	keyboard = func() {
		stage(3, "keyboard")
		w.Canvas().Focus(entry)
		// Whether the keyboard is up is read from the device
		// (dumpsys input_method); bellui does not report it to the app.
		time.AfterFunc(6*time.Second, func() { bellui.Do(nothing) })
	}
	nothing = func() {
		w.Canvas().Unfocus()
		stage(4, "nothing")
	}

	next(dlg)
	bellui.ShowAndRun(w)
}
