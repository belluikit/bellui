package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/test"
)

// TestNumericEntry_ReplaceAccessibleSelectionFilters guards #782: range
// replacements that bypass TypedRune (accessibility text protocols, the mobile
// IME mirror) must still be filtered to numeric content.
func TestNumericEntry_ReplaceAccessibleSelectionFilters(t *testing.T) {
	test.NewTempApp(t)

	e := NewNumericEntry()
	e.SetText("12")

	// Replace the whole selection with mixed content: only the digits survive.
	e.SetAccessibleSelection(0, 2)
	e.ReplaceAccessibleSelection("9a8b")
	assert.Equal(t, "98", e.Text)

	// A replacement that filters down to nothing is rejected (no-op), matching
	// TypedRune's silent rejection — the field is unchanged.
	e.SetAccessibleSelection(0, 2)
	e.ReplaceAccessibleSelection("abc")
	assert.Equal(t, "98", e.Text)

	// An empty replacement (deletion) still applies.
	e.SetAccessibleSelection(0, 1)
	e.ReplaceAccessibleSelection("")
	assert.Equal(t, "8", e.Text)
}
