package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// tappableChildren returns the tappable objects (the per-destination item
// widgets) from a renderer's object list, in order, skipping background
// rectangles, separators and other non-interactive decoration.
func tappableChildren(objs []core.CanvasObject) []event.Tappable {
	var out []event.Tappable
	for _, o := range objs {
		if tp, ok := o.(event.Tappable); ok {
			out = append(out, tp)
		}
	}
	return out
}

func TestNavigationBar_New(t *testing.T) {
	test.NewTempApp(t)
	items := []*widget.NavigationBarItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Search", Icon: theme.SearchIcon()},
	}
	nav := widget.NewNavigationBar(items, nil)
	assert.NotNil(t, nav)
	assert.Equal(t, 0, nav.Selected)
	assert.Len(t, nav.Items, 2)
}

func TestNavigationBar_MinSize(t *testing.T) {
	test.NewTempApp(t)
	items := []*widget.NavigationBarItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}
	nav := widget.NewNavigationBar(items, nil)
	min := nav.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))

	// Width must scale with item count — each item contributes its own min-width.
	items5 := []*widget.NavigationBarItem{
		{Label: "A", Icon: theme.HomeIcon()},
		{Label: "B", Icon: theme.HomeIcon()},
		{Label: "C", Icon: theme.HomeIcon()},
		{Label: "D", Icon: theme.HomeIcon()},
		{Label: "E", Icon: theme.HomeIcon()},
	}
	nav5 := widget.NewNavigationBar(items5, nil)
	min5 := nav5.MinSize()
	// 5 items must be wider than 2 items.
	assert.Greater(t, min5.Width, min.Width)
	// Per-item width should be at least 60dp (MD3 minimum ~48dp, our item uses barH*0.8).
	perItem := min5.Width / 5
	assert.GreaterOrEqual(t, perItem, float32(48),
		"per-item min width should be at least 48dp for MD3 compliance")
}

func TestNavigationBar_SelectedChange(t *testing.T) {
	test.NewTempApp(t)
	var selected int
	items := []*widget.NavigationBarItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Search", Icon: theme.SearchIcon()},
		{Label: "Profile", Icon: theme.AccountIcon()},
	}
	nav := widget.NewNavigationBar(items, func(idx int) { selected = idx })

	assert.Equal(t, 0, nav.Selected)

	nav.Selected = 2
	nav.Refresh()
	assert.Equal(t, 2, nav.Selected)

	// Verify callback was wired.
	assert.NotNil(t, nav.OnChanged)
	nav.OnChanged(1)
	assert.Equal(t, 1, selected)
}

func TestNavigationBar_ItemTap(t *testing.T) {
	test.NewTempApp(t)
	var got, calls int
	items := []*widget.NavigationBarItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Search", Icon: theme.SearchIcon()},
		{Label: "Profile", Icon: theme.AccountIcon()},
	}
	nav := widget.NewNavigationBar(items, func(idx int) { got = idx; calls++ })
	test.NewTempWindow(t, nav)

	taps := tappableChildren(test.TempWidgetRenderer(t, nav).Objects())
	assert.Len(t, taps, 3)

	// A real tap on the third destination must update Selected and fire OnChanged.
	test.Tap(taps[2])
	assert.Equal(t, 2, nav.Selected)
	assert.Equal(t, 2, got)
	assert.Equal(t, 1, calls)
}
