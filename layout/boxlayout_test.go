package layout_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

// NewMinSizeRect returns a new Rectangle instance
func NewMinSizeRect(min geo.Size) *canvas.Rectangle {
	rect := &canvas.Rectangle{}
	rect.SetMinSize(min)

	return rect
}

func TestHBoxLayout_Simple(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(150+(theme.Padding()*2), 50))

	assert.Equal(t, obj1.Size(), cellSize)
	cell2Pos := geo.NewPos(50+theme.Padding(), 0)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(100+theme.Padding()*2, 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestCustomPaddedHBoxLayout_Simple(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj3 := NewMinSizeRect(cellSize)

	padding := float32(15)
	container := container.New(layout.NewCustomPaddedHBoxLayout(padding), obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(150+padding*2, 50))

	assert.Equal(t, obj1.Size(), cellSize)
	cell2Pos := geo.NewPos(50+padding, 0)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(100+padding*2, 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestHBoxLayout_HiddenItem(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj2.Hide()
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(100+theme.Padding(), 50))

	assert.Equal(t, obj1.Size(), cellSize)
	cell3Pos := geo.NewPos(50+theme.Padding(), 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestHBoxLayout_Wide(t *testing.T) {
	cellSize := geo.NewSize(50, 100)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(0, 25)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(obj1, obj2, obj3)
	container.Resize(geo.NewSize(308, 100))
	assert.Equal(t, geo.NewSize(150+(theme.Padding()*2), 100), container.MinSize())

	assert.Equal(t, float32(50), obj1.Size().Width)
	assert.Equal(t, float32(50), obj2.Size().Width)
	cell2Pos := geo.NewPos(50+theme.Padding(), 0)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(100+theme.Padding()*2, 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestHBoxLayout_Tall(t *testing.T) {
	cellSize := geo.NewSize(50, 100)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(0, 25)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(150+(theme.Padding()*2), 100))

	assert.Equal(t, obj1.Size(), cellSize)
	assert.Equal(t, obj2.Size(), cellSize)
	cell2Pos := geo.NewPos(50+theme.Padding(), 0)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(100+theme.Padding()*2, 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestHBoxLayout_Spacer(t *testing.T) {
	cellSize := geo.NewSize(50, 100)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(0, 25)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(layout.NewSpacer(), obj1, obj2, obj3)
	container.Resize(geo.NewSize(300, 100))
	assert.Equal(t, container.MinSize(), geo.NewSize(150+(theme.Padding()*2), 100))

	assert.Equal(t, float32(50), obj1.Size().Width)
	assert.Equal(t, float32(50), obj2.Size().Width)
	cell2Pos := geo.NewPos(200-theme.Padding(), 0)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(250, 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestHBoxLayout_MiddleSpacer(t *testing.T) {
	cellSize := geo.NewSize(50, 100)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(0, 25)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(obj1, obj2, layout.NewSpacer(), obj3)
	container.Resize(geo.NewSize(300, 100))
	assert.Equal(t, container.MinSize(), geo.NewSize(150+(theme.Padding()*2), 100))

	assert.Equal(t, float32(50), obj1.Size().Width)
	assert.Equal(t, float32(50), obj2.Size().Width)
	cell2Pos := geo.NewPos(50+theme.Padding(), 0)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(250, 0)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestVBoxLayout_Simple(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewVBox(obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(50, 150+(theme.Padding()*2)))

	assert.Equal(t, obj1.Size(), cellSize)
	cell2Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(0, 100+theme.Padding()*2)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestCustomPaddedVBoxLayout_Simple(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj3 := NewMinSizeRect(cellSize)

	padding := float32(15)
	container := container.New(layout.NewCustomPaddedVBoxLayout(padding), obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(50, 150+padding*2))

	assert.Equal(t, obj1.Size(), cellSize)
	cell2Pos := geo.NewPos(0, 50+padding)
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(0, 100+padding*2)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestVBoxLayout_HiddenItem(t *testing.T) {
	cellSize := geo.NewSize(50, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj2.Hide()
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewVBox(obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(50, 100+theme.Padding()))

	assert.Equal(t, obj1.Size(), cellSize)
	cell3Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestVBoxLayout_Wide(t *testing.T) {
	cellSize := geo.NewSize(100, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(25, 0)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewVBox(obj1, obj2, obj3)

	// We are not in a window. Resize the container to a default size.
	container.Resize(container.MinSize())

	assert.Equal(t, container.MinSize(), geo.NewSize(100, 150+(theme.Padding()*2)))

	assert.Equal(t, obj1.Size(), cellSize)
	assert.Equal(t, obj2.Size(), cellSize)
	cell2Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(0, 100+theme.Padding()*2)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestVBoxLayout_Tall(t *testing.T) {
	cellSize := geo.NewSize(100, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(25, 0)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewVBox(obj1, obj2, obj3)
	container.Resize(geo.NewSize(100, 308))
	assert.Equal(t, container.MinSize(), geo.NewSize(100, 150+(theme.Padding()*2)))

	assert.Equal(t, float32(50), obj1.Size().Height)
	assert.Equal(t, float32(50), obj2.Size().Height)
	cell2Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(0, 100+theme.Padding()*2)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestVBoxLayout_Spacer(t *testing.T) {
	cellSize := geo.NewSize(100, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(25, 0)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewVBox(layout.NewSpacer(), obj1, obj2, obj3)
	container.Resize(geo.NewSize(100, 300))
	assert.Equal(t, container.MinSize(), geo.NewSize(100, 150+(theme.Padding()*2)))

	assert.Equal(t, float32(50), obj1.Size().Height)
	assert.Equal(t, float32(50), obj2.Size().Height)
	cell2Pos := geo.NewPos(0, 200-theme.Padding())
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(0, 250)
	assert.Equal(t, cell3Pos, obj3.Position())
}

func TestVBoxLayout_MiddleSpacer(t *testing.T) {
	cellSize := geo.NewSize(100, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize.Subtract(geo.NewSize(25, 0)))
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewVBox(obj1, obj2, layout.NewSpacer(), obj3)
	container.Resize(geo.NewSize(100, 300))
	assert.Equal(t, container.MinSize(), geo.NewSize(100, 150+(theme.Padding()*2)))

	assert.Equal(t, float32(50), obj1.Size().Height)
	assert.Equal(t, float32(50), obj2.Size().Height)
	cell2Pos := geo.NewPos(0, 50+theme.Padding())
	assert.Equal(t, cell2Pos, obj2.Position())
	cell3Pos := geo.NewPos(0, 250)
	assert.Equal(t, cell3Pos, obj3.Position())
}

// Test for issue #4259 - spacer in HBox with hidden item causing add'l trailing padding
func TestHBoxLayout_MiddleSpacerHiddenItem(t *testing.T) {
	cellSize := geo.NewSize(100, 50)

	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj3 := NewMinSizeRect(cellSize)

	container := container.NewHBox(obj1, obj2, layout.NewSpacer(), obj3)
	container.Resize(geo.NewSize(400, 100))
	assert.Equal(t, geo.NewPos(300, 0), obj3.Position())

	obj2.Hide()
	container.Refresh()
	assert.Equal(t, geo.NewPos(300, 0), obj3.Position())
}
