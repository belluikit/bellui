package widget

// richEntryEdit is a single reversible edit: it records that the styled content
// `before` occupying [at, at+len(before)) was replaced by `after`. Both ends
// carry the caret offset so undo/redo restore it. Expressing every mutation as a
// styled range-replace makes undo/redo preserve per-run styling (unlike Entry's
// flat rune model, #648/#656).
type richEntryEdit struct {
	at          int
	before      []RichTextSegment
	after       []RichTextSegment
	caretBefore int
	caretAfter  int
}

// segsPositionLen is the total number of caret positions the segments occupy
// (text runes plus one per atom), the unit undo/redo replaces ranges in.
func segsPositionLen(segs []RichTextSegment) int {
	n := 0
	for _, s := range segs {
		n += segPositions(s)
	}
	return n
}

// apply re-does (redo=true) or un-does (redo=false) the edit against the
// document and returns the caret offset to restore.
func (e *richEntryEdit) apply(d *richEntryDoc, redo bool) int {
	if redo {
		d.replaceRange(e.at, e.at+segsPositionLen(e.before), e.after)
		return e.caretAfter
	}
	d.replaceRange(e.at, e.at+segsPositionLen(e.after), e.before)
	return e.caretBefore
}

// tryMerge folds a following edit into this one when both are contiguous
// same-style insertions with no intervening deletion or newline, so a run of
// typed characters collapses into a single undo step (mirrors Entry's word-ish
// granularity: a space or newline ends the run).
func (e *richEntryEdit) tryMerge(next *richEntryEdit) bool {
	if len(e.before) != 0 || len(next.before) != 0 {
		return false // only pure insertions merge
	}
	if next.at != e.at+segsPositionLen(e.after) {
		return false // not contiguous
	}
	if !insertionsMergeable(e.after, next.after) {
		return false
	}
	e.after = append(e.after, cloneSegs(next.after)...)
	e.caretAfter = next.caretAfter
	return true
}

// insertionsMergeable reports whether a newly typed insertion should join the
// previous one. It groups a word together with its trailing whitespace ("foo "),
// breaking when a new word begins after whitespace or at a newline, and requires
// the join point to share a style — so undo/redo advances word by word.
func insertionsMergeable(prev, next []RichTextSegment) bool {
	if len(prev) == 0 || len(next) == 0 {
		return false
	}
	prevSeg, ok1 := prev[len(prev)-1].(*TextSegment)
	nextSeg, ok2 := next[0].(*TextSegment)
	if !ok1 || !ok2 {
		return false
	}
	pr, nx := []rune(prevSeg.Text), []rune(nextSeg.Text)
	if len(pr) == 0 || len(nx) == 0 {
		return false
	}
	prevLast, nextFirst := pr[len(pr)-1], nx[0]
	if prevLast == '\n' || nextFirst == '\n' {
		return false
	}
	if isTypingSpace(prevLast) && !isTypingSpace(nextFirst) {
		return false // a new word starts after whitespace
	}
	return richStyleEqual(prevSeg.Style, nextSeg.Style)
}

func isTypingSpace(r rune) bool { return r == ' ' || r == '\t' }

func cloneSegs(segs []RichTextSegment) []RichTextSegment {
	out := make([]RichTextSegment, 0, len(segs))
	for _, s := range segs {
		out = append(out, cloneSegment(s))
	}
	return out
}

// richEntryUndoStack is a linear undo/redo history (mirrors entryUndoStack): a
// slice of edits with a cursor; adding truncates any redo tail.
type richEntryUndoStack struct {
	items []*richEntryEdit
	index int // number of applied edits (items[:index] are undoable)
}

func (s *richEntryUndoStack) add(e *richEntryEdit) {
	if s.index > 0 && s.items[s.index-1].tryMerge(e) {
		s.items = s.items[:s.index] // a merge may have followed a truncation
		return
	}
	s.items = append(s.items[:s.index], e)
	s.index++
}

func (s *richEntryUndoStack) canUndo() bool { return s.index > 0 }
func (s *richEntryUndoStack) canRedo() bool { return s.index < len(s.items) }

func (s *richEntryUndoStack) undo() *richEntryEdit {
	if !s.canUndo() {
		return nil
	}
	s.index--
	return s.items[s.index]
}

func (s *richEntryUndoStack) redo() *richEntryEdit {
	if !s.canRedo() {
		return nil
	}
	e := s.items[s.index]
	s.index++
	return e
}

func (s *richEntryUndoStack) clear() {
	s.items = nil
	s.index = 0
}
