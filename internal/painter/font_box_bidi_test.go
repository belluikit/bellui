package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/lang"
)

// TestStyledLineLayoutBoxCaretXLTR pins the insertion points around a box in
// plain left-to-right flow: before = left edge, after = right edge (#913).
func TestStyledLineLayoutBoxCaretXLTR(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("ab"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("cd"), TextSize: 14},
	})
	require.Equal(t, 1, l.BoxCount())

	before := l.BoxCaretX(0, false)
	after := l.BoxCaretX(0, true)
	assert.InDelta(t, before+10, after, 0.5, "LTR: after = before + box width")
	assert.InDelta(t, l.CaretXAt(2, AffinityUpstream), before, 0.5,
		"LTR: before the box = trailing edge of the preceding rune")
	assert.InDelta(t, l.CaretXAt(2, AffinityDownstream), after, 0.5,
		"LTR: after the box = leading edge of the following rune")
}

// TestStyledLineLayoutBoxCaretXRTLWords pins the insertion points around a box
// between two right-to-left words. The line ordering primitive (lineVisualSeq,
// shared with painting) keeps runs in logical order and reverses within each
// RTL run, so the box sits between the words — after the first word's visual
// extent — and its insertion points are its own edges. Those edges are NOT the
// neighbouring text carets: the text caret at the same logical offset hugs the
// RTL word's logical end (its visual left), which is exactly why the caret
// beside an inline atom must come from the box extent (#913).
func TestStyledLineLayoutBoxCaretXRTLWords(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	first := StyledLineVisualLayout([]StyledRun{{Runes: []rune("مرحبا"), TextSize: 14}})
	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("مرحبا"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("سلام"), TextSize: 14},
	})
	require.True(t, l.HasRTL())
	require.Equal(t, 1, l.BoxCount())

	before := l.BoxCaretX(0, false)
	after := l.BoxCaretX(0, true)
	assert.InDelta(t, first.Width, before, 0.5, "the box trails the first word's visual extent")
	assert.InDelta(t, before+10, after, 0.5, "after = before + box width")
	// The same logical offset's *text* caret resolves to the RTL word's visual
	// left edge — the box edges and text carets genuinely differ here.
	assert.InDelta(t, 0, l.CaretXAt(5, AffinityUpstream), 0.5,
		"the upstream text caret hugs the RTL word's logical end (visual left)")
	assert.InDelta(t, l.Width, l.CaretXAt(5, AffinityDownstream), 0.5,
		"the downstream text caret hugs the next RTL word's logical start (visual right)")
}

// TestStyledLineLayoutSelectionSpansBoxes verifies caller-controlled box
// inclusion: an atom-only selection (empty text range) fills exactly the box,
// and a text selection with the box excluded leaves a gap where the legacy
// owner-rune heuristic would have filled it (#913).
func TestStyledLineLayoutSelectionSpansBoxes(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("ab"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("cd"), TextSize: 14},
	})
	boxLeft := l.BoxCaretX(0, false)
	boxRight := l.BoxCaretX(0, true)

	// Atom-only: empty text range, box selected.
	only := l.SelectionSpansBoxes(2, 2, func(int) bool { return true })
	require.Len(t, only, 1)
	assert.InDelta(t, boxLeft, only[0][0], 0.5)
	assert.InDelta(t, boxRight, only[0][1], 0.5)

	// Leading text without the box: the span must stop at the box's left edge
	// (SelectionSpans' owner heuristic would have filled it).
	head := l.SelectionSpansBoxes(0, 2, func(int) bool { return false })
	require.Len(t, head, 1)
	assert.InDelta(t, boxLeft, head[0][1], 0.5, "excluded box must not extend the span")

	// Everything: one contiguous span across text and box.
	all := l.SelectionSpansBoxes(0, 4, func(int) bool { return true })
	require.Len(t, all, 1)
	assert.InDelta(t, 0, all[0][0], 0.5)
	assert.InDelta(t, l.Width, all[0][1], 0.5)

	// Nothing selected at all yields no spans.
	assert.Empty(t, l.SelectionSpansBoxes(2, 2, func(int) bool { return false }))
}
