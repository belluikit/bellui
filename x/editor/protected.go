package editor

import (
	"strings"

	"github.com/belluikit/bellui/widget"
)

// SetProtectedRanges marks document ranges as read-only: any edit whose effect
// falls inside (or partially overlaps) one of them is rejected before it
// applies, across every interactive edit path (typing, IME commit, paste, cut,
// delete/backspace, word-delete, newline, and programmatic select-overwrite). A
// rejected edit never reaches the undo stack and cannot be re-introduced by
// undo/redo.
//
// Ranges are stored as (row, col) and anchor-tracked through edits (#130): an
// insert or delete above a range shifts it, text inserted exactly at a range's
// start or end lands outside it (the range does not grow, matching #108's
// atomic-edge behavior), and undo/redo shift ranges back — no host re-publish
// needed. A programmatic [Editor.SetText] is a document swap, not an edit:
// ranges are left as published and the host supplies ranges for the new
// document.
//
// Passing nil or an empty slice clears all protection, restoring normal editing.
func (e *Editor) SetProtectedRanges(ranges []TextRange) {
	e.protected.setRanges(ranges)
}

// ProtectedRanges returns a copy of the currently protected ranges, in the order
// they were last set. It returns nil when none are set.
func (e *Editor) ProtectedRanges() []TextRange {
	return e.protected.rangesCopy()
}

// OnProtectedEditAttempt registers a callback fired whenever an edit is rejected
// for overlapping a protected range. It receives the protected range that was
// hit. Pass nil to clear the callback.
func (e *Editor) OnProtectedEditAttempt(fn func(r TextRange)) {
	e.protected.onAttempt = fn
}

// protectedController owns the read-only ranges and the rejection callback. It
// enforces protection through the Entry's BeforeChange hook (not the
// ProtectedRanges field) because only the hook can surface a rejection callback;
// the field vetoes silently and is reserved for caret-barring of masked
// redactions (Phase 4 deliverable 2).
type protectedController struct {
	e         *Editor
	ranges    []TextRange
	onAttempt func(TextRange)
}

func newProtectedController(e *Editor) *protectedController {
	return &protectedController{e: e}
}

func (c *protectedController) setRanges(ranges []TextRange) {
	if len(ranges) == 0 {
		c.ranges = nil
		return
	}
	c.ranges = append([]TextRange{}, ranges...)
}

func (c *protectedController) rangesCopy() []TextRange {
	if len(c.ranges) == 0 {
		return nil
	}
	return append([]TextRange{}, c.ranges...)
}

// allow is the Entry.BeforeChange hook. It vetoes (returns false) any edit whose
// replaced range [sel.Start, sel.End) overlaps a protected range, firing
// onAttempt with the hit range first. sel offsets are rune indices into the
// current (pre-edit) Text, so protected ranges are converted against that same
// text for an apples-to-apples comparison.
func (c *protectedController) allow(sel widget.TextRange) bool {
	if len(c.ranges) == 0 {
		return true
	}
	// Synchronise deferred edits (#129): sel indexes the live pre-edit content,
	// so the ranges must be flattened against that same text.
	text := c.e.docText()
	for _, r := range c.ranges {
		ps, pe := rangeToOffsets(text, r)
		if editTouchesRange(sel.Start, sel.End, ps, pe) {
			if c.onAttempt != nil {
				c.onAttempt(r)
			}
			return false
		}
	}
	return true
}

// rangeToOffsets flattens a (row, col) TextRange into the half-open rune offsets
// [start, end) into text that the fork's edit gate operates on. It mirrors the
// fork's unexported textPosFromRowCol (rowBoundary(row).begin + col): a row's
// begin is the running sum of preceding lines' rune lengths plus one rune per
// newline. Out-of-range rows and columns are clamped to the text.
func rangeToOffsets(text string, r TextRange) (start, end int) {
	lines := strings.Split(text, "\n")
	return offsetAt(lines, r.StartRow, r.StartCol), offsetAt(lines, r.EndRow, r.EndCol)
}

// offsetAt returns the flat rune offset of (row, col) within lines (the text
// split on "\n"). lines always has at least one element.
func offsetAt(lines []string, row, col int) int {
	if row < 0 {
		row, col = 0, 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
		col = len([]rune(lines[row]))
	}
	off := 0
	for i := 0; i < row; i++ {
		off += len([]rune(lines[i])) + 1 // +1 for the newline joining the rows
	}
	n := len([]rune(lines[row]))
	if col < 0 {
		col = 0
	}
	if col > n {
		col = n
	}
	return off + col
}

// editTouchesRange mirrors the fork's unexported overlap test so fycoder's
// BeforeChange veto matches the fork's ProtectedRanges semantics exactly. A
// deletion or replacement (start < end) is blocked when it overlaps the span at
// all; a pure insertion (start == end) is blocked only when it falls strictly
// inside the span, so text can grow up against either boundary.
func editTouchesRange(start, end, ps, pe int) bool {
	if ps >= pe {
		return false // an empty span protects nothing
	}
	if start == end {
		return start > ps && start < pe
	}
	return start < pe && end > ps
}

// editTouchesRangeAtomic is the stricter sibling of editTouchesRange used for
// masked redactions, which must be atomic. It blocks the same deletions and
// overlapping replacements, but additionally blocks a pure insertion AT either
// boundary (start >= ps && start <= pe, not just strictly inside) so text cannot
// grow up against — and shift — a masked secret. Plain protected ranges
// deliberately allow boundary inserts so callers can append next to a read-only
// span; a masked span cannot, or the secret slides out from under its bar (#108).
func editTouchesRangeAtomic(start, end, ps, pe int) bool {
	if ps >= pe {
		return false // an empty span protects nothing
	}
	if start == end {
		return start >= ps && start <= pe
	}
	return start < pe && end > ps
}
