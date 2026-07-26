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

func TestGridLWrapLayout_Layout(t *testing.T) {
	gridSize := geo.NewSize(125, 125)
	cellSize := geo.NewSize(50, 50)

	obj1 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj3 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	container := &bellui.Container{
		Objects: []core.CanvasObject{obj1, obj2, obj3},
	}
	container.Resize(gridSize)

	layout.NewGridWrapLayout(cellSize).Layout(container.Objects, gridSize)

	assert.Equal(t, obj1.Size(), cellSize)
	cell2Pos := geo.NewPos(50+theme.Padding(), 0)
	assert.Equal(t, obj2.Position(), cell2Pos)
	cell3Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, obj3.Position(), cell3Pos)
}

func TestGridLWrapLayout_Layout_Min(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj3 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	container := &bellui.Container{
		Objects: []core.CanvasObject{obj1, obj2, obj3},
	}

	layout.NewGridWrapLayout(cellSize).Layout(container.Objects, container.MinSize())

	assert.Equal(t, obj1.Size(), cellSize)
	cell2Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, obj2.Position(), cell2Pos)
	cell3Pos := geo.NewPos(0, 100+theme.Padding()*2)
	assert.Equal(t, obj3.Position(), cell3Pos)
}

func TestGridLWrapLayout_Layout_HiddenItem(t *testing.T) {
	gridSize := geo.NewSize(125, 125)
	cellSize := geo.NewSize(50, 50)

	obj1 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2.Hide()
	obj3 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj4 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	container := &bellui.Container{
		Objects: []core.CanvasObject{obj1, obj2, obj3, obj4},
	}
	container.Resize(gridSize)

	layout.NewGridWrapLayout(cellSize).Layout(container.Objects, gridSize)

	assert.Equal(t, obj1.Size(), cellSize)
	assert.Equal(t, obj3.Position(), geo.NewPos(50+theme.Padding(), 0))
	assert.Equal(t, obj4.Position(), geo.NewPos(0, 50+theme.Padding()))
}

func TestGridLWrapLayout_MinSize(t *testing.T) {
	cellSize := geo.NewSize(50, 50)
	minSize := cellSize

	container := container.NewWithoutLayout(canvas.NewRectangle(color.NRGBA{0, 0, 0, 0}))
	layout := layout.NewGridWrapLayout(cellSize)

	layoutMin := layout.MinSize(container.Objects)
	assert.Equal(t, minSize, layoutMin)

	// This has a dynamic minSize so we need to check again after layout!
	layout.Layout(container.Objects, minSize)
	layoutMin = layout.MinSize(container.Objects)
	assert.Equal(t, minSize, layoutMin)

	// vertical 2 items
	container.Add(canvas.NewRectangle(color.NRGBA{0, 0, 0, 0}))
	layout.Layout(container.Objects, geo.NewSize(minSize.Width, minSize.Height*2.5))
	layoutMin = layout.MinSize(container.Objects)
	assert.Equal(t, minSize.Height*2+theme.Padding(), layoutMin.Height)

	// horizontal 2 items
	layout.Layout(container.Objects, geo.NewSize(minSize.Width*2.5, minSize.Height))
	layoutMin = layout.MinSize(container.Objects)
	assert.Equal(t, minSize.Height, layoutMin.Height)
}

func TestGridLWrapLayout_MinSize_Hidden(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	obj2.Hide()
	obj3 := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})

	container := container.NewWithoutLayout(obj1, obj2, obj3)
	layout := layout.NewGridWrapLayout(cellSize)

	layoutMin := layout.MinSize(container.Objects)
	assert.Equal(t, geo.NewSize(50, 50), layoutMin)

	// This has a dynamic minSize so we need to check again after layout!
	layout.Layout(container.Objects, geo.NewSize(50, 75))
	layoutMin = layout.MinSize(container.Objects)
	assert.Equal(t, geo.NewSize(50, 100+theme.Padding()), layoutMin)
}

func TestGridLWrapLayout_Resize_LessThanMinSize(t *testing.T) {
	cellSize := geo.NewSize(50, 50)
	minSize := cellSize

	container := container.NewWithoutLayout(canvas.NewRectangle(color.NRGBA{0, 0, 0, 0}))
	l := layout.NewGridWrapLayout(cellSize)
	container.Resize(geo.NewSize(25, 25))

	layoutMin := l.MinSize(container.Objects)
	assert.Equal(t, minSize, layoutMin)
}
