// Command deviceinfoprobe live-verifies device configuration plumbing on a
// real device or emulator: the canvas scale derived from the reported density
// (#272), the raw locale strings the system hands go-locale (#271), and the
// locale the lang package actually settled on (#282 — on Android the init-time
// lookup runs before the JVM attaches, so without the app-creation re-resolve
// the active locale stays the bare English fallback "en").
//
// It shows the values in labels and prints one machine-greppable line to
// stdout (logcat tag Bellui on Android) two seconds after launch, once the
// first size event has landed:
//
//	deviceinfo scale=2.6250 sizePt=411x891 locales=["en-US"] localesErr=<nil> system=en-US current=en-US
//
// current is lang.CurrentLocale — the tag driving translations, CLDR
// formatting, collation and direction. On a healthy start it matches the
// device's first parseable locale entry (region included); "en" on a non-en-US
// device is the #282 stuck-fallback state. A fresh line is also printed on
// every foregrounding — the driver re-resolves the system locale just before
// the foreground hook (#282), so backgrounding the app, switching the system
// (or per-app: `adb shell cmd locale set-app-locales <id> --locales pt-BR`)
// language and returning captures the pickup without a tap; the "report
// again" button re-reads on demand.
//
// Android: package with `bellui package --target android/arm64` from this
// directory, install, launch, then `adb logcat -d -s Bellui | grep deviceinfo`.
package main

import (
	"fmt"
	"log"
	"time"

	locale "github.com/jeandeaual/go-locale"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("deviceinfoprobe")

	scaleLabel := widget.NewLabel("scale: collecting…")
	localeLabel := widget.NewLabel("locales: collecting…")
	localeLabel.Wrapping = bellui.TextWrapWord
	var report func()
	w.SetContent(container.NewVBox(scaleLabel, localeLabel,
		widget.NewButton("report again", func() { report() })))

	report = func() {
		c := w.Canvas()
		// GetLocales is what lang.updateLocalizer consults; log it with %q so
		// an empty or vendor-form entry is visible verbatim (#271).
		locs, err := locale.GetLocales()
		line := fmt.Sprintf("deviceinfo scale=%.4f sizePt=%.0fx%.0f locales=%q localesErr=%v system=%s current=%s",
			c.Scale(), c.Size().Width, c.Size().Height, locs, err, lang.SystemLocale(), lang.CurrentLocale())
		log.Println(line)
		scaleLabel.SetText(fmt.Sprintf("scale: %.4f (size %.0fx%.0f pt)", c.Scale(), c.Size().Width, c.Size().Height))
		localeLabel.SetText(fmt.Sprintf("locales: %q (err %v)\nsystem: %s\ncurrent: %s", locs, err, lang.SystemLocale(), lang.CurrentLocale()))
	}

	// The scale is settled by the first size event; report shortly after the
	// window is up, and again on demand via the button.
	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(report)
	}()

	// Report on every foregrounding: the mobile driver re-resolves the system
	// locale immediately before this hook fires (#282), so this line shows
	// whether a language switched while backgrounded was picked up.
	a.Lifecycle().SetOnEnteredForeground(report)

	bellui.ShowAndRun(w)
}
