package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/dag/dag"
)

// newTestCanvas builds a canvas over a fresh graph inside a headless test
// app. NewDAGCanvas installs the command stack, wired to OnGraphChanged.
func newTestCanvas(t *testing.T) *DAGCanvas {
	t.Helper()
	test.NewApp()
	return NewDAGCanvas(dag.NewGraph("test", "Test"))
}

func addTestNode(t *testing.T, c *DAGCanvas, id string) *dag.Node {
	t.Helper()
	n := &dag.Node{
		ID: id, Label: id,
		Inputs:  []dag.Port{{Name: "in", Direction: dag.PortInput}},
		Outputs: []dag.Port{{Name: "out", Direction: dag.PortOutput}},
	}
	if err := c.AddNode(n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAddRemoveGroupCommandRoundtrip(t *testing.T) {
	c := newTestCanvas(t)
	g := &dag.Group{ID: "g1", Label: "Stage", Size: [2]float32{200, 100}}

	c.Commands.Do(&AddGroupCommand{Canvas: c, Group: g})
	if c.Graph.Groups["g1"] == nil || c.groups["g1"] == nil {
		t.Fatal("group not added to graph and widget map")
	}
	c.Commands.Undo()
	if c.Graph.Groups["g1"] != nil || c.groups["g1"] != nil {
		t.Fatal("undo left group behind")
	}
	c.Commands.Redo()
	if c.Graph.Groups["g1"] == nil || c.groups["g1"] == nil {
		t.Fatal("redo did not restore group")
	}
}

func TestRemoveGroupCommandRestoresCascades(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.AddGroup(&dag.Group{ID: "g1", Label: "G1", Members: []string{"a"}})
	c.AddGroup(&dag.Group{ID: "g2", Label: "G2"})
	if err := c.AddGroupEdge(&dag.GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "n", AnchorKind: dag.AnchorGroup, AnchorID: "g1"}); err != nil {
		t.Fatal(err)
	}

	g1 := c.Graph.Groups["g1"]
	c.Commands.Do(&RemoveGroupCommand{Canvas: c, Group: g1})
	if c.Graph.Groups["g1"] != nil {
		t.Fatal("group survived removal")
	}
	if len(c.Graph.GroupEdges) != 0 || len(c.Graph.Annotations) != 0 {
		t.Fatal("cascades did not run")
	}
	if c.groupEdges["ge1"] != nil || c.annotations["an1"] != nil {
		t.Fatal("cascaded widgets not reconciled")
	}

	c.Commands.Undo()
	if c.Graph.Groups["g1"] == nil {
		t.Fatal("undo did not restore group")
	}
	if c.Graph.GroupEdges["ge1"] == nil || c.groupEdges["ge1"] == nil {
		t.Fatal("undo did not restore group edge")
	}
	if c.Graph.Annotations["an1"] == nil || c.annotations["an1"] == nil {
		t.Fatal("undo did not restore annotation")
	}
	if c.Graph.GroupOf("a") != "g1" {
		t.Fatal("undo did not restore membership")
	}
}

func TestAssignNodeToGroupCommandRoundtrip(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.AddGroup(&dag.Group{ID: "g1"})

	cmd := &AssignNodeToGroupCommand{Canvas: c, NodeID: "a", OldGroupID: "", NewGroupID: "g1"}
	c.Commands.Do(cmd)
	if c.Graph.GroupOf("a") != "g1" {
		t.Fatal("assign did not apply")
	}
	c.Commands.Undo()
	if c.Graph.GroupOf("a") != "" {
		t.Fatal("undo did not ungroup")
	}
	c.Commands.Redo()
	if c.Graph.GroupOf("a") != "g1" {
		t.Fatal("redo did not re-assign")
	}
}

func TestMoveGroupCommandMovesMembers(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	n.Position = [2]float32{10, 10}
	c.AddGroup(&dag.Group{ID: "g1", Members: []string{"a"}, Position: [2]float32{0, 0}})

	cmd := &MoveGroupCommand{
		Canvas:    c,
		GroupID:   "g1",
		OldPos:    [2]float32{0, 0},
		NewPos:    [2]float32{50, 60},
		MemberOld: map[string][2]float32{"a": {10, 10}},
		MemberNew: map[string][2]float32{"a": {60, 70}},
	}
	c.Commands.Do(cmd)
	if c.Graph.Groups["g1"].Position != [2]float32{50, 60} {
		t.Fatal("group did not move")
	}
	if n.Position != [2]float32{60, 70} {
		t.Fatal("member did not move")
	}
	c.Commands.Undo()
	if c.Graph.Groups["g1"].Position != [2]float32{0, 0} || n.Position != [2]float32{10, 10} {
		t.Fatal("undo did not restore group and member positions")
	}
}

func TestToggleGroupCollapseCommandRoundtrip(t *testing.T) {
	c := newTestCanvas(t)
	c.AddGroup(&dag.Group{ID: "g1"})

	c.Commands.Do(&ToggleGroupCollapseCommand{Canvas: c, GroupID: "g1", Collapsed: true})
	if !c.Graph.Groups["g1"].Collapsed {
		t.Fatal("collapse did not apply")
	}
	c.Commands.Undo()
	if c.Graph.Groups["g1"].Collapsed {
		t.Fatal("undo did not expand")
	}
}

func TestAddGroupEdgeCommandCycleRejection(t *testing.T) {
	c := newTestCanvas(t)
	c.AddGroup(&dag.Group{ID: "g1"})
	c.AddGroup(&dag.Group{ID: "g2"})
	if err := c.AddGroupEdge(&dag.GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}); err != nil {
		t.Fatal(err)
	}

	// The reverse edge would cycle; Execute must leave no widget behind.
	c.Commands.Do(&AddGroupEdgeCommand{Canvas: c, Edge: &dag.GroupEdge{ID: "ge2", Source: "g2", Target: "g1"}})
	if c.Graph.GroupEdges["ge2"] != nil {
		t.Fatal("cyclic group edge accepted by model")
	}
	if c.groupEdges["ge2"] != nil {
		t.Fatal("cyclic group edge leaked a widget")
	}
	if c.canConnectGroups("g2", "g1") {
		t.Fatal("canConnectGroups should reject the cycle-forming reverse direction")
	}
	if c.canConnectGroups("g1", "g2") {
		t.Fatal("canConnectGroups should reject a duplicate pair")
	}
}

func TestRemoveNodeCommandRestoresMembershipAndAnnotations(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	addTestNode(t, c, "b")
	if err := c.Graph.AddEdge(&dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}); err != nil {
		t.Fatal(err)
	}
	c.syncFromGraph()
	c.AddGroup(&dag.Group{ID: "g1", Members: []string{"a"}})
	c.AddAnnotation(&dag.Annotation{ID: "onNode", AnchorKind: dag.AnchorNode, AnchorID: "a"})
	c.AddAnnotation(&dag.Annotation{ID: "onEdge", AnchorKind: dag.AnchorEdge, AnchorID: "e1"})

	node := c.Graph.Nodes["a"]
	c.Commands.Do(&RemoveNodeCommand{Canvas: c, Node: node})
	if len(c.Graph.Annotations) != 0 {
		t.Fatal("anchored annotations survived node removal")
	}
	if c.annotations["onNode"] != nil || c.annotations["onEdge"] != nil {
		t.Fatal("annotation widgets not reconciled")
	}

	c.Commands.Undo()
	if c.Graph.Nodes["a"] == nil || c.Graph.Edges["e1"] == nil {
		t.Fatal("undo did not restore node and edge")
	}
	if c.Graph.GroupOf("a") != "g1" {
		t.Fatal("undo did not restore membership")
	}
	if c.Graph.Annotations["onNode"] == nil || c.Graph.Annotations["onEdge"] == nil {
		t.Fatal("undo did not restore annotations")
	}
}

func TestAnnotationCommandsRoundtrip(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	a := &dag.Annotation{ID: "an1", Text: "hello", Position: [2]float32{1, 2}, Size: [2]float32{160, 60}}

	c.Commands.Do(&AddAnnotationCommand{Canvas: c, Annotation: a})
	if c.Graph.Annotations["an1"] == nil || c.annotations["an1"] == nil {
		t.Fatal("annotation not added")
	}

	c.Commands.Do(&SetAnnotationTextCommand{Canvas: c, AnnotationID: "an1", OldText: "hello", NewText: "world"})
	if c.Graph.Annotations["an1"].Text != "world" {
		t.Fatal("text change did not apply")
	}
	c.Commands.Undo()
	if c.Graph.Annotations["an1"].Text != "hello" {
		t.Fatal("text undo failed")
	}

	c.Commands.Do(&SetAnnotationAnchorCommand{
		Canvas: c, AnnotationID: "an1",
		OldKind: dag.AnchorNone, NewKind: dag.AnchorNode,
		OldAnchorID: "", NewAnchorID: "a",
	})
	if got := c.Graph.Annotations["an1"]; got.AnchorKind != dag.AnchorNode || got.AnchorID != "a" {
		t.Fatalf("anchor change did not apply: %+v", got)
	}
	c.Commands.Undo()
	if got := c.Graph.Annotations["an1"]; got.AnchorKind != dag.AnchorNone || got.AnchorID != "" {
		t.Fatalf("anchor undo failed: %+v", got)
	}

	c.Commands.Do(&RemoveAnnotationCommand{Canvas: c, Annotation: c.Graph.Annotations["an1"]})
	if len(c.Graph.Annotations) != 0 {
		t.Fatal("annotation not removed")
	}
	c.Commands.Undo()
	if c.Graph.Annotations["an1"] == nil {
		t.Fatal("annotation removal undo failed")
	}
}

func TestSetAnnotationAnchorRejectedIsNotRecorded(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	if err := c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "note"}); err != nil {
		t.Fatal(err)
	}
	notifications := 0
	c.OnGraphChanged = func() { notifications++ }

	c.Commands.Do(&SetAnnotationAnchorCommand{
		Canvas: c, AnnotationID: "an1",
		OldKind: dag.AnchorNone, NewKind: dag.AnchorNode,
		OldAnchorID: "", NewAnchorID: "ghost", // no such node
	})

	if got := c.Graph.Annotations["an1"]; got == nil || got.AnchorKind != dag.AnchorNone || got.AnchorID != "" {
		t.Fatalf("rejected re-anchor changed the annotation: %+v", got)
	}
	if c.Commands.CanUndo() {
		t.Fatal("rejected re-anchor was recorded")
	}
	if notifications != 0 {
		t.Fatalf("rejected re-anchor fired %d notifications", notifications)
	}
}

func TestAnnotationGrowsToFitText(t *testing.T) {
	c := newTestCanvas(t)
	a := &dag.Annotation{ID: "an1", Text: "short", Size: [2]float32{defaultAnnotationW, defaultAnnotationH}}
	if err := c.AddAnnotation(a); err != nil {
		t.Fatal(err)
	}
	aw := c.annotations["an1"]
	base := aw.effectiveSize()

	// A long single line must widen the box beyond its stored size.
	a.Text = "this is a rather long single line of documentation text that cannot fit"
	wide := aw.effectiveSize()
	if wide.Width <= base.Width {
		t.Fatalf("box did not widen for long text: %v -> %v", base, wide)
	}

	// Many lines must grow the height beyond the stored size.
	a.Text = "one\ntwo\nthree\nfour\nfive\nsix\nseven"
	tall := aw.effectiveSize()
	if tall.Height <= base.Height {
		t.Fatalf("box did not grow for multi-line text: %v -> %v", base, tall)
	}

	// Resize handles must not be able to shrink below the content.
	min := aw.minGraphSize()
	if min.Height <= annotationMinH {
		t.Fatalf("minGraphSize ignores content: %v", min)
	}
}

func TestDropHalfInGrowsGroup(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	c.AddGroup(&dag.Group{ID: "g1", Position: [2]float32{0, 0}, Size: [2]float32{280, 180}})

	// Simulate a drag ending with the node straddling the right border,
	// center just inside.
	nw := c.nodes["a"]
	nw.dragging = true
	nw.dragStartPos = [2]float32{500, 500}
	n.Position = [2]float32{200, 60} // node is ~160 wide: right edge at 360 > 280
	nw.DragEnd()

	if c.Graph.GroupOf("a") != "g1" {
		t.Fatal("node not assigned on center-inside drop")
	}
	grp := c.Graph.Groups["g1"]
	eff := nw.effectiveSize()
	if grp.Position[0]+grp.Size[0] < n.Position[0]+eff.Width+groupContentPadding {
		t.Fatalf("group did not grow to contain the node: group=%v/%v node right=%v",
			grp.Position, grp.Size, n.Position[0]+eff.Width)
	}
	if grp.Position[1] > n.Position[1]-groupTitleHeight {
		t.Fatalf("group top does not clear the title bar above the node: %v", grp.Position)
	}

	// One undo restores position, membership, and group geometry together.
	c.Commands.Undo()
	if c.Graph.GroupOf("a") != "" {
		t.Fatal("undo did not clear membership")
	}
	if n.Position != [2]float32{500, 500} {
		t.Fatalf("undo did not restore node position: %v", n.Position)
	}
	if grp.Size != [2]float32{280, 180} || grp.Position != [2]float32{0, 0} {
		t.Fatalf("undo did not restore group geometry: %v/%v", grp.Position, grp.Size)
	}
}

func TestDropHalfOutNudgesNodeClear(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	c.AddGroup(&dag.Group{ID: "g1", Position: [2]float32{0, 0}, Size: [2]float32{280, 180}, Members: []string{"a"}})
	n.Position = [2]float32{60, 50}

	// Drag the member out: center outside, but the rect still overlaps the
	// group's right border.
	nw := c.nodes["a"]
	nw.dragging = true
	nw.dragStartPos = [2]float32{60, 50}
	n.Position = [2]float32{240, 50} // center at 320 > 280, left edge at 240 < 280
	nw.DragEnd()

	if c.Graph.GroupOf("a") != "" {
		t.Fatal("node not un-assigned on center-outside drop")
	}
	if n.Position[0] < 280+nodeClearGap {
		t.Fatalf("node still overlaps the group after nudge: x=%v", n.Position[0])
	}

	// One undo restores membership and the pre-drag position.
	c.Commands.Undo()
	if c.Graph.GroupOf("a") != "g1" {
		t.Fatal("undo did not restore membership")
	}
	if n.Position != [2]float32{60, 50} {
		t.Fatalf("undo did not restore node position: %v", n.Position)
	}
}

func TestGroupGrowthIncludesEdgeCurves(t *testing.T) {
	c := newTestCanvas(t)
	a := addTestNode(t, c, "a")
	b := addTestNode(t, c, "b")
	// Backward connection: source node to the right of the target, so the
	// bezier routes around and bulges well past both node rects.
	a.Position = [2]float32{400, 60}
	b.Position = [2]float32{40, 60}
	if err := c.Graph.AddEdge(&dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}); err != nil {
		t.Fatal(err)
	}
	c.syncFromGraph()
	// Group snugly contains both node rects but not the curve bulge.
	c.AddGroup(&dag.Group{ID: "g1", Position: [2]float32{0, 0}, Size: [2]float32{600, 280}, Members: []string{"b"}})

	grp := c.Graph.Groups["g1"]
	pos, size, changed := c.groupGrowthToFit(grp, a, c.nodes["a"].effectiveSize())
	if !changed {
		t.Fatal("expected growth for the backward edge curve")
	}
	// The backward curve bulges right of a's output port; the group's right
	// border must clear the drawn curve...
	p0, _ := portCenterGraph(a, c.nodes["a"].effectiveSize(), "out", dag.PortOutput)
	p3, _ := portCenterGraph(b, c.nodes["b"].effectiveSize(), "in", dag.PortInput)
	cp1, cp2 := ComputeControlPoints(p0, p3)
	_, bmax := BezierTightBounds(p0, cp1, cp2, p3)
	if pos[0]+size[0] < bmax.X+groupContentPadding {
		t.Fatalf("group right %v does not clear curve extent %v", pos[0]+size[0], bmax.X)
	}
	if pos[1]+size[1] < bmax.Y+groupContentPadding {
		t.Fatalf("group bottom %v does not clear curve extent %v", pos[1]+size[1], bmax.Y)
	}
	// ...but must NOT balloon to the control polygon's extent (the curve
	// reaches nowhere near the control points on a long backward edge).
	_, polyMax := BezierBoundingBox(p0, cp1, cp2, p3)
	if polyMax.X-bmax.X > 20 { // sanity: this fixture makes the polygon overshoot
		if pos[0]+size[0] >= polyMax.X {
			t.Fatalf("group grew to the control polygon (%v), not the curve (%v)", polyMax.X, bmax.X)
		}
	}
}

func TestEdgeEndpointsWithoutLayout(t *testing.T) {
	// Regression: endpoints must come from model geometry, never from the
	// painted port-widget positions (driver.AbsolutePositionForObject),
	// which are stale on the first frame and right after culled nodes are
	// shown again — that froze a garbage curve until the next redraw.
	c := newTestCanvas(t)
	a := addTestNode(t, c, "a")
	b := addTestNode(t, c, "b")
	a.Position = [2]float32{150, 400} // the gallery's Timer/Merge shape:
	b.Position = [2]float32{724, 150} // far from the origin, culled at min size
	if err := c.Graph.AddEdge(&dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}); err != nil {
		t.Fatal(err)
	}
	c.syncFromGraph()

	// No canvas layout, no paint — compute endpoints cold.
	ew := c.edges["e1"]
	ew.updateEndpoints()

	wantP0, ok := c.portCenterScreen(dag.PortID{NodeID: "a", Name: "out"})
	if !ok {
		t.Fatal("source port did not resolve")
	}
	wantP3, ok := c.portCenterScreen(dag.PortID{NodeID: "b", Name: "in"})
	if !ok {
		t.Fatal("target port did not resolve")
	}
	if ew.p0 != wantP0 || ew.p3 != wantP3 {
		t.Fatalf("endpoints not from model geometry: p0=%v want %v, p3=%v want %v",
			ew.p0, wantP0, ew.p3, wantP3)
	}
	// Source port sits on a's right border, target on b's left border.
	effA := c.nodes["a"].effectiveSize()
	if ew.p0.X != a.Position[0]+effA.Width {
		t.Fatalf("p0 not on source right border: %v", ew.p0)
	}
	if ew.p3.X != b.Position[0] {
		t.Fatalf("p3 not on target left border: %v", ew.p3)
	}

	// Pan/zoom must shift endpoints deterministically too.
	c.Zoom = 2
	c.Offset = geo.NewPos(100, 50)
	ew.updateEndpoints()
	if got, _ := c.portCenterScreen(dag.PortID{NodeID: "a", Name: "out"}); ew.p0 != got {
		t.Fatalf("p0 stale after pan/zoom: %v want %v", ew.p0, got)
	}
}

func TestGroupAtGraphPosSkipsCollapsed(t *testing.T) {
	c := newTestCanvas(t)
	c.AddGroup(&dag.Group{ID: "g1", Position: [2]float32{0, 0}, Size: [2]float32{300, 200}})
	c.AddGroup(&dag.Group{ID: "g2", Position: [2]float32{50, 50}, Size: [2]float32{150, 100}})

	// Overlap resolves to the smaller group.
	if g := c.groupAtGraphPos(geo.NewPos(100, 100)); g == nil || g.ID != "g2" {
		t.Fatalf("expected smallest group g2, got %v", g)
	}
	// Collapsed groups are not drop targets.
	c.Graph.Groups["g2"].Collapsed = true
	if g := c.groupAtGraphPos(geo.NewPos(100, 100)); g == nil || g.ID != "g1" {
		t.Fatalf("expected g1 once g2 collapsed, got %v", g)
	}
}

func TestRectBorderPoint(t *testing.T) {
	r := geo.NewRectangle(0, 0, 100, 50)
	near := func(p geo.Position, x, y float32) bool {
		const eps = 1e-4
		return abs32(p.X-x) < eps && abs32(p.Y-y) < eps
	}

	// Cardinal directions from center (50, 25).
	if p := rectBorderPoint(r, geo.NewPos(200, 25)); !near(p, 100, 25) {
		t.Fatalf("right border point: %v", p)
	}
	if p := rectBorderPoint(r, geo.NewPos(-200, 25)); !near(p, 0, 25) {
		t.Fatalf("left border point: %v", p)
	}
	if p := rectBorderPoint(r, geo.NewPos(50, -100)); !near(p, 50, 0) {
		t.Fatalf("top border point: %v", p)
	}
	// Diagonal: toward (150, 125) from (50, 25) — dy limits first (25/100 < 50/100).
	if p := rectBorderPoint(r, geo.NewPos(150, 125)); abs32(p.Y-50) > 1e-4 {
		t.Fatalf("diagonal border point should hit bottom edge: %v", p)
	}
	// Degenerate: toward == center returns center.
	if p := rectBorderPoint(r, geo.NewPos(50, 25)); !near(p, 50, 25) {
		t.Fatalf("degenerate case: %v", p)
	}
}

func TestHitTestSegment(t *testing.T) {
	a := geo.NewPos(0, 0)
	b := geo.NewPos(100, 0)
	if !hitTestSegment(a, b, geo.NewPos(50, 5), 8) {
		t.Fatal("expected hit at 5px from segment")
	}
	if hitTestSegment(a, b, geo.NewPos(50, 20), 8) {
		t.Fatal("expected miss at 20px from segment")
	}
	if hitTestSegment(a, b, geo.NewPos(150, 0), 8) {
		t.Fatal("expected miss beyond segment end")
	}
}
