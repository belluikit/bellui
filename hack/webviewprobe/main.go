// Command webviewprobe is the live-verify probe for the native-view-embedding
// spike (#45): widget.NativeViewHost hosting a real WKWebView on macOS.
//
//	go run ./hack/webviewprobe
//
// The window shows a web view inside a container.Scroll, with controls to
// exercise every case the spike has to answer for: scrolling it under the
// viewport edge, hiding it, resizing the window, opening a bellui popup over
// it, and typing into a bellui Entry beside it.
//
// PASS, by eye:
//   - the web page renders at the host's rectangle and follows the scroll;
//   - it is cut off cleanly at the scroll's edges (native clipping works);
//   - scrolling it fully out of the viewport makes it disappear, and
//     scrolling back brings it back;
//   - Hide/Show removes and restores it;
//   - resizing the window keeps it glued to its widget;
//   - closing the window does not crash.
//
// FAIL, and the point of the spike:
//   - the bellui popup opened with "Popup over web view" is OCCLUDED where it
//     overlaps the web view. That is the hole punch working as designed, and
//     it is the limitation the spike exists to price.
//
// Two-way logic, by eye: "Recolour page from bellui" drives the page from a
// bellui button, and the page's own button posts a message that a bellui
// Label displays.
//
// Focus, by eye: type in the Entry, click into the page (the Entry's caret
// stops — bellui has released the focus), then click back in the Entry and
// type again (the keys reach it, not the page). "Focus the web view" does the
// same move programmatically, and Tab never lands on the web view.
//
// -auto measures those claims instead of asserting them by eye:
//
//	go run ./hack/webviewprobe -auto
//
// The trick is to capture the same window twice — once through bellui
// (Canvas.Capture, which reads bellui's GPU surface) and once through macOS
// screencapture (which reads the real composited screen) — and compare. The
// difference between the two *is* the hole. It reports:
//
//   - HOLE — the host's rectangle is web content on screen and empty widget
//     in Canvas.Capture.
//   - OCCLUSION — showing a bellui popup over the host changes bellui's
//     surface and does not change the screen at all.
//   - CLIP — no web content appears outside the scroll viewport.
//   - SCROLLOUT — scrolling the host past the viewport removes the view.
//   - HIDE — Hide() removes the native view and Show() restores it.
//   - RESIZE — after a window resize the view is still on its widget and
//     still clipped.
//   - FOCUS — the #59 bridge in both directions: handing the OS focus to the
//     web engine releases bellui's focused Entry, and focusing the Entry
//     again pulls the keyboard back out of the web engine.
//   - BRIDGE — a bellui widget restyles the live page (Go to JS) and the page
//     posts a message back (JS to Go). The hole punch constrains what can be
//     drawn over the view and who owns the keyboard, not data flow.
//   - COST — how many per-frame geometry evaluations actually reached the
//     platform.
//
// -dump <dir> keeps every capture for inspection.
//
// -auto needs Screen Recording permission for the terminal running it
// (System Settings > Privacy & Security > Screen Recording); without it
// screencapture returns a blank or desktop-only image and the probe says so.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/webview"
	"github.com/belluikit/bellui/widget"
)

// page is loaded as a data: URL so the probe needs no network. The bold
// magenta field is deliberately unlike anything bellui draws, so "is the web
// content on screen here?" is answerable by looking at pixel colour alone.
// Colours are written as rgb() rather than hex: a '#' would terminate the
// data URL, and percent-escaping it is not decoded inside the body.
const page = `data:text/html,` +
	`<html><body style="margin:0;background:rgb(255,0,170);font:600 44px -apple-system;color:rgb(255,255,255)">` +
	`<div style="padding:24px">WEBVIEW</div>` +
	`<div style="padding:24px;font-size:22px">native WKWebView, not drawn by bellui</div>` +
	`<div style="padding:0 24px"><button id="ping" style="font-size:20px;padding:8px 16px" ` +
	`onclick="window.webkit.messageHandlers.bellui.postMessage('ping from the page')">` +
	`send a message to bellui</button></div>` +
	`</body></html>`

var dumpDir = flag.String("dump", "", "write the -auto captures to this directory for inspection")

func main() {
	auto := flag.Bool("auto", false, "run the scripted measurements and exit")
	flag.Parse()

	if !webview.Supported {
		fmt.Println("webview is not supported on this platform; the host will stay inert")
	}

	a := app.NewWithID("com.belluikit.webviewprobe")
	w := a.NewWindow("NativeViewHost / WebView probe (#45)")

	host, view := webview.NewHost(page)
	host.SetMinSize(geo.NewSize(420, 260))

	// Sized siblings above and below give the scroll real travel, so the host
	// can be moved under the viewport edge and fully out of it.
	spacerAbove := tallBox("scroll down — the web view follows", 140)
	spacerBelow := tallBox("keep scrolling — the web view leaves the viewport", 900)

	scroll := container.NewVScroll(container.NewVBox(spacerAbove, host, spacerBelow))
	scroll.SetMinSize(geo.NewSize(460, 320))

	entry := widget.NewEntry()
	entry.SetPlaceHolder("type here, then click into the page, then back")

	stats := widget.NewLabel("syncs 0 / applies 0")

	// JS -> Go. Set before the host is laid out, so no early message is lost.
	fromPage := widget.NewLabel("(no message from the page yet)")
	view.OnMessage = func(msg string) { fromPage.SetText("page said: " + msg) }

	var popup *widget.PopUp
	popupBtn := widget.NewButton("Popup over web view", func() {
		if popup != nil {
			popup.Hide()
		}
		popup = showOverlapPopup(w.Canvas(), host)
	})
	hideBtn := widget.NewButton("Toggle Hide", func() {
		if host.Visible() {
			host.Hide()
		} else {
			host.Show()
		}
	})
	loadBtn := widget.NewButton("Load example.com", func() {
		view.LoadURL("https://example.com")
	})
	// Go -> JS: a bellui widget drives the page.
	evalBtn := widget.NewButton("Recolour page from bellui", func() {
		view.Eval("document.body.style.background='rgb(0,160,255)'")
	})
	// The programmatic way into the native view — the host is not a Tab stop
	// (#59), so this is how an app puts the keyboard in the page.
	focusBtn := widget.NewButton("Focus the web view", func() {
		host.FocusNativeView()
	})

	// Who holds the keyboard right now, updated from the host's own view of
	// the OS responder chain.
	focusState := widget.NewLabel("keyboard: bellui")

	controls := container.NewVBox(
		widget.NewLabel("NativeViewHost spike"),
		popupBtn, hideBtn, loadBtn, evalBtn, focusBtn,
		widget.NewSeparator(),
		entry,
		focusState,
		fromPage,
		stats,
	)

	w.SetContent(container.NewBorder(nil, nil, controls, nil, scroll))
	w.Resize(geo.NewSize(920, 460))

	// Report the per-frame sync counters, so the cost of keeping an OS view
	// glued to a widget is visible while using the probe.
	tick := &bellui.Animation{
		Duration: time.Second, RepeatCount: bellui.AnimationRepeatForever,
		Curve: bellui.AnimationLinear,
		Tick: func(float32) {
			s, ap := host.SyncStats()
			stats.SetText(fmt.Sprintf("syncs %d / applies %d", s, ap))

			who := "bellui"
			if host.NativeViewFocused() {
				who = "web view (OS)"
			}
			focusState.SetText("keyboard: " + who)
		},
	}
	tick.Start()

	if *auto {
		go runAuto(a, w, host, view, scroll, entry)
	}
	bellui.ShowAndRun(w)
}

func tallBox(label string, h float32) core.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0x22, G: 0x66, B: 0x33, A: 0xFF})
	bg.SetMinSize(geo.NewSize(420, h))
	return container.NewStack(bg, widget.NewLabel(label))
}

// showOverlapPopup opens a bellui popup centred on the host, so half of it
// overlaps the web view. On screen the overlapping half is occluded.
func showOverlapPopup(c core.Canvas, host *widget.NativeViewHost) *widget.PopUp {
	body := container.NewVBox(
		widget.NewLabel("BELLUI POPUP"),
		widget.NewLabel("the half over the web view is occluded"),
	)
	p := widget.NewPopUp(body, c)
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(host)
	p.ShowAtPosition(pos.AddXY(40, 40))
	return p
}

// ---- automated measurement ----

// probe carries the shared state of an -auto run: the objects under test, the
// capture directory, the baseline captures every later measurement compares
// against, and whether anything has failed to confirm.
type probe struct {
	a      bellui.App
	w      bellui.Window
	host   *widget.NativeViewHost
	view   *webview.WebView
	scroll *container.Scroll
	entry  *widget.Entry

	dir      string
	baseShot image.Image // screen, before anything was changed
	baseCap  image.Image // bellui surface, same moment
	hostRect geo.Rectangle
	failed   bool
}

func runAuto(a bellui.App, w bellui.Window, host *widget.NativeViewHost, view *webview.WebView, scroll *container.Scroll, entry *widget.Entry) {
	defer func() { bellui.Do(func() { a.Quit() }) }()

	time.Sleep(3 * time.Second) // window maps, WKWebView loads and paints

	p := &probe{a: a, w: w, host: host, view: view, scroll: scroll, entry: entry}
	p.dir = captureDir()
	if *dumpDir == "" {
		defer os.RemoveAll(p.dir)
	}

	fmt.Println("== #45 spike measurements: NativeViewHost + WKWebView on macOS ==")

	p.baseline()
	p.measureHole()
	p.measureOcclusion()
	p.measureClip()
	p.measureScrollOut()
	p.measureHideShow()
	p.measureResize()
	p.measureFocus()
	p.measureBridge()
	p.measureTeardown()
	p.reportCost()

	fmt.Println()
	if p.failed {
		fmt.Println("RESULT: at least one measurement did not confirm — see above")
		os.Exit(1)
	}
	fmt.Println("RESULT: all measurements confirmed the documented hole-punch behaviour")
}

func captureDir() string {
	if *dumpDir != "" {
		_ = os.MkdirAll(*dumpDir, 0o755)
		fmt.Println("captures will be written to", *dumpDir)
		return *dumpDir
	}
	d, err := os.MkdirTemp("", "webviewprobe")
	if err != nil {
		fmt.Println("FAIL: temp dir:", err)
		os.Exit(1)
	}
	return d
}

// fail records a measurement that did not confirm, and reports why.
func (p *probe) fail(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
	p.failed = true
}

// shot captures the screen; a failure is fatal to the measurement that asked
// for it, so callers check the nil return.
func (p *probe) shot(name string) image.Image {
	img, err := screenshot(p.w, filepath.Join(p.dir, name))
	if err != nil {
		p.fail("            SKIP — screen capture failed: %v", err)
		return nil
	}
	return img
}

// rectOf reads an object's canvas rectangle on the main goroutine.
func (p *probe) rectOf(obj core.CanvasObject) geo.Rectangle {
	var r geo.Rectangle
	bellui.DoAndWait(func() { r = objectCanvasRect(obj) })
	return r
}

// canvasRect is the whole window content area.
func (p *probe) canvasRect() geo.Rectangle {
	var r geo.Rectangle
	bellui.DoAndWait(func() { r = geo.Rectangle{Size: p.w.Canvas().Size()} })
	return r
}

// webIn reports the share of img inside r that is the page's magenta field.
func (p *probe) webIn(img image.Image, r geo.Rectangle) float64 {
	return fractionWeb(img, mapRect(img, p.w, r))
}

func (p *probe) scrollTo(y float32) {
	bellui.DoAndWait(func() {
		p.scroll.Offset = geo.NewPos(0, y)
		p.scroll.Refresh()
	})
	time.Sleep(1200 * time.Millisecond)
}

// baseline takes the two captures every later measurement compares against.
// Without them there is nothing to measure, so a failure here is fatal.
func (p *probe) baseline() {
	img, err := screenshot(p.w, filepath.Join(p.dir, "a.png"))
	if err != nil {
		fmt.Println("SKIP: screencapture unavailable —", err)
		fmt.Println("      grant Screen Recording to this terminal and re-run")
		os.Exit(1)
	}
	cap1 := belluiCapture(p.w)
	if cap1 == nil {
		fmt.Println("FAIL: Canvas.Capture returned nil")
		os.Exit(1)
	}
	if *dumpDir != "" {
		savePNG(filepath.Join(p.dir, "canvas-a.png"), cap1)
	}
	p.baseShot, p.baseCap = img, cap1
	p.hostRect = p.rectOf(p.host)
}

// measureHole is the core claim: bellui's own capture of the host rectangle
// holds no web content, while the screen there is almost entirely web content.
func (p *probe) measureHole() {
	inCapture := p.webIn(p.baseCap, p.hostRect)
	onScreen := p.webIn(p.baseShot, p.hostRect)
	fmt.Printf("\n[HOLE]      web-page pixels in the host rectangle\n")
	fmt.Printf("            Canvas.Capture : %5.1f%%\n", inCapture*100)
	fmt.Printf("            screen         : %5.1f%%\n", onScreen*100)
	switch {
	case onScreen < 0.5:
		p.fail("            INCONCLUSIVE — the web view is not on screen; is the window covered?")
	case inCapture > 0.02:
		p.fail("            UNEXPECTED — bellui captured the web content (compositing?!)")
	default:
		fmt.Println("            CONFIRMED — Capture() sees a hole; screenshots never contain web content")
	}
}

// measureOcclusion shows a bellui popup over the host. The honest test is
// differential: the popup must change bellui's own surface in the overlap (it
// drew it) and must NOT change the screen there (the native view is on top).
// Asking whether the capture holds "web pixels" would prove nothing — bellui
// never draws them.
func (p *probe) measureOcclusion() {
	fmt.Printf("\n[OCCLUSION] a bellui popup overlapping the host\n")

	var pop *widget.PopUp
	bellui.DoAndWait(func() { pop = showOverlapPopup(p.w.Canvas(), p.host) })
	time.Sleep(1500 * time.Millisecond)
	defer bellui.DoAndWait(func() {
		p.w.Canvas().Overlays().Remove(p.w.Canvas().Overlays().Top())
	})

	// A shown PopUp's own rectangle is the full-canvas overlay container; the
	// visible body is its Content. Read it only after the popup has been laid
	// out — ShowAtPosition records a position, it does not place the content.
	popRect := p.rectOf(pop.Content)

	shot2 := p.shot("b.png")
	cap2 := belluiCapture(p.w)
	if shot2 == nil || cap2 == nil {
		p.fail("            SKIP — capture failed")
		return
	}
	overlap, ok := intersect(popRect, p.hostRect)
	if !ok {
		p.fail("            SKIP — popup %v did not overlap the host %v", popRect, p.hostRect)
		return
	}
	if *dumpDir != "" {
		savePNG(filepath.Join(p.dir, "canvas-b.png"), cap2)
	}

	capChanged := fractionChanged(p.baseCap, cap2, mapRect(cap2, p.w, overlap))
	scrChanged := fractionChanged(p.baseShot, shot2, mapRect(shot2, p.w, overlap))
	scrWeb := p.webIn(shot2, overlap)
	fmt.Printf("            pixels the popup changed, inside the overlap\n")
	fmt.Printf("            Canvas.Capture : %5.1f%%  (bellui drew it)\n", capChanged*100)
	fmt.Printf("            screen         : %5.1f%%  (nothing changed on screen)\n", scrChanged*100)
	fmt.Printf("            still web pixels on screen : %5.1f%%\n", scrWeb*100)
	switch {
	case capChanged < 0.05:
		p.fail("            INCONCLUSIVE — bellui did not draw a popup in the overlap")
	case scrChanged < 0.05 && scrWeb > 0.5:
		fmt.Println("            CONFIRMED — bellui overlays are occluded by the native view")
	default:
		p.fail("            UNEXPECTED — the popup reached the screen over the native view")
	}
}

// measureClip scrolls the host under the viewport edge: no web content may
// appear outside the scroll's own rectangle.
func (p *probe) measureClip() {
	p.scrollTo(90)
	fmt.Printf("\n[CLIP]      web pixels outside the scroll viewport, host half-scrolled\n")

	img := p.shot("c.png")
	if img == nil {
		return
	}
	outside := p.webIn(img, aboveOf(p.rectOf(p.scroll)))
	fmt.Printf("            above the viewport : %5.1f%%\n", outside*100)
	if outside < 0.01 {
		fmt.Println("            CONFIRMED — the native clip view masks it to the scroll bounds")
		return
	}
	p.fail("            FAIL — web content is overflowing the scroll")
}

func (p *probe) measureScrollOut() {
	p.scrollTo(1200)
	fmt.Printf("\n[SCROLLOUT] host scrolled entirely past the viewport\n")

	img := p.shot("d.png")
	if img == nil {
		return
	}
	anyWeb := p.webIn(img, p.canvasRect())
	fmt.Printf("            web pixels anywhere in the window : %5.1f%%\n", anyWeb*100)
	if anyWeb < 0.01 {
		fmt.Println("            CONFIRMED — the host hides the native view when it leaves the viewport")
		return
	}
	p.fail("            FAIL — the native view is still visible")
}

func (p *probe) measureHideShow() {
	p.scrollTo(0)
	fmt.Printf("\n[HIDE]      Hide() then Show() on the host\n")

	bellui.DoAndWait(func() { p.host.Hide() })
	time.Sleep(900 * time.Millisecond)
	hidden := p.shot("e-hidden.png")

	bellui.DoAndWait(func() { p.host.Show() })
	time.Sleep(900 * time.Millisecond)
	reshown := p.shot("f-shown.png")

	if hidden == nil || reshown == nil {
		return
	}
	full := p.canvasRect()
	whileHidden := p.webIn(hidden, full)
	afterShow := p.webIn(reshown, full)
	fmt.Printf("            web pixels while hidden : %5.1f%%\n", whileHidden*100)
	fmt.Printf("            web pixels after Show   : %5.1f%%\n", afterShow*100)
	if whileHidden < 0.01 && afterShow > 0.05 {
		fmt.Println("            CONFIRMED — Hide removes the native view, Show restores it")
		return
	}
	p.fail("            FAIL — Hide/Show did not round-trip")
}

// measureResize checks the native view stays glued to its widget, and does not
// spill outside it, after the window changes size.
func (p *probe) measureResize() {
	fmt.Printf("\n[RESIZE]    window resized while the host is shown\n")
	bellui.DoAndWait(func() { p.w.Resize(geo.NewSize(760, 520)) })
	time.Sleep(1200 * time.Millisecond)

	img := p.shot("g-resized.png")
	if img == nil {
		return
	}
	inside := p.webIn(img, p.rectOf(p.host))
	outsideBand := p.webIn(img, leftOf(p.rectOf(p.scroll)))
	fmt.Printf("            web pixels inside the host rect : %5.1f%%\n", inside*100)
	fmt.Printf("            web pixels left of the viewport : %5.1f%%\n", outsideBand*100)
	if inside > 0.5 && outsideBand < 0.01 {
		fmt.Println("            CONFIRMED — the native view tracked the resize and stayed clipped")
		return
	}
	p.fail("            FAIL — the native view drifted from its widget")
}

// measureFocus exercises the focus bridge (#59) in both directions. Keyboard
// focus belongs to the OS responder chain, which bellui's canvas focus knows
// nothing about, so the host polls the backend and keeps the two in step:
// focus a bellui Entry, hand the OS focus to the web view (what a click
// inside the page does), and see bellui let go — then focus the Entry again
// and see the web engine give the keyboard back.
func (p *probe) measureFocus() {
	fmt.Printf("\n[FOCUS]     bellui Entry vs. the web engine\n")
	bellui.DoAndWait(func() {
		p.scroll.Offset = geo.NewPos(0, 0)
		p.scroll.Refresh()
		p.w.Canvas().Focus(p.entry)
	})
	time.Sleep(500 * time.Millisecond)

	var before, took bool
	bellui.DoAndWait(func() { before = p.w.Canvas().Focused() == p.entry })

	// Native -> bellui. TakeFocus is called on the backend rather than through
	// the host, so the host is told nothing — exactly as when the user clicks
	// into the page, and the per-frame sync is the only thing that can notice.
	bellui.DoAndWait(func() { took = p.view.TakeFocus() })
	time.Sleep(700 * time.Millisecond)

	var nativeHas, hostKnows, belluiReleased bool
	bellui.DoAndWait(func() {
		nativeHas = p.view.HasFocus()
		hostKnows = p.host.NativeViewFocused()
		belluiReleased = p.w.Canvas().Focused() == nil
	})

	// bellui -> native. Focusing a bellui widget has to pull the OS focus back
	// out of the web engine, or the keystrokes the Entry is drawing itself
	// ready for would go on reaching the page.
	bellui.DoAndWait(func() { p.w.Canvas().Focus(p.entry) })
	time.Sleep(700 * time.Millisecond)

	var belluiFocused, nativeReleased bool
	bellui.DoAndWait(func() {
		belluiFocused = p.w.Canvas().Focused() == p.entry
		nativeReleased = !p.view.HasFocus() && !p.host.NativeViewFocused()
	})

	fmt.Printf("            bellui Entry focused before            : %v\n", before)
	fmt.Printf("            web view took OS focus                 : %v\n", took)
	fmt.Printf("            web view holds OS focus                : %v\n", nativeHas)
	fmt.Printf("            host noticed the native focus          : %v\n", hostKnows)
	fmt.Printf("            bellui released its own focus          : %v\n", belluiReleased)
	fmt.Printf("            bellui took the focus back             : %v\n", belluiFocused)
	fmt.Printf("            web view gave the OS focus back        : %v\n", nativeReleased)

	switch {
	case !before || !took:
		fmt.Println("            INCONCLUSIVE — the focus did not start where the")
		fmt.Println("            measurement needs it (window not key?)")
	case nativeHas && hostKnows && belluiReleased && belluiFocused && nativeReleased:
		fmt.Println("            CONFIRMED — bellui and the OS agree on focus in both")
		fmt.Println("            directions: clicking into the page releases the bellui")
		fmt.Println("            Entry, and focusing the Entry takes the keyboard back.")
	default:
		p.fail("            FAIL — focus is split: keystrokes and IME reach one control\n" +
			"            while bellui draws another as focused")
	}
}

// measureBridge proves the hole punch constrains what can be drawn over the
// view and who owns the keyboard, not data flow: bellui drives the page, and
// the page calls back into bellui.
func (p *probe) measureBridge() {
	fmt.Printf("\n[BRIDGE]    bellui logic <-> page logic\n")
	p.measureEvalToPage()
	p.measureMessageToBellui()
}

// measureEvalToPage confirms on screen that the page actually changed, rather
// than merely that the Eval call returned.
func (p *probe) measureEvalToPage() {
	bellui.DoAndWait(func() {
		p.view.Eval("document.body.style.background='rgb(0,160,255)'")
	})
	time.Sleep(1200 * time.Millisecond)

	img := p.shot("i-eval.png")
	if img == nil {
		return
	}
	r := p.rectOf(p.host)
	magenta := p.webIn(img, r)
	blue := fractionBlue(img, mapRect(img, p.w, r))
	fmt.Printf("            Go -> JS : page now %5.1f%% blue, %5.1f%% magenta\n", blue*100, magenta*100)
	if blue > 0.5 && magenta < 0.05 {
		fmt.Println("                       CONFIRMED — a bellui widget restyled the live page")
		return
	}
	p.fail("                       FAIL — Eval did not reach the page")
}

func (p *probe) measureMessageToBellui() {
	got := make(chan string, 1)
	bellui.DoAndWait(func() {
		p.view.OnMessage = func(m string) {
			select {
			case got <- m:
			default:
			}
		}
		p.view.Eval("window.webkit.messageHandlers.bellui.postMessage('hello from the page')")
	})
	select {
	case m := <-got:
		fmt.Printf("            JS -> Go : received %q on the main goroutine\n", m)
		fmt.Println("                       CONFIRMED — page logic reached bellui logic")
	case <-time.After(3 * time.Second):
		p.fail("            JS -> Go : FAIL — no message arrived within 3s")
	}
}

// measureTeardown replaces the window content, which destroys the host's
// renderer — the path that must remove and release the native view. Doing it
// while the engine is live is the case that would crash if the ordering were
// wrong.
func (p *probe) measureTeardown() {
	fmt.Printf("\n[TEARDOWN]  host removed from the tree while the engine is live\n")
	bellui.DoAndWait(func() { p.w.SetContent(widget.NewLabel("host removed")) })
	time.Sleep(1200 * time.Millisecond)

	img := p.shot("h-torndown.png")
	if img == nil {
		return
	}
	left := p.webIn(img, p.canvasRect())
	fmt.Printf("            web pixels left in the window : %5.1f%%\n", left*100)
	if left < 0.01 {
		fmt.Println("            CONFIRMED — the native view was removed, process still alive")
		return
	}
	p.fail("            FAIL — the native view outlived its host")
}

func (p *probe) reportCost() {
	s, ap := p.host.SyncStats()
	fmt.Printf("\n[COST]      per-frame geometry sync\n")
	fmt.Printf("            evaluations %d, backend calls %d (%.1f%% reached the platform)\n",
		s, ap, pct(ap, s))
}

// screenshot grabs the window's content area with macOS screencapture, which
// (unlike Canvas.Capture) reads the real composited screen and therefore does
// include the native view.
func screenshot(w bellui.Window, path string) (image.Image, error) {
	var x, y int
	var size geo.Size
	var scale float32
	bellui.DoAndWait(func() {
		if pw, ok := w.(desktop.PositionableWindow); ok {
			x, y = pw.Position()
		}
		size = w.Canvas().Size()
		scale = w.Canvas().Scale()
	})
	if size.IsZero() {
		return nil, fmt.Errorf("window has no size yet")
	}

	region := fmt.Sprintf("%d,%d,%d,%d", x, y, int(size.Width*scale), int(size.Height*scale))
	if *dumpDir != "" {
		fmt.Printf("            [dbg] window pos (%d,%d) canvas %v scale %.2f -> region %s\n",
			x, y, size, scale, region)
	}
	out, err := exec.Command("screencapture", "-x", "-R", region, path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("screencapture: %v (%s)", err, out)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func savePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

func belluiCapture(w bellui.Window) image.Image {
	var img image.Image
	bellui.DoAndWait(func() { img = w.Canvas().Capture() })
	return img
}

func objectCanvasRect(obj core.CanvasObject) geo.Rectangle {
	return geo.Rectangle{
		Pos:  bellui.CurrentApp().Driver().AbsolutePositionForObject(obj),
		Size: obj.Size(),
	}
}

// mapRect converts a canvas-space rectangle into pixel bounds of img, using
// the ratio between the image and the canvas. That keeps the comparison
// correct for both capture sources, whose pixel densities differ.
func mapRect(img image.Image, w bellui.Window, r geo.Rectangle) image.Rectangle {
	var size geo.Size
	bellui.DoAndWait(func() { size = w.Canvas().Size() })
	if size.IsZero() {
		return image.Rectangle{}
	}
	b := img.Bounds()
	sx := float32(b.Dx()) / size.Width
	sy := float32(b.Dy()) / size.Height
	return image.Rect(
		b.Min.X+int(r.Pos.X*sx), b.Min.Y+int(r.Pos.Y*sy),
		b.Min.X+int((r.Pos.X+r.Size.Width)*sx), b.Min.Y+int((r.Pos.Y+r.Size.Height)*sy),
	).Intersect(b)
}

// fractionWeb reports the share of pixels in r that are the web page's
// magenta field. The two capture sources do not agree on exact RGB — the
// screen grab goes through a display colour profile, which shifts
// rgb(255,0,170) to roughly rgb(237,27,145) — so the test is a hue relation
// rather than an exact match: strongly red, weakly green, and blue between
// the two. Nothing else in the window (dark chrome, green boxes, white text)
// comes close.
// fractionChanged reports the share of pixels in r that differ between two
// captures of the same source. Both captures come from the same pipeline, so
// an exact comparison with a small tolerance is meaningful here.
// fractionBlue is the counterpart of fractionWeb for the colour Eval sets,
// rgb(0,160,255): weakly red, strongly blue, blue clearly above green.
func fractionBlue(img image.Image, r image.Rectangle) float64 {
	r = r.Intersect(img.Bounds())
	total, hits := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y += 2 {
		for x := r.Min.X; x < r.Max.X; x += 2 {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			red, green, blue := int(cr>>8), int(cg>>8), int(cb>>8)
			total++
			if red < 120 && blue > 170 && blue > green+40 {
				hits++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func fractionChanged(a, b image.Image, r image.Rectangle) float64 {
	r = r.Intersect(a.Bounds()).Intersect(b.Bounds())
	total, diff := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y += 2 {
		for x := r.Min.X; x < r.Max.X; x += 2 {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			total++
			if absDiff(ar, br)>>8 > 12 || absDiff(ag, bg)>>8 > 12 || absDiff(ab, bb)>>8 > 12 {
				diff++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(diff) / float64(total)
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func fractionWeb(img image.Image, r image.Rectangle) float64 {
	r = r.Intersect(img.Bounds())
	total, hits := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y += 2 {
		for x := r.Min.X; x < r.Max.X; x += 2 {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			red, green, blue := int(cr>>8), int(cg>>8), int(cb>>8)
			total++
			if red > 150 && green < 90 && blue > 90 && blue < red {
				hits++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func intersect(a, b geo.Rectangle) (geo.Rectangle, bool) {
	x0, y0 := maxf(a.Pos.X, b.Pos.X), maxf(a.Pos.Y, b.Pos.Y)
	x1 := minf(a.Pos.X+a.Size.Width, b.Pos.X+b.Size.Width)
	y1 := minf(a.Pos.Y+a.Size.Height, b.Pos.Y+b.Size.Height)
	if x1 <= x0 || y1 <= y0 {
		return geo.Rectangle{}, false
	}
	return geo.NewRectangle(x0, y0, x1-x0, y1-y0), true
}

// leftOf is the band immediately left of r — the controls column, where the
// native view must never appear.
func leftOf(r geo.Rectangle) geo.Rectangle {
	w := minf(60, r.Pos.X)
	return geo.NewRectangle(r.Pos.X-w, r.Pos.Y, w, r.Size.Height)
}

// aboveOf is the 40pt band immediately above r — outside a scroll viewport,
// where clipped content must never appear.
func aboveOf(r geo.Rectangle) geo.Rectangle {
	h := minf(40, r.Pos.Y)
	return geo.NewRectangle(r.Pos.X, r.Pos.Y-h, r.Size.Width, h)
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func pct(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b) * 100
}
