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

// measuredLineCount reports how many logical lines have been shaped (measured) so
// far in the lazy model — the white-box proxy for "shaping work done".
func measuredLineCount(t *RichText) int {
	n := 0
	for _, m := range t.lineMeasured {
		if m {
			n++
		}
	}
	return n
}

func newLargeWrappedEntry(t *testing.T, lines int) (*Entry, bellui.Window) {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(200, 200))
	e.SetText(strings.Repeat("the quick brown fox jumps over the lazy dog\n", lines))
	return e, w
}

// TestEntry_LazyWrap_NoFullMeasureOnFirstLayout is the pre-rebrand tracker #157 acceptance: a large
// wrapped document does no O(rows) shaping at first layout — only the lines spanning
// the viewport band are measured, the rest carry an estimate.
func TestEntry_LazyWrap_NoFullMeasureOnFirstLayout(t *testing.T) {
	const lines = 50000
	e, _ := newLargeWrappedEntry(t, lines)

	require.True(t, e.text.canVirtualize())
	require.Greater(t, e.text.buf.LineCount(), lines, "logical lines counted analytically")

	// Wrapping multiplies the visual-row estimate well past the logical-line count.
	require.Greater(t, e.text.rows(), lines)

	// Only a small viewport band is shaped, NOT every line.
	measured := measuredLineCount(&e.text)
	assert.Greater(t, measured, 0, "the visible band is measured")
	assert.Less(t, measured, 300,
		"first layout shaped only the viewport band (%d lines), not all %d", measured, lines)

	// The materialised object window stays bounded too.
	r := textRendererOf(e)
	assert.Less(t, len(r.origObjs), 200)
}

// TestEntry_LazyWrap_WindowMeasuresOnlyBand verifies scrolling measures only the
// newly-revealed band, leaving far-away lines unmeasured.
func TestEntry_LazyWrap_WindowMeasuresOnlyBand(t *testing.T) {
	e, _ := newLargeWrappedEntry(t, 50000)
	require.True(t, e.text.canVirtualize())

	// Scroll deep into the document and re-window.
	_, stride := e.RowMetrics()
	e.scroll.ScrollToOffset(geo.NewPos(0, 20000*stride))
	e.updateTextViewport()

	after := measuredLineCount(&e.text)
	// A deep scroll measures the new band (a handful of lines) but nowhere near all.
	assert.Less(t, after, 600,
		"a deep scroll measured %d lines total — should stay windowed", after)
}

// TestEntry_LazyWrap_CaretRoundTripDeep checks caret/selection CONTENT is exact even
// for offsets far outside the measured window: rowColFromTextPos and textPosFromRowCol
// are inverses, so an offset maps to a (row,col) and back to the same offset.
func TestEntry_LazyWrap_CaretRoundTripDeep(t *testing.T) {
	e, _ := newLargeWrappedEntry(t, 50000)
	runes := []rune(e.Text)
	provider := e.textProvider()

	for _, pos := range []int{0, 1, 100, 500000, len(runes) / 2, len(runes) - 1, len(runes)} {
		row, col := e.rowColFromTextPos(pos)
		got := textPosFromRowCol(row, col, provider)
		assert.Equal(t, pos, got, "round-trip offset %d via (row=%d,col=%d)", pos, row, col)
	}
}

// TestEntry_LazyWrap_ConvergesToExact checks the estimated total row count converges
// to the exact count once every line is measured, and the estimate is in the right
// ballpark beforehand (so the scroll extent is reasonable, not wildly wrong).
func TestEntry_LazyWrap_ConvergesToExact(t *testing.T) {
	e, _ := newLargeWrappedEntry(t, 5000) // > threshold ⇒ estimated
	require.True(t, e.text.canVirtualize())

	est := e.text.rows()

	// Force every line to be measured, then recompute the exact total.
	for i := 0; i < e.text.buf.LineCount(); i++ {
		e.text.ensureLineMeasured(i)
	}
	e.text.prefixDirty = true
	exact := e.text.lazyRowCount()

	assert.Equal(t, exact, e.text.rows(), "rows() is exact once all lines are measured")
	// The pre-measurement estimate should be within a modest factor of exact.
	assert.InEpsilon(t, exact, est, 0.5,
		"estimate %d should be within 50%% of exact %d", est, exact)
}

// TestEntry_LazyWrap_SelectAllContent checks selection CONTENT outside the measured
// window: select-all on a never-scrolled large wrapped document yields the full text.
func TestEntry_LazyWrap_SelectAllContent(t *testing.T) {
	e, _ := newLargeWrappedEntry(t, 3000)
	e.selectAll()
	assert.Equal(t, e.Text, e.SelectedText(), "select-all copies the whole document")
}

// TestEntry_LazyWrap_DenseParityBelowThreshold guards that a wrapped document just
// under the eager threshold is fully measured (exact, dense-equivalent), so caret and
// row count are exact for the common medium-document case.
func TestEntry_LazyWrap_DenseParityBelowThreshold(t *testing.T) {
	e, _ := newLargeWrappedEntry(t, lazyMeasureAllMaxLines-1)
	require.True(t, e.text.canVirtualize())
	assert.Equal(t, e.text.buf.LineCount(), measuredLineCount(&e.text),
		"a sub-threshold wrapped document is measured eagerly (exact)")
}
