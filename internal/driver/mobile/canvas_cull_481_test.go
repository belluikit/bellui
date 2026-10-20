//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
)

// objectRecordingPainter records which objects were painted.
type objectRecordingPainter struct {
	clipRecordingPainter
	painted map[core.CanvasObject]bool
}

func (p *objectRecordingPainter) Paint(obj core.CanvasObject, _ geo.Position, _ geo.Size) {
	p.painted[obj] = true
}

// TestPaintCullsOffClipKeepsNodes pins #481: the mobile paint walk skips the
// subtree of a clip-owner scrolled out of view, as the desktop one does
// (#464), sets up no clip for that clip-owner, and does not sweep the node
// map, so the skipped objects keep their nodes for the next walk that reaches
// them.
func TestPaintCullsOffClipKeepsNodes(t *testing.T) {
	items := make([]*roundedClipTestWidget, 100)
	objs := make([]core.CanvasObject, len(items))
	for i := range items {
		items[i] = newRoundedClipTestWidget()
		objs[i] = items[i]
	}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(container.NewVScroll(container.NewVBox(objs...)))
	c.Resize(geo.NewSize(100, 100))
	painter := &objectRecordingPainter{painted: map[core.CanvasObject]bool{}}
	c.SetPainter(painter)
	c.WalkTrees(nil, nil) // a full walk gives every object its node

	first, last := items[0].content, items[len(items)-1].content
	require.Greater(t, items[len(items)-1].Position().Y, float32(1000), "the last item lies far below the 100 px view")
	walk := &mobileTreeWalker{c: c, clips: &internal.ClipStack{}, size: geo.NewSize(100, 100)}
	walk.paint()

	assert.True(t, painter.painted[first], "an item in view is painted")
	assert.False(t, painter.painted[last], "an item scrolled out of view is not")
	_, ok := c.NodeFor(last)
	assert.True(t, ok, "the culled item's content keeps its node")

	drawn, starts, stops := 0, 0, 0
	for _, it := range items {
		if painter.painted[it.content] {
			drawn++
		}
	}
	for _, call := range painter.calls {
		switch call {
		case "startRounded 8":
			starts++
		case "stopRounded":
			stops++
		}
	}
	assert.Equal(t, drawn, starts, "only the items drawn set up their rounded clip")
	assert.Equal(t, starts, stops, "every clip set up is torn down")
}
