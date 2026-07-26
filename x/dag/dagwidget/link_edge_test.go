package dagwidget

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/dag/dag"
)

// laneCanvas builds a canvas with nodes a and b side by side on a horizontal
// center line, far enough apart that border points land on the facing edges.
func laneCanvas(t *testing.T) *DAGCanvas {
	t.Helper()
	c := newTestCanvas(t)
	a := addTestNode(t, c, "a")
	b := addTestNode(t, c, "b")
	a.Position = [2]float32{0, 0}
	b.Position = [2]float32{400, 0}
	return c
}

func segMidpoint(e *LinkEdgeWidget) geo.Position {
	return geo.NewPos((e.p0.X+e.p1.X)/2, (e.p0.Y+e.p1.Y)/2)
}

func TestLinkEdgeSingleStaysOnCenterLine(t *testing.T) {
	c := laneCanvas(t)
	if err := c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}); err != nil {
		t.Fatal(err)
	}
	e := c.linkEdges["l1"]
	e.updateEndpoints()

	// Nodes are 160x80 at y=0, so the center line is y=40; a's right border
	// is x=160, b's left border is x=400.
	centerY := float32(40)
	if abs32(e.p0.Y-centerY) > 0.5 || abs32(e.p1.Y-centerY) > 0.5 {
		t.Fatalf("single link edge off the center line: p0=%v p1=%v", e.p0, e.p1)
	}
	if abs32(e.p0.X-160) > 0.5 || abs32(e.p1.X-400) > 0.5 {
		t.Fatalf("border points not on facing edges: p0=%v p1=%v", e.p0, e.p1)
	}
}

func TestLinkEdgeOppositePairGetsSeparateLanes(t *testing.T) {
	c := laneCanvas(t)
	for _, le := range []*dag.LinkEdge{
		{ID: "l1", Source: "a", Target: "b"},
		{ID: "l2", Source: "b", Target: "a"},
	} {
		if err := c.AddLinkEdge(le); err != nil {
			t.Fatal(err)
		}
	}
	e1, e2 := c.linkEdges["l1"], c.linkEdges["l2"]
	e1.updateEndpoints()
	e2.updateEndpoints()

	m1, m2 := segMidpoint(e1), segMidpoint(e2)
	sep := abs32(m1.Y - m2.Y)
	if abs32(sep-linkEdgeLaneSpacing) > 0.5 {
		t.Fatalf("lane separation %v, want %v (mid1=%v mid2=%v)", sep, linkEdgeLaneSpacing, m1, m2)
	}
	// Both segments stay parallel to the center line: the lane shift moves
	// the whole segment, not just one end.
	if abs32(e1.p0.Y-e1.p1.Y) > 0.5 || abs32(e2.p0.Y-e2.p1.Y) > 0.5 {
		t.Fatalf("laned segments not parallel to center line: l1=%v..%v l2=%v..%v",
			e1.p0, e1.p1, e2.p0, e2.p1)
	}
	// Endpoints stay on the node borders.
	for _, e := range []*LinkEdgeWidget{e1, e2} {
		xs := []float32{e.p0.X, e.p1.X}
		for _, x := range xs {
			if abs32(x-160) > 0.5 && abs32(x-400) > 0.5 {
				t.Fatalf("endpoint left the node borders: %v..%v", e.p0, e.p1)
			}
		}
	}
}

func TestLinkEdgeParallelSameDirectionGetsSeparateLanes(t *testing.T) {
	c := laneCanvas(t)
	for _, le := range []*dag.LinkEdge{
		{ID: "l1", Source: "a", Target: "b"},
		{ID: "l2", Source: "a", Target: "b"},
		{ID: "l3", Source: "a", Target: "b"},
	} {
		if err := c.AddLinkEdge(le); err != nil {
			t.Fatal(err)
		}
	}
	ys := make(map[string]float32)
	for id, e := range c.linkEdges {
		e.updateEndpoints()
		ys[id] = segMidpoint(e).Y
	}
	// Ranks follow sorted IDs: l1 above, l2 centered, l3 below.
	if !(ys["l1"] < ys["l2"] && ys["l2"] < ys["l3"]) {
		t.Fatalf("lanes not ordered by ID rank: %v", ys)
	}
	if abs32(ys["l2"]-40) > 0.5 {
		t.Fatalf("middle lane of odd count should sit on the center line: %v", ys["l2"])
	}
}

func TestLinkEdgeLabelsStaggerAlongCanonicalAxis(t *testing.T) {
	c := laneCanvas(t)
	for _, le := range []*dag.LinkEdge{
		{ID: "l1", Source: "a", Target: "b", Label: "requests"},
		{ID: "l2", Source: "b", Target: "a", Label: "responds"},
	} {
		if err := c.AddLinkEdge(le); err != nil {
			t.Fatal(err)
		}
	}
	e1, e2 := c.linkEdges["l1"], c.linkEdges["l2"]
	e1.updateEndpoints()
	e2.updateEndpoints()

	labelPos := func(e *LinkEdgeWidget) geo.Position {
		return geo.NewPos(
			e.p0.X+(e.p1.X-e.p0.X)*e.labelT,
			e.p0.Y+(e.p1.Y-e.p0.Y)*e.labelT,
		)
	}
	// Segment length is 240 (borders at x=160 and x=400), so one stagger
	// step separates the two labels by 0.15·240 = 36 on the canonical axis.
	sep := abs32(labelPos(e1).X - labelPos(e2).X)
	want := linkEdgeLabelStagger * 240
	if abs32(sep-want) > 0.5 {
		t.Fatalf("label separation %v along the canonical axis, want %v (t1=%v t2=%v)",
			sep, want, e1.labelT, e2.labelT)
	}
}

func TestLinkEdgeSingleLabelStaysAtMidpoint(t *testing.T) {
	c := laneCanvas(t)
	if err := c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "b", Target: "a", Label: "only"}); err != nil {
		t.Fatal(err)
	}
	e := c.linkEdges["l1"]
	e.updateEndpoints()
	if e.labelT != 0.5 {
		t.Fatalf("lone edge labelT = %v, want 0.5", e.labelT)
	}
}

func TestLinkEdgeLaneSpacingCompressesInsteadOfSaturating(t *testing.T) {
	c := laneCanvas(t)
	const n = 9
	for i := 1; i <= n; i++ {
		le := &dag.LinkEdge{ID: fmt.Sprintf("l%d", i), Source: "a", Target: "b"}
		if err := c.AddLinkEdge(le); err != nil {
			t.Fatal(err)
		}
	}
	var ys []float32
	for i := 1; i <= n; i++ {
		e := c.linkEdges[fmt.Sprintf("l%d", i)]
		e.updateEndpoints()
		ys = append(ys, segMidpoint(e).Y)
	}
	// maxOff = 0.4·min(160,80) = 32, so the default 12px spacing (spread 96)
	// compresses to 64/8 = 8px. Every lane stays distinct and evenly spaced;
	// a saturating clamp would stack the outer ranks at ±32.
	const wantGap = float32(8)
	for i := 1; i < n; i++ {
		if gap := ys[i] - ys[i-1]; abs32(gap-wantGap) > 0.5 {
			t.Fatalf("gap between lanes %d and %d = %v, want %v (ys=%v)", i-1, i, gap, wantGap, ys)
		}
	}
	if abs32(ys[0]-(40-32)) > 0.5 || abs32(ys[n-1]-(40+32)) > 0.5 {
		t.Fatalf("outer lanes not at ±maxOff around the center line: %v", ys)
	}
}

func TestLinkEdgeAtPositionPicksNearestLane(t *testing.T) {
	c := laneCanvas(t)
	for _, le := range []*dag.LinkEdge{
		{ID: "l1", Source: "a", Target: "b"},
		{ID: "l2", Source: "b", Target: "a"},
	} {
		if err := c.AddLinkEdge(le); err != nil {
			t.Fatal(err)
		}
	}
	e1, e2 := c.linkEdges["l1"], c.linkEdges["l2"]
	e1.updateEndpoints()
	e2.updateEndpoints()

	m1, m2 := segMidpoint(e1), segMidpoint(e2)
	if got := c.linkEdgeAtPosition(m1); got != e1 {
		t.Fatalf("tap on l1's lane resolved to %v", got)
	}
	if got := c.linkEdgeAtPosition(m2); got != e2 {
		t.Fatalf("tap on l2's lane resolved to %v", got)
	}
	if got := c.linkEdgeAtPosition(geo.NewPos(m1.X, 300)); got != nil {
		t.Fatalf("tap far away resolved to %v", got)
	}
}

// The connection-drag tests below drive the gesture through the test driver:
// test.DragSteps resolves the connector handle by hit-test and delivers
// Dragged events with advancing absolute positions, so the whole
// start/update/finish state machine runs off real driver dispatch.

func TestLinkEdgeConnectionDragCreatesUndoableEdge(t *testing.T) {
	c := laneCanvas(t)
	tc := mountCanvas(t, c)

	const steps = 8
	dragBetween(tc, c.nodes["a"].connector, c.nodes["b"], steps, func(step int) {
		if c.dragState != dragConnectingLink || c.pendingNodeLink == nil {
			t.Fatalf("step %d: connector drag did not enter dragConnectingLink", step)
		}
		if c.pendingNodeLink.snapTarget == "a" {
			t.Fatalf("step %d: snapped to the source node", step)
		}
		if step == steps { // cursor over b's card
			if got := c.pendingNodeLink.snapTarget; got != "b" {
				t.Fatalf("snapTarget = %q over b, want b", got)
			}
			if !c.nodes["b"].DropHighlight {
				t.Fatal("target node not highlighted mid-drag")
			}
		}
	})

	if c.dragState != dragNone || c.pendingNodeLink != nil {
		t.Fatal("drag state not reset after finish")
	}
	if c.nodes["b"].DropHighlight {
		t.Fatal("drop highlight not cleared after finish")
	}
	if len(c.Graph.LinkEdges) != 1 {
		t.Fatalf("link edge count = %d, want 1", len(c.Graph.LinkEdges))
	}
	var le *dag.LinkEdge
	for _, l := range c.Graph.LinkEdges {
		le = l
	}
	if le.Source != "a" || le.Target != "b" {
		t.Fatalf("edge %s→%s, want a→b", le.Source, le.Target)
	}
	if c.linkEdges[le.ID] == nil {
		t.Fatal("no widget created for the new link edge")
	}
	if !c.Commands.CanUndo() {
		t.Fatal("creation not recorded as an undoable command")
	}
	c.Commands.Undo()
	if len(c.Graph.LinkEdges) != 0 {
		t.Fatal("undo did not remove the created link edge")
	}
}

func TestLinkEdgeConnectionDragCancelsOnEmptyDrop(t *testing.T) {
	c := laneCanvas(t)
	tc := mountCanvas(t, c)

	// The path crosses b's card (snap + highlight) and carries on past its
	// right edge into empty space, so the snap must clear and the drop cancel.
	sawSnap := false
	start := absCenter(c.nodes["a"].connector)
	end := absCenter(c.nodes["b"]).AddXY(250, 0)
	test.DragSteps(tc, start, end.X-start.X, end.Y-start.Y, 10, func(step int) {
		if c.pendingNodeLink.snapTarget == "b" {
			sawSnap = true
			if !c.nodes["b"].DropHighlight {
				t.Fatalf("step %d: snapped to b without highlight", step)
			}
		}
	})
	if !sawSnap {
		t.Fatal("drag path never snapped to b")
	}
	if len(c.Graph.LinkEdges) != 0 {
		t.Fatal("cancelled drag created a link edge")
	}
	if c.Commands.CanUndo() {
		t.Fatal("cancelled drag recorded a command")
	}
	if c.dragState != dragNone {
		t.Fatal("drag state not reset after cancel")
	}
	if c.nodes["b"].DropHighlight {
		t.Fatal("stale drop highlight left after cancel")
	}
}

func TestLinkEdgeConnectionSkipsCollapsedGroupMembers(t *testing.T) {
	c := laneCanvas(t)
	if err := c.AddGroup(&dag.Group{
		ID: "g1", Label: "G", Members: []string{"b"},
		Position: [2]float32{380, -20}, Size: [2]float32{220, 140}, Collapsed: true,
	}); err != nil {
		t.Fatal(err)
	}
	tc := mountCanvas(t, c)

	// Drop on b's card center; b is hidden behind g1's collapsed card, whose
	// own card ends above, so the cursor is over the hidden node only.
	start := absCenter(c.nodes["a"].connector)
	canvasAbs := fyneDriver().AbsolutePositionForObject(c)
	end := canvasAbs.AddXY(480, 40)
	test.DragSteps(tc, start, end.X-start.X, end.Y-start.Y, 6, func(step int) {
		if got := c.pendingNodeLink.snapTarget; got != "" {
			t.Fatalf("step %d: snapped to hidden node %q", step, got)
		}
	})
	if len(c.Graph.LinkEdges) != 0 {
		t.Fatal("drop on a hidden node created a link edge")
	}
}

func TestLinkEdgeDragRespectsCapabilities(t *testing.T) {
	c := laneCanvas(t)
	c.Capabilities = Capabilities{}
	c.startLinkEdgeConnection("a", geo.NewPos(0, 0))
	if c.dragState != dragNone || c.pendingNodeLink != nil {
		t.Fatal("startLinkEdgeConnection entered drag with LinkEdges off")
	}
}

func TestConnectorHandlesFollowCapabilities(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G", Position: [2]float32{0, 200}, Size: [2]float32{200, 120}}); err != nil {
		t.Fatal(err)
	}
	w := test.NewWindow(c)
	defer w.Close()
	w.Resize(geo.NewSize(400, 400))

	if !c.nodes["a"].connector.Visible() || !c.groups["g1"].connector.Visible() {
		t.Fatal("connector handles hidden with full capabilities")
	}

	// Flipping capabilities mid-session must take effect on the next canvas
	// Refresh even though children are not individually refreshed.
	c.Capabilities = Capabilities{}
	c.Refresh()
	if c.nodes["a"].connector.Visible() {
		t.Fatal("node connector still visible with LinkEdges off")
	}
	if c.groups["g1"].connector.Visible() {
		t.Fatal("group connector still visible with GroupEdges off")
	}

	c.Capabilities = AllCapabilities()
	c.Refresh()
	if !c.nodes["a"].connector.Visible() || !c.groups["g1"].connector.Visible() {
		t.Fatal("connector handles did not reappear after re-enabling")
	}
}
