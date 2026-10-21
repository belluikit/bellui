//go:build !nopartialrepaint && (partialrepaint || (!android && !ios && !js && !wasm && !mobile))

package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// fakeSizeable is a minimal SizeableCanvas: PartialDirtyRegion only needs
// impl.Size() (via RebuildSpatialIndex). As the canvas the driver finds for
// every object (regionDriver), it passes on what objects report, as a
// driver's canvas does: a placement change, and a refresh the way
// FreeDirtyTextures takes it from the queue.
type fakeSizeable struct {
	SizeableCanvas
	size geo.Size
	c    *Canvas
}

func (f *fakeSizeable) Size() geo.Size { return f.size }

func (f *fakeSizeable) MarkPlacementDirty(o core.CanvasObject) { f.c.MarkPlacementDirty(o) }

func (f *fakeSizeable) MarkMinSizeDirty(o core.CanvasObject) { f.c.MarkMinSizeDirty(o) }

func (f *fakeSizeable) SetDirty() {}

func (f *fakeSizeable) Refresh(o core.CanvasObject) { f.c.noteRefreshed(o) }

// regionDriver is the test driver, except that every object's canvas is c.
type regionDriver struct {
	bellui.Driver
	c core.Canvas
}

func (d *regionDriver) CanvasForObject(core.CanvasObject) core.Canvas { return d.c }

type regionApp struct {
	bellui.App
	d *regionDriver
}

func (a *regionApp) Driver() bellui.Driver { return a.d }

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
	c := newPartialCanvas(content, geo.NewSize(200, 200))

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

// newPartialCanvas returns a canvas showing content, which the current app's
// driver finds for every object until the test ends.
func newPartialCanvas(content core.CanvasObject, size geo.Size) *Canvas {
	c := &Canvas{}
	impl := &fakeSizeable{size: size, c: c}
	c.Initialize(impl, func() {})
	c.SetContentTreeAndFocusMgr(content)
	a := bellui.CurrentApp()
	bellui.SetCurrentApp(&regionApp{App: a, d: &regionDriver{Driver: a.Driver(), c: impl}})
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

	// Everything changing frame after frame keeps skipping.
	refresh(t, c, bg)
	spot.Move(geo.NewPos(70, 70))
	_, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.Equal(t, canvasSize, sz)
	assert.Equal(t, geo.NewPos(50, 50), node.lastPaintPos, "a second skipped frame must not walk")

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

// TestPartialDirtyRegion_BoxShadowReach pins that a box shadow's region is
// the blurred area the painters draw, not its bounds: moving it must clear
// the blur it drew outside them.
func TestPartialDirtyRegion_BoxShadowReach(t *testing.T) {
	test.NewTempApp(t)

	shadow := canvas.NewBoxShadow(color.Black, 5) // drawn 10 past each side
	shadow.OffsetY = 4
	shadow.Resize(geo.NewSize(20, 20))
	shadow.Move(geo.NewPos(50, 50))
	content := container.NewWithoutLayout(shadow)
	content.Resize(geo.NewSize(400, 400))
	c := newPartialCanvas(content, geo.NewSize(400, 400))
	c.PartialDirtyRegion()

	shadow.Move(geo.NewPos(200, 200))
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(40), "region should include the old blur")
	assert.LessOrEqual(t, pos.Y, float32(44), "region should include the old blur, offset")
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(230), "region should include the new blur")
	assert.GreaterOrEqual(t, pos.Y+sz.Height, float32(234), "region should include the new blur, offset")
}

// TestPartialDirtyRegion_LeftTree pins that an object leaving the render tree,
// hidden or removed, leaves its pixels in the region: no walk visits it any
// more, so nothing else would clear them.
func TestPartialDirtyRegion_LeftTree(t *testing.T) {
	test.NewTempApp(t)

	for name, leave := range map[string]func(c *bellui.Container, o core.CanvasObject){
		"hidden":  func(_ *bellui.Container, o core.CanvasObject) { o.Hide() },
		"removed": func(c *bellui.Container, o core.CanvasObject) { c.Remove(o) },
	} {
		t.Run(name, func(t *testing.T) {
			first := canvas.NewRectangle(color.White)
			first.Resize(geo.NewSize(20, 20))
			label := widget.NewLabel("gone")
			label.Resize(geo.NewSize(80, 30))
			label.Move(geo.NewPos(200, 200))
			content := container.NewWithoutLayout(first, label)
			content.Resize(geo.NewSize(400, 400))
			c := newPartialCanvas(content, geo.NewSize(400, 400))
			c.PartialDirtyRegion()

			leave(content, label)
			pos, sz, set := c.PartialDirtyRegion()
			require.True(t, set, "the object's pixels must be cleared")
			assert.LessOrEqual(t, pos.X, float32(200))
			assert.LessOrEqual(t, pos.Y, float32(200))
			assert.GreaterOrEqual(t, pos.X+sz.Width, float32(280))
			assert.GreaterOrEqual(t, pos.Y+sz.Height, float32(230))

			_, _, set = c.PartialDirtyRegion()
			assert.False(t, set, "and only once")
		})
	}
}

// TestPartialDirtyRegion_MarkDirtyRect pins that a rect a driver marks dirty,
// for something it paints that no canvas object stands for, joins the next
// region and only that one (#551).
func TestPartialDirtyRegion_MarkDirtyRect(t *testing.T) {
	test.NewTempApp(t)

	content := container.NewWithoutLayout(canvas.NewRectangle(color.White))
	content.Resize(geo.NewSize(400, 400))
	c := newPartialCanvas(content, geo.NewSize(400, 400))
	c.PartialDirtyRegion()

	c.MarkDirtyRect(geo.NewPos(100, 120), geo.NewSize(30, 20))
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(100))
	assert.LessOrEqual(t, pos.Y, float32(120))
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(130))
	assert.GreaterOrEqual(t, pos.Y+sz.Height, float32(140))

	_, _, set = c.PartialDirtyRegion()
	assert.False(t, set, "and only once")
}

// TestPartialDirtyRegion_TextOutsideItsBox pins #602: a text positioned but
// never resized, as the navigation drawer's captions were, still draws its
// glyphs, so refreshing it must clear and redraw where they are.
func TestPartialDirtyRegion_TextOutsideItsBox(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("File Transfer", color.White)
	text.Move(geo.NewPos(200, 200))
	content := container.NewWithoutLayout(text)
	content.Resize(geo.NewSize(400, 400))
	c := newPartialCanvas(content, geo.NewSize(400, 400))
	c.PartialDirtyRegion()

	text.Text = "Files"
	text.Refresh()
	min := canvas.NewText("File Transfer", color.White).MinSize()
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(200))
	assert.LessOrEqual(t, pos.Y, float32(200))
	assert.GreaterOrEqual(t, pos.X+sz.Width, 200+min.Width, "the old glyphs' width is cleared")
	assert.GreaterOrEqual(t, pos.Y+sz.Height, 200+min.Height, "and their height")
}

// TestPartialDirtyRegion_HiddenOnlyChild pins that hiding the only visible
// child of a container clears it, as hiding one of several does: the walk
// visited no child there, and kept the old one's node, so its pixels stayed
// and it stayed mapped.
func TestPartialDirtyRegion_HiddenOnlyChild(t *testing.T) {
	test.NewTempApp(t)

	child := canvas.NewRectangle(color.White)
	child.Move(geo.NewPos(10, 10))
	child.Resize(geo.NewSize(20, 20))
	inner := container.NewWithoutLayout(child)
	inner.Resize(geo.NewSize(50, 50))
	content := container.NewWithoutLayout(inner)
	content.Resize(geo.NewSize(100, 100))
	c := newPartialCanvas(content, geo.NewSize(100, 100))
	c.PartialDirtyRegion()

	child.Hide()
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set, "the hidden child's pixels must be cleared")
	assert.LessOrEqual(t, pos.X, float32(10))
	assert.LessOrEqual(t, pos.Y, float32(10))
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(30))
	assert.GreaterOrEqual(t, pos.Y+sz.Height, float32(30))
	_, ok := c.NodeFor(child)
	assert.False(t, ok, "and its node forgotten")
	node, _ := c.NodeFor(inner)
	assert.Nil(t, node.firstChild)
}
