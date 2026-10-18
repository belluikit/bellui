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
func (d *gLDriver) runEventLoop() {
	const frame = time.Second / 60
	nextFrame := time.Now()
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
		if !now.Before(nextFrame) {
			d.pollEvents()
			d.processEventTick()
			nextFrame = nextFrameAfter(nextFrame, now, frame)
			continue
		}

		// Sleep inside the native wait until the next frame is due, an
		// input event arrives, or wakeMainLoop posts. The wait is recomputed
		// here and clamped: time advances between the frame-due check above
		// and this call, and GLFW asserts on a negative timeout.
		if wait := time.Until(nextFrame); wait > 0 {
			glfw.WaitEventsTimeout(wait.Seconds())
		}
	}
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}
