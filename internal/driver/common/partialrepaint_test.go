//go:build partialrepaint

package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// fakeSizeable is a minimal SizeableCanvas: PartialDirtyRegion only needs
// impl.Size() (via RebuildSpatialIndex); the rest of the interface is unused.
type fakeSizeable struct {
	SizeableCanvas
	size geo.Size
}

func (f *fakeSizeable) Size() geo.Size { return f.size }

// TestPartialDirtyRegion verifies the heart of the dirty-region repair: a region
// that covers exactly what changed, includes both the old and new bounds of a
// moved object, and excludes untouched objects. Runs only under the
// partialrepaint build tag (the per-node bookkeeping is otherwise not kept).
func TestPartialDirtyRegion(t *testing.T) {
	test.NewTempApp(t)

	objA := canvas.NewRectangle(color.White)
	objA.Resize(geo.NewSize(20, 20))
	objA.Move(geo.NewPos(0, 0))
	objB := canvas.NewRectangle(color.White)
	objB.Resize(geo.NewSize(20, 20))
	objB.Move(geo.NewPos(100, 100))
	content := container.NewWithoutLayout(objA, objB)
	content.Resize(geo.NewSize(200, 200))

	c := &Canvas{}
	c.Initialize(&fakeSizeable{size: geo.NewSize(200, 200)}, func() {})
	c.SetContentTreeAndFocusMgr(content)

	// Frame 1: no recorded bounds yet, so every node is "moved" and the region
	// spans the content. This pass also records lastPaintBounds for next time.
	_, _, set := c.PartialDirtyRegion()
	require.True(t, set, "first frame should mark everything dirty")

	// Frame 2: nothing refreshed, nothing moved -> no dirty region.
	_, _, set = c.PartialDirtyRegion()
	assert.False(t, set, "an unchanged frame should produce no dirty region")

	// Frame 3: refresh only objA -> region covers objA and excludes objB.
	c.refreshedObjs = map[core.CanvasObject]struct{}{objA: {}}
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(0))
	assert.LessOrEqual(t, pos.Y, float32(0))
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(20))
	assert.GreaterOrEqual(t, pos.Y+sz.Height, float32(20))
	assert.Less(t, pos.X+sz.Width, float32(100), "region must not reach objB")
	assert.Less(t, pos.Y+sz.Height, float32(100), "region must not reach objB")

	// Frame 4: move objB -> region must include both where it was and where it
	// now is, so the old pixels are cleared and the new ones drawn.
	objB.Move(geo.NewPos(120, 120))
	pos, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(100), "region should include objB old position")
	assert.LessOrEqual(t, pos.Y, float32(100), "region should include objB old position")
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(140), "region should include objB new position")
	assert.GreaterOrEqual(t, pos.Y+sz.Height, float32(140), "region should include objB new position")
}
