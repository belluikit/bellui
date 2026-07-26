package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/widget"
)

// Tab / Shift-Tab block indent & dedent (#135), the block-editing companion to
// auto-indent-on-Enter (#134). Like #134, the mechanism lives in fycoder: Tab is
// intercepted before the Entry inserts a literal tab, and a possibly-multi-line
// edit is applied. It shares the indent unit ([Editor.TabWidth]/[Editor.UseSpaces])
// with auto-indent, so Enter-indent and Tab-indent agree, and it is governed by
// the same [Editor.AutoIndent] switch.
//
// A block edit is one Entry.ApplyEdits batch (fyne v2.14.0, fyne#433): one splice
// per touched line, applied atomically — a single undo step, one OnChanged, and
// per-splice BeforeChange gating with all-or-nothing veto. Each line's splice is
// gated exactly like a manual edit at that position: an insert at a protected
// range's start boundary is allowed (it shifts the span, #108), while a dedent
// that would delete protected characters vetoes the whole batch.

// handleTabIndent implements Tab (indent) and Shift-Tab (dedent). It returns true
// when it consumed the key. shift reports whether Shift is held.
//
// It runs from the onKey dispatcher after popovers, so an open completion popup
// accepts with Tab first. With AutoIndent off it declines the key, leaving the
// Entry's literal-tab (Tab) / no-op (Shift-Tab) behavior intact.
func (e *Editor) handleTabIndent(shift bool) bool {
	if !e.AutoIndent {
		return false
	}
	sel, hasSel := e.entry.SelectionRange()

	// Tab with no selection inserts one indent unit at the caret — a single-undo
	// pure insert, gated through BeforeChange like any edit.
	if !shift && !hasSel {
		e.insertTextAtCursor(e.indentUnit())
		return true
	}

	// Every other case is a whole-line operation over the touched row range.
	firstRow, lastRow := e.touchedRows(sel, hasSel)
	lines := strings.Split(e.docText(), "\n")
	if firstRow < 0 || lastRow >= len(lines) {
		return true
	}

	unit := e.indentUnit()
	width := e.tabWidthOrDefault()
	origCol := e.entry.CursorColumn

	// One splice per touched line, all at line-start offsets in the pre-edit
	// content (ApplyEdits applies them without observing each other's shift).
	var edits []widget.TextEdit
	lineStart := 0
	for i := 0; i < firstRow; i++ {
		lineStart += runeLen(lines[i]) + 1
	}
	firstRemoved := 0 // dedent runes removed from the caret's line (caret restore)
	for i := firstRow; i <= lastRow; i++ {
		line := lines[i]
		if shift {
			if n := runeLen(line) - runeLen(dedentLine(line, width)); n > 0 {
				edits = append(edits, widget.TextEdit{Start: lineStart, End: lineStart + n})
				if i == firstRow {
					firstRemoved = n
				}
			}
		} else if line != "" { // don't indent blank lines
			edits = append(edits, widget.TextEdit{Start: lineStart, End: lineStart, Text: unit})
		}
		lineStart += runeLen(line) + 1
	}
	if len(edits) == 0 {
		return true // e.g. Shift-Tab on lines already flush left
	}

	// Atomic batch: single undo step, one OnChanged; a protected line vetoes the
	// whole batch (no partial indent is ever observable).
	if !e.entry.ApplyEdits(edits) {
		return true
	}

	// Restore the selection (block op) or place the caret (single-line dedent);
	// ApplyEdits leaves the caret after the last splice with no selection.
	if hasSel {
		newLines := strings.Split(e.entry.Text, "\n")
		e.entry.Select(firstRow, 0, lastRow, runeLen(newLines[lastRow]))
	} else {
		newCol := origCol - firstRemoved
		if newCol < 0 {
			newCol = 0
		}
		e.SetCursorPos(firstRow, newCol)
	}
	return true
}

// touchedRows returns the inclusive row range a Tab/Shift-Tab edit acts on: the
// rows the selection touches when hasSel, else the caret's row. A selection that
// ends at column 0 of a line does not "touch" that line, so it is excluded.
func (e *Editor) touchedRows(sel widget.TextRange, hasSel bool) (firstRow, lastRow int) {
	if !hasSel {
		r := e.entry.CursorRow
		return r, r
	}
	runes := []rune(e.docText())
	start, end := sel.Start, sel.End
	if end > len(runes) {
		end = len(runes)
	}
	if start > end {
		start = end
	}
	firstRow = countNewlines(runes[:start])
	lastRow = countNewlines(runes[:end])
	// A selection ending exactly at a line start (the char before end is '\n')
	// leaves that line untouched.
	if end > start && end > 0 && runes[end-1] == '\n' {
		lastRow--
	}
	if lastRow < firstRow {
		lastRow = firstRow
	}
	return firstRow, lastRow
}

// dedentLine removes one indentation level from the start of line: a leading tab,
// or up to width leading spaces. A line already flush left is returned unchanged.
func dedentLine(line string, width int) string {
	if strings.HasPrefix(line, "\t") {
		return line[1:]
	}
	n := 0
	for n < len(line) && n < width && line[n] == ' ' {
		n++
	}
	return line[n:]
}

// tabWidthOrDefault returns TabWidth, falling back to the default when unset.
func (e *Editor) tabWidthOrDefault() int {
	if e.TabWidth <= 0 {
		return defaultTabWidth
	}
	return e.TabWidth
}

// runeLen returns the number of runes in s (Entry columns are rune indices).
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// countNewlines returns the number of '\n' runes in rs — i.e. the 0-based row of
// the position just past rs.
func countNewlines(rs []rune) int {
	n := 0
	for _, r := range rs {
		if r == '\n' {
			n++
		}
	}
	return n
}

// shiftHeld reports whether Shift is currently held, read from the driver's live
// modifier state (the KeyEvent carries no modifier). Mirrors the fork Entry's own
// Shift-Tab detection.
func (e *Editor) shiftHeld() bool {
	if dd, ok := bellui.CurrentApp().Driver().(desktop.Driver); ok {
		return dd.CurrentKeyModifiers()&event.KeyModifierShift != 0
	}
	return false
}
