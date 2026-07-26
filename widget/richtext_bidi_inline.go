package widget

import (
	paint "github.com/belluikit/bellui/internal/painter"
)

// Bidi-aware caret/selection geometry for rows mixing inline non-text atoms
// (inline images, #665) with right-to-left text (#913).
//
// Two column spaces meet here. RichEntry addresses content in document
// positions, where each inline atom occupies exactly one position; the bidi
// visual layout (rowVisualLayout) indexes carets by text-only rune columns,
// modelling each atom as an opaque box with visual extent but no column. The
// LTR-only inlineColumnX walk bridged the spaces by measuring prefixes, which
// is wrong under bidi reordering; these helpers instead resolve a document
// column into the visual layout — a text column, or an insertion point on the
// correct side of a box — and ask the layout for the visual geometry.

// rowHasInlineBox reports whether the row holds an inline non-text atom.
func (t *RichText) rowHasInlineBox(row int) bool {
	b := t.rowBoundary(row)
	if b == nil {
		return false
	}
	for _, seg := range b.segments {
		if _, isText := seg.(*TextSegment); !isText && seg.Inline() {
			return true
		}
	}
	return false
}

// rowDocColResolve maps a document-space column on the given row to the
// text-only column space of the row's visual layout. textCol is the count of
// text runes logically before the column (always valid); when the column sits
// at an inline atom boundary, isBox is true with the atom's logical box index
// and which side (afterBox) the insertion point is on. The per-segment rune
// windows mirror rowStyledRuns exactly, so textCol indexes the layout the row
// actually shaped.
func (t *RichText) rowDocColResolve(row, docCol int) (textCol, box int, afterBox, isBox bool) {
	b := t.rowBoundary(row)
	if b == nil {
		return 0, 0, false, false
	}
	if docCol < 0 {
		docCol = 0
	}
	consumed, boxIdx := 0, 0
	nSegs := len(b.segments)
	for i, seg := range b.segments {
		ts, isText := seg.(*TextSegment)
		if !isText {
			if docCol == consumed {
				return textCol, boxIdx, false, true
			}
			consumed++
			if docCol == consumed {
				return textCol, boxIdx, true, true
			}
			boxIdx++
			continue
		}
		runes := []rune(ts.Text)
		start, end := 0, len(runes)
		if i == 0 {
			start = b.begin
		}
		if i == nSegs-1 && b.end <= len(runes) && (b.end > 0 || b.begin >= b.end) {
			end = b.end
		}
		if start > end {
			start = end
		}
		n := end - start
		if docCol < consumed+n || (i == nSegs-1 && docCol <= consumed+n) {
			return textCol + (docCol - consumed), 0, false, false
		}
		consumed += n
		textCol += n
	}
	return textCol, 0, false, false
}

// rowBoxDocPositions returns the document-space position of each inline atom
// in the row, in logical (box) order, using the same rune windows as
// rowDocColResolve.
func (t *RichText) rowBoxDocPositions(row int) []int {
	b := t.rowBoundary(row)
	if b == nil {
		return nil
	}
	var positions []int
	consumed := 0
	nSegs := len(b.segments)
	for i, seg := range b.segments {
		ts, isText := seg.(*TextSegment)
		if !isText {
			positions = append(positions, consumed)
			consumed++
			continue
		}
		runes := []rune(ts.Text)
		start, end := 0, len(runes)
		if i == 0 {
			start = b.begin
		}
		if i == nSegs-1 && b.end <= len(runes) && (b.end > 0 || b.begin >= b.end) {
			end = b.end
		}
		if start > end {
			start = end
		}
		consumed += end - start
	}
	return positions
}

// bidiInlineColumnX resolves the caret x for a document-space column on a row
// that mixes inline atoms with right-to-left text — the bidi-aware counterpart
// of the LTR-only inlineColumnX (#913). The returned x uses the same baseline
// convention (innerPad-inset, gutter excluded). ok=false when the row has no
// inline atom or no RTL content, so callers keep the existing paths unchanged.
func (t *RichText) bidiInlineColumnX(row, docCol int, innerPad float32, downstream bool) (float32, bool) {
	if !t.rowHasInlineBox(row) {
		return 0, false
	}
	layout, ok := t.rowVisualLayout(row)
	if !ok {
		return 0, false
	}
	off := innerPad - t.inset.Width
	textCol, box, afterBox, isBox := t.rowDocColResolve(row, docCol)
	if isBox {
		return off + layout.BoxCaretX(box, afterBox), true
	}
	aff := paint.AffinityDownstream
	if !downstream {
		aff = paint.AffinityUpstream
	}
	return off + layout.CaretXAt(textCol, aff), true
}

// bidiInlineSelectionSpans returns the disjoint visual x-intervals covering
// the document-space column range [startDocCol, endDocCol) on a row that
// mixes inline atoms with right-to-left text — the bidi counterpart of the
// LTR selection walk over inlineColumnX (#893, #913). An atom is filled
// exactly when its own document position lies inside the range, matching
// RichEntry's position-space selection model. Coordinates match
// rtlSelectionSpans (offset by innerPad-inset). ok=false when the row has no
// inline atom or no RTL content.
func (t *RichText) bidiInlineSelectionSpans(row, startDocCol, endDocCol int, innerPad float32) ([][2]float32, bool) {
	if startDocCol > endDocCol {
		startDocCol, endDocCol = endDocCol, startDocCol
	}
	if !t.rowHasInlineBox(row) {
		return nil, false
	}
	layout, ok := t.rowVisualLayout(row)
	if !ok {
		return nil, false
	}
	startText, _, _, _ := t.rowDocColResolve(row, startDocCol)
	endText, _, _, _ := t.rowDocColResolve(row, endDocCol)
	boxDoc := t.rowBoxDocPositions(row)

	spans := layout.SelectionSpansBoxes(startText, endText, func(box int) bool {
		return box < len(boxDoc) && boxDoc[box] >= startDocCol && boxDoc[box] < endDocCol
	})
	off := innerPad - t.inset.Width
	for i := range spans {
		spans[i][0] += off
		spans[i][1] += off
	}
	return spans, true
}

// rowTextColToDoc maps a text-only column (the layout's rune-indexed caret
// space) to a document-space column on the given row, the inverse of
// rowDocColResolve's text side: it walks the row's segments in document order —
// the same rune windows as rowDocColResolve/rowBoxDocPositions — counting each
// atom as one document position, and stops when the text runes reach textCol.
// Atoms logically before the text column are counted; atoms after it are not.
func (t *RichText) rowTextColToDoc(row, textCol int) int {
	b := t.rowBoundary(row)
	if b == nil {
		return textCol
	}
	if textCol < 0 {
		textCol = 0
	}
	consumedText, docCol := 0, 0
	nSegs := len(b.segments)
	for i, seg := range b.segments {
		ts, isText := seg.(*TextSegment)
		if !isText {
			docCol++ // atom
			continue
		}
		runes := []rune(ts.Text)
		start, end := 0, len(runes)
		if i == 0 {
			start = b.begin
		}
		if i == nSegs-1 && b.end <= len(runes) && (b.end > 0 || b.begin >= b.end) {
			end = b.end
		}
		if start > end {
			start = end
		}
		n := end - start
		if textCol <= consumedText+n {
			return docCol + (textCol - consumedText)
		}
		consumedText += n
		docCol += n
	}
	return docCol
}

// bidiInlineHitTest maps a visual x on a row that mixes inline atoms with
// right-to-left text back to a caret position — the bidi-aware inverse of
// bidiInlineColumnX and the box-aware counterpart of the LTR inlineColumnAt
// (#983). x uses the same baseline as bidiInlineColumnX (innerPad-inset, gutter
// excluded); the row's visual layout resolves the hit against text carets and
// inline-box edges alike, so a click on an image resolves to the atom rather
// than snapping to the nearest text caret (HitTestAffinity is box-blind).
//
// It returns the position in both column spaces (textCol text-only, docCol
// atom-aware) so either caller model fits, mirroring inlineColumnAt. ok is false
// when the row has no inline atom or no RTL content, so callers keep the
// existing HitTestAffinity path unchanged for those rows.
func (t *RichText) bidiInlineHitTest(row int, x, innerPad float32) (textCol, docCol int, downstream, ok bool) {
	if !t.rowHasInlineBox(row) {
		return 0, 0, true, false
	}
	layout, layoutOK := t.rowVisualLayout(row)
	if !layoutOK {
		return 0, 0, true, false
	}
	off := innerPad - t.inset.Width
	tcol, aff, box, after, isBox := layout.HitTestBoxes(x - off)
	if isBox {
		boxDoc := t.rowBoxDocPositions(row)
		dc := 0
		if box >= 0 && box < len(boxDoc) {
			dc = boxDoc[box]
		}
		if after {
			dc++
		}
		// The text column at the atom boundary is the runes logically before it.
		tc, _, _, _ := t.rowDocColResolve(row, dc)
		return tc, dc, true, true
	}
	return tcol, t.rowTextColToDoc(row, tcol), aff == paint.AffinityDownstream, true
}
