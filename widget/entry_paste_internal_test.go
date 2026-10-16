package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestEntryPasteEmptyClipboardErasesSelection is the pre-rebrand tracker #427 regression test:
// pasting an empty clipboard over a selection is a selection deletion — Text
// must update, OnChanged must fire, and the recorded undo action must stay
// consistent with the document. Before the fix, Text kept the old value, the
// next refresh restored the pre-erase content from it (reverting the erase on
// screen) and a later undo re-inserted the "erased" text into a document that
// still contained it.
func TestEntryPasteEmptyClipboardErasesSelection(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("hello world")
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.textProvider().updateRowBounds()

	changed := 0
	var lastChanged string
	e.OnChanged = func(s string) { changed++; lastChanged = s }

	e.Select(0, 0, 0, 6) // "hello "
	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("")
	e.pasteFromClipboard(cb)

	assert.Equal(t, "world", e.Text)
	assert.Equal(t, "world", e.textProvider().String())
	assert.Equal(t, "world", string(e.textProvider().row(0)))
	assert.Equal(t, 1, changed)
	assert.Equal(t, "world", lastChanged)

	// The erase survives a refresh (it used to be reverted here).
	e.Refresh()
	assert.Equal(t, "world", e.Text)
	assert.Equal(t, "world", e.textProvider().String())

	// Undo restores exactly the erased text — no duplication.
	require.True(t, e.undoStack.CanUndo())
	e.Undo()
	assert.Equal(t, "hello world", e.Text)
	e.Redo()
	assert.Equal(t, "world", e.Text)
}

// TestEntryPasteEmptyClipboardNoSelection pins that an empty-clipboard paste
// without a selection stays a no-op: no text change, no OnChanged, no undo.
func TestEntryPasteEmptyClipboardNoSelection(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("hello world")
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	changed := 0
	e.OnChanged = func(string) { changed++ }

	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("")
	e.pasteFromClipboard(cb)

	assert.Equal(t, "hello world", e.Text)
	assert.Zero(t, changed)
	assert.False(t, e.undoStack.CanUndo())
}
