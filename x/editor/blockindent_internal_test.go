package editor

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

func TestTabIndent_NoSelectionInsertsUnitAtCaret(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetCursorPos(0, 3)

	e.handleTabIndent(false)

	if got, want := e.Text(), "foo    bar"; got != want {
		t.Fatalf("Tab text = %q, want %q", got, want)
	}
	if _, c := e.CursorPos(); c != 7 {
		t.Errorf("caret col = %d, want 7 (after the 4-space unit)", c)
	}
}

func TestTabIndent_NoSelectionUsesTabWhenNotSpaces(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foo")
	e.UseSpaces = false
	e.SetCursorPos(0, 3)

	e.handleTabIndent(false)

	if got, want := e.Text(), "foo\t"; got != want {
		t.Fatalf("Tab (tabs mode) text = %q, want %q", got, want)
	}
}

func TestTabDedent_NoSelectionDedentsCurrentLine(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        foo") // 8 spaces
	e.SetCursorPos(0, 11)

	e.handleTabIndent(true) // Shift-Tab

	if got, want := e.Text(), "    foo"; got != want { // removed one 4-space level
		t.Fatalf("Shift-Tab text = %q, want %q", got, want)
	}
}

func TestTabDedent_NoOpWhenFlushLeft(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abc")
	e.SetCursorPos(0, 1)

	if !e.handleTabIndent(true) {
		t.Error("Shift-Tab should be consumed even when there is nothing to dedent")
	}
	if got := e.Text(); got != "abc" {
		t.Errorf("Shift-Tab on a flush-left line changed text to %q", got)
	}
}

func TestTabIndent_SelectionIndentsEachLine(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\nb\nc")
	e.entry.Select(0, 0, 2, 1) // through all three lines

	e.handleTabIndent(false)

	if got, want := e.Text(), "    a\n    b\n    c"; got != want {
		t.Fatalf("block indent text = %q, want %q", got, want)
	}
}

func TestTabDedent_SelectionDedentsEachLine(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    a\n    b\n    c")
	e.entry.Select(0, 0, 2, 5)

	e.handleTabIndent(true)

	if got, want := e.Text(), "a\nb\nc"; got != want {
		t.Fatalf("block dedent text = %q, want %q", got, want)
	}
}

func TestTabIndent_SkipsBlankLines(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\n\nb")
	e.entry.Select(0, 0, 2, 1)

	e.handleTabIndent(false)

	if got, want := e.Text(), "    a\n\n    b"; got != want {
		t.Fatalf("block indent (blank line) text = %q, want %q", got, want)
	}
}

func TestTabIndent_SelectionEndingAtColumnZeroExcludesLastLine(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\nb\nc")
	e.entry.Select(0, 0, 2, 0) // ends at the start of row 2 → row 2 untouched

	e.handleTabIndent(false)

	if got, want := e.Text(), "    a\n    b\nc"; got != want {
		t.Fatalf("text = %q, want %q (last line excluded)", got, want)
	}
}

func TestTabIndent_NoSelectionSingleUndo(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foo")
	e.SetCursorPos(0, 3)

	e.handleTabIndent(false)
	if e.Text() != "foo    " {
		t.Fatalf("precondition: %q", e.Text())
	}
	e.entry.Undo()
	if got := e.Text(); got != "foo" {
		t.Errorf("after one undo = %q, want %q (no-selection Tab is one edit)", got, "foo")
	}
}

func TestTabIndent_OnChangedFiresOnce(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\nb\nc")
	e.entry.Select(0, 0, 2, 1)

	calls := 0
	e.OnChanged = func(string) { calls++ }

	e.handleTabIndent(false)

	if calls != 1 {
		t.Errorf("block indent fired OnChanged %d times, want 1", calls)
	}
}

func TestTabDedent_ProtectedRangeVetoesWholeBatch(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    aaa\n    bbb\n    ccc")
	// Protect the middle line including its leading whitespace: the dedent's
	// deletion overlaps the protected interior, so ApplyEdits vetoes the whole
	// batch — no line dedents (all-or-nothing, never a partial block).
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 7}})
	e.entry.Select(0, 0, 2, 7)

	calls := 0
	e.OnChanged = func(string) { calls++ }

	e.handleTabIndent(true)

	if got, want := e.Text(), "    aaa\n    bbb\n    ccc"; got != want {
		t.Errorf("protected block dedent changed text to %q, want unchanged", got)
	}
	if calls != 0 {
		t.Errorf("a vetoed block dedent must not fire OnChanged, got %d", calls)
	}
}

// TestTabIndent_ProtectedBoundaryInsertAllowed documents that block indent
// composes with plain protected ranges exactly like a manual edit at each line
// start: an insert at a protected range's start boundary is allowed (#108's
// edge-append rule) and shifts the span — where the old whole-region replace
// over-vetoed any block containing a protected character.
func TestTabIndent_ProtectedBoundaryInsertAllowed(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "aaa\nbbb\nccc")
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 3}})
	e.entry.Select(0, 0, 2, 3)

	e.handleTabIndent(false)

	if got, want := e.Text(), "    aaa\n    bbb\n    ccc"; got != want {
		t.Errorf("boundary-insert block indent = %q, want %q (allowed, like manual typing)", got, want)
	}
}

// TestTabIndent_MultiLineSingleUndo is the fyne v2.14.0 (ApplyEdits) headline:
// a multi-line block indent is ONE undo step, not one per line or an
// erase+reinsert pair.
func TestTabIndent_MultiLineSingleUndo(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\nb\nc")
	e.entry.Select(0, 0, 2, 1)

	e.handleTabIndent(false)
	if e.Text() != "    a\n    b\n    c" {
		t.Fatalf("precondition: %q", e.Text())
	}

	e.entry.Undo()
	if got, want := e.Text(), "a\nb\nc"; got != want {
		t.Errorf("after ONE undo = %q, want %q (block indent is a single undo step)", got, want)
	}
}

// TestTabDedent_MultiLineSingleUndo covers the dedent (pure-deletion) batch.
func TestTabDedent_MultiLineSingleUndo(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    a\n    b\n    c")
	e.entry.Select(0, 0, 2, 5)

	e.handleTabIndent(true)
	if e.Text() != "a\nb\nc" {
		t.Fatalf("precondition: %q", e.Text())
	}

	e.entry.Undo()
	if got, want := e.Text(), "    a\n    b\n    c"; got != want {
		t.Errorf("after ONE undo = %q, want %q (block dedent is a single undo step)", got, want)
	}
}

func TestTabIndent_DisabledFallsThroughToLiteralTab(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foo")
	e.AutoIndent = false
	e.SetCursorPos(0, 3)

	// Drive the real key path: with AutoIndent off, onKey declines and the Entry
	// inserts a literal tab.
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyTab})

	if got, want := e.Text(), "foo\t"; got != want {
		t.Fatalf("AutoIndent-off Tab = %q, want a literal tab %q", got, want)
	}
}

func TestDedentLine(t *testing.T) {
	tests := []struct {
		line, want string
		width      int
	}{
		{"        x", "    x", 4}, // 8 spaces → 4
		{"  x", "x", 4},           // fewer than width → all removed
		{"\tx", "x", 4},           // one leading tab
		{"x", "x", 4},             // flush left, unchanged
		{"", "", 4},               // empty, unchanged
	}
	for _, tc := range tests {
		if got := dedentLine(tc.line, tc.width); got != tc.want {
			t.Errorf("dedentLine(%q, %d) = %q, want %q", tc.line, tc.width, got, tc.want)
		}
	}
}
