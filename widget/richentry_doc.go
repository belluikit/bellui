package widget

import (
	"sort"
	"strings"

	"github.com/belluikit/bellui"
)

// richStyleEqual compares two RichTextStyles by value. It exists because
// bellui.TextStyle carries pointer fields (Weight, LineHeightScale, LetterSpacing,
// each a nil-sentinel per the fork's advanced-text-layout API), so a plain ==
// would compare those by identity and never merge visually-identical runs built
// from separate bellui.Ptr(...) calls.
func richStyleEqual(a, b RichTextStyle) bool {
	return a.Alignment == b.Alignment && a.ColorName == b.ColorName && a.Inline == b.Inline &&
		a.SizeName == b.SizeName && a.HeadingLevel == b.HeadingLevel && a.concealed == b.concealed &&
		textStyleEqual(a.TextStyle, b.TextStyle)
}

func textStyleEqual(a, b bellui.TextStyle) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Monospace == b.Monospace &&
		a.Symbol == b.Symbol && a.TabWidth == b.TabWidth && a.Underline == b.Underline &&
		a.Strikethrough == b.Strikethrough && a.Direction == b.Direction && a.Language == b.Language &&
		ptrEqual(a.Weight, b.Weight) && ptrEqual(a.LineHeightScale, b.LineHeightScale) &&
		ptrEqual(a.LetterSpacing, b.LetterSpacing)
}

func ptrEqual[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// richEntryDoc is the editable document model behind [RichEntry] (#648, Phase 1).
//
// The document is an ordered list of *inline* [TextSegment]s. A position is a
// global rune offset over their concatenated text; the helpers segAt/styleAt
// translate an offset into a (segment index, in-segment rune offset) pair — the
// (segmentIndex, runeOffset) position type the epic calls for. Because every
// segment is inline, line breaks are '\n' runes carried inside the text, so with
// wrapping off each '\n'-separated line is exactly one rendered row and the
// offset<->(row,col) mapping is a pure newline count (see offsetToRowCol).
//
// A non-text atom (an inline or block image) contributes exactly one position,
// so the caret can step over and select it; the arithmetic is centralised here
// (segPositions) rather than being corrupted across segment boundaries.
type richEntryDoc struct {
	segs []RichTextSegment

	// wrapRows, when non-nil, is a wrapping-aware row model derived from the
	// provider's real rowBounds (soft-wrap splits over multi-segment rows), used
	// instead of the pure newline-count rowModel when RichEntry.Wrapping is on
	// (#653). It is rebuilt from the provider after each layout; nil means the
	// fast wrap-off path.
	wrapRows []richRow

	// rowModelMemo caches the wrap-off rowModel between edits; the caret/selection
	// mapping calls activeRowModel several times per keystroke, and rebuilding the
	// O(segments) model each call was a needless per-keystroke cost on large
	// documents (#671). Invalidated by normalize on any content change; the
	// in-place typing paths patch it via rowsShiftText instead (#975).
	rowModelMemo  []richRow
	rowModelValid bool

	// posIndex memoizes each segment's cumulative start position (#975):
	// posIndex[i] is the position of segs[i]'s first caret slot and
	// posIndex[len(segs)] the document length, so segAt is a binary search and
	// len is O(1) instead of every position walk re-counting each segment's
	// runes. Rebuilt lazily after a structural edit (normalize invalidates);
	// the in-place typing paths patch it via posShift.
	posIndex      []int
	posIndexValid bool
}

// positions returns the memoized cumulative position index, rebuilding it once
// after a structural edit.
func (d *richEntryDoc) positions() []int {
	if d.posIndexValid {
		return d.posIndex
	}
	p := d.posIndex[:0]
	if cap(p) < len(d.segs)+1 {
		p = make([]int, 0, len(d.segs)+1)
	}
	pos := 0
	p = append(p, 0)
	for _, seg := range d.segs {
		pos += segPositions(seg)
		p = append(p, pos)
	}
	d.posIndex, d.posIndexValid = p, true
	return p
}

// posShift patches the position index after an in-place text length change of
// delta in segment segIdx; a no-op when the index is stale anyway.
func (d *richEntryDoc) posShift(segIdx, delta int) {
	if !d.posIndexValid {
		return
	}
	for i := segIdx + 1; i < len(d.posIndex); i++ {
		d.posIndex[i] += delta
	}
}

// rowsShiftText patches the memoized wrap-off row model for a same-line text
// splice at offset with position delta (the spliced text must contain no
// newline and no atoms): the row containing the offset grows/shrinks and every
// following row shifts. Reports false when the memo is stale or the offset
// resolves to an image row — the caller must invalidate instead.
func (d *richEntryDoc) rowsShiftText(offset, delta int) bool {
	if !d.rowModelValid {
		return false
	}
	rows := d.rowModelMemo
	idx := sort.Search(len(rows), func(i int) bool { return rows[i].end >= offset })
	if idx >= len(rows) || rows[idx].image || offset < rows[idx].start {
		return false
	}
	rows[idx].end += delta
	for j := idx + 1; j < len(rows); j++ {
		rows[j].start += delta
		rows[j].end += delta
	}
	return true
}

// spliceInLine records an in-place text splice (segment segIdx, document
// offset, inserted/removed runes) against the derived memos: the position
// index is patched; the row model is patched for a same-line splice and
// invalidated when the spliced text carries a newline.
func (d *richEntryDoc) spliceInLine(segIdx, offset, delta int, spliced []rune) {
	d.posShift(segIdx, delta)
	newline := false
	for _, r := range spliced {
		if r == '\n' {
			newline = true
			break
		}
	}
	if newline || !d.rowsShiftText(offset, delta) {
		d.rowModelValid = false
	}
}

// newRichEntryDoc returns an empty document holding a single empty inline
// segment, so there is always a row 0 and a place to insert.
func newRichEntryDoc() *richEntryDoc {
	return &richEntryDoc{segs: []RichTextSegment{&TextSegment{Style: RichTextStyle{Inline: true}}}}
}

// setSegments replaces the document content. TextSegments are forced inline (the
// document is a single flowing rich paragraph broken only by '\n'); a nil or
// empty list yields one empty inline segment.
func (d *richEntryDoc) setSegments(segs []RichTextSegment) {
	d.segs = make([]RichTextSegment, 0, len(segs))
	for _, seg := range segs {
		if ts, ok := seg.(*TextSegment); ok {
			ts.Style.Inline = true
		}
		d.segs = append(d.segs, seg)
	}
	d.normalize()
}

// segments returns the live backing slice (the RichText provider renders it
// directly). Callers must not mutate it outside the edit primitives.
func (d *richEntryDoc) segments() []RichTextSegment {
	return d.segs
}

// segPositions is the number of caret positions a segment occupies: one per rune
// for a text segment, and exactly one for any non-text atom (e.g. an image). It
// is the unit of the document's position space (#655).
func segPositions(seg RichTextSegment) int {
	if ts, ok := seg.(*TextSegment); ok {
		return len([]rune(ts.Text))
	}
	return 1
}

// len returns the total number of caret positions (text runes plus one per
// atom) — O(1) off the memoized position index.
func (d *richEntryDoc) len() int {
	p := d.positions()
	return p[len(p)-1]
}

// String returns the document as plain text.
func (d *richEntryDoc) String() string {
	var b strings.Builder
	for _, seg := range d.segs {
		if ts, ok := seg.(*TextSegment); ok {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

// segAt maps a global rune offset to (segment index, in-segment rune offset).
// Offsets are clamped to [0, len]. At a boundary between two segments the offset
// resolves to the start of the following segment (right-biased); the final
// offset resolves to the end of the last segment.
func (d *richEntryDoc) segAt(offset int) (idx, local int) {
	if offset < 0 {
		offset = 0
	}
	n := len(d.segs)
	if n == 0 {
		return 0, 0
	}
	p := d.positions()
	// First segment whose end exceeds the offset — right-biased at segment
	// boundaries, exactly like the linear walk this replaces (#975).
	i := sort.Search(n, func(i int) bool { return offset < p[i+1] })
	if i == n {
		// offset at or past the end: end of the last segment.
		return n - 1, p[n] - p[n-1]
	}
	return i, offset - p[i]
}

// styleAt returns the style newly typed text should inherit at offset: the style
// of the character immediately before it (left-biased), or the first segment's
// style at the document start.
func (d *richEntryDoc) styleAt(offset int) RichTextStyle {
	idx, local := d.segAt(offset)
	if local == 0 && idx > 0 {
		idx--
	}
	if ts, ok := d.segs[idx].(*TextSegment); ok {
		return ts.Style
	}
	return RichTextStyle{Inline: true}
}

// insertText splices runes into the document at the given offset with the given
// style, splitting the target segment when the style differs and letting
// normalize merge runs that end up adjacent with an equal style. It returns the
// caret offset after the inserted text.
func (d *richEntryDoc) insertText(offset int, runes []rune, style RichTextStyle) int {
	if len(runes) == 0 {
		return offset
	}
	style.Inline = true
	idx, local := d.segAt(offset)
	ts, ok := d.segs[idx].(*TextSegment)
	if ok && richStyleEqual(ts.Style, style) {
		r := []rune(ts.Text)
		ts.Text = string(concatRunes(r[:local], runes, r[local:]))
		// An in-place splice cannot change the merge structure (no segment is
		// emptied, no styles change), so the full normalize walk is unnecessary
		// — patch the derived memos instead (#975).
		d.spliceInLine(idx, offset, len(runes), runes)
		return offset + len(runes)
	}
	if ok && local == 0 && idx > 0 {
		// Typing at a boundary where the style matches the *previous* run (the
		// left-biased style styleAt hands out): append to that run instead of
		// splitting the following one — splitting would create a new segment
		// every keystroke that normalize immediately merges away, churning
		// segment identity (and the render caches keyed on it) for nothing.
		if prev, okPrev := d.segs[idx-1].(*TextSegment); okPrev && richStyleEqual(prev.Style, style) {
			prev.Text += string(runes)
			d.spliceInLine(idx-1, offset, len(runes), runes)
			return offset + len(runes)
		}
	}

	newSeg := &TextSegment{Style: style, Text: string(runes)}
	var repl []RichTextSegment
	if ok {
		r := []rune(ts.Text)
		if left := string(r[:local]); left != "" {
			repl = append(repl, &TextSegment{Style: ts.Style, Text: left})
		}
		repl = append(repl, newSeg)
		if right := string(r[local:]); right != "" {
			repl = append(repl, &TextSegment{Style: ts.Style, Text: right})
		}
		d.segs = replaceSegment(d.segs, idx, repl)
	} else {
		// Non-text atom (Phase 2): insert the run beside it, never into it.
		at := idx
		if local > 0 {
			at = idx + 1
		}
		d.segs = insertSegment(d.segs, at, newSeg)
	}
	d.normalize()
	return offset + len(runes)
}

// deleteRange removes the content in the half-open position range [from, to) and
// returns the removed text. A text segment is trimmed (dropped when emptied); an
// atom (image) whose single position falls in the range is dropped whole.
func (d *richEntryDoc) deleteRange(from, to int) []rune {
	from, to = clampRange(from, to, d.len())
	if from >= to {
		return nil
	}
	// In-place fast path (#975): the range lies inside one text segment and
	// does not empty it — trim without the full segment walk + normalize (a
	// mid-segment trim cannot change the merge structure).
	if idx, local := d.segAt(from); idx < len(d.segs) {
		if ts, ok := d.segs[idx].(*TextSegment); ok {
			if r := []rune(ts.Text); local+(to-from) <= len(r) && to-from < len(r) {
				removed := append([]rune(nil), r[local:local+(to-from)]...)
				ts.Text = string(concatRunes(r[:local], r[local+(to-from):]))
				d.spliceInLine(idx, from, from-to, removed)
				return removed
			}
		}
	}
	removed := []rune(d.textBetween(from, to))

	var out []RichTextSegment
	cum := 0
	for _, seg := range d.segs {
		start, end := cum, cum+segPositions(seg)
		cum = end
		lo, hi := max(from, start), min(to, end)
		if lo >= hi { // untouched
			out = append(out, seg)
			continue
		}
		ts, ok := seg.(*TextSegment)
		if !ok {
			continue // atom fully within the range: drop it
		}
		r := []rune(ts.Text)
		if kept := string(concatRunes(r[:lo-start], r[hi-start:])); kept != "" {
			ts.Text = kept
			out = append(out, ts)
		}
	}
	d.segs = out
	d.normalize()
	return removed
}

// textBetween returns the plain text in the half-open position range [from, to)
// (atoms contribute no text).
func (d *richEntryDoc) textBetween(from, to int) string {
	from, to = clampRange(from, to, d.len())
	if from >= to {
		return ""
	}
	p := d.positions()
	first, _ := d.segAt(from)
	var b strings.Builder
	for i := first; i < len(d.segs) && p[i] < to; i++ {
		ts, ok := d.segs[i].(*TextSegment)
		if !ok {
			continue
		}
		r := []rune(ts.Text)
		lo, hi := max(from, p[i]), min(to, p[i+1])
		if lo < hi {
			b.WriteString(string(r[lo-p[i] : hi-p[i]]))
		}
	}
	return b.String()
}

// segmentsBetween returns the styled content of the half-open position range
// [from, to) as a fresh, independent list of segments (deep-cloned so later
// document mutations never alias it) — text runs are sub-sliced and atoms are
// copied whole. It is the "before"/"after" capture used by undo/redo to preserve
// styling and embedded images.
func (d *richEntryDoc) segmentsBetween(from, to int) []RichTextSegment {
	from, to = clampRange(from, to, d.len())
	var out []RichTextSegment
	if from >= to {
		return out
	}
	p := d.positions()
	first, _ := d.segAt(from)
	for i := first; i < len(d.segs) && p[i] < to; i++ {
		seg := d.segs[i]
		lo, hi := max(from, p[i]), min(to, p[i+1])
		if lo >= hi {
			continue
		}
		if ts, ok := seg.(*TextSegment); ok {
			r := []rune(ts.Text)
			out = append(out, &TextSegment{Style: ts.Style, Text: string(r[lo-p[i] : hi-p[i]])})
			continue
		}
		out = append(out, cloneSegment(seg)) // atom copied whole
	}
	return out
}

// replaceRange deletes the runes in [from, to) and inserts the given styled
// segments at from. It backs undo/redo (a reversible range replace) and reuses
// insertText's split/merge so the result normalizes exactly like an interactive
// edit.
func (d *richEntryDoc) replaceRange(from, to int, segs []RichTextSegment) {
	from, to = clampRange(from, to, d.len())
	d.deleteRange(from, to)
	d.insertSegments(from, segs)
}

// insertSegments splices a list of segments (text runs and image atoms) at the
// offset and returns the position after the inserted content.
func (d *richEntryDoc) insertSegments(offset int, segs []RichTextSegment) int {
	pos := offset
	for _, seg := range segs {
		if ts, ok := seg.(*TextSegment); ok {
			pos = d.insertText(pos, []rune(ts.Text), ts.Style)
			continue
		}
		pos = d.insertAtom(pos, cloneSegment(seg)) // image atom
	}
	return pos
}

// insertAtom inserts a non-text atom (e.g. an image) at the position, splitting a
// text segment when the position falls inside one. It returns the position after
// the atom.
func (d *richEntryDoc) insertAtom(offset int, atom RichTextSegment) int {
	offset = clampInt(offset, 0, d.len())
	idx, local := d.segAt(offset)
	if ts, ok := d.segs[idx].(*TextSegment); ok && local > 0 && local < len([]rune(ts.Text)) {
		r := []rune(ts.Text)
		repl := []RichTextSegment{
			&TextSegment{Style: ts.Style, Text: string(r[:local])},
			atom,
			&TextSegment{Style: ts.Style, Text: string(r[local:])},
		}
		d.segs = replaceSegment(d.segs, idx, repl)
	} else {
		at := idx
		if local > 0 {
			at = idx + 1
		}
		d.segs = insertSegment(d.segs, at, atom)
	}
	d.normalize()
	return offset + 1
}

// applyStyle sets the style of every segment overlapping [from, to), splitting
// segments at the range boundaries so only the covered runes change. It is the
// machinery behind [RichEntry.ApplyStyle].
func (d *richEntryDoc) applyStyle(from, to int, style RichTextStyle) {
	from, to = clampRange(from, to, d.len())
	if from >= to {
		return
	}
	style.Inline = true

	var out []RichTextSegment
	cum := 0
	for _, seg := range d.segs {
		start, end := cum, cum+segPositions(seg)
		cum = end
		ts, ok := seg.(*TextSegment)
		if !ok {
			out = append(out, seg) // atoms carry no text style
			continue
		}
		r := []rune(ts.Text)
		lo, hi := max(from, start), min(to, end)
		if lo >= hi { // untouched
			out = append(out, ts)
			continue
		}
		if lo > start {
			out = append(out, &TextSegment{Style: ts.Style, Text: string(r[:lo-start])})
		}
		out = append(out, &TextSegment{Style: style, Text: string(r[lo-start : hi-start])})
		if hi < end {
			out = append(out, &TextSegment{Style: ts.Style, Text: string(r[hi-start:])})
		}
	}
	d.segs = out
	d.normalize()
}

// richRow is one rendered row of the document in position space: a text line
// spanning positions [start, end] (caret columns 0..end-start), or a block image
// occupying the single position start.
type richRow struct {
	start, end int
	image      bool
}

// rowModel splits the document into rendered rows. With wrapping off a text line
// is one row (a trailing '\n' yields a final empty row) and each block image is
// its own row — matching RichText's layout (validated in tests against
// provider.rows()).
func (d *richEntryDoc) rowModel() []richRow {
	if d.rowModelValid {
		return d.rowModelMemo
	}
	// Reuse the memo's backing array — the row model is rebuilt on every edit,
	// and reallocating O(rows) per keystroke shows up on large documents.
	// Callers treat the returned slice as transient (read within the call).
	rows := d.rowModelMemo[:0]
	pos, rowStart, open := 0, 0, false
	for _, seg := range d.segs {
		if ts, ok := seg.(*TextSegment); ok {
			for _, r := range ts.Text {
				if !open {
					rowStart, open = pos, true
				}
				if r == '\n' {
					rows = append(rows, richRow{rowStart, pos, false})
					pos++
					rowStart, open = pos, true // a new (possibly empty) line opens after '\n'
				} else {
					pos++
				}
			}
			continue
		}
		if seg.Inline() {
			// Inline image: flows within the current row as one position, opening a
			// row if none is (an image at line start), never breaking it (#665).
			if !open {
				rowStart, open = pos, true
			}
			pos++
			continue
		}
		// block atom: close any open text row (it stays a row), then own a row
		if open {
			rows = append(rows, richRow{rowStart, pos, false})
			open = false
		}
		rows = append(rows, richRow{pos, pos + 1, true})
		pos++
	}
	if open {
		rows = append(rows, richRow{rowStart, pos, false})
	} else if len(rows) == 0 {
		rows = append(rows, richRow{0, 0, false})
	}
	d.rowModelMemo, d.rowModelValid = rows, true
	return rows
}

// activeRowModel is the row model the caret/selection mapping resolves against:
// the wrapping-aware model built from the provider when wrapping is on (#653),
// else the fast newline-count model. Both use the same [start, end] convention —
// a row spans caret columns 0..end-start and any separator consumed between two
// rows (a '\n', or the space a soft wrap drops) sits in the one-position gap
// between them — so offsetToRowCol/rowColToOffset are identical for either.
func (d *richEntryDoc) activeRowModel() []richRow {
	if d.wrapRows != nil {
		return d.wrapRows
	}
	return d.rowModel()
}

// setWrapRows installs a wrapping-aware row model (see buildWrapRows). Passing
// nil reverts to the fast newline-count path.
func (d *richEntryDoc) setWrapRows(rows []richRow) {
	d.wrapRows = rows
}

// buildWrapRows derives the wrapping-aware row model from the provider's laid-out
// rowBounds (#653). Each visual row's document range is
// [segBase(firstSegment)+begin, +rowLength) — begin is the row's rune offset into
// its first segment and segBase is that segment's cumulative document position, so
// this handles soft-wrap splits within a run and multi-segment inline rows alike.
// A block image row occupies its single position. The provider renders exactly
// d.segs, so a row boundary's segment pointers key straight into the base map.
func (d *richEntryDoc) buildWrapRows(p *RichText) []richRow {
	base := make(map[RichTextSegment]int, len(d.segs))
	pos := 0
	for _, seg := range d.segs {
		base[seg] = pos
		pos += segPositions(seg)
	}
	total := pos

	n := p.rows()
	if n == 0 {
		return []richRow{{0, 0, false}}
	}
	rows := make([]richRow, n)
	for r := 0; r < n; r++ {
		b := p.rowBoundary(r)
		if b == nil || len(b.segments) == 0 {
			rows[r] = richRow{start: total, end: total, image: p.isBlockRow(r)}
			continue
		}
		start := total
		if bs, ok := base[b.segments[0]]; ok {
			start = bs + b.begin
		}
		// Walk the row's segments to count its positions. A text segment
		// contributes the runes in its row slice ([begin, end) for the first/last
		// segment, the whole run in between); an atom (inline image) contributes
		// exactly one — never trust rowBoundary.end for a trailing atom, where it
		// carries a stale text offset (#665).
		positions := 0
		last := len(b.segments) - 1
		for i, seg := range b.segments {
			ts, ok := seg.(*TextSegment)
			if !ok {
				positions++ // atom
				continue
			}
			runes := len([]rune(ts.Text))
			lo, hi := 0, runes
			if i == 0 {
				lo = b.begin
			}
			if i == last {
				hi = b.end
			}
			if hi < lo {
				hi = lo
			}
			positions += hi - lo
		}
		rows[r] = richRow{start: start, end: start + positions, image: p.isBlockRow(r)}
	}
	return rows
}

// rowCount returns the number of rendered rows.
func (d *richEntryDoc) rowCount() int {
	return len(d.activeRowModel())
}

// rowIsImage reports whether the given rendered row is a block image.
func (d *richEntryDoc) rowIsImage(row int) bool {
	rows := d.activeRowModel()
	return row >= 0 && row < len(rows) && rows[row].image
}

// offsetToRowCol maps a position to the rendered (row, col). An image row's start
// position is claimed by the image row (caret before the image); every other
// position resolves to the text row that contains it.
func (d *richEntryDoc) offsetToRowCol(offset int) (row, col int) {
	rows := d.activeRowModel()
	for r, rw := range rows {
		if rw.image && rw.start == offset {
			return r, 0
		}
	}
	for r, rw := range rows {
		if rw.image {
			continue
		}
		if offset >= rw.start && offset <= rw.end {
			return r, offset - rw.start
		}
	}
	// The offset fell in a consumed inter-row gap — normally impossible (a single
	// '\n' or wrapped space sits exactly at the preceding row's end), but a soft
	// wrap that dropped more than one space leaves interior gap positions. Assign
	// them to the nearest preceding text row (clamped to its end), never the
	// document end. Handles the past-the-end case too.
	for r := len(rows) - 1; r >= 0; r-- {
		if !rows[r].image && rows[r].start <= offset {
			return r, min(offset-rows[r].start, rows[r].end-rows[r].start)
		}
	}
	if n := len(rows); n > 0 {
		last := rows[n-1]
		return n - 1, last.end - last.start
	}
	return 0, 0
}

// rowColToOffset is the inverse of offsetToRowCol.
func (d *richEntryDoc) rowColToOffset(row, col int) int {
	rows := d.activeRowModel()
	if len(rows) == 0 {
		return 0
	}
	if row < 0 {
		row = 0
	}
	if row >= len(rows) {
		row = len(rows) - 1
	}
	rw := rows[row]
	if rw.image {
		return rw.start
	}
	if col < 0 {
		col = 0
	}
	return rw.start + min(col, rw.end-rw.start)
}

// normalize drops empty text segments, merges adjacent text segments that share
// an identical style, forces every text segment inline, and guarantees at least
// one segment remains.
func (d *richEntryDoc) normalize() {
	var out []RichTextSegment
	for _, seg := range d.segs {
		ts, ok := seg.(*TextSegment)
		if !ok {
			out = append(out, seg)
			continue
		}
		ts.Style.Inline = true
		if ts.Text == "" {
			continue
		}
		if len(out) > 0 {
			if prev, ok := out[len(out)-1].(*TextSegment); ok && richStyleEqual(prev.Style, ts.Style) {
				prev.Text += ts.Text
				continue
			}
		}
		out = append(out, ts)
	}
	if len(out) == 0 {
		out = []RichTextSegment{&TextSegment{Style: RichTextStyle{Inline: true}}}
	}
	d.segs = out
	// Content/structure changed — the memoized wrap-off row model and the
	// position index are both stale.
	d.rowModelValid = false
	d.posIndexValid = false
}

// --- small slice/int helpers (kept local to avoid touching shared utils) ---

// cloneSegment returns an independent copy of a segment (text runs and image
// atoms are copied; unknown segment types pass through).
func cloneSegment(seg RichTextSegment) RichTextSegment {
	switch s := seg.(type) {
	case *TextSegment:
		return &TextSegment{Style: s.Style, Text: s.Text}
	case *ImageSegment:
		cp := *s
		return &cp
	default:
		return seg
	}
}

func concatRunes(parts ...[]rune) []rune {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]rune, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func replaceSegment(segs []RichTextSegment, idx int, repl []RichTextSegment) []RichTextSegment {
	out := make([]RichTextSegment, 0, len(segs)-1+len(repl))
	out = append(out, segs[:idx]...)
	out = append(out, repl...)
	out = append(out, segs[idx+1:]...)
	return out
}

func insertSegment(segs []RichTextSegment, at int, seg RichTextSegment) []RichTextSegment {
	out := make([]RichTextSegment, 0, len(segs)+1)
	out = append(out, segs[:at]...)
	out = append(out, seg)
	out = append(out, segs[at:]...)
	return out
}

func clampRange(from, to, n int) (int, int) {
	if from > to {
		from, to = to, from
	}
	if from < 0 {
		from = 0
	}
	if to > n {
		to = n
	}
	return from, to
}
