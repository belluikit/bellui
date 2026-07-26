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

// TestRichText_rowAtY_LazyAnalytic (#160) checks the lazy row-at-Y inverse is exact
// and O(1) — it returns the row whose uniform-stride band contains y without shaping
// any additional lines (the click/gutter-tap hot path on a huge document).
func TestRichText_rowAtY_LazyAnalytic(t *testing.T) {
	e, _ := newLargeWrappedEntry(t, 50000)
	p := &e.text
	require.True(t, p.lazy)

	s := p.stride()
	top0 := p.contentTopOffset()
	before := measuredLineCount(p)

	last := p.rows() - 1
	for _, row := range []int{0, 1, 50, 5000, last} {
		y := top0 + float32(row)*s + s/2 // middle of the row's band
		assert.Equal(t, row, p.rowAtY(y), "rowAtY for row %d", row)
	}
	// Clicks above the top and below the bottom clamp into range.
	assert.Equal(t, 0, p.rowAtY(top0-1000))
	assert.Equal(t, last, p.rowAtY(top0+float32(last+100)*s))

	assert.Equal(t, before, measuredLineCount(p),
		"rowAtY must not shape any additional lines (O(1), no full scan)")
}

// TestRichText_rowAtY_SkipsFolds (#160) checks the inverse maps onto the visible-row
// axis: folded rows collapse to zero height, so a y past a fold lands on the first
// visible row after it, matching the layout.
func TestRichText_rowAtY_SkipsFolds(t *testing.T) {
	e := newWrappedEntry(t) // wrapped, small (eager-measured)
	line0 := rowsOfLine(e, 0)
	require.GreaterOrEqual(t, len(line0), 2)

	e.HiddenRanges = []LineRange{{Start: 0, End: 1}} // fold the wrapped first line
	e.Refresh()

	p := &e.text
	s := p.stride()
	top0 := p.contentTopOffset()

	// The first visible row sits at the top now (the folded rows have zero height).
	first := p.firstVisibleAtOrAfter(0)
	assert.Equal(t, first, p.rowAtY(top0+s/2),
		"y at the top maps to the first visible row, skipping the fold")
	// None of the folded rows are ever returned by a Y hit-test.
	for _, r := range line0 {
		got := p.rowAtY(top0 + float32(r)*s + s/2)
		assert.False(t, p.rowHidden(got), "rowAtY must not return a folded row (%d)", got)
	}
}

// fullSelect drives a selectable into a whole-document selection without going
// through hit-testing, so the test exercises only SelectedText reconstruction.
func fullSelect(t *testing.T, prov *RichText, sel *focusSelectable) {
	t.Helper()
	sel.selecting = true
	sel.selectRow, sel.selectColumn = 0, 0
	last := prov.rows() - 1
	sel.cursorRow = last
	sel.cursorColumn = prov.rowLength(last)
}

// TestLabel_SelectAcrossWrap_PreservesSpace (#161) checks that copying a word-wrapped
// selection preserves the space word-wrap consumes at the wrap point — selecting the
// whole label yields the original text, not a run with the wrap-space dropped.
func TestLabel_SelectAcrossWrap_PreservesSpace(t *testing.T) {
	test.NewTempApp(t)
	const text = "the quick brown fox jumps over the lazy dog"
	l := NewLabel(text)
	l.Wrapping = bellui.TextWrapWord
	l.Selectable = true
	w := test.NewWindow(l)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(90, 200)) // narrow ⇒ several word-wrapped rows

	sel := test.WidgetRenderer(l).Objects()[0].(*focusSelectable)
	require.Greater(t, l.provider.rows(), 1, "text must wrap to exercise the wrap-point case")

	fullSelect(t, l.provider, sel)
	assert.Equal(t, text, l.SelectedText(),
		"copying a wrapped selection preserves the consumed wrap-point spaces")
}

// TestRichText_SelectAcrossWrap_PreservesSpace (#161) is the same guard for a
// single-segment selectable RichText (not just Label).
func TestRichText_SelectAcrossWrap_PreservesSpace(t *testing.T) {
	test.NewTempApp(t)
	const text = "alpha beta gamma delta epsilon zeta eta theta"
	rt := NewRichTextWithText(text)
	rt.Wrapping = bellui.TextWrapWord
	rt.Selectable = true
	w := test.NewWindow(rt)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(90, 200))

	sel := test.WidgetRenderer(rt).Objects()[0].(*focusSelectable)
	require.Greater(t, rt.rows(), 1)

	fullSelect(t, rt, sel)
	assert.Equal(t, text, rt.SelectedText())
}

// TestRichText_MixedSegment_SelectAcrossWrap_PreservesSpace (#161) is the remaining
// mixed-segment case: a selectable RichText with two inline TextSegments whose
// second run word-wraps. Copying the whole selection must preserve the spaces the
// wrap consumes inside that run (the single-segment fast path does not apply here,
// so this exercises the row-reconstruction joiner).
func TestRichText_MixedSegment_SelectAcrossWrap_PreservesSpace(t *testing.T) {
	test.NewTempApp(t)
	seg1 := &TextSegment{Style: RichTextStyleInline, Text: "Alpha "}
	seg2 := &TextSegment{Style: RichTextStyleInline, Text: "bravo charlie delta echo foxtrot golf hotel"}
	rt := NewRichText(seg1, seg2)
	rt.Wrapping = bellui.TextWrapWord
	rt.Selectable = true
	w := test.NewWindow(rt)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(110, 200))

	sel := test.WidgetRenderer(rt).Objects()[0].(*focusSelectable)
	require.False(t, func() bool { _, ok := rt.singleSegment(); return ok }(), "must be multi-segment")
	require.Greater(t, rt.rows(), 1, "the second run must wrap across rows")

	fullSelect(t, rt, sel)
	assert.Equal(t, "Alpha bravo charlie delta echo foxtrot golf hotel", rt.SelectedText(),
		"mixed-segment wrapped copy preserves the consumed wrap-point spaces")
}

// TestRichText_MixedSegment_SelectAcrossWrap_WidthSweep (#164) guards multi-segment
// wrapped selection copy across many widths and segment shapes — including inline
// runs that carry trailing spaces and embedded newlines. A full-document selection
// must always reconstruct exactly the concatenation of the segment texts: word-wrap
// must not drop a consumed space nor insert a spurious newline at any wrap point.
//
// This also documents the #164 investigation: the cross-segment-boundary case it
// feared (a row ending segment A while the next row starts segment B mid logical
// line) does not arise for inline content — an inline run always begins on the
// current row, so the wrap stays inside one segment and softWrapGap already handles
// it. The sweep is the regression guard for that conclusion.
func TestRichText_MixedSegment_SelectAcrossWrap_WidthSweep(t *testing.T) {
	test.NewTempApp(t)
	cases := [][]string{
		{"alpha ", "beta gamma delta epsilon zeta"},
		{"alpha beta ", "gamma delta epsilon eta theta iota"},
		{"Bold ", "normal text that should wrap across several visual rows here"},
		{"line one\n", "still one ", "two words wrap here with a lot more text following"},
		{"aa bb cc ", "dd ee\nff gg ", "hh ii jj kk ll mm nn oo"},
	}
	for ci, parts := range cases {
		want := strings.Join(parts, "")
		for wpx := float32(60); wpx <= 200; wpx += 10 {
			segs := make([]RichTextSegment, len(parts))
			for i, p := range parts {
				segs[i] = &TextSegment{Style: RichTextStyleInline, Text: p}
			}
			rt := NewRichText(segs...)
			rt.Wrapping = bellui.TextWrapWord
			rt.Selectable = true
			w := test.NewWindow(rt)
			w.Resize(geo.NewSize(wpx, 400))
			rt.Refresh()

			sel := test.WidgetRenderer(rt).Objects()[0].(*focusSelectable)
			fullSelect(t, rt, sel)
			assert.Equalf(t, want, rt.SelectedText(), "case %d at width %.0f", ci, wpx)
			w.Close()
		}
	}
}
