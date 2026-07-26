package widget_test

import (
	"testing"

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
