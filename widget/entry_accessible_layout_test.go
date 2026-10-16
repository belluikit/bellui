package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// Entry's core.AccessibleTextLayouter implementation (pre-rebrand tracker #990): visual
// (soft-wrapped) line structure plus on-screen range geometry for the
// platform accessibility bridges.

func TestEntryAccessibleLayout_VisualLines(t *testing.T) {
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	e.SetText("the quick brown fox jumps over the lazy dog again and again\nshort")
	e.Resize(geo.NewSize(150, 200)) // narrow enough to force soft-wrapping

	lines := e.AccessibleLineCount()
	require.Greater(t, lines, 2, "the first logical line must soft-wrap into several visual lines")

	// Line ranges tile the document: contiguous, ascending, covering [0, len].
	prevEnd := 0
	for line := 0; line < lines; line++ {
		start, end, ok := e.AccessibleLineRange(line)
		require.True(t, ok, "line %d", line)
		assert.Equal(t, prevEnd, start, "line %d starts where line %d ended", line, line-1)
		assert.GreaterOrEqual(t, end, start)
		prevEnd = end
	}
	assert.Equal(t, len([]rune(e.Text)), prevEnd, "the last line ends at the content length")

	// LineForOffset agrees with the ranges.
	for line := 0; line < lines; line++ {
		start, end, _ := e.AccessibleLineRange(line)
		if end > start {
			assert.Equal(t, line, e.AccessibleLineForOffset(start), "start of line %d", line)
		}
	}
	_, _, ok := e.AccessibleLineRange(lines)
	assert.False(t, ok, "one past the last line is out of range")
	_, _, ok = e.AccessibleLineRange(-1)
	assert.False(t, ok)

	// Without wrapping the visual structure is the logical one.
	plain := NewMultiLineEntry()
	plain.SetText("one\ntwo\nthree")
	plain.Resize(geo.NewSize(300, 200))
	assert.Equal(t, 3, plain.AccessibleLineCount())
	start, end, ok := plain.AccessibleLineRange(1)
	require.True(t, ok)
	assert.Equal(t, 4, start)
	assert.Equal(t, 8, end, "the range covers the terminating newline")
}

func TestEntryAccessibleLayout_RangeBounds(t *testing.T) {
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	e.SetText("the quick brown fox jumps over the lazy dog\nsecond line here")
	e.Resize(geo.NewSize(150, 200))

	// A degenerate range yields the caret rectangle at that offset.
	rects := e.AccessibleRangeBounds(2, 2)
	require.Len(t, rects, 1)
	assert.Positive(t, rects[0].Size.Height)

	// A multi-line range yields one rect per visual line, top to bottom,
	// all inside the widget.
	total := len([]rune(e.Text))
	rects = e.AccessibleRangeBounds(0, total)
	lines := e.AccessibleLineCount()
	require.Len(t, rects, lines, "every visual line is visible in a tall widget")
	size := e.Size()
	for i, r := range rects {
		assert.Positive(t, r.Size.Width, "line %d", i)
		assert.Positive(t, r.Size.Height, "line %d", i)
		assert.GreaterOrEqual(t, r.Pos.X, float32(0))
		assert.GreaterOrEqual(t, r.Pos.Y, float32(0))
		assert.LessOrEqual(t, r.Pos.X+r.Size.Width, size.Width)
		assert.LessOrEqual(t, r.Pos.Y+r.Size.Height, size.Height)
		if i > 0 {
			assert.Greater(t, r.Pos.Y, rects[i-1].Pos.Y, "line rects run top to bottom")
		}
	}

	// Reversed offsets normalize.
	fwd := e.AccessibleRangeBounds(1, 5)
	rev := e.AccessibleRangeBounds(5, 1)
	assert.Equal(t, fwd, rev)
}

func TestEntryAccessibleLayout_RangeBoundsViewportClipped(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText(strings.Repeat("word word word\n", 200))
	e.Resize(geo.NewSize(200, 80)) // small viewport over a tall document

	total := len([]rune(e.Text))
	rects := e.AccessibleRangeBounds(0, total)
	require.NotEmpty(t, rects)
	// The result is bounded by the viewport, not the 200-line range.
	assert.Less(t, len(rects), 20, "scrolled-out lines are omitted")
	size := e.Size()
	for _, r := range rects {
		assert.LessOrEqual(t, r.Pos.Y+r.Size.Height, size.Height)
	}
}

func TestEntryAccessibleLayout_RangeBoundsScrolled(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText(strings.Repeat("word word word\n", 200))
	e.Resize(geo.NewSize(200, 80))

	// Scroll deep into the document: the rows above the viewport must be
	// skipped (the binary search over row tops), the visible ones clipped to
	// the viewport, and the walk must stop below the fold.
	e.scroll.Offset.Y = 1000
	total := len([]rune(e.Text))
	rects := e.AccessibleRangeBounds(0, total)
	require.NotEmpty(t, rects, "a mid-document viewport still reports its visible lines")
	assert.Less(t, len(rects), 20, "bounded by the viewport, not the 200-line range")

	size := e.Size()
	for i, r := range rects {
		assert.GreaterOrEqual(t, r.Pos.Y, float32(0), "line %d starts inside the viewport", i)
		assert.LessOrEqual(t, r.Pos.Y+r.Size.Height, size.Height, "line %d ends inside the viewport", i)
		if i > 0 {
			assert.Greater(t, r.Pos.Y, rects[i-1].Pos.Y, "line rects run top to bottom")
		}
	}
	// A line straddling the viewport top is clipped, not dropped: the first
	// rect is shorter than an interior (full-height) one.
	if len(rects) > 2 {
		assert.Less(t, rects[0].Size.Height, rects[1].Size.Height,
			"the partially scrolled-off first line is clipped to the viewport")
	}
}
