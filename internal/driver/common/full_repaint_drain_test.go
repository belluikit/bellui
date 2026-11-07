//go:build !nopartialrepaint && (partialrepaint || (!js && !wasm && !mobile && !(gl && (android || ios))))

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

// TestMarkFullRepaintDrainsPlacedNodes pins the leak behind daysong#365: a
// node added to the render tree is queued for the next partial-repaint
// region walk (addChild, queuePlaced), and only PartialDirtyRegion took the
// queue. A frame that repaints everything without asking for a region, which
// is every frame of a phone session after a background→resume marks the
// canvas FBO's retention unreliable (mobile driver.surfaceShown), called
// MarkFullRepaint instead, so the queue grew by every node added for the
// life of the canvas, each pinning its object: 26 MB in 26 minutes on a
// Pixel 9a playing DaySong, whose tab bar is rebuilt ten times a second.
func TestMarkFullRepaintDrainsPlacedNodes(t *testing.T) {
	test.NewTempApp(t)

	content := container.NewWithoutLayout()
	content.Resize(geo.NewSize(200, 200))
	c := newPartialCanvas(content, geo.NewSize(200, 200))
	c.PartialDirtyRegion() // the frames before the resume: regions, queue taken

	const frames = 50
	for i := 0; i < frames; i++ {
		// Each frame replaces the content's child, as a widget rebuilt on every
		// refresh does: the walk forgets the old node and adds one for the new
		// object, which queues it for a region.
		spot := canvas.NewRectangle(color.White)
		spot.Resize(geo.NewSize(10, 10))
		content.Objects = []core.CanvasObject{spot}
		c.WalkContentTreesCulled(c.CullOffClip, nil, nil) // the paint walk
		c.MarkFullRepaint()                               // a full clear, no region asked for
	}

	c.placed.mu.Lock()
	queued := len(c.placed.nodes)
	c.placed.mu.Unlock()
	assert.Zero(t, queued, "nodes queued for a region walk that full-repaint frames never make")

	// The frames' queueing also left the one live node unqueued, so the next
	// region, when retention is reliable again, queues it afresh.
	node, ok := c.NodeFor(content.Objects[0])
	require.True(t, ok)
	assert.False(t, node.placeQueued.Load(), "a drained node is no longer marked queued")
}
