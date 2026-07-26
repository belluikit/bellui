package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

// layoutFor builds a LineVisualLayout for s under the current locale/style.
func layoutFor(t *testing.T, s string, style bellui.TextStyle) LineLayout {
	t.Helper()
	face := CachedFontFace(style, nil, nil)
	return LineVisualLayout(face.Fonts, []rune(s), 14, style)
}

// TestLineLayoutLTRMonotonic confirms that for pure-LTR text the caret table is
// monotonically increasing and matches the measured width.
func TestLineLayoutLTRMonotonic(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := layoutFor(t, "abc def", bellui.TextStyle{})
	assert.False(t, l.HasRTL(), "Latin line should report no RTL run")
	assert.Len(t, l.Carets, len("abc def")+1)

	for i := 1; i < len(l.Carets); i++ {
		assert.GreaterOrEqual(t, l.Carets[i], l.Carets[i-1],
			"LTR carets must be non-decreasing at %d", i)
	}
	assert.InDelta(t, l.Width, l.Carets[len(l.Carets)-1], 0.01,
		"final caret should equal the line width")

	face := CachedFontFace(bellui.TextStyle{}, nil, nil)
	measured, _ := MeasureString(face.Fonts, "abc def", 14, bellui.TextStyle{})
	assert.InDelta(t, measured.Width, l.Width, 0.5, "layout width should match MeasureString")
}

// TestLineLayoutMixedNonMonotonic confirms that a mixed LTR/RTL line produces a
// non-monotonic caret table (the RTL span runs right-to-left) and that the RTL
// run is detected.
func TestLineLayoutMixedNonMonotonic(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	const s = "abc مرحبا def"
	l := layoutFor(t, s, bellui.TextStyle{})
	assert.True(t, l.HasRTL(), "mixed line should report an RTL run")

	monotonic := true
	for i := 1; i < len(l.Carets); i++ {
		if l.Carets[i] < l.Carets[i-1] {
			monotonic = false
			break
		}
	}
	assert.False(t, monotonic, "mixed-direction carets must not be monotonic")
}

// TestLineLayoutHitTestRoundTrip is the core correctness check: hit-testing the
// visual x of every logical caret returns that same logical offset.
func TestLineLayoutHitTestRoundTrip(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	for _, s := range []string{"abc def", "abc مرحبا def", "مرحبا world", "שלום abc"} {
		l := layoutFor(t, s, bellui.TextStyle{})
		for i := range l.Carets {
			// Under direction-boundary ties HitTest may pick an equivalent
			// offset, so assert the visual x round-trips (not the raw index).
			assert.Equal(t, l.Carets[i], l.Carets[l.HitTest(l.Carets[i])],
				"%q: HitTest(CaretX(%d)) must land on a caret with the same x", s, i)
		}
	}
}

// TestLineLayoutArabicLeftEdge confirms the visual-to-logical mapping direction:
// the visual left edge of the Arabic span maps to the *highest* logical offset
// of that span (RTL runs read right-to-left).
func TestLineLayoutArabicLeftEdge(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	// "abc " = offsets 0..4, Arabic "مرحبا" = offsets 4..9.
	const s = "abc مرحبا"
	l := layoutFor(t, s, bellui.TextStyle{})

	// The Arabic span's visual left edge sits at the highest logical offset (9),
	// its visual right edge (adjacent to the Latin) at the lowest (4).
	leftEdgeOffset := l.CaretX(9)
	rightEdgeOffset := l.CaretX(4)
	assert.Less(t, leftEdgeOffset, rightEdgeOffset,
		"in RTL, the trailing logical offset is visually left of the leading offset")
}

// TestLineLayoutVisualNeighbor confirms visual arrow stepping crosses the line
// left-to-right regardless of logical order.
func TestLineLayoutVisualNeighbor(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := layoutFor(t, "abc مرحبا def", bellui.TextStyle{})

	// Walk visually right from the leftmost caret; x must strictly increase and
	// we must visit every caret exactly once before running off the edge.
	start := l.HitTest(-1e6) // leftmost visual caret
	seen := map[int]bool{start: true}
	cur := start
	prevX := l.Carets[cur]
	for {
		next := l.VisualNeighbor(cur, +1)
		if next < 0 {
			break
		}
		assert.GreaterOrEqual(t, l.Carets[next], prevX, "visual-right step must not move left")
		assert.False(t, seen[next], "visual walk should not revisit caret %d", next)
		seen[next] = true
		prevX = l.Carets[next]
		cur = next
	}
	assert.Len(t, seen, len(l.Carets), "visual walk should reach every caret")
}

// TestSelectionSpansLTRSingle confirms a pure-LTR selection is one span.
func TestSelectionSpansLTRSingle(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, "abcdef", bellui.TextStyle{})
	spans := l.SelectionSpans(1, 4)
	assert.Len(t, spans, 1)
	assert.Less(t, spans[0][0], spans[0][1])
	assert.InDelta(t, l.Carets[1], spans[0][0], 0.01)
	assert.InDelta(t, l.Carets[4], spans[0][1], 0.01)
}

// TestSelectionSpansMixedDisjoint confirms a logically-contiguous selection that
// straddles a direction boundary yields multiple disjoint, ordered spans.
func TestSelectionSpansMixedDisjoint(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	// "abc مرحبا def": select logical [2,5) — 'c', the space, and only the first
	// Arabic letter (offset 4). Because the Arabic reads right-to-left, offset 4
	// sits at the far right of the Arabic block, so the selection is two disjoint
	// visual rectangles: "c " on the left and the lone Arabic letter on the right.
	l := layoutFor(t, "abc مرحبا def", bellui.TextStyle{})
	spans := l.SelectionSpans(2, 5)
	assert.Greater(t, len(spans), 1, "partial selection crossing a direction boundary must be multiple spans")

	// Spans must be ordered and non-overlapping.
	for i := 1; i < len(spans); i++ {
		assert.LessOrEqual(t, spans[i-1][1], spans[i][0]+0.5, "spans must be disjoint and ordered")
	}
	for _, s := range spans {
		assert.Less(t, s[0], s[1], "each span must have positive width")
	}
}

// TestSelectionSpansEmpty confirms an empty range yields no spans.
func TestSelectionSpansEmpty(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, "abc مرحبا def", bellui.TextStyle{})
	assert.Nil(t, l.SelectionSpans(3, 3))
}

// TestLineVisualLayoutEmpty confirms the degenerate case.
func TestLineVisualLayoutEmpty(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, "", bellui.TextStyle{})
	assert.Equal(t, []float32{0}, l.Carets)
	assert.Equal(t, float32(0), l.Width)
	assert.False(t, l.HasRTL())
}
