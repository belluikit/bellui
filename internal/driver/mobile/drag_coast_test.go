//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/widget"
)

// A released drag coasts on (common.DragCoast), and the next touch must stop
// it (#751): before, the coast kept dragging whatever it latched for about
// half a second after the next touch went down, and its late DragEnd could
// end that touch's own drag of the same widget.

// coastRecorder logs its drag events, and coasts like a Scroll. It is only touched on the main
// goroutine: tests read it through onMain.
type coastRecorder struct {
	widget.BaseWidget
	log []string
}

func newCoastRecorder() *coastRecorder {
	r := &coastRecorder{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *coastRecorder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(""))
}

func (r *coastRecorder) Dragged(*event.DragEvent) { r.log = append(r.log, "drag") }
func (r *coastRecorder) DragEnd()                 { r.log = append(r.log, "end") }

// DragMomentum opts the recorder into coasting (event.MomentumDraggable):
// only a Draggable that asks for momentum coasts (#751).
func (r *coastRecorder) DragMomentum() bool { return true }

// onMain runs fn on the test app's main goroutine and waits for it.
func onMain(fn func()) { d.DoFromGoroutine(fn, true) }

// coastWindow is a window filled by a coastRecorder, on a driver that queues
// to the test app's main loop, as the real one does, so a coast's steps run
// there.
func coastWindow(t *testing.T) (*driver, *window, *coastRecorder) {
	t.Helper()
	cd := &driver{queuedFuncs: d.queuedFuncs}
	var w *window
	rec := newCoastRecorder()
	onMain(func() {
		w = cd.CreateWindow("coast").(*window)
		w.SetContent(rec)
		w.Show()
		w.canvas.Resize(geo.NewSize(200, 200))
	})
	t.Cleanup(func() {
		// Stop a fling the test left running, so its steps do not reach the
		// next test, which may be closing a window meanwhile.
		onMain(func() {
			cd.coast.Stop()
			w.Close()
		})
	})
	return cd, w, rec
}

// flick drags across the window fast and lets go, which starts a coast.
func flick(cd *driver, w *window, seq touch.Sequence) {
	s := w.canvas.Scale()
	y := 100 * s
	cd.handleTouch(w, touch.Event{X: 10 * s, Y: y, Sequence: seq, Type: touch.TypeBegin})
	cd.handleTouch(w, touch.Event{X: 60 * s, Y: y, Sequence: seq, Type: touch.TypeMove})
	cd.handleTouch(w, touch.Event{X: 110 * s, Y: y, Sequence: seq, Type: touch.TypeMove})
	cd.handleTouch(w, touch.Event{X: 160 * s, Y: y, Sequence: seq, Type: touch.TypeEnd})
}

// coastFor is longer than any coast from a flick. Its moves arrive within one
// 16ms step, so it seeds the whole 150pt it covered (velocity.go, #751), which
// decays under the end threshold in about 52 steps: 0.85s.
const coastFor = 1500 * time.Millisecond

func Test_driver_TouchStopsDragCoast(t *testing.T) {
	cd, w, rec := coastWindow(t)
	s := w.canvas.Scale()

	onMain(func() { flick(cd, w, 0) })
	time.Sleep(50 * time.Millisecond) // a few coast steps in
	var atTouch []string
	onMain(func() {
		assert.NotContains(t, rec.log, "end", "the flick coasts")
		cd.handleTouch(w, touch.Event{X: 100 * s, Y: 100 * s, Sequence: 1, Type: touch.TypeBegin})
		atTouch = append([]string(nil), rec.log...)
	})
	assert.Equal(t, "end", atTouch[len(atTouch)-1], "the touch ends the coast's drag at once")

	time.Sleep(coastFor)
	onMain(func() {
		assert.Equal(t, atTouch, rec.log, "nothing from the coast arrives after the touch went down")
		cd.handleTouch(w, touch.Event{X: 100 * s, Y: 100 * s, Sequence: 1, Type: touch.TypeEnd})
	})
}

func Test_driver_DragCoastDoesNotEndALaterDrag(t *testing.T) {
	cd, w, rec := coastWindow(t)
	s := w.canvas.Scale()
	y := 100 * s

	onMain(func() { flick(cd, w, 0) })
	time.Sleep(50 * time.Millisecond)
	var started int
	onMain(func() {
		// A second drag of the same widget, held still past the coast.
		cd.handleTouch(w, touch.Event{X: 100 * s, Y: y, Sequence: 1, Type: touch.TypeBegin})
		started = len(rec.log)
		cd.handleTouch(w, touch.Event{X: 140 * s, Y: y, Sequence: 1, Type: touch.TypeMove})
	})

	time.Sleep(coastFor)
	onMain(func() {
		assert.Equal(t, []string{"drag"}, rec.log[started:],
			"the second drag is still going: no coast step or DragEnd landed in it")
		// Held still for coastFor, its release does not coast (#751): it
		// just ends the drag.
		cd.handleTouch(w, touch.Event{X: 140 * s, Y: y, Sequence: 1, Type: touch.TypeEnd})
	})
	time.Sleep(coastFor)
	onMain(func() {
		second := rec.log[started:]
		assert.Equal(t, "end", second[len(second)-1], "its own release ends it")
		assert.NotContains(t, second[:len(second)-1], "end", "once")
	})
}
