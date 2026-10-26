package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"
)

// tapDragRect is tappable and draggable, like a row in a scrolling list
// whose own Scroll latches the drag.
type tapDragRect struct {
	canvas.Rectangle
	taps, drags, ends int
}

func (r *tapDragRect) Tapped(*event.PointEvent) { r.taps++ }
func (r *tapDragRect) Dragged(*event.DragEvent) { r.drags++ }
func (r *tapDragRect) DragEnd()                 { r.ends++ }

// A contact marked with CatchCoast taps nothing on release, but can still
// drag; the mark ends with the contact, so the next touch taps (#792).
func TestCatchCoast_StopsWithoutTapping(t *testing.T) {
	test.NewTempApp(t)
	target := &tapDragRect{}
	target.SetMinSize(geo.NewSquareSize(100))
	c := test.NewCanvas()
	c.SetPadded(false)
	c.SetContent(container.NewStack(target))
	c.Resize(geo.NewSquareSize(100))
	h := &common.TouchHandler{}
	h.InitTouchHandler(&lifetimeFinder{canvas: c})
	tap := func(wid event.Tappable, ev *event.PointEvent) { wid.Tapped(ev) }
	drag := func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }
	release := func(wid event.Draggable, _ *event.DragEvent) { wid.DragEnd() }
	up := func(pos geo.Position, id int) {
		h.TapUp(pos, id, tap, func(event.SecondaryTappable, *event.PointEvent) {},
			func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {}, release)
	}
	at := geo.NewPos(50, 50)

	h.CatchCoast(1)
	h.TapDown(at, 1)
	up(at, 1)
	assert.Zero(t, target.taps, "the touch that caught a coast tapped")

	h.CatchCoast(2)
	h.TapDown(at, 2)
	h.TapMove(geo.NewPos(50, 80), 2, drag)
	up(geo.NewPos(50, 80), 2)
	assert.Positive(t, target.drags, "the touch that caught a coast could not drag on")
	assert.Equal(t, 1, target.ends)

	h.TapDown(at, 3)
	up(at, 3)
	assert.Equal(t, 1, target.taps, "the next touch, which caught nothing, did not tap")
}
