package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestTopAppBar_New(t *testing.T) {
	test.NewTempApp(t)
	bar := widget.NewTopAppBar("Title", theme.NavigateBackIcon(), nil)
	assert.NotNil(t, bar)
	assert.Equal(t, "Title", bar.Title)
}

func TestTopAppBar_MinSize(t *testing.T) {
	test.NewTempApp(t)
	bar := widget.NewTopAppBar("App", theme.NavigateBackIcon(), nil)
	min := bar.MinSize()
	assert.GreaterOrEqual(t, min.Width, float32(200))
	assert.GreaterOrEqual(t, min.Height, float32(64))
}

func TestTopAppBar_NoNavIcon(t *testing.T) {
	test.NewTempApp(t)
	bar := widget.NewTopAppBar("Simple", nil, nil)
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, bar) })
}

func TestTopAppBar_WithActions(t *testing.T) {
	test.NewTempApp(t)
	bar := widget.NewTopAppBar("Title", nil, nil)
	bar.Actions = []widget.TopAppBarAction{
		{Icon: theme.SearchIcon(), OnTapped: func() {}},
		{Icon: theme.MoreVerticalIcon(), OnTapped: func() {}},
	}
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, bar) })
}
