package editor

import (
	"github.com/belluikit/bellui/widget"
)

// TextEdit describes one splice of an [Editor.ApplyEdits] batch: the document
// range Range is replaced by Text. An empty (zero) Range inserts at its start
// position; an empty Text deletes the range.
type TextEdit struct {
	// Range is the replaced document range, in 0-based (row, col) coordinates.
	Range TextRange
	// Text is the replacement.
	Text string
}

// ApplyEdits applies a batch of splices to the document as one atomic edit
// with user-edit semantics — the programmatic counterpart to a keystroke, and
// the API hosts should use for quick fixes, refactors, formatting and other
// tool-driven rewrites (#160):
//
//   - one undo step, one [Editor.OnChanged] call with the full new text;
//   - gated like a manual edit: a splice touching a protected range or masked
//     redaction vetoes the WHOLE batch (nothing is applied, reported false);
//   - anchor-tracked (#130): protected/redaction/fold ranges shift through the
//     edit, no host re-publish needed;
//   - the caret lands at the end of the last splice's replacement text;
//   - a sync point under [Editor.DeferTextSync].
//
// All ranges address the document as it is BEFORE the batch; they may be given
// in any order but must not overlap (edits touching at an endpoint are fine).
// Out-of-range positions are clamped. It reports whether the batch was applied
// — false for an empty or all-no-op batch, overlapping ranges, or a veto.
//
// Completion and signature help do not re-trigger on the edit, matching
// completion acceptance. Contrast [Editor.SetText], which is a document SWAP:
// no OnChanged, undo history cleared, anchored ranges left for the host to
// re-publish.
func (e *Editor) ApplyEdits(edits []TextEdit) bool {
	if len(edits) == 0 {
		return false
	}
	text := e.docText()
	wEdits := make([]widget.TextEdit, len(edits))
	for i, ed := range edits {
		start := rowColToOffset(text, ed.Range.StartRow, ed.Range.StartCol)
		end := rowColToOffset(text, ed.Range.EndRow, ed.Range.EndCol)
		if start > end {
			start, end = end, start
		}
		wEdits[i] = widget.TextEdit{Start: start, End: end, Text: ed.Text}
	}

	e.suppressCompletion = true
	applied := e.entry.ApplyEdits(wEdits)
	if applied {
		// The fork defers the batch like any other edit under DeferTextSync;
		// materialize here so this API is a sync point as documented — hosts
		// observe OnChanged (and Text) immediately. A cheap no-op otherwise.
		e.entry.SyncText()
	}
	e.suppressCompletion = false
	return applied
}

// ApplyEdit replaces one document range with text — the single-splice
// convenience over [Editor.ApplyEdits], with the same user-edit semantics.
func (e *Editor) ApplyEdit(r TextRange, text string) bool {
	return e.ApplyEdits([]TextEdit{{Range: r, Text: text}})
}
