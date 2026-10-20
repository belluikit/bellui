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
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
)

// visibleCounter is a rectangle that counts the walks that ask whether it is
// visible.
type visibleCounter struct {
	*canvas.Rectangle
	asked int
}

func (v *visibleCounter) Visible() bool {
	v.asked++
	return v.Rectangle.Visible()
}

func newVisibleCounter(pos geo.Position) *visibleCounter {
	v := &visibleCounter{Rectangle: canvas.NewRectangle(color.White)}
	v.Resize(geo.NewSize(40, 40))
	v.Move(pos)
	return v
}

// clipOwner is a ClipToBounds container at pos holding child.
func clipOwner(pos geo.Position, child core.CanvasObject) *bellui.Container {
	c := container.NewWithoutLayout(child)
	c.ClipToBounds = true
	c.Resize(geo.NewSize(100, 50))
	c.Move(pos)
	return c
}

// TestSpatialIndexSkipsCulledOffGrid pins #481: rebuilding the hit-test index
// after a culled paint walk does not visit the children of a clip-owner the
// walk culled that lies off the grid, where no lookup can reach them. A culled
// clip-owner still on the grid keeps its children in the index, so a lookup
// there answers what the tree-walk hit test answers.
func TestSpatialIndexSkipsCulledOffGrid(t *testing.T) {
	test.NewTempApp(t)

	offChild, onChild := newVisibleCounter(geo.NewPos(5, 5)), newVisibleCounter(geo.NewPos(5, 5))
	// The clip region is y 200 to 300. One owner lies far below the 400 px
	// canvas, the other above the region but on the canvas, at y 50.
	region := container.NewWithoutLayout(
		clipOwner(geo.NewPos(0, 2000), offChild),
		clipOwner(geo.NewPos(0, -150), onChild),
	)
	region.ClipToBounds = true
	region.Resize(geo.NewSize(400, 100))
	region.Move(geo.NewPos(0, 200))
	content := container.NewWithoutLayout(region)
	content.Resize(geo.NewSize(400, 400))

	c := &Canvas{}
	c.Initialize(&windowSizedCanvas{size: geo.NewSize(400, 400)}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.WalkTrees(noWalk, noWalk) // a full walk reaches every object

	offChild.asked, onChild.asked = 0, 0
	c.WalkContentTreesCulled(CullOffClip, noWalk, noWalk)
	c.RebuildSpatialIndex()
	assert.Zero(t, offChild.asked, "the paint walk and the index skip a culled clip-owner off the grid")
	assert.NotZero(t, onChild.asked, "the index still visits a culled clip-owner on the grid")

	idx := c.SpatialIndex()
	require.NotNil(t, idx)
	at := geo.NewPos(10, 60) // inside onChild
	want, _, _ := intdriver.FindObjectAtPositionMatching(at, matchAll, nil, content)
	got, _, _ := idx.FindObjectAtPositionMatching(at, matchAll)
	assert.Same(t, want, got, "the index answers what the tree walk answers")
}
