package inspector

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// showInspector builds a target window with a single known rectangle and opens
// an inspector against it, returning both plus the leaf object.
func showInspector(t *testing.T) (*Inspector, *canvas.Rectangle) {
	t.Helper()
	test.NewTempApp(t)

	leaf := canvas.NewRectangle(color.Black)
	leaf.Move(geo.NewPos(20, 20))
	leaf.Resize(geo.NewSize(60, 30))
	target := test.NewTempWindow(t, container.NewWithoutLayout(leaf))
	target.Resize(geo.NewSize(200, 200))

	ins := Show(target)
	t.Cleanup(func() { ins.win.Close() })
	return ins, leaf
}

func overlayPresent(c core.Canvas, o core.CanvasObject) bool {
	for _, item := range c.Overlays().List() {
		if item == o {
			return true
		}
	}
	return false
}

func TestShow_BuildsTreeFromTarget(t *testing.T) {
	ins, leaf := showInspector(t)

	require.NotNil(t, ins.snap)
	assert.NotEmpty(t, ins.snap.roots)
	assert.Contains(t, ins.snap.byObj, core.CanvasObject(leaf))
}

func TestSelectNode_HighlightsAndDetails(t *testing.T) {
	ins, leaf := showInspector(t)

	n := ins.snap.byObj[leaf]
	require.NotNil(t, n)

	ins.selectNode(n.uid)

	// detail pane populated with the node's properties.
	assert.NotEmpty(t, ins.detail.Objects)
	// highlight now lives on the target and is visible over the leaf, placed at
	// the node's absolute canvas bounds (which include the window content inset).
	assert.True(t, ins.highlightLive)
	assert.True(t, ins.highlight.Visible())
	assert.True(t, overlayPresent(ins.target.Canvas(), ins.highlight))
	assert.Equal(t, n.absPos, ins.highlight.Position())
	assert.Equal(t, geo.NewSize(60, 30), ins.highlight.Size())

	// unknown UID is a no-op, not a panic.
	ins.selectNode("does-not-exist")
}

func TestPickMode_HoverCommitAndCancel(t *testing.T) {
	ins, leaf := showInspector(t)
	center := geo.NewPos(50, 35) // inside the leaf

	ins.togglePick()
	require.True(t, ins.picking)
	require.NotNil(t, ins.pick)
	assert.True(t, overlayPresent(ins.target.Canvas(), ins.pick))

	// hover previews the object under the cursor without leaving pick mode.
	ins.pickHover(center)
	assert.True(t, ins.picking)
	assert.True(t, ins.highlight.Visible())

	// commit selects it in the tree and exits pick mode.
	ins.pickCommit(center)
	assert.False(t, ins.picking)
	assert.Nil(t, ins.pick)
	assert.Equal(t, ins.snap.byObj[leaf].uid, ins.selectedUID)

	// arming again then cancelling via Escape tears the overlay down.
	ins.togglePick()
	require.NotNil(t, ins.pick)
	ins.pick.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, ins.picking)
	assert.Nil(t, ins.pick)
}

func TestClose_RemovesEverythingFromTarget(t *testing.T) {
	ins, leaf := showInspector(t)
	ins.selectNode(ins.snap.byObj[leaf].uid)
	ins.togglePick()
	require.True(t, overlayPresent(ins.target.Canvas(), ins.highlight))

	ins.onClose()

	assert.True(t, ins.closed)
	assert.False(t, ins.picking)
	assert.False(t, ins.highlightLive)
	assert.False(t, overlayPresent(ins.target.Canvas(), ins.highlight))
	assert.False(t, overlayPresent(ins.target.Canvas(), ins.pick))

	// refresh after close is a no-op.
	before := ins.snap
	ins.refresh()
	assert.Same(t, before, ins.snap)
}
