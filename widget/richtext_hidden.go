package widget

import "sort"

// LineRange is a 0-based range with inclusive Start and exclusive End. It
// identifies a contiguous block of lines, for example a folded region fed to
// [Entry.HiddenRanges] (where the unit is logical, newline-delimited lines).
//
// Since: 1.0
type LineRange struct {
	Start, End int
}

// normalizeLineRanges returns rs sorted by Start, with Start clamped to >= 0,
// empty ranges dropped, and overlapping or adjacent ranges merged. The result
// is a minimal, ordered, disjoint set (nil when nothing survives).
func normalizeLineRanges(rs []LineRange) []LineRange {
	cleaned := make([]LineRange, 0, len(rs))
	for _, r := range rs {
		s, e := r.Start, r.End
		if s < 0 {
			s = 0
		}
		if e > s {
			cleaned = append(cleaned, LineRange{s, e})
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	sort.Slice(cleaned, func(i, j int) bool { return cleaned[i].Start < cleaned[j].Start })

	merged := []LineRange{cleaned[0]}
	for _, r := range cleaned[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End { // overlapping or adjacent
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func lineRangesEqual(a, b []LineRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setHiddenRanges stores rs as the requested folds in logical-line coordinates
// and resolves them to visual rows. It reports whether the resolved (visual)
// folded set changed (so the caller can decide to relayout).
func (t *RichText) setHiddenRanges(rs []LineRange) bool {
	t.hiddenLogical = rs
	// Folds keep the dense varHeight path: the block-lazy build (pre-rebrand tracker #317) declines them
	// (a fold's collapsed extent needs exact per-row geometry), so a fold appearing
	// on a block-lazy document forces a rebuild onto the dense model.
	rebuild := t.blockLazy && len(rs) > 0
	changed := t.applyHiddenRanges()
	if rebuild {
		t.rowBoundsComputed = -1
		t.updateRowBounds()
	}
	return changed
}

// applyHiddenRanges recomputes the visual-row folded set (hiddenRanges) from the
// requested logical-line folds (hiddenLogical) and the current rowToLine map,
// then normalises. It reports whether the stored visual set changed. Both
// setHiddenRanges (request changed) and updateRowBounds (rowToLine rebuilt) call
// it, so the two inputs are reconciled from whichever side moves.
func (t *RichText) applyHiddenRanges() bool {
	norm := normalizeLineRanges(t.translateHiddenToRows(t.hiddenLogical))
	if lineRangesEqual(norm, t.hiddenRanges) {
		return false
	}
	t.hiddenRanges = norm
	// The variable-height prefix bakes folded rows in at zero height, so a fold
	// change must rebuild it (pre-rebrand tracker #158).
	t.prefixHeightDirty = true
	return true
}

// translateHiddenToRows maps logical-line ranges to the visual rows they occupy.
// A wrapped logical line covers several visual rows, so a fold over logical lines
// [Start,End) expands to every visual row whose logical line falls in that span.
// With no rowToLine map (multi-segment, or bounds not yet computed) the ranges
// are treated as already row-based — backward compatible, since wrap-off content
// has exactly one visual row per logical line.
func (t *RichText) translateHiddenToRows(logical []LineRange) []LineRange {
	// Lazy path serves firstRowOfLine analytically (no rowToLine slice); folded
	// documents are measured eagerly so the prefix is exact.
	if !t.lazy && len(t.rowToLine) == 0 {
		return logical
	}
	out := make([]LineRange, 0, len(logical))
	for _, lr := range logical {
		s := t.firstRowOfLine(lr.Start)
		e := t.firstRowOfLine(lr.End)
		if e > s {
			out = append(out, LineRange{Start: s, End: e})
		}
	}
	return out
}

// firstRowOfLine returns the smallest visual row whose logical line is >= line,
// or len(rowToLine) when every row precedes it. rowToLine is non-decreasing.
func (t *RichText) firstRowOfLine(line int) int {
	if t.lazy {
		return t.lazyFirstRowOfLine(line)
	}
	return sort.Search(len(t.rowToLine), func(i int) bool { return t.rowToLine[i] >= line })
}

// rowIsLogicalLineEnd reports whether visual row r is the last visual row of its
// logical line — i.e. a hard newline follows it, rather than a soft wrap continuing
// the same logical line onto row r+1. Selection text uses this so a copied wrapped
// line is not split by spurious newlines at its wrap points.
func (t *RichText) rowIsLogicalLineEnd(r int) bool {
	if r+1 >= t.rows() {
		return true
	}
	if t.lazy {
		l1, _ := t.lineOfVisualRow(r)
		l2, _ := t.lineOfVisualRow(r + 1)
		return l1 != l2
	}
	if len(t.rowToLine) > r+1 {
		return t.rowToLine[r] != t.rowToLine[r+1]
	}
	// Multi-segment content has no rowToLine map; derive the answer from the row
	// boundaries — a soft wrap continues the same TextSegment with no newline (pre-rebrand tracker #161).
	_, soft := t.softWrapGap(r)
	return !soft
}

// softWrapGap reports whether visual rows r and r+1 are a soft wrap of the same
// logical line — they continue the same *TextSegment with no '\n' between them — and
// returns the run word-wrap consumed at that break (a space, or "" for break-wrap,
// which drops nothing). It returns ("", false) at a hard newline or a segment
// boundary. Selection copy uses it to rejoin wrapped rows with the original
// separator instead of dropping it or inserting a spurious newline (pre-rebrand tracker #161). The
// consumed run is exactly the segment runes in [row r's end, row r+1's begin), which
// belong to neither row's rendered text.
func (t *RichText) softWrapGap(r int) (string, bool) {
	if r < 0 || r+1 >= t.rows() {
		return "", false
	}
	b1 := t.rowBoundary(r)
	b2 := t.rowBoundary(r + 1)
	if b1 == nil || b2 == nil || len(b1.segments) == 0 || len(b2.segments) == 0 {
		return "", false
	}
	seg, ok := b1.segments[len(b1.segments)-1].(*TextSegment)
	if !ok || b1.segments[len(b1.segments)-1] != b2.segments[0] {
		return "", false // wrap falls at a segment boundary, not inside one text run
	}
	runes := []rune(seg.Text)
	lo, hi := b1.end, b2.begin
	if lo < 0 {
		lo = 0
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	if lo >= hi {
		return "", true // contiguous (break-wrap): a soft wrap that consumed nothing
	}
	gap := runes[lo:hi]
	for _, c := range gap {
		if c == '\n' {
			return "", false // a hard line break, not a soft wrap
		}
	}
	return string(gap), true
}

// buildRowToLine recomputes the visual-row -> logical-line map for single-segment
// content (the Entry case). Each row's logical line is the number of newlines in
// the segment before the row's begin offset; rows are scanned in order and begin
// offsets are non-decreasing, so one forward pass over the runes suffices. For
// multi-segment content the map is cleared and folds keep their row-based meaning.
func (t *RichText) buildRowToLine() {
	seg, ok := t.singleSegment()
	if !ok {
		t.rowToLine = nil
		return
	}
	runes := []rune(seg.Text)
	m := make([]int, len(t.rowBounds))
	line, pos := 0, 0
	for i := range t.rowBounds {
		b := t.rowBounds[i].begin
		if b > len(runes) {
			b = len(runes)
		}
		for pos < b {
			if runes[pos] == '\n' {
				line++
			}
			pos++
		}
		m[i] = line
	}
	t.rowToLine = m
}

// anyHidden reports whether any folded range is set.
func (t *RichText) anyHidden() bool { return len(t.hiddenRanges) > 0 }

// rowHidden reports whether the logical row lies inside a folded range.
func (t *RichText) rowHidden(row int) bool {
	for _, r := range t.hiddenRanges {
		if row < r.Start {
			return false // ranges are sorted
		}
		if row < r.End {
			return true
		}
	}
	return false
}

// hiddenBefore returns the number of hidden rows in [0, row), clamping range
// ends to the current row count so stale ranges past a shrunken document do not
// over-count.
func (t *RichText) hiddenBefore(row int) int {
	n := t.rows()
	if row > n {
		row = n
	}
	count := 0
	for _, r := range t.hiddenRanges {
		if r.Start >= row {
			break
		}
		e := r.End
		if e > n {
			e = n
		}
		if e > row {
			e = row
		}
		if e > r.Start {
			count += e - r.Start
		}
	}
	return count
}

// visibleRowCount returns the number of rows that are not folded away.
func (t *RichText) visibleRowCount() int {
	return t.rows() - t.hiddenBefore(t.rows())
}

// firstVisibleAtOrAfter returns the smallest visible row >= row, or rows() when
// every row from row onward is folded.
func (t *RichText) firstVisibleAtOrAfter(row int) int {
	n := t.rows()
	for _, r := range t.hiddenRanges {
		if row < r.Start {
			break
		}
		if row < r.End {
			row = r.End
		}
	}
	if row > n {
		row = n
	}
	return row
}

// lastVisibleAtOrBefore returns the largest visible row <= row, or -1 when every
// row up to row is folded.
func (t *RichText) lastVisibleAtOrBefore(row int) int {
	for i := len(t.hiddenRanges) - 1; i >= 0; i-- {
		r := t.hiddenRanges[i]
		if row >= r.End {
			break
		}
		if row >= r.Start {
			row = r.Start - 1
		}
	}
	if row < -1 {
		row = -1
	}
	return row
}

// visibleIndexToRow maps a 0-based visible-row index (position among non-folded
// rows) to its logical row index.
func (t *RichText) visibleIndexToRow(k int) int {
	n := t.rows()
	row := 0
	for _, r := range t.hiddenRanges {
		s, e := r.Start, r.End
		if s > n {
			s = n
		}
		if e > n {
			e = n
		}
		if s < row {
			s = row
		}
		avail := s - row
		if k < avail {
			return row + k
		}
		k -= avail
		row = e
	}
	res := row + k
	if res > n {
		res = n
	}
	return res
}
