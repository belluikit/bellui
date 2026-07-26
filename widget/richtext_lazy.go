package widget

import (
	"math"
	"sort"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/textbuf"
	"github.com/belluikit/bellui/theme"
)

// lazyMeasureAllMaxLines is the logical-line count below which a wrapped document
// is measured eagerly (every line shaped up front) rather than estimated. Eager
// measurement keeps the row count, caret and selection exact and matches the dense
// path; it is cheap at this scale. Above the threshold, unmeasured lines carry a
// ceil(runeLen/estCharsPerRow) estimate and are shaped only when the viewport (or a
// caret/selection operation) reaches them — the #157 windowed-measurement goal.
const lazyMeasureAllMaxLines = 2000

// This file implements the lazy/windowed wrap-measurement row model (#157) used
// when canVirtualize() holds (a single TextSegment, truncation off, scrolled by an
// Entry). The dense rowBounds path is untouched; the shared accessors in
// richtext.go branch on t.lazy to serve the structures built here.
//
// The model keeps one entry per *logical* line (newline-delimited, known from a
// cheap rune scan) instead of one per *visual* row. A line's wrapped sub-rows are
// shaped only when something addresses one of its rows (ensureLineMeasured); until
// then its visual-row count is a ceil(runeLen/estCharsPerRow) estimate. The
// uniform stride invariant means a row's Y is analytic, so only the per-line row
// COUNT — needed for the total scroll extent and the visual-row<->line mapping — is
// approximate, converging to exact as the user scrolls. See SetVisibleRange /
// computeWindowRows for the windowed measurement.

// updateRowBoundsLazy (re)builds the lazy row model for the current single segment
// and width. It does no shaping: it scans newlines for the logical-line starts and
// estimates each line's wrapped-row count. Lines are shaped on demand later. When
// folds are active it measures eagerly so the visual fold extent is exact (a folded
// huge document keeps render windowing but loses measurement laziness — #157).
func (t *RichText) updateRowBoundsLazy(width float32) {
	seg, _ := t.singleSegment()
	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	maxWidth := width - 2*innerPadding + 2*t.inset.Width

	t.lazy = true
	t.varHeight = false // single-segment uses the uniform-stride model, not varHeight (#158)
	t.rowBounds = nil   // dense model is not used in the lazy path
	t.lazyMaxWidth = maxWidth
	// Reuse the canonical buffer when it already reflects seg.Text (maintained
	// incrementally by the lazy edit path, or built by a prior pass for the same
	// content), skipping the O(document) full build. The string compare is O(1)
	// when seg.Text is the same object the buffer last materialized — the steady
	// state after an edit — and falls back to a full build on a genuine content
	// change (#380). lazyFullBuilds counts those rebuilds so tests can assert the
	// fast edit path never silently degrades to per-keystroke rebuilds (#400).
	if seg.Text != t.lazyStr || t.buf == nil {
		t.buf = textbuf.FromString(seg.Text)
		t.lazyStr = seg.Text    // anchor to this exact object; FromString seeded the memo with it
		t.deferredDirty = false // an external write wins over any deferred edits (#410)
		t.lazyFullBuilds++
	}

	L := t.buf.LineCount()
	t.lineMeasured = make([]bool, L)
	t.lineWrap = make([][]rowBoundary, L)
	t.lineRowCount = make([]int, L)

	wrapOff := t.Wrapping == bellui.TextWrapOff
	if wrapOff {
		for i := range t.lineRowCount {
			t.lineRowCount[i] = 1
		}
	} else {
		t.estCharsPerRow = t.computeEstCharsPerRow(seg, maxWidth)
		// One vectorized buffer walk instead of a per-line LineStart query —
		// per-line queries cost a chunk scan each on the rope backing (#403).
		t.buf.ForEachLine(func(line, start, end int) bool {
			t.lineRowCount[line] = t.estimateRowCountForLen(end - start)
			return true
		})
	}
	t.vwDirty = true
	t.prefixDirty = true
	t.ensurePrefix()

	// Estimation only pays off for genuinely large wrapped documents (#157). Below
	// the threshold, or when folds are active (a fold's collapsed extent needs exact
	// per-line counts), measure every line up front: it is cheap at this size and
	// keeps the visual-row count, caret and selection exact — identical to the dense
	// path. Wrap-off lines are always exact (one row each) so they need no eager pass.
	eager := !wrapOff && (L <= lazyMeasureAllMaxLines || len(t.hiddenLogical) > 0)
	if eager {
		for i := 0; i < L; i++ {
			t.ensureLineMeasured(i)
		}
		t.ensurePrefix()
	}

	t.rowBoundsComputed = t.rowBoundsVersion
	t.rowBoundsWidth = t.Size().Width
	t.rowBoundsTruncation = t.Truncation
	t.rowBoundsWrapping = t.Wrapping

	// rowToLine is served analytically in the lazy path (firstRowOfLine branches on
	// t.lazy), so the dense per-row slice is cleared.
	t.rowToLine = nil
	t.applyHiddenRanges()
}

// estimateRowCount returns line i's estimated visual-row count: 1 for wrap-off (one
// row per line) or when the line is empty/unmeasurable, else ceil(runeLen/est).
func (t *RichText) estimateRowCount(line int, wrapOff bool) int {
	if wrapOff {
		return 1
	}
	return t.estimateRowCountForLen(t.lineRuneLen(line))
}

// estimateRowCountForLen is estimateRowCount for a known line rune length,
// letting bulk passes avoid the per-line line-index query.
func (t *RichText) estimateRowCountForLen(rl int) int {
	if rl <= 0 || t.estCharsPerRow <= 0 {
		return 1
	}
	n := int(math.Ceil(float64(rl) / float64(t.estCharsPerRow)))
	if n < 1 {
		n = 1
	}
	return n
}

// computeEstCharsPerRow derives the average number of glyphs that fit in maxWidth
// from a single measurement of a content sample (passwordChar when concealed),
// reused as the estimate denominator for unmeasured lines.
func (t *RichText) computeEstCharsPerRow(seg *TextSegment, maxWidth float32) float32 {
	if maxWidth <= 0 {
		return 1
	}
	n := t.buf.Len()
	if n > 400 {
		n = 400
	}
	if n == 0 {
		return 1
	}
	var txt string
	if seg.Style.concealed {
		txt = strings.Repeat(passwordChar, n)
	} else {
		sample := t.buf.Slice(0, n) // a copy — safe to rewrite in place
		for i, r := range sample {
			if r == '\n' {
				sample[i] = ' '
			}
		}
		txt = string(sample)
	}
	m, _ := bellui.MeasureTextFull(txt, seg.size(), seg.Style.TextStyle, seg.Style.FontSource)
	w := m.Width
	if w <= 0 {
		return 1
	}
	per := maxWidth / (w / float32(n))
	if per < 1 {
		per = 1
	}
	return per
}

// lineEnd returns the exclusive rune offset where logical line i ends (the newline
// position, or the text length for the last line).
func (t *RichText) lineEnd(line int) int {
	if line+1 < t.buf.LineCount() {
		return t.buf.LineStart(line+1) - 1 // exclude the '\n'
	}
	return t.buf.Len()
}

// lineRuneLen returns the rune length of logical line i (excluding its newline).
func (t *RichText) lineRuneLen(line int) int {
	return t.lineEnd(line) - t.buf.LineStart(line)
}

// lineText returns logical line i's text (without its newline) from the lazy
// buffer — a zero-copy Substring against the clean memo (#401).
func (t *RichText) lineText(line int) string {
	return t.buf.Substring(t.buf.LineStart(line), t.lineEnd(line))
}

// ensurePrefix rebuilds lineStartRow (prefix sum of lineRowCount) when a row count
// changed. O(lines) integer work, never shaping.
func (t *RichText) ensurePrefix() {
	if !t.prefixDirty && len(t.lineStartRow) == len(t.lineRowCount)+1 {
		return
	}
	L := len(t.lineRowCount)
	if cap(t.lineStartRow) >= L+1 {
		t.lineStartRow = t.lineStartRow[:L+1]
	} else {
		t.lineStartRow = make([]int, L+1)
	}
	acc := 0
	for i := 0; i < L; i++ {
		t.lineStartRow[i] = acc
		acc += t.lineRowCount[i]
	}
	t.lineStartRow[L] = acc
	t.prefixDirty = false
}

// lazyRowCount returns the (estimated, converging) total visual-row count.
func (t *RichText) lazyRowCount() int {
	if len(t.lineRowCount) == 0 {
		return 0
	}
	t.ensurePrefix()
	return t.lineStartRow[len(t.lineRowCount)]
}

// lineOfVisualRow maps an absolute visual row to its logical line and sub-row
// (offset within that line's wrapped rows), using the current prefix sum.
func (t *RichText) lineOfVisualRow(row int) (line, sub int) {
	t.ensurePrefix()
	L := len(t.lineRowCount)
	if L == 0 {
		return 0, 0
	}
	if row < 0 {
		row = 0
	}
	if total := t.lineStartRow[L]; row >= total && total > 0 {
		row = total - 1
	}
	line = sort.Search(L, func(i int) bool { return t.lineStartRow[i+1] > row }) //nolint:gosec
	if line >= L {
		line = L - 1
	}
	return line, row - t.lineStartRow[line]
}

// lazyFirstRowOfLine returns the first visual row of logical line (the lazy,
// analytic equivalent of firstRowOfLine — prefix lookup instead of a rowToLine
// binary search). Clamps line to [0, lines].
func (t *RichText) lazyFirstRowOfLine(line int) int {
	t.ensurePrefix()
	L := len(t.lineRowCount)
	if line < 0 {
		line = 0
	}
	if line > L {
		line = L
	}
	return t.lineStartRow[line]
}

// ensureLineMeasured shapes logical line's wrapped sub-rows (exact begin/end and
// count) if not already measured, replacing its estimate. Idempotent. Shaping one
// line in isolation reuses lineBoundsForRange (#157 B0).
func (t *RichText) ensureLineMeasured(line int) {
	if line < 0 || line >= len(t.lineRowCount) || t.lineMeasured[line] {
		return
	}
	seg, _ := t.singleSegment()
	low := t.buf.LineStart(line)
	high := t.lineEnd(line)

	// lineWrap sub-rows are stored LINE-RELATIVE (offsets from the line start), so
	// retained measurements survive edits earlier in the document without any
	// offset shifting; lazyRowBoundary / lazyRowColFromTextPos convert back to
	// absolute at the read points (#400). Materializing the line once into a
	// reusable scratch makes the produced bounds line-relative by construction.
	var subs []rowBoundary
	if t.Wrapping == bellui.TextWrapOff {
		subs = []rowBoundary{{segments: []RichTextSegment{seg}, begin: 0, end: high - low}}
	} else {
		style := seg.Style.TextStyle
		size := seg.size()
		source := seg.Style.FontSource
		measurer := func(text []rune) geo.Size {
			m, _ := bellui.MeasureTextFull(string(text), size, style, source)
			return m
		}
		t.lineScratch = t.buf.AppendRunes(t.lineScratch[:0], low, high)
		lineRunes := t.lineScratch
		// firstWidth == max width for a single segment (no inline predecessor), so
		// the width check uses the full wrap width like the dense lineBounds does.
		widthChecker := func(lo, hi int) bool {
			return measurer(lineRunes[lo:hi]).Width <= t.lazyMaxWidth
		}
		st := lineWrapState{measureWidth: t.lazyMaxWidth}
		// Truncation is off in the lazy path, so the height limit never fires; pass a
		// large height so the height-guarded early return is inert.
		max := geo.NewSize(t.lazyMaxWidth, float32(math.MaxInt32))
		lineBoundsForRange(seg, lineRunes, 0, high-low, t.Wrapping, bellui.TextTruncateOff, max, measurer, widthChecker, &st)
		subs = st.bounds
	}
	if len(subs) == 0 { // empty line safety (lineBoundsForRange appends one row when low==high)
		subs = []rowBoundary{{segments: []RichTextSegment{seg}, begin: 0, end: high - low}}
	}

	t.lineWrap[line] = subs
	t.lineMeasured[line] = true
	if len(subs) != t.lineRowCount[line] {
		t.lineRowCount[line] = len(subs)
		t.prefixDirty = true
	}
}

// lazyRowBoundary returns the boundary of an absolute visual row, measuring its
// logical line on demand. firstSegmentReuse is set to the absolute visual row so
// the window render keys cachedSegmentVisual by a stable per-row offset (the dense
// document-wide counter would collide across independently measured lines). Returns
// a fresh value each call (callers read it read-only).
func (t *RichText) lazyRowBoundary(row int) *rowBoundary {
	if row < 0 || row >= t.lazyRowCount() {
		return nil
	}
	line, sub := t.lineOfVisualRow(row)
	t.ensureLineMeasured(line)
	subs := t.lineWrap[line]
	if len(subs) == 0 {
		return nil
	}
	if sub < 0 {
		sub = 0
	}
	if sub >= len(subs) {
		sub = len(subs) - 1
	}
	b := subs[sub]
	ls := t.buf.LineStart(line) // lineWrap offsets are line-relative; return absolute
	return &rowBoundary{
		segments:          []RichTextSegment{t.Segments[0]},
		firstSegmentReuse: row,
		begin:             ls + b.begin,
		end:               ls + b.end,
		ellipsis:          b.ellipsis,
	}
}

// computeWindowRowsLazy returns the contiguous visible-row band for the viewport,
// measuring exactly the logical lines that span it (and refining their counts) so
// the returned rows are consistent with the post-measurement prefix. Used for the
// no-fold lazy path; folds fall back to the shared computeWindowRows (folded
// documents are measured eagerly, so every line's count is already exact).
func (t *RichText) computeWindowRowsLazy() []int {
	if len(t.lineRowCount) == 0 {
		return nil
	}
	s := t.stride()
	if s <= 0 {
		return nil
	}
	h := t.viewHeight
	if h <= 0 {
		h = 40 * s // bootstrap before the real viewport is known
	}
	top0 := t.contentTopOffset()
	total := t.lazyRowCount()

	kStart := int((t.viewTop-top0)/s) - virtualRowBuffer
	kEnd := int((t.viewTop+h-top0)/s) + 1 + virtualRowBuffer
	if kStart < 0 {
		kStart = 0
	}
	if kStart >= total {
		return nil
	}
	if kEnd > total {
		kEnd = total
	}

	startLine, _ := t.lineOfVisualRow(kStart)
	r := t.lineStartRow[startLine] // first row of startLine (preceding estimates are stable)
	L := len(t.lineRowCount)
	rows := make([]int, 0, kEnd-kStart+1)
	for line := startLine; line < L && r < kEnd; line++ {
		t.ensureLineMeasured(line) // exact count for every line that spans the band
		cnt := t.lineRowCount[line]
		for sub := 0; sub < cnt && r < kEnd; sub++ {
			if r >= kStart {
				rows = append(rows, r)
			}
			r++
		}
	}
	return rows
}

// lazyRowColFromTextPos maps a rune offset to (visual row, column) without the
// dense O(rows) scan: binary-search the logical line, measure only that line, then
// locate the sub-row. The rule "largest sub-row whose begin <= pos" reproduces the
// dense wrap affinity (a position at a soft-wrap boundary lands downstream) for both
// break and word wrap, including the word-wrap case where a space is consumed at the
// break. Returned row is absolute (in the current estimated prefix), consistent with
// the layout/caret which use the same prefix.
func (t *RichText) lazyRowColFromTextPos(pos int) (row, col int) {
	L := len(t.lineRowCount)
	if L == 0 {
		return 0, 0
	}
	if pos < 0 {
		pos = 0
	}
	if n := t.buf.Len(); pos > n {
		pos = n
	}
	line := t.buf.LineOfPos(pos)
	t.ensureLineMeasured(line)
	t.ensurePrefix()
	subs := t.lineWrap[line]
	rel := pos - t.buf.LineStart(line) // lineWrap offsets are line-relative
	k := 0
	for j := range subs {
		if subs[j].begin <= rel {
			k = j
		} else {
			break
		}
	}
	if len(subs) == 0 {
		return t.lineStartRow[line], rel
	}
	return t.lineStartRow[line] + k, rel - subs[k].begin
}

// lazyVirtualWidth returns the widest-row width for the lazy MinSize. Wrapped rows
// are bounded by the wrap width, so it returns that directly; wrap-off rows can be
// arbitrarily wide, so it scans logical lines (cheap integer compare) for the
// longest and measures only that one.
func (t *RichText) lazyVirtualWidth() float32 {
	if t.Wrapping != bellui.TextWrapOff {
		return t.lazyMaxWidth
	}
	seg, ok := t.singleSegment()
	if !ok {
		return 0
	}
	// The widest line is cached (vwLine/vwLen) and maintained incrementally by
	// repairLazyLineCaches; a full rescan — one vectorized buffer walk — runs
	// only after a rebuild or when the cached longest line may have shrunk.
	// Per-line queries here would cost a chunk scan each on the rope (#403).
	if t.vwDirty {
		t.vwLen, t.vwLine = -1, 0
		t.buf.ForEachLine(func(line, start, end int) bool {
			if l := end - start; l > t.vwLen {
				t.vwLen, t.vwLine = l, line
			}
			return true
		})
		t.vwDirty = false
	}
	if t.vwLen <= 0 {
		return 0
	}
	begin := t.buf.LineStart(t.vwLine)
	end := t.lineEnd(t.vwLine)
	txt := t.buf.Substring(begin, end)
	if seg.Style.concealed {
		txt = strings.Repeat(passwordChar, end-begin)
	}
	m, _ := bellui.MeasureTextFull(txt, seg.size(), seg.Style.TextStyle, seg.Style.FontSource)
	return m.Width
}

// --- Incremental single-segment editing (#380, storage behind textbuf #400) ---
//
// In the lazy single-segment path t.buf (internal/textbuf.Buffer) is the
// authoritative content buffer and logical-line index. An edit splices it in place
// and re-materializes seg.Text from the buffer's memoized String (one byte-splice
// inside the buffer — only the inserted runes are UTF-8 encoded, never the whole
// document), then repairs the per-line measurement caches — replacing the
// per-keystroke O(document) decode, re-encode and full slice reallocation with
// work proportional to the edit.

// lazyCanEdit reports whether the incremental single-segment fast edit path applies:
// the lazy uniform-stride model is live, the content is exactly one text segment,
// no folds are active (folds need eager per-line measurement), and the buffer still
// reflects the segment's current Text (else an external write left it stale and we
// fall back to the rebuild path). When true, t.buf is the content buffer.
func (t *RichText) lazyCanEdit() bool {
	if !t.lazy || len(t.hiddenLogical) > 0 || t.buf == nil {
		return false
	}
	seg, ok := t.singleSegment()
	if !ok {
		return false
	}
	// O(1) in the steady state: after an edit seg.Text is the same string object as
	// lazyStr, so == short-circuits on the data pointer. A genuine external change
	// yields a different object (usually a different length ⇒ still O(1) false).
	return seg.Text == t.lazyStr
}

// lazySpliceText replaces the rune range [low:high] of the buffer with ins,
// re-materializes the segment Text from the buffer's memo, repairs the per-line
// measurement caches and the row-bounds cache keys so updateRowBounds reuses the
// incrementally maintained model. Returns a copy of the removed runes.
// Preconditions per lazyCanEdit.
func (t *RichText) lazySpliceText(low, high int, ins []rune) []rune {
	seg := t.Segments[0].(*TextSegment)
	n := t.buf.Len()
	if low < 0 {
		low = 0
	}
	if high > n {
		high = n
	}
	if low > high {
		low = high
	}

	removed := t.buf.Slice(low, high)

	// Pre-edit line coordinates for the cache repair below; the buffer repairs its
	// own line index inside Splice.
	a := t.buf.LineOfPos(low)
	z := t.buf.LineOfPos(high)

	t.buf.Splice(low, high, ins)

	if t.deferText {
		// Deferred text sync (#410): skip the document materialization — the
		// one remaining per-keystroke O(document) cost. seg.Text and lazyStr
		// keep the last-synced string object, so lazyCanEdit's identity gate
		// still holds (both stale, still equal) and a later syncSegments store
		// of the same object stays a no-op; buf alone carries the new content
		// until lazyMaterialize re-anchors at a sync point.
		t.deferredDirty = true
	} else {
		// The memoized materialization becomes the new seg.Text / identity anchor: one
		// byte-splice inside the buffer, shared by every full-string consumer.
		newText := t.buf.String()
		seg.Text = newText
		t.lazyStr = newText
		t.deferredDirty = false
	}

	t.repairLazyLineCaches(a, z, ins)

	// Keep the cache valid so updateRowBounds reuses this model, and align the
	// content fingerprint so a following SyncState does not force a rebuild.
	t.rowBoundsVersion++
	t.rowBoundsComputed = t.rowBoundsVersion
	t.lastContentFP = t.contentFingerprint()

	return removed
}

// lazyMaterialize re-anchors seg.Text and lazyStr to the buffer's current
// materialization after deferred edits (#410) — the one document-sized
// alloc+memcpy the deferred mode postpones to a sync point. A no-op when
// nothing is deferred; when the identity chain was broken by an external
// write the pending edits are already forfeit (the next rebuild resets buf
// from seg.Text), so only the dirty flag is cleared.
func (t *RichText) lazyMaterialize() {
	if !t.deferredDirty {
		return
	}
	t.deferredDirty = false
	if !t.lazyCanEdit() {
		return
	}
	newText := t.buf.String()
	t.Segments[0].(*TextSegment).Text = newText
	t.lazyStr = newText
	t.lastContentFP = t.contentFingerprint()
}

// repairLazyLineCaches splices/invalidates the per-line measurement slices after
// the buffer had logical lines [a..z] replaced by (newlines in ins)+1 lines.
// lineWrap offsets are line-relative, so lines outside the edited span keep their
// measurement untouched — no offset shifting (#400).
func (t *RichText) repairLazyLineCaches(a, z int, ins []rune) {
	insLines := 0
	for _, r := range ins {
		if r == '\n' {
			insLines++
		}
	}
	oldCount := z - a + 1
	newCount := insLines + 1
	wrapOff := t.Wrapping == bellui.TextWrapOff

	if newCount == oldCount {
		// No change in logical-line count (the common no-newline edit): only the
		// edited lines' content changed. Invalidate their measurement; downstream
		// ensureLineMeasured reshapes them.
		for k := a; k <= z; k++ {
			t.invalidateLazyLine(k)
		}
	} else {
		// Structural change: splice the parallel per-line slices so their length
		// tracks the new logical-line count. The replaced lines start unmeasured
		// (estimated); everything outside [a..z] is preserved.
		t.lineMeasured = spliceSlice(t.lineMeasured, a, z+1, make([]bool, newCount))
		t.lineWrap = spliceSlice(t.lineWrap, a, z+1, make([][]rowBoundary, newCount))
		t.lineRowCount = spliceSlice(t.lineRowCount, a, z+1, make([]int, newCount))
		for k := a; k < a+newCount; k++ {
			t.lineRowCount[k] = t.estimateRowCount(k, wrapOff)
		}
		t.prefixDirty = true
	}

	// Maintain the widest-line cache (wrap-off virtual width): shift the cached
	// index past the edit, grow it from the edited lines, and fall back to a
	// lazy rescan only when the cached longest line itself may have shrunk.
	if !t.vwDirty {
		delta := newCount - oldCount
		switch {
		case t.vwLine > z:
			t.vwLine += delta
		case t.vwLine >= a && delta == 0:
			if l := t.lineRuneLen(t.vwLine); l >= t.vwLen {
				t.vwLen = l // grew (or held): still the longest
			} else {
				t.vwDirty = true // may no longer be the longest — rescan lazily
			}
		case t.vwLine >= a:
			t.vwDirty = true
		}
		if !t.vwDirty {
			for k := a; k < a+newCount && k < len(t.lineRowCount); k++ {
				if l := t.lineRuneLen(k); l > t.vwLen {
					t.vwLen, t.vwLine = l, k
				}
			}
		}
	}

	// Small wrapped documents are measured eagerly (see updateRowBoundsLazy), so keep
	// the edited lines exact to preserve the exact row count / caret geometry. Large
	// documents refine on demand as the viewport reaches them.
	if !wrapOff && len(t.lineRowCount) <= lazyMeasureAllMaxLines {
		for k := a; k < a+newCount && k < len(t.lineRowCount); k++ {
			t.ensureLineMeasured(k)
		}
	}
	t.ensurePrefix()

	// Let per-line caches layered above the provider (Entry's highlight cache)
	// splice themselves in step with the line index (#401).
	if t.onLazyLinesEdited != nil {
		t.onLazyLinesEdited(a, z, newCount)
	}
}

// invalidateLazyLine marks a logical line unmeasured so ensureLineMeasured reshapes
// it. The previous row count is kept as the line's estimate: for the common in-line
// edit it is usually still exact, and leaving it untouched keeps the prefix sums
// valid so a keystroke pays no O(lines) rebuild; ensureLineMeasured corrects the
// count (and marks the prefix dirty) if the reshape disagrees (#400).
func (t *RichText) invalidateLazyLine(line int) {
	t.lineMeasured[line] = false
	t.lineWrap[line] = nil
}

// spliceSlice replaces s[low:high] with ins, reusing s's backing array when it has
// the capacity. Go's copy is memmove-safe, so the overlapping tail shift is correct.
func spliceSlice[T any](s []T, low, high int, ins []T) []T {
	newLen := low + len(ins) + (len(s) - high)
	if newLen <= cap(s) {
		tail := s[high:]
		s = s[:newLen]
		copy(s[low+len(ins):], tail) // shift tail (overlap-safe)
		copy(s[low:], ins)
		return s
	}
	ns := make([]T, newLen)
	copy(ns, s[:low])
	copy(ns[low:], ins)
	copy(ns[low+len(ins):], s[high:])
	return ns
}
