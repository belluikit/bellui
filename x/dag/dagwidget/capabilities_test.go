package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/dag/dag"
)

func TestNewDAGCanvasDefaultsToAllCapabilities(t *testing.T) {
	c := newTestCanvas(t)
	if c.Capabilities != AllCapabilities() {
		t.Fatalf("default capabilities: %+v", c.Capabilities)
	}
}

func TestDeleteSelectedRespectsCapabilities(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	addTestNode(t, c, "b")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(c.Graph.AddEdge(&dag.Edge{ID: "e1", Source: dag.PortID{NodeID: "a", Name: "out"}, Target: dag.PortID{NodeID: "b", Name: "in"}}))
	c.edges["e1"] = NewEdgeWidget("e1", dag.PortID{NodeID: "a", Name: "out"}, dag.PortID{NodeID: "b", Name: "in"}, c)
	must(c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}))
	must(c.AddGroup(&dag.Group{ID: "g1", Label: "G"}))
	must(c.AddGroup(&dag.Group{ID: "g2", Label: "H"}))
	must(c.AddGroupEdge(&dag.GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}))
	must(c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "note"}))

	// Select everything, forbid everything: nothing may be deleted.
	c.nodes["a"].Selected = true
	c.edges["e1"].Selected = true
	c.linkEdges["l1"].Selected = true
	c.groups["g1"].Selected = true
	c.groupEdges["ge1"].Selected = true
	c.annotations["an1"].Selected = true
	c.Capabilities = Capabilities{}
	c.DeleteSelected()

	if len(c.Graph.Nodes) != 2 || len(c.Graph.Edges) != 1 || len(c.Graph.LinkEdges) != 1 ||
		len(c.Graph.Groups) != 2 || len(c.Graph.GroupEdges) != 1 || len(c.Graph.Annotations) != 1 {
		t.Fatalf("read-only canvas deleted something: %d nodes %d edges %d linkEdges %d groups %d groupEdges %d annotations",
			len(c.Graph.Nodes), len(c.Graph.Edges), len(c.Graph.LinkEdges),
			len(c.Graph.Groups), len(c.Graph.GroupEdges), len(c.Graph.Annotations))
	}

	// Allow only link-edge editing: exactly the link edge goes.
	c.Capabilities = Capabilities{LinkEdges: true}
	c.DeleteSelected()
	if len(c.Graph.LinkEdges) != 0 {
		t.Fatal("link edge not deleted with LinkEdges capability on")
	}
	if len(c.Graph.Nodes) != 2 || len(c.Graph.Edges) != 1 || len(c.Graph.Groups) != 2 ||
		len(c.Graph.GroupEdges) != 1 || len(c.Graph.Annotations) != 1 {
		t.Fatal("LinkEdges capability leaked into other kinds")
	}
}

func TestAddEntryPointsRespectCapabilities(t *testing.T) {
	c := newTestCanvas(t)
	c.Resize(geo.NewSize(400, 300))
	c.Capabilities = Capabilities{}

	comp := &dag.Component{ID: "x", Name: "X"}
	if _, err := c.AddComponentNode(comp, [2]float32{0, 0}); err == nil {
		t.Fatal("AddComponentNode succeeded with AddNodes off")
	}
	if len(c.Graph.Nodes) != 0 {
		t.Fatal("node added despite AddNodes off")
	}

	c.AddGroupAtCenter()
	if len(c.Graph.Groups) != 0 {
		t.Fatal("group added despite EditGroups off")
	}
	c.AddAnnotationAtCenter()
	if len(c.Graph.Annotations) != 0 {
		t.Fatal("annotation added despite Annotations off")
	}
}

func TestConnectionDragsRespectCapabilities(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.Capabilities = Capabilities{}

	c.startConnection(dag.PortID{NodeID: "a", Name: "out"}, geo.NewPos(0, 0))
	if c.dragState != dragNone || c.pendingEdge != nil {
		t.Fatal("startConnection entered dragConnecting with ConnectEdges off")
	}

	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G"}); err == nil {
		c.startGroupEdgeConnection("g1", geo.NewPos(0, 0))
		if c.dragState != dragNone || c.pendingGroupLink != nil {
			t.Fatal("startGroupEdgeConnection entered drag with GroupEdges off")
		}
	}
}

func TestNodeDragRespectsMoveResize(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	c.Capabilities = Capabilities{}

	before := n.Position
	c.nodes["a"].Dragged(&event.DragEvent{Dragged: geo.Delta{DX: 50, DY: 30}})
	if n.Position != before {
		t.Fatalf("node moved with MoveResize off: %v", n.Position)
	}
}

func TestNodeDropRespectsGroupMembership(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G", Position: [2]float32{200, 200}, Size: [2]float32{400, 300}}); err != nil {
		t.Fatal(err)
	}
	caps := AllCapabilities()
	caps.GroupMembership = false
	c.Capabilities = caps

	// Drag the node's center well inside the group and drop it.
	nw := c.nodes["a"]
	nw.Dragged(&event.DragEvent{Dragged: geo.Delta{DX: 300, DY: 300}})
	nw.DragEnd()

	if got := c.Graph.GroupOf("a"); got != "" {
		t.Fatalf("membership changed with GroupMembership off: %q", got)
	}
	if n.Position == ([2]float32{0, 0}) {
		t.Fatal("move itself should still work with MoveResize on")
	}
	if c.Commands == nil || !c.Commands.CanUndo() {
		t.Fatal("move command not recorded")
	}
}

func TestPortDragRespectsConnectEdges(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.Capabilities = Capabilities{}

	out := c.nodes["a"].Outputs[0]
	out.Dragged(&event.DragEvent{})
	if c.dragState != dragNone {
		t.Fatal("port drag entered dragConnecting with ConnectEdges off")
	}
}

func TestSetPropRespectsEditProperties(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")

	p := NewPropertiesPanel(nil, c.Commands)
	p.CanvasRef = c
	p.SetNode(n)

	caps := AllCapabilities()
	caps.EditProperties = false
	c.Capabilities = caps
	p.setProp("key", "value")
	if len(n.Metadata) != 0 {
		t.Fatalf("metadata written with EditProperties off: %s", n.Metadata)
	}

	c.Capabilities = AllCapabilities()
	p.setProp("key", "value")
	if len(n.Metadata) == 0 {
		t.Fatal("metadata not written with EditProperties on")
	}
}

// Resize handles must react to a mid-session Capabilities flip on the next
// canvas Refresh, even though children are not individually refreshed
// (#22 — same staleness the connector handles had before PR #21).
func TestResizeHandlesFollowCapabilities(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G", Position: [2]float32{0, 200}, Size: [2]float32{200, 120}}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "note", Position: [2]float32{250, 200}, Size: [2]float32{120, 60}}); err != nil {
		t.Fatal(err)
	}
	w := test.NewWindow(c)
	defer w.Close()
	w.Resize(geo.NewSize(500, 500))

	c.nodes["a"].Selected = true
	c.groups["g1"].Selected = true
	c.annotations["an1"].Selected = true
	c.Refresh()
	handleSets := map[string][]*HandleWidget{
		"node":       c.nodes["a"].handles,
		"group":      c.groups["g1"].handles,
		"annotation": c.annotations["an1"].handles,
	}
	for kind, hs := range handleSets {
		if !hs[0].Visible() {
			t.Fatalf("%s handles hidden while selected with MoveResize on", kind)
		}
	}

	caps := AllCapabilities()
	caps.MoveResize = false
	c.Capabilities = caps
	c.Refresh()
	for kind, hs := range handleSets {
		if hs[0].Visible() {
			t.Fatalf("%s handles still visible after MoveResize turned off", kind)
		}
	}

	c.Capabilities = AllCapabilities()
	c.Refresh()
	for kind, hs := range handleSets {
		if !hs[0].Visible() {
			t.Fatalf("%s handles did not reappear after re-enabling MoveResize", kind)
		}
	}
}
