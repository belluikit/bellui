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
	"github.com/belluikit/bellui/widget"
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
// walk culled. Off the grid no lookup could reach them; on it, they lie outside
// their clip (#566), so a lookup there answers what the tree-walk hit test
// answers without them.
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
	assert.Zero(t, onChild.asked, "and one on the grid")

	idx := c.SpatialIndex()
	require.NotNil(t, idx)
	at := geo.NewPos(10, 60) // inside onChild
	want, _, _ := intdriver.FindObjectAtPositionMatching(at, matchAll, nil, content)
	got, _, _ := idx.FindObjectAtPositionMatching(at, matchAll)
	assert.Same(t, want, got, "the index answers what the tree walk answers")
}

// TestSpatialIndexNestedClip pins #566 for the hit-test index: a button in a
// horizontal scroll that the outer vertical scroll has moved up under a header
// is out of reach on the header, in the index as in the tree walk.
func TestSpatialIndexNestedClip(t *testing.T) {
	test.NewTempApp(t)

	button := widget.NewButton("hidden", func() {})
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(geo.NewSize(10, 1000))
	outer := container.NewVScroll(container.NewVBox(container.NewHScroll(button), spacer))
	header := canvas.NewRectangle(color.Black)
	header.SetMinSize(geo.NewSize(10, 40))
	content := container.NewBorder(header, nil, nil, nil, outer)
	content.Resize(geo.NewSize(300, 300))
	outer.ScrollToOffset(geo.NewPos(0, 30))

	c := &Canvas{}
	c.Initialize(&windowSizedCanvas{size: geo.NewSize(300, 300)}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.WalkTrees(noWalk, noWalk)
	c.RebuildSpatialIndex()
	idx := c.SpatialIndex()
	require.NotNil(t, idx)

	isButton := func(o core.CanvasObject) bool {
		_, ok := o.(*widget.Button)
		return ok
	}
	onHeader := geo.NewPos(5, 20)
	got, _, _ := idx.FindObjectAtPositionMatching(onHeader, isButton)
	assert.Nil(t, got, "the index does not reach the button under the header")
	want, _, _ := intdriver.FindObjectAtPositionMatching(onHeader, isButton, nil, content)
	assert.Nil(t, want, "nor does the tree walk")

	below := geo.NewPos(5, 50) // inside the outer scroll, on the button
	got, _, _ = idx.FindObjectAtPositionMatching(below, isButton)
	assert.Same(t, button, got)
}

// TestSpatialIndexSkipsCulledOnGrid pins that the index leaves out the
// children of every clip-owner the paint walk culled, on the grid too: the
// culled walk did not visit them, so their nodes keep where they were, and the
// owner misses its clip, so none of them can be hit (#566). The index then
// answers where a child used to be what the tree walk answers.
func TestSpatialIndexSkipsCulledOnGrid(t *testing.T) {
	test.NewTempApp(t)

	child := newVisibleCounter(geo.NewPos(5, 5))
	owner := clipOwner(geo.NewPos(0, 10), child)
	region := container.NewWithoutLayout(owner)
	region.ClipToBounds = true
	region.Resize(geo.NewSize(400, 100))
	region.Move(geo.NewPos(0, 200))
	content := container.NewWithoutLayout(region)
	content.Resize(geo.NewSize(400, 400))

	c := &Canvas{}
	c.Initialize(&windowSizedCanvas{size: geo.NewSize(400, 400)}, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.WalkContentTreesCulled(CullOffClip, noWalk, noWalk)
	c.RebuildSpatialIndex()
	at := geo.NewPos(10, 220) // on the child, at y 215
	got, _, _ := c.SpatialIndex().FindObjectAtPositionMatching(at, matchAll)
	require.Same(t, child, got)

	// Move the owner above the region, still on the canvas: the paint walk
	// culls it and leaves the child's node where it was.
	owner.Move(geo.NewPos(0, -150))
	c.WalkContentTreesCulled(CullOffClip, noWalk, noWalk)
	c.RebuildSpatialIndex()
	want, _, _ := intdriver.FindObjectAtPositionMatching(at, matchAll, nil, content)
	require.Same(t, region, want)
	got, _, _ = c.SpatialIndex().FindObjectAtPositionMatching(at, matchAll)
	assert.Same(t, want, got, "the index answers what the tree walk answers")
}
