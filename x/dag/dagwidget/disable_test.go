package dagwidget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// canvasCallbacks counts every callback a DAGCanvas fires.
type canvasCallbacks struct{ n map[string]int }

func (cb *canvasCallbacks) wire(c *DAGCanvas) {
	cb.n = map[string]int{}
	hit := func(name string) { cb.n[name]++ }
	c.OnNodeSelected = func(string) { hit("node selected") }
	c.OnEdgeSelected = func(string) { hit("edge selected") }
	c.OnGroupSelected = func(string) { hit("group selected") }
	c.OnLinkEdgeSelected = func(string) { hit("link selected") }
	c.OnAnnotationSelected = func(string) { hit("annotation selected") }
	c.OnNodeActivated = func(string) { hit("node activated") }
	c.OnGroupActivated = func(string) { hit("group activated") }
	c.OnNodeSecondaryTapped = func(string, *event.PointEvent) { hit("node menu") }
	c.OnEdgeSecondaryTapped = func(string, *event.PointEvent) { hit("edge menu") }
	c.OnGroupSecondaryTapped = func(string, *event.PointEvent) { hit("group menu") }
	c.OnLinkEdgeSecondaryTapped = func(string, *event.PointEvent) { hit("link menu") }
	c.OnAnnotationSecondaryTapped = func(string, *event.PointEvent) { hit("annotation menu") }
	c.OnSelectionCleared = func() { hit("selection cleared") }
	c.OnGraphChanged = func() { hit("graph changed") }
}

func (cb *canvasCallbacks) total() int {
	n := 0
	for _, v := range cb.n {
		n += v
	}
	return n
}

// canvasLook is what the disabled look changes on the canvas's own parts.
type canvasLook struct {
	title, port, edge color.Color
	handleShown       bool
	connectorShown    bool
}

func lookOfCanvas(t *testing.T, c *DAGCanvas) canvasLook {
	t.Helper()
	nw := c.nodes["a"]
	nr := test.TempWidgetRenderer(t, nw).(*nodeRenderer)
	pr := test.TempWidgetRenderer(t, nw.Outputs[0]).(*portRenderer)
	er := test.TempWidgetRenderer(t, c.edges["e1"]).(*edgeRenderer)
	require.NotEmpty(t, er.lines)
	return canvasLook{
		title:          nr.titleText.Color,
		port:           pr.circle.StrokeColor,
		edge:           er.lines[0].StrokeColor,
		handleShown:    nw.handles[0].Visible(),
		connectorShown: nw.connector.Visible(),
	}
}

// A disabled DAGCanvas is visible but inert (#1217, belluikit/player#641).
// Disable works before the canvas is drawn; while disabled no tap, drag,
// pan, zoom, pinch, key or shortcut changes the graph or the view, or fires a
// callback; Tab skips the canvas and a focus on it is dropped; its parts
// draw in the disabled colours and its handles hide. Enable restores it.
func TestDAGCanvas_Disable(t *testing.T) {
	c := fullCanvas(t)
	c.nodes["a"].Selected = true  // so its resize handles would show, and Delete would act
	c.edges["e1"].Selected = true // drawn in Primary, which the test theme tells apart from Disabled
	var cb canvasCallbacks
	cb.wire(c)

	// Before the first draw: no window, no renderer.
	c.Disable()
	require.True(t, c.Disabled())

	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 600))
	cnv := w.Canvas()
	c.Refresh()

	disabled := theme.Color(theme.ColorNameDisabled)
	look := lookOfCanvas(t, c)
	assert.Equal(t, disabled, look.title, "node title in the disabled colour")
	assert.Equal(t, disabled, look.port, "port in the disabled colour")
	assert.Equal(t, disabled, look.edge, "edge in the disabled colour")
	assert.False(t, look.handleShown, "no resize handles while disabled")
	assert.False(t, look.connectorShown, "no connector handle while disabled")

	nodeA := c.nodes["a"]
	origin := bellui.CurrentApp().Driver().AbsolutePositionForObject(c)
	nodeAbs := origin.AddXY(80, 50) // inside node a's card
	empty := origin.AddXY(700, 500) // empty canvas
	nodePos := c.Graph.Nodes["a"].Position
	sizeA := c.Graph.Nodes["a"].Size
	zoom, offset := c.Zoom, c.Offset
	edges, linkEdges, groupEdges := len(c.Graph.Edges), len(c.Graph.LinkEdges), len(c.Graph.GroupEdges)
	collapsed := c.Graph.Groups["g1"].Collapsed

	refuse := func() {
		t.Helper()
		test.TapCanvas(cnv, nodeAbs)
		nodeA.Tapped(&event.PointEvent{})
		nodeA.Tapped(&event.PointEvent{}) // a double tap would activate
		nodeA.TappedSecondary(&event.PointEvent{})
		gw := c.groups["g1"]
		gw.titleBar.Tapped(&event.PointEvent{})
		gw.titleBar.TappedSecondary(&event.PointEvent{})
		gw.collapse.Tapped(&event.PointEvent{})
		c.annotations["an1"].Tapped(&event.PointEvent{})
		c.annotations["an1"].TappedSecondary(&event.PointEvent{})
		c.Tapped(&event.PointEvent{Position: geo.NewPos(700, 500)})
		c.TappedSecondary(&event.PointEvent{Position: geo.NewPos(280, 40)})

		test.Drag(cnv, nodeAbs, 60, 30) // move node a
		test.Drag(cnv, empty, 60, 30)   // pan
		nodeA.handles[0].Dragged(&event.DragEvent{Dragged: geo.NewDelta(40, 40)})
		nodeA.handles[0].DragEnd()
		port := nodeA.Outputs[0]
		port.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: origin.AddXY(420, 50)}})
		port.DragEnd()
		nodeA.connector.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: origin.AddXY(420, 50)}})
		nodeA.connector.DragEnd()
		gw.connector.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: origin.AddXY(100, 250)}})
		gw.connector.DragEnd()
		gw.titleBar.Dragged(&event.DragEvent{Dragged: geo.NewDelta(30, 30)})
		gw.titleBar.DragEnd()
		c.annotations["an1"].Dragged(&event.DragEvent{Dragged: geo.NewDelta(30, 30)})
		c.annotations["an1"].DragEnd()

		test.Scroll(cnv, empty, 0, -300) // wheel zoom
		c.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(40, 40), Source: event.ScrollSourceTouch})
		c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(300, 300)}, Scale: 2})
		c.PinchEnded()

		for _, k := range []*event.KeyEvent{
			tabKey(), shiftTabKey(),
			{Name: event.KeyReturn},
			menuKey(), shiftF10(),
			{Name: event.KeyEscape},
			{Name: event.KeyDelete},
		} {
			c.TypedKey(k)
		}
		c.TypedShortcut(&bellui.ShortcutUndo{})
		c.TypedShortcut(&bellui.ShortcutRedo{})
	}

	refuse()
	assert.Zero(t, cb.total(), "no callback fires while disabled: %v", cb.n)
	assert.Equal(t, nodePos, c.Graph.Nodes["a"].Position, "node a did not move")
	assert.Equal(t, sizeA, c.Graph.Nodes["a"].Size, "node a was not resized")
	assert.Equal(t, zoom, c.Zoom, "the view did not zoom")
	assert.Equal(t, offset, c.Offset, "the view did not pan")
	assert.Equal(t, edges, len(c.Graph.Edges), "no edge added, removed or undone")
	assert.Equal(t, linkEdges, len(c.Graph.LinkEdges), "no link edge added")
	assert.Equal(t, groupEdges, len(c.Graph.GroupEdges), "no group edge added")
	assert.Equal(t, collapsed, c.Graph.Groups["g1"].Collapsed, "the group did not collapse")
	assert.NotNil(t, c.Graph.Nodes["a"], "Delete removed nothing")
	assert.True(t, nodeA.Selected, "Escape cleared nothing")
	assert.Equal(t, dragNone, c.dragState, "no gesture left open")
	assert.True(t, c.TryScrolled(&event.ScrollEvent{}) == false, "a disabled canvas does not consume a scroll")
	assert.False(t, c.DragMomentum(), "nothing to coast")
	assert.Equal(t, desktop.DefaultCursor, nodeA.Outputs[0].Cursor(), "no connect cursor over a port")

	test.FocusNext(cnv)
	assert.Nil(t, cnv.Focused(), "Tab skips a disabled canvas")

	// Enable restores the look and the input.
	c.Enable()
	look = lookOfCanvas(t, c)
	assert.NotEqual(t, disabled, look.title, "node title back to its colour")
	assert.NotEqual(t, disabled, look.port, "port back to its colour")
	assert.NotEqual(t, disabled, look.edge, "edge back to its colour")
	assert.True(t, look.handleShown, "resize handles back on the selected node")
	assert.True(t, look.connectorShown, "connector handle back")

	nodeA.Tapped(&event.PointEvent{})
	assert.Equal(t, 1, cb.n["node selected"], "enabled, a tap selects")
	test.Scroll(cnv, empty, 0, -300)
	assert.NotEqual(t, zoom, c.Zoom, "enabled, the wheel zooms")
	test.FocusNext(cnv)
	assert.True(t, cnv.Focused() == c, "enabled, Tab reaches the canvas")

	// Disable after the first draw, with the focus on the canvas.
	require.True(t, cnv.Focused() == c)
	c.Disable()
	assert.Nil(t, cnv.Focused(), "Disable drops the focus on the canvas")
	assert.Equal(t, disabled, lookOfCanvas(t, c).title, "the look applies after the first draw")

	// Disable in the middle of a connection drag cancels it: no edge.
	c.Enable()
	before := len(c.Graph.Edges)
	c.startConnection(nodeA.Outputs[0].PortID, geo.NewPos(160, 50))
	require.Equal(t, dragConnecting, c.dragState)
	c.pendingEdge.snapTarget = c.nodes["b"].Inputs[0]
	c.Disable()
	assert.Equal(t, dragNone, c.dragState, "Disable cancels the connection drag")
	nodeA.Outputs[0].DragEnd()
	assert.Equal(t, before, len(c.Graph.Edges), "the cancelled drag adds no edge")

	// Repeated calls are no-ops.
	c.Disable()
	assert.True(t, c.Disabled())
	c.Enable()
	c.Enable()
	assert.False(t, c.Disabled())
}
