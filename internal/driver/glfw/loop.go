package glfw

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/scale"
)

type funcData struct {
	f    func()
	done chan struct{} // Zero allocation signalling channel
}

// channel for queuing functions on the main thread
var (
	funcQueue        = async.NewUnboundedChan[funcData]()
	running, drained atomic.Bool

	// pendingFuncs counts work posted to funcQueue and not yet run. It goes up
	// before the work is queued and the loop is woken, and down once the work
	// has run, so the loop can tell work is on its way even before it reaches
	// funcQueue.Out(). A goroutine moves it there from In(), and when that
	// goroutine runs late, the loop used to wake, find nothing, and sleep a
	// whole idle tick with the work queued (#981, player#287).
	pendingFuncs atomic.Int64

	// postMu makes a post's check of drained and its send to funcQueue one
	// step against shutdownLoop setting drained and closing the queue (#1021).
	postMu sync.RWMutex
)

// Arrange that main.main runs on main thread.
func init() {
	runtime.LockOSThread()
	async.SetMainGoroutine()
}

// force a function f to run on the main thread
func runOnMain(f func()) {
	runOnMainWithWait(f, true)
}

// force a function f to run on the main thread and specify if we should wait for it to return
func runOnMainWithWait(f func(), wait bool) {
	// If we are on main before app run just execute - otherwise add it to the main queue and wait.
	// We also need to run it as-is if the app is in the process of shutting down as the queue will be stopped.
	// The check and the send are one step against shutdownLoop, which closes
	// the queue: a send after that panics (#1021).
	postMu.RLock()
	if (!running.Load() && async.IsMainGoroutine()) || drained.Load() {
		postMu.RUnlock()
		f()
		return
	}

	if wait {
		done := common.DonePool.Get()
		defer common.DonePool.Put(done)

		pendingFuncs.Add(1)
		funcQueue.In() <- funcData{f: f, done: done}
		postMu.RUnlock()
		wakeMainLoop()
		<-done
	} else {
		pendingFuncs.Add(1)
		funcQueue.In() <- funcData{f: f}
		postMu.RUnlock()
		wakeMainLoop()
	}
}

// nextFrameAfter returns when the frame after one due at due should run, the
// due frame having run at now. Frames stay on the 1/60 s grid when the wake-up
// comes late, as a ticker's would. Scheduling from the wake-up time instead put
// frames 1/60 s plus each wake-up's lateness apart: 57.6 fps on macOS (#427).
// After a stall of a frame or more, the grid restarts from now, so missed
// frames are not run back to back.
func nextFrameAfter(due, now time.Time, frame time.Duration) time.Time {
	if next := due.Add(frame); next.After(now) {
		return next
	}
	return now.Add(frame)
}

func (d *gLDriver) drawSingleFrame() {
	refreshed := false
	now := time.Now()
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.closing {
			continue
		}

		// CheckDirtyAndClear must be checked after visibility,
		// because when a window becomes visible, it could be
		// showing old content without a dirty flag set to true.
		// Do the clear if and only if the window is visible.
		// A window waiting to retry a frame that could not begin keeps its
		// dirty flag until the retry is due.
		if !w.visible || w.waitingToRetryPaint(now) || !w.canvas.CheckDirtyAndClear() {
			// Window hidden or not being redrawn, mark canvasForObject
			// cache alive if it hasn't been done recently
			// n.b. we need to make sure threshold is a bit *after*
			// time.Now() - CacheDuration()
			threshold := time.Now().Add(10*time.Second - cache.ValidDuration)
			if w.lastWalkedTime.Before(threshold) {
				w.canvas.WalkTrees(nil, func(node *common.RenderCacheNode, _ geo.Position) {
					// marks canvas for object cache entry alive
					_ = cache.GetCanvasForObject(node.Obj())
					// marks renderer cache entry alive
					if wid, ok := node.Obj().(core.Widget); ok {
						_, _ = cache.CachedRenderer(wid)
					}
				})
				w.lastWalkedTime = time.Now()
			}
			continue
		}

		w.RunWithContext(func() {
			if w.driver.repaintWindow(w) {
				refreshed = true
			}
		})
		w.updateAccessibility()
	}
	cache.Clean(refreshed)
}

func (d *gLDriver) runGL() {
	if !running.CompareAndSwap(false, true) {
		return // Run was called twice.
	}

	d.init()
	if d.trayStart != nil {
		d.trayStart()
	}

	bellui.CurrentApp().Settings().AddListener(func(set bellui.Settings) {
		painter.ClearFontCache()
		cache.ResetThemeCaches()
		app.ApplySettingsWithCallback(set, bellui.CurrentApp(), func(w bellui.Window) {
			c, ok := w.Canvas().(*glCanvas)
			if !ok {
				return
			}
			c.applyThemeOutOfTreeObjects()
			// Cached RepaintBoundary FBOs hold pixels rendered with the old
			// theme; invalidate explicitly — reloadScale also does this but
			// skips windows that are not currently visible.
			if bp, ok := c.Painter().(gpu.BoundaryPainter); ok {
				bp.InvalidateAllBoundaries()
			}
			c.reloadScale()
		})
	})

	if f := bellui.CurrentApp().Lifecycle().(*app.Lifecycle).OnStarted(); f != nil {
		f()
	}

	d.runEventLoop()
}

// shutdownLoop tears the driver down when d.done closes: terminate GLFW,
// fire the stopped callback and drain the pending funcQueue.
func (d *gLDriver) shutdownLoop() {
	profiler.Flush()
	verifier.Flush()
	d.Terminate()
	l := bellui.CurrentApp().Lifecycle().(*app.Lifecycle)
	if f := l.OnStopped(); f != nil {
		l.QueueEvent(f)
	}

	closeFuncQueue()
}

// closeFuncQueue stops posting to funcQueue, closes it and releases
// everything still in it. Work posted from then on runs where it is posted.
// Reading only what had reached Out() missed work still on its way, whose
// waiters then never returned, and a post after the close panicked (#1021).
func closeFuncQueue() {
	postMu.Lock()
	drained.Store(true)
	postMu.Unlock()
	funcQueue.Close()
	for f := range funcQueue.Out() {
		pendingFuncs.Add(-1)
		if f.done != nil {
			f.done <- struct{}{}
		}
	}
}

// processEventTick runs one frame's worth of work: window lifecycle,
// animations and the draw pass. Native events must have been pumped by the
// caller (pollEvents / the event wait).
func (d *gLDriver) processEventTick() {
	for i := 0; i < len(d.windows); i++ {
		w := d.windows[i].(*window)
		if w.viewport == nil {
			continue
		}

		if w.viewport.ShouldClose() {
			d.destroyWindow(w, i)
			i-- // Trailing windows are moved forward one step.
			continue
		}

		expand := w.shouldExpand
		fullScreen := w.fullScreen

		if expand && !fullScreen {
			w.fitContent()
			shouldExpand := w.shouldExpand
			w.shouldExpand = false
			view := w.viewport

			if shouldExpand && runtime.GOOS != "js" {
				view.SetSize(w.shouldWidth, w.shouldHeight)
			}
		}
	}

	d.animation.TickAnimations()
	d.drawSingleFrame()
}

func (d *gLDriver) destroyWindow(w *window, index int) {
	w.visible = false
	w.releaseModal() // re-enable a natively-blocked parent before teardown (pre-rebrand tracker #498)
	w.cleanupAccessibilityForWindow()
	w.viewport.Destroy()
	w.destroy(d)

	if index < len(d.windows)-1 {
		copy(d.windows[index:], d.windows[index+1:])
	}
	d.windows[len(d.windows)-1] = nil
	d.windows = d.windows[:len(d.windows)-1]

	if len(d.windows) == 0 {
		d.Quit()
	}
}

func (d *gLDriver) repaintWindow(w *window) bool {
	canvas := w.canvas
	counter, counts := canvas.Painter().(interface{ FramesBegun() uint64 })
	var before uint64
	if counts {
		before = counter.FramesBegun()
	}
	freed := prepareFrame(w)

	updateGLContext(w)

	var frameStart, paintEnd time.Time
	if profiler.enabled {
		frameStart = time.Now()
	}
	canvas.paint(canvas.Size())
	if profiler.enabled {
		paintEnd = time.Now()
	}

	view := w.viewport
	visible := w.visible

	if view != nil && visible {
		presentWindow(w) // GL SwapBuffers, or WebGPU surface present
	}
	if profiler.enabled {
		now := time.Now()
		var gpu time.Duration
		if g, ok := canvas.Painter().(interface{ GPUFrameNanos() (uint64, bool) }); ok {
			if ns, has := g.GPUFrameNanos(); has {
				gpu = time.Duration(ns)
			}
		}
		profiler.record(paintEnd.Sub(frameStart), now.Sub(frameStart), gpu)
	}

	// mark that we have walked the window and don't
	// need to walk it again to mark caches alive
	w.lastWalkedTime = time.Now()
	w.noteFrameBegan(!counts || counter.FramesBegun() != before, w.lastWalkedTime)
	verifier.afterFrame(w)
	return freed
}

// paintRetry is how long a window whose frame could not begin waits before it
// tries again. Painting is cheap to attempt but not free, and a window can stay
// unpaintable for hours, so the retry does not run at the frame rate. It is
// also how long a window that becomes paintable again can show old content.
const paintRetry = 100 * time.Millisecond

// noteFrameBegan records whether the paint that just ran reached the screen.
// The painter cannot begin a frame while it has no surface to draw on, as for
// a window macOS reports occluded (#631), and the paint then shows nothing.
// The walk has taken the canvas' dirty flag all the same, so without another
// change the window would keep its old content once it can paint again (#864).
// So the canvas is marked dirty again, to be painted after paintRetry. The
// painters already make that next frame a full one (#564, #595).
func (w *window) noteFrameBegan(began bool, now time.Time) {
	if began {
		w.paintRetryAt = time.Time{}
		return
	}
	w.canvas.SetDirty()
	w.paintRetryAt = now.Add(paintRetry)
}

// waitingToRetryPaint reports whether the window's last frame could not begin
// and its retry is not due yet.
func (w *window) waitingToRetryPaint(now time.Time) bool {
	return !w.paintRetryAt.IsZero() && now.Before(w.paintRetryAt)
}

// prepareFrame does what a paint needs done first: the min-size pass, and
// taking the refreshed objects off the queue, which frees their textures and
// marks their RepaintBoundaries and partial-repaint nodes dirty. It reports
// whether any texture was freed.
func prepareFrame(w *window) bool {
	if w.canvas.EnsureMinSize() {
		w.shouldExpand = true
	}
	return w.canvas.FreeDirtyTextures() > 0
}

// refreshWindow requests that the specified window be redrawn
func refreshWindow(w *window) {
	w.canvas.SetDirty()
}

func updateGLContext(w *window) {
	canvas := w.canvas
	size := canvas.Size()

	// w.width and w.height are not correct if we are maximised, so figure from canvas
	winWidth := float32(scale.ToScreenCoordinate(canvas, size.Width)) * canvas.texScale
	winHeight := float32(scale.ToScreenCoordinate(canvas, size.Height)) * canvas.texScale

	canvas.Painter().SetFrameBufferScale(canvas.texScale)
	canvas.Painter().SetOutputSize(int(winWidth), int(winHeight))
}
