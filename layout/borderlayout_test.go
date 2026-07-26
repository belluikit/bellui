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
	_ "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestNewBorderContainer(t *testing.T) {
	top := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	top.SetMinSize(geo.NewSize(10, 10))
	right := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	right.SetMinSize(geo.NewSize(10, 10))
	middle := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	c := container.NewBorder(top, nil, nil, right, middle)
	assert.Len(t, c.Objects, 3)

	c.Resize(geo.NewSize(100, 100))
	assert.Equal(t, float32(0), top.Position().X)
	assert.Equal(t, float32(0), top.Position().Y)
	assert.Equal(t, float32(90), right.Position().X)
	assert.Equal(t, 10+theme.Padding(), right.Position().Y)
	assert.Equal(t, float32(0), middle.Position().X)
	assert.Equal(t, 10+theme.Padding(), middle.Position().Y)
	assert.Equal(t, 90-theme.Padding(), middle.Size().Width)
	assert.Equal(t, 90-theme.Padding(), middle.Size().Height)
}

func TestBorderLayout_Size_Empty(t *testing.T) {
	size := geo.NewSize(100, 100)

	obj := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	container := &bellui.Container{
		Objects: []core.CanvasObject{obj},
	}
	container.Resize(size)

	layout.NewBorderLayout(nil, nil, nil, nil).Layout(container.Objects, size)

	assert.Equal(t, obj.Size(), size)
}

func TestBorderLayout_Size_TopBottom(t *testing.T) {
	size := geo.NewSize(100, 100)

	obj1 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj3 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	container := &bellui.Container{
		Objects: []core.CanvasObject{obj1, obj2, obj3},
	}
	container.Resize(size)

	layout.NewBorderLayout(obj1, obj2, nil, nil).Layout(container.Objects, size)

	innerSize := geo.NewSize(size.Width, size.Height-obj1.Size().Height-obj2.Size().Height-theme.Padding()*2)
	assert.Equal(t, innerSize, obj3.Size())
	assert.Equal(t, geo.NewPos(0, 0), obj1.Position())
	assert.Equal(t, geo.NewPos(0, size.Height-obj2.Size().Height), obj2.Position())
	assert.Equal(t, geo.NewPos(0, obj1.Size().Height+theme.Padding()), obj3.Position())
}

func TestBorderLayout_Size_LeftRight(t *testing.T) {
	size := geo.NewSize(100, 100)

	obj1 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj3 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	container := &bellui.Container{
		Objects: []core.CanvasObject{obj1, obj2, obj3},
	}
	container.Resize(size)

	layout.NewBorderLayout(nil, nil, obj1, obj2).Layout(container.Objects, size)

	innerSize := geo.NewSize(size.Width-obj1.Size().Width-obj2.Size().Width-theme.Padding()*2, size.Height)
	assert.Equal(t, innerSize, obj3.Size())
	assert.Equal(t, geo.NewPos(0, 0), obj1.Position())
	assert.Equal(t, geo.NewPos(size.Width-obj2.Size().Width, 0), obj2.Position())
	assert.Equal(t, geo.NewPos(obj1.Size().Width+theme.Padding(), 0), obj3.Position())
}

func TestBorderLayout_MinSize_Center(t *testing.T) {
	text := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	layoutMin := layout.NewBorderLayout(nil, nil, nil, nil).MinSize(container.Objects)

	assert.Equal(t, minSize, layoutMin)
}

func TestBorderLayout_MinSize_TopBottom(t *testing.T) {
	text1 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text2 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text3 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := geo.NewSize(text3.MinSize().Width, text1.MinSize().Height+text2.MinSize().Height+text3.MinSize().Height+theme.Padding()*2)

	container := container.NewWithoutLayout(text1, text2, text3)
	layoutMin := layout.NewBorderLayout(text1, text2, nil, nil).MinSize(container.Objects)

	assert.Equal(t, minSize, layoutMin)
}

func TestBorderLayout_MinSize_TopBottomHidden(t *testing.T) {
	text1 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text1.Hide()
	text2 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text2.Hide()
	text3 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})

	container := container.NewWithoutLayout(text1, text2, text3)
	layoutMin := layout.NewBorderLayout(text1, text2, nil, nil).MinSize(container.Objects)

	assert.Equal(t, text1.MinSize(), layoutMin)
}

func TestBorderLayout_MinSize_TopOnly(t *testing.T) {
	text1 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := geo.NewSize(text1.MinSize().Width, text1.MinSize().Height+theme.Padding())

	container := container.NewWithoutLayout(text1)
	layoutMin := layout.NewBorderLayout(text1, nil, nil, nil).MinSize(container.Objects)

	assert.Equal(t, minSize, layoutMin)
}

func TestBorderLayout_MinSize_LeftRight(t *testing.T) {
	text1 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text2 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text3 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := geo.NewSize(text1.MinSize().Width+text2.MinSize().Width+text3.MinSize().Width+theme.Padding()*2, text3.MinSize().Height)

	container := container.NewWithoutLayout(text1, text2, text3)
	layoutMin := layout.NewBorderLayout(nil, nil, text1, text2).MinSize(container.Objects)

	assert.Equal(t, minSize, layoutMin)
}

func TestBorderLayout_MinSize_LeftRightHidden(t *testing.T) {
	text1 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text1.Hide()
	text2 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	text2.Hide()
	text3 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})

	container := container.NewWithoutLayout(text1, text2, text3)
	layoutMin := layout.NewBorderLayout(nil, nil, text1, text2).MinSize(container.Objects)

	assert.Equal(t, text3.MinSize(), layoutMin)
}

func TestBorderLayout_MinSize_LeftOnly(t *testing.T) {
	text1 := canvas.NewText("Padding", color.NRGBA{0, 0xff, 0, 0})
	minSize := geo.NewSize(text1.MinSize().Width+theme.Padding(), text1.MinSize().Height)

	container := container.NewWithoutLayout(text1)
	layoutMin := layout.NewBorderLayout(nil, nil, text1, nil).MinSize(container.Objects)

	assert.Equal(t, minSize, layoutMin)
}
