package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// TestEntry_WrappedVirtual_WindowFollowsScroll: a large word-wrapping entry
// (#139) materialises only the window, the window tracks the scroll position,
// and every row's geometry stays exact (uniform stride, no folds) even far
// outside the materialised window.
func TestEntry_WrappedVirtual_WindowFollowsScroll(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(200, 200))
	e.SetText(strings.Repeat("the quick brown fox jumps over the lazy dog\n", 1000))

	require.True(t, e.text.canVirtualize())
	rows := e.text.rows()
	require.Greater(t, rows, 1000, "wrapping multiplies the visual row count")

	topWin := append([]int(nil), e.text.winRows...)
	require.NotEmpty(t, topWin)
	assert.Equal(t, 0, topWin[0], "window starts at row 0 before scrolling")

	// Scroll near the end and re-window.
	_, stride := e.RowMetrics()
	e.scroll.ScrollToOffset(geo.NewPos(0, float32(rows-20)*stride))
	e.updateTextViewport()
	cache.Renderer(&e.text).Refresh()

	botWin := e.text.winRows
	require.NotEmpty(t, botWin)
	assert.NotEqual(t, topWin, botWin, "the materialised window moves with scroll")
	assert.Greater(t, botWin[0], rows-80, "window tracks the scrolled position near the end")

	r := textRendererOf(e)
	assert.Less(t, len(r.origObjs), 200,
		"only a window of wrapped rows is materialised after a deep scroll, got %d", len(r.origObjs))

	// Geometry for a deep, non-materialised row is exact (uniform stride).
	top0 := e.text.contentTopOffset()
	deep := rows - 10
	yPos, _ := e.RowPositions()
	require.Equal(t, rows, len(yPos))
	assert.InDelta(t, top0+float32(deep)*stride, yPos[deep], 0.5,
		"a deep wrapped row's Y follows the uniform stride")
}

// TestEntry_WrappedVirtual_EditDeepRow: caret offset mapping runs through the
// full rowBounds, so editing a logical line whose wrapped rows are outside the
// materialised window still lands at the right place.
func TestEntry_WrappedVirtual_EditDeepRow(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(200, 200))

	var b strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "line %d the quick brown fox jumps over\n", i)
	}
	e.SetText(b.String())
	require.True(t, e.text.canVirtualize())

	// Map logical line 400 to its first visual row, place the caret there, type.
	// The lazy model (#157) serves this analytically (no dense rowToLine slice).
	row := e.text.firstRowOfLine(400)
	require.GreaterOrEqual(t, row, 0, "logical line 400 must have a visual row")
	require.Less(t, row, e.text.rows())
	e.CursorRow, e.CursorColumn = row, 0
	e.TypedRune('X')

	lines := strings.Split(e.Text, "\n")
	require.Greater(t, len(lines), 400)
	assert.True(t, strings.HasPrefix(lines[400], "Xline 400"),
		"edit landed at the start of logical line 400, got %q", lines[400])
	assert.True(t, strings.HasPrefix(lines[399], "line 399"),
		"neighbouring lines are untouched, got %q", lines[399])
}
