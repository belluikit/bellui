// Command androidwebviewprobe is the #76 on-device pass for the Android
// android.webkit.WebView backend (#63): hosting on the decor view, the
// runOnUiThread threading discipline under churn (the #973 deadlock shape),
// geometry + native clip inside a container.Scroll, the #59 focus bridge, the
// bridge and navigation API, and teardown.
//
// On mobile there is no programmatic tap injection, so the flow self-drives
// what it can on a timer and appends every result to the on-screen log AND to
// stdout — launch with `adb logcat (tag GoLog)` and the
// results stream back to the host machine. The two steps that need a finger
// are prompted on screen and detected automatically when done:
//
//   - tap INSIDE the web page's input → the probe watches the native view
//     take the OS focus and bellui release its canvas focus;
//   - tap the bellui Entry → the probe watches the web view resign.
//
// Both taps can be injected from the host machine with `adb shell input tap`
// — the probe prints a tapinfo line with the canvas coordinates, scale and
// inset needed to compute device pixels. The spinner top-right animates for
// the whole run; a freeze means the UI thread stalled.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/webview"
	"github.com/belluikit/bellui/widget"
)

const page = `data:text/html,` +
	`<html><body style="margin:0;background:rgb(255,0,170);font:600 28px -apple-system;color:white">` +
	`<div style="padding:16px">WEBVIEW on iOS</div>` +
	`<div style="padding:16px;font-size:16px">tap anywhere in this magenta area when asked</div>` +
	`<input id="in" style="font-size:20px;margin:16px" placeholder="page input">` +
	`<div style="height:900px"></div>` +
	`<script>` +
	// On Android (and wasm) the shim arrives at document LOAD, not document
	// start — the documented #63 bend — so the page's own first lines may not
	// see window.bellui. Registering via a short poll is the pattern a real
	// page needs on those platforms, and it is portable everywhere else.
	`(function reg() {` +
	`  if (window.bellui) { bellui.handle('greet', function (arg) { return 'hello ' + arg.name; }); }` +
	`  else { setTimeout(reg, 50); }` +
	`})();` +
	`</script></body></html>`

func main() {
	a := app.NewWithID("com.trendvidia.androidwebviewprobe")
	w := a.NewWindow("webview #76")

	logLbl := widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	var lines []string
	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		fmt.Println("[probe] " + msg) // devicectl --console streams this back
		lines = append(lines, msg)
		if len(lines) > 12 {
			lines = lines[len(lines)-12:]
		}
		bellui.Do(func() { logLbl.SetText(strings.Join(lines, "\n")) })
	}

	host, view := webview.NewHost(page)
	view.EnableBridge(webview.TrustOrigins(webview.OriginOpaque))

	var loads []webview.LoadEvent
	view.OnLoadChanged = func(ev webview.LoadEvent) { loads = append(loads, ev) }
	blocked := ""
	view.OnNavigate = func(req webview.NavigationRequest) bool {
		return blocked == "" || !strings.Contains(req.URL, blocked)
	}

	host.SetMinSize(geo.NewSize(300, 380))
	tall := container.NewVBox(host, widget.NewLabel("below the web view — only this strip scrolls by touch; the web area above owns its own touches (hole punch)"))
	scroll := container.NewScroll(tall)
	scroll.SetMinSize(geo.NewSize(320, 300))

	entry := widget.NewEntry()
	entry.SetPlaceHolder("bellui Entry — tap when asked")

	spinner := widget.NewProgressBarInfinite() // frozen == the #1124 stall

	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, spinner, widget.NewLabel("#76 webview pass")),
		container.NewVBox(entry, logLbl),
		nil, nil, scroll,
	))

	go run(a, w, host, view, scroll, entry, &loads, &blocked, logf)
	bellui.ShowAndRun(w)
}

func run(a bellui.App, w bellui.Window, host *webview.Host, view *webview.WebView,
	scroll *container.Scroll, entry *widget.Entry, loads *[]webview.LoadEvent,
	blocked *string, logf func(string, ...any),
) {
	step := func(d time.Duration) { time.Sleep(d) }
	step(3 * time.Second) // window maps, WKWebView attaches and loads

	// --- hosting + load -----------------------------------------------------
	ok := false
	for i := 0; i < 20 && !ok; i++ {
		bellui.DoAndWait(func() {
			for _, ev := range *loads {
				if ev.State == webview.LoadStateComplete {
					ok = true
				}
			}
		})
		step(500 * time.Millisecond)
	}
	logf("1 hosting/load: %s — page visible? spinner moving? (the #1124 stall check is by eye)", pf(ok))

	// --- bridge round-trip ---------------------------------------------------
	done := make(chan string, 1)
	bellui.DoAndWait(func() {
		view.Call("greet", map[string]string{"name": "droid"}, func(res json.RawMessage, err error) {
			if err != nil {
				done <- "err: " + err.Error()
				return
			}
			done <- string(res)
		})
	})
	select {
	case r := <-done:
		logf("2 bridge Call: %s (%s)", pf(r == `"hello droid"`), r)
	case <-time.After(8 * time.Second):
		logf("2 bridge Call: FAIL — no answer in 8s")
	}

	// --- eval result ---------------------------------------------------------
	er := make(chan string, 1)
	bellui.DoAndWait(func() {
		view.EvalResult("20+22", func(res json.RawMessage, err error) {
			if err != nil {
				er <- "err: " + err.Error()
				return
			}
			er <- string(res)
		})
	})
	select {
	case r := <-er:
		logf("3 eval result: %s (%s)", pf(r == "42"), r)
	case <-time.After(8 * time.Second):
		logf("3 eval result: FAIL — no answer in 8s")
	}

	// --- vetoed navigation ---------------------------------------------------
	*blocked = "blocked.invalid"
	before := 0
	bellui.DoAndWait(func() { before = len(*loads) })
	bellui.DoAndWait(func() { view.LoadURL("https://blocked.invalid/nope") })
	veto := false
	for i := 0; i < 16 && !veto; i++ {
		bellui.DoAndWait(func() {
			for _, ev := range (*loads)[before:] {
				if ev.State == webview.LoadStateFailed && errors.Is(ev.Err, webview.ErrNavigationCancelled) {
					veto = true
				}
			}
		})
		step(500 * time.Millisecond)
	}
	logf("4 nav veto: %s — refused load reported cancelled", pf(veto))

	// --- clip: scroll the host half out, then fully out ----------------------
	bellui.DoAndWait(func() { scroll.Offset = geo.NewPos(0, 190); scroll.Refresh() })
	logf("5 clip: LOOK — page must be CUT at the scroll edge, not overflow (7s)")
	step(7 * time.Second)
	bellui.DoAndWait(func() { scroll.Offset = geo.NewPos(0, 0); scroll.Refresh() })

	// --- threading churn: the #973 shape under load ---------------------------
	// Scroll offsets, focus flips and shadow reads hammered together: every
	// down-call posts to the UI thread, every read is shadow state, and the
	// navigation veto is the one DoAndWait — a deadlock here freezes the app
	// (and this log goes silent).
	churnEnd := time.Now().Add(5 * time.Second)
	n := 0
	for time.Now().Before(churnEnd) {
		n++
		bellui.DoAndWait(func() {
			scroll.Offset = geo.NewPos(0, float32(10+(n%18)*10))
			scroll.Refresh()
			host.FocusNativeView()
			view.CanGoBack()
			w.Canvas().Unfocus()
		})
		time.Sleep(16 * time.Millisecond)
	}
	logf("5b threading churn: PASS — %d scroll+focus+shadow iterations, no deadlock", n)
	bellui.DoAndWait(func() { scroll.Offset = geo.NewPos(0, 0); scroll.Refresh() })

	// Reset focus so step 6 measures a real transition, not churn residue:
	// focusing the Entry makes the host's bridge resign the web view, then
	// unfocusing leaves everything neutral.
	bellui.DoAndWait(func() { w.Canvas().Focus(entry) })
	step(1 * time.Second)
	bellui.DoAndWait(func() { w.Canvas().Unfocus() })
	step(1 * time.Second)
	neutral := false
	bellui.DoAndWait(func() { neutral = !host.NativeViewFocused() && w.Canvas().Focused() == nil })
	logf("   focus reset: neutral=%v (both sides must start empty)", neutral)

	// tapinfo: everything the host machine needs to aim adb shell input tap.
	bellui.DoAndWait(func() {
		d := bellui.CurrentApp().Driver()
		hp := d.AbsolutePositionForObject(host)
		ep := d.AbsolutePositionForObject(entry)
		es := entry.Size()
		ip, _ := w.Canvas().InteractiveArea()
		logf("tapinfo host=(%.0f,%.0f) entry=(%.0f,%.0f size %.0fx%.0f) scale=%.2f inset=(%.0f,%.0f)",
			hp.X, hp.Y, ep.X, ep.Y, es.Width, es.Height, w.Canvas().Scale(), ip.X, ip.Y)
	})

	// --- focus bridge: native side ------------------------------------------
	logf("6 focus: TAP the white 'page input' field in the magenta page now")
	stopTrace := traceFocus(w, host, view, logf)
	native := waitFor(25*time.Second, func() bool {
		got := false
		bellui.DoAndWait(func() { got = host.NativeViewFocused() && w.Canvas().Focused() == nil })
		return got
	})
	logf("6 focus native<-tap: %s — web view took OS focus, bellui released", pf(native))

	// --- focus bridge: bellui side ------------------------------------------
	logf("7 focus: now TAP the bellui Entry below")
	back := waitFor(25*time.Second, func() bool {
		got := false
		bellui.DoAndWait(func() { got = !host.NativeViewFocused() && w.Canvas().Focused() != nil })
		return got
	})
	stopTrace()
	logf("7 focus bellui<-tap: %s — web view resigned when the Entry took focus", pf(back))

	// --- ResignFocus must not raise the keyboard -----------------------------
	bellui.DoAndWait(func() {
		w.Canvas().Unfocus()
		host.FocusNativeView()
	})
	step(1 * time.Second)
	bellui.DoAndWait(func() { w.Canvas().Unfocus() }) // leave web view, focus nothing
	logf("8 keyboard: LOOK — did a keyboard just pop up with nothing focused? It must NOT (7s)")
	step(7 * time.Second)

	// --- teardown ------------------------------------------------------------
	pending := make(chan error, 1)
	bellui.DoAndWait(func() {
		view.Call("greet", map[string]string{"name": "x"}, func(_ json.RawMessage, err error) { pending <- err })
		view.Destroy()
	})
	select {
	case err := <-pending:
		logf("9 teardown: %s — pending Call failed with ErrNotAttached (%v)", pf(errors.Is(err, webview.ErrNotAttached)), err)
	case <-time.After(5 * time.Second):
		logf("9 teardown: FAIL — pending Call was never failed")
	}
	logf("done. leave the app open if any step needs a re-look; results were streamed to the console")
	_ = a
}

// traceFocus streams every change of the three focus facts while the manual
// steps run, so a failed check says WHICH layer never moved: the backend's
// first-responder shadow (raw), the host's bridged state (native), or the
// canvas focus (canvas).
func traceFocus(w bellui.Window, host *webview.Host, view *webview.WebView, logf func(string, ...any)) func() {
	stop := make(chan struct{})
	go func() {
		last := ""
		for {
			select {
			case <-stop:
				return
			case <-time.After(500 * time.Millisecond):
			}
			state := ""
			bellui.DoAndWait(func() {
				state = fmt.Sprintf("raw=%v native=%v canvas=%v",
					view.HasFocus(), host.NativeViewFocused(), w.Canvas().Focused() != nil)
			})
			if state != last {
				last = state
				logf("   focus trace: %s", state)
			}
		}
	}()
	return func() { close(stop) }
}

func pf(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}
