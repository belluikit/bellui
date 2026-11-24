package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// A segment whose first row is narrowed with nothing before it, as a
// blockquote's is by its left padding, never leaves that row empty: neither a
// rune too wide for it nor a word that fits only a full row moves the segment
// down a row (#1435). Following an earlier segment, both still do.
func TestLineBounds_NarrowedFirstRowWithoutContentIsNotLeftEmpty(t *testing.T) {
	measurer := func(text []rune) geo.Size { return geo.NewSize(float32(len(text))*10, 10) }
	maxSize := geo.NewSize(100, 1000)
	for _, c := range []struct {
		name       string
		wrap       bellui.TextWrap
		text       string
		firstWidth float32
	}{
		{"word wrap, rune too wide", bellui.TextWrapWord, "W", 5},
		{"word wrap, word fits only a full row", bellui.TextWrapWord, "abcdefghi is a word", 85},
		{"word wrap, no space and the rest fits a full row", bellui.TextWrapWord, "abcdefghi", 85},
		{"break wrap, rune too wide", bellui.TextWrapBreak, "W", 5},
	} {
		seg := &TextSegment{Text: c.text}
		fresh, _ := lineBoundsAfter(seg, c.wrap, bellui.TextTruncateOff, c.firstWidth, maxSize, measurer, false)
		require.NotEmpty(t, fresh, c.name)
		assert.Greater(t, fresh[0].end, fresh[0].begin, "%s: a first row with nothing before it keeps what it can", c.name)

		after, _ := lineBoundsAfter(seg, c.wrap, bellui.TextTruncateOff, c.firstWidth, maxSize, measurer, true)
		require.Greater(t, len(after), 1, c.name)
		assert.Equal(t, after[0].begin, after[0].end, "%s: after earlier content the segment starts on the next row", c.name)
	}
}

// A blockquote's first row is never empty at any width, in word and in
// character wrapping (#1435: a word that fit a full row but not the padded
// first row left it empty, and so did a rune too wide for it).
func TestRichText_BlockquoteFirstRowNeverEmpty(t *testing.T) {
	test.NewTempApp(t)
	for _, wrap := range []bellui.TextWrap{bellui.TextWrapWord, bellui.TextWrapBreak} {
		for _, text := range []string{"Supercalifragilisticexpialidocious is a word", "W"} {
			rt := NewRichText(&TextSegment{Style: RichTextStyleBlockquote, Text: text})
			rt.Wrapping = wrap
			w := test.NewWindow(rt)
			for width := float32(10); width <= 400; width++ {
				w.Resize(geo.NewSize(width+40, 400))
				rt.Resize(geo.NewSize(width, 300))
				rows := richRowStrings(rt)
				require.NotEmpty(t, rows)
				assert.NotEmpty(t, rows[0], "wrap %v, %q at width %v: rows %q", wrap, text, width, rows)
			}
			w.Close()
		}
	}
}
