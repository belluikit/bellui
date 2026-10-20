package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/theme"
)

// TestWalkConfirmsTheNodeMapOncePerGeneration pins the node map shortcut
// (#487): a walk looks an object's entry up only when it has not confirmed it
// in the current full-walk generation. An entry removed from the map comes
// back on the first walk of the next generation, which is when the sweep at
// the end of a full walk can have removed it. (Through the public API a node
// outlives its entry only defensively: an object a full walk does not reach
// is hidden, and its node is dropped too.)
func TestWalkConfirmsTheNodeMapOncePerGeneration(t *testing.T) {
	rect := canvas.NewRectangle(color.Black)
	tree := &renderCacheTree{root: &RenderCacheNode{obj: container.NewWithoutLayout(rect)}}
	c := &Canvas{}
	c.Initialize(nil, func() {})
	c.SetContentTreeAndFocusMgr(&canvas.Rectangle{FillColor: theme.Color(theme.ColorNameBackground)})

	c.walkTree(tree, nil, nil, nil, nil)
	node, ok := c.nodeMap.Load(rect)
	assert.True(t, ok, "the first walk did not map the object")

	// Within a generation the walk trusts its confirmation.
	c.nodeMap.Delete(rect)
	c.walkTree(tree, nil, nil, nil, nil)
	_, ok = c.nodeMap.Load(rect)
	assert.False(t, ok, "the walk looked up an entry it had confirmed this generation")

	// A new generation confirms it again, and reuses the node.
	c.walkGen++
	c.walkTree(tree, nil, nil, nil, nil)
	again, ok := c.nodeMap.Load(rect)
	assert.True(t, ok, "the first walk of a new generation did not restore the entry")
	assert.Same(t, node, again)
}
