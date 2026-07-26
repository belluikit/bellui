package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// mixedBlockSegments builds a multi-segment document mixing wrapping paragraphs,
// inline-composed runs (bold/emphasis sharing a row), headings, separators and a
// volatile HeightForWidth block — exercising every shapeBlock branch.
func mixedBlockSegments(paras int) []RichTextSegment {
	segs := make([]RichTextSegment, 0, paras*3)
	for i := 0; i < paras; i++ {
		segs = append(
			segs,
			&TextSegment{Text: fmt.Sprintf("paragraph number %d wraps across several visual rows when laid out narrow enough to force word wrapping here", i)},
			&TextSegment{Style: RichTextStyleStrong, Text: fmt.Sprintf("strong %d ", i)},
			&TextSegment{Style: RichTextStyleEmphasis, Text: fmt.Sprintf("and emphasis %d on the same row", i)},
		)
		switch i % 50 {
		case 10:
			segs = append(segs, &SeparatorSegment{})
		case 20:
			segs = append(segs, &TextSegment{Style: RichTextStyleHeading, Text: fmt.Sprintf("Heading %d", i)})
		case 30:
			segs = append(segs, &countingHFWSegment{count: new(int), height: 90})
		}
	}
	return segs
}

// TestRichText_BlockLazy_ShapeMatchesDense is the core correctness guarantee (#317):
// shaping every block on demand must reproduce the dense build's rowBounds exactly
// (begin/end/ellipsis/firstSegmentReuse and segment identity), so the windowed build
// renders identically.
func TestRichText_BlockLazy_ShapeMatchesDense(t *testing.T) {
	test.NewTempApp(t)

	for _, wrap := range []bellui.TextWrap{bellui.TextWrapWord, bellui.TextWrapBreak, bellui.TextWrapOff} {
		t.Run(fmt.Sprintf("wrap%d", wrap), func(t *testing.T) {
			// 2200 paragraphs so even wrap-off (one row per paragraph) crosses the
			// windowing threshold for every wrap mode.
			const paras = 2200
			// Dense reference: virtualisation off ⇒ full eager rowBounds.
			dense := NewRichText(mixedBlockSegments(paras)...)
			dense.Wrapping = wrap
			dense.Resize(geo.NewSize(400, 600))
			dense.updateRowBounds()
			require.False(t, dense.blockLazy)
			require.Greater(t, len(dense.rowBounds), varHeightMinRows)

			// Block-lazy: same content (distinct segment instances), virtualised.
			lazy := NewRichText(mixedBlockSegments(paras)...)
			lazy.Wrapping = wrap
			lazy.virtualize = true
			lazy.Resize(geo.NewSize(400, 600))
			lazy.updateRowBounds()
			require.True(t, lazy.blockLazy, "large multi-segment content must build block-lazy")

			var got []rowBoundary
			gotSegs := lazy.Segments
			for b := range lazy.blocks {
				lazy.ensureBlockMeasured(b)
				got = append(got, lazy.blockRows[b]...)
			}

			require.Equal(t, len(dense.rowBounds), len(got), "row count must match the dense build")
			// Segment instances differ across the two documents, so identity is checked
			// positionally: the j-th segment of row i must be the same index in each
			// document's flat segment list.
			indexOf := func(segs []RichTextSegment, target RichTextSegment) int {
				for k, s := range segs {
					if s == target {
						return k
					}
				}
				return -1
			}
			for i := range dense.rowBounds {
				d, g := dense.rowBounds[i], got[i]
				assert.Equal(t, d.begin, g.begin, "row %d begin", i)
				assert.Equal(t, d.end, g.end, "row %d end", i)
				assert.Equal(t, d.ellipsis, g.ellipsis, "row %d ellipsis", i)
				assert.Equal(t, d.firstSegmentReuse, g.firstSegmentReuse, "row %d reuse", i)
				require.Equal(t, len(d.segments), len(g.segments), "row %d segment count", i)
				for j := range d.segments {
					assert.Equal(t, indexOf(dense.Segments, d.segments[j]), indexOf(gotSegs, g.segments[j]),
						"row %d segment %d must map to the same flat-list index", i, j)
				}
			}
		})
	}
}

// TestRichText_BlockLazy_WindowedBuild verifies that the cold build shapes only a
// bounded number of blocks (it does not shape the whole document up front), while
// rows()/RowGeometry stay consistent.
func TestRichText_BlockLazy_WindowedBuild(t *testing.T) {
	test.NewTempApp(t)
	rich := NewRichText(mixedBlockSegments(1500)...)
	rich.Wrapping = bellui.TextWrapWord
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 600))
	r := cache.Renderer(rich).(*textRenderer)
	rich.updateRowBounds()
	require.True(t, rich.blockLazy)

	rich.SetVisibleRange(0, 600)
	r.Refresh()

	measured := 0
	for _, m := range rich.blockMeasured {
		if m {
			measured++
		}
	}
	assert.Greater(t, measured, 0, "viewport must shape its blocks")
	assert.Less(t, measured, len(rich.blocks)/4, "only a window of blocks is shaped, got %d/%d", measured, len(rich.blocks))
	assert.Less(t, len(r.origObjs), 400, "windowed object count")
}

// TestRichText_BlockLazy_NonASCIIBuildIsLazy guards the #313/#317 interaction: the
// cold build must not shape per block even for non-ASCII (CJK) content, where line
// height cannot be served from the ASCII style cache. Heights are estimated from the
// style's ASCII proxy and only the viewport's blocks are shaped on demand.
func TestRichText_BlockLazy_NonASCIIBuildIsLazy(t *testing.T) {
	test.NewTempApp(t)
	var segs []RichTextSegment
	for i := 0; i < 2500; i++ {
		segs = append(segs, &TextSegment{Text: fmt.Sprintf("段落番号 %d これは日本語のテキストで折り返しを強制します", i)})
	}
	rich := NewRichText(segs...)
	rich.Wrapping = bellui.TextWrapWord
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 600))
	rich.updateRowBounds()
	require.True(t, rich.blockLazy)
	require.Greater(t, rich.rows(), varHeightMinRows)

	// Only the viewport's blocks are shaped — non-ASCII content (whose line height
	// cannot use the ASCII style cache) must still window rather than shape per block.
	rich.SetVisibleRange(0, 600)
	cache.Renderer(rich).Refresh()
	measured := 0
	for _, m := range rich.blockMeasured {
		if m {
			measured++
		}
	}
	assert.Greater(t, measured, 0, "the viewport's blocks are shaped")
	assert.Less(t, measured, len(rich.blocks)/4,
		"only a window of non-ASCII blocks is shaped, got %d/%d", measured, len(rich.blocks))
}

// TestRichText_BlockLazy_FoldFallbackRebuilds confirms a fold appearing on a
// block-lazy document switches it to the dense varHeight model and back.
func TestRichText_BlockLazy_FoldFallbackRebuilds(t *testing.T) {
	test.NewTempApp(t)
	rich := NewRichText(mixedBlockSegments(900)...)
	rich.Wrapping = bellui.TextWrapWord
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 600))
	rich.updateRowBounds()
	require.True(t, rich.blockLazy)

	rich.setHiddenRanges([]LineRange{{Start: 2, End: 5}})
	assert.False(t, rich.blockLazy, "a fold leaves the block-lazy model")
	assert.True(t, rich.varHeight, "a fold switches to the dense varHeight model")
	require.NotNil(t, rich.rowBounds)

	rich.setHiddenRanges(nil)
	rich.rowBoundsVersion++ // a content/state change re-evaluates the model
	rich.updateRowBounds()
	assert.True(t, rich.blockLazy, "clearing the fold returns to the windowed build")
}

// TestRichText_BlockLazy_GeometryMonotonic checks RowGeometry is prefix-derived and
// strictly increasing, and rowAtY inverts it across the variable heights.
func TestRichText_BlockLazy_GeometryMonotonic(t *testing.T) {
	test.NewTempApp(t)
	rich := NewRichText(mixedBlockSegments(900)...)
	rich.Wrapping = bellui.TextWrapWord
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 600))
	rich.updateRowBounds()
	require.True(t, rich.blockLazy)
	n := rich.rows()

	prevY := float32(-1)
	for _, row := range []int{0, 1, 10, 500, 1500, n - 1} {
		if row < 0 || row >= n {
			continue
		}
		y, h := rich.RowGeometry(row)
		assert.Greater(t, h, float32(0), "row %d height", row)
		assert.Greater(t, y, prevY, "row %d Y must increase", row)
		prevY = y
		assert.Equal(t, row, rich.rowAtY(y+h/2), "rowAtY inverts row %d", row)
	}
}
