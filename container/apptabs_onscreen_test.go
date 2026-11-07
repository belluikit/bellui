package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestAppTabs_IsOnScreen is #1061: a widget on a tab that is not selected is
// not on screen, although neither it nor its tab's content reports hidden to
// it, and is again once the tab is selected.
func TestAppTabs_IsOnScreen(t *testing.T) {
	test.NewTempApp(t)
	spinner := widget.NewProgressBarInfinite()
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewVBox(spinner)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	test.NewTempWindow(t, tabs)
	assert.True(t, bellui.IsOnScreen(spinner))

	tabs.SelectIndex(1)
	assert.True(t, spinner.Visible(), "the widget itself reports visible")
	assert.False(t, bellui.IsOnScreen(spinner), "on screen on a tab not selected")

	tabs.SelectIndex(0)
	assert.True(t, bellui.IsOnScreen(spinner), "not on screen on the tab selected again")
}
