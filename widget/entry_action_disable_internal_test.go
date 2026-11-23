package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// A disabled entry disables its action item with it: a SearchField's clear
// button stayed bright and live in a disabled field (#1404). Enabling the
// entry enables the item again.
func TestEntry_DisableDisablesActionItem(t *testing.T) {
	test.NewTempApp(t)
	sf := NewSearchField(nil)
	test.NewTempWindow(t, sf)
	sf.SetText("query")
	require.NotNil(t, sf.clearBtn)
	require.True(t, sf.clearBtn.Visible())

	sf.Disable()
	assert.True(t, sf.clearBtn.Disabled(), "the clear button is disabled with the field")
	test.Tap(sf.clearBtn)
	assert.Equal(t, "query", sf.Text, "and takes no tap")

	sf.Enable()
	assert.False(t, sf.clearBtn.Disabled())
	test.Tap(sf.clearBtn)
	assert.Empty(t, sf.Text, "enabled again, it clears")
}

// A DateEntry's calendar button goes with its entry too.
func TestDateEntry_DisableDisablesCalendarButton(t *testing.T) {
	test.NewTempApp(t)
	de := NewDateEntry()
	test.NewTempWindow(t, de)
	btn, ok := de.ActionItem.(core.Disableable)
	require.True(t, ok, "the calendar button can be disabled")
	de.Disable()
	assert.True(t, btn.Disabled())
	de.Enable()
	assert.False(t, btn.Disabled())
}

// An action item the app disabled itself stays disabled when the entry is
// enabled again, and a replaced item is released.
func TestEntry_ActionItemKeepsTheAppsState(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	own := NewButtonWithIcon("", theme.SearchIcon(), nil)
	own.Disable()
	e.ActionItem = own
	test.NewTempWindow(t, e)
	e.Disable()
	e.Enable()
	assert.True(t, own.Disabled(), "the app's own disabled item stays disabled")

	first := NewButtonWithIcon("", theme.SearchIcon(), nil)
	e.ActionItem = first
	e.Refresh()
	e.Disable()
	require.True(t, first.Disabled())
	second := NewButtonWithIcon("", theme.SearchIcon(), nil)
	e.ActionItem = second
	e.Refresh()
	assert.False(t, first.Disabled(), "the replaced item is released")
	assert.True(t, second.Disabled(), "the new item is disabled with the entry")
	e.Enable()
	assert.False(t, second.Disabled())
}

// A tap through the canvas, the driver's own path, on a disabled field's
// clear button does not clear it. On main before #1404 it did: the button
// took the tap inside the disabled field.
func TestSearchField_DisabledClearIgnoresCanvasTap(t *testing.T) {
	test.NewTempApp(t)
	sf := NewSearchField(nil)
	w := test.NewTempWindow(t, sf)
	w.Resize(geo.NewSize(300, 60))
	sf.SetText("query")
	sf.Disable()
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(sf.clearBtn).
		Add(geo.NewPos(sf.clearBtn.Size().Width/2, sf.clearBtn.Size().Height/2))
	test.TapCanvas(w.Canvas(), pos)
	assert.Equal(t, "query", sf.Text)
}
