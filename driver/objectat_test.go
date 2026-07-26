package driver_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestObjectAtPosition_DeepestWithBounds(t *testing.T) {
	test.NewTempApp(t)

	leaf := canvas.NewRectangle(color.Black)
	leaf.Move(geo.NewPos(20, 20))
	leaf.Resize(geo.NewSize(60, 30))
	content := container.NewWithoutLayout(leaf)

	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	// A point inside the leaf resolves to the leaf, with bounds sized to the
	// leaf and containing the queried point. (The absolute origin includes the
	// window content inset applied by the test canvas, so it is not hardcoded.)
	pick := geo.NewPos(30, 30)
	obj, bounds, ok := driver.ObjectAtPosition(w.Canvas(), pick)
	require.True(t, ok)
	assert.Same(t, leaf, obj)
	assert.Equal(t, geo.NewSize(60, 30), bounds.Size)
	assert.GreaterOrEqual(t, pick.X, bounds.Pos.X)
	assert.GreaterOrEqual(t, pick.Y, bounds.Pos.Y)
	assert.Less(t, pick.X, bounds.Pos.X+bounds.Size.Width)
	assert.Less(t, pick.Y, bounds.Pos.Y+bounds.Size.Height)

	// A point in empty space still resolves the content root or nothing —
	// never panics — and reports consistent ok/obj.
	obj, _, ok = driver.ObjectAtPosition(w.Canvas(), geo.NewPos(150, 150))
	assert.Equal(t, ok, obj != nil)
}

func TestObjectAtPositionMatching_SkipsRejected(t *testing.T) {
	test.NewTempApp(t)

	// A button whose renderer paints internal primitives; the deepest object at
	// a point is a renderer internal, but the predicate can pull back to the
	// widget the caller cares about.
	btn := widget.NewButton("Click", nil)
	btn.Move(geo.NewPos(10, 10))
	btn.Resize(geo.NewSize(120, 40))
	content := container.NewWithoutLayout(btn)

	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	center := geo.NewPos(40, 25)

	deepest, _, ok := driver.ObjectAtPosition(w.Canvas(), center)
	require.True(t, ok)

	onlyButton := func(o core.CanvasObject) bool { _, is := o.(*widget.Button); return is }
	matched, bounds, ok := driver.ObjectAtPositionMatching(w.Canvas(), center, onlyButton)
	require.True(t, ok)
	assert.Same(t, btn, matched)
	assert.NotSame(t, deepest, matched, "unfiltered pick lands on a renderer internal, not the button")
	assert.Equal(t, geo.NewSize(120, 40), bounds.Size)
}

func TestObjectAtPosition_OverlayAboveContent(t *testing.T) {
	test.NewTempApp(t)

	base := canvas.NewRectangle(color.Black)
	base.Resize(geo.NewSize(100, 100))
	w := test.NewWindow(base)
	defer w.Close()
	w.Resize(geo.NewSize(100, 100))

	over := canvas.NewRectangle(color.White)
	over.Resize(geo.NewSize(100, 100))
	w.Canvas().Overlays().Add(over)

	obj, _, ok := driver.ObjectAtPosition(w.Canvas(), geo.NewPos(50, 50))
	require.True(t, ok)
	assert.Same(t, over, obj, "overlays are hit-tested above content")
}

func TestObjectAtPosition_NilSafe(t *testing.T) {
	obj, _, ok := driver.ObjectAtPosition(nil, geo.NewPos(0, 0))
	assert.False(t, ok)
	assert.Nil(t, obj)

	test.NewTempApp(t)
	w := test.NewWindow(widget.NewLabel("x"))
	defer w.Close()
	_, _, ok = driver.ObjectAtPositionMatching(w.Canvas(), geo.NewPos(0, 0), nil)
	assert.False(t, ok)
}
