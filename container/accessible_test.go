package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestTabButton_Accessible(t *testing.T) {
	test.NewTempApp(t)

	tabs := NewAppTabs(
		NewTabItem("First", widget.NewLabel("first content")),
		NewTabItem("Second", widget.NewLabel("second content")),
	)
	test.TempWidgetRenderer(t, tabs) // realize the renderer
	tabs.Refresh()                   // build the tab buttons

	first := tabs.Items[0].button
	second := tabs.Items[1].button
	assert.NotNil(t, first)
	assert.NotNil(t, second)

	assert.Equal(t, core.RoleTab, first.AccessibleRole())
	assert.Equal(t, "First", first.AccessibleLabel())
	assert.True(t, first.AccessibleState().Has(core.StateSelected))
	assert.False(t, second.AccessibleState().Has(core.StateSelected))

	second.PerformAccessibleAction(core.ActionSelect)
	assert.Equal(t, 1, tabs.SelectedIndex())
	assert.True(t, second.AccessibleState().Has(core.StateSelected))
	assert.False(t, first.AccessibleState().Has(core.StateSelected))

	var _ core.Accessible = (*tabButton)(nil)
	var _ core.AccessibleStateful = (*tabButton)(nil)
	var _ core.AccessibleActioner = (*tabButton)(nil)
}
