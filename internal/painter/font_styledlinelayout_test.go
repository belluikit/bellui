package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

// styledRunsSameStyle splits s into StyledRuns at the given rune offsets, all
// sharing one style — the layout must then match a whole-line LineVisualLayout.
func styledRunsSameStyle(s string, style bellui.TextStyle, splits ...int) []StyledRun {
	runes := []rune(s)
	bounds := append([]int{0}, splits...)
	bounds = append(bounds, len(runes))
	runs := make([]StyledRun, 0, len(bounds)-1)
	for i := 0; i+1 < len(bounds); i++ {
		runs = append(runs, StyledRun{Runes: runes[bounds[i]:bounds[i+1]], TextSize: 14, Style: style})
	}
	return runs
}

// TestStyledLineLayoutMatchesSingleStyle is the core invariant: when a line is
// split into several same-style runs at non-joining boundaries (spaces, direction
// changes), StyledLineVisualLayout reproduces the whole-line LineVisualLayout —
// same carets, extents and width. This pins the multi-segment caret model to the
// already-tested single-style primitive.
func TestStyledLineLayoutMatchesSingleStyle(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	cases := []struct {
		s      string
		splits []int
	}{
		{"abc def", []int{4}},          // pure LTR, split at the space
		{"abc مرحبا def", []int{4, 9}}, // LTR | RTL word | LTR, split at both spaces
		{"שלום abc", []int{5}},         // RTL word | LTR
		{"abc مرحبا", []int{4}},        // LTR | RTL
	}
	for _, tc := range cases {
		whole := layoutFor(t, tc.s, bellui.TextStyle{})
		styled := StyledLineVisualLayout(styledRunsSameStyle(tc.s, bellui.TextStyle{}, tc.splits...))

		assert.Equal(t, whole.HasRTL(), styled.HasRTL(), "%q: HasRTL mismatch", tc.s)
		assert.InDelta(t, whole.Width, styled.Width, 0.5, "%q: width mismatch", tc.s)
		assert.Len(t, styled.Carets, len(whole.Carets), "%q: caret count mismatch", tc.s)
		for i := range whole.Carets {
			assert.InDelta(t, whole.Carets[i], styled.Carets[i], 0.5,
				"%q: caret %d mismatch", tc.s, i)
		}
	}
}

// TestStyledLineLayoutHitTestRoundTrip confirms hit-testing round-trips on a
// genuinely multi-style line (a bold LTR run beside a regular RTL run).
func TestStyledLineLayoutHitTestRoundTrip(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	runs := []StyledRun{
		{Runes: []rune("abc "), TextSize: 14, Style: bellui.TextStyle{Bold: true}},
		{Runes: []rune("מרחבا"), TextSize: 18, Style: bellui.TextStyle{}},
		{Runes: []rune(" def"), TextSize: 14, Style: bellui.TextStyle{Italic: true}},
	}
	l := StyledLineVisualLayout(runs)
	assert.True(t, l.HasRTL(), "mixed multi-style line should report an RTL run")

	for i := range l.Carets {
		assert.Equal(t, l.Carets[i], l.Carets[l.HitTest(l.Carets[i])],
			"HitTest(CaretX(%d)) must land on a caret with the same x", i)
	}
}

// TestStyledLineLayoutVisualWalk confirms a left-to-right visual walk over a
// multi-style mixed line reaches every caret exactly once without moving left.
func TestStyledLineLayoutVisualWalk(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	runs := []StyledRun{
		{Runes: []rune("abc "), TextSize: 14, Style: bellui.TextStyle{Bold: true}},
		{Runes: []rune("מרחבا def"), TextSize: 14, Style: bellui.TextStyle{}},
	}
	l := StyledLineVisualLayout(runs)

	start := l.HitTest(-1e6)
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

// TestStyledLineLayoutPerRunSize confirms each run is shaped at its own size: a
// larger run contributes proportionally more width than the same text at the
// default size.
func TestStyledLineLayoutPerRunSize(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	small := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("abc"), TextSize: 14, Style: bellui.TextStyle{}},
		{Runes: []rune("def"), TextSize: 14, Style: bellui.TextStyle{}},
	})
	big := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("abc"), TextSize: 14, Style: bellui.TextStyle{}},
		{Runes: []rune("def"), TextSize: 28, Style: bellui.TextStyle{}},
	})
	assert.Greater(t, big.Width, small.Width,
		"a larger second run must widen the line")
}

// TestStyledLineLayoutEmpty confirms the degenerate cases.
func TestStyledLineLayoutEmpty(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	assert.Equal(t, []float32{0}, StyledLineVisualLayout(nil).Carets)
	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune(""), TextSize: 14, Style: bellui.TextStyle{}},
		{Runes: []rune(""), TextSize: 14, Style: bellui.TextStyle{}},
	})
	assert.Equal(t, []float32{0}, l.Carets)
	assert.False(t, l.HasRTL())
}
