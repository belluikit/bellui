package widget_test

import (
	"runtime"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// "Hello World!" rune indices:
//
//	H0 e1 l2 l3 o4 ' '5 W6 o7 r8 l9 d10 !11   (len 12)
//
// "World" occupies the half-open rune range [6, 11).
const protSample = "Hello World!"

func protectWorld(e *widget.Entry, caretOpaque bool) {
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 6, End: 11, CaretOpaque: caretOpaque}}
}

// --- BeforeChange hook ---------------------------------------------------

func TestEntry_BeforeChange_Veto(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	e.CursorColumn = 2

	allow := false
	e.BeforeChange = func(widget.TextRange, string) bool { return allow }

	e.TypedRune('X')
	assert.Equal(t, protSample, e.Text, "veto must leave Text untouched")

	allow = true
	e.TypedRune('X')
	assert.Equal(t, "HeXllo World!", e.Text, "an allowed edit proceeds normally")
}

func TestEntry_BeforeChange_ReceivesRangeAndNext(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)

	var gotSel widget.TextRange
	var gotNext string
	e.BeforeChange = func(sel widget.TextRange, next string) bool {
		gotSel, gotNext = sel, next
		return true
	}

	// Caret insertion: empty range at the caret.
	e.CursorColumn = 3
	e.TypedRune('Z')
	assert.Equal(t, widget.TextRange{Start: 3, End: 3}, gotSel)
	assert.Equal(t, "Z", gotNext)

	// Selection replacement: the full selected range and the replacement.
	e.SetText(protSample)
	e.Select(0, 6, 0, 11) // select "World"
	e.TypedRune('Q')
	assert.Equal(t, widget.TextRange{Start: 6, End: 11}, gotSel)
	assert.Equal(t, "Q", gotNext)
}

// --- ProtectedRanges reject every mutation path --------------------------

func TestEntry_Protected_Type(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 8 // inside "World"
	e.TypedRune('X')
	assert.Equal(t, protSample, e.Text, "typing inside a protected span is rejected")
}

func TestEntry_Protected_BoundaryInsertAllowed(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 6 // exactly at the start boundary of "World"
	e.TypedRune('X')
	assert.Equal(t, "Hello XWorld!", e.Text, "insertion at the leading boundary is allowed")

	e.SetText(protSample)
	protectWorld(e, false)
	e.CursorColumn = 11 // exactly at the trailing boundary
	e.TypedRune('Y')
	assert.Equal(t, "Hello WorldY!", e.Text, "insertion at the trailing boundary is allowed")
}

func TestEntry_Protected_Backspace(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 7 // backspace would delete 'W' (offset [6,7))
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, protSample, e.Text, "backspace into a protected span is rejected")

	// Backspace just before the span deletes the space and is allowed.
	e.CursorColumn = 6
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, "HelloWorld!", e.Text)
}

func TestEntry_Protected_Delete(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 6 // delete would remove 'W' (offset [6,7))
	e.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	assert.Equal(t, protSample, e.Text, "delete into a protected span is rejected")

	// Delete just before the span removes the space and is allowed.
	e.CursorColumn = 5
	e.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	assert.Equal(t, "HelloWorld!", e.Text)
}

func TestEntry_Protected_DeleteWord(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	mod := event.KeyModifierShortcutDefault
	if runtime.GOOS == "darwin" {
		mod = event.KeyModifierAlt
	}
	e.CursorColumn = 11 // word-delete would erase the word "World"
	e.TypedShortcut(&desktop.CustomShortcut{Modifier: mod, KeyName: event.KeyBackspace})
	assert.Equal(t, protSample, e.Text, "word-delete overlapping a protected span is rejected")
}

func TestEntry_Protected_Return(t *testing.T) {
	e := widget.NewEntry()
	e.MultiLine = true
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 8 // newline strictly inside the span
	e.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, protSample, e.Text, "inserting a newline inside a protected span is rejected")
}

func TestEntry_Protected_Paste(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	clip := test.NewClipboard()
	clip.SetContent("INJECT")

	e.CursorColumn = 8 // caret inside the span
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clip})
	assert.Equal(t, protSample, e.Text, "paste into a protected span is rejected")

	// Pasting over a selection that overlaps the span is also rejected.
	e.Select(0, 4, 0, 8)
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clip})
	assert.Equal(t, protSample, e.Text, "paste replacing a selection overlapping the span is rejected")
}

func TestEntry_Protected_Cut(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	clip := test.NewClipboard()
	e.Select(0, 6, 0, 11) // select "World"
	e.TypedShortcut(&bellui.ShortcutCut{Clipboard: clip})
	assert.Equal(t, protSample, e.Text, "cutting a protected selection must not delete it")
}

func TestEntry_Protected_SelectionDelete(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.Select(0, 4, 0, 9) // overlaps the protected span
	e.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	assert.Equal(t, protSample, e.Text, "deleting a selection overlapping the span is rejected")
}

func TestEntry_Protected_ProgrammaticSelectOverwrite(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	// find-replace style: select a region overlapping the span and type over it.
	e.Select(0, 0, 0, 7)
	e.TypedRune('!')
	assert.Equal(t, protSample, e.Text, "programmatic select+overwrite touching the span is rejected")
}

func TestEntry_Protected_IMECommit(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 8 // caret inside the span
	e.ComposeCommit("文")
	assert.Equal(t, protSample, e.Text, "IME commit inside a protected span is rejected")
}

func TestEntry_Protected_UnaffectedElsewhere(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false)

	e.CursorColumn = 2 // well outside the span
	e.TypedRune('X')
	assert.Equal(t, "HeXllo World!", e.Text, "an edit outside any protected span is unaffected")
}

func TestEntry_Protected_UndoDoesNotResurrect(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)

	// A legitimate edit first.
	e.CursorColumn = 0
	e.TypedRune('A') // "AHello World!"
	assert.Equal(t, "AHello World!", e.Text)

	// Now protect "World" (shifted by the inserted 'A') and attempt a vetoed edit.
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 7, End: 12}}
	e.CursorColumn = 9
	e.TypedRune('X') // rejected, never recorded
	assert.Equal(t, "AHello World!", e.Text)

	// Undo unwinds only the legitimate 'A'; the vetoed edit is not on the stack.
	e.Undo()
	assert.Equal(t, protSample, e.Text, "undo must not resurrect a vetoed edit")
}

// TestEntry_Protected_UndoRedoReplaysHistory pins policy (a) for #146: undo/redo
// replay recorded history verbatim and are NOT re-gated against ProtectedRanges,
// even when a range is added after the edit was recorded. Protection applies to
// new interactive edits, not history replay.
func TestEntry_Protected_UndoRedoReplaysHistory(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)

	// A legitimate edit recorded while no range exists.
	e.CursorColumn = 0
	e.TypedRune('A') // "AHello World!"
	assert.Equal(t, "AHello World!", e.Text)

	// Protect a span covering the just-inserted 'A' (offset 0). A new edit there
	// would now be vetoed, but history replay is not.
	e.ProtectedRanges = []widget.ProtectedRange{{Start: 0, End: 6}}

	e.Undo()
	assert.Equal(t, protSample, e.Text, "undo replays recorded history across a protected span")

	e.Redo()
	assert.Equal(t, "AHello World!", e.Text, "redo replays recorded history across a protected span")
}

// --- CaretOpaque bars caret entry ----------------------------------------

func TestEntry_CaretOpaque_ArrowRightSkips(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, true)

	e.CursorColumn = 6 // at the leading boundary
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 11, e.CursorColumn, "right arrow must jump past a caret-opaque span")
}

func TestEntry_CaretOpaque_ArrowLeftSkips(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, true)

	e.CursorColumn = 11 // at the trailing boundary
	e.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 6, e.CursorColumn, "left arrow must jump past a caret-opaque span")
}

func TestEntry_CaretOpaque_NonOpaqueAllowsTravel(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, false) // protected but caret-visible

	e.CursorColumn = 6
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 7, e.CursorColumn, "a non-opaque protected span still allows caret travel")

	// ...but edits there are still rejected.
	e.TypedRune('X')
	assert.Equal(t, protSample, e.Text)
}

func TestEntry_CaretOpaque_ClickSnapsToBoundary(t *testing.T) {
	test.NewTempApp(t)
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, true)

	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(300, 80))

	// Click at the horizontal centre of a glyph well inside "World".
	r := e.CaretRect(0, 8)
	e.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: r.Pos}})

	assert.False(t, e.CursorColumn > 6 && e.CursorColumn < 11,
		"a click inside a caret-opaque span must snap to a boundary, got col %d", e.CursorColumn)
}

// TestEntry_CaretOpaque_TouchSnapsToBoundary covers #147: the mobile tap-to-
// position path (TouchDown -> updateMousePointer) enforces CaretOpaque just like
// a desktop click. A tap strictly inside a caret-opaque span snaps to a boundary.
func TestEntry_CaretOpaque_TouchSnapsToBoundary(t *testing.T) {
	test.NewTempApp(t)
	e := widget.NewEntry()
	e.SetText(protSample)
	protectWorld(e, true)

	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(300, 80))

	// Tap at the horizontal centre of a glyph well inside "World".
	r := e.CaretRect(0, 8)
	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: r.Pos}})

	assert.False(t, e.CursorColumn > 6 && e.CursorColumn < 11,
		"a tap inside a caret-opaque span must snap to a boundary, got col %d", e.CursorColumn)
}

// --- Zero regression when nothing is set ---------------------------------

func TestEntry_NoProtection_Unchanged(t *testing.T) {
	e := widget.NewEntry()
	e.SetText(protSample)

	e.CursorColumn = 8
	e.TypedRune('X')
	assert.Equal(t, "Hello WoXrld!", e.Text, "with no hook or ranges, editing is unchanged")
}
