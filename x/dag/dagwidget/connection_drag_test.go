package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/dag/dag"
)

// mountCanvas puts the canvas in a test window so connection drags can be
// driven through the driver: test.DragSteps hit-tests the drag source and
// delivers events with real, advancing absolute positions (fyne >= v3.0.0-rc.4,
// trendvidia/fyne#1018).
func mountCanvas(t *testing.T, c *DAGCanvas) core.Canvas {
	t.Helper()
	w := test.NewWindow(c)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(700, 320))
	return w.Canvas()
}

// absCenter is the absolute position of an object's center — the coordinate
// space test.DragSteps operates in.
func absCenter(o core.CanvasObject) geo.Position {
	s := o.Size()
	return fyneDriver().AbsolutePositionForObject(o).AddXY(s.Width/2, s.Height/2)
}

// dragBetween drives a full drag gesture from the center of src to the center
// of dst through the test driver.
func dragBetween(tc core.Canvas, src, dst core.CanvasObject, steps int, onStep func(int)) {
	from, to := absCenter(src), absCenter(dst)
	test.DragSteps(tc, from, to.X-from.X, to.Y-from.Y, steps, onStep)
}

func TestPortConnectionDragCreatesEdge(t *testing.T) {
	c := laneCanvas(t)
	tc := mountCanvas(t, c)
	out, in := c.nodes["a"].Outputs[0], c.nodes["b"].Inputs[0]

	dragBetween(tc, out, in, 6, func(step int) {
		if c.dragState != dragConnecting || c.pendingEdge == nil {
			t.Fatalf("step %d: port drag did not enter dragConnecting", step)
		}
		if !in.ValidDrop {
			t.Fatalf("step %d: compatible input not marked as valid drop target", step)
		}
	})

	if c.dragState != dragNone || c.pendingEdge != nil {
		t.Fatal("drag state not reset after drop")
	}
	if len(c.Graph.Edges) != 1 {
		t.Fatalf("edge count = %d, want 1", len(c.Graph.Edges))
	}
	for _, e := range c.Graph.Edges {
		if e.Source.NodeID != "a" || e.Target.NodeID != "b" {
			t.Fatalf("edge %v→%v, want a→b", e.Source, e.Target)
		}
		if c.edges[e.ID] == nil {
			t.Fatal("no widget created for the new edge")
		}
	}
	if !out.Connected || !in.Connected {
		t.Fatal("ports not marked connected")
	}
	if in.ValidDrop {
		t.Fatal("valid-drop state not reset after drop")
	}

	if !c.Commands.CanUndo() {
		t.Fatal("creation not recorded as an undoable command")
	}
	c.Commands.Undo()
	if len(c.Graph.Edges) != 0 || len(c.edges) != 0 {
		t.Fatal("undo did not remove the created edge")
	}
	if out.Connected || in.Connected {
		t.Fatal("ports still marked connected after undo")
	}
	c.Commands.Redo()
	if len(c.Graph.Edges) != 1 {
		t.Fatal("redo did not restore the edge")
	}
	if !out.Connected || !in.Connected {
		t.Fatal("ports not re-marked connected after redo")
	}
}

func TestPortConnectionDragCancelsOnEmptyDrop(t *testing.T) {
	c := laneCanvas(t)
	tc := mountCanvas(t, c)
	out := c.nodes["a"].Outputs[0]

	test.DragSteps(tc, absCenter(out), 60, 180, 4, nil) // into empty canvas

	if len(c.Graph.Edges) != 0 {
		t.Fatal("cancelled drag created an edge")
	}
	if c.dragState != dragNone || c.pendingEdge != nil {
		t.Fatal("drag state not reset after cancel")
	}
	if c.nodes["b"].Inputs[0].ValidDrop {
		t.Fatal("valid-drop state not reset after cancel")
	}
}

// detachCanvas builds a canvas with nodes a→b connected and a free node c,
// with an empty command stack (the setup edge bypasses it).
func detachCanvas(t *testing.T) *DAGCanvas {
	t.Helper()
	c := laneCanvas(t) // a at (0,0), b at (400,0)
	n := addTestNode(t, c, "c")
	n.Position = [2]float32{400, 160}
	if err := c.Graph.AddEdge(&dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}); err != nil {
		t.Fatal(err)
	}
	c.syncFromGraph()
	return c
}

func soleEdgeTarget(t *testing.T, c *DAGCanvas) dag.PortID {
	t.Helper()
	if len(c.Graph.Edges) != 1 {
		t.Fatalf("edge count = %d, want 1", len(c.Graph.Edges))
	}
	for _, e := range c.Graph.Edges {
		return e.Target
	}
	return dag.PortID{}
}

func TestDetachRerouteRecordsOneUndoStep(t *testing.T) {
	c := detachCanvas(t)
	tc := mountCanvas(t, c)
	bIn, cIn := c.nodes["b"].Inputs[0], c.nodes["c"].Inputs[0]

	// Grab b's connected input and drop it on c's input.
	dragBetween(tc, bIn, cIn, 6, nil)

	if got := soleEdgeTarget(t, c); got != cIn.PortID {
		t.Fatalf("edge target = %v, want c's input", got)
	}
	if bIn.Connected || !cIn.Connected {
		t.Fatal("port connected state not rerouted")
	}
	if !c.Commands.CanUndo() {
		t.Fatal("reroute not recorded")
	}
	c.Commands.Undo()
	if got := soleEdgeTarget(t, c); got != bIn.PortID {
		t.Fatalf("one undo did not restore a→b (target = %v)", got)
	}
	if !bIn.Connected || cIn.Connected {
		t.Fatal("port connected state not restored by undo")
	}
	if c.Commands.CanUndo() {
		t.Fatal("reroute recorded more than one undo step")
	}
}

func TestDetachDropOnEmptyDeletesEdgeAsOneStep(t *testing.T) {
	c := detachCanvas(t)
	tc := mountCanvas(t, c)
	bIn := c.nodes["b"].Inputs[0]

	test.DragSteps(tc, absCenter(bIn), -100, 100, 4, nil) // into empty canvas

	if len(c.Graph.Edges) != 0 {
		t.Fatal("detach drop on empty space did not delete the edge")
	}
	if bIn.Connected {
		t.Fatal("port still marked connected after detach-delete")
	}
	c.Commands.Undo()
	if got := soleEdgeTarget(t, c); got != bIn.PortID {
		t.Fatalf("undo did not restore the edge (target = %v)", got)
	}
	if c.Commands.CanUndo() {
		t.Fatal("detach-delete recorded more than one undo step")
	}
}

func TestDetachDropOnSamePortIsNoOp(t *testing.T) {
	c := detachCanvas(t)
	tc := mountCanvas(t, c)
	bIn := c.nodes["b"].Inputs[0]
	notifications := 0
	c.OnGraphChanged = func() { notifications++ }

	// Wiggle within snap range of the port it started on.
	test.DragSteps(tc, absCenter(bIn), 10, 0, 3, nil)

	if got := soleEdgeTarget(t, c); got != bIn.PortID {
		t.Fatalf("edge changed by a same-port drop (target = %v)", got)
	}
	if !bIn.Connected {
		t.Fatal("port lost its connected state")
	}
	if c.Commands.CanUndo() {
		t.Fatal("net no-op gesture recorded a command")
	}
	if notifications != 0 {
		t.Fatalf("net no-op gesture fired %d notifications", notifications)
	}
}

// groupPairCanvas builds a canvas with two empty groups side by side.
func groupPairCanvas(t *testing.T) *DAGCanvas {
	t.Helper()
	c := newTestCanvas(t)
	for _, g := range []*dag.Group{
		{ID: "g1", Label: "G1", Position: [2]float32{0, 120}, Size: [2]float32{200, 120}},
		{ID: "g2", Label: "G2", Position: [2]float32{400, 120}, Size: [2]float32{200, 120}},
	} {
		if err := c.AddGroup(g); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestGroupEdgeConnectionDragCreatesUndoableEdge(t *testing.T) {
	c := groupPairCanvas(t)
	tc := mountCanvas(t, c)

	const steps = 8
	dragBetween(tc, c.groups["g1"].connector, c.groups["g2"], steps, func(step int) {
		if c.dragState != dragConnectingGroup || c.pendingGroupLink == nil {
			t.Fatalf("step %d: connector drag did not enter dragConnectingGroup", step)
		}
		if c.pendingGroupLink.snapTarget == "g1" {
			t.Fatalf("step %d: snapped to the source group", step)
		}
		if step == steps { // cursor over g2's rect
			if got := c.pendingGroupLink.snapTarget; got != "g2" {
				t.Fatalf("snapTarget = %q over g2, want g2", got)
			}
			if !c.groups["g2"].DropHighlight {
				t.Fatal("target group not highlighted mid-drag")
			}
		}
	})

	if c.dragState != dragNone || c.pendingGroupLink != nil {
		t.Fatal("drag state not reset after finish")
	}
	if c.groups["g2"].DropHighlight {
		t.Fatal("drop highlight not cleared after finish")
	}
	if len(c.Graph.GroupEdges) != 1 {
		t.Fatalf("group edge count = %d, want 1", len(c.Graph.GroupEdges))
	}
	for _, ge := range c.Graph.GroupEdges {
		if ge.Source != "g1" || ge.Target != "g2" {
			t.Fatalf("edge %s→%s, want g1→g2", ge.Source, ge.Target)
		}
		if c.groupEdges[ge.ID] == nil {
			t.Fatal("no widget created for the new group edge")
		}
	}
	if !c.Commands.CanUndo() {
		t.Fatal("creation not recorded as an undoable command")
	}
	c.Commands.Undo()
	if len(c.Graph.GroupEdges) != 0 {
		t.Fatal("undo did not remove the created group edge")
	}
}

func TestGroupEdgeConnectionDragCancelsOnEmptyDrop(t *testing.T) {
	c := groupPairCanvas(t)
	tc := mountCanvas(t, c)

	start := absCenter(c.groups["g1"].connector)
	test.DragSteps(tc, start, 100, -80, 4, nil) // above the groups: empty space

	if len(c.Graph.GroupEdges) != 0 {
		t.Fatal("cancelled drag created a group edge")
	}
	if c.Commands.CanUndo() {
		t.Fatal("cancelled drag recorded a command")
	}
	if c.dragState != dragNone || c.pendingGroupLink != nil {
		t.Fatal("drag state not reset after cancel")
	}
}

func TestGroupEdgeConnectionNeverSnapsToCycleTarget(t *testing.T) {
	c := groupPairCanvas(t)
	if err := c.AddGroupEdge(&dag.GroupEdge{ID: "ge1", Source: "g2", Target: "g1"}); err != nil {
		t.Fatal(err)
	}
	tc := mountCanvas(t, c)

	// g2→g1 exists, so g1→g2 would form a cycle: g2 must never snap.
	dragBetween(tc, c.groups["g1"].connector, c.groups["g2"], 6, func(step int) {
		if got := c.pendingGroupLink.snapTarget; got != "" {
			t.Fatalf("step %d: snapped to cycle-forming target %q", step, got)
		}
		if c.groups["g2"].DropHighlight {
			t.Fatalf("step %d: cycle-forming target highlighted", step)
		}
	})

	if len(c.Graph.GroupEdges) != 1 {
		t.Fatalf("group edge count = %d, want the pre-existing 1", len(c.Graph.GroupEdges))
	}
}

func TestGroupEdgeConnectionSnapsToCollapsedTarget(t *testing.T) {
	c := groupPairCanvas(t)
	c.Graph.Groups["g2"].Collapsed = true
	tc := mountCanvas(t, c)

	// A collapsed group's compact card is a valid drop target.
	start := absCenter(c.groups["g1"].connector)
	r := c.groupScreenRect(c.Graph.Groups["g2"])
	canvasAbs := fyneDriver().AbsolutePositionForObject(c)
	end := canvasAbs.AddXY(r.Pos.X+r.Size.Width/2, r.Pos.Y+r.Size.Height/2)
	test.DragSteps(tc, start, end.X-start.X, end.Y-start.Y, 6, func(step int) {
		if step == 6 {
			if got := c.pendingGroupLink.snapTarget; got != "g2" {
				t.Fatalf("snapTarget = %q over collapsed card, want g2", got)
			}
		}
	})

	if len(c.Graph.GroupEdges) != 1 {
		t.Fatalf("group edge count = %d, want 1", len(c.Graph.GroupEdges))
	}
	for _, ge := range c.Graph.GroupEdges {
		if ge.Source != "g1" || ge.Target != "g2" {
			t.Fatalf("edge %s→%s, want g1→g2", ge.Source, ge.Target)
		}
	}
}
