//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"sync"
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

// touchLongPressableObject records the full gesture vocabulary a touchscreen can
// reach, so the tests can tell hold-fire long-press, tap and secondary tap
// apart (#266/#758).
//
// The hold-fire arrives through bellui.CurrentApp().Driver().DoFromGoroutine,
// which in this test binary is the *test* driver (bellui/test's init makes it
// the current app, while the windows here are real glfw ones) — it runs the
// dispatch inline on the hold timer's goroutine instead of hopping onto the
// glfw main thread as the real driver does. So the long-press count is written
// off-main and must be guarded, or the assertions race it.
type touchLongPressableObject struct {
	*canvas.Rectangle
	tappable
	longPressLock   sync.Mutex
	longPressEvents []any
}

var _ event.LongPressable = (*touchLongPressableObject)(nil)

func (l *touchLongPressableObject) LongPressed(e *event.PointEvent) {
	l.longPressLock.Lock()
	defer l.longPressLock.Unlock()
	l.longPressEvents = append(l.longPressEvents, e)
}

// longPresses is the guarded read of the events LongPressed records.
func (l *touchLongPressableObject) longPresses() []any {
	l.longPressLock.Lock()
	defer l.longPressLock.Unlock()
	return append([]any(nil), l.longPressEvents...)
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
	o := &touchLongPressableObject{Rectangle: canvas.NewRectangle(color.White)}
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
		assert.Len(t, o.longPresses(), 1, "LongPressed hold-fired while the finger is still down")

		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 60)
		assert.Nil(t, o.popTapEvent(), "the release of a hold-fired touch never delivers Tapped")
		assert.NotNil(t, o.popSecondaryTapEvent(), "release in place offers the secondary-tap surface")
	})
}

func TestWindow_TouchLongPress_SlopCancel(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &touchLongPressableObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 70, 80) // past the tap slop
	})

	time.Sleep(common.TapSecondaryDelay + 150*time.Millisecond)

	runOnMain(func() {
		assert.Empty(t, o.longPresses(), "movement past the slop cancels the pending hold-fire")
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

// touchVocabularyObject offers the desktop-mouse surfaces alongside the tap
// one, so a touch can be checked against the vocabulary it is documented to
// dispatch: the mobile one, never hover or desktop.Mouseable (#266).
type touchVocabularyObject struct {
	*canvas.Rectangle
	tappable
	hoverable
	mouseable
}

func TestWindow_TouchDispatchesMobileVocabularyOnly(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	o := &touchVocabularyObject{Rectangle: canvas.NewRectangle(color.White)}
	w.SetContent(container.NewStack(prop, o))
	repaintWindow(w)

	runOnMain(func() {
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 51, 61) // inside the tap slop
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 51, 61)

		assert.NotNil(t, o.popTapEvent(), "touch taps the object")
		assert.Nil(t, o.popMouseInEvent(), "touch never hovers in")
		assert.Nil(t, o.popMouseMovedEvent(), "touch never hovers moved")
		assert.Nil(t, o.popMouseOutEvent(), "touch never hovers out")
		assert.Nil(t, o.popMouseEvent(), "touch never dispatches desktop.Mouseable")
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

func TestWindow_TouchScrollContent(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	big := canvas.NewRectangle(color.White)
	big.SetMinSize(geo.NewSize(400, 400))
	tapObj := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
	scroll := container.NewScroll(container.NewStack(big, tapObj))
	w.SetContent(container.NewStack(prop, scroll))
	repaintWindow(w)

	runOnMain(func() {
		// Desktop Scroll is not Draggable (#291): the drag latches no target,
		// becomes a scroll gesture, and the content follows the finger.
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 50, 40)
		assert.Equal(t, float32(20), scroll.Offset.Y, "content follows the finger")
		assert.Equal(t, float32(0), scroll.Offset.X)

		// Once latched, moves below the tap slop still scroll — a slow finger
		// must not stutter.
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 50, 39)
		assert.Equal(t, float32(21), scroll.Offset.Y, "small moves keep scrolling once latched")

		// The settling release (last delta under the momentum threshold) ends
		// the gesture in place, and is not a tap on the content.
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 39)
		assert.Equal(t, float32(21), scroll.Offset.Y)
		assert.Nil(t, tapObj.popTapEvent(), "a scroll gesture's release is not a tap")
	})
}

func TestWindow_TouchScrollDraggablePriority(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	big := canvas.NewRectangle(color.White)
	big.SetMinSize(geo.NewSize(400, 400))
	drag := &draggableTappableObject{Rectangle: canvas.NewRectangle(color.White)}
	scroll := container.NewScroll(container.NewStack(big, drag))
	w.SetContent(container.NewStack(prop, scroll))
	repaintWindow(w)

	runOnMain(func() {
		// A Draggable under the finger (thumb, slider, handle) still wins over
		// content scrolling.
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 50, 40)
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 40)

		assert.Len(t, drag.events, 1, "the Draggable latches the gesture")
		assert.Len(t, drag.endEvents, 1)
		assert.Equal(t, float32(0), scroll.Offset.Y, "no content scroll under a latched drag")
	})
}

func TestWindow_TouchScrollNestedPropagation(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	innerContent := canvas.NewRectangle(color.White)
	innerContent.SetMinSize(geo.NewSize(50, 50))
	inner := container.NewScroll(innerContent)
	big := canvas.NewRectangle(color.White)
	big.SetMinSize(geo.NewSize(400, 400))
	outer := container.NewScroll(container.NewStack(big, inner))
	w.SetContent(container.NewStack(prop, outer))
	repaintWindow(w)

	runOnMain(func() {
		// The touch scroll rides the wheel's nested-propagation chain: the
		// inner scroller's content fits, so it reports not-consumed and the
		// outer one scrolls.
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 60)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 50, 40)
		assert.Equal(t, float32(0), inner.Offset.Y, "fitting inner content does not consume")
		assert.Equal(t, float32(20), outer.Offset.Y, "the outer scroller consumes the propagated scroll")

		w.touchEvent(w.viewport, 1, glfw.TouchMove, 50, 39)
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 39)
	})
}

func TestWindow_TouchScrollFlickMomentum(t *testing.T) {
	w := createWindow("Test")
	prop := canvas.NewRectangle(color.White)
	prop.SetMinSize(geo.NewSize(100, 100))
	big := canvas.NewRectangle(color.White)
	big.SetMinSize(geo.NewSize(400, 400))
	scroll := container.NewScroll(big)
	w.SetContent(container.NewStack(prop, scroll))
	repaintWindow(w)

	runOnMain(func() {
		w.touchEvent(w.viewport, 1, glfw.TouchBegin, 50, 70)
		w.touchEvent(w.viewport, 1, glfw.TouchMove, 50, 40) // a 30px flick
		assert.Equal(t, float32(30), scroll.Offset.Y)
		w.touchEvent(w.viewport, 1, glfw.TouchEnd, 50, 40)
	})

	// The flick's momentum keeps scrolling after the release, decaying per
	// tick. All reads hop to the main goroutine, where the momentum ticks run.
	deadline := time.Now().Add(3 * time.Second)
	continued := false
	for time.Now().Before(deadline) {
		var off float32
		runOnMain(func() { off = scroll.Offset.Y })
		if off > 100 {
			continued = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.True(t, continued, "flick momentum continues scrolling past the release")

	// Touch-to-stop: a new contact halts the in-flight momentum dead.
	var stopAt float32
	runOnMain(func() {
		w.touchEvent(w.viewport, 2, glfw.TouchBegin, 50, 50)
		stopAt = scroll.Offset.Y
	})
	time.Sleep(150 * time.Millisecond)
	runOnMain(func() {
		assert.Equal(t, stopAt, scroll.Offset.Y, "a new contact halts flick momentum")
		w.touchEvent(w.viewport, 2, glfw.TouchEnd, 50, 50)
	})
}
