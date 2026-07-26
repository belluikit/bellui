package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestNavigationDrawer_New(t *testing.T) {
	items := []*widget.NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}
	d := widget.NewNavigationDrawer(items, nil)
	assert.NotNil(t, d)
	assert.Len(t, d.Items, 2)
	assert.Equal(t, 0, d.Selected)
	assert.True(t, d.Modal)
}

func TestNavigationDrawer_HideWithoutShow(t *testing.T) {
	d := widget.NewNavigationDrawer(nil, nil)
	assert.NotPanics(t, func() { d.Hide() })
	assert.False(t, d.IsOpen())
}

func TestNavigationDrawer_IsOpen(t *testing.T) {
	d := widget.NewNavigationDrawer(nil, nil)
	assert.False(t, d.IsOpen())
}

func TestNavigationDrawer_WithDivider(t *testing.T) {
	items := []*widget.NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{IsDivider: true},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}
	d := widget.NewNavigationDrawer(items, nil)
	assert.Len(t, d.Items, 3)
	assert.True(t, d.Items[1].IsDivider)
}
