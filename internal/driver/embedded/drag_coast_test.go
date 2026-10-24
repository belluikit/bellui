package embedded_test

import (
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/embedded"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intNoos "github.com/belluikit/bellui/internal/driver/embedded"
	"github.com/belluikit/bellui/widget"
)

// A released drag coasts on, and the next touch must stop it (#751): before,
// the coast kept dragging for about half a second after the next touch went
// down, and its late DragEnd could end that touch's own drag of the widget.

// coastRecorder logs its drag events. Only the driver's loop touches it:
// tests read it through onLoop.
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

// coastDriver runs a driver whose window is filled by a coastRecorder. Its
// event channel is unbuffered, so a send returns once the loop has the event,
// and the loop handles it before it runs anything queued after the send.
func coastDriver(t *testing.T) (bellui.Driver, chan embedded.Event, *coastRecorder) {
	t.Helper()
	events := make(chan embedded.Event)
	size := func() geo.Size { return geo.NewSquareSize(200) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, events, size)
	rec := newCoastRecorder()
	w := d.CreateWindow("coast")
	w.SetContent(rec)
	stopped := make(chan struct{})
	go func() {
		d.Run()
		close(stopped)
	}()
	t.Cleanup(func() {
		close(events)
		<-stopped
	})
	return d, events, rec
}

func flickEvents(events chan embedded.Event) {
	events <- &embedded.TouchDownEvent{Position: geo.NewPos(10, 100)}
	events <- &embedded.TouchMoveEvent{Position: geo.NewPos(60, 100)}
	events <- &embedded.TouchMoveEvent{Position: geo.NewPos(110, 100)}
	events <- &embedded.TouchUpEvent{Position: geo.NewPos(160, 100)}
}

// coastFor is longer than any coast from a flick: 50pt a step decays under
// the end threshold in about 40 steps of 16ms.
const coastFor = time.Second

func TestNoOSDriver_TouchStopsDragCoast(t *testing.T) {
	d, events, rec := coastDriver(t)
	onLoop := func(fn func()) { d.DoFromGoroutine(fn, true) }

	flickEvents(events)
	time.Sleep(50 * time.Millisecond)
	onLoop(func() { assert.NotContains(t, rec.log, "end", "the flick coasts") })
	events <- &embedded.TouchDownEvent{Position: geo.NewPos(100, 100), ID: 1}
	var atTouch []string
	onLoop(func() { atTouch = append([]string(nil), rec.log...) })
	assert.Equal(t, "end", atTouch[len(atTouch)-1], "the touch ends the coast's drag at once")

	time.Sleep(coastFor)
	onLoop(func() {
		assert.Equal(t, atTouch, rec.log, "nothing from the coast arrives after the touch went down")
	})
	events <- &embedded.TouchUpEvent{Position: geo.NewPos(100, 100), ID: 1}
}

func TestNoOSDriver_DragCoastDoesNotEndALaterDrag(t *testing.T) {
	d, events, rec := coastDriver(t)
	onLoop := func(fn func()) { d.DoFromGoroutine(fn, true) }

	flickEvents(events)
	time.Sleep(50 * time.Millisecond)
	events <- &embedded.TouchDownEvent{Position: geo.NewPos(100, 100), ID: 1}
	var started int
	onLoop(func() { started = len(rec.log) })
	events <- &embedded.TouchMoveEvent{Position: geo.NewPos(140, 100), ID: 1}

	time.Sleep(coastFor)
	onLoop(func() {
		assert.Equal(t, []string{"drag"}, rec.log[started:],
			"the second drag is still going: no coast step or DragEnd landed in it")
	})
	events <- &embedded.TouchUpEvent{Position: geo.NewPos(140, 100), ID: 1}
	time.Sleep(coastFor)
	onLoop(func() {
		second := rec.log[started:]
		assert.Equal(t, "end", second[len(second)-1], "its own release ends it")
		assert.NotContains(t, second[:len(second)-1], "end", "once")
	})
}
