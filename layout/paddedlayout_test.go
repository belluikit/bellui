package layout_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestPaddedLayout(t *testing.T) {
	size := geo.NewSize(100, 100)

	obj := canvas.NewRectangle(color.Black)
	container := &bellui.Container{
		Objects: []core.CanvasObject{obj},
	}
	container.Resize(size)

	layout.NewPaddedLayout().Layout(container.Objects, size)

	assert.Equal(t, obj.Size().Width, size.Width-2*theme.Padding())
	assert.Equal(t, obj.Size().Height, size.Height-2*theme.Padding())
}

func TestPaddedLayout_MinSize(t *testing.T) {
	text := canvas.NewText("FooBar", color.Black)
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	layoutMin := layout.NewPaddedLayout().MinSize(container.Objects)

	assert.Equal(t, minSize.Width+2*theme.Padding(), layoutMin.Width)
	assert.Equal(t, minSize.Height+2*theme.Padding(), layoutMin.Height)
}

func TestContainer_PaddedLayout_MinSize(t *testing.T) {
	text := canvas.NewText("FooBar", color.Black)
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	container.Layout = layout.NewPaddedLayout()
	layoutMin := container.MinSize()

	assert.Equal(t, minSize.Width+2*theme.Padding(), layoutMin.Width)
	assert.Equal(t, minSize.Height+2*theme.Padding(), layoutMin.Height)
}
