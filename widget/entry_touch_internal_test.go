package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// A touch position is entry-relative, so caret placement must translate it by
// the internal scroll offset — the touch mirror of MouseDown (pre-rebrand tracker #758).
func TestEntry_TouchDown_ScrolledCaretPlacement(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(120, 60))
	e.SetText("a\nb\nc\nd\ne\nf\ng\nh\ni\nj")
	require.NotNil(t, e.scroll, "a multi-line entry has an internal scroll")

	pos := geo.NewPos(10, 10)
	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: pos}})
	unscrolledRow := e.CursorRow

	stride := e.CaretRect(1, 0).Pos.Y - e.CaretRect(0, 0).Pos.Y
	require.Positive(t, stride)
	e.scroll.Offset = geo.NewPos(0, 3*stride)
	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: pos}})

	assert.Equal(t, unscrolledRow+3, e.CursorRow, "touch in a scrolled entry maps through the scroll offset")
}
