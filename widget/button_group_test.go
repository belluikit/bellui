package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestButtonGroup_New(t *testing.T) {
	test.NewTempApp(t)
	b1 := widget.NewButton("A", nil)
	b2 := widget.NewButton("B", nil)
	g := widget.NewButtonGroup(b1, b2)
	assert.NotNil(t, g)
	assert.Len(t, g.Buttons, 2)
}

func TestButtonGroup_MinSize(t *testing.T) {
	test.NewTempApp(t)
	b1 := widget.NewButton("Save", nil)
	b2 := widget.NewButton("Cancel", nil)
	g := widget.NewButtonGroup(b1, b2)
	min := g.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestButtonGroup_Renderer(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewButtonGroup(widget.NewButton("OK", nil))
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, g) })
}

func TestButtonGroup_Empty(t *testing.T) {
	test.NewTempApp(t)
	g := widget.NewButtonGroup()
	assert.NotNil(t, g)
	assert.Empty(t, g.Buttons)
}
