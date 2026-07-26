package layout_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// rtlLayout runs fn with the process-wide layout direction forced to RTL and
// restores the default afterwards.
func rtlLayout(t *testing.T, fn func()) {
	t.Helper()
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
	fn()
}

func TestHBoxLayout_RTL(t *testing.T) {
	cellSize := geo.NewSize(50, 50)
	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)
	obj3 := NewMinSizeRect(cellSize)

	rtlLayout(t, func() {
		c := container.NewHBox(obj1, obj2, obj3)
		c.Resize(c.MinSize())
		w := c.Size().Width

		// First child is now at the right edge, last child at the left edge.
		assert.Equal(t, geo.NewPos(w-50, 0), obj1.Position())
		assert.Equal(t, geo.NewPos(0, 0), obj3.Position())
		// Middle child sits between, mirrored.
		assert.Equal(t, geo.NewPos(50+theme.Padding(), 0), obj2.Position())
		// Sizes are unchanged by mirroring.
		assert.Equal(t, cellSize, obj1.Size())
	})
}

func TestHBoxLayout_LTRUnchanged(t *testing.T) {
	// With the default (auto, non-RTL test locale) the layout is identical to
	// the historical behaviour: first child at the left edge.
	cellSize := geo.NewSize(50, 50)
	obj1 := NewMinSizeRect(cellSize)
	obj2 := NewMinSizeRect(cellSize)

	c := container.NewHBox(obj1, obj2)
	c.Resize(c.MinSize())

	assert.Equal(t, geo.NewPos(0, 0), obj1.Position())
	assert.Equal(t, geo.NewPos(50+theme.Padding(), 0), obj2.Position())
}

func TestBorderLayout_RTL(t *testing.T) {
	left := NewMinSizeRect(geo.NewSize(20, 100))
	right := NewMinSizeRect(geo.NewSize(30, 100))
	middle := NewMinSizeRect(geo.NewSize(10, 10))

	rtlLayout(t, func() {
		c := container.New(layout.NewBorderLayout(nil, nil, left, right), left, right, middle)
		c.Resize(geo.NewSize(200, 100))

		// left object moves to the right edge, right object to the left edge.
		assert.Equal(t, float32(0), right.Position().X, "right edge object mirrored to x=0")
		assert.Equal(t, float32(200-20), left.Position().X, "left edge object mirrored to right")
		// Middle fills the gap between them, mirrored.
		assert.Equal(t, right.Size().Width+theme.Padding(), middle.Position().X)
	})
}

func TestGridLayout_RTL(t *testing.T) {
	obj1 := NewMinSizeRect(geo.NewSize(50, 50))
	obj2 := NewMinSizeRect(geo.NewSize(50, 50))

	rtlLayout(t, func() {
		c := container.New(layout.NewGridLayoutWithColumns(2), obj1, obj2)
		c.Resize(geo.NewSize(100+theme.Padding(), 50))

		// Columns flip: first item on the right, second on the left.
		assert.Equal(t, float32(0), obj2.Position().X, "second column mirrored to the left")
		assert.Greater(t, obj1.Position().X, obj2.Position().X, "first column is to the right of the second")
	})
}

func TestFormLayout_RTL(t *testing.T) {
	label := NewMinSizeRect(geo.NewSize(40, 30))
	content := NewMinSizeRect(geo.NewSize(60, 30))

	rtlLayout(t, func() {
		c := container.New(layout.NewFormLayout(), label, content)
		c.Resize(c.MinSize())

		// Label column moves to the right, content column to the left.
		assert.Greater(t, label.Position().X, content.Position().X,
			"label sits to the right of the content under RTL")
		assert.Equal(t, float32(0), content.Position().X, "content mirrored to the left edge")
	})
}
