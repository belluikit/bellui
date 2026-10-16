package editor

import (
	"strings"

	"github.com/belluikit/bellui/widget"
)

// Electric characters / reindent-on-type (pre-rebrand tracker #136) — "layer 2" of the indentation
// family: typing a character that changes the current line's meaning (a closing
// brace in C-like languages, the ':' of an "else:" clause in Python-like ones)
// re-indents that line immediately.
//
// The mechanism mirrors pre-rebrand tracker #134/#135: fycoder owns the hook (editorEntry.TypedRune
// → Editor.handleElectricRune, after the Entry has inserted the char), the
// language rule is the provider's. The rule is the optional [ReindentProvider]
// extension of [IndentProvider], so electric behavior is opt-in per provider:
// no provider — or one without ReindentOnType — leaves typing untouched.

// ReindentProvider is the optional extension of [IndentProvider] for electric
// characters. An IndentProvider that also implements it is consulted after each
// typed character; returning ok reindents the current line to the returned
// leading whitespace as a single gated edit.
//
// ReindentOnType is called on the UI goroutine for every typed rune and must
// return promptly — return ok=false fast for non-electric characters (the
// common case).
type ReindentProvider interface {
	// ReindentOnType returns the new leading whitespace for the caret's line
	// after typed was inserted, and whether to apply it. ctx describes the line
	// with typed already in place. Returning (indent, true) with indent equal to
	// the current leading whitespace is a no-op.
	ReindentOnType(ctx IndentContext, typed rune) (newIndent string, ok bool)
}

// handleElectricRune runs after the Entry inserts a typed rune (see
// editorEntry.TypedRune). It consults the provider's optional ReindentOnType and
// replaces the current line's leading whitespace as one atomic edit.
//
// The reindent is a distinct edit from the keystroke: gated through
// BeforeChange (protected/masked ranges veto it silently), one undo step —
// pressing undo restores the pre-reindent whitespace first, then a second undo
// removes the typed character. It is inert while a popover owns input, so it
// cannot fight the completion popup's anchoring.
func (e *Editor) handleElectricRune(r rune) {
	if !e.AutoIndent || len(e.activePopovers) > 0 {
		return
	}
	rp, ok := e.IndentProvider.(ReindentProvider)
	if !ok {
		return
	}

	// Synchronise deferred edits (pre-rebrand tracker #129): the just-typed rune must be in the
	// inspected line for the reindent decision.
	row, col := e.entry.CursorRow, e.entry.CursorColumn
	lines := strings.Split(e.docText(), "\n")
	if row < 0 || row >= len(lines) {
		return
	}
	line := lines[row]
	cur := leadingWhitespace(line)
	newIndent, ok := rp.ReindentOnType(IndentContext{
		Line:       line,
		Col:        col,
		PrevIndent: cur,
		Unit:       e.indentUnit(),
	}, r)
	if !ok || newIndent == cur {
		return
	}

	// Replace the line's leading whitespace as one gated splice. Offsets are
	// rune indices into the pre-edit content.
	lineStart := 0
	for i := 0; i < row; i++ {
		lineStart += runeLen(lines[i]) + 1
	}
	if !e.entry.ApplyEdits([]widget.TextEdit{{
		Start: lineStart,
		End:   lineStart + runeLen(cur),
		Text:  newIndent,
	}}) {
		return // vetoed (protected/masked) or no-op
	}

	// ApplyEdits parks the caret at the end of the replacement; restore it to
	// its logical position in the line, shifted by the indent delta.
	newCol := col + runeLen(newIndent) - runeLen(cur)
	if newCol < 0 {
		newCol = 0
	}
	e.SetCursorPos(row, newCol)
}
