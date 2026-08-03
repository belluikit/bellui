package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestNavigationRail_New(t *testing.T) {
	test.NewTempApp(t)
	items := []*widget.NavigationRailItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Search", Icon: theme.SearchIcon()},
	}
	rail := widget.NewNavigationRail(items, nil)
	assert.NotNil(t, rail)
	assert.True(t, rail.ShowLabels)
	assert.Equal(t, 0, rail.Selected)
}

func TestNavigationRail_MinSize(t *testing.T) {
	test.NewTempApp(t)
	items := []*widget.NavigationRailItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}
	rail := widget.NewNavigationRail(items, nil)
	min := rail.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestNavigationRail_SelectedChange(t *testing.T) {
	test.NewTempApp(t)
	var selected int
	items := []*widget.NavigationRailItem{
		{Label: "A", Icon: theme.HomeIcon()},
		{Label: "B", Icon: theme.SearchIcon()},
	}
	rail := widget.NewNavigationRail(items, func(idx int) { selected = idx })
	assert.Equal(t, 0, rail.Selected)

	rail.OnChanged(1)
	assert.Equal(t, 1, selected)
}

func TestNavigationRail_ItemTap(t *testing.T) {
	test.NewTempApp(t)
	var got, calls int
	items := []*widget.NavigationRailItem{
		{Label: "A", Icon: theme.HomeIcon()},
		{Label: "B", Icon: theme.SearchIcon()},
		{Label: "C", Icon: theme.AccountIcon()},
	}
	rail := widget.NewNavigationRail(items, func(idx int) { got = idx; calls++ })
	test.NewTempWindow(t, rail)

	taps := tappableChildren(test.TempWidgetRenderer(t, rail).Objects())
	assert.Len(t, taps, 3)

	test.Tap(taps[1])
	assert.Equal(t, 1, rail.Selected)
	assert.Equal(t, 1, got)
	assert.Equal(t, 1, calls)
}

// Items is a public slice, so a host can replace, append to or truncate it
// after the renderer exists. Refresh must re-sync the drawn destinations with
// the live slice (#203, #234).
func TestNavigationRail_RefreshResyncsItems(t *testing.T) {
	test.NewTempApp(t)
	var got int
	rail := widget.NewNavigationRail([]*widget.NavigationRailItem{
		{Label: "A", Icon: theme.HomeIcon()},
	}, func(idx int) { got = idx })
	test.NewTempWindow(t, rail)
	r := test.TempWidgetRenderer(t, rail)
	assert.Len(t, tappableChildren(r.Objects()), 1)
	minBefore := r.MinSize()

	// Append: the new destination must draw, contribute to MinSize and select
	// on tap.
	rail.Items = append(rail.Items, &widget.NavigationRailItem{Label: "B", Icon: theme.SearchIcon()})
	rail.Refresh()
	taps := tappableChildren(r.Objects())
	assert.Len(t, taps, 2)
	assert.Greater(t, r.MinSize().Height, minBefore.Height)
	test.Tap(taps[1])
	assert.Equal(t, 1, rail.Selected)
	assert.Equal(t, 1, got)

	// Wholesale replacement re-points the surviving widget at the new item.
	rail.Items = []*widget.NavigationRailItem{{Label: "C", Icon: theme.AccountIcon()}}
	rail.Selected = 0
	rail.Refresh()
	taps = tappableChildren(r.Objects())
	assert.Len(t, taps, 1)
	assert.Equal(t, "C", taps[0].(core.Accessible).AccessibleLabel())
}
