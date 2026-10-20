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
	refresh(t, c, objA)
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

// refresh does for PartialDirtyRegion what FreeDirtyTextures does for an
// object taken from the refresh queue.
func refresh(t *testing.T, c *Canvas, objs ...core.CanvasObject) {
	t.Helper()
	for _, o := range objs {
		node, ok := c.NodeFor(o)
		require.True(t, ok, "object has no node")
		c.notePaintRefresh(node)
	}
}

func newPartialCanvas(content core.CanvasObject, size geo.Size) *Canvas {
	c := &Canvas{}
	c.Initialize(&fakeSizeable{size: size}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	return c
}

// TestPartialDirtyRegion_SkipsWalkWhenRefreshCoversCanvas pins the shortcut
// that keeps a frame where everything changes as cheap as a full repaint
// (#496): refreshed objects whose previous bounds cover the canvas make the
// whole canvas dirty without a walk, and the next frame walks to record the
// bounds again and repaints fully too, before regions are partial again.
func TestPartialDirtyRegion_SkipsWalkWhenRefreshCoversCanvas(t *testing.T) {
	test.NewTempApp(t)

	bg := canvas.NewRectangle(color.White)
	bg.Resize(geo.NewSize(200, 200))
	spot := canvas.NewRectangle(color.White)
	spot.Resize(geo.NewSize(10, 10))
	spot.Move(geo.NewPos(50, 50))
	content := container.NewWithoutLayout(bg, spot)
	content.Resize(geo.NewSize(200, 200))
	canvasSize := geo.NewSize(200, 200)
	c := newPartialCanvas(content, canvasSize)
	c.PartialDirtyRegion() // first frame records the bounds

	// bg covers the canvas: the frame is full, and no walk sees spot move.
	refresh(t, c, bg)
	spot.Move(geo.NewPos(60, 60))
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.Equal(t, geo.Position{}, pos)
	assert.Equal(t, canvasSize, sz)
	node, _ := c.NodeFor(spot)
	assert.Equal(t, geo.NewPos(50, 50), node.lastPaintPos, "the skipped frame must not walk")

	// spot moving back must not hide behind the stale bounds (50,50): this
	// frame repaints fully, and records the bounds.
	spot.Move(geo.NewPos(50, 50))
	pos, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.Equal(t, geo.Position{}, pos)
	assert.Equal(t, canvasSize, sz)

	_, _, set = c.PartialDirtyRegion()
	assert.False(t, set, "with the bounds recorded, an unchanged frame is clean")

	refresh(t, c, spot)
	_, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.Less(t, sz.Width, float32(50), "a small refresh is partial again")
}

// TestPartialDirtyRegion_AfterFullRepaint pins that a full repaint the driver
// did without asking for a region (a cleared canvas texture, a resize) makes
// the next region the whole canvas: an object that moved during it and back
// would otherwise match its recorded bounds and keep the pixels it left.
func TestPartialDirtyRegion_AfterFullRepaint(t *testing.T) {
	test.NewTempApp(t)

	obj := canvas.NewRectangle(color.White)
	obj.Resize(geo.NewSize(20, 20))
	content := container.NewWithoutLayout(obj)
	content.Resize(geo.NewSize(200, 200))
	canvasSize := geo.NewSize(200, 200)
	c := newPartialCanvas(content, canvasSize)
	c.PartialDirtyRegion()

	obj.Move(geo.NewPos(100, 100))
	c.MarkFullRepaint() // painted at (100,100) without a region
	obj.Move(geo.NewPos(0, 0))

	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.Equal(t, geo.Position{}, pos)
	assert.Equal(t, canvasSize, sz)
}

// TestPartialDirtyRegion_KeepsWalkGeneration pins that the region walk is not
// a full walk: one would start a node-map generation every frame, making every
// node look itself up again (#487), which was most of partial repaint's extra
// CPU (#496).
func TestPartialDirtyRegion_KeepsWalkGeneration(t *testing.T) {
	test.NewTempApp(t)

	obj := canvas.NewRectangle(color.White)
	obj.Resize(geo.NewSize(20, 20))
	content := container.NewWithoutLayout(obj)
	content.Resize(geo.NewSize(200, 200))
	c := newPartialCanvas(content, geo.NewSize(200, 200))

	gen := c.walkGen
	c.PartialDirtyRegion()
	obj.Move(geo.NewPos(10, 10))
	_, _, set := c.PartialDirtyRegion()
	assert.True(t, set, "the walk still sees the move")
	assert.Equal(t, gen, c.walkGen)
}

// TestPartialDirtyRegion_CulledClipOwner pins that the walk culls what the
// paint walk culls (#496) and still clears what a culled subtree drew: the
// children of a clip-owner that leaves its clip may draw outside it (shadows
// with the stencil off), so their last bounds join the region and are
// forgotten, and when the clip-owner comes back they count as new.
func TestPartialDirtyRegion_CulledClipOwner(t *testing.T) {
	test.NewTempApp(t)

	// Overflows the clip-owner by 10 on each side, as a shadow would.
	child := canvas.NewRectangle(color.White)
	child.Resize(geo.NewSize(50, 50))
	child.Move(geo.NewPos(-10, -10))
	owner := container.NewWithoutLayout(child)
	owner.ClipToBounds = true
	owner.Resize(geo.NewSize(30, 30))
	owner.Move(geo.NewPos(40, 40))
	clip := container.NewWithoutLayout(owner)
	clip.ClipToBounds = true
	clip.Resize(geo.NewSize(200, 200))
	content := container.NewWithoutLayout(clip)
	content.Resize(geo.NewSize(400, 400))
	c := newPartialCanvas(content, geo.NewSize(400, 400))
	c.PartialDirtyRegion()

	owner.Move(geo.NewPos(300, 300)) // off clip, beyond cullMargin
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(30), "region should include the child's last bounds")
	assert.LessOrEqual(t, pos.Y, float32(30), "region should include the child's last bounds")
	childNode, ok := c.NodeFor(child)
	require.True(t, ok)
	assert.False(t, childNode.lastPaintSet, "a culled child is not walked, and its bounds are forgotten")

	_, _, set = c.PartialDirtyRegion()
	assert.False(t, set, "a subtree that stays culled is clean")

	owner.Move(geo.NewPos(40, 40))
	pos, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(30), "region should include the child's new bounds")
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(80), "region should include the child's new bounds")
	assert.True(t, childNode.lastPaintSet)
}
