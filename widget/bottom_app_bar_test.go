package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestBottomAppBar_New(t *testing.T) {
	test.NewTempApp(t)
	actions := []*widget.BottomAppBarAction{
		{Icon: theme.SearchIcon(), OnTapped: func() {}},
	}
	bar := widget.NewBottomAppBar(actions, nil)
	assert.NotNil(t, bar)
	assert.Len(t, bar.Actions, 1)
	assert.Nil(t, bar.FAB)
}

func TestBottomAppBar_MinSize(t *testing.T) {
	test.NewTempApp(t)
	bar := widget.NewBottomAppBar(nil, nil)
	min := bar.MinSize()
	assert.GreaterOrEqual(t, min.Width, float32(200))
	assert.GreaterOrEqual(t, min.Height, float32(80))
}

func TestBottomAppBar_WithFAB(t *testing.T) {
	test.NewTempApp(t)
	fab := widget.NewFAB(theme.ContentAddIcon(), nil)
	bar := widget.NewBottomAppBar(nil, fab)
	assert.Equal(t, fab, bar.FAB)
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, bar) })
}

func TestBottomAppBar_Renderer(t *testing.T) {
	test.NewTempApp(t)
	actions := []*widget.BottomAppBarAction{
		{Icon: theme.SearchIcon()},
		{Icon: theme.MoreVerticalIcon()},
	}
	bar := widget.NewBottomAppBar(actions, nil)
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, bar) })
}
