// Package composition defines the IME composition (preedit) event used by the
// mobile driver to carry input-method composition state from the native
// platform layer to the focused widget.
package composition

// Event reports IME composition progress on mobile platforms. An in-progress
// composition delivers preedit updates (Commit false) terminated by a single
// commit (Commit true) carrying the finalized text.
type Event struct {
	// Text is the composition text. While composing it is the inline preedit;
	// on commit it is the finalized text to insert.
	Text string

	// CursorPos is the caret position within Text, measured in runes. Only
	// meaningful for preedit updates.
	CursorPos int

	// Commit is true when Text is the final committed value rather than an
	// in-progress preedit update.
	Commit bool
}
