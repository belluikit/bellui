package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
)

var _ event.Tabbable = (*DataGrid)(nil)

// Tab moves the keyboard cursor to the next cell and Shift+Tab to the
// previous one, wrapping across rows (owner decision on #1512). The grid keeps
// Tab while enabled; without that the toolkit moved window focus and the
// grid's Tab handling was never reached.
func TestDataGrid_TabMovesCellToCell(t *testing.T) {
	g, _ := pressGrid(t) // columns a and b
	assert.True(t, g.AcceptsTab(), "an enabled grid keeps Tab")
	g.currentFocus = CellID{Row: 1, Col: 0}

	tab := func(mod event.KeyModifier) { g.TypedKey(&event.KeyEvent{Name: event.KeyTab, Modifier: mod}) }
	tab(0)
	assert.Equal(t, CellID{Row: 1, Col: 1}, g.currentFocus)
	tab(0)
	assert.Equal(t, CellID{Row: 2, Col: 0}, g.currentFocus, "Tab wraps to the next row")

	tab(event.KeyModifierShift)
	assert.Equal(t, CellID{Row: 1, Col: 1}, g.currentFocus, "Shift+Tab goes back, wrapping")
	tab(event.KeyModifierShift)
	assert.Equal(t, CellID{Row: 1, Col: 0}, g.currentFocus)

	// Shift comes from the key, not from the last mouse press.
	g.lastModifier = event.KeyModifierShift
	tab(0)
	assert.Equal(t, CellID{Row: 1, Col: 1}, g.currentFocus, "a Shift held at the last click turned Tab around")

	g.Disable()
	assert.False(t, g.AcceptsTab(), "a disabled grid keeps no Tab")
}
