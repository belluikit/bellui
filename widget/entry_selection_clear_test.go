package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntry_SetText_ClearsSelection is the pre-rebrand tracker #154 repro: a wholesale SetText must
// drop any active selection, so the next edit inserts cleanly instead of
// replacing a phantom range left by the stale (clamped) anchor.
func TestEntry_SetText_ClearsSelection(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Hello World!")
	e.Select(0, 6, 0, 11) // "World"
	require.Equal(t, "World", e.SelectedText())

	e.SetText("Hello World!") // reset to the same text
	assert.Equal(t, "", e.SelectedText(), "SetText must clear the selection")

	e.CursorColumn = 5
	e.TypedRune('Z')
	assert.Equal(t, "HelloZ World!", e.Text,
		"after SetText the edit inserts rather than replacing a phantom selection")
}

// TestEntry_SetText_NotifiesOnSelectionCleared: clearing a selection via SetText
// fires OnSelectChanged with the empty string.
func TestEntry_SetText_NotifiesOnSelectionCleared(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Hello World!")
	e.Select(0, 6, 0, 11)

	var got string
	fired := 0
	e.OnSelectChanged = func(s string) { got = s; fired++ }

	e.SetText("new text")
	assert.Equal(t, 1, fired, "SetText that clears a selection fires OnSelectChanged once")
	assert.Equal(t, "", got, "OnSelectChanged receives the empty string")
}

// TestEntry_Unselect clears the selection programmatically, preserves the cursor,
// and fires OnSelectChanged("").
func TestEntry_Unselect(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Hello World!")

	var got string
	fired := 0
	e.OnSelectChanged = func(s string) { got = s; fired++ }

	e.Select(0, 0, 0, 5) // "Hello", cursor ends at col 5
	require.Equal(t, "Hello", e.SelectedText())
	fired = 0

	e.Unselect()
	assert.Equal(t, "", e.SelectedText(), "Unselect clears the selection")
	assert.Equal(t, 1, fired, "Unselect fires OnSelectChanged once")
	assert.Equal(t, "", got, "OnSelectChanged receives the empty string")
	assert.Equal(t, 0, e.CursorRow, "cursor row preserved")
	assert.Equal(t, 5, e.CursorColumn, "cursor column preserved")
}

// TestEntry_Unselect_NoOpWhenEmpty: Unselect with nothing selected does not fire
// OnSelectChanged.
func TestEntry_Unselect_NoOpWhenEmpty(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Hello")

	fired := 0
	e.OnSelectChanged = func(string) { fired++ }

	e.Unselect()
	assert.Equal(t, 0, fired, "Unselect on an empty selection must not fire OnSelectChanged")
	assert.Equal(t, "", e.SelectedText())
}
