package editor

// Anchor-tracking (#130): protected ranges, redactions and fold regions are all
// caller-managed (row, col) ranges. Before this file, fycoder re-derived their
// rune offsets from the current text on each edit — robust to edits that keep
// the line count, but an insert/delete of a line above a range drifted it until
// the host re-published. Here the editor tracks every text change and shifts
// the stored ranges themselves, so line-count shifts no longer need a host
// re-publish.
//
// The edit is recovered from the OnChanged text pair rather than observed at
// the edit sites: a common prefix/suffix diff of old vs. new yields the single
// contiguous splice every interactive path produces (typing, paste, IME
// commit, InsertText). That one seam also covers the paths that never fire
// BeforeChange — undo and redo replay history verbatim — and an ApplyEdits
// batch coalesces into one covering splice, which shifts ranges outside it
// exactly and treats the batch's interior conservatively as replaced text.
// Programmatic SetText is a document swap, not an edit: tracking resets and
// the host re-publishes ranges as before.
//
// Gravity at range boundaries follows #108's atomic-edge behavior: a range
// does not absorb text inserted exactly at its start or end (the insert lands
// outside; the range shifts or stays), while an edit strictly inside a range
// resizes it. A deletion that swallows a range drops it; one that eats its
// head or tail truncates it.

// spliceBounds recovers the single contiguous splice turning old into new: old
// runes [start, oldEnd) were replaced by new runes [start, newEnd). changed is
// false when the texts are identical. Offsets are rune indices.
func spliceBounds(old, new []rune) (start, oldEnd, newEnd int, changed bool) {
	lo, ln := len(old), len(new)
	max := lo
	if ln < max {
		max = ln
	}
	p := 0
	for p < max && old[p] == new[p] {
		p++
	}
	if p == lo && p == ln {
		return 0, 0, 0, false
	}
	s := 0
	for s < max-p && old[lo-1-s] == new[ln-1-s] {
		s++
	}
	return p, lo - s, ln - s, true
}

// shiftSpan maps an anchored span [as, ae) through the splice replacing
// [es, eeOld) with [es, eeNew). keep is false when the deletion swallowed the
// span entirely. Offsets are rune indices into the pre-edit text; the returned
// span indexes the post-edit text.
func shiftSpan(as, ae, es, eeOld, eeNew int) (newStart, newEnd int, keep bool) {
	delta := eeNew - eeOld
	switch {
	case eeOld <= as:
		// Splice entirely before the span — including an insert exactly at its
		// start, which the span does not absorb (boundary gravity): shift whole.
		return as + delta, ae + delta, true
	case es >= ae:
		// Splice entirely after — including an insert exactly at the end, which
		// does not extend the span: untouched.
		return as, ae, true
	case es >= as && eeOld <= ae:
		// Edit strictly inside (or flush with one edge): the span resizes.
		return as, ae + delta, true
	case es < as && eeOld >= ae:
		// The deletion swallowed the span.
		return 0, 0, false
	case es < as:
		// The span's head was eaten: what survives starts where the replacement
		// text ends.
		return eeNew, ae + delta, true
	default:
		// The span's tail was eaten: truncate at the splice.
		return as, es, true
	}
}

// shiftRanges maps caller-managed (row, col) ranges through the oldText→newText
// edit. It returns the ranges re-anchored against newText and whether anything
// differs from the input; swallowed ranges are dropped. Ranges that flatten to
// empty are passed through untouched (they protect/mask/fold nothing).
func shiftRanges(oldText, newText string, ranges []TextRange) ([]TextRange, bool) {
	if len(ranges) == 0 {
		return ranges, false
	}
	es, eeOld, eeNew, changed := spliceBounds([]rune(oldText), []rune(newText))
	if !changed {
		return ranges, false
	}
	out := make([]TextRange, 0, len(ranges))
	dirty := false
	for _, r := range ranges {
		s, e := rangeToOffsets(oldText, r)
		if s >= e {
			out = append(out, r)
			continue
		}
		ns, ne, keep := shiftSpan(s, e, es, eeOld, eeNew)
		if !keep {
			dirty = true
			continue
		}
		if ns == s && ne == e {
			// Same flat offsets, but the (row, col) shape may still change when
			// the splice rewrites line structure before the span on its line.
			nr := offsetsToRange(newText, ns, ne)
			if nr != r {
				dirty = true
			}
			out = append(out, nr)
			continue
		}
		dirty = true
		out = append(out, offsetsToRange(newText, ns, ne))
	}
	return out, dirty
}

// offsetsToRange converts half-open flat rune offsets into a (row, col)
// TextRange against text — the inverse of rangeToOffsets.
func offsetsToRange(text string, start, end int) TextRange {
	sr, sc := offsetToRowCol(text, start)
	er, ec := offsetToRowCol(text, end)
	return TextRange{StartRow: sr, StartCol: sc, EndRow: er, EndCol: ec}
}

// trackAnchors shifts every anchored range set through the edit that produced
// newText. Called from the Entry's OnChanged before the redaction reconcile so
// the re-derived caret-bar offsets see the already-shifted spans. Programmatic
// SetText (suppressChange) resets the snapshot without shifting: it is a
// document swap and hosts re-publish ranges for it, as before anchor-tracking.
func (e *Editor) trackAnchors(newText string) {
	oldText := e.anchorText
	e.anchorText = newText
	if e.suppressChange || oldText == newText {
		return
	}

	if e.protected != nil && len(e.protected.ranges) > 0 {
		if shifted, dirty := shiftRanges(oldText, newText, e.protected.ranges); dirty {
			e.protected.ranges = shifted
		}
	}
	if e.redaction != nil && len(e.redaction.ranges) > 0 {
		if shifted, dirty := shiftRanges(oldText, newText, e.redaction.ranges); dirty {
			// The caller's OnChanged reconcile re-derives the caret-bar offsets
			// from these shifted spans; the repaint re-reads Spans() live.
			e.redaction.ranges = shifted
			e.RefreshDecorators()
		}
	}
	if e.fold != nil && len(e.fold.regions) > 0 {
		if shifted, dirty := shiftFoldRegions(e.fold.regions, oldText, newText); dirty {
			e.fold.regions = shifted
			e.fold.apply() // re-derive HiddenRanges from the shifted regions
		}
	}
}

// shiftFoldRegions shifts each fold region's Range through the edit, preserving
// Kind/Collapsed/Summary and dropping regions the edit swallowed. dirty reports
// whether anything differs from the input.
func shiftFoldRegions(regions []FoldRegion, oldText, newText string) ([]FoldRegion, bool) {
	es, eeOld, eeNew, changed := spliceBounds([]rune(oldText), []rune(newText))
	if !changed {
		return regions, false
	}
	out := make([]FoldRegion, 0, len(regions))
	dirty := false
	for _, fr := range regions {
		s, e := rangeToOffsets(oldText, fr.Range)
		if s >= e {
			out = append(out, fr)
			continue
		}
		ns, ne, keep := shiftSpan(s, e, es, eeOld, eeNew)
		if !keep {
			dirty = true
			continue
		}
		nr := offsetsToRange(newText, ns, ne)
		if nr != fr.Range {
			dirty = true
		}
		fr.Range = nr
		out = append(out, fr)
	}
	return out, dirty
}
