package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// textObjCount returns the number of materialised canvas objects backing the
// virtualised text (one per windowed row).
func textRendererOf(e *Entry) *textRenderer {
	return cache.Renderer(&e.text).(*textRenderer)
}

// TestCachedSegmentVisual_NonContiguousOffsets guards the row-virtualisation
// requirement that segment visuals may be requested for sparse, out-of-order
// offsets (only the visible window of rows is materialised). The cache must key
// strictly by offset: the same offset returns the same object and distinct
// offsets return distinct objects, regardless of request order.
func TestCachedSegmentVisual_NonContiguousOffsets(t *testing.T) {
	seg := &TextSegment{Text: "the quick brown fox"}
	rt := NewRichText(seg)

	// Request a high offset first (empty cache), then a low one, then the high
	// one again. The old append-on-miss logic would have stored the first miss
	// at index 0, so the third call would return the row-0 object for offset 5000.
	high1 := rt.cachedSegmentVisual(seg, 5000)
	low := rt.cachedSegmentVisual(seg, 0)
	high2 := rt.cachedSegmentVisual(seg, 5000)

	assert.NotNil(t, high1)
	assert.NotNil(t, low)
	assert.Same(t, high1, high2, "same offset must return the same cached object")
	assert.NotSame(t, low, high1, "distinct offsets must return distinct objects")
}

const virtTestRows = 5000

// newVirtualEntry builds a multi-line Entry in a small window so its content
// scroll has a viewport much shorter than the document — the virtualisation case.
func newVirtualEntry(t *testing.T, text string) (*Entry, bellui.Window) {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 200))
	e.SetText(text)
	return e, w
}

func repeatLines(n int) string { return strings.Repeat("the quick brown fox\n", n) }

func TestEntry_Virtualizes_LargeDocument(t *testing.T) {
	e, _ := newVirtualEntry(t, repeatLines(virtTestRows))

	require.True(t, e.text.canVirtualize(), "multi-line non-wrapping entry should virtualise")
	rows := e.text.rows()
	require.GreaterOrEqual(t, rows, virtTestRows)

	// Only a small window of rows is materialised, not the whole document.
	r := textRendererOf(e)
	assert.Greater(t, len(r.origObjs), 0)
	assert.Less(t, len(r.origObjs), 200,
		"expected a windowed object count, got %d for %d rows", len(r.origObjs), rows)

	// Row geometry is served analytically for every row (selection/cursor/gutter):
	// the lazy model (pre-rebrand tracker #157) carries no dense per-row array, but RowPositions
	// synthesises one on demand.
	yPos, heights := e.RowPositions()
	assert.Len(t, yPos, rows)
	assert.Len(t, heights, rows)

	// MinSize reflects the full document height so the scrollbar extent is right.
	stride := e.text.stride()
	gotH := e.text.MinSize().Height
	assert.InDelta(t, float32(rows)*stride, gotH, float64(stride*2),
		"MinSize height should reflect all rows")
}

func TestEntry_Virtual_RowGeometryUniform(t *testing.T) {
	e, _ := newVirtualEntry(t, repeatLines(virtTestRows))
	stride := e.text.stride()
	require.Greater(t, stride, float32(0))

	// Analytic geometry: every row advances by exactly one stride.
	y0, _ := e.text.RowGeometry(0)
	for _, row := range []int{1, 100, virtTestRows - 1} {
		y, h := e.text.RowGeometry(row)
		assert.InDelta(t, y0+float32(row)*stride, y, 0.01)
		assert.InDelta(t, stride, h, 0.01)
	}
}

func TestEntry_Virtual_ScrollMovesWindow(t *testing.T) {
	e, w := newVirtualEntry(t, repeatLines(virtTestRows))
	rows := e.text.rows()

	assert.Equal(t, 0, e.text.winStart, "window starts at the top")
	require.Less(t, e.text.winEnd, rows)

	// Scroll to the bottom via a real wheel event; the window should follow.
	stride := e.text.stride()
	test.Scroll(w.Canvas(), geo.NewPos(150, 100), 0, -float32(rows)*stride)

	assert.Greater(t, e.text.winStart, 0, "window start should advance after scrolling down")
	assert.Equal(t, rows, e.text.winEnd, "window should reach the last row at the bottom")

	r := textRendererOf(e)
	assert.Less(t, len(r.origObjs), 200, "window stays bounded after scrolling")
}

func TestEntry_Virtual_WindowContentCorrect(t *testing.T) {
	var b strings.Builder
	for i := 0; i < virtTestRows; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	e, w := newVirtualEntry(t, b.String())
	require.True(t, e.text.canVirtualize())

	// Scroll into the middle of the document (a non-contiguous window — the case
	// the old append-based visual cache corrupted).
	stride := e.text.stride()
	test.Scroll(w.Canvas(), geo.NewPos(150, 100), 0, -float32(2500)*stride)

	r := textRendererOf(e)
	require.Greater(t, e.text.winStart, 0)
	require.Equal(t, len(r.origObjs), e.text.winEnd-e.text.winStart)

	// Each materialised object must hold the text of its absolute row.
	for k := 0; k < len(r.origObjs); k += 5 {
		txt, ok := r.origObjs[k].(*canvas.Text)
		require.True(t, ok)
		assert.Equal(t, fmt.Sprintf("line %d", e.text.winStart+k), txt.Text)
	}
}

func TestEntry_Virtual_CursorScrollRewindows(t *testing.T) {
	e, _ := newVirtualEntry(t, repeatLines(virtTestRows))
	require.Equal(t, 0, e.text.winStart)

	// Move the cursor to the last row. ensureCursorVisible scrolls
	// programmatically (ScrollToOffset, which does not fire OnScrolled), so the
	// window must be re-derived from that path, not OnScrolled.
	e.CursorRow = e.text.rows() - 1
	e.Refresh()

	assert.Greater(t, e.text.winStart, 0, "window should follow a cursor-driven scroll")
	r := textRendererOf(e)
	assert.Less(t, len(r.origObjs), 200, "window stays bounded after cursor scroll")
}

// TestEntry_Virtualizes_WrappedDocument: a large word-wrapping multi-line entry
// now virtualises too (pre-rebrand tracker #139). Wrapped rows share the uniform stride, so only a
// window of visual rows is materialised even though wrapping multiplies the row
// count well past the logical-line count.
func TestEntry_Virtualizes_WrappedDocument(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 200))
	e.SetText(strings.Repeat("the quick brown fox jumps over the lazy dog\n", 2000))

	require.True(t, e.text.canVirtualize(), "a wrapping multi-line entry virtualises")
	rows := e.text.rows()
	require.Greater(t, rows, 2000, "wrapping produces more visual rows than logical lines")

	r := textRendererOf(e)
	assert.Greater(t, len(r.origObjs), 0)
	assert.Less(t, len(r.origObjs), 200,
		"only a window of wrapped rows is materialised, got %d for %d rows", len(r.origObjs), rows)
}
