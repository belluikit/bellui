package widget

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// rowTexts returns the visible text objects rt draws, in row order, whichever
// path laid them out: the dense one, the dense one inside a scroller, or the
// lazy window.
func rowTexts(t *testing.T, rt *RichText) []*canvas.Text {
	t.Helper()
	r := test.TempWidgetRenderer(t, rt).(*textRenderer)
	objs := r.origObjs
	if len(objs) == 0 && rt.scr != nil {
		objs = rt.scr.Content.(*bellui.Container).Objects[1].(*bellui.Container).Objects
	}
	if len(objs) == 0 {
		objs = r.Objects()
	}
	var out []*canvas.Text
	for _, o := range objs {
		if tx, ok := o.(*canvas.Text); ok && tx.Visible() && strings.TrimSpace(tx.Text) != "" {
			out = append(out, tx)
		}
	}
	return out
}

// A selectable RichText row's carets sit on its glyphs whatever the row's
// alignment and indent, on the dense and the lazy path (#1441). The caret
// offset read the alignment from a widget-wide field that a RichText never
// set, so a trailing row's carets sat at the leading edge (about 250 px from
// "Hello" in a 300 px window), and it left out a blockquote's indent (16 px
// left of the text).
func TestRichText_CaretsFollowRowAlignmentAndIndent(t *testing.T) {
	test.NewTempApp(t)
	trailing, centre := RichTextStyleParagraph, RichTextStyleParagraph
	trailing.Alignment = bellui.TextAlignTrailing
	centre.Alignment = bellui.TextAlignCenter
	styles := []struct {
		name  string
		style RichTextStyle
	}{
		{"leading", RichTextStyleParagraph},
		{"trailing", trailing},
		{"centre", centre},
		{"blockquote", RichTextStyleBlockquote},
	}
	for _, virt := range []bool{false, true} {
		for _, st := range styles {
			where := fmt.Sprintf("%s, lazy path %v", st.name, virt)
			rt := NewRichText(&TextSegment{Style: st.style, Text: "Hello"})
			rt.Selectable = true
			rt.Scroll = widget.ScrollVerticalOnly
			rt.Virtualize = virt
			w := test.NewWindow(rt)
			w.Resize(geo.NewSize(300, 120))
			require.Equal(t, virt, rt.lazy, where)

			texts := rowTexts(t, rt)
			require.Len(t, texts, 1, where)
			pad := rt.Theme().Size(theme.SizeNameInnerPadding)
			size := rt.Theme().Size(theme.SizeNameText)
			for _, col := range []int{0, 5} {
				caret := rt.lineSizeToColumn(col, 0, size, pad, true).Width + rt.selection.alignOffsetX(0)
				want := texts[0].Position().X + bellui.MeasureText(texts[0].Text[:col], texts[0].TextSize, texts[0].TextStyle).Width
				assert.InDelta(t, want, caret, 0.05, "%s: caret at column %d", where, col)
			}
			w.Close()
		}
	}
}

// RichEntry places its caret itself and hit-tests through the selection, so
// both follow the row's alignment and indent too (#1441): a tap on a glyph
// puts the caret on it.
func TestRichEntry_CaretFollowsRowAlignmentAndIndent(t *testing.T) {
	test.NewTempApp(t)
	trailing := normalStyle()
	trailing.Alignment = bellui.TextAlignTrailing
	for _, st := range []struct {
		name  string
		style RichTextStyle
	}{{"leading", normalStyle()}, {"trailing", trailing}, {"blockquote", RichTextStyleBlockquote}} {
		e := NewRichEntry()
		e.SetSegments([]RichTextSegment{&TextSegment{Style: st.style, Text: "Hello"}})
		w := test.NewWindow(e)
		w.Resize(geo.NewSize(300, 120))

		texts := rowTexts(t, &e.provider)
		require.Len(t, texts, 1, st.name)
		border := e.Theme().Size(theme.SizeNameInputBorder)
		for col := 0; col <= 5; col++ {
			pos, _ := e.caretRectAt(0, col, false)
			want := texts[0].Position().X + bellui.MeasureText(texts[0].Text[:col], texts[0].TextSize, texts[0].TextStyle).Width
			assert.InDelta(t, want, pos.X+border/2, 0.05, "%s: caret at column %d", st.name, col)

			// A point just right of the caret resolves to the same column.
			_, y := e.provider.RowGeometry(0)
			row, got, _ := e.sel.getRowCol(geo.NewPos(pos.X+border/2+0.5, y/2+2))
			assert.Equal(t, 0, row, st.name)
			assert.Equal(t, col, got, "%s: a tap at column %d's caret", st.name, col)
		}
		w.Close()
	}
}

// The lazy path draws a blockquote at its indent and wraps it narrower by it,
// into the rows the dense path makes (#1441). It used to draw every row at the
// unindented edge and wrap at the full width: no row overran, but the quote
// had no indent there.
func TestRichText_LazyBlockquoteWrapsAndDrawsAtItsIndent(t *testing.T) {
	const text = "A blockquote holds a long quotation that wraps over several rows, so every row after the first is measured at some width and drawn at another, and the last word of a full row shows where it ends."
	for _, th := range []bellui.Theme{theme.DefaultTheme(), wideIndentTheme{theme.DefaultTheme()}} {
		app := test.NewTempApp(t)
		app.Settings().SetTheme(th)
		mk := func(virt bool) (*RichText, bellui.Window) {
			rt := NewRichText(&TextSegment{Style: RichTextStyleBlockquote, Text: text})
			rt.Wrapping = bellui.TextWrapWord
			rt.Scroll = widget.ScrollVerticalOnly
			rt.Virtualize = virt
			return rt, test.NewWindow(rt)
		}
		lazy, lw := mk(true)
		dense, dw := mk(false)
		indent := blockquoteIndent(lazy.Theme())
		pad := lazy.Theme().Size(theme.SizeNameInnerPadding)
		for width := float32(120); width <= 500; width++ {
			lw.Resize(geo.NewSize(width+20, 1000))
			dw.Resize(geo.NewSize(width+20, 1000))
			require.True(t, lazy.lazy, "the lazy path must be taken")
			require.False(t, dense.lazy)
			lr, dr := richRowStrings(lazy), richRowStrings(dense)
			require.Greater(t, len(lr), 1, "the text must wrap")
			require.True(t, slices.Equal(dr, lr), "width %v: lazy rows %q, dense rows %q", width, lr, dr)
			for _, tx := range rowTexts(t, lazy) {
				require.InDelta(t, pad+indent, tx.Position().X, 0.01, "width %v: row %q", width, tx.Text)
				end := tx.Position().X + bellui.MeasureText(strings.TrimRight(tx.Text, " "), tx.TextSize, tx.TextStyle).Width
				require.LessOrEqual(t, end, lazy.Size().Width-pad+0.5, "width %v: row %q ends past the right edge", width, tx.Text)
			}
		}
		lw.Close()
		dw.Close()
	}
}

// An unwrapped blockquote's minimum width counts its indent on every path, so
// the widget asks for room for the text where it is drawn (#1441). Counting
// the text's width alone, a blockquote sized to its minimum was drawn from 24
// to 222.8 in a widget 214.8 wide.
func TestRichText_BlockquoteMinWidthCountsItsIndent(t *testing.T) {
	test.NewTempApp(t)
	const quote = "A quoted line that is not wrapped and is the widest line of all"
	measure := func(rt *RichText) float32 {
		seg := &TextSegment{Style: RichTextStyleBlockquote, Text: quote}
		return bellui.MeasureText(quote, seg.size(), seg.Style.TextStyle).Width
	}
	many := func() []RichTextSegment {
		var segs []RichTextSegment
		for i := range 2200 {
			segs = append(segs, &TextSegment{Style: RichTextStyleParagraph, Text: fmt.Sprintf("para %d", i)})
			if i == 3 {
				segs = append(segs, &TextSegment{Style: RichTextStyleBlockquote, Text: quote})
			}
		}
		return segs
	}
	paths := []struct {
		name  string
		build func() *RichText
		check func(*RichText) bool
	}{
		{"dense", func() *RichText {
			return NewRichText(&TextSegment{Style: RichTextStyleBlockquote, Text: quote})
		}, func(rt *RichText) bool { return !rt.lazy && !rt.blockLazy && !rt.varHeight }},
		{"lazy", func() *RichText {
			rt := NewRichText(&TextSegment{Style: RichTextStyleBlockquote, Text: quote})
			rt.virtualize = true
			return rt
		}, func(rt *RichText) bool { return rt.lazy }},
		{"windowed blocks", func() *RichText {
			rt := NewRichText(many()...)
			rt.virtualize = true
			return rt
		}, func(rt *RichText) bool { return rt.blockLazy }},
		{"folded (variable height)", func() *RichText {
			rt := NewRichText(many()...)
			rt.virtualize = true
			rt.Resize(geo.NewSize(400, 600))
			rt.CreateRenderer()
			rt.updateRowBounds()
			rt.setHiddenRanges([]LineRange{{Start: 0, End: 1}})
			return rt
		}, func(rt *RichText) bool { return rt.varHeight }},
	}
	for _, p := range paths {
		rt := p.build()
		rt.Resize(geo.NewSize(400, 600))
		rt.CreateRenderer()
		rt.updateRowBounds()
		require.True(t, p.check(rt), "%s: the path must be taken", p.name)
		pad := rt.Theme().Size(theme.SizeNameInnerPadding)
		want := pad + blockquoteIndent(rt.Theme()) + measure(rt) + pad
		got := cache.Renderer(rt).MinSize().Width
		assert.InDelta(t, want, got, 0.5, "%s: minimum width", p.name)
	}
}

// A blockquote that the variable-height path's width sample misses widens the
// minimum width by its indent too when it scrolls into view
// (widenVirtualWidthForWindow, #1441).
func TestRichText_BlockquoteScrolledInWidensByItsIndent(t *testing.T) {
	test.NewTempApp(t)
	const quote = "A quoted line that is not wrapped and is the widest line of all"
	const paras = 2200
	n := paras + 1
	step := float64(n) / float64(virtualWidthSampleRows)
	sampled := map[int]bool{}
	for k := range virtualWidthSampleRows {
		sampled[int(float64(k)*step)] = true
	}
	row := 1500
	for sampled[row] {
		row++
	}
	var segs []RichTextSegment
	for i := range paras {
		if i == row {
			segs = append(segs, &TextSegment{Style: RichTextStyleBlockquote, Text: quote})
		}
		segs = append(segs, &TextSegment{Style: RichTextStyleParagraph, Text: fmt.Sprintf("para %d", i)})
	}
	rt := NewRichText(segs...)
	rt.virtualize = true
	rt.Resize(geo.NewSize(400, 600))
	rt.CreateRenderer()
	rt.updateRowBounds()
	rt.setHiddenRanges([]LineRange{{Start: 0, End: 1}})
	require.True(t, rt.varHeight)
	require.Len(t, rt.rowBounds, n)
	require.Equal(t, quote, richRowStrings(rt)[row])

	seg := &TextSegment{Style: RichTextStyleBlockquote, Text: quote}
	pad := rt.Theme().Size(theme.SizeNameInnerPadding)
	want := pad + blockquoteIndent(rt.Theme()) + bellui.MeasureText(quote, seg.size(), seg.Style.TextStyle).Width + pad
	r := cache.Renderer(rt)
	require.Less(t, r.MinSize().Width, want-1, "the sample must miss the blockquote")

	y, _ := rt.RowGeometry(row)
	rt.SetVisibleRange(y, 600)
	r.Refresh()
	require.Contains(t, rt.winRows, row)
	assert.InDelta(t, want, r.MinSize().Width, 0.5)
}
