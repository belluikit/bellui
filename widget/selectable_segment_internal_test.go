package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

// selectRange forces the given selection columns on a RichText's selection
// overlay and returns the copied text, bypassing pointer interaction.
func selectRange(rt *RichText, rowA, colA, rowB, colB int) string {
	sel := rt.selection
	sel.selecting = true
	sel.selectRow, sel.selectColumn = rowA, colA
	sel.cursorRow, sel.cursorColumn = rowB, colB
	return sel.SelectedText()
}

// TestSelectable_CopyIncludesInlineSegment verifies that selecting across an
// inline non-text segment (a HyperlinkSegment) copies that segment's Textual()
// content, so the copied text matches the highlighted rectangles (pre-rebrand issue #46).
func TestSelectable_CopyIncludesInlineSegment(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt := NewRichText(
		&TextSegment{Text: "see ", Style: RichTextStyleInline},
		&HyperlinkSegment{Text: "docs"},
		&TextSegment{Text: " now", Style: RichTextStyleInline},
	)
	rt.Wrapping = bellui.TextWrapOff
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(320, 80))

	// The text-only row is "see  now" (8 runes); the link's opaque box trails
	// rune index 3 (the trailing space of "see ").
	rowLen := rt.rowLength(0)
	assert.Equal(t, 8, rowLen)

	// Selecting the whole row splices the link text in logical order.
	assert.Equal(t, "see docs now", selectRange(rt, 0, 0, 0, rowLen))

	// A selection ending exactly at the box owner (col 4 covers rune 3) still
	// includes the link, matching the filled rectangle.
	assert.Equal(t, "see docs", selectRange(rt, 0, 0, 0, 4))

	// A selection that stops before the box owner excludes the link, matching the
	// unfilled rectangle.
	assert.Equal(t, "see", selectRange(rt, 0, 0, 0, 3))

	// A selection after the box excludes the link.
	assert.Equal(t, "now", selectRange(rt, 0, 5, 0, 8))
}

// TestSelectable_CopyIncludesInlineSegmentRTL checks the same splicing in an RTL
// row: the copied text is in logical order and includes the inline link.
func TestSelectable_CopyIncludesInlineSegmentRTL(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt := NewRichText(
		&TextSegment{Text: "אבג ", Style: RichTextStyleInline},
		&HyperlinkSegment{Text: "lnk"},
		&TextSegment{Text: " דהו", Style: RichTextStyleInline},
	)
	rt.Wrapping = bellui.TextWrapOff
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(320, 80))

	rowLen := rt.rowLength(0)
	// Whole-row selection yields logical order with the link spliced in.
	assert.Equal(t, "אבג lnk דהו", selectRange(rt, 0, 0, 0, rowLen))
}
