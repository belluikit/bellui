package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// gridOfSized builds a VBox of rows, each an HBox of sized widgets.
func gridOfSized(rows, cols int) (core.CanvasObject, [][]*sizedWidget) {
	all := make([][]*sizedWidget, rows)
	objs := make([]core.CanvasObject, rows)
	for r := range rows {
		cells := make([]core.CanvasObject, cols)
		all[r] = make([]*sizedWidget, cols)
		for c := range cols {
			all[r][c] = newSizedWidget(geo.NewSize(20, 20))
			cells[c] = all[r][c]
		}
		objs[r] = container.NewHBox(cells...)
	}
	return container.NewVBox(objs...), all
}

// settle runs EnsureMinSize until the layout has nothing left to do.
func settle(c *canvas) {
	for range 3 {
		c.EnsureMinSize()
	}
}

// TestEnsureMinSizeWalksOnlyChangedPaths pins #487: with one widget refreshed,
// the pass visits the path to it, not the whole tree, and still lays out its
// new min size.
func TestEnsureMinSizeWalksOnlyChangedPaths(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	content, cells := gridOfSized(20, 20)
	c.SetContent(container.NewStack(content))
	c.Resize(geo.NewSize(800, 800))
	settle(c)

	c.EnsureMinSize()
	assert.Less(t, c.MinSizeVisits(), 10, "a pass with nothing refreshed walked %d nodes", c.MinSizeVisits())

	w := cells[7][3]
	w.min = geo.NewSize(30, 20)
	w.Refresh()
	c.EnsureMinSize()
	assert.Less(t, c.MinSizeVisits(), 60, "one refresh walked %d of more than 400 nodes", c.MinSizeVisits())
	assert.Equal(t, float32(30), w.Size().Width, "the refreshed widget's new min size was not laid out")
	assert.Equal(t, float32(20), cells[7][4].Size().Width)
	assert.Greater(t, cells[7][4].Position().X, w.Position().X+w.Size().Width-1, "its row was not re-laid out")
}

// TestEnsureMinSizeForgetsRemovedObjects checks that an object removed from a
// container leaves the node map in the pass that sees it go, although a pass
// that walks only changed paths does not sweep the map (#380, #487).
func TestEnsureMinSizeForgetsRemovedObjects(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	content, cells := gridOfSized(4, 4)
	c.SetContent(container.NewStack(content))
	c.Resize(geo.NewSize(400, 400))
	settle(c)

	row := content.(*bellui.Container).Objects[1].(*bellui.Container)
	gone := cells[1][2]
	_, ok := c.NodeFor(gone)
	assert.True(t, ok, "the cell should have a node before it is removed")

	row.Remove(gone)
	c.Refresh(row) // as the driver delivers the row's refresh to its canvas
	c.EnsureMinSize()
	assert.Less(t, c.MinSizeVisits(), 15, "this should have been a pass over changed paths only")
	_, ok = c.NodeFor(gone)
	assert.False(t, ok, "a removed object kept its node")
}

// TestEnsureMinSizeFullWalkAfterShow checks that showing a container, which
// had no node while hidden, makes the next pass walk everything and lay it out.
func TestEnsureMinSizeFullWalkAfterShow(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	hidden := container.NewHBox(newSizedWidget(geo.NewSize(40, 40)))
	hidden.Hide()
	other := newSizedWidget(geo.NewSize(40, 40))
	c.SetContent(container.NewStack(container.NewHBox(hidden, other)))
	c.Resize(geo.NewSize(400, 300))
	settle(c)
	assert.Zero(t, other.Position().X)

	hidden.Show()
	c.EnsureMinSize()
	assert.NotZero(t, other.Position().X, "the shown container took no room")
}
