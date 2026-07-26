package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestEntry_Compose_PreeditDoesNotMutate verifies that an in-progress IME
// composition shows inline without touching Text, OnChanged, or undo.
func TestEntry_Compose_PreeditDoesNotMutate(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	changed := 0
	entry.OnChanged = func(string) { changed++ }
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})

	assert.Equal(t, "", entry.Text, "preedit must not appear in Text")
	assert.Zero(t, changed, "OnChanged must not fire during composition")
	// Caret sits at the end of the preedit (anchor 0 + cursor 2).
	assert.Equal(t, 2, entry.CursorColumn)

	// Undo has nothing to undo — Text stays empty.
	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)
}

// TestEntry_Compose_Replace verifies a later Compose replaces the earlier
// preedit rather than accumulating.
func TestEntry_Compose_Replace(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "n", CursorPos: 1})
	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	entry.Compose(&event.PreeditEvent{Text: "nih", CursorPos: 3})

	assert.Equal(t, "", entry.Text)
	assert.Equal(t, 3, entry.CursorColumn)
}

// TestEntry_ComposeCommit verifies committing inserts as a single edit that
// fires OnChanged once and is undone in one step.
func TestEntry_ComposeCommit(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	changed := 0
	var last string
	entry.OnChanged = func(s string) {
		changed++
		last = s
	}
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	entry.ComposeCommit("你好")

	assert.Equal(t, "你好", entry.Text)
	assert.Equal(t, 1, changed, "commit fires OnChanged exactly once")
	assert.Equal(t, "你好", last)
	assert.Equal(t, 2, entry.CursorColumn, "caret after committed text")

	// A single undo removes the whole committed string.
	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)
}

// TestEntry_ComposeCommit_MidText commits at the caret within existing text.
func TestEntry_ComposeCommit_MidText(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.SetText("ab")
	entry.FocusGained()
	entry.Select(0, 1, 0, 1) // collapsed caret between a and b (clears select-all)

	entry.Compose(&event.PreeditEvent{Text: "X", CursorPos: 1})
	assert.Equal(t, "ab", entry.Text, "preedit not committed yet")

	entry.ComposeCommit("你")
	assert.Equal(t, "a你b", entry.Text)
	assert.Equal(t, 2, entry.CursorColumn)
}

// TestEntry_ComposeCancel discards the composition and restores the caret.
func TestEntry_ComposeCancel(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.SetText("ab")
	entry.FocusGained()
	entry.Select(0, 1, 0, 1) // collapsed caret between a and b

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	entry.ComposeCancel()

	assert.Equal(t, "ab", entry.Text)
	assert.Equal(t, 1, entry.CursorColumn, "caret restored to composition anchor")
}

// TestEntry_ComposeCommitEmpty behaves like cancel.
func TestEntry_ComposeCommitEmpty(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.SetText("ab")
	entry.FocusGained()
	entry.Select(0, 1, 0, 1) // collapsed caret between a and b

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	entry.ComposeCommit("")

	assert.Equal(t, "ab", entry.Text)
	assert.Equal(t, 1, entry.CursorColumn)
}

// TestEntry_ComposeCommit_PreservesExternalCaret verifies a bare caret placed
// OUTSIDE the preedit after composition started (an external mid-composition
// caret move — on iOS a tap finalizes the composition through this path, #1118)
// survives the commit: the committed text lands at the anchor and the caret
// stays where the user put it, remapped for the preedit→committed swap.
func TestEntry_ComposeCommit_PreservesExternalCaret(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewMultiLineEntry()
	entry.SetText("ab\ncd")
	entry.FocusGained()
	entry.Select(0, 1, 0, 1) // collapsed caret between a and b

	entry.Compose(&event.PreeditEvent{Text: "nih", CursorPos: 3})
	// External caret placement on line 2, after "c" — display coordinates
	// include the preedit, so "anihb\ncd" puts that at offset 7.
	entry.SetAccessibleSelection(7, 7)
	entry.ComposeCommit("nih")

	assert.Equal(t, "anihb\ncd", entry.Text, "committed at the anchor")
	assert.Equal(t, 1, entry.CursorRow, "caret kept on the tapped line")
	assert.Equal(t, 1, entry.CursorColumn, "caret kept at the tapped column")
}

// TestEntry_Compose_ReplacesSelection verifies composing while text is selected
// erases the selection first (matching typing behaviour).
func TestEntry_Compose_ReplacesSelection(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.SetText("hello")
	entry.FocusGained()
	entry.Select(0, 0, 0, 5) // select all

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	assert.Equal(t, "", entry.Text, "selection erased on compose start")

	entry.ComposeCommit("你")
	assert.Equal(t, "你", entry.Text)
}

// TestEntry_Compose_ReadOnly is a no-op on read-only entries.
func TestEntry_Compose_ReadOnly(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.SetText("ab")
	entry.ReadOnly = true
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	entry.ComposeCommit("你")
	assert.Equal(t, "ab", entry.Text, "read-only entry rejects composition")
}

// TestEntry_Compose_ViaCanvas exercises the focused-object path used by drivers
// through the test helpers.
func TestEntry_Compose_ViaCanvas(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	w := test.NewTempWindow(t, entry)
	c := w.Canvas()
	c.Focus(entry)

	test.Compose(c, "ni", 2)
	assert.Equal(t, "", entry.Text)

	test.ComposeCommit(c, "你好")
	assert.Equal(t, "你好", entry.Text)

	test.Compose(c, "x", 1)
	test.ComposeCancel(c)
	assert.Equal(t, "你好", entry.Text)
}

// TestEntry_Compose_MultiLine places preedit on a non-first row.
func TestEntry_Compose_MultiLine(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewMultiLineEntry()
	entry.SetText("line1\nline2")
	entry.FocusGained()
	entry.CursorRow = 1
	entry.CursorColumn = 2 // after "li" on second line

	entry.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	assert.Equal(t, "line1\nline2", entry.Text)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 4, entry.CursorColumn, "caret advanced by preedit length on the row")

	entry.ComposeCommit("你")
	assert.Equal(t, "line1\nli你ne2", entry.Text)
}

// TestEntry_DesktopCommitReplacesPreedit covers the desktop IME commit path
// (#354): the finalized text arrives as char events (TypedRune) while a
// composition is active, and must REPLACE the inline preedit, not be appended
// next to it (which produced duplication like あ -> ああ).
func TestEntry_DesktopCommitReplacesPreedit(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	changed := 0
	entry.OnChanged = func(string) { changed++ }
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})
	entry.TypedRune('あ') // IME commits the converted char as a char event

	assert.Equal(t, "あ", entry.Text, "committed text must replace the preedit, not append (#354)")
	assert.Equal(t, 1, changed, "commit fires OnChanged exactly once")
	assert.Equal(t, 1, entry.CursorColumn)

	// Composition is finished: a following plain rune appends normally.
	entry.TypedRune('A')
	assert.Equal(t, "あA", entry.Text)
}

// TestEntry_DesktopCommitMultiRune covers a multi-rune commit delivered as
// successive char events while composing (#354).
func TestEntry_DesktopCommitMultiRune(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "nihao", CursorPos: 5})
	entry.TypedRune('你') // first committed rune finalizes the composition...
	entry.TypedRune('好') // ...the rest insert normally

	assert.Equal(t, "你好", entry.Text)
	assert.Equal(t, 2, entry.CursorColumn)
}

// TestEntry_EscapeCancelsComposition verifies Esc discards an in-progress
// composition so the user can ignore the suggestion (#354).
func TestEntry_EscapeCancelsComposition(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})
	entry.TypedKey(&event.KeyEvent{Name: event.KeyEscape})

	assert.Equal(t, "", entry.Text, "Esc discards the composition without committing")
	entry.TypedRune('A') // composition ended → normal typing
	assert.Equal(t, "A", entry.Text)
}

// TestEntry_FocusLostCancelsComposition verifies a composition is dropped on
// focus-out rather than left live to be implicitly committed (#354).
func TestEntry_FocusLostCancelsComposition(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.FocusGained()

	entry.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})
	entry.FocusLost()

	assert.Equal(t, "", entry.Text, "focus-out discards the uncommitted composition")
	entry.FocusGained()
	entry.TypedRune('A')
	assert.Equal(t, "A", entry.Text)
}

// A selection made after composition started — a mid-composition long-press
// word-select on mobile — must survive ComposeCommit: the committed text
// lands at the composition anchor, never over the newer selection (#945
// device finding: the anchor-restored caret paired with the selection's
// anchor deleted everything in between).
func TestEntry_ComposeCommit_AfterMidCompositionSelection(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.FocusGained()
	entry.SetText("alpha beta gamma")
	entry.CursorRow, entry.CursorColumn = 0, 16 // compose at the end
	entry.Compose(&event.PreeditEvent{Text: "あか", CursorPos: 2})

	// The user selects "beta" mid-composition (the long-press flow).
	entry.Select(0, 6, 0, 10)
	assert.Equal(t, "beta", entry.SelectedText())

	entry.ComposeCommit("あか")

	assert.Equal(t, "alpha beta gammaあか", entry.Text,
		"commit lands at the composition anchor, nothing deleted")
	assert.Equal(t, "beta", entry.SelectedText(),
		"the mid-composition selection survives the commit")
}

// The same selection survives ComposeCancel, with no text change at all.
func TestEntry_ComposeCancel_AfterMidCompositionSelection(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	entry.FocusGained()
	entry.SetText("alpha beta gamma")
	entry.CursorRow, entry.CursorColumn = 0, 16
	entry.Compose(&event.PreeditEvent{Text: "あか", CursorPos: 2})

	entry.Select(0, 6, 0, 10)
	assert.Equal(t, "beta", entry.SelectedText())

	entry.ComposeCancel()

	assert.Equal(t, "alpha beta gamma", entry.Text)
	assert.Equal(t, "beta", entry.SelectedText(),
		"the mid-composition selection survives the cancel")
}
