package dag

import (
	"errors"
	"testing"
)

// linkedPairGraph returns a graph with two unconnected nodes a and b.
func linkedPairGraph(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, []string{"out"}))
	return g
}

func TestAddLinkEdge(t *testing.T) {
	g := linkedPairGraph(t)
	if err := g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b", Label: "goes to"}); err != nil {
		t.Fatal(err)
	}
	if le := g.LinkEdges["l1"]; le == nil || le.Label != "goes to" {
		t.Fatalf("link edge not stored: %+v", g.LinkEdges["l1"])
	}
}

func TestAddLinkEdgeErrors(t *testing.T) {
	g := linkedPairGraph(t)
	g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b"})

	if err := g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b"}); !errors.Is(err, ErrLinkEdgeExists) {
		t.Fatalf("duplicate ID: got %v", err)
	}
	if err := g.AddLinkEdge(&LinkEdge{ID: "l2", Source: "missing", Target: "b"}); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing source: got %v", err)
	}
	if err := g.AddLinkEdge(&LinkEdge{ID: "l2", Source: "a", Target: "missing"}); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing target: got %v", err)
	}
	if err := g.AddLinkEdge(&LinkEdge{ID: "l2", Source: "a", Target: "a"}); !errors.Is(err, ErrLinkEdgeSelfLoop) {
		t.Fatalf("self-loop: got %v", err)
	}
}

func TestLinkEdgeParallelAndBidirectional(t *testing.T) {
	g := linkedPairGraph(t)
	for _, le := range []*LinkEdge{
		{ID: "l1", Source: "a", Target: "b"},
		{ID: "l2", Source: "a", Target: "b"}, // parallel, same direction
		{ID: "l3", Source: "b", Target: "a"}, // opposite direction
	} {
		if err := g.AddLinkEdge(le); err != nil {
			t.Fatalf("%s: %v", le.ID, err)
		}
	}
	if len(g.LinkEdges) != 3 {
		t.Fatalf("want 3 link edges, got %d", len(g.LinkEdges))
	}
}

func TestRemoveLinkEdge(t *testing.T) {
	g := linkedPairGraph(t)
	g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b"})
	if err := g.AddAnnotation(&Annotation{ID: "an1", Text: "note", AnchorKind: AnchorLinkEdge, AnchorID: "l1"}); err != nil {
		t.Fatal(err)
	}

	if err := g.RemoveLinkEdge("l1"); err != nil {
		t.Fatal(err)
	}
	if len(g.LinkEdges) != 0 {
		t.Fatal("link edge not removed")
	}
	if len(g.Annotations) != 0 {
		t.Fatal("anchored annotation did not cascade")
	}
	if err := g.RemoveLinkEdge("l1"); !errors.Is(err, ErrLinkEdgeNotFound) {
		t.Fatalf("remove unknown: got %v", err)
	}
}

func TestRemoveNodeCascadesLinkEdges(t *testing.T) {
	g := linkedPairGraph(t)
	g.AddNode(makeTestNode("c", []string{"in"}, []string{"out"}))
	g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b"})
	g.AddLinkEdge(&LinkEdge{ID: "l2", Source: "b", Target: "c"})
	g.AddAnnotation(&Annotation{ID: "an1", Text: "note", AnchorKind: AnchorLinkEdge, AnchorID: "l1"})

	if got := len(g.LinkEdgesForNode("b")); got != 2 {
		t.Fatalf("LinkEdgesForNode(b): want 2, got %d", got)
	}

	if _, err := g.RemoveNode("a"); err != nil {
		t.Fatal(err)
	}
	if g.LinkEdges["l1"] != nil {
		t.Fatal("link edge touching removed node survived")
	}
	if g.LinkEdges["l2"] == nil {
		t.Fatal("unrelated link edge removed")
	}
	if g.Annotations["an1"] != nil {
		t.Fatal("annotation anchored to cascaded link edge survived")
	}
}

func TestAnnotationLinkEdgeAnchorValidation(t *testing.T) {
	g := linkedPairGraph(t)
	g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b"})

	if err := g.AddAnnotation(&Annotation{ID: "an1", AnchorKind: AnchorLinkEdge, AnchorID: "l1"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddAnnotation(&Annotation{ID: "an2", AnchorKind: AnchorLinkEdge, AnchorID: "missing"}); !errors.Is(err, ErrAnchorNotFound) {
		t.Fatalf("missing anchor: got %v", err)
	}
}

// TestLinkEdgePXFRoundtrip exercises the full save/load path (PXF bytes, not
// just proto structs) with bidirectional link edges — the acceptance case.
func TestLinkEdgesExemptFromCycleChecks(t *testing.T) {
	g := linkedPairGraph(t)
	g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b"})
	g.AddLinkEdge(&LinkEdge{ID: "l2", Source: "b", Target: "a"})

	// A data-flow edge a→b must still be legal despite link edge b→a.
	if err := g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}}); err != nil {
		t.Fatalf("link edges leaked into cycle detection: %v", err)
	}
	if g.WouldCycle("a", "b") {
		t.Fatal("WouldCycle consulted link edges")
	}
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder failed with cyclic link edges: %v", err)
	}
	if len(order) != 2 || order[0].ID != "a" || order[1].ID != "b" {
		t.Fatalf("unexpected order: %v", order)
	}
}
