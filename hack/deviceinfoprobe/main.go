// Command deviceinfoprobe live-verifies device configuration plumbing on a
// real device or emulator: the canvas scale derived from the reported density
// (#272) and the raw locale strings the system hands go-locale (#271).
//
// It shows the values in labels and prints one machine-greppable line to
// stdout (logcat tag Bellui on Android) two seconds after launch, once the
// first size event has landed:
//
//	deviceinfo scale=2.6250 sizePt=411x891 locales=["en-US"] localesErr=<nil> system=en-US
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
		line := fmt.Sprintf("deviceinfo scale=%.4f sizePt=%.0fx%.0f locales=%q localesErr=%v system=%s",
			c.Scale(), c.Size().Width, c.Size().Height, locs, err, lang.SystemLocale())
		log.Println(line)
		scaleLabel.SetText(fmt.Sprintf("scale: %.4f (size %.0fx%.0f pt)", c.Scale(), c.Size().Width, c.Size().Height))
		localeLabel.SetText(fmt.Sprintf("locales: %q (err %v)\nsystem: %s", locs, err, lang.SystemLocale()))
	}

	// The scale is settled by the first size event; report shortly after the
	// window is up, and again on demand via the button.
	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(report)
	}()

	bellui.ShowAndRun(w)
}
