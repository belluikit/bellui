package widget

import (
	"slices"
	"strings"
)

// This file holds Entry's programmatic editing surface: the selection-range
// accessor (#431), the insert-at-caret primitive (#432), the atomic
// multi-range edit with its compound undo action (#433) and the logical
// line-count accessor (#459). All exist so a host (e.g. a code editor doing
// block indent/dedent or sizing a gutter) can read and mutate the content
// through the same gated, undo-recorded paths interactive edits use.

// LineCount returns the number of logical (newline-delimited) lines in the
// entry's content; an empty entry has one line. Under [Entry.DeferTextSync]
// it reflects the live buffer, not the possibly-stale Text field, so a host
// can size its gutter or scroll model against a deferred entry without
// forcing a materialization. O(1) on the lazy single-segment path (the
// buffer keeps a line index); O(content) otherwise.
//
// Since: 1.0
func (e *Entry) LineCount() int {
	p := &e.text
	// The identity chain (#400/#410) keeps Entry.Text and the buffer's
	// string anchor as the same object while the buffer is current —
	// including between DeferTextSync sync points, where the anchor is the
	// last-synced string and the buffer runs ahead with the live edits. A
	// direct external Text write breaks that identity (the compare fails
	// fast on the pointer), and the count follows the new field value that
	// the next refresh will apply.
	if p.lazy && p.buf != nil && e.Text == p.lazyStr {
		return p.buf.LineCount()
	}
	return strings.Count(e.Text, "\n") + 1
}

// SelectionRange returns the current selection as a half-open rune range
// [Start, End) into the entry's content, normalized so Start <= End. ok is
// false when nothing is selected. The caret always sits at one end of the
// range; compare [Entry.CursorTextOffset] against Start and End to recover
// which end is the anchor.
//
// Since: 1.0
func (e *Entry) SelectionRange() (r TextRange, ok bool) {
	if e.sel == nil || !e.sel.selecting {
		return TextRange{}, false
	}
	e.syncSelectable()
	a, b := e.sel.selection()
	if a < 0 || a == b {
		return TextRange{}, false
	}
	return TextRange{Start: a, End: b}, true
}

// InsertText inserts text at the caret as one edit, exactly as if it had been
// typed in a single action: any active selection is replaced, the edit is
// gated through [Entry.ProtectedRanges] and [Entry.BeforeChange], recorded in
// the undo history and OnChanged fires once. It is the exported form of the
// primitive behind TypedRune and IME commit — use it instead of routing
// programmatic inserts through [Entry.ComposeCommit]. Any in-progress IME
// composition is cancelled first; its preedit is uncommitted display state,
// discarded like on any other programmatic write.
// A no-op for read-only or disabled entries and for an empty string.
// It is safe to call from any goroutine.
//
// Since: 1.0
func (e *Entry) InsertText(text string) {
	onMain(func() {
		e.cancelComposition()
		e.insertRunesAtCursor([]rune(text))
	})
}

// TextEdit describes one splice of an [Entry.ApplyEdits] batch: the half-open
// rune range [Start, End) of the current content is replaced by Text.
// Start == End inserts at that offset; an empty Text deletes the range.
//
// Since: 1.0
type TextEdit struct {
	Start, End int
	Text       string
}

// ApplyEdits applies a set of splices to the entry's content as one atomic
// edit: a single undo/redo step, one Validator run and one OnChanged call
// (one sync point under [Entry.DeferTextSync]). It is the primitive for
// multi-region operations — block indent/dedent, replace-all — that must not
// fragment into per-splice undo steps the way sequential edits do.
//
// All offsets are rune indices into the content as it is before the batch;
// edits are applied in one pass, so no edit observes another's shift. The
// ranges may be given in any order but must not overlap (edits touching at an
// endpoint are fine, and insertions sharing an offset keep their given order).
// Every edit is gated through [Entry.ProtectedRanges] and [Entry.BeforeChange]
// before anything is applied: one veto rejects the whole batch, so a partial
// application can never be observed. Out-of-range offsets are clamped.
//
// It reports whether the batch was applied — false for a read-only or disabled
// entry, an empty or all-no-op batch, overlapping ranges, or a vetoed edit.
// Afterwards any selection is cleared and the caret sits at the end of the
// last edit's replacement text. Any in-progress IME composition is cancelled
// first. It is safe to call from any goroutine.
//
// Since: 1.0
func (e *Entry) ApplyEdits(edits []TextEdit) bool {
	applied := false
	onMain(func() {
		applied = e.applyEdits(edits)
	})
	return applied
}

func (e *Entry) applyEdits(edits []TextEdit) bool {
	if e.isReadOnly() || len(edits) == 0 {
		return false
	}
	e.cancelComposition()

	provider := e.textProvider()
	n := provider.len()

	// Normalize against the current content: order each range, clamp to the
	// document and drop no-ops — an effect-free batch must not reach the undo
	// stack, or undo would need a press to replay nothing.
	norm := make([]TextEdit, 0, len(edits))
	for _, ed := range edits {
		if ed.Start > ed.End {
			ed.Start, ed.End = ed.End, ed.Start
		}
		ed.Start = clampInt(ed.Start, 0, n)
		ed.End = clampInt(ed.End, 0, n)
		if ed.Start == ed.End && ed.Text == "" {
			continue
		}
		norm = append(norm, ed)
	}
	if len(norm) == 0 {
		return false
	}
	slices.SortStableFunc(norm, func(a, b TextEdit) int {
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		return a.End - b.End
	})
	for i := 1; i < len(norm); i++ {
		if norm[i].Start < norm[i-1].End {
			return false // overlapping edits have no well-defined result
		}
	}

	// Gate the whole batch before mutating anything.
	for _, ed := range norm {
		if !e.editAllowed(ed.Start, ed.End, ed.Text) {
			return false
		}
	}

	cb := e.OnChanged
	e.syncSelectable()

	// Apply right-to-left so every splice lands at its offset in the original
	// content, unshifted by the edits below it. Each recorded action keeps its
	// apply-time position, making the compound replays self-consistent: redo
	// re-runs the same descending sequence, undo inverts it in reverse.
	batch := &entryBatchModifyAction{}
	for i := len(norm) - 1; i >= 0; i-- {
		ed := norm[i]
		if ed.Start < ed.End {
			erased := provider.deleteFromTo(ed.Start, ed.End)
			batch.actions = append(batch.actions, &entryModifyAction{Delete: true, Position: ed.Start, Text: erased})
		}
		if ed.Text != "" {
			runes := []rune(ed.Text)
			provider.insertAt(ed.Start, runes)
			batch.actions = append(batch.actions, &entryModifyAction{Position: ed.Start, Text: runes})
		}
	}

	// The caret lands after the last (document-order) edit's replacement,
	// shifted by the length deltas of the edits before it; undo restores it to
	// the end of that edit's original range.
	last := norm[len(norm)-1]
	caret := last.Start + len([]rune(last.Text))
	for _, ed := range norm[:len(norm)-1] {
		caret += len([]rune(ed.Text)) - (ed.End - ed.Start)
	}
	batch.caretRedo = caret
	batch.caretUndo = last.End
	e.undoStack.Add(batch)

	// Deferred text sync (#410): the splices stayed in the buffer, so skip the
	// document materialization and its notifications until the next sync point.
	deferred := e.editDeferred()
	var content string
	if !deferred {
		content = provider.String()
		e.updateText(content, false)
	}
	e.CursorRow, e.CursorColumn = e.rowColFromTextPos(caret)
	e.cursorUpstream = true
	e.syncSelectable()
	e.sel.selecting = false
	e.sel.selectRow, e.sel.selectColumn = e.CursorRow, e.CursorColumn

	if !deferred {
		e.validate()
		if cb != nil {
			cb(content)
		}
	}
	e.Refresh()
	return true
}

// entryBatchModifyAction groups the splices of one atomic multi-range edit
// (Entry.ApplyEdits) into a single undo step. actions holds the primitive
// splices in application order — descending document position, so every
// recorded Position is valid in the content state it is applied to. Redo
// replays them forward; undo inverts them in reverse order. Deliberately not
// mergeable: a batch is one user-visible operation.
type entryBatchModifyAction struct {
	actions   []*entryModifyAction
	caretRedo int
	caretUndo int
}

func (b *entryBatchModifyAction) Apply(p *RichText, redo bool) int {
	if redo {
		for _, a := range b.actions {
			a.Apply(p, true)
		}
		return b.caretRedo
	}
	for i := len(b.actions) - 1; i >= 0; i-- {
		b.actions[i].Apply(p, false)
	}
	return b.caretUndo
}
