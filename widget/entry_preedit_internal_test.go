package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// PreeditCursorRectangle reports where an IME candidate window should sit. It
// must track the caret column and subtract the entry's scroll offset (pre-rebrand tracker #291).

func TestEntry_PreeditCursorRectangle_TracksColumn(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(200, 60))
	e.SetText("hello world")

	e.CursorRow, e.CursorColumn = 0, 0
	atStart := e.PreeditCursorRectangle()

	e.CursorRow, e.CursorColumn = 0, 6
	atSix := e.PreeditCursorRectangle()

	assert.Greater(t, atSix.Pos.X, atStart.Pos.X, "the caret rectangle advances with the cursor column")
}

func TestEntry_PreeditCursorRectangle_SubtractsScrollOffset(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(120, 60))
	e.SetText("a\nb\nc\nd\ne\nf\ng\nh")
	e.CursorRow, e.CursorColumn = 4, 0
	require.NotNil(t, e.scroll, "a multi-line entry has an internal scroll")

	e.scroll.Offset = geo.NewPos(0, 0)
	unscrolled := e.PreeditCursorRectangle()

	e.scroll.Offset = geo.NewPos(7, 20)
	scrolled := e.PreeditCursorRectangle()

	assert.InDelta(t, unscrolled.Pos.X-7, scrolled.Pos.X, 0.01, "scroll X is subtracted")
	assert.InDelta(t, unscrolled.Pos.Y-20, scrolled.Pos.Y, 0.01, "scroll Y is subtracted")
}
