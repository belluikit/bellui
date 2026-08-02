//go:build linux && !android && cgo

package webview

// The process-wide CEF runtime behind the Linux backend.
//
// CEF is once-per-process: one Initialize, one browser UI thread — which must
// be the thread that keeps pumping its message loop — and no second chance
// after Shutdown. bellui's main goroutine is locked to the main OS thread by
// the driver, and every method of this package is documented main-goroutine
// only, so calling Initialize from the first Attach and pumping from a
// repeat-forever bellui.Animation (the widget.NativeViewHost ticker pattern)
// satisfies CEF's thread rule without this file doing anything about threads
// itself. Every CEF callback is delivered inside pump ticks, which is what
// makes the whole backend main-goroutine by construction.
//
// Shutdown is deliberately never called. CEF cannot be re-initialized
// afterwards, so the only safe moment would be process exit — where the OS
// reclaims everything anyway. The pump, by contrast, does stop: it runs only
// while browsers exist, because a repeat-forever animation keeps the canvas
// repainting at display rate and that cost should end with the last view.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/gocef"
)

// cefRuntime is main-goroutine state, like everything else in this package.
var cefRuntime struct {
	running bool
	failed  error // a permanent failure; Initialize is once-per-process

	pump     *bellui.Animation
	browsers int // live browsers, counted from create request to Closed
}

// ensureCEFRuntime locates, loads and initializes CEF the first time a view
// attaches. A missing payload reports [ErrRuntimeMissing] and is retried on
// the next Attach — the payload cannot appear mid-process in the packaged
// layout, but an explicit BELLUI_CEF_DIR can be fixed and retried. A failed
// Initialize is permanent: CEF refuses a second attempt in the same process.
func ensureCEFRuntime() error {
	if cefRuntime.running {
		return nil
	}
	if cefRuntime.failed != nil {
		return cefRuntime.failed
	}

	dir, err := gocef.Locate()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRuntimeMissing, err)
	}
	if err := gocef.Load(dir); err != nil {
		return fmt.Errorf("%w: %v", ErrRuntimeMissing, err)
	}

	// The profile choice (#125): persistent by default under the app's
	// storage root, in-memory as the explicit opt-out — see cefprofile.go.
	// Locked from here on even if Initialize fails, since a failure is
	// permanent for the process and a later "successful" Set would be a lie.
	cache, root := resolveCEFProfile()
	cefProfile.locked = true

	cfg := gocef.Config{
		Dir:           dir,
		HelperPath:    filepath.Join(dir, "cef_helper"),
		SandboxPath:   resolveCEFSandbox(dir),
		CachePath:     cache,
		RootCachePath: root,
		// Diagnostic and deployment seams, matching BELLUI_CEF_DIR's shape:
		// extra Chromium switches (a display-less probe run needs
		// --ozone-platform=headless), and Chromium's own log for a browser
		// that silently does nothing (gocef raises severity to VERBOSE).
		Switches: cefOzonePlatformSwitches(
			strings.Fields(os.Getenv("BELLUI_CEF_SWITCHES")), os.Getenv("DISPLAY"),
		),
		LogFile: os.Getenv("BELLUI_CEF_LOG"),
	}
	if os.Getenv("BELLUI_CEF_EXTERNAL_PUMP") == "1" {
		// Phase A of #120, opt-in: CEF asks for Work between frames
		// (external_message_pump), cutting the ≤16 ms latency floor a
		// fixed-rate pump puts under everything a page schedules. The
		// repeat-forever animation pump keeps running alongside — over-pumping
		// is harmless, and removing it is Phase B (with a soak behind it),
		// tracked in its own issue.
		cfg.PumpScheduler = cefSchedulePump
	}
	err = gocef.Initialize(cfg)
	if err != nil {
		cefRuntime.failed = fmt.Errorf("webview: CEF failed to initialize: %w", err)
		return cefRuntime.failed
	}
	cefRuntime.running = true
	return nil
}

// cefOzonePlatformSwitches pins the CEF process to ozone-x11 whenever an X
// display is reachable and the caller has not chosen a platform — the #139
// decision.
//
// CEF renders offscreen here and opens no visible windows, so its ozone
// platform is free to differ from the session bellui runs on. It matters
// because Chromium's ozone-wayland does not honour the OSR client's screen
// info: the page's devicePixelRatio comes out as min(GetScreenInfo answer,
// wl_output scale) — measured across mutter, weston --scale=2 and XWayland on
// linux_parallels — and --force-device-scale-factor is ignored there too
// (crbug 40604649). Under ozone-x11 the client answer is authoritative, which
// is the documented OSR contract, and upstream CEF itself moves OSR toward
// forced x11 (cef#3953). So:
//
//   - An explicit --ozone-platform anywhere in BELLUI_CEF_SWITCHES wins;
//     this function never second-guesses the operator.
//   - Otherwise, DISPLAY set (a real X server or XWayland — every desktop
//     GNOME/KDE Wayland session has one) pins --ozone-platform=x11. On an X11
//     session this matches what Chromium's auto-selection did anyway.
//   - Otherwise the switches pass through untouched and Chromium picks
//     wayland or headless itself. On that pure-Wayland fallback, HiDPI above
//     the compositor's own output scale is capped by the min() above — a
//     recorded Chromium limitation, not something the backend can lift.
func cefOzonePlatformSwitches(user []string, display string) []string {
	for _, s := range user {
		if strings.HasPrefix(s, "--ozone-platform") {
			return user
		}
	}
	if display == "" {
		return user
	}
	return append(user, "--ozone-platform=x11")
}

// resolveCEFSandbox is the #107 sandbox decision, recorded here rather than
// left to chance. Chromium's Linux sandbox needs the payload's chrome-sandbox
// helper owned by root with the setuid bit — and with the sandbox enabled but
// no usable helper (user namespaces are restricted on recent Debian/Ubuntu),
// Chromium aborts the process rather than reporting an error. So:
//
//   - In a Flatpak the sandbox stays off with no complaint: bubblewrap
//     already provides the isolation — what Chromium-based Flatpaks do — and
//     a Flatpak cannot ship a setuid binary anyway. (A flatpak'd app built
//     without the flatpak tag still lands right: its helper cannot be
//     setuid-root inside the container, so the ownership gate below applies.)
//   - Outside one, the helper is used exactly when it is present and
//     correctly owned — `make install` from the packaged tarball sets that
//     up — and otherwise the sandbox is disabled with a notice, never a
//     guess that could turn a missing file into a crash.
//
// Returns the helper path to enable the sandbox with, or "" to run without.
func resolveCEFSandbox(dir string) string {
	if build.IsFlatpak {
		return ""
	}
	sb := filepath.Join(dir, "chrome-sandbox")
	fi, err := os.Stat(sb)
	if err != nil {
		bellui.LogError("webview: running with the CEF sandbox disabled — the payload has no chrome-sandbox helper", nil)
		return ""
	}
	if !sandboxHelperUsable(fi) {
		bellui.LogError("webview: running with the CEF sandbox disabled — "+sb+
			" must be owned by root with the setuid bit (chown root:root && chmod 4755, as the packaged Makefile's install does)", nil)
		return ""
	}
	return sb
}

// sandboxHelperUsable reports whether the stat result describes a helper the
// zygote host will accept: owned by root, setuid.
func sandboxHelperUsable(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return st.Uid == 0 && fi.Mode()&os.ModeSetuid != 0
}

// cefPump is the external-pump scheduler state (#120, Phase A). CEF's
// contract: a schedule request REPLACES any pending one — delay <= 0 means
// pump reasonably soon, > 0 after that many milliseconds. One timer, reset on
// every request, implements exactly that.
var cefPump struct {
	mu    sync.Mutex
	timer *time.Timer
}

// cefSchedulePump is handed to gocef as Config.PumpScheduler. It fires from
// arbitrary CEF threads, so it only arms a timer; the Work call itself hops
// to the main goroutine via bellui.Do — which also wakes a parked event loop
// — preserving the backend's invariant that every CEF callback arrives inside
// Work() on the main goroutine.
func cefSchedulePump(delayMS int64) {
	d := time.Duration(delayMS) * time.Millisecond
	if d <= 0 {
		d = time.Millisecond
	}
	cefPump.mu.Lock()
	defer cefPump.mu.Unlock()
	if cefPump.timer == nil {
		cefPump.timer = time.AfterFunc(d, func() { bellui.Do(cefPumpOnce) })
		return
	}
	cefPump.timer.Reset(d)
}

// cefPumpOnce runs one scheduled Work on the main goroutine. The runtime is
// never shut down, so a late-firing timer at worst pumps an idle loop.
func cefPumpOnce() {
	if cefRuntime.running {
		gocef.Work()
	}
}

// cefBrowserCreated counts a browser from the moment its creation is
// requested — creation itself only completes inside pump ticks, so the pump
// has to be running before gocef.NewBrowser is called.
func cefBrowserCreated() {
	cefRuntime.browsers++
	if cefRuntime.pump != nil {
		return
	}
	anim := &bellui.Animation{
		Duration:    time.Second, // irrelevant: Tick fires every frame while running
		RepeatCount: bellui.AnimationRepeatForever,
		Curve:       bellui.AnimationLinear,
		Tick:        func(float32) { gocef.Work() },
	}
	cefRuntime.pump = anim
	anim.Start()
}

// cefBrowserClosed is called when CEF reports a browser fully gone (or when
// creating one failed). The pump stops with the last browser; a stop from
// inside its own tick is fine, it just ends after the current frame.
func cefBrowserClosed() {
	cefRuntime.browsers--
	if cefRuntime.browsers > 0 || cefRuntime.pump == nil {
		return
	}
	cefRuntime.pump.Stop()
	cefRuntime.pump = nil
}
