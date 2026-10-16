package widget

import (
	"math"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// virtualBufferPixels is the vertical over-scan (above and below the viewport)
// the variable-height window materialises so a small scroll does not immediately
// require a re-window. Expressed in pixels, not rows: with variable heights a tall
// image row would make a fixed row buffer span an unpredictable distance (pre-rebrand tracker #158).
const virtualBufferPixels = float32(240)

// virtualWidthSampleRows bounds the widest-row scan for a wrap-off variable-height
// document. Scanning every row to find the widest would be O(rows) shaping and
// partly defeat virtualisation on a huge wrap-off document (pre-rebrand tracker #170). Instead the
// natural width is estimated from a bounded uniform sample of rows plus the
// currently materialised window, and only ever grows (see widenVirtualWidthForWindow)
// so the horizontal scroll extent always covers what the user can actually see and
// never snaps narrower while scrolling. The off-screen extent can momentarily
// under-report if the single widest row falls outside both the sample and the
// window, correcting as that row scrolls into view — the same estimate-then-refine
// trade the variable-height path already makes for row heights (pre-rebrand tracker #157). Documents
// with no more than this many rows are scanned exactly.
const virtualWidthSampleRows = 512

// virtualContentWidth is the width text/blocks are laid out and measured at in the
// virtualised paths — the widget width minus the left/right content inset. It
// mirrors lineWidth in Layout (no gutter: line numbers are an Entry/single-segment
// feature, and Entry never takes the multi-segment varHeight path).
func (t *RichText) virtualContentWidth() float32 {
	left := t.Theme().Size(theme.SizeNameInnerPadding) - t.inset.Width
	return t.Size().Width - left*2
}

// rowVolatile reports whether a row's height can only be known by measuring a
// non-text widget — an image (size firms up after the canvas exists) or a
// HeightForWidth block (table, code). Such rows are estimated off-screen and
// measured on demand when they enter the window; rows of only text/hyperlink
// segments have stable text-metric heights and are measured exactly and cached.
func (t *RichText) rowVolatile(row int) bool {
	if row < 0 || row >= len(t.rowBounds) {
		return false
	}
	for _, seg := range t.rowBounds[row].segments {
		switch seg.(type) {
		case *TextSegment, *HyperlinkSegment:
			// stable text-metric height
		default:
			return true
		}
	}
	return false
}

// rowHeightOrEstimate returns row's height for the variable-height prefix: the
// cached measured value when present; an exactly-measured (and cached) value for
// stable text rows; or the running-average estimate for a not-yet-measured
// volatile row (images/tables refine to exact once they enter the window, pre-rebrand tracker #157
// scroll-extent jitter). The cache must already be sized (ensureHeightPrefix
// calls ensureRowHeightCache first).
func (t *RichText) rowHeightOrEstimate(row int) float32 {
	if row >= 0 && row < len(t.rowHeightCache) && !math.IsNaN(float64(t.rowHeightCache[row])) {
		return t.rowHeightCache[row]
	}
	if t.rowVolatile(row) {
		if t.volatileEstCount > 0 {
			return t.volatileEstAvg
		}
		return t.stride()
	}
	h := t.rowVisualHeight(row)
	if row >= 0 && row < len(t.rowHeightCache) {
		t.rowHeightCache[row] = h
	}
	return h
}

// measureVolatileRow measures a volatile row exactly at the current content width
// (HeightForWidth for table/code blocks, MinSize for images) and stores it in the
// height cache. It folds the first measurement of each row into the volatile-row
// height average (the off-screen estimate source). Returns whether the stored
// height changed, so the caller can mark the prefix dirty.
func (t *RichText) measureVolatileRow(row int) bool {
	if row < 0 || row >= len(t.rowBounds) || row >= len(t.rowHeightCache) {
		return false
	}
	width := t.virtualContentWidth()
	var h float32
	for _, seg := range t.rowBounds[row].segments {
		if text, ok := seg.(*TextSegment); ok {
			ms, _ := bellui.MeasureTextFull(text.Text, text.size(), text.Style.TextStyle, text.Style.FontSource)
			if ms.Height > h {
				h = ms.Height
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
	}
	prev := t.rowHeightCache[row]
	t.rowHeightCache[row] = h
	if math.IsNaN(float64(prev)) { // first measurement of this row feeds the estimate
		t.volatileEstCount++
		t.volatileEstAvg += (h - t.volatileEstAvg) / float32(t.volatileEstCount)
		return true
	}
	return prev != h
}

// spacingAfter is the inter-row line-spacing added below row, matching the dense
// path: spacing follows a non-inline row that is not continued by a wrapped
// fragment of the same segment.
func (t *RichText) spacingAfter(row int) float32 {
	n := len(t.rowBounds)
	if row < 0 || row+1 >= n {
		return 0
	}
	b := t.rowBounds[row]
	lastSeg := b.segments[len(b.segments)-1]
	if !lastSeg.Inline() && t.rowBounds[row+1].segments[0] != lastSeg {
		return t.Theme().Size(theme.SizeNameLineSpacing)
	}
	return 0
}

// ensureHeightPrefix (re)builds rowYPrefix, the prefix sum of per-row heights with
// folds (zero height) and inter-row spacing baked in, so rowYPrefix[i] is the top
// of row i and rowYPrefix[n] is the total content height. O(rows) float work and
// no shaping beyond first-time text measurement (cached). Rebuilt when the width
// or rowBounds version changes, or a row height/fold refined (prefixHeightDirty).
func (t *RichText) ensureHeightPrefix() {
	n := len(t.rowBounds)
	width := t.virtualContentWidth()
	if !t.prefixHeightDirty &&
		len(t.rowYPrefix) == n+1 &&
		t.prefixHeightWidth == width &&
		t.prefixHeightVer == t.rowBoundsVersion {
		return
	}
	t.ensureRowHeightCache(width, t.Theme())
	if cap(t.rowYPrefix) >= n+1 {
		t.rowYPrefix = t.rowYPrefix[:n+1]
	} else {
		t.rowYPrefix = make([]float32, n+1)
	}
	var acc float32
	for i := 0; i < n; i++ {
		t.rowYPrefix[i] = acc
		if !t.rowHidden(i) {
			acc += t.rowHeightOrEstimate(i) + t.spacingAfter(i)
		}
	}
	t.rowYPrefix[n] = acc
	t.prefixHeightDirty = false
	t.prefixHeightWidth = width
	t.prefixHeightVer = t.rowBoundsVersion
}

// rowGeometryVar serves RowGeometry for the variable-height model: Y from the
// prefix sum (folds already collapsed to zero height), height from the per-row
// measure/estimate. O(1) once the prefix is built.
func (t *RichText) rowGeometryVar(row int) (yPos, height float32) {
	t.ensureHeightPrefix()
	if row < 0 || row >= len(t.rowBounds) {
		return 0, t.charMinSize(false, bellui.TextStyle{}, t.Theme().Size(theme.SizeNameText)).Height
	}
	y := t.contentTopOffset() + t.rowYPrefix[row]
	if t.rowHidden(row) {
		return y, 0
	}
	return y, t.rowHeightOrEstimate(row)
}

// varHeightVirtualWidth estimates the widest wrap-off row's width for the virtual
// MinSize without the O(rows) scan that would shape every row (pre-rebrand tracker #170). It measures a
// bounded uniform sample across the document plus every materialised window row, so
// the visible region is always covered exactly while the cost stays O(sample+window).
// This is the baseline computed once per rowBoundsVersion (virtualWidth caches it);
// widenVirtualWidthForWindow then grows the cached value as later windows scroll wide
// rows into view, so the reported extent is monotonic for the lifetime of a version.
func (t *RichText) varHeightVirtualWidth() float32 {
	n := len(t.rowBounds)
	if n == 0 {
		return 0
	}
	textSize := t.Theme().Size(theme.SizeNameText)
	var widest float32
	measure := func(i int) {
		if i < 0 || i >= n {
			return
		}
		if rw := t.rowWidth(i, textSize); rw > widest {
			widest = rw
		}
	}
	if n <= virtualWidthSampleRows {
		for i := 0; i < n; i++ {
			measure(i)
		}
		return widest
	}
	step := float64(n) / float64(virtualWidthSampleRows)
	for k := 0; k < virtualWidthSampleRows; k++ {
		measure(int(float64(k) * step))
	}
	for _, row := range t.winRows { // visible rows are always covered exactly
		measure(row)
	}
	return widest
}

// widenVirtualWidthForWindow grows the cached wrap-off varHeight virtual width to
// cover the freshly materialised window rows, so scrolling onto a row wider than the
// bounded sample (pre-rebrand tracker #170) extends the horizontal scroll extent in place. Called after
// each re-window; monotonic and O(window). A no-op until varHeightVirtualWidth has
// computed the baseline for the current version (then minVirtual reads the grown
// cache directly without re-sampling).
func (t *RichText) widenVirtualWidthForWindow() {
	if (!t.varHeight && !t.blockLazy) || t.Wrapping != bellui.TextWrapOff {
		return
	}
	if !t.virtWidthOK || t.virtWidthVer != t.rowBoundsVersion {
		return
	}
	textSize := t.Theme().Size(theme.SizeNameText)
	for _, row := range t.winRows {
		if rw := t.rowWidth(row, textSize); rw > t.virtWidth {
			t.virtWidth = rw
		}
	}
}

// rowAtYVar serves rowAtY for the variable-height model: a binary search over the
// height prefix sum (O(log rows)). Folded rows have zero extent in the prefix, so
// a hit on a fold seam is snapped forward to the next visible row.
func (t *RichText) rowAtYVar(y float32) int {
	t.ensureHeightPrefix()
	n := len(t.rowBounds)
	if n == 0 {
		return -1
	}
	target := y - t.contentTopOffset()
	if target < 0 {
		return t.firstVisibleAtOrAfter(0)
	}
	i := sort.Search(n, func(i int) bool { return t.rowYPrefix[i+1] > target })
	if i >= n {
		i = n - 1
	}
	if t.rowHidden(i) {
		if v := t.firstVisibleAtOrAfter(i); v < n {
			i = v
		}
	}
	return i
}

// computeWindowRowsVar returns the visible-window rows for the variable-height
// model: a binary search for the first row crossing the over-scanned viewport
// top, then a forward walk accumulating until past the bottom. Volatile rows are
// measured as they enter the window, refining the prefix.
func (t *RichText) computeWindowRowsVar() []int {
	t.ensureHeightPrefix()
	n := len(t.rowBounds)
	if n == 0 {
		return nil
	}
	top0 := t.contentTopOffset()
	h := t.viewHeight
	if h <= 0 {
		h = 40 * t.stride() // bootstrap before the real viewport is known
	}
	bandTop := t.viewTop - top0 - virtualBufferPixels
	bandBot := t.viewTop - top0 + h + virtualBufferPixels
	if bandTop < 0 {
		bandTop = 0
	}

	start := sort.Search(n, func(i int) bool { return t.rowYPrefix[i+1] > bandTop })
	if start >= n {
		start = n - 1
	}

	var rows []int
	refined := false
	for row := start; row < n && t.rowYPrefix[row] < bandBot; row++ {
		if t.rowHidden(row) {
			continue
		}
		if t.rowVolatile(row) && t.measureVolatileRow(row) {
			t.prefixHeightDirty = true
			refined = true
		}
		rows = append(rows, row)
	}
	if refined {
		// Make the geometry consistent with the freshly measured heights; the
		// buffer absorbs the small band shift, and the next pass converges.
		t.ensureHeightPrefix()
	}
	return rows
}

// layoutVirtualVar positions the window's multi-segment rows for the variable-
// height model. Unlike the single-object-per-row lazy/uniform paths, a row here
// owns several objects (one per segment) laid out via the shared per-row layout;
// each row is pinned to its prefix-sum Y, so a huge document lays out in
// O(window) time with no dense geometry arrays.
func (r *textRenderer) layoutVirtualVar(size geo.Size) {
	t := r.obj
	if t.isSelectable() && t.selection != nil {
		t.selection.Resize(size)
	}
	t.ensureHeightPrefix()
	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	lineSpacing := th.Size(theme.SizeNameLineSpacing)
	t.GutterWidth = 0 // line numbers are a single-segment/Entry feature
	xInset := innerPadding - t.inset.Width
	lineWidth := size.Width - xInset*2

	// No dense geometry arrays in the variable-height path — RowGeometry serves Y
	// from the prefix sum on demand.
	t.rowYPos = nil
	t.rowHeight = nil

	objs := r.origObjs
	oi := 0
	for _, row := range t.winRows {
		if row < 0 || row >= len(t.rowBounds) {
			continue
		}
		yTop, _ := t.RowGeometry(row)
		consumed, _ := r.layoutOneRow(t.rowBounds[row], objs, oi, yTop, xInset, lineWidth, lineSpacing)
		oi += consumed
		if oi >= len(objs) {
			break
		}
	}
}
