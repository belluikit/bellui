package inspector

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestResolveObjectAt_DeepestUnderCursor(t *testing.T) {
	test.NewTempApp(t)

	leaf := canvas.NewRectangle(color.Black)
	leaf.Move(geo.NewPos(20, 20))
	leaf.Resize(geo.NewSize(60, 30))
	content := container.NewWithoutLayout(leaf)

	w := test.NewTempWindow(t, content)
	w.Resize(geo.NewSize(200, 200))

	obj, ok := resolveObjectAt(w.Canvas(), geo.NewPos(30, 30))
	require.True(t, ok)
	assert.Same(t, leaf, obj, "pick resolves the deepest object at the point")

	// A point outside any content object resolves to the content root or nothing,
	// but never panics.
	_, _ = resolveObjectAt(w.Canvas(), geo.NewPos(1, 1))
}

func TestNewHighlight_HiddenByDefault(t *testing.T) {
	test.NewTempApp(t)
	h := newHighlight()
	assert.False(t, h.Visible(), "highlight starts hidden until a node is selected")
	assert.Equal(t, float32(2), h.StrokeWidth)
}
