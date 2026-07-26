package widget

import (
	"math"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// This file implements the windowed multi-segment ("block-lazy") build (#317):
// the cold-build analogue of the single-segment lazy model (richtext_lazy.go).
//
// A virtualised multi-segment varHeight RichText already windows its rendering
// (scroll is O(window), #158), but its cold build still shapes every row up front
// in updateRowBounds to learn the wrap boundaries — ~58% MeasureText / HarfBuzz on
// a 10k-segment document, all before the first frame. This model removes that floor
// by shaping rows only when they are addressed.
//
// The model keeps one entry per *logical block* — a maximal inline run terminated
// by a non-inline segment (the points where updateRowBounds' iterateSegments resets
// currentBound). A block's visual-row count and height are estimated from a cheap
// rune scan until something addresses one of its rows (ensureBlockMeasured), which
// shapes the block in isolation. Because every block starts at a "fresh" point in
// the full pass (currentBound==nil, wrapWidth==maxWidth), shaping a block on its own
// yields byte-identical rowBounds to that block's slice of the dense build, so the
// rendered output is unchanged (guarded by TestRichText_BlockLazy_ShapeMatchesDense).
//
// Block-level prefix sums (blockStartRow for the row axis, blockTopY for the Y axis)
// give O(log blocks) row<->Y mapping; they are refined as blocks are measured,
// converging the estimated scroll extent to exact — the same estimate-then-refine
// trade #157/#158 already make.

// richBlock is the cheap structural description of one logical block, built without
// shaping. runeLen/newlines drive the row-count estimate; volatile marks a block
// containing a non-text segment (image/table/separator) whose height needs measuring.
type richBlock struct {
	segs     []RichTextSegment
	runeLen  int  // total text runes across the block's text segments
	newlines int  // hard line breaks (each forces a visual row)
	volatile bool // contains a non-text segment
}

// shapeBlock shapes one block's leaf segments into exact rowBounds, reproducing
// updateRowBounds' iterateSegments inner loop for the no-truncation case (truncation
// is always off when block-lazy is active). The block starts from a fresh row
// (currentBound==nil, wrapWidth==maxWidth) — the definition of a block boundary — so
// the result matches the same block's slice of a full dense build exactly, including
// per-row firstSegmentReuse (set per-segment by lineBounds).
func (t *RichText) shapeBlock(segs []RichTextSegment, maxWidth, innerPadding float32) []rowBoundary {
	var bounds []rowBoundary
	var currentBound *rowBoundary
	wrapWidth := maxWidth
	bigHeight := geo.NewSize(maxWidth, float32(math.MaxInt32))
	for _, seg := range segs {
		if _, ok := seg.(*TextSegment); !ok {
			// A block (non-inline) segment fills the whole line width, so it cannot
			// share a row with a trailing inline run not terminated by a newline
			// (#207): close the open inline row first.
			if !seg.Inline() && currentBound != nil {
				currentBound = nil
				wrapWidth = maxWidth
			}
			if currentBound == nil {
				bound := rowBoundary{segments: []RichTextSegment{seg}}
				bounds = append(bounds, bound)
				currentBound = &bound
			} else {
				bounds[len(bounds)-1].segments = append(bounds[len(bounds)-1].segments, seg)
			}
			itemMin := t.cachedSegmentVisual(seg, 0).MinSize()
			if seg.Inline() {
				wrapWidth -= itemMin.Width
			} else {
				wrapWidth = maxWidth
				currentBound = nil
			}
			continue
		}
		textSeg := seg.(*TextSegment)
		textStyle := textSeg.Style.TextStyle
		textSize := textSeg.size()
		textSource := textSeg.Style.FontSource

		leftPad := float32(0)
		if textSeg.Style.isBlockquote() {
			leftPad = innerPadding * 2
		}
		retBounds, _ := lineBounds(textSeg, t.Wrapping, bellui.TextTruncateOff, wrapWidth-leftPad, bigHeight, func(text []rune) geo.Size {
			m, _ := bellui.MeasureTextFull(string(text), textSize, textStyle, textSource)
			return m
		})
		if currentBound != nil {
			if len(retBounds) > 0 {
				bounds[len(bounds)-1].end = retBounds[0].end
				bounds[len(bounds)-1].segments = append(bounds[len(bounds)-1].segments, seg)
				bounds = append(bounds, retBounds[1:]...)
			}
		} else {
			bounds = append(bounds, retBounds...)
		}
		currentBound = &bounds[len(bounds)-1]
		if seg.Inline() {
			last := bounds[len(bounds)-1]
			begin := 0
			if len(last.segments) == 1 {
				begin = last.begin
			}
			runes := []rune(textSeg.Text)
			if begin > len(runes) {
				begin = len(runes)
			}
			end := last.end
			if end > len(runes) {
				end = len(runes)
			}
			text := string(runes[begin:end])
			lastWidthSize, _ := bellui.MeasureTextFull(text, textSeg.size(), textSeg.Style.TextStyle, textSeg.Style.FontSource)
			lastWidth := lastWidthSize.Width
			if len(retBounds) == 1 {
				wrapWidth -= lastWidth
			} else {
				wrapWidth = maxWidth - lastWidth
			}
		} else {
			currentBound = nil
			wrapWidth = maxWidth
		}
	}
	return bounds
}

// buildBlocks partitions the (flattened) segment tree into logical blocks without
// shaping. A block boundary falls after every non-inline leaf segment and after a
// RichTextBlock whose last child is non-inline — exactly the points where
// iterateSegments resets currentBound — so each block begins at a fresh-state point.
func (t *RichText) buildBlocks() []richBlock {
	var blocks []richBlock
	var cur []RichTextSegment
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, t.makeBlock(cur))
			cur = nil
		}
	}
	var walk func(segs []RichTextSegment)
	walk = func(segs []RichTextSegment) {
		for _, seg := range segs {
			if parent, ok := seg.(RichTextBlock); ok {
				children := parent.Segments()
				walk(children)
				if len(children) > 0 && !children[len(children)-1].Inline() {
					flush()
				}
				continue
			}
			cur = append(cur, seg)
			if !seg.Inline() {
				flush()
			}
		}
	}
	walk(t.Segments)
	flush()
	return blocks
}

// makeBlock records a block's cheap structural metrics (rune length, hard newline
// count, volatility) from its segments — one O(runes) scan, no shaping.
func (t *RichText) makeBlock(segs []RichTextSegment) richBlock {
	b := richBlock{segs: append([]RichTextSegment(nil), segs...)}
	for _, seg := range segs {
		if text, ok := seg.(*TextSegment); ok {
			for _, r := range text.Text {
				b.runeLen++
				if r == '\n' {
					b.newlines++
				}
			}
		} else {
			b.volatile = true
		}
	}
	return b
}

// blockLazyEligible reports whether the document should use the windowed block
// build: virtualisation opted in, truncation off, wrapping in {off, word, break},
// more than one segment, no active folds (those keep the dense varHeight path), and
// a structural row estimate large enough that windowing pays off.
func (t *RichText) blockLazyEligible(width float32) bool {
	if !t.virtualize || t.Truncation != bellui.TextTruncateOff {
		return false
	}
	switch t.Wrapping {
	case bellui.TextWrapOff, bellui.TextWrapWord, bellui.TextWrapBreak:
	default:
		return false
	}
	if _, ok := t.singleSegment(); ok {
		return false
	}
	if len(t.hiddenLogical) > 0 {
		return false
	}
	if len(t.Segments) == 0 {
		return false
	}
	return t.estimateTotalRows(width) > varHeightMinRows
}

// estimateTotalRows builds the block list and sums each block's estimated visual-row
// count — the cheap (shaping-free) basis for deciding whether to window the build.
// It populates t.blocks so updateRowBoundsBlockLazy can reuse the partition.
func (t *RichText) estimateTotalRows(width float32) int {
	innerPadding := t.Theme().Size(theme.SizeNameInnerPadding)
	maxWidth := width - 2*innerPadding + 2*t.inset.Width
	t.blocks = t.buildBlocks()
	t.blockMaxWidth = maxWidth
	t.blockInnerPad = innerPadding
	t.blockEstCharsPerRow = t.computeBlockEstCharsPerRow(maxWidth)
	total := 0
	for i := range t.blocks {
		total += t.estimateBlockRows(i)
	}
	return total
}

// computeBlockEstCharsPerRow estimates the average glyphs that fit in maxWidth at
// the base text size from one sample measurement, reused as the wrap estimate
// denominator. Mirrors the single-segment computeEstCharsPerRow.
func (t *RichText) computeBlockEstCharsPerRow(maxWidth float32) float32 {
	if maxWidth <= 0 || t.Wrapping == bellui.TextWrapOff {
		return 1
	}
	// Sample text from the first text-bearing block.
	var sample string
	for _, blk := range t.blocks {
		for _, seg := range blk.segs {
			if text, ok := seg.(*TextSegment); ok && len(text.Text) > 0 {
				sample = text.Text
				break
			}
		}
		if sample != "" {
			break
		}
	}
	if sample == "" {
		return 1
	}
	runes := []rune(sample)
	if len(runes) > 200 {
		runes = runes[:200]
	}
	for i, r := range runes {
		if r == '\n' {
			runes[i] = ' '
		}
	}
	size := t.Theme().Size(theme.SizeNameText)
	w := bellui.MeasureText(string(runes), size, bellui.TextStyle{}).Width
	if w <= 0 {
		return 1
	}
	per := maxWidth / (w / float32(len(runes)))
	if per < 1 {
		per = 1
	}
	return per
}

// estimateBlockRows returns block i's estimated visual-row count: for wrap-off, one
// row per hard line; otherwise the hard-line count plus a wrap estimate of the
// non-newline runes. Floor of 1.
func (t *RichText) estimateBlockRows(i int) int {
	blk := &t.blocks[i]
	lines := blk.newlines + 1
	if t.Wrapping == bellui.TextWrapOff {
		if blk.volatile && blk.runeLen == 0 {
			return 1
		}
		return lines
	}
	rows := lines
	if t.blockEstCharsPerRow > 0 {
		nonNL := blk.runeLen - blk.newlines
		if nonNL < 0 {
			nonNL = 0
		}
		wrapRows := int(math.Ceil(float64(nonNL) / float64(t.blockEstCharsPerRow)))
		if wrapRows > rows {
			rows = wrapRows
		}
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// estimateBlockRowHeight returns block i's estimated per-row height: a volatile
// block uses the running volatile-row mean (or stride), a text block the maximum
// text-segment line height in the block.
func (t *RichText) estimateBlockRowHeight(i int) float32 {
	blk := &t.blocks[i]
	var h float32
	for _, seg := range blk.segs {
		if text, ok := seg.(*TextSegment); ok {
			// Estimate from the style's nominal (ASCII) line height — exact heights
			// are deferred to ensureBlockMeasured, so this never shapes the block's
			// (possibly non-ASCII) text up front.
			if m := t.styleNominalHeight(text.size(), text.Style.TextStyle, text.Style.FontSource); m > h {
				h = m
			}
		}
	}
	if blk.volatile {
		var vol float32
		if t.volatileEstCount > 0 {
			vol = t.volatileEstAvg
		} else {
			vol = t.stride()
		}
		if vol > h {
			h = vol
		}
	}
	if h <= 0 {
		h = t.stride()
	}
	return h
}

// updateRowBoundsBlockLazy (re)builds the block-lazy model. It does no shaping: it
// partitions segments into blocks (reusing the partition estimateTotalRows already
// built), seeds each block's estimated row count/height, and builds the prefix sums.
// Blocks are shaped on demand later (ensureBlockMeasured).
func (t *RichText) updateRowBoundsBlockLazy(width float32) {
	t.blockLazy = true
	t.varHeight = false
	t.lazy = false
	t.rowBounds = nil
	t.rowToLine = nil

	// estimateTotalRows (called from blockLazyEligible) already built t.blocks and
	// set blockMaxWidth/blockInnerPad/blockEstCharsPerRow for this width.
	n := len(t.blocks)
	t.blockMeasured = make([]bool, n)
	t.blockRows = make([][]rowBoundary, n)
	t.blockRowHeights = make([][]float32, n)
	t.blockRowCount = make([]int, n)
	t.blockHeight = make([]float32, n)
	t.blockEstRowH = make([]float32, n)
	for i := 0; i < n; i++ {
		rows := t.estimateBlockRows(i)
		rh := t.estimateBlockRowHeight(i)
		t.blockRowCount[i] = rows
		t.blockEstRowH[i] = rh
		t.blockHeight[i] = float32(rows) * rh
	}
	t.blockPrefixDirty = true
	t.ensureBlockPrefix()

	t.rowBoundsComputed = t.rowBoundsVersion
	t.rowBoundsWidth = t.Size().Width
	t.rowBoundsTruncation = t.Truncation
	t.rowBoundsWrapping = t.Wrapping
}

// updateRowBoundsBlockLazyReset drops any stale block-lazy model so the dense or
// single-segment lazy accessors do not see it. Called when the build switches away
// from the block-lazy model.
func (t *RichText) updateRowBoundsBlockLazyReset() {
	if !t.blockLazy && t.blocks == nil {
		return
	}
	t.blockLazy = false
	t.blocks = nil
	t.blockMeasured = nil
	t.blockRows = nil
	t.blockRowHeights = nil
	t.blockRowCount = nil
	t.blockHeight = nil
	t.blockEstRowH = nil
	t.blockStartRow = nil
	t.blockTopY = nil
}

// blockEndsNonInline reports whether block i's last segment is non-inline, i.e. a
// line-spacing gap follows it (matching the dense spacingAfter rule).
func (t *RichText) blockEndsNonInline(i int) bool {
	segs := t.blocks[i].segs
	if len(segs) == 0 {
		return false
	}
	return !segs[len(segs)-1].Inline()
}

// ensureBlockPrefix rebuilds blockStartRow (row-count prefix) and blockTopY (Y
// prefix) when a block's count or height changed. O(blocks) integer/float work, no
// shaping. blockTopY bakes in the inter-block line spacing that follows each
// non-inline block.
func (t *RichText) ensureBlockPrefix() {
	n := len(t.blocks)
	if !t.blockPrefixDirty && len(t.blockStartRow) == n+1 && len(t.blockTopY) == n+1 {
		return
	}
	if cap(t.blockStartRow) >= n+1 {
		t.blockStartRow = t.blockStartRow[:n+1]
	} else {
		t.blockStartRow = make([]int, n+1)
	}
	if cap(t.blockTopY) >= n+1 {
		t.blockTopY = t.blockTopY[:n+1]
	} else {
		t.blockTopY = make([]float32, n+1)
	}
	lineSpacing := t.Theme().Size(theme.SizeNameLineSpacing)
	accRow := 0
	var accY float32
	for i := 0; i < n; i++ {
		t.blockStartRow[i] = accRow
		t.blockTopY[i] = accY
		accRow += t.blockRowCount[i]
		accY += t.blockHeight[i]
		if i < n-1 && t.blockEndsNonInline(i) {
			accY += lineSpacing
		}
	}
	t.blockStartRow[n] = accRow
	t.blockTopY[n] = accY
	t.blockPrefixDirty = false
}

// blockLazyRowCount returns the (estimated, converging) total visual-row count.
func (t *RichText) blockLazyRowCount() int {
	if len(t.blocks) == 0 {
		return 0
	}
	t.ensureBlockPrefix()
	return t.blockStartRow[len(t.blocks)]
}

// blockOfRow maps an absolute visual row to its block and sub-row offset.
func (t *RichText) blockOfRow(row int) (block, sub int) {
	t.ensureBlockPrefix()
	n := len(t.blocks)
	if n == 0 {
		return 0, 0
	}
	if row < 0 {
		row = 0
	}
	if total := t.blockStartRow[n]; row >= total && total > 0 {
		row = total - 1
	}
	block = sort.Search(n, func(i int) bool { return t.blockStartRow[i+1] > row })
	if block >= n {
		block = n - 1
	}
	return block, row - t.blockStartRow[block]
}

// ensureBlockMeasured shapes block b's rows (exact bounds and per-row heights) if not
// already measured, replacing its estimate and marking the prefixes dirty when the
// count or height changed. Idempotent.
func (t *RichText) ensureBlockMeasured(b int) {
	if b < 0 || b >= len(t.blocks) || t.blockMeasured[b] {
		return
	}
	rows := t.shapeBlock(t.blocks[b].segs, t.blockMaxWidth, t.blockInnerPad)
	if len(rows) == 0 {
		// An empty block still occupies one row (shapeBlock mirrors lineBounds, which
		// appends a row for empty text); guard against a zero-row block regardless.
		rows = []rowBoundary{{segments: t.blocks[b].segs}}
	}
	heights := make([]float32, len(rows))
	var total float32
	for i := range rows {
		h := t.measureBlockRow(rows[i])
		heights[i] = h
		total += h
	}
	t.blockRows[b] = rows
	t.blockRowHeights[b] = heights
	t.blockMeasured[b] = true
	changed := false
	if len(rows) != t.blockRowCount[b] {
		t.blockRowCount[b] = len(rows)
		changed = true
	}
	if total != t.blockHeight[b] {
		t.blockHeight[b] = total
		changed = true
	}
	if changed {
		t.blockPrefixDirty = true
	}
}

// measureBlockRow measures one shaped row's height: the max line height of its text
// segments, or — for a volatile (non-text) segment — its HeightForWidth/MinSize,
// folded into the volatile-row running mean so unmeasured volatile blocks estimate
// from real data (mirrors measureVolatileRow).
func (t *RichText) measureBlockRow(bound rowBoundary) float32 {
	width := t.virtualContentWidth()
	var h float32
	for _, seg := range bound.segments {
		if text, ok := seg.(*TextSegment); ok {
			if m := t.textSegmentHeight(text); m > h {
				h = m
			}
			continue
		}
		obj := t.cachedSegmentVisual(seg, 0)
		seg.Update(obj)
		var m float32
		if hfw, ok := obj.(interface{ HeightForWidth(float32) float32 }); ok && width > 0 {
			m = hfw.HeightForWidth(width)
		} else {
			m = obj.MinSize().Height
		}
		if m > h {
			h = m
		}
		t.volatileEstCount++
		t.volatileEstAvg += (m - t.volatileEstAvg) / float32(t.volatileEstCount)
	}
	return h
}

// blockLazyRowBoundary returns the boundary of an absolute visual row, measuring its
// block on demand. firstSegmentReuse is rebased to the absolute visual row so the
// window render keys cachedSegmentVisual by a stable per-row offset that never
// collides across independently measured blocks (mirrors lazyRowBoundary).
func (t *RichText) blockLazyRowBoundary(row int) *rowBoundary {
	if row < 0 || row >= t.blockLazyRowCount() {
		return nil
	}
	b, sub := t.blockOfRow(row)
	t.ensureBlockMeasured(b)
	rows := t.blockRows[b]
	if len(rows) == 0 {
		return nil
	}
	if sub < 0 {
		sub = 0
	}
	if sub >= len(rows) {
		sub = len(rows) - 1
	}
	src := rows[sub]
	cp := rowBoundary{
		segments:          src.segments,
		firstSegmentReuse: row,
		begin:             src.begin,
		end:               src.end,
		ellipsis:          src.ellipsis,
	}
	return &cp
}

// rowGeometryBlockLazy serves RowGeometry for the block-lazy model: block top from
// blockTopY, within-block offset from exact (measured) or estimated per-row heights.
func (t *RichText) rowGeometryBlockLazy(row int) (yPos, height float32) {
	t.ensureBlockPrefix()
	n := len(t.blocks)
	if n == 0 || row < 0 || row >= t.blockStartRow[n] {
		return 0, t.charMinSize(false, bellui.TextStyle{}, t.Theme().Size(theme.SizeNameText)).Height
	}
	b, sub := t.blockOfRow(row)
	top := t.contentTopOffset() + t.blockTopY[b]
	if t.blockMeasured[b] {
		hs := t.blockRowHeights[b]
		for i := 0; i < sub && i < len(hs); i++ {
			top += hs[i]
		}
		if sub < len(hs) {
			return top, hs[sub]
		}
		return top, t.blockEstRowH[b]
	}
	return top + float32(sub)*t.blockEstRowH[b], t.blockEstRowH[b]
}

// rowAtYBlockLazy serves rowAtY for the block-lazy model: binary-search the block Y
// prefix, then locate the sub-row within the block (exact heights when measured,
// estimate otherwise).
func (t *RichText) rowAtYBlockLazy(y float32) int {
	t.ensureBlockPrefix()
	n := len(t.blocks)
	if n == 0 {
		return -1
	}
	target := y - t.contentTopOffset()
	if target < 0 {
		return 0
	}
	b := sort.Search(n, func(i int) bool { return t.blockTopY[i+1] > target })
	if b >= n {
		b = n - 1
	}
	local := target - t.blockTopY[b]
	if local < 0 {
		local = 0
	}
	var sub int
	if t.blockMeasured[b] {
		var acc float32
		hs := t.blockRowHeights[b]
		for sub = 0; sub < len(hs)-1; sub++ {
			if acc+hs[sub] > local {
				break
			}
			acc += hs[sub]
		}
	} else if t.blockEstRowH[b] > 0 {
		sub = int(local / t.blockEstRowH[b])
		if sub >= t.blockRowCount[b] {
			sub = t.blockRowCount[b] - 1
		}
		if sub < 0 {
			sub = 0
		}
	}
	row := t.blockStartRow[b] + sub
	if row >= t.blockStartRow[n] {
		row = t.blockStartRow[n] - 1
	}
	return row
}

// computeWindowRowsBlockLazy returns the visible-window rows for the block-lazy
// model: binary-search the first block crossing the over-scanned viewport top, then
// walk blocks forward, shaping each (ensureBlockMeasured) so the band is computed
// against exact rows, collecting rows that intersect the band.
func (t *RichText) computeWindowRowsBlockLazy() []int {
	t.ensureBlockPrefix()
	n := len(t.blocks)
	if n == 0 {
		return nil
	}
	top0 := t.contentTopOffset()
	h := t.viewHeight
	if h <= 0 {
		h = 40 * t.stride()
	}
	bandTop := t.viewTop - top0 - virtualBufferPixels
	bandBot := t.viewTop - top0 + h + virtualBufferPixels
	if bandTop < 0 {
		bandTop = 0
	}

	start := sort.Search(n, func(i int) bool { return t.blockTopY[i+1] > bandTop })
	if start >= n {
		start = n - 1
	}

	var rows []int
	for b := start; b < n && t.blockTopY[b] < bandBot; b++ {
		if !t.blockMeasured[b] {
			t.ensureBlockMeasured(b)
			if t.blockPrefixDirty {
				// Measuring the block refined its rows/height; rebuild the prefix so
				// blockTopY for this and following blocks is exact. blockTopY[b] is
				// unaffected (it depends only on earlier blocks), so the walk is
				// consistent; the over-scan buffer absorbs the small band shift.
				t.ensureBlockPrefix()
			}
		}
		// Collect this block's rows that fall in the band.
		base := t.blockStartRow[b]
		y := t.blockTopY[b]
		hs := t.blockRowHeights[b]
		for sub := 0; sub < t.blockRowCount[b]; sub++ {
			rh := t.blockEstRowH[b]
			if sub < len(hs) {
				rh = hs[sub]
			}
			if y+rh > bandTop && y < bandBot {
				rows = append(rows, base+sub)
			}
			y += rh
		}
	}
	return rows
}

// blockLazyVirtualWidth estimates the widest row's width for the virtual MinSize. For
// wrapped content every row fits the content width, so (like the dense path) it
// reports 0; only wrap-off content needs a widest-row scan. To stay shaping-free it
// samples a bounded set of blocks by rune length and measures the widest block's
// first line, plus the materialised window, growing monotonically as wide rows
// scroll in (mirrors varHeightVirtualWidth + widenVirtualWidthForWindow).
func (t *RichText) blockLazyVirtualWidth() float32 {
	if t.Wrapping != bellui.TextWrapOff {
		return 0
	}
	n := len(t.blocks)
	if n == 0 {
		return 0
	}
	textSize := t.Theme().Size(theme.SizeNameText)
	var widest float32
	measureBlock := func(i int) {
		if i < 0 || i >= n {
			return
		}
		t.ensureBlockMeasured(i)
		base := t.blockStartRow[i]
		for sub := range t.blockRows[i] {
			if rw := t.rowWidth(base+sub, textSize); rw > widest {
				widest = rw
			}
		}
	}
	if n <= virtualWidthSampleRows {
		for i := 0; i < n; i++ {
			measureBlock(i)
		}
	} else {
		step := float64(n) / float64(virtualWidthSampleRows)
		for k := 0; k < virtualWidthSampleRows; k++ {
			measureBlock(int(float64(k) * step))
		}
		for _, row := range t.winRows {
			b, _ := t.blockOfRow(row)
			measureBlock(b)
		}
	}
	return widest
}

// layoutVirtualBlockLazy positions the window's rows for the block-lazy model. Each
// window row is materialised on demand via blockLazyRowBoundary and pinned to its
// prefix-derived Y, so a huge document lays out in O(window) time with no dense
// geometry arrays (mirrors layoutVirtualVar over the block model).
func (r *textRenderer) layoutVirtualBlockLazy(size geo.Size) {
	t := r.obj
	if t.isSelectable() && t.selection != nil {
		t.selection.Resize(size)
	}
	t.ensureBlockPrefix()
	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	lineSpacing := th.Size(theme.SizeNameLineSpacing)
	t.GutterWidth = 0 // line numbers are a single-segment/Entry feature
	xInset := innerPadding - t.inset.Width
	lineWidth := size.Width - xInset*2

	// No dense geometry arrays — RowGeometry serves Y from the prefix on demand.
	t.rowYPos = nil
	t.rowHeight = nil

	objs := r.origObjs
	oi := 0
	for _, row := range t.winRows {
		bound := t.blockLazyRowBoundary(row)
		if bound == nil {
			continue
		}
		yTop, _ := t.RowGeometry(row)
		consumed, _ := r.layoutOneRow(*bound, objs, oi, yTop, xInset, lineWidth, lineSpacing)
		oi += consumed
		if oi >= len(objs) {
			break
		}
	}
}
