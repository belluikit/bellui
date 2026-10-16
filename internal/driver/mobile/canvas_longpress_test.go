//go:build !windows || !ci

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Hold-fire long-press (pre-rebrand tracker #758): a stationary touch dispatches LongPressed at
// the long-press threshold while the finger is still down; the release then
// dispatches SecondaryTappable. Moving past the tap slop or lifting early
// cancels the pending fire.

type longPressRecorder struct {
	widget.BaseWidget
	longPressed, tapped, secondary int
}

func newLongPressRecorder() *longPressRecorder {
	r := &longPressRecorder{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *longPressRecorder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel("hold me"))
}

func (r *longPressRecorder) LongPressed(*event.PointEvent)     { r.longPressed++ }
func (r *longPressRecorder) Tapped(*event.PointEvent)          { r.tapped++ }
func (r *longPressRecorder) TappedSecondary(*event.PointEvent) { r.secondary++ }

func lprTapUp(c *canvas, pos geo.Position) {
	c.TapUp(pos, 0, func(wid event.Tappable, ev *event.PointEvent) {
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		wid.TappedSecondary(ev)
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
		wid.DragEnd()
	})
}

func Test_canvas_LongPressHoldFire(t *testing.T) {
	r := newLongPressRecorder()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(r)
	c.Resize(geo.NewSize(80, 40))

	pos := geo.NewPos(20, 20)
	c.TapDown(pos, 0)
	waitAndCheck(400, func() {
		assert.Equal(t, 1, r.longPressed, "long-press fires while the finger is still down")
		assert.Zero(t, r.secondary)
	})

	lprTapUp(c, pos)
	waitAndCheck(100, func() {
		assert.Equal(t, 1, r.longPressed, "release does not re-fire the long-press")
		assert.Equal(t, 1, r.secondary, "release after a hold-fire offers the secondary tap")
		assert.Zero(t, r.tapped)
	})
}

func Test_canvas_LongPressQuickTapDoesNotFire(t *testing.T) {
	r := newLongPressRecorder()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(r)
	c.Resize(geo.NewSize(80, 40))

	pos := geo.NewPos(20, 20)
	c.TapDown(pos, 0)
	lprTapUp(c, pos)

	waitAndCheck(400, func() {
		assert.Zero(t, r.longPressed, "a quick tap never long-presses")
		assert.Equal(t, 1, r.tapped)
	})
}

func Test_canvas_LongPressCancelledByDrag(t *testing.T) {
	r := newLongPressRecorder()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(r)
	c.Resize(geo.NewSize(80, 40))

	c.TapDown(geo.NewPos(20, 20), 0)
	c.TapMove(geo.NewPos(40, 20), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})

	waitAndCheck(400, func() {
		assert.Zero(t, r.longPressed, "moving past the tap slop cancels the hold-fire")
	})
}

func Test_canvas_LongPressEntrySelectsWordThenMenu(t *testing.T) {
	e := widget.NewMultiLineEntry()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(e)
	c.Resize(geo.NewSize(200, 100))
	e.SetText("hello world this is a test\nsecond line of text here")

	pos := geo.NewPos(30, 20)
	c.TapDown(pos, 0)
	waitAndCheck(400, func() {
		assert.Equal(t, "hello", e.SelectedText(), "holding on a word selects it")
	})

	secondary := false
	c.TapUp(pos, 0, func(wid event.Tappable, ev *event.PointEvent) {
		wid.Tapped(ev)
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		secondary = true // the Entry context menu path
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
		wid.DragEnd()
	})

	waitAndCheck(100, func() {
		assert.True(t, secondary, "lifting after the hold offers the context menu")
		assert.Equal(t, "hello", e.SelectedText(), "the selection survives the release")
	})
}
