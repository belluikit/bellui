//go:build wasm

package glfw

import (
	"time"

	"github.com/belluikit/bellui"

	"github.com/belluikit/gl-js"
	"github.com/belluikit/glfw-js"
)

func (d *gLDriver) initGLFW() {
	err := glfw.Init(gl.ContextWatcher)
	if err != nil {
		bellui.LogError("failed to initialise GLFW", err)
		return
	}
}

func (d *gLDriver) pollEvents() {
	glfw.PollEvents() // This call blocks while window is being resized, which prevents freeDirtyTextures from being called
}

// wakeMainLoop is a no-op on wasm: the event loop stays on the classic
// 60Hz ticker (glfw-js has no PostEmptyEvent/WaitEventsTimeout).
func wakeMainLoop() {}

// runEventLoop is the wasm main loop: the before pre-rebrand tracker #576 ticker+select model,
// kept as-is because the browser runtime has no native event wait to sit in.
func (d *gLDriver) runEventLoop() {
	eventTick := time.NewTicker(time.Second / 60)
	for {
		select {
		case <-d.done:
			eventTick.Stop()
			d.shutdownLoop()
			return
		case f := <-funcQueue.Out():
			f.f()
			if f.done != nil {
				f.done <- struct{}{}
			}
		case <-eventTick.C:
			d.pollEvents()
			d.processEventTick()
		}
	}
}

// runOnMainAsync posts fn onto the main run loop (funcQueue) without waiting for
// it to complete. It is the wasm event-callback counterpart to runOnMain.
//
// On wasm there is no native event poll: PollEvents is a no-op and every browser
// event is delivered by glfw-js straight from a DOM event listener. Those
// listeners run off the main goroutine — and focus/blur/close run *synchronously*
// inside the JS callback. A driver callback that runs its bellui handler inline
// then takes async.EnsureMain's blocking fallback (bellui.DoAndWait) on the first
// Refresh. For the synchronous listeners that parks the single-threaded wasm JS
// thread inside the callback, so runGL can never drain funcQueue to signal
// completion → deadlock (pre-rebrand tracker #113). Posting without waiting lets the JS callback
// return immediately while the handler still runs on the main goroutine, in
// funcQueue (FIFO) order — matching desktop GLFW, which invokes callbacks
// synchronously on main inside pollEvents.
func runOnMainAsync(fn func()) {
	runOnMainWithWait(fn, false)
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}
