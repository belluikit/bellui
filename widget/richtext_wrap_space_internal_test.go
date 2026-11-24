package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// richRowStrings returns the text of each laid-out row of rt, joining the
// slices of every segment the row holds.
func richRowStrings(rt *RichText) []string {
	var out []string
	for r := 0; r < rt.rows(); r++ {
		b := rt.rowBoundary(r)
		var sb strings.Builder
		last := len(b.segments) - 1
		for i, seg := range b.segments {
			ts, ok := seg.(*TextSegment)
			if !ok {
				continue
			}
			runes := []rune(ts.Text)
			lo, hi := 0, len(runes)
			if i == 0 {
				lo = b.begin
			}
			if i == last {
				hi = b.end
			}
			if hi < lo {
				hi = lo
			}
			sb.WriteString(string(runes[lo:hi]))
		}
		out = append(out, sb.String())
	}
	return out
}

// assertWrapRows checks that no wrapped row starts with a space and that the
// rows, with one space for each break, give the text back: nothing is lost or
// doubled.
func assertWrapRows(t *testing.T, rows []string, text, where string) {
	t.Helper()
	for i, row := range rows {
		if i > 0 && row != "" {
			assert.False(t, breakableSpace([]rune(row)[0]), "%s: row %d %q starts with a space; rows %q", where, i, row, rows)
		}
	}
	var joined []string
	for _, row := range rows {
		if row != "" {
			joined = append(joined, strings.TrimRight(row, " "))
		}
	}
	assert.Equal(t, strings.Join(strings.Fields(text), " "), strings.Join(joined, " "), "%s: rows %q", where, rows)
}

var wrapSpaceCases = []struct {
	name string
	segs []string
}{
	// The break inside one segment drops its space; this was always right.
	{"inside a segment", []string{"Select a word and make it bold, italic or code."}},
	// The next segment starts with the space the row breaks at (#1423).
	{"segment starts with the space", []string{"Select a word and make it bold, italic", " or code."}},
	// The previous segment ends with it.
	{"segment ends with the space", []string{"Select a word and make it bold, italic ", "or code."}},
	{"right to left", []string{"שלום עולם זה טקסט ארוך מאוד", " שעובר לשורה הבאה בסוף."}},
}

// A word-wrapped RichText never starts a row with the space it broke at,
// whether the break falls inside a segment or where one segment meets the
// next, at every width the text wraps at (#1423).
func TestRichText_WrapNeverStartsRowWithBreakSpace(t *testing.T) {
	test.NewTempApp(t)
	for _, c := range wrapSpaceCases {
		t.Run(c.name, func(t *testing.T) {
			var segs []RichTextSegment
			for _, s := range c.segs {
				segs = append(segs, &TextSegment{Style: RichTextStyleInline, Text: s})
			}
			rt := NewRichText(segs...)
			rt.Wrapping = bellui.TextWrapWord
			w := test.NewWindow(rt)
			defer w.Close()
			text := strings.Join(c.segs, "")
			wrapped := 0
			for width := float32(120); width <= 400; width += 2 {
				w.Resize(geo.NewSize(width+20, 400))
				rt.Resize(geo.NewSize(width, 300))
				rows := richRowStrings(rt)
				if len(rows) > 1 {
					wrapped++
				}
				assertWrapRows(t, rows, text, c.name)
			}
			assert.Greater(t, wrapped, 10, "the text must wrap at many of the widths")
		})
	}
}

// Split anywhere at a space, two segments of one style lay out the rows the
// same text does as one segment: the split neither pushes the next word down a
// row with its space in front nor cuts it where the leftover ends (#1423).
func TestRichText_WrapSplitAtSpaceMatchesOneSegment(t *testing.T) {
	test.NewTempApp(t)
	const text = "Select a word and make it bold, italic or code."
	whole := NewRichText(&TextSegment{Style: RichTextStyleInline, Text: text})
	whole.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(whole)
	defer w.Close()
	w.Resize(geo.NewSize(500, 400))
	for width := float32(160); width <= 340; width += 4 {
		whole.Resize(geo.NewSize(width, 300))
		want := richRowStrings(whole)
		for i, r := range text {
			if r != ' ' {
				continue
			}
			for _, cut := range []int{i, i + 1} { // the space leads the second segment, or ends the first
				split := NewRichText(
					&TextSegment{Style: RichTextStyleInline, Text: text[:cut]},
					&TextSegment{Style: RichTextStyleInline, Text: text[cut:]},
				)
				split.Wrapping = bellui.TextWrapWord
				split.Resize(geo.NewSize(width, 300))
				got := richRowStrings(split)
				for j := range got {
					got[j] = strings.TrimRight(got[j], " ")
				}
				assert.Equal(t, want, got, "width %v, split %q | %q", width, text[:cut], text[cut:])
			}
		}
	}
}

// When even one rune of the next segment does not fit what is left of the
// row, the segment starts on the next row, and not with its leading space.
func TestLineBounds_WrapWordLeftoverSkipsLeadingSpace(t *testing.T) {
	measurer := func(text []rune) geo.Size { return geo.NewSize(float32(len(text))*10, 10) }
	seg := &TextSegment{Text: " or code."}
	bounds, _ := lineBounds(seg, bellui.TextWrapWord, bellui.TextTruncateOff, 5, geo.NewSize(200, 1000), measurer)
	require.Len(t, bounds, 2)
	assert.Equal(t, 0, bounds[0].begin)
	assert.Equal(t, 0, bounds[0].end, "nothing of the segment fits the leftover")
	assert.Equal(t, 1, bounds[1].begin, "the next row starts after the space")
	assert.Equal(t, 9, bounds[1].end)
}

// A word of the next segment that does not fit what is left of the row moves
// whole to the next row instead of being cut where the leftover ends; CJK
// text, which breaks between any two characters, still fills the leftover.
func TestLineBounds_WrapWordLeftoverKeepsWordWhole(t *testing.T) {
	measurer := func(text []rune) geo.Size { return geo.NewSize(float32(len(text))*10, 10) }
	maxSize := geo.NewSize(100, 1000)

	seg := &TextSegment{Text: "make it bold"}
	bounds, _ := lineBounds(seg, bellui.TextWrapWord, bellui.TextTruncateOff, 30, maxSize, measurer)
	require.NotEmpty(t, bounds)
	assert.Equal(t, 0, bounds[0].end, "\"mak\" fits the leftover, but the word moves down whole")
	assert.Equal(t, 0, bounds[1].begin)
	assert.Equal(t, 7, bounds[1].end, "the next row holds \"make it\"")

	// Its first "word", up to the space, would fit a full row, so only the CJK
	// rule keeps it in the leftover.
	cjk := &TextSegment{Text: "昆比亚萨尔 萨雷鬼桑巴探戈"}
	bounds, _ = lineBounds(cjk, bellui.TextWrapWord, bellui.TextTruncateOff, 30, maxSize, measurer)
	require.NotEmpty(t, bounds)
	assert.Equal(t, 3, bounds[0].end, "CJK fills the leftover")
}

// A word-wrapped RichEntry rows and caret positions follow: the dropped space
// sits between the rows, the caret before it ends the first row and the one
// after it starts the second, and no position is lost (#1423).
func TestRichEntry_WrapAtSegmentSpace(t *testing.T) {
	test.NewTempApp(t)
	first := "Select a word and make it bold, italic"
	e := NewRichEntry()
	e.Wrapping = bellui.TextWrapWord
	// The second run is italic: RichEntry merges runs of one style, so a
	// same-style split would be a single segment.
	italic := RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Italic: true}}
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: first},
		&TextSegment{Style: italic, Text: " or code."},
	})
	w := test.NewWindow(e)
	defer w.Close()
	require.Len(t, e.provider.Segments, 2, "the runs stay two segments")

	found := false
	for width := float32(200); width <= 360 && !found; width += 2 {
		w.Resize(geo.NewSize(width, 300))
		rows := richRowStrings(&e.provider)
		assertWrapRows(t, rows, first+" or code.", "RichEntry")
		if len(rows) != 2 || rows[0] != first {
			continue
		}
		found = true
		model := e.doc.activeRowModel()
		require.Len(t, model, 2)
		space := len([]rune(first)) // the position of the dropped space
		assert.Equal(t, space, model[0].end)
		assert.Equal(t, space+1, model[1].start)
		row, col := e.doc.offsetToRowCol(space)
		assert.Equal(t, 0, row, "the caret before the space ends the first row")
		assert.Equal(t, space, col)
		row, col = e.doc.offsetToRowCol(space + 1)
		assert.Equal(t, 1, row, "the caret after the space starts the second row")
		assert.Equal(t, 0, col)
		assert.Equal(t, space+1, e.doc.rowColToOffset(1, 0))
		assert.Equal(t, e.doc.len(), model[1].end, "the last row reaches the end")
	}
	require.True(t, found, "some width must break exactly at the segment boundary")
}
