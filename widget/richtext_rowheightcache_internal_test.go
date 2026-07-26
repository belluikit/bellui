package widget

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// countingRect is a non-text, non-volatile CanvasObject that counts MinSize
// calls. It does NOT implement HeightForWidth and is not a *richImage, so rows
// backed by it are eligible for the per-row height cache (#158 Step 1).
type countingRect struct {
	*canvas.Rectangle
	count *int
	min   geo.Size
}

func (c *countingRect) MinSize() geo.Size {
	*c.count++
	return c.min
}

// countingSegment is a block (non-inline) segment whose visual is a countingRect.
type countingSegment struct {
	count  *int
	height float32
}

func (s *countingSegment) Inline() bool             { return false }
func (s *countingSegment) Textual() string          { return "" }
func (s *countingSegment) Update(core.CanvasObject) {}
func (s *countingSegment) Visual() core.CanvasObject {
	return &countingRect{Rectangle: canvas.NewRectangle(color.Transparent), count: s.count, min: geo.NewSize(20, s.height)}
}
func (s *countingSegment) Select(geo.Position, geo.Position) {}
func (s *countingSegment) SelectedText() string              { return "" }
func (s *countingSegment) Unselect()                         {}

// countingHFWRect is a volatile CanvasObject (implements HeightForWidth), standing
// in for a table/code block whose size can change after async content resolves.
// Rows backed by it are intentionally excluded from the height cache.
type countingHFWRect struct {
	*canvas.Rectangle
	count *int
	min   geo.Size
}

func (c *countingHFWRect) MinSize() geo.Size              { *c.count++; return c.min }
func (c *countingHFWRect) HeightForWidth(float32) float32 { *c.count++; return c.min.Height }

type countingHFWSegment struct {
	count  *int
	height float32
}

func (s *countingHFWSegment) Inline() bool             { return false }
func (s *countingHFWSegment) Textual() string          { return "" }
func (s *countingHFWSegment) Update(core.CanvasObject) {}
func (s *countingHFWSegment) Visual() core.CanvasObject {
	return &countingHFWRect{Rectangle: canvas.NewRectangle(color.Transparent), count: s.count, min: geo.NewSize(20, s.height)}
}
func (s *countingHFWSegment) Select(geo.Position, geo.Position) {}
func (s *countingHFWSegment) SelectedText() string              { return "" }
func (s *countingHFWSegment) Unselect()                         {}

// rowHeightCacheFixture builds a multi-segment RichText interleaving text rows
// with a non-volatile spy block and a volatile (HeightForWidth) spy block, lays
// it out, and returns the widget, its renderer, and the two call counters.
func rowHeightCacheFixture(t *testing.T) (rich *RichText, r *textRenderer, nonVolatile, volatile *int) {
	t.Helper()
	test.NewTempApp(t)

	nv := new(int)
	v := new(int)
	segs := []RichTextSegment{
		&TextSegment{Text: "first line of text"},
		&countingSegment{count: nv, height: 40},
		&TextSegment{Text: "second line of text"},
		&countingHFWSegment{count: v, height: 80},
		&TextSegment{Text: "third line of text"},
	}
	rich = NewRichText(segs...)
	// Word-wrap mirrors real mixed content (markdown/HTML). In this mode the
	// width branch of calculateMin is skipped, so the per-row height cache is the
	// only per-row measurement — exactly what Step 1 targets.
	rich.Wrapping = bellui.TextWrapWord
	rich.Resize(geo.NewSize(400, 600))
	rich.CreateRenderer()
	test.TempWidgetRenderer(t, rich) // force a layout pass so rowBounds + objects exist

	r = cache.Renderer(rich).(*textRenderer)
	return rich, r, nv, v
}

// TestRichText_RowHeightCache_NoRemeasure is the acceptance test: after the first
// measurement pass, a re-layout at the same width/version does not re-measure
// unchanged non-volatile rows, while volatile (image/table) rows are re-measured.
func TestRichText_RowHeightCache_NoRemeasure(t *testing.T) {
	rich, r, nv, v := rowHeightCacheFixture(t)

	// Start from a cold cache so the first MinSize below is the populating pass.
	rich.rowHeightCacheVer = -1
	*nv, *v = 0, 0

	r.MinSize()
	nv1, v1 := *nv, *v
	require.Greater(t, nv1, 0, "the non-volatile row must be measured on the first pass")
	require.Greater(t, v1, 0, "the volatile row must be measured on the first pass")

	// The cache is populated for non-volatile rows and left unmeasured (NaN) for
	// volatile rows.
	require.Len(t, rich.rowHeightCache, len(rich.rowBounds))
	nvRow, vRow := -1, -1
	for i, b := range rich.rowBounds {
		switch b.segments[0].(type) {
		case *countingSegment:
			nvRow = i
		case *countingHFWSegment:
			vRow = i
		}
	}
	require.GreaterOrEqual(t, nvRow, 0)
	require.GreaterOrEqual(t, vRow, 0)
	assert.False(t, math.IsNaN(float64(rich.rowHeightCache[nvRow])), "non-volatile row height must be cached")
	assert.True(t, math.IsNaN(float64(rich.rowHeightCache[vRow])), "volatile row must be left unmeasured")

	// Second pass at the same width/version: non-volatile rows served from cache
	// (no new MinSize calls); the volatile row is re-measured.
	r.MinSize()
	assert.Equal(t, nv1, *nv, "unchanged non-volatile row must not be re-measured")
	assert.Greater(t, *v, v1, "volatile row must be re-measured each pass")
}

// TestRichText_RowHeightCache_WidthChangeInvalidates verifies a width change
// rebuilds the cache (heights are width-dependent).
func TestRichText_RowHeightCache_WidthChangeInvalidates(t *testing.T) {
	rich, r, nv, _ := rowHeightCacheFixture(t)

	rich.rowHeightCacheVer = -1
	*nv = 0
	r.MinSize()
	nv1 := *nv
	r.MinSize()
	require.Equal(t, nv1, *nv, "warm cache: no re-measure")

	// A resize changes the measurement width and bumps rowBoundsVersion, so the
	// cache key no longer matches and the row is measured afresh.
	rich.Resize(geo.NewSize(250, 600))
	rich.updateRowBounds()
	r.MinSize()
	assert.Greater(t, *nv, nv1, "width change must invalidate the height cache")
}

// TestRichText_RowHeightCache_FoldKeepsCache verifies that folding a row does not
// invalidate the cache: folded rows contribute zero height without re-measuring
// the unchanged rows.
func TestRichText_RowHeightCache_FoldKeepsCache(t *testing.T) {
	rich, r, nv, _ := rowHeightCacheFixture(t)

	rich.rowHeightCacheVer = -1
	*nv = 0
	r.MinSize()
	nv1 := *nv

	// Fold the first visual row. Folding only updates the hidden set (hiddenRanges)
	// — it does not rebuild rowBounds or bump the version, so the height cache for
	// the still-visible rows stays valid and MinSize re-measures nothing.
	rich.setHiddenRanges([]LineRange{{Start: 0, End: 0}})
	r.MinSize()
	assert.Equal(t, nv1, *nv, "folding must not force a re-measure of unchanged rows")
}
