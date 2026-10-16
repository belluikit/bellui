package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
)

// Entry's core.AccessibleTexter implementation (pre-rebrand tracker #501): the seam platform
// accessibility bridges use for screen-reader text navigation and editing.

func TestEntryAccessibleText(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("héllo\nwörld")
	assert.Equal(t, "héllo\nwörld", e.AccessibleText())

	p := NewPasswordEntry()
	p.SetText("sécret")
	masked := p.AccessibleText()
	assert.Equal(t, strings.Repeat(passwordChar, 6), masked, "masked, same rune length")
	assert.NotContains(t, masked, "s")
}

func TestEntryAccessibleSelection(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("héllo\nwörld")

	// No selection: both offsets are the caret.
	e.CursorRow, e.CursorColumn = 1, 2
	start, end := e.AccessibleSelection()
	assert.Equal(t, 8, start)
	assert.Equal(t, 8, end)
	assert.Equal(t, 8, e.AccessibleCaret())

	e.Select(0, 2, 1, 3)
	start, end = e.AccessibleSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 9, end)
}

func TestEntrySetAccessibleSelection(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("héllo\nwörld")

	e.SetAccessibleSelection(2, 9)
	assert.Equal(t, "llo\nwör", e.SelectedText())
	assert.Equal(t, 9, e.AccessibleCaret(), "caret at the end of the selection")

	// Reversed and out-of-range offsets normalize and clamp.
	e.SetAccessibleSelection(100, 6)
	assert.Equal(t, "wörld", e.SelectedText())

	// Equal offsets: caret move, no selection.
	e.SetAccessibleSelection(3, 3)
	_, ok := e.SelectionRange()
	assert.False(t, ok)
	assert.Equal(t, 3, e.AccessibleCaret())
}

func TestEntryReplaceAccessibleSelection(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("héllo\nwörld")
	changes := 0
	e.OnChanged = func(string) { changes++ }

	// Replaces the selection as one edit with one OnChanged.
	e.SetAccessibleSelection(2, 9)
	e.ReplaceAccessibleSelection("y th")
	assert.Equal(t, "héy thld", e.Text)
	assert.Equal(t, 1, changes)

	// One undo step restores the original content.
	e.Undo()
	assert.Equal(t, "héllo\nwörld", e.Text)

	// No selection: inserts at the caret.
	e.SetAccessibleSelection(5, 5)
	e.ReplaceAccessibleSelection("!!")
	assert.Equal(t, "héllo!!\nwörld", e.Text)

	// Read-only entries reject the edit.
	e.ReadOnly = true
	e.ReplaceAccessibleSelection("nope")
	assert.Equal(t, "héllo!!\nwörld", e.Text)
}

func TestEntryAccessibleTexterInterface(t *testing.T) {
	var tx core.AccessibleTexter = NewEntry()
	require.NotNil(t, tx)
}
