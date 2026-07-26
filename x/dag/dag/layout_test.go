package dag

import (
	"testing"
)

func chainGraph(t *testing.T, ids ...string) *Graph {
	t.Helper()
	g := NewGraph("g", "test")
	for _, id := range ids {
		g.AddNode(&Node{
			ID:      id,
			Inputs:  []Port{{Name: "in", Direction: PortInput}},
			Outputs: []Port{{Name: "out", Direction: PortOutput}},
		})
	}
	for i := 0; i < len(ids)-1; i++ {
		err := g.AddEdge(&Edge{
			ID:     "e" + ids[i] + ids[i+1],
			Source: PortID{NodeID: ids[i], Name: "out"},
			Target: PortID{NodeID: ids[i+1], Name: "in"},
		})
		if err != nil {
			t.Fatalf("AddEdge: %v", err)
		}
	}
	return g
}

func TestLayeredLayoutChain(t *testing.T) {
	g := chainGraph(t, "a", "b", "c")

	pos, err := LayeredLayout(g, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(pos))
	}
	// Left-to-right flow with increasing x, constant y.
	if !(pos["a"][0] < pos["b"][0] && pos["b"][0] < pos["c"][0]) {
		t.Fatalf("x should increase along the chain: %v", pos)
	}
	if pos["a"][1] != pos["b"][1] || pos["b"][1] != pos["c"][1] {
		t.Fatalf("chain should stay on one row: %v", pos)
	}
}

func TestLayeredLayoutDiamond(t *testing.T) {
	g := chainGraph(t, "a") // just node a
	for _, id := range []string{"b", "c", "d"} {
		g.AddNode(&Node{
			ID:      id,
			Inputs:  []Port{{Name: "in", Direction: PortInput}, {Name: "in2", Direction: PortInput}},
			Outputs: []Port{{Name: "out", Direction: PortOutput}},
		})
	}
	edges := []struct{ src, tgt, tgtPort string }{
		{"a", "b", "in"},
		{"a", "c", "in"},
		{"b", "d", "in"},
		{"c", "d", "in2"},
	}
	for _, e := range edges {
		err := g.AddEdge(&Edge{
			ID:     "e" + e.src + e.tgt,
			Source: PortID{NodeID: e.src, Name: "out"},
			Target: PortID{NodeID: e.tgt, Name: e.tgtPort},
		})
		if err != nil {
			t.Fatalf("AddEdge %s->%s: %v", e.src, e.tgt, err)
		}
	}

	pos, err := LayeredLayout(g, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	// b and c share the middle layer: same x, different y.
	if pos["b"][0] != pos["c"][0] {
		t.Fatalf("b and c should share a layer: %v", pos)
	}
	if pos["b"][1] == pos["c"][1] {
		t.Fatalf("b and c should not overlap: %v", pos)
	}
	if !(pos["a"][0] < pos["b"][0] && pos["b"][0] < pos["d"][0]) {
		t.Fatalf("layers should flow left to right: %v", pos)
	}
}

func TestLayeredLayoutLongestPathRank(t *testing.T) {
	// a -> b -> c and a -> c: c must land after b, not beside it.
	g := chainGraph(t, "a", "b", "c")
	g.Nodes["c"].Inputs = append(g.Nodes["c"].Inputs, Port{Name: "in2", Direction: PortInput})
	err := g.AddEdge(&Edge{
		ID:     "eac",
		Source: PortID{NodeID: "a", Name: "out"},
		Target: PortID{NodeID: "c", Name: "in2"},
	})
	if err != nil {
		t.Fatal(err)
	}

	pos, err := LayeredLayout(g, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !(pos["b"][0] > pos["a"][0] && pos["c"][0] > pos["b"][0]) {
		t.Fatalf("longest-path ranking should place c after b: %v", pos)
	}
}

func TestLayeredLayoutDeterministic(t *testing.T) {
	g := chainGraph(t, "a", "b", "c")
	first, err := LayeredLayout(g, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := LayeredLayout(g, DefaultLayoutOptions())
		if err != nil {
			t.Fatal(err)
		}
		for id, p := range first {
			if again[id] != p {
				t.Fatalf("layout not deterministic for %s: %v vs %v", id, p, again[id])
			}
		}
	}
}

func TestLayeredLayoutEmpty(t *testing.T) {
	g := NewGraph("g", "empty")
	pos, err := LayeredLayout(g, DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 0 {
		t.Fatalf("expected no positions, got %v", pos)
	}
}

// linkChainGraph builds port-less nodes connected only by link edges.
func linkChainGraph(t *testing.T, ids ...string) *Graph {
	t.Helper()
	g := NewGraph("g", "test")
	for _, id := range ids {
		g.AddNode(&Node{ID: id})
	}
	for i := 0; i < len(ids)-1; i++ {
		err := g.AddLinkEdge(&LinkEdge{
			ID:     "l" + ids[i] + ids[i+1],
			Source: ids[i],
			Target: ids[i+1],
		})
		if err != nil {
			t.Fatalf("AddLinkEdge: %v", err)
		}
	}
	return g
}

func TestLayeredLayoutRankLinkEdges(t *testing.T) {
	g := linkChainGraph(t, "a", "b", "c")

	// Default: link edges ignored, everything collapses to one column.
	opts := DefaultLayoutOptions()
	pos, err := LayeredLayout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	if pos["a"][0] != pos["b"][0] || pos["b"][0] != pos["c"][0] {
		t.Fatalf("without RankLinkEdges all nodes should share one column: %v", pos)
	}

	// With the flag, the link chain ranks like a dataflow chain.
	opts.RankLinkEdges = true
	pos, err = LayeredLayout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !(pos["a"][0] < pos["b"][0] && pos["b"][0] < pos["c"][0]) {
		t.Fatalf("x should increase along the link chain: %v", pos)
	}
}

func TestLayeredLayoutRankLinkEdgesCycleTolerant(t *testing.T) {
	// Opposite-direction link edges are legal and form a 2-cycle; c hangs off b.
	g := linkChainGraph(t, "a", "b", "c")
	err := g.AddLinkEdge(&LinkEdge{ID: "lba", Source: "b", Target: "a"})
	if err != nil {
		t.Fatal(err)
	}

	opts := DefaultLayoutOptions()
	opts.RankLinkEdges = true
	first, err := LayeredLayout(g, opts)
	if err != nil {
		t.Fatalf("cycle among link edges must not error: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("expected 3 positions, got %v", first)
	}
	// The cycle breaks at the smallest ID: a still precedes b, b precedes c.
	if !(first["a"][0] < first["b"][0] && first["b"][0] < first["c"][0]) {
		t.Fatalf("broken cycle should still flow a<b<c: %v", first)
	}
	for i := 0; i < 5; i++ {
		again, err := LayeredLayout(g, opts)
		if err != nil {
			t.Fatal(err)
		}
		for id, p := range first {
			if again[id] != p {
				t.Fatalf("layout not deterministic for %s: %v vs %v", id, p, again[id])
			}
		}
	}
}

func TestLayeredLayoutRankLinkEdgesMixed(t *testing.T) {
	// Dataflow a->b plus link edge b->c: c must rank after b.
	g := chainGraph(t, "a", "b")
	g.AddNode(&Node{ID: "c"})
	if err := g.AddLinkEdge(&LinkEdge{ID: "lbc", Source: "b", Target: "c"}); err != nil {
		t.Fatal(err)
	}

	opts := DefaultLayoutOptions()
	opts.RankLinkEdges = true
	pos, err := LayeredLayout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !(pos["a"][0] < pos["b"][0] && pos["b"][0] < pos["c"][0]) {
		t.Fatalf("link edge should extend the dataflow chain: %v", pos)
	}
}

func TestNeedsLayout(t *testing.T) {
	g := NewGraph("g", "test")
	if NeedsLayout(g) {
		t.Fatal("empty graph never needs layout")
	}

	g.AddNode(&Node{ID: "a"})
	if NeedsLayout(g) {
		t.Fatal("single node never needs layout")
	}

	g.AddNode(&Node{ID: "b"})
	if !NeedsLayout(g) {
		t.Fatal("two nodes at the zero position need layout")
	}

	g.Nodes["b"].Position = [2]float32{100, 50}
	if NeedsLayout(g) {
		t.Fatal("distinct stored positions should be preserved")
	}
}
