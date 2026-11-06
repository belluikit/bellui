//go:build !wasm

package glfw

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"

	"github.com/belluikit/glfw"
)

// platform values returned by forcePlatform to override GLFW's auto-detection.
const (
	platformAuto    = ""
	platformX11     = "x11"
	platformWayland = "wayland"
)

// glfwLive gates wakeMainLoop: glfw.PostEmptyEvent may only be called while
// GLFW is initialised, and panics after glfw.Terminate. Work queued before
// init is picked up by the event loop's first drain, and work queued after
// Terminate by shutdownLoop's drain, so neither needs a wake. glfwMu makes the
// check and the call one step against a concurrent Terminate: a Do from
// another goroutine during shutdown crashed the process (#1021).
var (
	glfwMu   sync.RWMutex
	glfwLive bool
)

// wakePending is set while an empty event posted by wakeMainLoop has not yet
// been seen by the loop, which clears it before each drain of funcQueue. One
// pending wake is enough for all work queued behind it, so further posts skip
// theirs: an event per post let goroutines posting back to back add events
// faster than GLFW's native pump cleared them, and the pump never returned
// (#1023).
var wakePending atomic.Bool

func (d *gLDriver) initGLFW() {
	// On linux the default build compiles both the X11 and Wayland backends and
	// GLFW auto-selects per session; forcePlatform lets BELLUI_PLATFORM and the
	// CSD heuristic (wayland_csd_linux.go) pin the choice before init. On other
	// platforms and forced single-backend builds forcePlatform is platformAuto.
	switch forcePlatform() {
	case platformX11:
		glfw.InitHint(glfw.PlatformHint, int(glfw.PlatformX11))
	case platformWayland:
		glfw.InitHint(glfw.PlatformHint, int(glfw.PlatformWayland))
	}

	// On macOS, stop GLFW from building its own basic menu bar / Services menu
	// during glfw.Init(): bellui owns and constructs the native main menu instead
	// (ensureBelluiMainMenu below), so GLFW's orphaned scaffolding — the macOS-26
	// use-after-free candidate (pre-rebrand tracker #831) — is never created. No-op elsewhere and
	// under -tags no_native_menus.
	disableGLFWMenubar()

	err := glfw.Init()
	if err != nil {
		bellui.LogError("failed to initialise GLFW", err)
		return
	}

	// GLFW no longer builds the main menu bar on macOS, so construct bellui's own
	// (app-named submenu + Window/Services) once, up front, giving every app the
	// standard app menu even before it sets a custom main menu. Idempotent; no-op
	// on other platforms and under -tags no_native_menus.
	ensureBelluiMainMenu()

	// GLFW promoted an unbundled process to a foreground UI app (activation policy
	// Regular) during init, but only when its Cocoa menu bar was enabled — which
	// disableGLFWMenubar just turned off (pre-rebrand tracker #831). That promotion is what lets a
	// terminal-launched (unbundled) app take keyboard focus, so bellui, now the
	// owner of the menu bar, must set the policy itself (pre-rebrand tracker #901). No-op elsewhere and
	// for a bundled .app (already Regular).
	ensureRegularActivationPolicy()

	// Record the backend GLFW actually selected so the runtime IsWayland
	// branches (positioning, scaling, native surface/handle) are correct in the
	// default both-backend build. A forced build already has the right value.
	if glfw.GetPlatform() == glfw.PlatformWayland {
		build.IsWayland = true
	}

	initCursors()
	glfwMu.Lock()
	glfwLive = true
	glfwMu.Unlock()
}

func (d *gLDriver) pollEvents() {
	glfw.PollEvents() // This call blocks while window is being resized, which prevents freeDirtyTextures from being called
}

// wakeMainLoop interrupts the event loop's native wait so freshly queued
// main-thread work runs promptly.
func wakeMainLoop() {
	if !wakePending.CompareAndSwap(false, true) {
		return
	}
	glfwMu.RLock()
	defer glfwMu.RUnlock()
	if glfwLive {
		glfw.PostEmptyEvent()
	}
}

// runEventLoop is the desktop main loop. The main thread spends its idle
// time inside glfw.WaitEventsTimeout — with the native event/runloop pump
// live — rather than parked in a Go select between 60Hz poll ticks. This is
// what keeps the platform's accessibility IPC responsive (pre-rebrand tracker #576): macOS
// delivers AX requests on the main runloop and assistive technologies issue
// dozens of sequential round-trips per element, so serialising them on a
// 16ms tick made VoiceOver time out mid-element ("application is not
// responding") and silently skip widgets. Input events also wake the loop,
// so this cuts event latency generally; queued funcQueue work wakes it via
// glfw.PostEmptyEvent (wakeMainLoop).
//
// While nothing needs a frame (see idle), ticks are idleTick apart instead of
// a frame apart, so an idle app does not wake 60 times a second (#470). Input
// events and wakeMainLoop end the wait early as before, and as soon as
// something needs a frame the next tick is the frame grid's (#427).
func (d *gLDriver) runEventLoop() {
	now := time.Now()
	nextFrame, lastTick := now, now
	for {
		// Clear before the drain, so work queued after it posts a fresh wake.
		wakePending.Store(false)

		// Drain control channels without blocking. Work posted but not yet
		// on funcQueue.Out() is waited for here, briefly, by yielding to the
		// goroutine that moves it: a native wait for it lasts pendingWait, so
		// each round ran one Out() buffer of work, and four goroutines posting
		// back to back queued work faster than that (#1023). drainBudget keeps
		// such a flood from holding off native events.
		var drainEnd time.Time
		for {
			select {
			case <-d.done:
				d.shutdownLoop()
				return
			case f := <-funcQueue.Out():
				f.f()
				pendingFuncs.Add(-1)
				if f.done != nil {
					f.done <- struct{}{}
				}
			default:
				if pendingFuncs.Load() == 0 {
					goto drained
				}
				if drainEnd.IsZero() {
					drainEnd = time.Now().Add(drainBudget)
				} else if time.Now().After(drainEnd) {
					goto drained
				}
				runtime.Gosched()
			}
		}
	drained:

		now := time.Now()
		frame := d.frameInterval(now)
		nextTick := withPaintRetry(nextTickDue(nextFrame, lastTick, d.idle(now)), d.nextPaintRetry(now))
		if !now.Before(nextTick) {
			d.pollEvents()
			d.processEventTick()
			nextFrame = nextFrameAfter(nextFrame, now, frame)
			lastTick = now
			continue
		}

		// Sleep inside the native wait until the next tick is due, an
		// input event arrives, or wakeMainLoop posts. The wait is recomputed
		// here and clamped: time advances between the tick-due check above
		// and this call, and GLFW asserts on a negative timeout.
		if wait := time.Until(nextTick); wait > 0 {
			// Work posted but not yet on funcQueue.Out() has already spent
			// its wake-up, so wait only briefly for it (#981, player#287).
			if pendingFuncs.Load() > 0 {
				wait = min(wait, pendingWait)
			}
			glfw.WaitEventsTimeout(wait.Seconds())
		}
	}
}

// frameRecheck is how often the loop looks again at which monitors the windows
// are on: finding a window's monitor costs GLFW calls, and a window dragged to
// another display can wait half a second for its new pace.
const frameRecheck = 500 * time.Millisecond

// frameInterval is how far apart frames are: one refresh of the fastest
// monitor a visible window is on, so a 120 or 144 Hz display gets every
// frame it can show (#480). It is recomputed every frameRecheck.
func (d *gLDriver) frameInterval(now time.Time) time.Duration {
	if d.frameDur != 0 && now.Sub(d.frameChecked) < frameRecheck {
		return d.frameDur
	}
	d.frameChecked = now
	var rates []int
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.viewport == nil || !w.visible {
			continue
		}
		if m := w.getMonitorForWindow(); m != nil {
			if mode := m.GetVideoMode(); mode != nil {
				rates = append(rates, mode.RefreshRate)
			}
		}
	}
	d.frameDur = frameForRates(rates)
	return d.frameDur
}

// frameForRates returns the frame duration for the fastest of the monitors'
// refresh rates in Hz: 60 Hz when none is known, and kept to 30–240 Hz, since
// a monitor can report 0 or nonsense.
func frameForRates(rates []int) time.Duration {
	best := 0
	for _, r := range rates {
		best = max(best, r)
	}
	if best <= 0 {
		best = 60
	}
	best = min(max(best, 30), 240)
	return time.Second / time.Duration(best)
}

// pendingWait bounds the loop's wait while posted work has not reached
// funcQueue.Out(): the goroutine moving it there runs within this, unless the
// machine is very busy, and the loop then looks again.
const pendingWait = time.Millisecond

// drainBudget bounds how long one drain of funcQueue waits for work still on
// its way to funcQueue.Out(), so a steady stream of posts still lets the loop
// pump native events every round.
const drainBudget = 2 * time.Millisecond

// idleTick is how far apart ticks are while the app is idle. Ticks then only
// do housekeeping (cache expiry, keeping cached renderers alive), whose
// deadlines are tens of seconds away.
const idleTick = time.Second

// nextTickDue returns when the next tick is due: the frame grid's nextFrame,
// or idleTick after the last tick while the app is idle. Leaving idle, a grid
// slot already past is due at once.
func nextTickDue(nextFrame, lastTick time.Time, idle bool) time.Time {
	if idle {
		return lastTick.Add(idleTick)
	}
	return nextFrame
}

// idle reports whether the next tick has nothing to do but housekeeping: no
// animation runs, and no visible window needs painting, resizing to fit its
// content, or closing. It is checked before every wait, so main-thread work
// that dirties a canvas gets its frame on time.
func (d *gLDriver) idle(now time.Time) bool {
	if d.animation.Active() {
		return false
	}
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.viewport == nil {
			continue
		}
		if w.shouldExpand || w.viewport.ShouldClose() || w.needsPaint(now) {
			return false
		}
	}
	return true
}

// needsPaint reports whether the window has changes to paint now: a window
// waiting to retry a frame that could not begin does not, until the retry is
// due (see nextPaintRetry).
func (w *window) needsPaint(now time.Time) bool {
	return w.visible && w.canvas.IsDirty() && !w.waitingToRetryPaint(now)
}

// nextPaintRetry returns the earliest time a window waiting to retry a frame
// is due, or the zero time if none is waiting. A retry already due is not
// waiting: needsPaint then reports the window, and its frame comes on the grid.
func (d *gLDriver) nextPaintRetry(now time.Time) time.Time {
	var next time.Time
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.viewport == nil || !w.visible || w.closing || !w.waitingToRetryPaint(now) {
			continue
		}
		if next.IsZero() || w.paintRetryAt.Before(next) {
			next = w.paintRetryAt
		}
	}
	return next
}

// withPaintRetry brings the next tick forward to a paint retry due before it,
// so an idle app retries on time rather than at its next idle tick.
func withPaintRetry(nextTick, retry time.Time) time.Time {
	if !retry.IsZero() && retry.Before(nextTick) {
		return retry
	}
	return nextTick
}

// Terminate ends GLFW. wakeMainLoop stops posting first, as it would panic
// after it (#1021).
func (d *gLDriver) Terminate() {
	glfwMu.Lock()
	defer glfwMu.Unlock()
	glfwLive = false
	glfw.Terminate()
}
