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

// windowSizedCanvas is the minimal SizeableCanvas RebuildSpatialIndex needs,
// at a full-HD window's size.
type windowSizedCanvas struct {
	SizeableCanvas
	size geo.Size
}

func (s *windowSizedCanvas) Size() geo.Size { return s.size }

func noWalk(*RenderCacheNode, geo.Position) {}

// TestRebuildSpatialIndexReusesBuffers guards #435: rebuilding the hit-test
// index every paint must not allocate once the buffers have grown, must
// alternate between two indexes (so the one a reader just loaded is not
// rewritten by the next rebuild), and must not keep entries from the previous
// build when it reuses a buffer.
func TestRebuildSpatialIndexReusesBuffers(t *testing.T) {
	test.NewTempApp(t)

	bg := canvas.NewRectangle(color.Black) // covers every cell, like a window background
	bg.Resize(geo.NewSize(1920, 1080))
	moved := canvas.NewRectangle(color.White)
	moved.Resize(geo.NewSize(40, 40))
	moved.Move(geo.NewPos(100, 100))
	content := container.NewWithoutLayout(bg, moved)
	content.Resize(geo.NewSize(1920, 1080))

	c := &Canvas{}
	c.Initialize(&windowSizedCanvas{size: geo.NewSize(1920, 1080)}, func() {})
	c.SetContentTreeAndFocusMgr(content)

	c.WalkTrees(noWalk, noWalk)
	first := c.SpatialIndex()
	require.NotNil(t, first)
	hit, _, ok := first.FindObjectAtPositionMatching(geo.NewPos(110, 110), matchAll)
	require.True(t, ok)
	assert.Same(t, moved, hit)

	moved.Move(geo.NewPos(500, 500))
	c.WalkTrees(noWalk, noWalk)
	second := c.SpatialIndex()
	assert.NotSame(t, first, second, "a rebuild must not rewrite the index just published")
	hit, _, _ = first.FindObjectAtPositionMatching(geo.NewPos(110, 110), matchAll)
	assert.Same(t, moved, hit, "the previous index stays intact through one rebuild")

	c.WalkTrees(noWalk, noWalk)
	third := c.SpatialIndex()
	assert.Same(t, first, third, "the two buffers alternate")
	hit, _, _ = third.FindObjectAtPositionMatching(geo.NewPos(110, 110), matchAll)
	assert.Same(t, bg, hit, "a reused buffer keeps no entry from its previous build")
	hit, _, _ = third.FindObjectAtPositionMatching(geo.NewPos(510, 510), matchAll)
	assert.Same(t, moved, hit)

	allocs := testing.AllocsPerRun(20, c.RebuildSpatialIndex)
	assert.Zero(t, allocs, "a steady-state rebuild allocates")
}

// TestSpatialIndexCellOrderIsInsertionOrder: within a cell, entries come back in
// the order they were added (DFS order), which "last match wins" relies on,
// including for entries that span different cell ranges.
func TestSpatialIndexCellOrderIsInsertionOrder(t *testing.T) {
	objs := make([]core.CanvasObject, 4)
	b := newSpatialIndexBuilder(800, 600)
	for i := range objs {
		r := canvas.NewRectangle(color.White)
		objs[i] = r
		// Each starts further left and is wider, so they overlap cell (2,2)
		// while covering different cell ranges.
		x := float32(150 - 40*i)
		r.Resize(geo.NewSize(30+float32(60*i), 30))
		b.add(r, geo.NewTranslation(x, 140), r.Size().Width, r.Size().Height, geo.NewPos(0, 0), geo.NewSize(800, 600))
	}
	idx := b.build()

	cell, ok := idx.cell(geo.NewPos(150, 140))
	require.True(t, ok)
	require.Len(t, cell, len(objs))
	for i, e := range cell {
		assert.Same(t, objs[i], idx.entries[e].obj, "position %d", i)
	}

	all := idx.FindAllObjectsAtPositionMatching(geo.NewPos(155, 145), matchAll)
	require.Len(t, all, len(objs))
	assert.Same(t, objs[3], all[0].Object, "deepest (last added) first")
}

// TestWalkTreesStaticDoesNotAllocate: walking an unchanged tree, which every
// paint does, allocates nothing, at any object count (#435). Re-storing each
// node's existing nodeMap entry cost one allocation per object per walk.
func TestWalkTreesStaticDoesNotAllocate(t *testing.T) {
	test.NewTempApp(t)

	objs := make([]core.CanvasObject, 1000)
	for i := range objs {
		r := canvas.NewRectangle(color.White)
		r.Resize(geo.NewSize(10, 10))
		r.Move(geo.NewPos(float32(i%100)*12, float32(i/100)*12))
		objs[i] = r
	}
	content := container.NewWithoutLayout(objs...)
	content.Resize(geo.NewSize(1920, 1080))

	c := &Canvas{}
	c.Initialize(&windowSizedCanvas{size: geo.NewSize(1920, 1080)}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.WalkTrees(noWalk, noWalk)

	assert.Zero(t, testing.AllocsPerRun(20, func() { c.WalkTrees(noWalk, noWalk) }))
}
