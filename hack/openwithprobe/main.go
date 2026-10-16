// Command openwithprobe verifies iOS "open with" / deep-link delivery (pre-rebrand tracker #622):
// bellui.OpenHandlerApp's SetOnURLOpened / SetOnFileOpened firing from the native
// application:openURL: delegate, end to end including the packaging metadata
// (CFBundleURLTypes emitted from manifest.pxf URLSchemes = ["belluiopen"], pre-rebrand tools tracker #4).
//
// It self-verifies: a few seconds after launch it calls bellui.OpenURL on its own
// registered scheme (belluiopen://selftest). iOS resolves the scheme back to this
// app and delivers it through application:openURL: -> onOpenIntent -> the
// SetOnURLOpened callback, which appends to the on-screen log. If the log shows
// the URL, the whole receive+route chain works on device.
package main

import (
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("com.trendvidia.openwithprobe")
	w := a.NewWindow("Open-With Probe (#622)")

	logLbl := widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	var lines []string
	logf := func(format string, args ...any) {
		msg := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
		lines = append(lines, msg)
		bellui.Do(func() { logLbl.SetText(joinLines(lines)) })
	}

	oh, ok := a.(bellui.OpenHandlerApp)
	logf("OpenHandlerApp supported = %v", ok)
	if ok {
		oh.SetOnURLOpened(func(u *url.URL) {
			logf("✅ SetOnURLOpened fired: %s", u.String())
		})
		oh.SetOnFileOpened(func(uri bellui.URI) {
			logf("✅ SetOnFileOpened fired: %s", uri.String())
			// Prove the handed-over file is actually READABLE (the pre-rebrand tracker #622
			// security-scoped-resource point) — read + show a preview.
			rc, err := storage.Reader(uri)
			if err != nil {
				logf("   ❌ read open failed: %v", err)
				return
			}
			defer rc.Close()
			data, err := io.ReadAll(io.LimitReader(rc, 200))
			if err != nil {
				logf("   ❌ read failed: %v", err)
				return
			}
			logf("   📄 read %d bytes: %q", len(data), string(data))
		})
	}

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("iOS open-with probe (#622)", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		widget.NewLabel("scheme: belluiopen://  (declared in manifest.pxf)"),
		widget.NewButton("Self-open belluiopen://buttontap", func() {
			openSelf(logf, "belluiopen://buttontap")
		}),
		widget.NewSeparator(),
		logLbl,
	))
	w.Resize(geo.NewSize(700, 700))

	// Self-verify: open our own scheme so application:openURL: routes it back.
	time.AfterFunc(3*time.Second, func() {
		bellui.Do(func() { openSelf(logf, "belluiopen://selftest?ok=1") })
	})
	logf("probe started — will self-open in 3s")

	bellui.ShowAndRun(w)
}

func openSelf(logf func(string, ...any), raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		logf("bad url %q: %v", raw, err)
		return
	}
	logf("calling bellui.OpenURL(%s) …", raw)
	if err := bellui.OpenURL(u); err != nil {
		logf("OpenURL error: %v", err)
	}
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
