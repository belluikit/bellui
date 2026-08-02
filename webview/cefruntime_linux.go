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

	err = gocef.Initialize(gocef.Config{
		Dir:           dir,
		HelperPath:    filepath.Join(dir, "cef_helper"),
		SandboxPath:   resolveCEFSandbox(dir),
		CachePath:     cache,
		RootCachePath: root,
		// Diagnostic and deployment seams, matching BELLUI_CEF_DIR's shape:
		// extra Chromium switches (a display-less probe run needs
		// --ozone-platform=headless), and Chromium's own log for a browser
		// that silently does nothing (gocef raises severity to VERBOSE).
		Switches: strings.Fields(os.Getenv("BELLUI_CEF_SWITCHES")),
		LogFile:  os.Getenv("BELLUI_CEF_LOG"),
	})
	if err != nil {
		cefRuntime.failed = fmt.Errorf("webview: CEF failed to initialize: %w", err)
		return cefRuntime.failed
	}
	cefRuntime.running = true
	return nil
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
