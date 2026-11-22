package layout_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
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

// TestRowWrapLayout_RTL: under RTL each row starts at the right edge and
// fills leftwards, including the second row a wrap starts (#1372).
func TestRowWrapLayout_RTL(t *testing.T) {
	rtlLayout(t, func() {
		gap := float32(4)
		objs := []core.CanvasObject{
			NewMinSizeRect(geo.NewSize(50, 20)),
			NewMinSizeRect(geo.NewSize(30, 20)),
			NewMinSizeRect(geo.NewSize(40, 20)),
		}
		// 50+4+30 fit in 100; the third wraps.
		layout.NewRowWrapLayoutWithCustomPadding(gap, gap).Layout(objs, geo.NewSize(100, 100))

		assert.Equal(t, geo.NewPos(50, 0), objs[0].Position(), "first object at the right edge")
		assert.Equal(t, geo.NewPos(100-50-gap-30, 0), objs[1].Position(), "second to its left")
		assert.Equal(t, geo.NewPos(60, 20+gap), objs[2].Position(), "the wrapped row starts at the right too")
		assert.Equal(t, geo.NewSize(30, 20), objs[1].Size(), "sizes are unchanged")
	})
}

// TestGridWrapLayout_RTL: under RTL the cells fill each row from the right
// edge, and the wrapped row does too (#1372).
func TestGridWrapLayout_RTL(t *testing.T) {
	rtlLayout(t, func() {
		p := theme.Padding()
		cell := geo.NewSize(50, 50)
		objs := []core.CanvasObject{NewMinSizeRect(cell), NewMinSizeRect(cell), NewMinSizeRect(cell)}
		width := 2*cell.Width + p + 10 // two columns, with slack
		layout.NewGridWrapLayout(cell).Layout(objs, geo.NewSize(width, 200))

		assert.Equal(t, geo.NewPos(width-50, 0), objs[0].Position())
		assert.Equal(t, geo.NewPos(width-50-p-50, 0), objs[1].Position())
		assert.Equal(t, geo.NewPos(width-50, 50+p), objs[2].Position())
	})
}

// TestFlexHBoxLayout_RTL: under RTL the first child sits at the right edge,
// and a flex child still takes the rest of the row (#1372).
func TestFlexHBoxLayout_RTL(t *testing.T) {
	rtlLayout(t, func() {
		a := NewMinSizeRect(geo.NewSize(30, 20))
		b := NewMinSizeRect(geo.NewSize(10, 20))
		l := &layout.FlexHBoxLayout{Flex: []float32{0, 1}, Padding: 4, ExactPadding: true}
		l.Layout([]core.CanvasObject{a, b}, geo.NewSize(100, 20))

		assert.Equal(t, geo.NewPos(70, 0), a.Position())
		assert.Equal(t, geo.NewPos(0, 0), b.Position())
		assert.Equal(t, float32(66), b.Size().Width, "the flex child fills what is left")
	})
}

// TestFlexVBoxLayout_RTL: CrossStart is the leading edge, which under RTL
// is the right one, and CrossEnd the left (#1372).
func TestFlexVBoxLayout_RTL(t *testing.T) {
	rtlLayout(t, func() {
		for align, wantX := range map[layout.CrossAlign]float32{
			layout.CrossStart:   70,
			layout.CrossCenter:  35,
			layout.CrossEnd:     0,
			layout.CrossStretch: 0,
		} {
			a := NewMinSizeRect(geo.NewSize(30, 20))
			(&layout.FlexVBoxLayout{CrossAlign: align}).Layout([]core.CanvasObject{a}, geo.NewSize(100, 100))
			assert.Equal(t, wantX, a.Position().X, "CrossAlign %d", align)
		}
	})
}
