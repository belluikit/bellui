package widget_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foldEntry builds a virtualised multi-line Entry with `lines` rows of the form
// "lineNN", rendered in a window so row geometry is populated.
func foldEntry(t *testing.T, lines int) *widget.Entry {
	t.Helper()
	test.NewTempApp(t)
	e := widget.NewMultiLineEntry()

	var sb strings.Builder
	for i := 0; i < lines; i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "line%02d", i)
	}
	e.SetText(sb.String())

	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	return e
}

func TestEntry_HiddenRanges_CollapsesGeometry(t *testing.T) {
	e := foldEntry(t, 20)
	yp0, _ := e.RowPositions()
	require.Len(t, yp0, 20)
	_, stride := e.RowMetrics()
	require.Greater(t, stride, float32(0))

	e.HiddenRanges = []widget.LineRange{{Start: 5, End: 10}} // fold rows 5..9
	e.Refresh()

	yp, h := e.RowPositions()
	require.Len(t, yp, 20, "folded rows remain in the geometry model")

	for r := 5; r < 10; r++ {
		assert.Equal(t, float32(0), h[r], "folded row %d has zero height", r)
	}
	assert.Greater(t, h[4], float32(0), "the row above the fold keeps its height")
	assert.Greater(t, h[10], float32(0), "the row below the fold keeps its height")

	// The fold collapses: every row below it shifts up by the hidden height, and
	// the hidden rows sit at the fold point (== the first visible row after).
	assert.InDelta(t, yp[4]+stride, yp[10], 0.5, "row below the fold abuts the row above it")
	assert.InDelta(t, yp[5], yp[10], 0.5, "hidden rows collapse onto the fold point")
	assert.InDelta(t, yp0[10]-5*stride, yp[10], 0.5, "row 10 shifted up by the 5 folded rows")
}

func TestEntry_HiddenRanges_ScrollExtentShrinks(t *testing.T) {
	e := foldEntry(t, 30)
	yp0, _ := e.RowPositions()
	_, stride := e.RowMetrics()

	e.HiddenRanges = []widget.LineRange{{Start: 10, End: 20}} // fold 10 rows
	e.Refresh()
	yp, _ := e.RowPositions()

	assert.InDelta(t, yp0[29]-10*stride, yp[29], 0.5,
		"the last row's Y (and so the scroll extent) shrinks by the folded height")
}

func TestEntry_HiddenRanges_TextUnaffected(t *testing.T) {
	e := foldEntry(t, 12)
	before := e.Text

	e.HiddenRanges = []widget.LineRange{{Start: 3, End: 8}}
	e.Refresh()

	assert.Equal(t, before, e.Text, "folding is a view concern: Text() is unchanged")
	assert.Contains(t, e.Text, "line05", "a folded line is still in the model")
}

func TestEntry_HiddenRanges_CaretRectZeroForHidden(t *testing.T) {
	e := foldEntry(t, 20)
	e.HiddenRanges = []widget.LineRange{{Start: 5, End: 10}}
	e.Refresh()

	assert.True(t, e.CaretRect(7, 0).IsZero(), "a folded row has no caret rectangle")
	assert.False(t, e.CaretRect(4, 0).IsZero(), "a visible row above the fold has a caret rect")

	below := e.CaretRect(10, 0)
	require.False(t, below.IsZero(), "a visible row below the fold has a caret rect")
	yp, _ := e.RowPositions()
	assert.InDelta(t, yp[10], below.Pos.Y, 0.5, "caret Y matches the collapsed row geometry")
}

func TestEntry_HiddenRanges_CaretNavSkips(t *testing.T) {
	e := foldEntry(t, 20)
	e.HiddenRanges = []widget.LineRange{{Start: 5, End: 10}}
	e.Refresh()

	// Down from the row above the fold lands on the first visible row after it.
	e.CursorRow, e.CursorColumn = 4, 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 10, e.CursorRow, "down-arrow skips the folded block")

	// Up from the row below the fold lands on the last visible row before it.
	e.CursorRow, e.CursorColumn = 10, 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 4, e.CursorRow, "up-arrow skips the folded block")
}

func TestEntry_HiddenRanges_SelectionIncludesHiddenText(t *testing.T) {
	e := foldEntry(t, 12)
	e.HiddenRanges = []widget.LineRange{{Start: 5, End: 10}}
	e.Refresh()

	// A selection spanning the fold still includes the hidden text in the model.
	e.Select(4, 0, 11, 0)
	sel := e.SelectedText()
	assert.Contains(t, sel, "line07", "selection across a fold includes the hidden rows")
}

func TestEntry_HiddenRanges_ReconcileOnShrink(t *testing.T) {
	e := foldEntry(t, 20)
	e.HiddenRanges = []widget.LineRange{{Start: 15, End: 18}}
	e.Refresh()

	// Document shrinks below the folded range — the stale range must be ignored.
	e.SetText("a\nb\nc\nd\ne")
	e.Refresh()

	yp, h := e.RowPositions()
	require.Len(t, yp, 5)
	for r := range h {
		assert.Greater(t, h[r], float32(0), "no row is folded after the document shrank below the range")
	}
}

func TestEntry_HiddenRanges_ZeroRegression(t *testing.T) {
	e := foldEntry(t, 10)
	yp, h := e.RowPositions()
	require.Len(t, yp, 10)
	_, stride := e.RowMetrics()

	for r := 0; r < 10; r++ {
		assert.InDelta(t, stride, h[r], 0.5, "with no folds every row keeps full height")
		if r > 0 {
			assert.InDelta(t, yp[r-1]+stride, yp[r], 0.5, "rows are evenly spaced by the stride")
		}
	}
}
