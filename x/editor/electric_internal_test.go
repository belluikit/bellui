package editor

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui/test"
)

// colonDedenter is a test IndentProvider+ReindentProvider: Enter inherits the
// indent; typing ':' on a line whose code is "x:" dedents one unit.
type colonDedenter struct{}

func (colonDedenter) IndentForNewLine(ctx IndentContext) string { return ctx.PrevIndent }

func (colonDedenter) ReindentOnType(ctx IndentContext, typed rune) (string, bool) {
	if typed != ':' || !strings.HasSuffix(strings.TrimSpace(ctx.Line), "x:") {
		return "", false
	}
	return strings.TrimSuffix(ctx.PrevIndent, ctx.Unit), true
}

// sameIndenter reports ok with the unchanged indent — must be a pure no-op.
type sameIndenter struct{ colonDedenter }

func (sameIndenter) ReindentOnType(ctx IndentContext, _ rune) (string, bool) {
	return ctx.PrevIndent, true
}

func TestElectric_ReindentsOnTrigger(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x") // two 4-space levels
	e.IndentProvider = colonDedenter{}
	e.SetCursorPos(0, 9)

	e.entry.TypedRune(':') // completes "x:" → dedent one level

	if got, want := e.Text(), "    x:"; got != want {
		t.Fatalf("electric reindent text = %q, want %q", got, want)
	}
	// Caret keeps its logical spot in the line, shifted by the removed indent.
	if r, c := e.CursorPos(); r != 0 || c != 6 {
		t.Errorf("caret = (%d,%d), want (0,6) after the dedent", r, c)
	}
}

func TestElectric_NonTriggerRuneUntouched(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x")
	e.IndentProvider = colonDedenter{}
	e.SetCursorPos(0, 9)

	e.entry.TypedRune('y') // not electric

	if got, want := e.Text(), "        xy"; got != want {
		t.Fatalf("non-trigger rune text = %q, want %q", got, want)
	}
}

func TestElectric_InertWithoutReindentProvider(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x")
	// A plain IndentProvider without the optional extension: never electric.
	e.IndentProvider = fixedIndentProvider{indent: "  "}
	e.SetCursorPos(0, 9)

	e.entry.TypedRune(':')

	if got, want := e.Text(), "        x:"; got != want {
		t.Fatalf("plain-provider text = %q, want %q", got, want)
	}
}

func TestElectric_InertWhenAutoIndentOff(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x")
	e.IndentProvider = colonDedenter{}
	e.AutoIndent = false
	e.SetCursorPos(0, 9)

	e.entry.TypedRune(':')

	if got, want := e.Text(), "        x:"; got != want {
		t.Fatalf("AutoIndent-off text = %q, want %q", got, want)
	}
}

func TestElectric_InertWhilePopoverOpen(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x")
	e.IndentProvider = colonDedenter{}
	e.SetCursorPos(0, 9)
	e.activePopovers = []*popover{newTestPopover()} // a popover owns input

	e.entry.TypedRune(':')

	if got, want := e.Text(), "        x:"; got != want {
		t.Fatalf("popover-open text = %q, want %q (reindent must be inert)", got, want)
	}
}

func TestElectric_SameIndentIsNoOp(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "    x")
	e.IndentProvider = sameIndenter{}
	e.SetCursorPos(0, 5)

	e.entry.TypedRune(':')

	if got, want := e.Text(), "    x:"; got != want {
		t.Fatalf("same-indent text = %q, want %q", got, want)
	}
	// The no-op reindent must not add an undo step: one undo removes the char.
	e.entry.Undo()
	if got, want := e.Text(), "    x"; got != want {
		t.Errorf("after one undo = %q, want %q (no phantom reindent undo step)", got, want)
	}
}

func TestElectric_UndoIsDistinctStep(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x")
	e.IndentProvider = colonDedenter{}
	e.SetCursorPos(0, 9)

	e.entry.TypedRune(':')
	if e.Text() != "    x:" {
		t.Fatalf("precondition: %q", e.Text())
	}

	e.entry.Undo() // first undo: restore the pre-reindent whitespace
	if got, want := e.Text(), "        x:"; got != want {
		t.Fatalf("after undo #1 = %q, want %q (reindent undone, char kept)", got, want)
	}
	e.entry.Undo() // second undo: remove the typed character
	if got, want := e.Text(), "        x"; got != want {
		t.Errorf("after undo #2 = %q, want %q (keystroke undone)", got, want)
	}
}

func TestElectric_ProtectedIndentVetoes(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "        x")
	e.IndentProvider = colonDedenter{}
	// Protect the leading whitespace: the reindent's deletion overlaps it and is
	// vetoed silently; the typed character stays.
	e.SetProtectedRanges([]TextRange{{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 8}})
	e.SetCursorPos(0, 9)

	e.entry.TypedRune(':')

	if got, want := e.Text(), "        x:"; got != want {
		t.Errorf("protected reindent text = %q, want %q (veto keeps the char)", got, want)
	}
}
