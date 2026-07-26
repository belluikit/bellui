package dag

import (
	"testing"
)

func makeTestNode(id string, inputs, outputs []string) *Node {
	n := &Node{ID: id, Label: id}
	for _, name := range inputs {
		n.Inputs = append(n.Inputs, Port{Name: name, Direction: PortInput})
	}
	for _, name := range outputs {
		n.Outputs = append(n.Outputs, Port{Name: name, Direction: PortOutput})
	}
	return n
}

func TestAddRemoveNode(t *testing.T) {
	g := NewGraph("test", "Test")
	n := makeTestNode("a", []string{"in"}, []string{"out"})
	if err := g.AddNode(n); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(n); err == nil {
		t.Fatal("expected duplicate node error")
	}
	if _, err := g.RemoveNode("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.RemoveNode("a"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestAddEdgeValidation(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, nil))

	// Valid edge
	e := &Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}}
	if err := g.AddEdge(e); err != nil {
		t.Fatal(err)
	}

	// Duplicate edge ID
	if err := g.AddEdge(e); err == nil {
		t.Fatal("expected duplicate edge error")
	}

	// Target already connected
	e2 := &Edge{ID: "e2", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}}
	if err := g.AddEdge(e2); err == nil {
		t.Fatal("expected target occupied error")
	}
}

func TestAddEdgeDirectionValidation(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, []string{"out"}))

	// Wrong direction: input -> input
	e := &Edge{ID: "e1", Source: PortID{"a", "in"}, Target: PortID{"b", "in"}}
	if err := g.AddEdge(e); err == nil {
		t.Fatal("expected direction error")
	}
}

func TestAddEdgeNonexistentPort(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, nil))

	e := &Edge{ID: "e1", Source: PortID{"a", "nope"}, Target: PortID{"b", "in"}}
	if err := g.AddEdge(e); err == nil {
		t.Fatal("expected port not found error")
	}
}

func TestCycleDetection(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("c", []string{"in"}, []string{"out"}))

	// a -> b -> c
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})
	g.AddEdge(&Edge{ID: "e2", Source: PortID{"b", "out"}, Target: PortID{"c", "in"}})

	// c -> a would create cycle
	e3 := &Edge{ID: "e3", Source: PortID{"c", "out"}, Target: PortID{"a", "in"}}
	if err := g.AddEdge(e3); err == nil {
		t.Fatal("expected cycle error")
	}

	// Self-loop
	g2 := NewGraph("test2", "Test2")
	g2.AddNode(makeTestNode("x", []string{"in"}, []string{"out"}))
	eSelf := &Edge{ID: "es", Source: PortID{"x", "out"}, Target: PortID{"x", "in"}}
	if err := g2.AddEdge(eSelf); err == nil {
		t.Fatal("expected cycle error for self-loop")
	}
}

func TestRemoveNodeCascadesEdges(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("c", []string{"in"}, nil))

	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})
	g.AddEdge(&Edge{ID: "e2", Source: PortID{"b", "out"}, Target: PortID{"c", "in"}})

	removed, err := g.RemoveNode("b")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed edges, got %d", len(removed))
	}
	if len(g.Edges) != 0 {
		t.Fatalf("expected 0 remaining edges, got %d", len(g.Edges))
	}
}

func TestTopologicalOrder(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("c", []string{"in"}, nil))

	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})
	g.AddEdge(&Edge{ID: "e2", Source: PortID{"b", "out"}, Target: PortID{"c", "in"}})

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes in order, got %d", len(order))
	}

	// Verify ordering: a before b, b before c
	idx := make(map[string]int)
	for i, n := range order {
		idx[n.ID] = i
	}
	if idx["a"] >= idx["b"] || idx["b"] >= idx["c"] {
		t.Fatalf("invalid topological order: a=%d b=%d c=%d", idx["a"], idx["b"], idx["c"])
	}
}

func TestEdgesForNode(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("c", []string{"in"}, nil))

	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})
	g.AddEdge(&Edge{ID: "e2", Source: PortID{"b", "out"}, Target: PortID{"c", "in"}})

	edges := g.EdgesForNode("b")
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges for node b, got %d", len(edges))
	}

	edges = g.EdgesForNode("a")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge for node a, got %d", len(edges))
	}
}
