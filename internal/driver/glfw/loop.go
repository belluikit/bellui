package glfw

import (
	"runtime"
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
	if (!running.Load() && async.IsMainGoroutine()) || drained.Load() {
		f()
		return
	}

	if wait {
		done := common.DonePool.Get()
		defer common.DonePool.Put(done)

		funcQueue.In() <- funcData{f: f, done: done}
		wakeMainLoop()
		<-done
	} else {
		funcQueue.In() <- funcData{f: f}
		wakeMainLoop()
	}
}

func (d *gLDriver) drawSingleFrame() {
	refreshed := false
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.closing {
			continue
		}

		// CheckDirtyAndClear must be checked after visibility,
		// because when a window becomes visible, it could be
		// showing old content without a dirty flag set to true.
		// Do the clear if and only if the window is visible.
		if !w.visible || !w.canvas.CheckDirtyAndClear() {
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
	d.Terminate()
	l := bellui.CurrentApp().Lifecycle().(*app.Lifecycle)
	if f := l.OnStopped(); f != nil {
		l.QueueEvent(f)
	}

	// as we are shutting down make sure we drain the pending funcQueue and close it out.
	for len(funcQueue.Out()) > 0 {
		f := <-funcQueue.Out()
		if f.done != nil {
			f.done <- struct{}{}
		}
	}
	drained.Store(true)
	funcQueue.Close()
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
	w.releaseModal() // re-enable a natively-blocked parent before teardown (#498)
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
	freed := false
	if canvas.EnsureMinSize() {
		w.shouldExpand = true
	}
	freed = canvas.FreeDirtyTextures() > 0

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
	return freed
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
