//go:build linux && cgo

package main

// The -live mode is #127's harness: the same composited CEF host, but under
// the REAL driver in a real display session — everything the headless run
// deliberately stubs. What only this can exercise:
//
//   - the GL/WebGPU painter's streaming-raster path under a real window (the
//     headless run uses the software painter's PixelImage path);
//   - the backend's own pump animation (headless pumps gocef.Work itself;
//     under the test driver animations tick exactly once);
//   - X11 vs Wayland sessions, which the offscreen model promises are
//     indistinguishable — run once per backend, forced via BELLUI_PLATFORM;
//   - PET_POPUP compositing: a <select> dropdown arrives as a second CEF
//     surface that host_offscreen_linux.go must blit over the view;
//   - HiDPI end to end: the page's devicePixelRatio must equal the canvas
//     scale the session imposes;
//   - real-driver input, through GLFW rather than the interface seam —
//     synthesized with xdotool where the session allows it (X11; Wayland
//     compositors refuse synthetic input without a portal, reported as SKIP).
//
// Because the content is composited INTO the canvas, Canvas.Capture sees the
// web content — the exact opposite of the hole punch — so every check here
// asserts on bellui's own capture; no OS screenshot tool is needed, which is
// what keeps one probe portable across X11, Wayland and HiDPI sessions.
//
// The probe never calls gocef.Work in live mode: the backend's repeat-forever
// pump animation must drive everything, and that it does is itself a check.
//
//	BELLUI_CEF_DIR=/path/to/payload go run ./hack/cefhostprobe -live
//	BELLUI_PLATFORM=x11 BELLUI_CEF_DIR=... go run ./hack/cefhostprobe -live

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/build"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/webview"
	"github.com/belluikit/gocef"
)

// livePage keeps the probe's unmistakable magenta, adds a large <select> in
// the top-left for the popup-surface check, and records input.
const livePage = `<!doctype html>
<body style="margin:0;background:#ff00aa">
<select id="sel" style="width:220px;height:44px;font-size:20px;margin:8px">
<option>alpha</option><option>bravo</option><option>charlie</option>
<option>delta</option><option>echo</option><option>foxtrot</option>
</select>
<script>
window.downs = [];
window.keys = [];
document.addEventListener('mousedown', function (e) { window.downs.push([e.clientX, e.clientY]); });
document.addEventListener('keydown', function (e) { window.keys.push(e.key); });
</script>
</body>`

func runLive() {
	fmt.Println("== #127: the composited CEF host on a live session ==")
	fmt.Printf("  session   XDG_SESSION_TYPE=%q BELLUI_PLATFORM=%q\n",
		os.Getenv("XDG_SESSION_TYPE"), os.Getenv("BELLUI_PLATFORM"))

	page := "data:text/html;charset=utf-8;base64," +
		base64.StdEncoding.EncodeToString([]byte(livePage))
	host, view := webview.NewHost(page)

	if err := webview.SetCEFProfileInMemory(); err != nil { // hermetic across runs
		fmt.Println("  profile  ", err)
	}
	a := app.NewWithID("com.belluikit.cefhostprobe")
	w := a.NewWindow("cefhostprobe -live (#127)")

	// Smaller than the headless run's 800x600: at scale 2 the window is
	// liveW*2 x liveH*2 pixels and still has to fit the session's display, or
	// the bottom-right overlay lands off screen and the compose check lies.
	const liveW, liveH = 640, 440
	badge := canvas.NewRectangle(overlayColor)
	badge.Resize(geo.NewSize(160, 60))
	badge.Move(geo.NewPos(liveW-200, liveH-100))
	content := container.NewWithoutLayout(host, badge)
	w.SetContent(content)
	w.Resize(geo.NewSize(liveW, liveH))
	host.Resize(geo.NewSize(liveW, liveH))

	go func() {
		defer func() { bellui.Do(a.Quit) }()
		liveChecks(w, view)
	}()

	bellui.ShowAndRun(w)

	fmt.Printf("\n== %d checks: %d PASS, %d FAIL ==\n", pass+fail, pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

// liveChecks runs off the main goroutine; every canvas touch goes through
// DoAndWait, and waiting is plain sleeping — the backend's pump animation on
// the main goroutine is what makes progress happen.
func liveChecks(w bellui.Window, view *webview.WebView) {
	// --- backend pump + load ------------------------------------------------
	fmt.Println("\n[live/pump] the backend's pump animation drives the engine")
	res, err := liveEval(view, "document.readyState", 30*time.Second)
	check(err == nil && strings.Contains(string(res), "complete"),
		fmt.Sprintf("the page loaded with no probe-side pumping (got %s, err %v)", res, err))
	fmt.Printf("  wayland   build.IsWayland=%v (runtime-selected)\n", build.IsWayland)

	// --- composition through the real painter -------------------------------
	fmt.Println("\n[live/compose] web frames through the real painter, seen by Capture")
	img := captureWait(w, 30*time.Second)
	web := countNear(img, webColor, 40)
	over := countNear(img, overlayColor, 40)
	total := imgArea(img)
	writePNG(img, "live_compose.png")
	fmt.Printf("  web-content pixels   %d / %d  (%.1f%%)\n", web, total, pct(web, total))
	fmt.Printf("  bellui overlay above %d / %d  (%.1f%%)\n", over, total, pct(over, total))
	check(total > 0 && web > total/2, "the streaming raster reached the real painter's framebuffer")
	check(over > 0, "bellui draws over the composited content")

	// --- HiDPI ---------------------------------------------------------------
	fmt.Println("\n[live/hidpi] the canvas scale reaches the page")
	var scale float32
	bellui.DoAndWait(func() { scale = w.Canvas().Scale() })
	fmt.Printf("  [dbg] gocef stats: %+v\n", gocef.ReadStats())
	res, err = liveEval(view, "window.devicePixelRatio", 10*time.Second)
	fmt.Printf("  [dbg] gocef stats after eval: %+v\n", gocef.ReadStats())
	check(err == nil && nearFloat(string(res), scale),
		fmt.Sprintf("devicePixelRatio %s matches the canvas scale %.2f", res, scale))

	// --- popup surface -------------------------------------------------------
	fmt.Println("\n[live/popup] a <select> dropdown is a second surface, composited")
	beforeFull := captureNow(w)
	before := cropBelowSelect(beforeFull, scale)
	// Focus the host first, as the driver would on a real click: Chromium
	// dismisses a select popup immediately when its browser is unfocused,
	// which arrives as a lone OnPopupShow(false).
	liveFocusAt(w, geo.NewPos(110, 30))
	liveSeamClick(w, geo.NewPos(110, 30)) // open the select, dead centre
	time.Sleep(1500 * time.Millisecond)
	afterFull := captureNow(w)
	after := cropBelowSelect(afterFull, scale)
	writePNG(beforeFull, "live_popup_before.png")
	writePNG(afterFull, "live_popup_after.png")
	changed := diffCount(before, after, 40)
	area := imgArea(after)
	res, err = liveEval(view, "JSON.stringify(window.downs)", 5*time.Second)
	fmt.Printf("  the click the page saw: %s (err %v)\n", res, err)
	fmt.Printf("  pixels changed under the select: %d / %d (%.1f%%); whole frame: %d\n",
		changed, area, pct(changed, area), diffCount(beforeFull, afterFull, 40))
	check(area > 0 && changed > area/20,
		"the dropdown surface (PET_POPUP) was composited over the view")
	liveSeamClick(w, geo.NewPos(500, 400)) // close it again
	time.Sleep(500 * time.Millisecond)

	// --- real input ----------------------------------------------------------
	fmt.Println("\n[live/input] input through the real OS event path")
	if err := xdotoolInput(w); err != nil {
		fmt.Printf("  SKIP — %v\n", err)
	} else {
		time.Sleep(800 * time.Millisecond)
		// No JSON.stringify: EvalResult already delivers the value as JSON, and
		// stringifying first would wrap it in a string the decoder below refuses.
		res, err = liveEval(view, `[window.downs, window.keys]`, 10*time.Second)
		// The click is positional: X delivers it to the window under the
		// pointer, so it must arrive — through XWayland, GLFW, the driver and
		// the host into CEF. The keystroke goes to the seat's keyboard focus,
		// which a Wayland compositor does not hand to a synthetically
		// activated XWayland window — on a pure X11 session it arrives, under
		// XWayland it is reported rather than failed.
		// The OS click is at page coords ≈(333,222) less the window frame
		// offset; the seam clicks are at (110,30) and (500,400), so a down
		// near (333,222) can only have come through the OS path.
		osClick, osKey := false, false
		var rec []json.RawMessage
		if err == nil && json.Unmarshal(res, &rec) == nil && len(rec) == 2 {
			var downs [][2]float64
			var keys []string
			_ = json.Unmarshal(rec[0], &downs)
			_ = json.Unmarshal(rec[1], &keys)
			for _, d := range downs {
				if d[0] > 320 && d[0] <= 333 && d[1] > 209 && d[1] <= 222 {
					osClick = true
				}
			}
			for _, k := range keys {
				if k == "a" {
					osKey = true
				}
			}
		}
		check(osClick,
			fmt.Sprintf("an OS-synthesized click travelled the whole real input path (got %s, err %v)", res, err))
		if osKey {
			check(true, "the OS-synthesized keystroke arrived too")

			// --- in-page Tab through the OS path (#126) -----------------------
			// Only meaningful when the seat's keyboard focus reaches the window
			// (the same gate as the keystroke above). The page's first tab stop
			// is the <select>.
			fmt.Println("\n[live/tab] plain Tab walks into the page's own tab order")
			_ = exec.Command("xdotool", "key", "Tab").Run()
			time.Sleep(500 * time.Millisecond)
			res, err = liveEval(view, "document.activeElement.id", 10*time.Second)
			check(err == nil && strings.Contains(string(res), "sel"),
				fmt.Sprintf("Tab reached the page's first tab stop (activeElement %s)", res))
		} else {
			fmt.Println("  SKIP — the keystroke needs the seat's keyboard focus, which the compositor withholds from a synthetically activated window; run the Xvfb leg or type on the real session once")
		}
	}

	time.Sleep(300 * time.Millisecond)
}

// xdotoolInput synthesizes a click into the page and a keystroke through the
// OS itself, exercising the whole GLFW input path. Only X11 sessions can do
// this without a portal; everywhere else it reports why it cannot.
func xdotoolInput(w bellui.Window) error {
	if build.IsWayland || os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		// Covers the XWayland case too: xdotool's XTEST events stay inside
		// XWayland — the Wayland compositor owns the seat and routes real
		// input only, so the synthesized click and key would silently go
		// nowhere and the check would blame the toolkit.
		return fmt.Errorf("the seat belongs to a Wayland compositor, which does not route synthetic X input — run the Xvfb leg (xvfb-run … -live) for the OS input path")
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		return fmt.Errorf("xdotool is not installed")
	}
	var x, y int
	var scale float32
	positioned := false
	bellui.DoAndWait(func() {
		if pw, isP := w.(desktop.PositionableWindow); isP {
			x, y = pw.Position()
			positioned = true
		}
		scale = w.Canvas().Scale()
	})
	if !positioned {
		return fmt.Errorf("the window does not report a screen position")
	}
	px := x + int(333*scale) // inside the page, away from every seam click
	py := y + int(222*scale)
	// Activate the window first: a window mapped by a background process does
	// not get X input focus under GNOME's focus-stealing prevention, so the
	// synthesized keystroke would land elsewhere while the click (which is
	// positional) still arrives.
	if out, err := exec.Command("xdotool", "search", "--name", "cefhostprobe",
		"windowactivate", "--sync").CombinedOutput(); err != nil {
		// No window manager (an Xvfb leg): set the X input focus directly.
		if out2, err2 := exec.Command("xdotool", "search", "--name", "cefhostprobe",
			"windowfocus", "--sync").CombinedOutput(); err2 != nil {
			return fmt.Errorf("xdotool activate: %v (%s); focus: %v (%s)", err, out, err2, out2)
		}
	}
	if out, err := exec.Command("xdotool", "mousemove", strconv.Itoa(px), strconv.Itoa(py),
		"click", "1", "sleep", "0.3", "key", "a").CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool: %v (%s)", err, out)
	}
	if out, err := exec.Command("xdotool", "getactivewindow", "getwindowname").CombinedOutput(); err == nil {
		fmt.Printf("  [dbg] X active window after key: %q\n", strings.TrimSpace(string(out)))
	}
	return nil
}

// ---- helpers (live mode never pumps; these only sleep and DoAndWait) -----

// liveEval waits for an eval result the backend's own pump delivers.
func liveEval(view *webview.WebView, js string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)
	for {
		var res json.RawMessage
		var evalErr error
		done := make(chan struct{})
		bellui.DoAndWait(func() {
			view.EvalResult(js, func(r json.RawMessage, err error) {
				res, evalErr = r, err
				close(done)
			})
		})
		select {
		case <-done:
			// Before the engine is ready an eval can fail fast; keep trying
			// until the deadline, since creation is asynchronous by design.
			if evalErr == nil || time.Now().After(deadline) {
				return res, evalErr
			}
			time.Sleep(300 * time.Millisecond)
		case <-time.After(time.Until(deadline)):
			return nil, fmt.Errorf("no eval result within %v", timeout)
		}
	}
}

func captureNow(w bellui.Window) image.Image {
	var img image.Image
	bellui.DoAndWait(func() { img = w.Canvas().Capture() })
	return img
}

// captureWait polls Capture until web content dominates or the timeout ends,
// returning the last capture either way.
func captureWait(w bellui.Window, timeout time.Duration) image.Image {
	deadline := time.Now().Add(timeout)
	var img image.Image
	for time.Now().Before(deadline) {
		img = captureNow(w)
		if total := imgArea(img); total > 0 && countNear(img, webColor, 40) > total/2 {
			return img
		}
		time.Sleep(250 * time.Millisecond)
	}
	return img
}

// cropBelowSelect cuts the region the dropdown list opens into.
func cropBelowSelect(img image.Image, scale float32) image.Image {
	if img == nil {
		return nil
	}
	x0, y0 := int(8*scale), int(56*scale)
	x1, y1 := int(260*scale), int(400*scale)
	b := img.Bounds()
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	out := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			out.Set(x-x0, y-y0, img.At(x, y))
		}
	}
	return out
}

// diffCount reports how many pixels differ beyond tol in any channel.
func diffCount(a, b image.Image, tol int) int {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
		return 0
	}
	n := 0
	bo := a.Bounds()
	for y := bo.Min.Y; y < bo.Max.Y; y++ {
		for x := bo.Min.X; x < bo.Max.X; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			if !near(int(ar>>8), int(br>>8), tol) || !near(int(ag>>8), int(bg>>8), tol) ||
				!near(int(ab>>8), int(bb>>8), tol) {
				n++
			}
		}
	}
	return n
}

func imgArea(img image.Image) int {
	if img == nil {
		return 0
	}
	b := img.Bounds()
	return b.Dx() * b.Dy()
}

// nearFloat compares the page's devicePixelRatio literal with the canvas
// scale, tolerating float formatting.
func nearFloat(js string, scale float32) bool {
	v, err := strconv.ParseFloat(strings.TrimSpace(js), 64)
	if err != nil {
		return false
	}
	d := v - float64(scale)
	return d < 0.05 && d > -0.05
}

// liveFocusAt gives canvas focus to the Focusable at pos, as the real driver
// does before delivering a click.
func liveFocusAt(w bellui.Window, pos geo.Position) {
	bellui.DoAndWait(func() {
		c := w.Canvas()
		o, _, _ := intdriver.FindObjectAtPositionMatching(pos, func(o core.CanvasObject) bool {
			_, ok := o.(event.Focusable)
			return ok
		}, c.Overlays().List(), c.Content())
		if o != nil {
			c.Focus(o.(event.Focusable))
		}
	})
	time.Sleep(250 * time.Millisecond)
}

// liveSeamClick presses and releases at the widget seam, on the main
// goroutine, without pumping — used where exact page coordinates matter. The
// press and release are separate main-goroutine hops with a real gap, like a
// human click.
func liveSeamClick(w bellui.Window, pos geo.Position) {
	var m desktop.Mouseable
	var ev *desktop.MouseEvent
	bellui.DoAndWait(func() {
		c := w.Canvas()
		o, p, _ := intdriver.FindObjectAtPositionMatching(pos, func(o core.CanvasObject) bool {
			_, ok := o.(desktop.Mouseable)
			return ok
		}, c.Overlays().List(), c.Content())
		if o == nil {
			fmt.Println("  no Mouseable at", pos)
			return
		}
		m = o.(desktop.Mouseable)
		ev = &desktop.MouseEvent{
			PointEvent: event.PointEvent{AbsolutePosition: pos, Position: p},
			Button:     desktop.MouseButtonPrimary,
		}
		m.MouseDown(ev)
	})
	if m == nil {
		return
	}
	time.Sleep(150 * time.Millisecond)
	bellui.DoAndWait(func() { m.MouseUp(ev) })
}
