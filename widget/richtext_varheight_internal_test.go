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
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// newVarHeightRichText builds a large multi-segment RichText (text paragraphs
// interleaved with separators and volatile HeightForWidth blocks) with
// virtualisation opted in, large enough to cross varHeightMinRows so the
// variable-height window engages.
func newVarHeightRichText(t *testing.T, paras int) (*RichText, *textRenderer) {
	t.Helper()
	test.NewTempApp(t)

	var segs []RichTextSegment
	for i := 0; i < paras; i++ {
		segs = append(segs, &TextSegment{Text: fmt.Sprintf("paragraph number %d with some words", i)})
		switch i % 200 {
		case 50:
			segs = append(segs, &SeparatorSegment{})
		case 150:
			segs = append(segs, &countingHFWSegment{count: new(int), height: 120})
		}
	}
	rich := NewRichText(segs...)
	rich.Wrapping = bellui.TextWrapWord
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 600))
	rich.CreateRenderer()
	rich.updateRowBounds()

	r := cache.Renderer(rich).(*textRenderer)
	return rich, r
}

// TestRichText_VarHeight_Engages confirms the variable-height window turns on for
// a large multi-segment document and stays off for a small one.
func TestRichText_VarHeight_Engages(t *testing.T) {
	rich, _ := newVarHeightRichText(t, 2100)
	require.Greater(t, rich.rows(), varHeightMinRows)
	assert.True(t, rich.blockLazy, "large multi-segment content must use the windowed block build (#317)")
	assert.False(t, rich.varHeight, "the windowed build supersedes the dense varHeight model when no folds are active")
	assert.False(t, rich.lazy, "multi-segment content never uses the single-segment lazy model")
	assert.Nil(t, rich.rowBounds, "the block-lazy build keeps rowBounds nil")
	assert.True(t, rich.canVirtualize())

	small := NewRichText(&TextSegment{Text: "a"}, &SeparatorSegment{}, &TextSegment{Text: "b"})
	small.Wrapping = bellui.TextWrapWord
	small.virtualize = true
	small.Resize(geo.NewSize(400, 600))
	small.CreateRenderer()
	small.updateRowBounds()
	assert.False(t, small.varHeight, "small multi-segment content stays on the full path")
	assert.False(t, small.blockLazy, "small multi-segment content stays on the full path")
}

// TestRichText_VarHeight_WindowsAndGeometry checks that only a window of objects is
// materialised, geometry is monotonic and prefix-derived, and rowAtY inverts it.
func TestRichText_VarHeight_WindowsAndGeometry(t *testing.T) {
	rich, r := newVarHeightRichText(t, 2100)
	n := rich.rows()

	rich.SetVisibleRange(0, 600)
	r.Refresh()

	assert.Greater(t, len(r.origObjs), 0)
	assert.Less(t, len(r.origObjs), 400,
		"expected a windowed object count, got %d for %d rows", len(r.origObjs), n)

	// Geometry is strictly increasing across visible rows.
	prevY := float32(-1)
	for _, row := range []int{0, 1, 10, 500, 1500, n - 1} {
		y, h := rich.RowGeometry(row)
		assert.Greater(t, h, float32(0), "row %d height", row)
		assert.Greater(t, y, prevY, "row %d Y must increase", row)
		prevY = y
	}

	// rowAtY inverts RowGeometry across variable heights.
	for _, row := range []int{0, 7, 123, 800, 1999, n - 1} {
		y, h := rich.RowGeometry(row)
		got := rich.rowAtY(y + h/2)
		assert.Equal(t, row, got, "rowAtY must map the middle of row %d back to it", row)
	}

	// minVirtual height covers the whole document (block Y-prefix total plus padding).
	rich.ensureBlockPrefix()
	total := rich.blockTopY[len(rich.blocks)]
	min := r.minVirtual()
	assert.Greater(t, min.Height, total, "scroll extent covers the whole document")
}

// TestRichText_VarHeight_WindowFollowsScroll verifies the window moves to deep rows
// on scroll and the object count stays bounded.
func TestRichText_VarHeight_WindowFollowsScroll(t *testing.T) {
	rich, r := newVarHeightRichText(t, 2100)
	n := rich.rows()

	rich.SetVisibleRange(0, 600)
	r.Refresh()
	topWin := append([]int(nil), rich.winRows...)
	require.NotEmpty(t, topWin)
	assert.Less(t, topWin[0], 20, "top window starts near the document start")

	// Scroll to near the bottom.
	deepY, _ := rich.RowGeometry(n - 5)
	rich.SetVisibleRange(deepY, 600)
	r.Refresh()
	require.NotEmpty(t, rich.winRows)
	assert.Greater(t, rich.winRows[0], n-200, "deep scroll materialises deep rows")
	assert.Less(t, len(r.origObjs), 400, "object count stays bounded after scrolling")
}

// TestRichText_VarHeight_ScrollDrivesWindow exercises the standalone-scroll
// integration: a scrollable RichText auto-enables virtualisation and drives its
// own window from the internal scroll offset (pre-rebrand tracker #158).
func TestRichText_VarHeight_ScrollDrivesWindow(t *testing.T) {
	test.NewTempApp(t)
	var segs []RichTextSegment
	for i := 0; i < 2100; i++ {
		segs = append(segs, &TextSegment{Text: fmt.Sprintf("line number %d here", i)})
	}
	rich := NewRichText(segs...)
	rich.Wrapping = bellui.TextWrapWord
	rich.Scroll = intWidget.ScrollVerticalOnly
	rich.Virtualize = true // opt in (pre-rebrand tracker #169)
	rich.Resize(geo.NewSize(400, 600))

	r := cache.Renderer(rich).(*textRenderer)
	rich.updateRowBounds() // builds dense bounds and flips on varHeight
	r.MinSize()            // sizes the scroll content extent (prop) via minVirtual
	r.Refresh()
	r.Layout(geo.NewSize(400, 600))

	require.True(t, rich.virtualize, "Virtualize opts the scrollable RichText into windowing")
	require.True(t, rich.blockLazy)
	require.NotNil(t, rich.scr)

	require.NotEmpty(t, rich.winRows)
	assert.Less(t, rich.winRows[0], 30, "initial window near the top")
	assert.Less(t, len(r.origObjs), 400, "initial object count bounded")

	// Scroll deep via the scroll container; OnScrolled re-windows.
	deepY, _ := rich.RowGeometry(2000)
	rich.scr.ScrollToOffset(geo.NewPos(0, deepY))
	rich.updateVirtualViewport() // ensure the window is recomputed in the test harness

	require.NotEmpty(t, rich.winRows)
	assert.Greater(t, rich.winRows[0], 1800, "deep scroll materialises deep rows")
	assert.Less(t, len(r.origObjs), 400, "object count stays bounded after scrolling")
}

// TestRichText_VarHeight_FoldCollapsesHeight verifies a folded row collapses to
// zero height in the prefix and shifts following rows up, while unfolding restores
// the geometry.
func TestRichText_VarHeight_FoldCollapsesHeight(t *testing.T) {
	rich, _ := newVarHeightRichText(t, 2100)
	require.True(t, rich.blockLazy, "starts on the windowed block build")

	// Folding a block-lazy document forces the dense varHeight model: a fold's
	// collapsed extent needs exact per-row geometry, so block-lazy declines folds (pre-rebrand tracker #317).
	rich.setHiddenRanges([]LineRange{{Start: 0, End: 1}})
	require.True(t, rich.varHeight, "a fold switches to the dense varHeight model")
	require.False(t, rich.blockLazy)
	rich.ensureHeightPrefix()

	_, h0Folded := rich.RowGeometry(0)
	assert.Equal(t, float32(0), h0Folded, "folded row collapses to zero height")
	y1Folded, _ := rich.RowGeometry(1)
	totalFolded := rich.rowYPrefix[len(rich.rowBounds)]

	// Unfold restores the geometry (stays on the dense model).
	rich.setHiddenRanges(nil)
	rich.ensureHeightPrefix()
	_, h0 := rich.RowGeometry(0)
	assert.Greater(t, h0, float32(0), "unfold restores the row height")
	y1Unfolded, _ := rich.RowGeometry(1)
	assert.Greater(t, y1Unfolded, y1Folded, "unfolding shifts following rows back down")
	assert.Greater(t, rich.rowYPrefix[len(rich.rowBounds)], totalFolded, "unfolding restores the collapsed height")
}

// TestRichText_VarHeight_Selection confirms text selection works across the whole
// document even though only a window is materialised: SelectedText reconstructs
// from the eager rowBounds, and the selection geometry uses the variable-height
// RowGeometry.
func TestRichText_VarHeight_Selection(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	var segs []RichTextSegment
	for i := 0; i < 2100; i++ {
		segs = append(segs, &TextSegment{Text: fmt.Sprintf("paragraph %d", i), Style: RichTextStyleParagraph})
	}
	rich := NewRichText(segs...)
	rich.Wrapping = bellui.TextWrapWord
	rich.virtualize = true
	rich.Selectable = true
	w := test.NewTempWindow(t, rich)
	w.Resize(geo.NewSize(400, 600))
	require.True(t, rich.blockLazy)

	rich.SetVisibleRange(0, 600)
	cache.Renderer(rich).Refresh()

	rich.selection.selectAll()
	sel := rich.SelectedText()
	assert.Contains(t, sel, "paragraph 0", "selection covers the first row")
	assert.Contains(t, sel, "paragraph 2099", "selection covers the last (off-screen) row")
}

// scrollableRichText builds a scrollable multi-segment RichText large enough to
// cross varHeightMinRows, without opting into virtualisation.
func scrollableRichText(t *testing.T, n int, wrap bellui.TextWrap) *RichText {
	t.Helper()
	var segs []RichTextSegment
	for i := 0; i < n; i++ {
		segs = append(segs, &TextSegment{Text: fmt.Sprintf("line %d", i)})
	}
	rich := NewRichText(segs...)
	rich.Wrapping = wrap
	rich.Scroll = intWidget.ScrollVerticalOnly
	return rich
}

// TestRichText_Virtualize_OptIn confirms the pre-rebrand tracker #169 decision: a scrollable RichText
// virtualises only when Virtualize is set; the default leaves a large scrolling
// document on the (before pre-rebrand tracker #158) full-materialisation path.
func TestRichText_Virtualize_OptIn(t *testing.T) {
	test.NewTempApp(t)

	off := scrollableRichText(t, 2100, bellui.TextWrapWord)
	off.Resize(geo.NewSize(400, 600))
	off.CreateRenderer()
	off.updateRowBounds()
	assert.False(t, off.virtualize, "scrollable RichText does not virtualise unless opted in (#169)")
	assert.False(t, off.varHeight, "no windowing without opt-in")
	assert.False(t, off.canVirtualize())

	on := scrollableRichText(t, 2100, bellui.TextWrapWord)
	on.Virtualize = true
	on.Resize(geo.NewSize(400, 600))
	on.CreateRenderer()
	on.updateRowBounds()
	assert.True(t, on.virtualize, "Virtualize opts the scrollable RichText into windowing (#169)")
	assert.True(t, on.blockLazy, "large multi-segment document windows its build once opted in (#317)")
	assert.True(t, on.canVirtualize())
}

// TestRichText_VarHeight_VirtualWidth_BoundedSample guards pre-rebrand tracker #170: the wrap-off
// widest-row estimate scans only a bounded sample plus the materialised window, so
// a deep wide row outside both is not reflected until it scrolls into view — at
// which point the extent grows and stays grown (monotonic).
func TestRichText_VarHeight_VirtualWidth_BoundedSample(t *testing.T) {
	test.NewTempApp(t)

	const n = 2100
	var segs []RichTextSegment
	for i := 0; i < n; i++ {
		segs = append(segs, &TextSegment{Text: fmt.Sprintf("row %d", i)})
	}

	// Pick a deep row outside both the uniform sample and the initial top window.
	sampled := map[int]bool{}
	step := float64(n) / float64(virtualWidthSampleRows)
	for k := 0; k < virtualWidthSampleRows; k++ {
		sampled[int(float64(k)*step)] = true
	}
	wide := -1
	for i := 1500; i < n; i++ {
		if !sampled[i] {
			wide = i
			break
		}
	}
	require.Greater(t, wide, 0, "expected an unsampled deep row")
	segs[wide] = &TextSegment{Text: strings.Repeat("really wide content ", 40)}

	rich := NewRichText(segs...)
	rich.Wrapping = bellui.TextWrapOff
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 600))
	rich.CreateRenderer()
	rich.updateRowBounds()
	require.True(t, rich.blockLazy)

	r := cache.Renderer(rich).(*textRenderer)
	textSize := rich.Theme().Size(theme.SizeNameText)
	wideWidth := rich.rowWidth(wide, textSize)

	rich.SetVisibleRange(0, 600)
	r.Refresh()
	initial := r.virtualWidth()
	assert.Less(t, initial, wideWidth,
		"bounded sample skips the deep wide row instead of an O(rows) full scan")

	// Scroll the wide row into the window; widenVirtualWidthForWindow finds it.
	yTop, _ := rich.RowGeometry(wide)
	rich.SetVisibleRange(yTop, 600)
	r.Refresh()
	grown := r.virtualWidth()
	assert.GreaterOrEqual(t, grown, wideWidth, "scrolling onto the wide row grows the extent")

	// Scroll back to the top: the extent must not snap narrower (monotonic).
	rich.SetVisibleRange(0, 600)
	r.Refresh()
	assert.GreaterOrEqual(t, r.virtualWidth(), grown, "horizontal extent is monotonic")
}
