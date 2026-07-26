package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// TestRichEntry_ClickMovesCaret drives a click through the canvas hit-test (as a
// real pointer does), not by calling RichEntry.Tapped directly. It is the
// regression guard for the bug where the embedded selection layer (a child
// object that implements the pointer interfaces) shadowed RichEntry's own
// handlers, so clicking never moved the caret — only keyboard navigation did.
func TestRichEntry_ClickMovesCaret(t *testing.T) {
	test.NewTempApp(t)

	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "line one\nsecond line here"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 200))
	c := w.Canvas()

	// Resolve the pixel position of a known offset on row 1 via caretRect, then
	// convert to canvas coordinates.
	target := e.doc.rowColToOffset(1, 6) // "second| line here"
	e.caret, e.anchor = target, target
	e.syncSel()
	cr, sz := e.caretRect()
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(&e.provider)
	tapPos := geo.NewPos(abs.X+cr.X+1, abs.Y+cr.Y+sz.Height/2)

	// Reset the caret to 0 and tap where offset `target` sits.
	e.caret, e.anchor = 0, 0
	e.syncSel()
	test.TapCanvas(c, tapPos)

	assert.Equal(t, target, e.caret, "a click must move the caret to the tapped position")
	row, _ := e.doc.offsetToRowCol(e.caret)
	assert.Equal(t, 1, row)
}
