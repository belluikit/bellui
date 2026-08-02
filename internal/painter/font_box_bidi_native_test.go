// The RTL caret-box math measurably DIVERGES under GOOS=js — a real
// behavioural difference (caret placement in RTL text on the web target),
// tracked in #168; this tag keeps the leg green while that is root-caused,
// it is not the fix. (#79 Phase B.)
//go:build !wasm

package painter

import (
	"testing"

	"github.com/belluikit/bellui/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
