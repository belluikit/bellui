package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// tallInlineSegment is a non-text inline RichTextSegment whose visual is taller
// than the surrounding text, standing in for an inline image taller than the row's
// text (issue #47). Its width is small so it shares a row with adjacent text.
type tallInlineSegment struct {
	height float32
}

func (s *tallInlineSegment) Inline() bool             { return true }
func (s *tallInlineSegment) Textual() string          { return "" }
func (s *tallInlineSegment) Update(core.CanvasObject) {}

func (s *tallInlineSegment) Visual() core.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(geo.NewSize(10, s.height))
	return r
}

func (s *tallInlineSegment) Select(geo.Position, geo.Position) {}
func (s *tallInlineSegment) SelectedText() string              { return "" }
func (s *tallInlineSegment) Unselect()                         {}

// TestRichText_MixedRow_CaretHeightIncludesNonText verifies that a row whose
// tallest element is a non-text segment reports that height, so rtlCaretX draws
// the caret at the full row height rather than the text height (issue #47).
func TestRichText_MixedRow_CaretHeightIncludesNonText(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	// RTL text so rtlCaretX engages (the bidi caret path #47 targets), plus an
	// inline non-text segment deliberately taller than the text.
	txt := &TextSegment{Text: "مرحبا", Style: RichTextStyle{Inline: true}}
	tall := &tallInlineSegment{height: 100}
	rich := NewRichText(txt, tall)
	rich.Resize(geo.NewSize(400, 200))
	rich.CreateRenderer()
	test.TempWidgetRenderer(t, rich) // force a layout pass so rowBounds exist

	require.Len(t, rich.rowBoundary(0).segments, 2, "text and box must share one row")

	textH := bellui.MeasureText("مرحبا", rich.Theme().Size(theme.SizeNameText), bellui.TextStyle{}).Height
	require.Greater(t, float32(100), textH, "the non-text segment must be taller than the text")

	assert.InDelta(t, float32(100), rich.rowVisualHeight(0), 0.5,
		"row height must include the taller non-text segment")

	_, h, ok := rich.rtlCaretX(0, 1, true)
	require.True(t, ok, "an RTL row mixing text and a box must resolve via the bidi layout")
	assert.InDelta(t, float32(100), h, 0.5,
		"caret height must equal the row height, not the text height")
}
