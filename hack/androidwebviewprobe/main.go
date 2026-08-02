// Command androidwebviewprobe is the on-device pass for the Android
// android.webkit.WebView backend (#63, #76) and the #143 IME polish work:
// hosting in the panel window, the runOnUiThread threading discipline under
// churn (the #973 deadlock shape), geometry + native clip inside a
// container.Scroll, the #59 focus bridge, the bridge and navigation API,
// teardown — and instrumented cold-tap / keystroke-latency measurement.
//
// The probe appends every result to the on-screen log AND to stdout — launch
// with `adb logcat -s Bellui` and the results stream back to the host machine.
// Taps are injected from the host with `adb shell input tap` aimed by the
// tapinfo lines (canvas coordinates, scale and inset for computing device
// pixels); hack/androidwebviewprobe/drive.sh automates the whole loop.
//
// Modes, selected by a system property so no rebuild or reinstall is needed
// (`adb shell setprop debug.bellui.probe <mode>` before launching):
//
//   - "" (default): the scripted #76 flow. Steps 6/7 wait for a tap (adb
//     injectable); a fresh tapinfo line is emitted after step 6 because the
//     keyboard-driven relayout moves the Entry (#143 item 3), and step 7b
//     drives the same handback programmatically through the
//     FocusNativeView/Canvas.Focus seam, no finger needed.
//   - "coldtap": hosts a full-viewport page input (any tap inside the host
//     area lands on the editor — no CSS coordinate math) and then idles;
//     the launch → tap → keyboard-visible? loop is driven entirely from the
//     host machine (#143 item 1).
//   - "latency": the same full-viewport input plus in-page instrumentation
//     that reports, per keystroke, the delivery latency (event.timeStamp →
//     JS handler) and the paint latency (input event → next rAF) over the
//     bridge. The stall-detector spinner is NOT created in this mode — it
//     forces a full-canvas GL redraw at frame rate, which is probe cost,
//     not backend cost (#143 item 2).
//   - "latency-spinner": latency mode WITH the spinner, to price the
//     spinner itself by comparison.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/webview"
	"github.com/belluikit/bellui/widget"
)

const page = `data:text/html,` +
	`<html><body style="margin:0;background:rgb(255,0,170);font:600 28px sans-serif;color:white">` +
	`<div style="padding:16px">WEBVIEW on Android</div>` +
	`<div style="padding:16px;font-size:16px">tap anywhere in this magenta area when asked</div>` +
	// Explicit geometry so drive.sh can aim: block flow puts this input's
	// center near (146,151) CSS px from the page's top-left.
	`<input id="in" style="font-size:20px;margin:16px;width:260px;height:40px;box-sizing:border-box" placeholder="page input">` +
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

// fullInputPage fills the whole viewport with one page editor, so an
// adb-injected tap anywhere inside the host rectangle is a tap on the
// editor — the coordinate contract the coldtap and latency loops need.
// The latency instrumentation reports two numbers per keystroke:
//
//	deliver: performance.now()-event.timeStamp at the keydown handler — the
//	         platform's routing cost from event creation (or adb injection)
//	         to the renderer's JS thread;
//	paint:   input event → the next requestAnimationFrame — how long the
//	         renderer took to get the edit on screen.
const fullInputPage = `data:text/html,` +
	`<html><body style="margin:0;background:rgb(255,0,170)">` +
	`<input id="in" autocomplete="off" placeholder="tap here" ` +
	`style="box-sizing:border-box;width:100%25;height:100vh;border:none;outline:none;` +
	`background:rgb(255,0,170);color:white;font:600 28px sans-serif;padding:16px">` +
	`<script>` +
	`(function reg() {` +
	`  if (!window.bellui) { setTimeout(reg, 50); return; }` +
	`  var inp = document.getElementById('in');` +
	`  inp.addEventListener('keydown', function (e) {` +
	`    var d = performance.now() - e.timeStamp;` +
	`    if (d >= 0 && d < 10000) { bellui.post({deliver: d}); }` +
	`  });` +
	`  inp.addEventListener('input', function () {` +
	`    var t0 = performance.now();` +
	`    requestAnimationFrame(function () { bellui.post({paint: performance.now() - t0}); });` +
	`  });` +
	`})();` +
	`</script></body></html>`

// probeMode reads debug.bellui.probe. getprop is exec'd rather than bound:
// SystemProperties has no public API, and a probe can afford the fork. An
// error (SELinux, not Android) is the default mode.
func probeMode() string {
	out, err := exec.Command("/system/bin/getprop", "debug.bellui.probe").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func main() {
	mode := probeMode()

	a := app.NewWithID("com.trendvidia.androidwebviewprobe")
	w := a.NewWindow("webview #143")

	logLbl := widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	var lines []string
	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		fmt.Println("[probe] " + msg) // adb logcat -s Bellui streams this back
		lines = append(lines, msg)
		if len(lines) > 12 {
			lines = lines[len(lines)-12:]
		}
		bellui.Do(func() { logLbl.SetText(strings.Join(lines, "\n")) })
	}

	pageURL := page
	if mode == "coldtap" || strings.HasPrefix(mode, "latency") {
		pageURL = fullInputPage
	}

	host, view := webview.NewHost(pageURL)
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

	// The spinner is the stall detector (frozen == the #1124 stall) — and a
	// full-canvas GL redraw per frame, which is exactly the probe cost the
	// latency measurement must exclude (#143 item 2).
	var top core.CanvasObject = widget.NewLabel("#143 webview pass [" + mode + "]")
	if mode != "latency" {
		top = container.NewBorder(nil, nil, nil, widget.NewProgressBarInfinite(), top)
	}

	w.SetContent(container.NewBorder(
		top,
		container.NewVBox(entry, logLbl),
		nil, nil, scroll,
	))

	switch {
	case mode == "coldtap":
		go runColdTap(w, host, entry, &loads, logf)
	case strings.HasPrefix(mode, "latency"):
		go runLatency(w, host, view, entry, &loads, logf)
	default:
		go run(a, w, host, view, scroll, entry, &loads, &blocked, logf)
	}
	bellui.ShowAndRun(w)
}

// tapinfo emits everything the host machine needs to aim `adb shell input
// tap`: canvas-point positions, the canvas scale, and the interactive-area
// inset. Emitted fresh whenever asked because keyboard-driven relayout moves
// the Entry (#143 item 3) — the host must always aim with the LATEST line.
func tapinfo(w bellui.Window, host *webview.Host, entry *widget.Entry, logf func(string, ...any)) {
	bellui.DoAndWait(func() {
		d := bellui.CurrentApp().Driver()
		hp := d.AbsolutePositionForObject(host)
		hs := host.Size()
		ep := d.AbsolutePositionForObject(entry)
		es := entry.Size()
		ip, _ := w.Canvas().InteractiveArea()
		logf("tapinfo host=(%.0f,%.0f size %.0fx%.0f) entry=(%.0f,%.0f size %.0fx%.0f) scale=%.2f inset=(%.0f,%.0f)",
			hp.X, hp.Y, hs.Width, hs.Height, ep.X, ep.Y, es.Width, es.Height,
			w.Canvas().Scale(), ip.X, ip.Y)
	})
}

// runColdTap hosts the full-viewport editor and idles: launching, tapping,
// and asking dumpsys whether the keyboard came are all the host machine's
// job (drive.sh coldtap). The probe's contributions are the ready signal and
// a tapinfo line re-emitted whenever geometry changes.
func runColdTap(w bellui.Window, host *webview.Host, entry *widget.Entry,
	loads *[]webview.LoadEvent, logf func(string, ...any),
) {
	ok := false
	for i := 0; i < 40 && !ok; i++ {
		bellui.DoAndWait(func() {
			for _, ev := range *loads {
				if ev.State == webview.LoadStateComplete {
					ok = true
				}
			}
		})
		time.Sleep(250 * time.Millisecond)
	}
	logf("coldtap ready: load=%s", pf(ok))
	last := ""
	for {
		cur := ""
		bellui.DoAndWait(func() {
			d := bellui.CurrentApp().Driver()
			hp := d.AbsolutePositionForObject(host)
			hs := host.Size()
			cur = fmt.Sprintf("%.0f,%.0f,%.0fx%.0f,%.2f", hp.X, hp.Y, hs.Width, hs.Height, w.Canvas().Scale())
		})
		if cur != last {
			last = cur
			tapinfo(w, host, entry, logf)
		}
		time.Sleep(2 * time.Second)
	}
}

// runLatency aggregates the per-keystroke samples the page posts over the
// bridge and prints a summary block every 20 samples. The host drives the
// keystrokes (drive.sh latency): promote the panel with a tap, then
// `adb shell input text` in a loop.
func runLatency(w bellui.Window, host *webview.Host, view *webview.WebView,
	entry *widget.Entry, loads *[]webview.LoadEvent, logf func(string, ...any),
) {
	var deliver, paint []float64
	report := func(name string, s []float64) {
		if len(s) == 0 {
			return
		}
		c := append([]float64(nil), s...)
		sort.Float64s(c)
		logf("latency %s n=%d min=%.1f p50=%.1f p90=%.1f max=%.1f (ms)",
			name, len(c), c[0], c[len(c)/2], c[len(c)*9/10], c[len(c)-1])
	}
	bellui.DoAndWait(func() {
		view.OnMessage = func(msg webview.Message) {
			var sample struct {
				Deliver *float64 `json:"deliver"`
				Paint   *float64 `json:"paint"`
			}
			if err := msg.Decode(&sample); err != nil {
				return
			}
			switch {
			case sample.Deliver != nil:
				deliver = append(deliver, *sample.Deliver)
			case sample.Paint != nil:
				paint = append(paint, *sample.Paint)
				if len(paint)%20 == 0 {
					report("deliver", deliver)
					report("paint", paint)
				}
			}
		}
	})

	ok := false
	for i := 0; i < 40 && !ok; i++ {
		bellui.DoAndWait(func() {
			for _, ev := range *loads {
				if ev.State == webview.LoadStateComplete {
					ok = true
				}
			}
		})
		time.Sleep(250 * time.Millisecond)
	}
	logf("latency ready: load=%s — tap the page, then inject text", pf(ok))
	tapinfo(w, host, entry, logf)
	for {
		time.Sleep(30 * time.Second)
		report("deliver", deliver)
		report("paint", paint)
	}
}

func run(a bellui.App, w bellui.Window, host *webview.Host, view *webview.WebView,
	scroll *container.Scroll, entry *widget.Entry, loads *[]webview.LoadEvent,
	blocked *string, logf func(string, ...any),
) {
	step := func(d time.Duration) { time.Sleep(d) }
	step(3 * time.Second) // window maps, the WebView attaches and loads

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

	tapinfo(w, host, entry, logf)

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
	// Fresh tapinfo first: the keyboard that step 6 raised relayouts the
	// canvas, so the Entry is NOT where the pre-step-6 line said (#143 item
	// 3 — fixed-coordinate injection misses). Give the relayout a beat to
	// settle, then re-aim.
	step(1500 * time.Millisecond)
	tapinfo(w, host, entry, logf)
	logf("7 focus: now TAP the bellui Entry below")
	back := waitFor(25*time.Second, func() bool {
		got := false
		bellui.DoAndWait(func() { got = !host.NativeViewFocused() && w.Canvas().Focused() != nil })
		return got
	})
	logf("7 focus bellui<-tap: %s — web view resigned when the Entry took focus", pf(back))

	// --- focus handback, seam-level ------------------------------------------
	// The same transition step 7 verified by tap, driven programmatically
	// through the FocusNativeView/Canvas.Focus seam (#143 item 3). This is
	// what the per-frame bridge actually arbitrates on, so it needs no
	// finger and no coordinates — it runs even when nobody tapped above.
	bellui.DoAndWait(func() {
		w.Canvas().Unfocus()
		host.FocusNativeView()
	})
	taken := waitFor(5*time.Second, func() bool {
		got := false
		bellui.DoAndWait(func() { got = host.NativeViewFocused() })
		return got
	})
	bellui.DoAndWait(func() { w.Canvas().Focus(entry) })
	handedBack := waitFor(5*time.Second, func() bool {
		got := false
		bellui.DoAndWait(func() { got = !host.NativeViewFocused() && w.Canvas().Focused() == entry })
		return got
	})
	stopTrace()
	logf("7b focus handback (programmatic): %s — take=%v handback=%v", pf(taken && handedBack), taken, handedBack)

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
// focus shadow (raw), the host's bridged state (native), or the canvas focus
// (canvas).
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
