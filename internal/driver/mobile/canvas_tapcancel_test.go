//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// TapCancel (#266) is the system-cancel path the glfw driver feeds from
// glfw.TouchCancel — a palm rejection or an edge gesture takes the contact
// over. It lives in the shared handler, and the glfw tests that drive it are
// tagged out of every `-tags ci` leg, so the behaviour is locked here, on the
// canvas that embeds the same handler.

type cancelRecorder struct {
	widget.BaseWidget
	tapped, dragged, dragEnded    int
	touchDown, touchUp, cancelled int
	longPressed                   int
}

var (
	_ event.Tappable      = (*cancelRecorder)(nil)
	_ event.Draggable     = (*cancelRecorder)(nil)
	_ event.LongPressable = (*cancelRecorder)(nil)
	_ mobile.Touchable    = (*cancelRecorder)(nil)
)

func newCancelRecorder() *cancelRecorder {
	r := &cancelRecorder{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *cancelRecorder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel("cancel me"))
}

func (r *cancelRecorder) Tapped(*event.PointEvent)      { r.tapped++ }
func (r *cancelRecorder) LongPressed(*event.PointEvent) { r.longPressed++ }
func (r *cancelRecorder) Dragged(*event.DragEvent)      { r.dragged++ }
func (r *cancelRecorder) DragEnd()                      { r.dragEnded++ }

func (r *cancelRecorder) TouchDown(*mobile.TouchEvent)   { r.touchDown++ }
func (r *cancelRecorder) TouchUp(*mobile.TouchEvent)     { r.touchUp++ }
func (r *cancelRecorder) TouchCancel(*mobile.TouchEvent) { r.cancelled++ }

func newCancelCanvas(r *cancelRecorder) *canvas {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(r)
	c.Resize(geo.NewSize(80, 40))
	return c
}

func Test_canvas_TapCancelIsNeverATap(t *testing.T) {
	r := newCancelRecorder()
	c := newCancelCanvas(r)

	pos := geo.NewPos(20, 20)
	c.TapDown(pos, 0)
	c.TapCancel(pos, 0)

	assert.Equal(t, 1, r.touchDown, "the press still reached the Touchable")
	assert.Equal(t, 1, r.cancelled, "a cancelled contact reports TouchCancel")
	assert.Zero(t, r.touchUp, "a cancel is not a lift")
	assert.Zero(t, r.tapped, "a cancelled contact never taps")

	waitAndCheck(400, func() {
		assert.Zero(t, r.longPressed, "the cancel dropped the pending hold-fire")
	})
}

func Test_canvas_TapCancelEndsTheDragItOwns(t *testing.T) {
	r := newCancelRecorder()
	c := newCancelCanvas(r)

	c.TapDown(geo.NewPos(20, 20), 0)
	c.TapMove(geo.NewPos(40, 20), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
	assert.Equal(t, 1, r.dragged, "the move past the slop started a drag")

	c.TapCancel(geo.NewPos(40, 20), 0)
	assert.Equal(t, 1, r.dragEnded, "the cancelled contact's drag is ended")
	assert.Zero(t, r.tapped, "ending a drag by cancel is not a tap")
}

func Test_canvas_TapCancelLeavesAnotherContactsDrag(t *testing.T) {
	r := newCancelRecorder()
	c := newCancelCanvas(r)

	// The palm lands first, the finger then drags: the rejection of the palm
	// that follows must not end the drag it never latched.
	c.TapDown(geo.NewPos(21, 21), 0)
	c.TapDown(geo.NewPos(20, 20), 1)
	c.TapMove(geo.NewPos(40, 20), 1, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
	assert.Equal(t, 1, r.dragged, "the move past the slop started a drag")

	c.TapCancel(geo.NewPos(21, 21), 0)
	assert.Zero(t, r.dragEnded, "a cancel of some other contact must not end the drag")

	c.TapCancel(geo.NewPos(40, 20), 1)
	assert.Equal(t, 1, r.dragEnded, "cancelling the dragging contact does end it")
}
