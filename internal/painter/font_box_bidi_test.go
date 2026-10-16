package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/lang"
)

// TestStyledLineLayoutBoxCaretXLTR pins the insertion points around a box in
// plain left-to-right flow: before = left edge, after = right edge (pre-rebrand tracker #913).
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
