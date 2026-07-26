package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/x/dag/dag"
)

// connectedPair builds a canvas with nodes a, b and an a→b edge added
// through the command stack.
func connectedPair(t *testing.T) (*DAGCanvas, *dag.Edge) {
	t.Helper()
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	addTestNode(t, c, "b")
	edge := &dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}
	c.Commands.Do(&AddEdgeCommand{Canvas: c, Edge: edge})
	if len(c.Graph.Edges) != 1 || c.edges["e1"] == nil {
		t.Fatal("setup edge not added")
	}
	return c, edge
}

func TestAddEdgeCommandRejectedIsNotRecorded(t *testing.T) {
	c, _ := connectedPair(t)

	// b→a would close a cycle: the model rejects it, so the command must
	// leave no trace and must not land on the undo stack.
	rejected := &dag.Edge{
		ID:     "e2",
		Source: dag.PortID{NodeID: "b", Name: "out"},
		Target: dag.PortID{NodeID: "a", Name: "in"},
	}
	c.Commands.Do(&AddEdgeCommand{Canvas: c, Edge: rejected})

	if len(c.Graph.Edges) != 1 {
		t.Fatalf("edge count = %d, want 1", len(c.Graph.Edges))
	}
	if c.edges["e2"] != nil {
		t.Fatal("widget created for a rejected edge")
	}
	// The only recorded command must still be the setup add — undoing once
	// removes e1, proving the rejected command occupies no undo slot.
	c.Commands.Undo()
	if len(c.Graph.Edges) != 0 {
		t.Fatal("undo did not pop the setup add: a rejected command was recorded")
	}
	if c.Commands.CanUndo() {
		t.Fatal("undo stack not empty: the rejected command was recorded")
	}
}

func TestEdgeCommandsSyncPortConnections(t *testing.T) {
	c, edge := connectedPair(t)
	out, in := c.nodes["a"].Outputs[0], c.nodes["b"].Inputs[0]

	if !out.Connected || !in.Connected {
		t.Fatal("ports not marked connected after AddEdgeCommand")
	}
	c.Commands.Do(&RemoveEdgeCommand{Canvas: c, Edge: edge})
	if out.Connected || in.Connected {
		t.Fatal("ports still marked connected after RemoveEdgeCommand")
	}
	c.Commands.Undo()
	if !out.Connected || !in.Connected {
		t.Fatal("ports not re-marked connected after undoing the removal")
	}
}

func TestGraphChangeNotificationsFireExactlyOnce(t *testing.T) {
	c := newTestCanvas(t)
	count := 0
	c.OnGraphChanged = func() { count++ }
	step := func(what string, want int, op func()) {
		t.Helper()
		count = 0
		op()
		if count != want {
			t.Fatalf("%s: %d notifications, want %d", what, count, want)
		}
	}

	node := func(id string) *dag.Node {
		return &dag.Node{
			ID: id, Label: id,
			Inputs:  []dag.Port{{Name: "in", Direction: dag.PortInput}},
			Outputs: []dag.Port{{Name: "out", Direction: dag.PortOutput}},
		}
	}

	step("programmatic AddNode", 1, func() {
		if err := c.AddNode(node("a")); err != nil {
			t.Fatal(err)
		}
	})
	step("Do(AddNodeCommand)", 1, func() {
		c.Commands.Do(&AddNodeCommand{Canvas: c, Node: node("b")})
	})
	step("Do(AddEdgeCommand)", 1, func() {
		c.Commands.Do(&AddEdgeCommand{Canvas: c, Edge: &dag.Edge{
			ID:     "e1",
			Source: dag.PortID{NodeID: "a", Name: "out"},
			Target: dag.PortID{NodeID: "b", Name: "in"},
		}})
	})
	// RemoveNodeCommand cascades (edge widgets, port sync) through the
	// silent internals — still exactly one notification.
	step("Do(RemoveNodeCommand)", 1, func() {
		c.Commands.Do(&RemoveNodeCommand{Canvas: c, Node: c.Graph.Nodes["b"]})
	})
	step("Undo", 1, func() { c.Commands.Undo() })
	step("Redo", 1, func() { c.Commands.Redo() })
	step("rejected Do", 0, func() {
		c.Commands.Do(&AddNodeCommand{Canvas: c, Node: node("a")}) // duplicate ID
	})
	for c.Commands.CanUndo() {
		c.Commands.Undo()
	}
	step("Undo on empty stack", 0, func() { c.Commands.Undo() })
	for c.Commands.CanRedo() {
		c.Commands.Redo()
	}
	step("Redo on empty stack", 0, func() { c.Commands.Redo() })
	step("PushDone", 1, func() {
		c.Commands.PushDone(&MoveNodeCommand{Canvas: c, NodeID: "a", OldPos: [2]float32{0, 0}, NewPos: [2]float32{10, 10}})
	})
}

func TestRejectedCommandsAreNotRecorded(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a") // programmatic: leaves the stack empty

	for _, cmd := range []Command{
		&AddNodeCommand{Canvas: c, Node: &dag.Node{ID: "a"}},                                      // duplicate ID
		&AddLinkEdgeCommand{Canvas: c, Edge: &dag.LinkEdge{ID: "l", Source: "a", Target: "a"}},    // self-loop
		&AddGroupEdgeCommand{Canvas: c, Edge: &dag.GroupEdge{ID: "ge", Source: "x", Target: "y"}}, // missing groups
		&RemoveEdgeCommand{Canvas: c, Edge: &dag.Edge{ID: "missing"}},
		&RemoveNodeCommand{Canvas: c, Node: &dag.Node{ID: "ghost"}},
		&RemoveGroupCommand{Canvas: c, Group: &dag.Group{ID: "ghost"}},
	} {
		c.Commands.Do(cmd)
		if c.Commands.CanUndo() {
			t.Fatalf("%T recorded despite rejection", cmd)
		}
	}
	if len(c.Graph.Nodes) != 1 || len(c.Graph.LinkEdges) != 0 || len(c.Graph.GroupEdges) != 0 {
		t.Fatal("a rejected command mutated the graph")
	}
}

func TestLoadGraphSyncsPortConnections(t *testing.T) {
	c, _ := connectedPair(t)

	g := dag.NewGraph("g2", "Other")
	for _, id := range []string{"x", "y"} {
		if err := g.AddNode(&dag.Node{
			ID: id, Label: id,
			Inputs:  []dag.Port{{Name: "in", Direction: dag.PortInput}},
			Outputs: []dag.Port{{Name: "out", Direction: dag.PortOutput}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.AddEdge(&dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "x", Name: "out"},
		Target: dag.PortID{NodeID: "y", Name: "in"},
	}); err != nil {
		t.Fatal(err)
	}

	c.LoadGraph(g)
	if !c.nodes["x"].Outputs[0].Connected || !c.nodes["y"].Inputs[0].Connected {
		t.Fatal("loaded graph's connected ports not marked connected")
	}
}
