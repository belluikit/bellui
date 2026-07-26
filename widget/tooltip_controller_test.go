package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/test"
)

func TestBaseWidget_ToolTipText(t *testing.T) {
	btn := NewButton("New", nil)
	assert.Equal(t, "", btn.ToolTip(), "no tooltip by default")

	btn.SetToolTipText("New file")
	assert.Equal(t, "New file", btn.ToolTip())

	// Every widget is a core.ToolTipable so the driver can find it by hit-test.
	var _ core.ToolTipable = &btn.BaseWidget
	tt, ok := any(btn).(core.ToolTipable)
	assert.True(t, ok, "Button must satisfy core.ToolTipable")
	assert.Equal(t, "New file", tt.ToolTip())
}

func TestToolTipController_WiredIntoCore(t *testing.T) {
	// The widget package installs the driver-level seam on init.
	assert.NotNil(t, core.ShowToolTip, "widget init must install core.ShowToolTip")
	assert.NotNil(t, core.HideToolTip, "widget init must install core.HideToolTip")
}

func TestToolTipController_PresentAndHide(t *testing.T) {
	test.NewTempApp(t)
	btn := NewButton("New", nil)
	btn.SetToolTipText("New file")
	w := test.NewTempWindow(t, btn)
	c := w.Canvas()

	ctl := &toolTipController{active: btn} // present only fires for the active target
	before := len(c.Overlays().List())
	ctl.present(btn, c, "New file")
	assert.Equal(t, before+1, len(c.Overlays().List()), "present adds one tooltip overlay")

	ctl.hide()
	assert.Equal(t, before, len(c.Overlays().List()), "hide removes the tooltip overlay")
}

func TestToolTipController_ShowDedup(t *testing.T) {
	test.NewTempApp(t)
	btn := NewButton("New", nil)
	btn.SetToolTipText("New file")
	w := test.NewTempWindow(t, btn)
	c := w.Canvas()

	ctl := &toolTipController{}
	ctl.show(btn, c)
	first := ctl.timer
	assert.Equal(t, core.CanvasObject(btn), ctl.active)
	assert.NotNil(t, first, "first show arms a delay timer")

	ctl.show(btn, c) // same target — must not restart
	assert.Same(t, first, ctl.timer, "re-showing the same target must not restart the timer")

	ctl.hide()
	assert.Nil(t, ctl.active)
	assert.Nil(t, ctl.timer, "hide cancels the timer")
}

func TestToolTipController_EmptyTextNoTimer(t *testing.T) {
	test.NewTempApp(t)
	btn := NewButton("New", nil) // no tooltip text
	w := test.NewTempWindow(t, btn)
	c := w.Canvas()

	ctl := &toolTipController{}
	ctl.show(btn, c)
	assert.Equal(t, core.CanvasObject(btn), ctl.active, "empty target still tracked so it dedups")
	assert.Nil(t, ctl.timer, "no timer armed for empty tooltip text")
}
