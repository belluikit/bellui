package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// --- helper math ---------------------------------------------------------

func TestNormalizeLineRanges(t *testing.T) {
	// Sort, drop empties, clamp Start>=0, merge overlapping and adjacent.
	got := normalizeLineRanges([]LineRange{
		{Start: 10, End: 12},
		{Start: 3, End: 3}, // empty, dropped
		{Start: -5, End: 2},
		{Start: 1, End: 4},   // overlaps the clamped [0,2)
		{Start: 12, End: 15}, // adjacent to [10,12)
	})
	assert.Equal(t, []LineRange{{Start: 0, End: 4}, {Start: 10, End: 15}}, got)

	assert.Nil(t, normalizeLineRanges(nil))
	assert.Nil(t, normalizeLineRanges([]LineRange{{Start: 5, End: 5}}))
}

func TestRichTextHiddenHelpers(t *testing.T) {
	// Pure geometry helpers depend only on hiddenRanges and the row count, so a
	// bare rowBounds of the right length is enough (no layout/theme needed).
	rt := &RichText{rowBounds: make([]rowBoundary, 30)}
	require.Equal(t, 30, rt.rows())

	rt.setHiddenRanges([]LineRange{{Start: 5, End: 10}, {Start: 20, End: 25}})

	// rowHidden
	assert.False(t, rt.rowHidden(4))
	assert.True(t, rt.rowHidden(5))
	assert.True(t, rt.rowHidden(9))
	assert.False(t, rt.rowHidden(10))
	assert.True(t, rt.rowHidden(24))
	assert.False(t, rt.rowHidden(25))

	// hiddenBefore / visibleRowCount: 5 + 5 hidden of 30 -> 20 visible.
	assert.Equal(t, 0, rt.hiddenBefore(5))
	assert.Equal(t, 5, rt.hiddenBefore(10))
	assert.Equal(t, 5, rt.hiddenBefore(20))
	assert.Equal(t, 10, rt.hiddenBefore(30))
	assert.Equal(t, 20, rt.visibleRowCount())

	// firstVisibleAtOrAfter / lastVisibleAtOrBefore around the first fold.
	assert.Equal(t, 4, rt.firstVisibleAtOrAfter(4))
	assert.Equal(t, 10, rt.firstVisibleAtOrAfter(5))
	assert.Equal(t, 10, rt.firstVisibleAtOrAfter(9))
	assert.Equal(t, 4, rt.lastVisibleAtOrBefore(9))
	assert.Equal(t, 4, rt.lastVisibleAtOrBefore(5))
	assert.Equal(t, 10, rt.lastVisibleAtOrBefore(10))

	// visibleIndexToRow maps the visible-row axis back to logical rows.
	assert.Equal(t, 0, rt.visibleIndexToRow(0))
	assert.Equal(t, 4, rt.visibleIndexToRow(4))
	assert.Equal(t, 10, rt.visibleIndexToRow(5))  // first visible after the first fold
	assert.Equal(t, 14, rt.visibleIndexToRow(9))  // last before the second fold (row 19)
	assert.Equal(t, 19, rt.visibleIndexToRow(14)) // then jumps past [20,25)
	assert.Equal(t, 25, rt.visibleIndexToRow(15))
}

// TestRichTextRowToLineTranslation exercises the #145 logical-line -> visual-row
// translation in isolation: buildRowToLine derives the map from a single
// segment's newlines and hand-crafted wrapped-row begin offsets, and a fold over
// a wrapped logical line expands to all of its visual rows.
func TestRichTextRowToLineTranslation(t *testing.T) {
	// "aa\nbbbb cccc\ndd": a0 a1 \n2 b3..6 ' '7 c8..11 \n12 d13 d14.
	// Simulated wrapped layout: r0 "aa"@0, r1 "bbbb"@3, r2 "cccc"@8, r3 "dd"@13.
	seg := &TextSegment{Text: "aa\nbbbb cccc\ndd"}
	rt := &RichText{Segments: []RichTextSegment{seg}, rowBounds: []rowBoundary{
		{begin: 0}, {begin: 3}, {begin: 8}, {begin: 13},
	}}
	rt.buildRowToLine()
	assert.Equal(t, []int{0, 1, 1, 2}, rt.rowToLine, "line 1 spans two wrapped rows")

	// Folding logical line 1 hides both of its wrapped visual rows.
	changed := rt.setHiddenRanges([]LineRange{{Start: 1, End: 2}})
	assert.True(t, changed)
	assert.Equal(t, []LineRange{{Start: 1, End: 3}}, rt.hiddenRanges)
	assert.False(t, rt.rowHidden(0))
	assert.True(t, rt.rowHidden(1))
	assert.True(t, rt.rowHidden(2))
	assert.False(t, rt.rowHidden(3))

	// A multi-line fold spanning lines 0..1 covers rows 0..2.
	rt.setHiddenRanges([]LineRange{{Start: 0, End: 2}})
	assert.Equal(t, []LineRange{{Start: 0, End: 3}}, rt.hiddenRanges)
}

// --- windowing must not materialise a folded block -----------------------

// TestVirtualWindow_SkipsHugeFold guards the core composition with #119: a fold
// that collapses thousands of rows in the middle of the document must not force
// those rows to materialise when the window sits at the fold boundary.
func TestVirtualWindow_SkipsHugeFold(t *testing.T) {
	e, _ := newVirtualEntry(t, repeatLines(virtTestRows))
	require.True(t, e.text.canVirtualize())

	// Fold an enormous middle block, leaving rows 0..49 and the tail visible.
	e.HiddenRanges = []LineRange{{Start: 50, End: virtTestRows - 50}}
	e.Refresh()

	// Scroll to the seam: just past the visible head, where the fold collapses.
	_, stride := e.RowMetrics()
	e.scroll.ScrollToOffset(geo.NewPos(0, 45*stride))
	e.updateTextViewport()
	cache.Renderer(&e.text).Refresh()

	r := textRendererOf(e)
	assert.Less(t, len(r.origObjs), 200,
		"a collapsed fold must not materialise its hidden rows, got %d objects", len(r.origObjs))

	// No materialised row is inside the fold.
	for _, row := range e.text.winRows {
		assert.False(t, e.text.rowHidden(row), "materialised row %d is inside the fold", row)
	}

	// The visible-row extent shrank by exactly the folded block's height.
	hidden := (virtTestRows - 50) - 50
	assert.Equal(t, e.text.rows()-hidden, e.text.visibleRowCount())
}

func TestVirtualWindow_NoFoldUnchanged(t *testing.T) {
	e, _ := newVirtualEntry(t, repeatLines(200))
	require.True(t, e.text.canVirtualize())

	// With no folds the window is a contiguous run, matching pre-folding behaviour.
	rows := e.text.computeWindowRows()
	require.NotEmpty(t, rows)
	for i := 1; i < len(rows); i++ {
		assert.Equal(t, rows[i-1]+1, rows[i], "window rows are contiguous without folds")
	}
}
