package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestFABMenu_New(t *testing.T) {
	items := []*widget.FABMenuItem{
		{Label: "Add", Icon: theme.ContentAddIcon()},
		{Label: "Edit", Icon: theme.DocumentCreateIcon()},
	}
	menu := widget.NewFABMenu(theme.ContentAddIcon(), items)
	assert.NotNil(t, menu)
	assert.Len(t, menu.Items, 2)
	assert.False(t, menu.IsOpen())
}

func TestFABMenu_HideWithoutShow(t *testing.T) {
	menu := widget.NewFABMenu(theme.ContentAddIcon(), nil)
	assert.NotPanics(t, func() { menu.Hide() })
	assert.False(t, menu.IsOpen())
}

func TestFABMenu_IsOpen(t *testing.T) {
	menu := widget.NewFABMenu(theme.ContentAddIcon(), nil)
	assert.False(t, menu.IsOpen())
}
