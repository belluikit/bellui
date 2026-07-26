package widget

import (
	"strings"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/lang"
)

// objectReplacementRune stands in for a non-text atom (e.g. an image) in the
// [RichEntry.AccessibleText] rune stream so accessible offsets align with the
// document's global position space (segPositions: one position per rune, one per
// atom). U+FFFC OBJECT REPLACEMENT CHARACTER is the conventional placeholder.
const objectReplacementRune = '￼'

// RichEntry implements [core.AccessibleTexter] (#659/#773), the editor contract
// of the neutral text-input mirror seam (#768). This is what binds RichEntry to
// the mobile IME / accessibility text bridge: with it, composition, plain
// typing, selection and replace-selection all flow through the same seam as
// widget.Entry (e.g. the iOS GoInputView document mirror, #771). Without it
// Session.Bind unbinds on RichEntry focus and every mirror op is inert.
var (
	_ core.Accessible         = (*RichEntry)(nil)
	_ core.AccessibleValuer   = (*RichEntry)(nil)
	_ core.AccessibleTexter   = (*RichEntry)(nil)
	_ core.AccessibleActioner = (*RichEntry)(nil)
)

// AccessibleRole reports the rich editor as a multi-line text input — the
// semantic-role half of #659, deferred when the AccessibleTexter seam shipped
// and delivered during the #951 device pass: without a text role the editor
// publishes no accessibility element at all, so a screen reader cannot focus
// it and the mobile input-view swap (#570/#1124) never engages — leaving
// VoiceOver caret navigation with no editing context. The role also makes the
// editor one semantic unit (NonInteractiveChildrenPresentational prunes the
// internal render widgets that paint its value).
func (e *RichEntry) AccessibleRole() core.AccessibleRole {
	return core.RoleMultiLineTextBox
}

// AccessibleLabel returns no intrinsic name; hosts name the editor via
// [BaseWidget.SetAccessibilityLabel].
func (e *RichEntry) AccessibleLabel() string { return "" }

// AccessibleValue returns the document's accessible rune stream (atoms as
// U+FFFC), matching the offsets the editable-text protocols use.
func (e *RichEntry) AccessibleValue() string { return e.AccessibleText() }

// AccessibleRange is not applicable to text editors; it returns zeros.
func (e *RichEntry) AccessibleRange() (min, max, current float64) { return 0, 0, 0 }

// AccessibleActions declares the editor's undo/redo actions (#1025), each
// gated on its history being non-empty — the same availability rule as
// [Entry.AccessibleActions] and the TappedSecondary context menu. The
// backends surface them data-driven (VoiceOver's actions rotor, TalkBack's
// actions menu, AT-SPI Action) now that the editor carries its semantic role.
func (e *RichEntry) AccessibleActions() []core.AccessibleAction {
	var acts []core.AccessibleAction
	if e.undo.canUndo() {
		acts = append(acts, core.AccessibleAction{Name: core.ActionUndo, Description: lang.L("Undo")})
	}
	if e.undo.canRedo() {
		acts = append(acts, core.AccessibleAction{Name: core.ActionRedo, Description: lang.L("Redo")})
	}
	return acts
}

// PerformAccessibleAction invokes the named accessible action.
func (e *RichEntry) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionUndo, core.ActionRedo:
		// Cancel an in-flight IME composition before replaying history —
		// the preedit is display-only and its position context would be
		// invalidated by the undo (#1034, same rule as Entry).
		e.ComposeCancel()
		if name == core.ActionUndo {
			e.Undo()
		} else {
			e.Redo()
		}
	}
}

// AccessibleText returns the document as a rune stream in its global position
// space: text runes verbatim and each non-text atom as one U+FFFC object
// replacement character, so a rune offset into the result is a caret offset
// (matching [RichEntry.AccessibleSelection]).
//
// The result is memoized and invalidated only on committed-content changes
// (afterEditNotify — edits, undo/redo, paste, image insert — and SetSegments),
// not on the broad syncProvider that also runs on every repaint, IME preedit and
// restyle: those leave this rune stream unchanged, so dropping the memo there
// would rebuild the whole string per query for no reason — the exact large-doc
// thrash the desktop a11y-text bridges hit while a screen reader tracks the caret
// during CJK composition (#794). The mobile text-input mirror's per-paint-frame
// sync (#783) thus reuses the same string object until the next real edit, so the
// seam's redundant-push filter stays O(1). Main-goroutine-owned, like the rest of
// the widget's state.
func (e *RichEntry) AccessibleText() string {
	if e.axTextValid {
		return e.axText
	}
	var b strings.Builder
	for _, seg := range e.doc.segments() {
		if ts, ok := seg.(*TextSegment); ok {
			b.WriteString(ts.Text)
		} else {
			b.WriteRune(objectReplacementRune)
		}
	}
	e.axText, e.axTextValid = b.String(), true
	return e.axText
}

// AccessibleComposing reports whether an IME composition is currently active,
// for the text-input mirror seam's stale-finalize detection (#1057) — see the
// [Entry.AccessibleComposing] doc.
func (e *RichEntry) AccessibleComposing() bool {
	return e.composing
}

// AccessibleSelection returns the selection as a half-open [start, end) rune
// range; with no selection both offsets are the caret position.
func (e *RichEntry) AccessibleSelection() (start, end int) {
	if from, to, ok := e.selection(); ok {
		return from, to
	}
	return e.caret, e.caret
}

// AccessibleCaret returns the rune offset of the insertion point.
func (e *RichEntry) AccessibleCaret() int {
	return e.caret
}

// SetAccessibleSelection selects [start, end), clamped to the document; equal
// offsets place the caret there with no selection. Safe to call from any
// goroutine.
func (e *RichEntry) SetAccessibleSelection(start, end int) {
	onMain(func() {
		n := e.doc.len()
		start = clampInt(start, 0, n)
		end = clampInt(end, 0, n)
		if start > end {
			start, end = end, start
		}
		e.anchor = start
		e.caret = end
		e.selecting = start != end
		e.interruptCursor()
		e.Refresh()
	})
}

// ReplaceAccessibleSelection replaces the current selection — or inserts at the
// caret — with text, as one gated, undo-recorded edit (one OnChanged). An empty
// text deletes the selection. Safe to call from any goroutine.
func (e *RichEntry) ReplaceAccessibleSelection(text string) {
	onMain(func() {
		if text == "" {
			e.deleteSelection()
			return
		}
		e.insertRunes([]rune(text))
	})
}
