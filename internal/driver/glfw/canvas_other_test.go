//go:build !no_glfw && !ci && !mobile && (!darwin || no_native_menus)

package glfw

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"

	"github.com/stretchr/testify/assert"
)

func TestGlCanvas_FocusHandlingWhenActivatingOrDeactivatingTheMenu(t *testing.T) {
	w := createWindow("Test")
	w.SetPadded(false)
	runOnMain(func() {
		w.SetMainMenu(
			bellui.NewMainMenu(
				bellui.NewMenu("test", bellui.NewMenuItem("item", func() {})),
				bellui.NewMenu("other", bellui.NewMenuItem("item", func() {})),
			),
		)
	})
	c := w.Canvas()

	ce1 := &focusable{id: "ce1"}
	ce2 := &focusable{id: "ce2"}
	content := container.NewVBox(ce1, ce2)
	w.SetContent(content)

	assert.Nil(t, c.Focused())
	m := c.menu.(*MenuBar)
	assert.False(t, m.IsActive())

	c.FocusPrevious()
	assert.Equal(t, ce2, c.Focused())
	assert.True(t, ce2.focused)

	runOnMain(func() {
		m.Items[0].(*menuBarItem).Tapped(&event.PointEvent{})
	})
	assert.True(t, m.IsActive())
	ctxt := "activating the menu changes focus handler and focuses the menu bar item but does not remove focus from content"
	assert.True(t, ce2.focused, ctxt)
	assert.Equal(t, m.Items[0], c.Focused(), ctxt)

	c.FocusNext()
	ctxt = "changing focus with active menu does not affect content focus"
	assert.True(t, ce2.focused, ctxt)
	assert.Equal(t, m.Items[1], c.Focused(), ctxt)

	runOnMain(func() {
		m.Items[0].(*menuBarItem).Tapped(&event.PointEvent{})
	})
	assert.False(t, m.IsActive())
	ctxt = "deactivating the menu restores focus handler from content"
	assert.True(t, ce2.focused, ctxt)
	assert.Equal(t, ce2, c.Focused(), ctxt)

	c.FocusPrevious()
	assert.Equal(t, ce1, c.Focused(), ctxt)
	assert.True(t, ce1.focused, ctxt)
	assert.False(t, ce2.focused, ctxt)
}
