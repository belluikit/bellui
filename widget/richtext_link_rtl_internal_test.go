package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// newRTLLinkRichText builds a RichText whose single row mixes an RTL Arabic
// TextSegment with an inline HyperlinkSegment — the row #41 targets: a non-text
// segment embedded in bidi text. Before #41 such a row fell back to the legacy
// logical-prefix path (rowStyledRuns bailed on the non-text segment).
func newRTLLinkRichText(t *testing.T) *RichText {
	t.Helper()
	rtl := &TextSegment{Text: "مرحبا", Style: RichTextStyle{Inline: true}}
	link := &HyperlinkSegment{Text: "link"}
	rich := NewRichText(rtl, link)
	rich.Resize(geo.NewSize(400, 50))
	rich.CreateRenderer()
	test.TempWidgetRenderer(t, rich) // force a layout pass so rowBounds exist
	return rich
}

// linkBoxWidth returns the measured visual width of the row's non-text segment,
// the same width rowStyledRuns folds into the bidi layout as an opaque box.
func linkBoxWidth(t *testing.T, rich *RichText) float32 {
	t.Helper()
	segs := rich.rowBoundary(0).segments
	link := segs[len(segs)-1]
	_, isText := link.(*TextSegment)
	require.False(t, isText, "the trailing segment under test must be non-text")
	return rich.cachedSegmentVisual(link, 0).MinSize().Width
}

// TestRichText_RTLRowWithLink_LayoutResolves confirms a row mixing RTL text and a
// link now resolves via the bidi visual layout instead of bailing (#41).
func TestRichText_RTLRowWithLink_LayoutResolves(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newRTLLinkRichText(t)
	require.Len(t, rich.rowBoundary(0).segments, 2, "text and link must share one row")
	assert.Equal(t, []rune("مرحبا"), rich.row(0), "the link contributes no caret columns")

	layout, ok := rich.rowVisualLayout(0)
	require.True(t, ok, "a row mixing RTL text and a non-text segment must be bidi-resolved")
	assert.True(t, layout.HasRTL())
}

// TestRichText_RTLRowWithLink_WidthIncludesBox confirms the link's width is part
// of the row's measured width (so alignment and the layout account for it).
func TestRichText_RTLRowWithLink_SelectionFillsBox(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newRTLLinkRichText(t)
	innerPad := rich.Theme().Size(theme.SizeNameInnerPadding)
	n := rich.rowLength(0)

	spans, ok := rich.rtlSelectionSpans(0, 0, n, innerPad)
	require.True(t, ok)
	var maxRight float32
	for _, s := range spans {
		if s[1] > maxRight {
			maxRight = s[1]
		}
	}

	layout, _ := rich.rowVisualLayout(0)
	off := innerPad - rich.inset.Width
	assert.InDelta(t, layout.Width+off, maxRight, 1.0,
		"selecting the whole row must cover the link box at the right edge")
}

// TestRichText_RTLRowWithLink_HitTestColumns confirms click hit-testing across the
// row maps every x to a valid caret column (no panic, in range).
func TestRichText_RTLRowWithLink_HitTestColumns(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newRTLLinkRichText(t)
	layout, ok := rich.rowVisualLayout(0)
	require.True(t, ok)
	n := rich.rowLength(0)

	for x := float32(-20); x <= layout.Width+40; x += 3 {
		col := layout.HitTest(x)
		require.GreaterOrEqual(t, col, 0)
		require.LessOrEqual(t, col, n, "hit-test column must stay within the row")
	}
}
