package dag

import (
	"testing"

	pb "github.com/belluikit/bellui/x/dag/dag/dagpb"
)

func TestGraphProtoRoundtrip(t *testing.T) {
	g := NewGraph("p1", "Pipeline")
	a := makeTestNode("a", nil, []string{"out"})
	a.Metadata = []byte(`{"key":"value","n":3}`)
	g.AddNode(a)
	g.AddNode(makeTestNode("b", []string{"in"}, nil))
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})
	g.AddGroup(&Group{ID: "g1", Label: "G1", Members: []string{"a"}, Collapsed: true, Color: "#112233"})
	g.AddGroup(&Group{ID: "g2", Label: "G2"})
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}); err != nil {
		t.Fatal(err)
	}
	// Link edges in both directions — legal precisely because they are
	// exempt from the acyclicity invariant.
	if err := g.AddLinkEdge(&LinkEdge{ID: "l1", Source: "a", Target: "b", Label: "navigates to"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddLinkEdge(&LinkEdge{ID: "l2", Source: "b", Target: "a"}); err != nil {
		t.Fatal(err)
	}
	anns := []*Annotation{
		{ID: "an1", Text: "floating"},
		{ID: "an2", Text: "n", AnchorKind: AnchorNode, AnchorID: "a"},
		{ID: "an3", Text: "e", AnchorKind: AnchorEdge, AnchorID: "e1"},
		{ID: "an4", Text: "g", AnchorKind: AnchorGroup, AnchorID: "g1"},
		{ID: "an5", Text: "l", AnchorKind: AnchorLinkEdge, AnchorID: "l1"},
	}
	for _, an := range anns {
		if err := g.AddAnnotation(an); err != nil {
			t.Fatal(err)
		}
	}

	g2 := GraphFromProto(GraphToProto(g))

	if len(g2.Nodes) != 2 || len(g2.Edges) != 1 {
		t.Fatalf("node/edge counts: %d/%d", len(g2.Nodes), len(g2.Edges))
	}
	if len(g2.Groups) != 2 || len(g2.GroupEdges) != 1 || len(g2.Annotations) != 5 {
		t.Fatalf("group/groupEdge/annotation counts: %d/%d/%d",
			len(g2.Groups), len(g2.GroupEdges), len(g2.Annotations))
	}
	if len(g2.LinkEdges) != 2 {
		t.Fatalf("link edge count: %d", len(g2.LinkEdges))
	}
	if le := g2.LinkEdges["l1"]; le == nil || le.Source != "a" || le.Target != "b" || le.Label != "navigates to" {
		t.Fatalf("link edge l1 mismatch: %+v", le)
	}
	if le := g2.LinkEdges["l2"]; le == nil || le.Source != "b" || le.Target != "a" {
		t.Fatalf("link edge l2 mismatch: %+v", le)
	}
	if len(g2.Nodes["a"].Metadata) == 0 {
		t.Fatal("node metadata dropped")
	}
	grp := g2.Groups["g1"]
	if grp.Label != "G1" || !grp.Collapsed || grp.Color != "#112233" ||
		len(grp.Members) != 1 || grp.Members[0] != "a" {
		t.Fatalf("group mismatch: %+v", grp)
	}
	for _, want := range anns {
		got := g2.Annotations[want.ID]
		if got == nil || got.AnchorKind != want.AnchorKind || got.AnchorID != want.AnchorID {
			t.Fatalf("annotation %s mismatch: %+v", want.ID, got)
		}
	}
}

func TestAnchorKindProtoMapping(t *testing.T) {
	kinds := []AnchorKind{AnchorNone, AnchorNode, AnchorEdge, AnchorGroup, AnchorLinkEdge}
	for _, k := range kinds {
		if got := anchorKindFromProto(anchorKindToProto(k)); got != k {
			t.Fatalf("anchor kind %d round-trips to %d", k, got)
		}
	}
	if got := anchorKindFromProto(pb.AnchorKind(99)); got != AnchorNone {
		t.Fatalf("unknown proto kind should map to AnchorNone, got %d", got)
	}
	if got := anchorKindToProto(AnchorKind(99)); got != pb.AnchorKind_ANCHOR_KIND_UNSPECIFIED {
		t.Fatalf("unknown dag kind should map to UNSPECIFIED, got %d", got)
	}
}
