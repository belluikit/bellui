package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// row0GlyphsEnd returns where the glyphs of rt's first row end: the right edge
// of its last visible character, a trailing space not counted, in the
// alignment its text object draws with inside its box.
func row0GlyphsEnd(t *testing.T, rt *RichText) float32 {
	t.Helper()
	end := float32(-1)
	y0 := float32(-1)
	for _, o := range test.TempWidgetRenderer(t, rt).Objects() {
		tx, ok := o.(*canvas.Text)
		if !ok || !tx.Visible() {
			continue
		}
		if y0 < 0 {
			y0 = tx.Position().Y
		}
		if tx.Position().Y != y0 {
			continue
		}
		trimmed := strings.TrimRight(tx.Text, " ")
		if trimmed == "" {
			continue
		}
		full := bellui.MeasureText(tx.Text, tx.TextSize, tx.TextStyle).Width
		glyphs := bellui.MeasureText(trimmed, tx.TextSize, tx.TextStyle).Width
		x := tx.Position().X
		var e float32
		switch tx.Alignment {
		case bellui.TextAlignTrailing:
			e = x + tx.Size().Width - (full - glyphs)
		case bellui.TextAlignCenter:
			e = x + (tx.Size().Width-full)/2 + glyphs
		default:
			e = x + glyphs
		}
		if e > end {
			end = e
		}
	}
	return end
}

// A row that wraps where one segment ends with a space, trailing or centred,
// ends its glyphs where the same text in one segment does: the break space is
// not counted in its width (#1435). Selection alignment (rowWidth) agrees.
func TestRichText_BreakSpaceAtSegmentEndNotAligned(t *testing.T) {
	test.NewTempApp(t)
	const first, second = "Select a word and make it bold, italic ", "or code."
	for _, align := range []bellui.TextAlign{bellui.TextAlignTrailing, bellui.TextAlignCenter} {
		st := RichTextStyleInline
		st.Alignment = align
		split := NewRichText(&TextSegment{Style: st, Text: first}, &TextSegment{Style: st, Text: second})
		whole := NewRichText(&TextSegment{Style: st, Text: first + second})
		split.Wrapping, whole.Wrapping = bellui.TextWrapWord, bellui.TextWrapWord
		w := test.NewWindow(&bellui.Container{Objects: []core.CanvasObject{split, whole}})
		w.Resize(geo.NewSize(600, 600))

		compared := 0
		for width := float32(246); width <= 300; width += 0.5 {
			split.Resize(geo.NewSize(width, 200))
			whole.Resize(geo.NewSize(width, 200))
			sRows, wRows := richRowStrings(split), richRowStrings(whole)
			require.NotEmpty(t, sRows)
			require.NotEmpty(t, wRows)
			if sRows[0] != first || wRows[0] != strings.TrimRight(first, " ") {
				continue // this width does not break at the segment boundary
			}
			compared++
			assert.InDelta(t, row0GlyphsEnd(t, whole), row0GlyphsEnd(t, split), 0.5,
				"align %v, width %v: the split row's glyphs end where the whole row's do", align, width)
			textSize := split.Theme().Size(theme.SizeNameText)
			assert.InDelta(t, whole.rowWidth(0, textSize), split.rowWidth(0, textSize), 0.5,
				"align %v, width %v: rowWidth leaves the break space out", align, width)
		}
		assert.Greater(t, compared, 10, "align %v: the boundary break must be measured at many widths", align)
		w.Close()
	}
}

// A space before a hard line break is text, not a wrap: split there, the row
// keeps its width, as the same text in one segment does.
func TestRichText_SpaceBeforeHardBreakStaysInRowWidth(t *testing.T) {
	test.NewTempApp(t)
	st := RichTextStyleInline
	st.Alignment = bellui.TextAlignTrailing
	for _, wrap := range []bellui.TextWrap{bellui.TextWrapOff, bellui.TextWrapWord} {
		split := NewRichText(&TextSegment{Style: st, Text: "Trailing text "}, &TextSegment{Style: st, Text: "\nnext line"})
		whole := NewRichText(&TextSegment{Style: st, Text: "Trailing text \nnext line"})
		split.Wrapping, whole.Wrapping = wrap, wrap
		w := test.NewWindow(&bellui.Container{Objects: []core.CanvasObject{split, whole}})
		w.Resize(geo.NewSize(600, 600))
		split.Resize(geo.NewSize(400, 200))
		whole.Resize(geo.NewSize(400, 200))
		require.Equal(t, "Trailing text ", richRowStrings(split)[0])
		textSize := split.Theme().Size(theme.SizeNameText)
		assert.InDelta(t, whole.rowWidth(0, textSize), split.rowWidth(0, textSize), 0.5, "wrap %v", wrap)
		w.Close()
	}
}

// The space stays in the row's text, so RichEntry caret offsets are those of
// #1423: the caret before the space ends the first row, the one after it
// starts the second (#1435).
func TestRichEntry_BreakSpaceAtSegmentEndKeepsOffsets(t *testing.T) {
	test.NewTempApp(t)
	const first = "Select a word and make it bold, italic "
	e := NewRichEntry()
	e.Wrapping = bellui.TextWrapWord
	italic := RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Italic: true}}
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: first},
		&TextSegment{Style: italic, Text: "or code."},
	})
	w := test.NewWindow(e)
	defer w.Close()
	require.Len(t, e.provider.Segments, 2)

	n := len([]rune(first))
	found := false
	for width := float32(200); width <= 360 && !found; width++ {
		w.Resize(geo.NewSize(width, 300))
		rows := richRowStrings(&e.provider)
		if len(rows) != 2 || rows[0] != first {
			continue
		}
		found = true
		model := e.doc.activeRowModel()
		require.Len(t, model, 2)
		assert.Equal(t, n, model[0].end, "the first row keeps its space")
		assert.Equal(t, n, model[1].start)
		row, col := e.doc.offsetToRowCol(n - 1)
		assert.Equal(t, 0, row, "the caret before the space is on the first row")
		assert.Equal(t, n-1, col)
		assert.Equal(t, n, e.doc.rowColToOffset(1, 0), "the second row starts after the space")
		assert.Equal(t, e.doc.len(), model[1].end)
	}
	require.True(t, found, "some width must break at the segment boundary")
}
