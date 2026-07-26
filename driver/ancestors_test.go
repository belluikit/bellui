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
)

func TestAncestorsOfObject_RootToLeafChain(t *testing.T) {
	test.NewTempApp(t)

	leaf := canvas.NewRectangle(color.Black)
	inner := container.NewWithoutLayout(leaf)
	content := container.NewWithoutLayout(inner)

	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(geo.NewSize(200, 200))

	path, ok := driver.AncestorsOfObject(w.Canvas(), leaf)
	require.True(t, ok)
	require.NotEmpty(t, path)

	// First element is the canvas root, last is the queried object, and the two
	// wrapping containers appear in between in top-down order.
	assert.Same(t, w.Canvas().Content(), path[0])
	assert.Same(t, leaf, path[len(path)-1])

	innerIdx, leafIdx := -1, -1
	for i, o := range path {
		if o == core.CanvasObject(inner) {
			innerIdx = i
		}
		if o == core.CanvasObject(leaf) {
			leafIdx = i
		}
	}
	require.NotEqual(t, -1, innerIdx, "inner container must be on the path")
	assert.Less(t, innerIdx, leafIdx, "ancestor precedes descendant")
}

func TestAncestorsOfObject_NotFound(t *testing.T) {
	test.NewTempApp(t)

	w := test.NewWindow(container.NewWithoutLayout())
	defer w.Close()

	orphan := canvas.NewRectangle(color.White)
	_, ok := driver.AncestorsOfObject(w.Canvas(), orphan)
	assert.False(t, ok, "an object outside the tree is not found")

	_, ok = driver.AncestorsOfObject(nil, orphan)
	assert.False(t, ok, "nil canvas is handled")

	_, ok = driver.AncestorsOfObject(w.Canvas(), nil)
	assert.False(t, ok, "nil object is handled")
}
