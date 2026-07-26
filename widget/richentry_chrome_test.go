package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func richEntryChrome(t *testing.T, e *RichEntry) *richEntryRenderer {
	t.Helper()
	r, ok := e.CreateRenderer().(*richEntryRenderer)
	require.True(t, ok)
	e.Resize(geo.NewSize(200, 40))
	r.Layout(geo.NewSize(200, 40))
	r.Refresh()
	return r
}

// TestRichEntry_ChromeFilledDefault verifies the default (filled) RichEntry now
// draws MD3 chrome: a filled rounded background box behind the content and a
// bordered outline drawn on top (#749) — no longer a bare underline.
func TestRichEntry_ChromeFilledDefault(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	r := richEntryChrome(t, e)

	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	assert.Equal(t, th.Color(theme.ColorNameInputBackground, v), r.box.FillColor, "filled style fills the box")
	assert.Positive(t, r.box.CornerRadius, "rounded corners")
	assert.Equal(t, th.Color(theme.ColorNameInputBorder, v), r.border.StrokeColor, "unfocused border color")

	objs := r.Objects()
	assert.Same(t, r.box, objs[0], "box is drawn first (behind content)")
	assert.Same(t, r.border, objs[len(objs)-1], "border is drawn last (on top)")
}

// TestRichEntry_ChromeFocusBorder verifies the border adopts the primary color
// while focused and reverts when focus is lost.
func TestRichEntry_ChromeFocusBorder(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	r := richEntryChrome(t, e)
	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	e.focused = true
	r.Refresh()
	assert.Equal(t, th.Color(theme.ColorNamePrimary, v), r.border.StrokeColor, "focused border is primary")

	e.focused = false
	r.Refresh()
	assert.Equal(t, th.Color(theme.ColorNameInputBorder, v), r.border.StrokeColor, "unfocused border reverts")
}

// TestRichEntry_ChromeOutlined verifies the outlined style is transparent-filled
// with an outline-colored border.
func TestRichEntry_ChromeOutlined(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.Style = EntryStyleOutlined
	r := richEntryChrome(t, e)
	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	assert.Equal(t, color.Transparent, r.box.FillColor, "outlined style has no fill")
	assert.Equal(t, th.Color(theme.ColorNameOutline, v), r.border.StrokeColor, "outlined border color")
}

// TestRichEntry_EmptyFieldCaretAtLeadingEdge is the #749 caret fix: an empty
// RichEntry places its caret at the leading content edge (where col 0 of a text
// row renders), not stranded at the trailing edge.
func TestRichEntry_EmptyFieldCaretAtLeadingEdge(t *testing.T) {
	test.NewTempApp(t)

	e := NewRichEntry()
	e.CreateRenderer()
	e.Resize(geo.NewSize(300, 40))
	e.FocusGained()
	emptyPos, emptySize := e.caretRect()

	// The caret before the first glyph of a one-char doc is the leading edge.
	ref := NewRichEntry()
	ref.CreateRenderer()
	ref.Resize(geo.NewSize(300, 40))
	ref.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "x"}})
	ref.moveCaret(0, false)
	refPos, _ := ref.caretRect()

	assert.Equal(t, refPos.X, emptyPos.X, "empty-field caret sits at the leading content edge")
	assert.Less(t, emptyPos.X, float32(50), "caret is near the leading edge, not the trailing edge")
	assert.Positive(t, emptySize.Height, "caret has a visible height")
}

// TestRichEntry_BlankLineCaretAtLeadingEdge extends the fix to a blank line in
// the middle of a document (a bare '\n' row has no glyphs either).
func TestRichEntry_BlankLineCaretAtLeadingEdge(t *testing.T) {
	test.NewTempApp(t)

	e := NewRichEntry()
	e.CreateRenderer()
	e.Resize(geo.NewSize(300, 60))
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "a\n\nb"}})
	e.moveCaret(2, false) // start of the empty middle row

	row, _ := e.doc.offsetToRowCol(e.caret)
	require.Equal(t, 0, e.provider.rowLength(row), "the middle row is empty")
	pos, _ := e.caretRect()
	assert.Less(t, pos.X, float32(50), "blank-line caret sits at the leading edge")
}
