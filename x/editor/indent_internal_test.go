package editor

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

// pressEnter drives a Return key through the real editor key path (onKey →
// auto-indent), matching how the fork driver delivers Enter.
func pressEnter(e *Editor) {
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
}

// fixedIndentProvider is a test IndentProvider that always returns a fixed
// string, proving a provider fully controls the new-line indentation.
type fixedIndentProvider struct{ indent string }

func (p fixedIndentProvider) IndentForNewLine(IndentContext) string { return p.indent }

// levelIndentProvider inherits the current indent and adds one unit, exercising
// IndentContext.Unit (the shared TabWidth/UseSpaces indent unit).
type levelIndentProvider struct{}

func (levelIndentProvider) IndentForNewLine(ctx IndentContext) string {
	return ctx.PrevIndent + ctx.Unit
}

func TestAutoIndent_InheritsCurrentLineIndent(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    foo")
	e.SetCursorPos(0, 7) // end of "    foo"

	pressEnter(e)

	if got, want := e.Text(), "    foo\n    "; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if r, c := e.CursorPos(); r != 1 || c != 4 {
		t.Errorf("cursor = (%d,%d), want (1,4) after the inherited indent", r, c)
	}
}

func TestAutoIndent_CaretMidLineSplitsWithIndent(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    foobar")
	e.SetCursorPos(0, 7) // between "foo" and "bar"

	pressEnter(e)

	if got, want := e.Text(), "    foo\n    bar"; got != want {
		t.Fatalf("mid-line split text = %q, want %q", got, want)
	}
}

func TestAutoIndent_ColumnZeroLineFallsThrough(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foo")
	e.SetCursorPos(0, 3)

	pressEnter(e)

	// No leading whitespace to inherit → a bare newline, caret at column 0.
	if got, want := e.Text(), "foo\n"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if r, c := e.CursorPos(); r != 1 || c != 0 {
		t.Errorf("cursor = (%d,%d), want (1,0)", r, c)
	}
}

func TestAutoIndent_DisabledInsertsBareNewline(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    foo")
	e.AutoIndent = false
	e.SetCursorPos(0, 7)

	pressEnter(e)

	if got, want := e.Text(), "    foo\n"; got != want {
		t.Fatalf("disabled text = %q, want %q (no auto-indent)", got, want)
	}
}

func TestAutoIndent_SingleUndoRemovesNewlineAndIndent(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    foo")
	e.SetCursorPos(0, 7)

	pressEnter(e)
	if e.Text() != "    foo\n    " {
		t.Fatalf("precondition failed: %q", e.Text())
	}

	e.entry.Undo()
	if got, want := e.Text(), "    foo"; got != want {
		t.Errorf("after one undo text = %q, want %q (newline+indent is one edit)", got, want)
	}
}

func TestAutoIndent_OnChangedFiresOnce(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    foo")
	e.SetCursorPos(0, 7)

	calls := 0
	e.OnChanged = func(string) { calls++ }

	pressEnter(e)

	if calls != 1 {
		t.Errorf("OnChanged fired %d times for one Enter, want 1", calls)
	}
}

func TestAutoIndent_ProtectedRangeVetoes(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    secret")
	// Protect the whole line; a caret strictly inside the range makes Enter's
	// insert an interior edit, which BeforeChange must veto.
	e.SetProtectedRanges([]TextRange{{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 10}})
	e.SetCursorPos(0, 5)

	calls := 0
	e.OnChanged = func(string) { calls++ }

	pressEnter(e)

	if got, want := e.Text(), "    secret"; got != want {
		t.Errorf("protected Enter changed text to %q, want unchanged %q", got, want)
	}
	if calls != 0 {
		t.Errorf("a vetoed Enter must not fire OnChanged, got %d", calls)
	}
}

func TestAutoIndent_ProviderControlsIndent(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "  foo")
	e.IndentProvider = fixedIndentProvider{indent: "\t\t"}
	e.SetCursorPos(0, 5)

	pressEnter(e)

	if got, want := e.Text(), "  foo\n\t\t"; got != want {
		t.Fatalf("provider indent text = %q, want %q", got, want)
	}
}

func TestAutoIndent_ProviderReadsIndentUnit(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "  foo")
	e.IndentProvider = levelIndentProvider{}
	e.TabWidth = 2
	e.UseSpaces = true
	e.SetCursorPos(0, 5)

	pressEnter(e)

	// Inherit "  " plus one 2-space unit → 4 spaces.
	if got, want := e.Text(), "  foo\n    "; got != want {
		t.Fatalf("unit-aware indent text = %q, want %q", got, want)
	}
}

func TestAutoIndent_SelectionFallsThrough(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    foobar")
	e.SetCursorPos(0, 4)
	// Select "foo" (cols 4..7); Enter should replace it with a bare newline.
	e.entry.Select(0, 4, 0, 7)

	pressEnter(e)

	if got, want := e.Text(), "    \nbar"; got != want {
		t.Fatalf("selection Enter text = %q, want %q (default replace-then-newline)", got, want)
	}
}

func TestIndentUnit_SpacesVsTabs(t *testing.T) {
	e := &Editor{TabWidth: 3, UseSpaces: true}
	if got := e.indentUnit(); got != "   " {
		t.Errorf("spaces unit = %q, want 3 spaces", got)
	}
	e.UseSpaces = false
	if got := e.indentUnit(); got != "\t" {
		t.Errorf("tab unit = %q, want a tab", got)
	}
	e.UseSpaces = true
	e.TabWidth = 0
	if got := e.indentUnit(); got != "    " {
		t.Errorf("zero TabWidth should fall back to %d spaces, got %q", defaultTabWidth, got)
	}
}
