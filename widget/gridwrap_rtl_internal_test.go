package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

// Under RTL the left arrow advances to the next item and the right arrow
// retreats to the previous one — the mirror of the LTR behaviour, so the
// keystrokes match the mirrored columns.
func TestGridWrap_ArrowKeys_RTL(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	list := createGridWrap(100)
	window := test.NewWindow(list)
	defer window.Close()
	window.Resize(list.MinSize().Max(geo.NewSize(150, 200)))

	canvas := window.Canvas().(test.WindowlessCanvas)
	canvas.FocusNext()
	assert.Equal(t, 0, canvas.Focused().(*GridWrap).currentFocus)

	list.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 1, list.currentFocus, "RTL KeyLeft should advance")
	list.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 2, list.currentFocus)
	list.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 1, list.currentFocus, "RTL KeyRight should retreat")
}

// Under RTL the grid columns flip: the first item in a row sits in the rightmost
// column. Rows are unchanged.
func TestGridWrap_CellMirror_RTL(t *testing.T) {
	test.NewTempApp(t)

	ltrX := gridFirstItemX(t, bellui.DirectionLTR)
	rtlX := gridFirstItemX(t, bellui.DirectionRTL)

	// The first item (column 0) is on the left under LTR and on the right under
	// RTL, so its x grows when the direction flips.
	assert.Greater(t, rtlX, ltrX, "first cell should mirror to the right under RTL")
}

// gridFirstItemX lays out a multi-column GridWrap under dir and returns the x of
// the first visible item (logical id 0).
func gridFirstItemX(t *testing.T, dir bellui.Direction) float32 {
	t.Helper()
	bellui.SetLayoutDirection(dir)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	list := createGridWrap(100)
	window := test.NewWindow(list)
	t.Cleanup(window.Close)
	window.Resize(list.MinSize().Max(geo.NewSize(400, 200)))

	children := list.scroller.Content.(*bellui.Container).Objects
	assert.NotEmpty(t, children)
	return children[0].Position().X
}
