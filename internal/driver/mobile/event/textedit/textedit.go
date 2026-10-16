// Package textedit defines the committed text-edit event used by the mobile
// driver to carry a native IME/keyboard edit — a range replacement — from the
// platform layer to the focused editor through the text-input mirror seam
// (pre-rebrand tracker #771). It is the committed-edit counterpart of the composition (preedit)
// event: composition carries in-progress preedit; textedit carries a finalized
// edit that replaces a rune range with new text.
package textedit

// Event reports a committed edit as a replacement of the half-open [Start, End)
// rune range with Text. A bare insertion has Start == End (insert at the
// caret); a deletion has empty Text; typing over a selection replaces the whole
// range in one edit (which is why offsets, not a caret delta, are carried —
// pre-rebrand tracker #748). Offsets are rune indices into the editor's current text.
type Event struct {
	Start int
	End   int
	Text  string

	// Selection marks the event as a pure selection change (pre-rebrand tracker #1030): select the
	// [Start, End) rune range — equal offsets place a bare caret — without
	// touching the text. Text is ignored. Carries the IME's own caret moves and
	// Select All (InputConnection.setSelection) to bellui, which owns the caret:
	// an unforwarded selection is invisible and collapses on the next state
	// push.
	Selection bool
}
