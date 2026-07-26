package editor

// Deferred text sync (#129, fyne#410). The fork's Entry.DeferTextSync mode
// stops materializing the whole document into Entry.Text on every keystroke:
// edits update only the internal buffer, and Text — with its OnChanged,
// Validator and binding notifications — is refreshed at sync points instead.
// That flattens per-keystroke latency on multi-megabyte documents, but it
// means Entry.Text is stale between sync points.
//
// fycoder's adoption splits its Entry.Text readers into two classes:
//
//   - Discrete-action readers (find, indent, block indent, electric reindent,
//     completion, the protected/redaction edit gates, Editor.Text) read via
//     [Editor.docText], which synchronises first. With the mode off this is
//     exactly Entry.Text.
//
//   - Render-path readers (positionAtPoint, RangeRects) keep reading the raw
//     field; their staleness is bounded to the caret's row between sync
//     points. Geometry counts never go stale at all: lineCount and the
//     gutter/overlay row counts read the fork's live O(1) Entry.LineCount
//     (#149, fyne#459), not the Text field.
//
// Sync triggers, consumed in [Editor.consumeDeferredSync] from the Entry's
// OnCursorChanged (which the fork fires from every content refresh, i.e.
// after every edit):
//
//   - a caret row change — bounds the anchor-tracking diff (#130) to edits
//     within a single row, so the coalesced splice stays contiguous;
//   - a live line-count change — an edit added or removed a newline (Enter,
//     line-join backspace/delete, multi-line paste or selection replace), so
//     anchors and decorations re-derive against the new line structure. The
//     comparison is O(1) via Entry.LineCount, so mid-row deletions no longer
//     sync at all (#149) — held-down Backspace/Delete stays flat;
//   - [Editor.noteDeferredEdit], flagged from the BeforeChange gate when an
//     active feature needs the full current text on every edit
//     (completion/signature providers, protected ranges, masked redactions,
//     a keyboard-invoked hover card);
//   - undo/redo, flagged by the shortcut dispatcher — history replay bypasses
//     the BeforeChange gate and can restore mid-row content with no row or
//     line-count change.
//
// Focus loss is a sync point inside the fork itself; [Editor.ApplyEdits]
// syncs explicitly after applying (the fork defers the batch like any other
// edit). Active folds disable the fork's deferred path entirely (Text stays
// current), so fold rendering needs no special handling.

// SetDeferTextSync toggles deferred text sync (see [Editor.DeferTextSync]) at
// runtime — e.g. after loading a large document into an already-shown editor.
// Disabling flushes any pending edits first, so Entry.Text and OnChanged
// observers are current before per-keystroke behavior resumes.
func (e *Editor) SetDeferTextSync(on bool) {
	e.DeferTextSync = on
	if !on {
		e.entry.SyncText()
	}
	e.entry.DeferTextSync = on
	e.entry.Refresh()
}

// docText returns the current document text. Under [Editor.DeferTextSync] it
// synchronises pending edits first — Entry.Text is refreshed and the deferred
// OnChanged/Validator/binding notifications run — so callers always see the
// live content. With the mode off it is exactly Entry.Text (SyncText returns
// the field without side effects when nothing is deferred).
func (e *Editor) docText() string {
	return e.entry.SyncText()
}

// noteDeferredEdit runs inside the Entry's BeforeChange gate, after the
// protected/redaction vetoes have passed, and flags a sync when an active
// feature needs the full current text on every edit. Line-structure changes
// need no flagging here: the pump compares the live Entry.LineCount after the
// edit applies (#149), which is both cheaper and exact — the pre-#149
// classification conservatively synced on every deletion because the gate
// could not tell whether a newline was dying.
func (e *Editor) noteDeferredEdit() {
	if !e.DeferTextSync || e.deferredSyncPending {
		return
	}
	if e.deferredPerEditSync() {
		e.deferredSyncPending = true
	}
}

// deferredPerEditSync reports whether some active feature needs the full
// current text after every edit, forcing a per-edit sync point: completion
// and signature help re-query on each keystroke, the protected/redaction
// gates (both fycoder's and the fork's ProtectedRanges check) compare edit
// offsets against text-derived range offsets, and a keyboard-invoked hover
// card dismisses on the first edit. Editors using these features keep
// today's per-keystroke behavior; the deferred win applies to plain large-
// document editing.
func (e *Editor) deferredPerEditSync() bool {
	if e.completion != nil && e.completion.provider != nil {
		return true
	}
	if e.signature != nil && e.signature.provider != nil {
		return true
	}
	if e.protected != nil && len(e.protected.ranges) > 0 {
		return true
	}
	if e.redaction != nil && e.redaction.mode == Masked && len(e.redaction.ranges) > 0 {
		return true
	}
	if e.hover != nil && e.hover.keyboardShown {
		return true
	}
	return false
}

// consumeDeferredSync is the sync-point pump, called from the Entry's
// OnCursorChanged (which the fork fires from every content refresh, after
// every edit). It synchronises when an edit was flagged by noteDeferredEdit,
// when the caret changed rows (bounding the #130 anchor-tracking diff to a
// single row), or when the live line count changed — the O(1) Entry.LineCount
// comparison that lets mid-row deletions stay deferred (#149) while an edit
// that actually adds or removes a newline syncs immediately. SyncText is a
// cheap no-op when nothing is pending, and the bookkeeping is updated before
// syncing so the OnChanged pipeline re-entering this function cannot loop.
func (e *Editor) consumeDeferredSync() {
	if !e.DeferTextSync {
		return
	}
	row, lines := e.entry.CursorRow, e.entry.LineCount()
	if !e.deferredSyncPending && row == e.deferredLastRow && lines == e.deferredLastLines {
		return
	}
	e.deferredSyncPending = false
	e.deferredLastRow, e.deferredLastLines = row, lines
	e.entry.SyncText()
}
