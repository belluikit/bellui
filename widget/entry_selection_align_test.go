package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestEntry_SelectionAdoptsAlignment guards pre-rebrand tracker #781: a non-leading Entry alignment
// must propagate to the selectable so the selection highlight (and drag
// hit-testing) tracks the text edge instead of being pinned to the left.
func TestEntry_SelectionAdoptsAlignment(t *testing.T) {
	test.NewTempApp(t)

	e := NewEntry()
	e.Alignment = bellui.TextAlignTrailing
	e.SetText("hi")
	e.syncSelectable()

	assert.Equal(t, bellui.TextAlignTrailing, e.sel.alignment,
		"selection must adopt the Entry's alignment (#781)")

	// Laid out wider than the text, a trailing-aligned row now offsets the
	// highlight toward the trailing edge rather than returning 0 (left).
	e.sel.provider = e.textProvider()
	e.sel.Resize(geo.NewSize(300, 40))
	assert.Greater(t, e.sel.alignOffsetX(0), float32(0),
		"trailing alignment must offset the selection highlight")

	// Leading alignment stays at 0 (no regression).
	e.Alignment = bellui.TextAlignLeading
	e.syncSelectable()
	assert.Equal(t, bellui.TextAlignLeading, e.sel.alignment)
	assert.Equal(t, float32(0), e.sel.alignOffsetX(0))
}
