package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/lang"
)

// TestStyledLineLayoutBoxWidth confirms an opaque box (a non-text segment such as
// a link or image) consumes its width in the line's visual order while adding no
// caret column: carets stay text-only and the caret after the box jumps by the
// box width (issue #41).
func TestStyledLineLayoutBoxWidth(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	plain := StyledLineVisualLayout([]StyledRun{{Runes: []rune("abcd"), TextSize: 14}})
	boxed := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("ab"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("cd"), TextSize: 14},
	})

	assert.False(t, boxed.HasRTL())
	assert.Len(t, boxed.Carets, 5, "the box must contribute no caret column (4 runes → 5 carets)")
	assert.InDelta(t, plain.Width+10, boxed.Width, 0.5, "the box must widen the line by its width")
	assert.InDelta(t, plain.Carets[2]+10, boxed.Carets[2], 0.5,
		"the caret after the box must include the box width")
	assert.InDelta(t, plain.Carets[1], boxed.Carets[1], 0.5,
		"carets before the box are unchanged")
}

// TestStyledLineLayoutBoxSelection confirms a box is filled by a selection iff the
// rune it trails is selected — matching the legacy single-rectangle behaviour.
func TestStyledLineLayoutBoxSelection(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	l := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("ab"), TextSize: 14},
		{Box: true, BoxWidth: 10},
		{Runes: []rune("cd"), TextSize: 14},
	})
	afterBox := l.Carets[2] // x just past the box

	full := l.SelectionSpans(0, 4)
	require.Len(t, full, 1, "selecting the whole row is one contiguous span")
	assert.InDelta(t, 0, full[0][0], 0.5)
	assert.InDelta(t, l.Width, full[0][1], 0.5, "the span must reach the box at the line's end")

	head := l.SelectionSpans(0, 2)
	require.Len(t, head, 1)
	assert.InDelta(t, afterBox, head[0][1], 0.5,
		"selecting the leading text fills the box (its owning rune is selected)")

	tail := l.SelectionSpans(2, 4)
	require.Len(t, tail, 1)
	assert.InDelta(t, afterBox, tail[0][0], 0.5,
		"selecting only the trailing text must start after the box, not fill it")
}

// TestStyledLineLayoutBoxRTL confirms a box interleaves into a right-to-left
// line's visual order and is reachable by a selection of the whole row.
func TestStyledLineLayoutBoxRTL(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	noBox := StyledLineVisualLayout([]StyledRun{{Runes: []rune("مرحبا"), TextSize: 14}})
	withBox := StyledLineVisualLayout([]StyledRun{
		{Runes: []rune("مرحبا"), TextSize: 14},
		{Box: true, BoxWidth: 12},
	})

	require.True(t, withBox.HasRTL())
	assert.InDelta(t, noBox.Width+12, withBox.Width, 0.5, "the box must widen the RTL line")

	spans := withBox.SelectionSpans(0, 5)
	var maxRight float32
	for _, s := range spans {
		if s[1] > maxRight {
			maxRight = s[1]
		}
	}
	assert.InDelta(t, withBox.Width, maxRight, 0.5,
		"selecting the whole RTL row must reach the box at the visual right edge")
}
