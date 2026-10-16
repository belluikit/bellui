package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

// TestRichText_TrailingInlineBeforeBlock_NoOverlap reproduces pre-rebrand tracker #207: an inline
// segment that is not terminated by a newline, immediately followed by a block
// segment, must not share a row with the block. Before the fix the block was
// appended to the trailing-inline row and laid out at the same Y, overlapping it.
func TestRichText_TrailingInlineBeforeBlock_NoOverlap(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	inline := &TextSegment{Text: "trailing inline", Style: RichTextStyle{Inline: true}}
	block := &blockSegment{text: "block", width: 40, height: 30}
	rich := NewRichText(inline, block)
	rich.Resize(geo.NewSize(400, 200))
	r := test.TempWidgetRenderer(t, rich).(*textRenderer)

	// The block must occupy its own row, not be appended to the inline row.
	require.Len(t, rich.rowBounds, 2, "trailing inline and block must be on separate rows")
	require.Len(t, rich.rowBoundary(0).segments, 1, "inline row holds only the inline segment")
	require.Len(t, rich.rowBoundary(1).segments, 1, "block row holds only the block segment")

	// And the rendered visuals must not overlap vertically.
	objs := r.origObjs
	require.Len(t, objs, 2)
	inlineBottom := objs[0].Position().Y + objs[0].Size().Height
	blockTop := objs[1].Position().Y
	assert.GreaterOrEqual(t, blockTop, inlineBottom,
		"block must start below the trailing inline run, not overlap it")
}
