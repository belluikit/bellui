package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestNewContextMenu(t *testing.T) {
	label := NewLabel("Right-click me")
	item := bellui.NewMenuItem("Action", func() {})
	cm := NewContextMenu(label, item)

	assert.NotNil(t, cm)
	assert.Equal(t, label, cm.Content)
	assert.Len(t, cm.Items, 1)
}

func TestContextMenu_Render(t *testing.T) {
	label := NewLabel("Test")
	cm := NewContextMenu(label, bellui.NewMenuItem("Do", func() {}))
	r := test.WidgetRenderer(cm)

	assert.NotNil(t, r)
	min := cm.MinSize()
	assert.Equal(t, label.MinSize(), min)
}

func TestContextMenu_Tapped(t *testing.T) {
	tapped := false
	btn := NewButton("Click", func() { tapped = true })
	cm := NewContextMenu(btn, bellui.NewMenuItem("Menu", func() {}))
	_ = test.WidgetRenderer(cm)

	test.Tap(cm)
	assert.True(t, tapped)
}

func TestContextMenu_TappedSecondary(t *testing.T) {
	test.NewTempApp(t)
	label := NewLabel("Test")
	var actionCalled bool
	cm := NewContextMenu(label, bellui.NewMenuItem("Action", func() {
		actionCalled = true
	}))

	w := test.NewWindow(cm)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	test.TapSecondary(cm)
	assert.NotNil(t, cm.popUp)
	_ = actionCalled // menu displayed, action not called until selection
}

func TestContextMenu_WithMenu(t *testing.T) {
	label := NewLabel("Test")
	menu := bellui.NewMenu("Test", bellui.NewMenuItem("A", func() {}), bellui.NewMenuItem("B", func() {}))
	cm := NewContextMenuWithMenu(label, menu)

	assert.NotNil(t, cm.Menu)
	assert.Equal(t, menu, cm.buildMenu())
}

func TestContextMenu_EmptyMenu(t *testing.T) {
	label := NewLabel("Test")
	cm := NewContextMenu(label)

	assert.Nil(t, cm.buildMenu())
}

func TestContextMenu_Hide(t *testing.T) {
	label := NewLabel("Test")
	cm := NewContextMenu(label, bellui.NewMenuItem("X", func() {}))
	_ = test.WidgetRenderer(cm)
	cm.Hide()
	assert.False(t, cm.Visible())
}
