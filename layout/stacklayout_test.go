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

	"github.com/stretchr/testify/assert"
)

func TestStackLayout(t *testing.T) {
	size := geo.NewSize(100, 100)

	obj := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	container := &bellui.Container{
		Objects: []core.CanvasObject{obj},
	}
	container.Resize(size)

	layout.NewStackLayout().Layout(container.Objects, size)

	assert.Equal(t, obj.Size(), size)
}

func TestStackLayoutMinSize(t *testing.T) {
	text := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	layoutMin := layout.NewStackLayout().MinSize(container.Objects)

	assert.Equal(t, minSize, layoutMin)
}

func TestContainerStackLayoutMinSize(t *testing.T) {
	text := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	container.Layout = layout.NewStackLayout()
	layoutMin := container.MinSize()

	assert.Equal(t, minSize, layoutMin)
}
