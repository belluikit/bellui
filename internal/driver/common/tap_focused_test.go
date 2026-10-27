package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// A tap on the widget that already has the focus calls OnTapFocused, which
// the mobile driver uses to bring back a keyboard the system hid while the
// widget kept the focus (#822, #806). A tap that focuses another widget, and
// a drag over the focused one, do not.
func TestTouchHandler_TapOnTheFocusedWidget(t *testing.T) {
	test.NewTempApp(t)
	focused, other := widget.NewEntry(), widget.NewEntry()
	c := test.NewCanvas()
	c.SetPadded(false)
	c.SetContent(container.NewGridWithRows(2, focused, other))
	c.Resize(geo.NewSize(200, 200))
	c.Focus(focused)

	var calls []event.Focusable
	h := &common.TouchHandler{}
	h.InitTouchHandler(&lifetimeFinder{canvas: c})
	h.OnTapFocused = func(f event.Focusable) { calls = append(calls, f) }
	tap := func(wid event.Tappable, ev *event.PointEvent) { wid.Tapped(ev) }
	drag := func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }
	end := func(wid event.Draggable, _ *event.DragEvent) { wid.DragEnd() }
	up := func(pos geo.Position, id int) {
		h.TapUp(pos, id, tap, func(event.SecondaryTappable, *event.PointEvent) {},
			func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {}, end)
	}
	inFocused, inOther := geo.NewPos(100, 50), geo.NewPos(100, 150)

	h.TapDown(inFocused, 1)
	up(inFocused, 1)
	assert.Equal(t, []event.Focusable{focused}, calls, "a tap on the focused entry")

	calls = nil
	h.TapDown(inFocused, 2)
	h.TapMove(geo.NewPos(160, 50), 2, drag)
	up(geo.NewPos(160, 50), 2)
	assert.Empty(t, calls, "a drag over the focused entry is not a tap")

	h.TapDown(inOther, 3)
	up(inOther, 3)
	assert.Empty(t, calls, "a tap on an entry without the focus")
}
