package dag

import (
	"errors"
	"testing"
)

func TestAddRemoveAnnotation(t *testing.T) {
	g := NewGraph("test", "Test")
	a := &Annotation{ID: "an1", Text: "hello", Position: [2]float32{5, 5}}
	if err := g.AddAnnotation(a); err != nil {
		t.Fatal(err)
	}
	if err := g.AddAnnotation(a); !errors.Is(err, ErrAnnotationExists) {
		t.Fatalf("expected ErrAnnotationExists, got %v", err)
	}
	if err := g.RemoveAnnotation("an1"); err != nil {
		t.Fatal(err)
	}
	if err := g.RemoveAnnotation("an1"); !errors.Is(err, ErrAnnotationNotFound) {
		t.Fatalf("expected ErrAnnotationNotFound, got %v", err)
	}
}

func TestAddAnnotationAnchorValidation(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, nil))
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})
	g.AddGroup(&Group{ID: "g1"})

	if err := g.AddAnnotation(&Annotation{ID: "x", AnchorKind: AnchorNode, AnchorID: "nope"}); !errors.Is(err, ErrAnchorNotFound) {
		t.Fatalf("expected ErrAnchorNotFound for node, got %v", err)
	}
	if err := g.AddAnnotation(&Annotation{ID: "x", AnchorKind: AnchorEdge, AnchorID: "nope"}); !errors.Is(err, ErrAnchorNotFound) {
		t.Fatalf("expected ErrAnchorNotFound for edge, got %v", err)
	}
	if err := g.AddAnnotation(&Annotation{ID: "x", AnchorKind: AnchorGroup, AnchorID: "nope"}); !errors.Is(err, ErrAnchorNotFound) {
		t.Fatalf("expected ErrAnchorNotFound for group, got %v", err)
	}
	if err := g.AddAnnotation(&Annotation{ID: "x", AnchorKind: AnchorNone, AnchorID: "a"}); !errors.Is(err, ErrInvalidAnchor) {
		t.Fatalf("expected ErrInvalidAnchor for floating with target, got %v", err)
	}
	if err := g.AddAnnotation(&Annotation{ID: "x", AnchorKind: AnchorKind(99), AnchorID: "a"}); !errors.Is(err, ErrInvalidAnchor) {
		t.Fatalf("expected ErrInvalidAnchor for unknown kind, got %v", err)
	}

	valid := []Annotation{
		{ID: "an1", AnchorKind: AnchorNone},
		{ID: "an2", AnchorKind: AnchorNode, AnchorID: "a"},
		{ID: "an3", AnchorKind: AnchorEdge, AnchorID: "e1"},
		{ID: "an4", AnchorKind: AnchorGroup, AnchorID: "g1"},
	}
	for i := range valid {
		if err := g.AddAnnotation(&valid[i]); err != nil {
			t.Fatalf("valid annotation %s rejected: %v", valid[i].ID, err)
		}
	}
}

func TestRemoveNodeDropsAnchoredAnnotations(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, nil))
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})

	g.AddAnnotation(&Annotation{ID: "onNode", AnchorKind: AnchorNode, AnchorID: "a"})
	g.AddAnnotation(&Annotation{ID: "onEdge", AnchorKind: AnchorEdge, AnchorID: "e1"})
	g.AddAnnotation(&Annotation{ID: "floating", AnchorKind: AnchorNone})

	if _, err := g.RemoveNode("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := g.Annotations["onNode"]; ok {
		t.Fatal("node-anchored annotation survived RemoveNode")
	}
	if _, ok := g.Annotations["onEdge"]; ok {
		t.Fatal("edge-anchored annotation survived removal of its edge's node")
	}
	if _, ok := g.Annotations["floating"]; !ok {
		t.Fatal("floating annotation removed")
	}
}

func TestRemoveEdgeDropsAnchoredAnnotations(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, []string{"out"}))
	g.AddNode(makeTestNode("b", []string{"in"}, nil))
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"a", "out"}, Target: PortID{"b", "in"}})

	g.AddAnnotation(&Annotation{ID: "onEdge", AnchorKind: AnchorEdge, AnchorID: "e1"})
	g.AddAnnotation(&Annotation{ID: "onNode", AnchorKind: AnchorNode, AnchorID: "a"})

	if err := g.RemoveEdge("e1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := g.Annotations["onEdge"]; ok {
		t.Fatal("edge-anchored annotation survived RemoveEdge")
	}
	if _, ok := g.Annotations["onNode"]; !ok {
		t.Fatal("unrelated annotation removed")
	}
}

func TestAnnotationsFor(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, nil))
	g.AddAnnotation(&Annotation{ID: "an1", AnchorKind: AnchorNode, AnchorID: "a"})
	g.AddAnnotation(&Annotation{ID: "an2", AnchorKind: AnchorNode, AnchorID: "a"})
	g.AddAnnotation(&Annotation{ID: "an3", AnchorKind: AnchorNone})

	got := g.AnnotationsFor(AnchorNode, "a")
	if len(got) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(got))
	}
	if len(g.AnnotationsFor(AnchorGroup, "a")) != 0 {
		t.Fatal("expected no group annotations")
	}
}
