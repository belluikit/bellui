//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"

	"github.com/belluikit/glfw"
)

// longPressableObject records the full gesture vocabulary a touchscreen can
// reach, so the tests can tell hold-fire long-press, tap and secondary tap
// apart (#266/#758).
type longPressableObject struct {
	*canvas.Rectangle
	tappable
	longPressEvents []any
}

var _ event.LongPressable = (*longPressableObject)(nil)

func (l *longPressableObject) LongPressed(e *event.PointEvent) {
	l.longPressEvents = append(l.longPressEvents, e)
}

func TestWindow_TouchTap(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 60)

		assert.Nil(t, o.popSecondaryTapEvent(), "no secondary tap from a quick touch")
		if e, _ := o.popTapEvent().(*event.PointEvent); assert.NotNil(t, e, "touch tap dispatched") {
			assert.Equal(t, geo.NewPos(50, 60), e.AbsolutePosition)
			assert.Equal(t, geo.NewPos(46, 56), e.Position)
		}
	})
}

func TestWindow_TouchLongPress_HoldFire(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &longPressableObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
	})

	// The hold timer fires at TapSecondaryDelay off the event goroutine and
	// hops back via DoFromGoroutine; sleep outside runOnMain so the loop can
	// service it.
	time.Sleep(common.TapSecondaryDelay + 150*time.Millisecond)

	runOnMain(func() {
		assert.Len(t, o.longPressEvents, 1, "LongPressed hold-fired while the finger is still down")

		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 60)
		assert.Nil(t, o.popTapEvent(), "the release of a hold-fired touch never delivers Tapped")
		assert.NotNil(t, o.popSecondaryTapEvent(), "release in place offers the secondary-tap surface")
	})
}

func TestWindow_TouchLongPress_SlopCancel(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &longPressableObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 70, 80) // past the tap slop
	})

	time.Sleep(common.TapSecondaryDelay + 150*time.Millisecond)

	runOnMain(func() {
		assert.Empty(t, o.longPressEvents, "movement past the slop cancels the pending hold-fire")
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 70, 80)
	})
}

func TestWindow_TouchDrag_EndAndCancel(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &draggableTappableObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		// Drag ended by a normal release: Dragged then DragEnd, and the
		// release is not a tap.
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 60, 70)
		assert.Len(t, o.events, 1, "drag dispatched once past the slop")
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 60, 70)
		assert.Len(t, o.endEvents, 1, "DragEnd on release")
		assert.Nil(t, o.popTapEvent(), "a drag's release is not a tap")

		// Drag ended by a system cancel: DragEnd still delivered, no tap.
		w.touchEvent(w.viewport, 2, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 2, glfw.TouchMove, 60, 70)
		w.touchEvent(w.viewport, 2, glfw.TouchCancel, 60, 70)
		assert.Len(t, o.endEvents, 2, "DragEnd on system cancel")
		assert.Nil(t, o.popTapEvent(), "a cancelled touch is not a tap")
	})
}

func TestWindow_TouchUnknownIDIgnored(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		// A callback set mid-sequence delivers moves and ends for ids that
		// never began (see glfw.SetTouchCallback); they must be ignored.
		w.touchEvent(w.viewport, 7, glfw.TouchMove, 50, 60)
		w.touchEvent(w.viewport, 7, glfw.TouchEnd, 50, 60)
		w.touchEvent(w.viewport, 8, glfw.TouchCancel, 0, 0)

		assert.Nil(t, o.popTapEvent(), "an end without a begin is not a tap")
	})
}
