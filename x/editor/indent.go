package editor

import (
	"strings"

	"github.com/belluikit/bellui/event"
)

// defaultTabWidth is the fallback indentation width (in spaces) when
// [Editor.TabWidth] is unset. NewEditor sets TabWidth to this.
const defaultTabWidth = 4

// IndentContext describes the caret's line at the moment Enter is pressed. It is
// passed to an [IndentProvider] so a language rule can decide the indentation of
// the new line.
type IndentContext struct {
	// Line is the full text of the line the caret is on (no trailing newline).
	Line string
	// Col is the caret column on that line (0-based rune index).
	Col int
	// PrevIndent is the leading whitespace of Line — the indentation the
	// language-agnostic default inherits onto the new line.
	PrevIndent string
	// Unit is one indentation level for this editor: [Editor.TabWidth] spaces
	// when [Editor.UseSpaces] is set, otherwise a single tab. A provider adds or
	// removes multiples of Unit so its output agrees with Tab/Shift-Tab.
	Unit string
}

// IndentProvider is the optional language rule for auto-indent-on-Enter — the
// policy half of the indentation axis, mirroring [Highlighter], CompletionProvider
// and LinkProvider. fycoder owns the mechanism (intercepting Enter and inserting
// "\n"+indent as a single edit); a provider only decides the whitespace.
//
// With no provider installed the editor inherits the current line's indentation
// (see [Editor.AutoIndent]). Install one with [Editor.IndentProvider]; the
// `starlark` sub-package ships [starlark.IndentProvider] as a template.
//
// IndentForNewLine is called on the UI goroutine during Enter handling and must
// return promptly.
type IndentProvider interface {
	// IndentForNewLine returns the whitespace to place at the start of the new
	// line created by Enter. Returning "" starts the new line at column 0.
	IndentForNewLine(ctx IndentContext) string
}

// indentUnit returns one indentation level for the editor: TabWidth spaces when
// UseSpaces is set, otherwise a single tab. TabWidth defaults to 4 when unset.
func (e *Editor) indentUnit() string {
	w := e.TabWidth
	if w <= 0 {
		w = defaultTabWidth
	}
	if e.UseSpaces {
		return strings.Repeat(" ", w)
	}
	return "\t"
}

// leadingWhitespace returns the run of spaces and tabs at the start of s.
func leadingWhitespace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

// handleEnterAutoIndent is the Enter-key intercept for auto-indent. It runs from
// the onKey dispatcher, after popovers, so an open completion popup accepts with
// Enter first. It returns true when it has consumed the key by inserting an
// indented newline; false to let the Entry insert a bare newline (the default).
//
// The indented newline is inserted as a single edit via insertTextAtCursor, so
// it composes with the rest of the editor for free: protected/masked ranges veto
// it through BeforeChange, undo removes it in one step, and OnChanged fires once.
func (e *Editor) handleEnterAutoIndent() bool {
	if !e.AutoIndent {
		return false
	}
	// A selection makes the post-erase insertion point ambiguous (it can span
	// lines); v1 defers to the Entry's default replace-then-newline behavior.
	if e.entry.SelectedText() != "" {
		return false
	}
	row, col := e.entry.CursorRow, e.entry.CursorColumn
	lines := strings.Split(e.docText(), "\n")
	if row < 0 || row >= len(lines) {
		return false
	}
	line := lines[row]
	newIndent := leadingWhitespace(line)
	if e.IndentProvider != nil {
		newIndent = e.IndentProvider.IndentForNewLine(IndentContext{
			Line:       line,
			Col:        col,
			PrevIndent: newIndent,
			Unit:       e.indentUnit(),
		})
	}
	if newIndent == "" {
		// A bare newline is byte-identical to the Entry's default Enter, so let
		// it handle the key rather than routing an empty insert through our path.
		return false
	}
	e.insertTextAtCursor("\n" + newIndent)
	return true
}

// insertTextAtCursor inserts s at the caret (replacing any active selection) as a
// single atomic edit: one BeforeChange gate, one undo step, and one OnChanged.
// Entry.InsertText (fyne v2.14.0, fyne#432) is exactly that primitive; this
// wrapper survives as the editor's single seam for caret inserts.
func (e *Editor) insertTextAtCursor(s string) {
	e.entry.InsertText(s)
}

// isEnterKey reports whether the key event is a newline-inserting Return/Enter.
func isEnterKey(ev *event.KeyEvent) bool {
	return ev.Name == event.KeyReturn || ev.Name == event.KeyEnter
}
