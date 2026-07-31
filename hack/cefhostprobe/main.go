//go:build linux && cgo

// Command cefhostprobe is the #105 acceptance run: the Linux webview backend —
// CEF offscreen, composited into bellui's canvas — driven end to end through
// the real webview.Host widget rather than through raw CEF as hack/cefprobe
// did.
//
// Unlike every hole-punch backend, Canvas.Capture genuinely sees the web
// content here, so this probe asserts on pixels: content IN the canvas,
// bellui drawing OVER it. It then walks the seams the portable API promises —
// document-start bridge injection, both bridge directions, eval with a real
// exception (the #83 defect Linux does not inherit), the navigation veto, the
// focus path and keyboard input, and teardown.
//
// It runs headless: the test app and the software renderer need no display,
// and CEF is told --ozone-platform=headless when there is none. The CEF pump
// is driven by this probe's own loop — under the test driver the backend's
// pump animation ticks only once — which also keeps every CEF call on this
// locked main thread.
//
// Usage on a machine with a payload (see hack/cefprobe/setup.sh or a
// gocef-libs release):
//
//	BELLUI_CEF_DIR=/path/to/payload go run ./hack/cefhostprobe
//
// With -profile it instead runs one leg of the #125 persistence matrix —
// write / read / read-empty paired invocations proving profile state does
// (or, in-memory, does not) survive into a new process; see the flag docs.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/webview"
	"github.com/belluikit/gocef"
)

const viewW, viewH = 800, 600

var (
	webColor     = color.NRGBA{R: 0xff, G: 0x00, B: 0xaa, A: 0xff}
	overlayColor = color.NRGBA{R: 0, G: 200, B: 80, A: 255}
)

// probePage records, from the page side, everything the probe later reads
// back: whether the shim existed at document start, and the input that
// arrived. The background is the same unmistakable colour wpeprobe and
// cefprobe used, so the three reports compare directly.
const probePage = `<!doctype html>
<body style="margin:0;background:#ff00aa">
<script>
window.shimAtStart = typeof window.bellui;
window.downs = [];
window.keys = [];
document.addEventListener('mousedown', function (e) {
  window.downs.push([e.clientX, e.clientY]);
});
document.addEventListener('keydown', function (e) {
  window.keys.push(e.key);
});
if (window.bellui) {
  window.bellui.handle('greet', function (name) {
    return 'hello ' + name;
  });
  window.bellui.onMessage = function (p) { window.posted = p; };
}
</script>
</body>`

var (
	dump = flag.String("dump", "", "directory to write captured PNGs to")

	// The #125 persistence matrix. Cross-process persistence needs two
	// processes (CEF initializes once per process), so it runs as paired
	// invocations against the same fixed-port origin:
	//
	//	cefhostprobe -profile write      -profile-dir /tmp/p   # then:
	//	cefhostprobe -profile read       -profile-dir /tmp/p   # survived?
	//	cefhostprobe -profile write                            # in-memory, then:
	//	cefhostprobe -profile read-empty                       # discarded?
	profileMode = flag.String("profile", "", "persistence check step: write, read, or read-empty")
	profileDir  = flag.String("profile-dir", "", "profile directory for -profile; empty opts into in-memory")
	profilePort = flag.Int("profile-port", 38913, "fixed local port, so both runs share one origin")
	settle      = flag.Duration("settle", 35*time.Second, "how long the write step pumps before exiting: Chromium commits cookies to the profile on a ~30s cadence and there is no bound flush call yet (#134 — measured: at 2s the cookie is lost, localStorage survives)")

	pass = 0
	fail = 0
)

func main() {
	// CEF's browser UI thread is whichever thread called initialize, and every
	// call after that must come from it (belluikit/bellui#102's lesson).
	runtime.LockOSThread()
	flag.Parse()

	if os.Getenv("BELLUI_CEF_DIR") == "" {
		fmt.Println("BELLUI_CEF_DIR is unset; point it at a payload directory (libcef.so, resources, cef_helper)")
		os.Exit(1)
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Println("  session   headless → --ozone-platform=headless")
		os.Setenv("BELLUI_CEF_SWITCHES", strings.TrimSpace(
			os.Getenv("BELLUI_CEF_SWITCHES")+" --ozone-platform=headless",
		))
	}

	if *profileMode != "" {
		runProfileStep()
		fmt.Printf("\n== %d checks: %d PASS, %d FAIL ==\n", pass+fail, pass, fail)
		if fail > 0 {
			os.Exit(1)
		}
		return
	}

	fmt.Println("== #105: the composited CEF host, end to end ==")

	page := "data:text/html;charset=utf-8;base64," +
		base64.StdEncoding.EncodeToString([]byte(probePage))
	host, view := webview.NewHost(page)

	// The page is a data: URL, so its origin is opaque — trusting exactly that
	// origin is itself a check of the CDP origin plumbing.
	view.EnableBridge(webview.TrustOrigins(webview.OriginOpaque))

	var loads []webview.LoadEvent
	view.OnLoadChanged = func(ev webview.LoadEvent) { loads = append(loads, ev) }
	var messages []json.RawMessage
	view.OnMessage = func(m webview.Message) { messages = append(messages, m.Payload) }
	blocked := ""
	view.OnNavigate = func(req webview.NavigationRequest) bool {
		return blocked == "" || !strings.Contains(req.URL, blocked)
	}

	badge := canvas.NewRectangle(overlayColor)
	badge.Resize(geo.NewSize(160, 60))
	badge.Move(geo.NewPos(viewW-200, viewH-100))

	content := container.NewWithoutLayout(host, badge)
	win := test.NewApp().NewWindow("cefhostprobe")
	win.SetContent(content)
	win.Resize(geo.NewSize(viewW, viewH))
	tc := win.Canvas()
	host.Resize(geo.NewSize(viewW, viewH))
	badge.Move(geo.NewPos(viewW-200, viewH-100)) // NewWithoutLayout keeps it

	// --- frames + composition -------------------------------------------------
	fmt.Println("\n[compose] frames through the host, captured by bellui itself")
	ok := pump(30*time.Second, func() bool {
		img := tc.Capture()
		return countNear(img, webColor, 40) > viewW*viewH/2
	})
	img := tc.Capture()
	writePNG(img, "compose.png")
	web := countNear(img, webColor, 40)
	over := countNear(img, overlayColor, 40)
	total := viewW * viewH
	fmt.Printf("  web-content pixels   %7d / %d  (%.1f%%)\n", web, total, pct(web, total))
	fmt.Printf("  bellui overlay above %7d / %d  (%.1f%%)\n", over, total, pct(over, total))
	check(ok && web > 0, "the web content reached bellui's own Capture")
	check(over > 0, "bellui draws OVER the content — impossible under the hole punch")

	// --- load reporting -------------------------------------------------------
	fmt.Println("\n[load] OnLoadChanged")
	sawLoading, sawComplete := false, false
	for _, ev := range loads {
		if ev.State == webview.LoadStateLoading {
			sawLoading = true
		}
		if ev.State == webview.LoadStateComplete {
			sawComplete = true
		}
	}
	check(sawLoading && sawComplete, "the initial navigation reported loading and complete")

	// --- document-start injection --------------------------------------------
	fmt.Println("\n[shim] injection at document start")
	res, err := evalResult(view, "window.shimAtStart")
	check(err == nil && string(res) == `"object"`,
		fmt.Sprintf("the page's first script line saw window.bellui (got %s, err %v)", res, err))

	// --- bridge, both directions ---------------------------------------------
	fmt.Println("\n[bridge] page → bellui and back")
	view.Eval("window.bellui.post({answer: 42})")
	pump(5*time.Second, func() bool { return len(messages) > 0 })
	check(len(messages) == 1 && string(messages[0]) == `{"answer":42}`,
		fmt.Sprintf("a page post arrived through OnMessage (got %v)", messages))

	var callRes json.RawMessage
	var callErr error
	callDone := false
	view.Call("greet", "linux", func(r json.RawMessage, err error) {
		callRes, callErr, callDone = r, err, true
	})
	pump(5*time.Second, func() bool { return callDone })
	check(callDone && callErr == nil && string(callRes) == `"hello linux"`,
		fmt.Sprintf("Call resolved through the page handler (got %s, err %v)", callRes, callErr))

	view.Post(map[string]any{"ping": true})
	res, err = evalResult(view, "JSON.stringify(window.posted)")
	check(err == nil && string(res) == `"{\"ping\":true}"`,
		fmt.Sprintf("Post reached the page's onMessage (got %s, err %v)", res, err))

	// --- eval with a real exception ------------------------------------------
	fmt.Println("\n[eval] a throwing script reports an error, not null (#83)")
	_, err = evalResult(view, "(function(){ throw new Error('boom') })()")
	check(err != nil && strings.Contains(err.Error(), "boom"),
		fmt.Sprintf("the exception crossed with its message (err %v)", err))

	// --- input ----------------------------------------------------------------
	fmt.Println("\n[input] a click and a keystroke reach the page")
	clickAt := geo.NewPos(120, 80)
	rel := mouseClick(tc, clickAt)
	res, err = evalResult(view, "JSON.stringify(window.downs)")
	want := fmt.Sprintf("[%d,%d]", int(rel.X), int(rel.Y))
	check(err == nil && strings.Contains(string(res), want),
		fmt.Sprintf("the click arrived at its host-relative coordinates %s (got %s)", want, res))

	focusHost(tc, clickAt)
	keyboard(tc, clickAt)
	res, err = evalResult(view, "JSON.stringify(window.keys)")
	check(err == nil && strings.Contains(string(res), "a"),
		fmt.Sprintf("the keystroke arrived (got %s)", res))

	// --- navigation veto ------------------------------------------------------
	fmt.Println("\n[nav] OnNavigate veto")
	blocked = "blocked.invalid"
	before := len(loads)
	view.LoadURL("https://blocked.invalid/nope")
	pump(5*time.Second, func() bool { return len(loads) > before })
	vetoOK := false
	for _, ev := range loads[before:] {
		if ev.State == webview.LoadStateFailed && errors.Is(ev.Err, webview.ErrNavigationCancelled) {
			vetoOK = true
		}
	}
	check(vetoOK, "the refused navigation reported ErrNavigationCancelled")
	check(view.URL() != "https://blocked.invalid/nope", "URL() did not follow the refused navigation")

	// --- teardown -------------------------------------------------------------
	fmt.Println("\n[teardown] Destroy fails pending work and survives")
	pendingFailed := false
	view.Call("greet", "again", func(_ json.RawMessage, err error) {
		pendingFailed = errors.Is(err, webview.ErrNotAttached)
	})
	view.Destroy()
	pump(2*time.Second, nil)
	check(pendingFailed, "the pending Call was failed with ErrNotAttached")

	fmt.Printf("\n== %d checks: %d PASS, %d FAIL ==\n", pass+fail, pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

// runProfileStep is one leg of the #125 persistence matrix: it applies the
// profile knob, loads a page from a fixed-port local origin, and writes or
// reads state depending on -profile. localStorage and a max-age cookie are
// both exercised — the cookie is the login-flow claim #125 was filed for, and
// the two live in different stores (a leveldb area, the cookie SQLite) with
// different commit cadences, so each is its own evidence.
func runProfileStep() {
	fmt.Printf("== #125: profile persistence, step %q ==\n", *profileMode)

	if *profileDir != "" {
		if err := webview.SetCEFProfileDir(*profileDir); err != nil {
			check(false, fmt.Sprintf("SetCEFProfileDir: %v", err))
			return
		}
		fmt.Println("  profile   persistent at", *profileDir)
	} else {
		if err := webview.SetCEFProfileInMemory(); err != nil {
			check(false, fmt.Sprintf("SetCEFProfileInMemory: %v", err))
			return
		}
		fmt.Println("  profile   in-memory")
	}

	// A real HTTP origin, because data: URLs are opaque and opaque origins
	// have no localStorage or cookies at all. The port is fixed so the write
	// and read processes see the same origin.
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *profilePort))
	if err != nil {
		check(false, fmt.Sprintf("listen on the fixed probe port: %v", err))
		return
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><body style="margin:0;background:#ff00aa">probe</body>`)
	}))

	host, view := webview.NewHost(fmt.Sprintf("http://127.0.0.1:%d/", *profilePort))
	loaded := false
	view.OnLoadChanged = func(ev webview.LoadEvent) {
		if ev.State == webview.LoadStateComplete {
			loaded = true
		}
	}

	win := test.NewApp().NewWindow("cefhostprobe-profile")
	win.SetContent(container.NewWithoutLayout(host))
	win.Resize(geo.NewSize(400, 300))
	host.Resize(geo.NewSize(400, 300))

	if !pump(30*time.Second, func() bool { return loaded }) {
		check(false, "the probe page loaded")
		return
	}

	switch *profileMode {
	case "write":
		res, err := evalResult(view, `(function () {
			localStorage.setItem('belluiprobe', '42');
			document.cookie = 'belluiprobe=42; max-age=31536000';
			return localStorage.getItem('belluiprobe');
		})()`)
		check(err == nil && string(res) == `"42"`,
			fmt.Sprintf("the page stored localStorage and a persistent cookie (got %s, err %v)", res, err))
		fmt.Printf("  settling %s so both stores commit to the profile before exit\n", *settle)
		pump(*settle, nil)

	case "read":
		res, err := evalResult(view, `localStorage.getItem('belluiprobe')`)
		check(err == nil && string(res) == `"42"`,
			fmt.Sprintf("localStorage survived into a new process (got %s, err %v)", res, err))
		res, err = evalResult(view, `document.cookie`)
		check(err == nil && strings.Contains(string(res), "belluiprobe=42"),
			fmt.Sprintf("the cookie survived into a new process (got %s, err %v)", res, err))

	case "read-empty":
		res, err := evalResult(view, `localStorage.getItem('belluiprobe')`)
		check(err == nil && string(res) == "null",
			fmt.Sprintf("the in-memory profile kept nothing across processes (got %s, err %v)", res, err))
		res, err = evalResult(view, `document.cookie`)
		check(err == nil && !strings.Contains(string(res), "belluiprobe"),
			fmt.Sprintf("no cookie leaked across in-memory processes (got %s, err %v)", res, err))

	default:
		check(false, fmt.Sprintf("unknown -profile step %q (want write, read or read-empty)", *profileMode))
		return
	}

	view.Destroy()
	pump(2*time.Second, nil)
}

// evalResult runs js and waits for its result, pumping CEF meanwhile.
func evalResult(view *webview.WebView, js string) (json.RawMessage, error) {
	var res json.RawMessage
	var resErr error
	done := false
	view.EvalResult(js, func(r json.RawMessage, err error) {
		res, resErr, done = r, err, true
	})
	pump(5*time.Second, func() bool { return done })
	if !done {
		return nil, errors.New("no eval result within 5s")
	}
	return res, resErr
}

// mouseClick delivers a press-release pair to whatever Mouseable is at pos —
// which is how the desktop driver reaches the composited host. It reports the
// position relative to the object, which is what the page should see.
func mouseClick(c core.Canvas, pos geo.Position) geo.Position {
	o, p, _ := intdriver.FindObjectAtPositionMatching(pos, func(o core.CanvasObject) bool {
		_, ok := o.(desktop.Mouseable)
		return ok
	}, c.Overlays().List(), c.Content())
	if o == nil {
		fmt.Println("  no Mouseable at", pos)
		return pos
	}
	m := o.(desktop.Mouseable)
	ev := &desktop.MouseEvent{
		PointEvent: event.PointEvent{AbsolutePosition: pos, Position: p},
		Button:     desktop.MouseButtonPrimary,
	}
	m.MouseDown(ev)
	pump(300*time.Millisecond, nil)
	m.MouseUp(ev)
	pump(300*time.Millisecond, nil)
	return p
}

// focusHost gives the host canvas focus, as the desktop driver would on click.
func focusHost(c core.Canvas, pos geo.Position) {
	o, _, _ := intdriver.FindObjectAtPositionMatching(pos, func(o core.CanvasObject) bool {
		_, ok := o.(event.Focusable)
		return ok
	}, c.Overlays().List(), c.Content())
	if o == nil {
		fmt.Println("  no Focusable at", pos)
		return
	}
	c.Focus(o.(event.Focusable))
	pump(200*time.Millisecond, nil)
}

// keyboard types the letter a the way the desktop driver would: a raw key
// down, the character, and the release.
func keyboard(c core.Canvas, pos geo.Position) {
	o, _, _ := intdriver.FindObjectAtPositionMatching(pos, func(o core.CanvasObject) bool {
		_, ok := o.(desktop.Keyable)
		return ok
	}, c.Overlays().List(), c.Content())
	if o == nil {
		fmt.Println("  no Keyable at", pos)
		return
	}
	k := o.(desktop.Keyable)
	k.KeyDown(&event.KeyEvent{Name: event.KeyA})
	k.TypedRune('a')
	pump(300*time.Millisecond, nil)
	k.KeyUp(&event.KeyEvent{Name: event.KeyA})
	pump(300*time.Millisecond, nil)
}

// pump drives CEF's message loop from this (locked) thread until cond holds.
func pump(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	lastCheck := time.Time{}
	for time.Now().Before(deadline) {
		gocef.Work()
		if cond != nil && time.Since(lastCheck) > 40*time.Millisecond {
			lastCheck = time.Now()
			if cond() {
				return true
			}
		}
		time.Sleep(3 * time.Millisecond)
	}
	return cond == nil
}

func check(ok bool, what string) {
	if ok {
		pass++
		fmt.Println("  PASS:", what)
	} else {
		fail++
		fmt.Println("  FAIL:", what)
	}
}

func countNear(img image.Image, want color.NRGBA, tol int) int {
	if img == nil {
		return 0
	}
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if near(int(r>>8), int(want.R), tol) && near(int(g>>8), int(want.G), tol) &&
				near(int(bb>>8), int(want.B), tol) {
				n++
			}
		}
	}
	return n
}

func near(a, b, tol int) bool { return a-b <= tol && b-a <= tol }

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func writePNG(img image.Image, name string) {
	if *dump == "" || img == nil {
		return
	}
	if err := os.MkdirAll(*dump, 0o755); err != nil {
		fmt.Println("  dump:", err)
		return
	}
	f, err := os.Create(filepath.Join(*dump, name))
	if err != nil {
		fmt.Println("  dump:", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Println("  dump:", err)
	}
}
