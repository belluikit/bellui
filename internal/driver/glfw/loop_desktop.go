//go:build !wasm

package glfw

import (
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

// glfwInitDone gates wakeMainLoop: glfw.PostEmptyEvent may only be called
// once GLFW is initialised. Work queued before that is picked up by the
// event loop's first drain anyway.
var glfwInitDone atomic.Bool

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
	glfwInitDone.Store(true)
}

func (d *gLDriver) pollEvents() {
	glfw.PollEvents() // This call blocks while window is being resized, which prevents freeDirtyTextures from being called
}

// wakeMainLoop interrupts the event loop's native wait so freshly queued
// main-thread work runs promptly.
func wakeMainLoop() {
	if glfwInitDone.Load() {
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
	const frame = time.Second / 60
	now := time.Now()
	nextFrame, lastTick := now, now
	for {
		// Drain control channels without blocking.
		for {
			select {
			case <-d.done:
				d.shutdownLoop()
				return
			case f := <-funcQueue.Out():
				f.f()
				if f.done != nil {
					f.done <- struct{}{}
				}
			default:
				goto drained
			}
		}
	drained:

		now := time.Now()
		nextTick := nextTickDue(nextFrame, lastTick, d.idle())
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
			glfw.WaitEventsTimeout(wait.Seconds())
		}
	}
}

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
func (d *gLDriver) idle() bool {
	if d.animation.Active() {
		return false
	}
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.viewport == nil {
			continue
		}
		if w.shouldExpand || w.viewport.ShouldClose() || (w.visible && w.canvas.IsDirty()) {
			return false
		}
	}
	return true
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}
