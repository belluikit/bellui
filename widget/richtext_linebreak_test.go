package widget

import (
	"fmt"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// The marks #1300 names, checked against the tables on their own so a table
// edit cannot drop one unnoticed.
func TestCJKLineBreakTables(t *testing.T) {
	for _, r := range "、，。）：；！？」』】〕’”・々" {
		assert.True(t, unicode.Is(cjkNoLineStart, r), "%q may not start a line", r)
		assert.False(t, unicode.Is(cjkNoLineEnd, r), "%q may end a line", r)
	}
	for _, r := range "（「『【〔“‘" {
		assert.True(t, unicode.Is(cjkNoLineEnd, r), "%q may not end a line", r)
		assert.False(t, unicode.Is(cjkNoLineStart, r), "%q may start a line", r)
	}
	for _, r := range "昆比亚あア한aZ0,.!?()" {
		assert.False(t, unicode.Is(cjkNoLineStart, r), "%q is not in the start table", r)
		assert.False(t, unicode.Is(cjkNoLineEnd, r), "%q is not in the end table", r)
	}
}

// cjkWrapText is the DaySong label of #1300 with every mark of the issue's
// done-when added, opening and closing.
const cjkWrapText = "Choro、昆比亚、萨尔萨、雷鬼、桑巴、探戈。（流行）“摇滚”！真的？是：对；好，「爵士」『蓝调』【古典】〔民谣〕‘乡村’。"

func assertCJKRows(t *testing.T, rows []string, where string) {
	t.Helper()
	for _, row := range rows {
		if row == "" {
			continue
		}
		r := []rune(row)
		assert.False(t, unicode.Is(cjkNoLineStart, r[0]), "%s: row %q starts with %q", where, row, r[0])
		assert.False(t, unicode.Is(cjkNoLineEnd, r[len(r)-1]), "%s: row %q ends with %q", where, row, r[len(r)-1])
	}
}

// A word-wrapped CJK line never starts with closing punctuation or ends with
// an opening mark, at any width, and no text is lost or doubled (#1300).
func TestLineBounds_WrapWordCJKPunctuation(t *testing.T) {
	measurer := func(text []rune) geo.Size { return geo.NewSize(float32(len(text))*10, 10) }
	text := []rune(cjkWrapText)
	for w := float32(20); w <= float32(len(text))*10; w += 10 {
		maxSize := geo.NewSize(w, 100000)
		seg := &TextSegment{Text: cjkWrapText}
		bounds, _ := lineBounds(seg, bellui.TextWrapWord, bellui.TextTruncateOff, w, maxSize, measurer)

		var rows []string
		joined := ""
		for _, b := range bounds {
			row := string(text[b.begin:b.end])
			rows = append(rows, row)
			joined += row
		}
		require.Equal(t, cjkWrapText, joined, "width %v: rows must hold the text exactly once", w)
		assertCJKRows(t, rows, fmt.Sprintf("width %v", w))
	}
}

// Latin text keeps its wrapping: a word wider than the line still breaks
// where it overflows, even before a mark that is not CJK.
func TestLineBounds_WrapWordLatinFallbackUnchanged(t *testing.T) {
	measurer := func(text []rune) geo.Size { return geo.NewSize(float32(len(text))*10, 10) }
	maxSize := geo.NewSize(50, 10000)

	seg := &TextSegment{Text: "aaaaa)bbbb"}
	bounds, _ := lineBounds(seg, bellui.TextWrapWord, bellui.TextTruncateOff, maxSize.Width, maxSize, measurer)

	text := []rune(seg.Text)
	var rows []string
	for _, b := range bounds {
		rows = append(rows, string(text[b.begin:b.end]))
	}
	assert.Equal(t, []string{"aaaaa", ")bbbb"}, rows)
}

// Widget-level #1300, through a Label as DaySong's Settings showed it, with
// the test font's real metrics.
func TestLabel_WrapWordCJKPunctuation(t *testing.T) {
	test.NewTempApp(t)

	l := NewLabel(cjkWrapText)
	l.Wrapping = bellui.TextWrapWord
	r := test.TempWidgetRenderer(t, l)
	text := []rune(cjkWrapText)

	wrapped := 0
	for w := float32(40); w <= 600; w += 5 {
		l.Resize(geo.NewSize(w, 2000))
		r.Refresh()
		var rows []string
		for _, b := range l.provider.rowBounds {
			rows = append(rows, string(text[b.begin:b.end]))
		}
		if len(rows) > 1 {
			wrapped++
		}
		assertCJKRows(t, rows, fmt.Sprintf("width %v", w))
	}
	require.Greater(t, wrapped, 50, "the sweep must wrap the text at most widths")
}

// A row of one character at a line's end, as a narrow label's last glyph is,
// is the end of the text: there is nothing after it to check.
func TestCJKLineBreak_AtTextEnd(t *testing.T) {
	assert.Equal(t, 1, cjkLineBreak([]rune("a"), 1))
	assert.Equal(t, 2, cjkLineBreak([]rune("好。"), 1), "。 stays with 好")
	assert.Equal(t, 1, cjkLineBreak([]rune("好好"), 1))
	assert.Equal(t, 2, cjkLineBreak([]rune("（好"), 1), "（ goes down with 好")
}
