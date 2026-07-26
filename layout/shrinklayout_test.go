package layout_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"

	"github.com/stretchr/testify/assert"
)

func TestShrinkLayout_MinSize(t *testing.T) {
	l := layout.NewShrinkLayout()

	// MinSize should always be (0, 0) regardless of children.
	r1 := canvas.NewRectangle(color.Black)
	r1.SetMinSize(geo.NewSize(100, 50))
	r2 := canvas.NewRectangle(color.Black)
	r2.SetMinSize(geo.NewSize(200, 80))

	min := l.MinSize([]core.CanvasObject{r1, r2})
	assert.Equal(t, float32(0), min.Width)
	assert.Equal(t, float32(0), min.Height)
}

func TestShrinkLayout_MinSize_NoChildren(t *testing.T) {
	l := layout.NewShrinkLayout()
	min := l.MinSize(nil)
	assert.Equal(t, float32(0), min.Width)
	assert.Equal(t, float32(0), min.Height)
}

func TestShrinkLayout_Layout(t *testing.T) {
	l := layout.NewShrinkLayout()
	r := canvas.NewRectangle(color.Black)
	r.SetMinSize(geo.NewSize(50, 30))

	size := geo.NewSize(200, 100)
	l.Layout([]core.CanvasObject{r}, size)

	assert.Equal(t, geo.NewPos(0, 0), r.Position())
	assert.Equal(t, size, r.Size())
}

func TestShrinkLayout_Layout_MultipleChildren(t *testing.T) {
	l := layout.NewShrinkLayout()
	r1 := canvas.NewRectangle(color.Black)
	r2 := canvas.NewRectangle(color.White)

	size := geo.NewSize(300, 150)
	l.Layout([]core.CanvasObject{r1, r2}, size)

	assert.Equal(t, geo.NewPos(0, 0), r1.Position())
	assert.Equal(t, size, r1.Size())
	assert.Equal(t, geo.NewPos(0, 0), r2.Position())
	assert.Equal(t, size, r2.Size())
}

func TestShrinkLayout_Layout_Empty(t *testing.T) {
	l := layout.NewShrinkLayout()
	assert.NotPanics(t, func() {
		l.Layout(nil, geo.NewSize(100, 100))
	})
}
