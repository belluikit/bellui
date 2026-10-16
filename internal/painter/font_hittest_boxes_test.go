package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/lang"
)

// TestStyledLineLayoutHitTestBoxesLTR pins the box-aware inverse of BoxCaretX in
// plain left-to-right flow: a click inside the box resolves to that box (nearer
// edge → before/after), a click in the surrounding text resolves to text (pre-rebrand tracker #983).
func TestStyledLineLayoutHitTestBoxesLTR(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("ab"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("cd"), TextSize: 14},
	})
	require.Equal(t, 1, l.BoxCount())

	before := l.BoxCaretX(0, false) // left edge (LTR before-side)
	after := l.BoxCaretX(0, true)   // right edge
	mid := (before + after) / 2

	// Click on the box's leading half → before it.
	_, _, box, aft, isBox := l.HitTestBoxes(before + (mid-before)/2)
	assert.True(t, isBox)
	assert.Equal(t, 0, box)
	assert.False(t, aft, "leading-half click on the box resolves before it")

	// Click on the box's trailing half → after it.
	_, _, box, aft, isBox = l.HitTestBoxes(after - (after-mid)/2)
	assert.True(t, isBox)
	assert.Equal(t, 0, box)
	assert.True(t, aft, "trailing-half click on the box resolves after it")

	// Click well inside "a" → text, not a box.
	off, _, _, _, isBox := l.HitTestBoxes(0.5)
	assert.False(t, isBox, "a click in the text resolves to a text caret")
	assert.LessOrEqual(t, off, 1)

	// Click at the far right (past "cd") → text end, not a box.
	off, _, _, _, isBox = l.HitTestBoxes(l.Width + 5)
	assert.False(t, isBox)
	assert.Equal(t, len(l.Carets)-1, off, "past the line end resolves to the last text caret")
}

// TestStyledLineLayoutHitTestBoxesRTLWords pins the box-aware inverse between two
// right-to-left words, where the box edges genuinely differ from the text carets
// (the pre-rebrand tracker #913 property): a click inside the box must resolve to the box rather than
// snap to the neighbouring RTL text caret, and the before/after sides track the
// box's own before-/after-edge positions (BoxCaretX) — direction-agnostic.
func TestStyledLineLayoutHitTestBoxesRTLWords(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("مرحبا"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("سلام"), TextSize: 14},
	})
	require.True(t, l.HasRTL())
	require.Equal(t, 1, l.BoxCount())

	beforeX := l.BoxCaretX(0, false) // the before-side edge (facing the preceding word)
	afterX := l.BoxCaretX(0, true)   // the after-side edge
	left, right := beforeX, afterX
	if left > right {
		left, right = right, left
	}

	// Click in the middle of the box → the box, not the neighbouring text caret.
	_, _, box, _, isBox := l.HitTestBoxes((left + right) / 2)
	assert.True(t, isBox, "a click inside the box resolves to it, not the neighbouring RTL text caret")
	assert.Equal(t, 0, box)

	// A click nearer the before-edge → before; nearer the after-edge → after.
	_, _, _, aft, isBox := l.HitTestBoxes(beforeX + (afterX-beforeX)*0.25)
	require.True(t, isBox)
	assert.False(t, aft, "a click nearer the before-edge resolves before the box")

	_, _, _, aft, isBox = l.HitTestBoxes(beforeX + (afterX-beforeX)*0.75)
	require.True(t, isBox)
	assert.True(t, aft, "a click nearer the after-edge resolves after the box")
}
