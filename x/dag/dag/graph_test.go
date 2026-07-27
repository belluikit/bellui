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

func TestSerializationRoundtrip(t *testing.T) {
	g := NewGraph("test", "Test Graph")
	a := makeTestNode("a", nil, []string{"out"})
	a.Position = [2]float32{10, 20}
	a.Size = [2]float32{220, 120}
	a.Metadata = []byte(`{"threshold":5}`)
	g.AddNode(a)
	g.AddNode(makeTestNode("b", []string{"in"}, nil))
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})

	g.AddGroup(&Group{
		ID: "g1", Label: "Stage 1", Members: []string{"a"},
		Position: [2]float32{5, 5}, Size: [2]float32{300, 200},
		Collapsed: true, Color: "#336699",
	})
	g.AddGroup(&Group{ID: "g2", Label: "Stage 2"})
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}); err != nil {
		t.Fatal(err)
	}
	g.AddAnnotation(&Annotation{ID: "an1", Text: "floating\nnote", Position: [2]float32{50, 60}, Size: [2]float32{160, 60}})
	if err := g.AddAnnotation(&Annotation{ID: "an2", Text: "about g1", AnchorKind: AnchorGroup, AnchorID: "g1"}); err != nil {
		t.Fatal(err)
	}

	data, err := MarshalGraph(g)
	if err != nil {
		t.Fatal(err)
	}

	g2, err := UnmarshalGraph(data)
	if err != nil {
		t.Fatal(err)
	}

	if g2.ID != g.ID || g2.Name != g.Name {
		t.Fatalf("graph metadata mismatch")
	}
	if len(g2.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g2.Nodes))
	}
	if len(g2.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g2.Edges))
	}

	e := g2.Edges["e1"]
	if e.Source.NodeID != "a" || e.Target.NodeID != "b" {
		t.Fatal("edge data mismatch")
	}

	a2 := g2.Nodes["a"]
	if a2.Position != a.Position || a2.Size != a.Size {
		t.Fatalf("node geometry mismatch: pos=%v size=%v", a2.Position, a2.Size)
	}
	if len(a2.Metadata) == 0 {
		t.Fatal("node metadata lost in round-trip")
	}

	if len(g2.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(g2.Groups))
	}
	g1 := g2.Groups["g1"]
	if g1 == nil || g1.Label != "Stage 1" || !g1.Collapsed || g1.Color != "#336699" {
		t.Fatalf("group g1 mismatch: %+v", g1)
	}
	if g1.Position != [2]float32{5, 5} || g1.Size != [2]float32{300, 200} {
		t.Fatalf("group geometry mismatch: pos=%v size=%v", g1.Position, g1.Size)
	}
	if len(g1.Members) != 1 || g1.Members[0] != "a" {
		t.Fatalf("group members mismatch: %v", g1.Members)
	}
	if len(g2.GroupEdges) != 1 {
		t.Fatalf("expected 1 group edge, got %d", len(g2.GroupEdges))
	}
	ge := g2.GroupEdges["ge1"]
	if ge.Source != "g1" || ge.Target != "g2" {
		t.Fatalf("group edge mismatch: %+v", ge)
	}

	if len(g2.Annotations) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(g2.Annotations))
	}
	an1 := g2.Annotations["an1"]
	if an1.Text != "floating\nnote" || an1.AnchorKind != AnchorNone {
		t.Fatalf("annotation an1 mismatch: %+v", an1)
	}
	if an1.Position != [2]float32{50, 60} || an1.Size != [2]float32{160, 60} {
		t.Fatalf("annotation geometry mismatch: %+v", an1)
	}
	an2 := g2.Annotations["an2"]
	if an2.AnchorKind != AnchorGroup || an2.AnchorID != "g1" {
		t.Fatalf("annotation an2 anchor mismatch: %+v", an2)
	}
}
