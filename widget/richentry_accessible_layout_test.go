package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// RichEntry's core.AccessibleTextLayouter implementation (pre-rebrand tracker #1116): visual
// (rendered) row structure plus on-screen range geometry for the platform
// accessibility bridges (the iOS VoiceOver editing ring).

func TestRichEntryAccessibleLayout_VisualLines(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog again and again\nshort"},
	})
	defer w.Close()

	lines := e.AccessibleLineCount()
	require.Greater(t, lines, 2, "the first logical line must soft-wrap into several visual rows")

	// Line ranges tile the document: contiguous, ascending, covering [0, len].
	prevEnd := 0
	for line := 0; line < lines; line++ {
		start, end, ok := e.AccessibleLineRange(line)
		require.True(t, ok, "line %d", line)
		assert.Equal(t, prevEnd, start, "line %d starts where line %d ended", line, line-1)
		assert.GreaterOrEqual(t, end, start)
		prevEnd = end
	}
	assert.Equal(t, e.doc.len(), prevEnd, "the last line ends at the content length")

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
}

func TestRichEntryAccessibleLayout_LogicalLines(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "one\ntwo\nthree"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(300, 200))

	// Without wrapping the visual structure is the logical one.
	assert.Equal(t, 3, e.AccessibleLineCount())
	start, end, ok := e.AccessibleLineRange(1)
	require.True(t, ok)
	assert.Equal(t, 4, start)
	assert.Equal(t, 8, end, "the range covers the terminating newline")
}

func TestRichEntryAccessibleLayout_RangeBounds(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog\nsecond line here"},
	})
	defer w.Close()
	// A tall widget so every visual row is inside the viewport.
	w.Resize(geo.NewSize(120, 400))

	// A degenerate range yields the caret rectangle at that offset.
	rects := e.AccessibleRangeBounds(2, 2)
	require.Len(t, rects, 1)
	assert.Positive(t, rects[0].Size.Height)

	// A multi-line range yields one rect per visible visual row, top to bottom,
	// all inside the widget.
	total := e.doc.len()
	rects = e.AccessibleRangeBounds(0, total)
	lines := e.AccessibleLineCount()
	require.Len(t, rects, lines, "every visual row is visible in a tall widget")
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

func TestRichEntryAccessibleLayout_RangeBoundsViewportClipped(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: strings.Repeat("word word word\n", 200)}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(200, 80)) // a small viewport over a tall document

	total := e.doc.len()
	rects := e.AccessibleRangeBounds(0, total)
	require.NotEmpty(t, rects)
	// The result is bounded by the viewport, not the 200-line range: rows below
	// the fold are dropped by the top-to-bottom walk's early break.
	assert.Less(t, len(rects), 20, "scrolled-out rows are omitted")
	size := e.Size()
	for _, r := range rects {
		assert.LessOrEqual(t, r.Pos.Y+r.Size.Height, size.Height)
	}
}
