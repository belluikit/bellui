package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// newMixedDirRichText builds a RichText whose single row is two inline
// TextSegments of opposite direction: an LTR Latin span followed by an RTL
// Arabic span. The row therefore mixes directions across a segment boundary,
// which is what issue #24 (cross-segment bidi) exercises.
func newMixedDirRichText(t *testing.T) *RichText {
	t.Helper()
	ltr := &TextSegment{Text: "abc ", Style: RichTextStyle{Inline: true}}
	rtl := &TextSegment{Text: "مرحبا", Style: RichTextStyle{Inline: true}}
	rich := NewRichText(ltr, rtl)
	rich.Resize(geo.NewSize(300, 50))
	rich.CreateRenderer()
	test.TempWidgetRenderer(t, rich) // force a layout pass so rowBounds exist
	return rich
}

// TestRichText_MultiSegmentRowAssembled is a precondition check: the two inline
// segments really do land on one row, so the rest of the tests exercise the
// multi-segment bidi path rather than the single-segment fast path.
func TestRichText_MultiSegmentRowAssembled(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newMixedDirRichText(t)
	require.Equal(t, 1, rich.rows())
	require.Len(t, rich.rowBoundary(0).segments, 2,
		"both inline segments must share a single row")
	assert.Equal(t, []rune("abc مرحبا"), rich.row(0))
}

// TestRichText_RTLCaretXMultiSegment confirms rtlCaretX now resolves carets on a
// multi-segment RTL row (it used to bail to ok=false for len(segments)!=1).
func TestRichText_RTLCaretXMultiSegment(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newMixedDirRichText(t)

	x0, h, ok := rich.rtlCaretX(0, 0, true)
	require.True(t, ok, "multi-segment RTL row must be handled")
	assert.Greater(t, h, float32(0), "caret height should be the row height")

	// The Arabic span (offsets 4..9) reads right-to-left, so its trailing logical
	// offset (9) sits visually left of its leading offset (4).
	left := mustCaretX(t, rich, 9)
	right := mustCaretX(t, rich, 4)
	assert.Less(t, left, right,
		"in the RTL span the trailing offset must be visually left of the leading one")

	// The start-of-line caret (offset 0, the LTR 'a') is at the far left.
	assert.LessOrEqual(t, x0, left, "the leading LTR offset should be the leftmost caret")
}

// TestRichText_RTLSelectionSpansMultiSegment confirms a logically-contiguous
// selection crossing the LTR→RTL segment boundary maps to multiple disjoint
// visual rectangles.
func TestRichText_RTLSelectionSpansMultiSegment(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newMixedDirRichText(t)
	innerPad := rich.Theme().Size(theme.SizeNameInnerPadding)

	// Select logical [2,5): 'c', the space (both LTR) and the first Arabic letter
	// (offset 4), which sits at the far right of the Arabic block — disjoint.
	spans, ok := rich.rtlSelectionSpans(0, 2, 5, innerPad)
	require.True(t, ok)
	assert.Greater(t, len(spans), 1,
		"a selection crossing the segment direction boundary must be multiple rects")
	for i := 1; i < len(spans); i++ {
		assert.LessOrEqual(t, spans[i-1][1], spans[i][0]+0.5, "spans must be ordered and disjoint")
	}

	// A selection entirely within the LTR segment is a single rectangle.
	spans, ok = rich.rtlSelectionSpans(0, 0, 3, innerPad)
	require.True(t, ok)
	assert.Len(t, spans, 1, "an all-LTR selection should be a single rectangle")
}

// TestRichText_VisualColNeighborMultiSegment confirms visual arrow stepping walks
// the whole multi-segment row left-to-right, visiting every caret once.
func TestRichText_VisualColNeighborMultiSegment(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newMixedDirRichText(t)
	n := rich.rowLength(0)

	// Find the visually-leftmost column by walking left until we hit the edge.
	col := 0
	for {
		prev, moved, atEdge := rich.visualColNeighbor(0, col, -1)
		if atEdge || !moved {
			break
		}
		col = prev
	}

	seen := map[int]bool{col: true}
	prevX := mustCaretX(t, rich, col)
	for {
		next, moved, atEdge := rich.visualColNeighbor(0, col, +1)
		if atEdge || !moved {
			break
		}
		x := mustCaretX(t, rich, next)
		assert.GreaterOrEqual(t, x, prevX, "a visual-right step must not move left")
		assert.False(t, seen[next], "visual walk must not revisit column %d", next)
		seen[next] = true
		prevX, col = x, next
	}
	assert.Len(t, seen, n+1, "the visual walk should reach every caret (%d columns)", n+1)
}

func mustCaretX(t *testing.T, rich *RichText, col int) float32 {
	t.Helper()
	x, _, ok := rich.rtlCaretX(0, col, true)
	require.True(t, ok)
	return x
}
