package editor

// RevealRange scrolls r into view and selects it, leaving the cursor at the end
// of the range. It is the host's hook for "click a problem → jump to it" (a
// problems list calling back into the editor) and is reused by go-to-definition.
//
// Revealing relies on the Entry's KeepSelectionVisible to scroll an off-screen
// range into the viewport — the same mechanism the find bar's next-match
// navigation uses. A zero-width (point) range still scrolls and places the
// cursor at that position, so go-to-definition can land the caret precisely.
//
// If the target lies inside a collapsed fold, RevealRange first unfolds it (and
// any enclosing collapsed regions) so the caret can land on a visible line —
// jumping to a problem, match, or definition inside a fold exposes it.
//
// RevealRange does not take keyboard focus; a host that wants the user to type
// immediately after jumping should follow it with [Editor.Focus].
func (e *Editor) RevealRange(r TextRange) {
	e.fold.revealRows(r.StartRow, r.EndRow)
	e.entry.Select(r.StartRow, r.StartCol, r.EndRow, r.EndCol)
	e.syncExternalWidgets()
}
