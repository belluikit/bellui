package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newWrappedEntry builds a word-wrapping multi-line Entry in a narrow window so
// its first logical line wraps to several visual rows — the #145 case. It
// returns the entry once its first logical line is confirmed to span >1 row.
func newWrappedEntry(t *testing.T) *Entry {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(120, 200))
	e.Resize(geo.NewSize(100, 180))

	// A long first line that must wrap, then two short logical lines.
	e.SetText("alpha bravo charlie delta echo foxtrot golf\nsecond\nthird")
	e.Refresh()
	return e
}

// rowsOfLine returns the visual rows whose logical line equals line. It uses the
// analytic firstRowOfLine so it works for both the dense rowToLine map and the lazy
// model (#157), which carries no per-row map.
func rowsOfLine(e *Entry, line int) []int {
	start := e.text.firstRowOfLine(line)
	end := e.text.firstRowOfLine(line + 1)
	var rows []int
	for r := start; r < end; r++ {
		rows = append(rows, r)
	}
	return rows
}

// lineOfRowForTest returns the logical line of a visual row for both row models.
func lineOfRowForTest(e *Entry, row int) int {
	if e.text.lazy {
		line, _ := e.text.lineOfVisualRow(row)
		return line
	}
	return e.text.rowToLine[row]
}

// TestEntry_WrappedFold_HidesAllWrappedRows is the core #145 acceptance: folding
// a logical line that wraps to several visual rows hides every one of those rows.
func TestEntry_WrappedFold_HidesAllWrappedRows(t *testing.T) {
	e := newWrappedEntry(t)

	require.True(t, e.text.canVirtualize(), "wrap-on entry now virtualises (#139)")
	line0 := rowsOfLine(e, 0)
	require.GreaterOrEqual(t, len(line0), 2, "first logical line must wrap to multiple rows")
	require.Greater(t, e.text.rows(), 3, "document has more visual rows than logical lines")

	visBefore := e.text.visibleRowCount()

	// Fold logical line 0 — a single logical-line range.
	e.HiddenRanges = []LineRange{{Start: 0, End: 1}}
	e.Refresh()

	// Every visual row of line 0 is hidden; no other row is.
	for r := 0; r < e.text.rows(); r++ {
		l := lineOfRowForTest(e, r)
		if l == 0 {
			assert.True(t, e.text.rowHidden(r), "wrapped row %d of folded line 0 must be hidden", r)
		} else {
			assert.False(t, e.text.rowHidden(r), "row %d (line %d) must stay visible", r, l)
		}
	}

	// Visible-row count drops by exactly the folded line's wrapped-row span.
	assert.Equal(t, visBefore-len(line0), e.text.visibleRowCount(),
		"folding a wrapped line removes all of its visual rows from the visible count")

	// Geometry: each folded row collapses to zero height (scroll extent shrinks).
	_, heights := e.RowPositions()
	require.Equal(t, e.text.rows(), len(heights))
	for _, r := range line0 {
		assert.Equal(t, float32(0), heights[r], "folded wrapped row %d keeps zero height", r)
	}
}

// TestEntry_WrappedFold_CaretNavSkips checks the F4 caret contract holds for a
// wrapped fold: arrow-down from before the fold lands on the first visible row
// past all the folded wrapped rows.
func TestEntry_WrappedFold_CaretNavSkips(t *testing.T) {
	e := newWrappedEntry(t)
	line0 := rowsOfLine(e, 0)
	require.GreaterOrEqual(t, len(line0), 2)

	// Fold the wrapped first line, then put the caret on its first row.
	e.HiddenRanges = []LineRange{{Start: 0, End: 1}}
	e.Refresh()

	e.CursorRow, e.CursorColumn = 0, 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyDown})

	assert.False(t, e.text.rowHidden(e.CursorRow), "caret must not land on a folded row")
	assert.Equal(t, len(line0), e.CursorRow,
		"caret skips all wrapped rows of the folded line to the first visible row")
}

// TestEntry_WrappedFold_WrapOffIdentity guards backward compatibility: with
// wrapping off, one logical line is one visual row, so a logical-line fold maps
// to exactly that row (the original F4 behaviour).
func TestEntry_WrappedFold_WrapOffIdentity(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(200, 200))
	e.SetText("aaa\nbbb\nccc\nddd")
	e.Refresh()

	// Wrap-off: one logical line is one visual row, so the row<->line map is the
	// identity (served analytically in the lazy model #157).
	require.Equal(t, 4, e.text.rows())
	for i := 0; i < 4; i++ {
		require.Equal(t, i, e.text.firstRowOfLine(i))
		require.Equal(t, i, lineOfRowForTest(e, i))
	}

	e.HiddenRanges = []LineRange{{Start: 1, End: 2}}
	e.Refresh()

	assert.False(t, e.text.rowHidden(0))
	assert.True(t, e.text.rowHidden(1), "wrap-off: logical line 1 is exactly visual row 1")
	assert.False(t, e.text.rowHidden(2))
	assert.Equal(t, 3, e.text.visibleRowCount())
}
