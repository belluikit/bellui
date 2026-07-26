package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/x/dag/dag"
)

// twoNodeCanvas returns a test canvas with nodes a and b.
func twoNodeCanvas(t *testing.T) *DAGCanvas {
	t.Helper()
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	addTestNode(t, c, "b")
	return c
}

func TestAddRemoveLinkEdgeCommandRoundtrip(t *testing.T) {
	c := twoNodeCanvas(t)
	le := &dag.LinkEdge{ID: "l1", Source: "a", Target: "b", Label: "goes to"}

	c.Commands.Do(&AddLinkEdgeCommand{Canvas: c, Edge: le})
	if c.Graph.LinkEdges["l1"] == nil || c.linkEdges["l1"] == nil {
		t.Fatal("link edge not added to graph and widget map")
	}
	c.Commands.Undo()
	if c.Graph.LinkEdges["l1"] != nil || c.linkEdges["l1"] != nil {
		t.Fatal("undo left link edge behind")
	}
	c.Commands.Redo()
	if c.Graph.LinkEdges["l1"] == nil || c.linkEdges["l1"] == nil {
		t.Fatal("redo did not restore link edge")
	}

	c.Commands.Do(&RemoveLinkEdgeCommand{Canvas: c, Edge: le})
	if c.Graph.LinkEdges["l1"] != nil || c.linkEdges["l1"] != nil {
		t.Fatal("remove left link edge behind")
	}
	c.Commands.Undo()
	if c.Graph.LinkEdges["l1"] == nil || c.linkEdges["l1"] == nil {
		t.Fatal("undo did not restore link edge")
	}
}

func TestRemoveLinkEdgeCommandRestoresAnnotations(t *testing.T) {
	c := twoNodeCanvas(t)
	le := &dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}
	if err := c.AddLinkEdge(le); err != nil {
		t.Fatal(err)
	}
	an := &dag.Annotation{ID: "an1", Text: "note", AnchorKind: dag.AnchorLinkEdge, AnchorID: "l1"}
	if err := c.AddAnnotation(an); err != nil {
		t.Fatal(err)
	}

	c.Commands.Do(&RemoveLinkEdgeCommand{Canvas: c, Edge: le})
	if c.Graph.Annotations["an1"] != nil || c.annotations["an1"] != nil {
		t.Fatal("anchored annotation did not cascade")
	}
	c.Commands.Undo()
	if c.Graph.Annotations["an1"] == nil || c.annotations["an1"] == nil {
		t.Fatal("undo did not restore anchored annotation")
	}
	if c.Graph.Annotations["an1"].AnchorID != "l1" {
		t.Fatal("restored annotation lost its anchor")
	}
}

func TestSetLinkEdgeLabelCommand(t *testing.T) {
	c := twoNodeCanvas(t)
	le := &dag.LinkEdge{ID: "l1", Source: "a", Target: "b", Label: "old"}
	if err := c.AddLinkEdge(le); err != nil {
		t.Fatal(err)
	}

	c.Commands.Do(&SetLinkEdgeLabelCommand{Canvas: c, EdgeID: "l1", Old: "old", New: "new"})
	if le.Label != "new" {
		t.Fatalf("label not applied: %q", le.Label)
	}
	c.Commands.Undo()
	if le.Label != "old" {
		t.Fatalf("undo did not restore label: %q", le.Label)
	}
	c.Commands.Redo()
	if le.Label != "new" {
		t.Fatalf("redo did not re-apply label: %q", le.Label)
	}
}

func TestRemoveNodeCommandRestoresLinkEdges(t *testing.T) {
	c := twoNodeCanvas(t)
	addTestNode(t, c, "z")
	for _, le := range []*dag.LinkEdge{
		{ID: "l1", Source: "a", Target: "b", Label: "kept on undo"},
		{ID: "l2", Source: "b", Target: "a"},
		{ID: "l3", Source: "b", Target: "z"},
	} {
		if err := c.AddLinkEdge(le); err != nil {
			t.Fatal(err)
		}
	}
	an := &dag.Annotation{ID: "an1", Text: "note", AnchorKind: dag.AnchorLinkEdge, AnchorID: "l1"}
	if err := c.AddAnnotation(an); err != nil {
		t.Fatal(err)
	}

	n := c.Graph.Nodes["a"]
	c.Commands.Do(&RemoveNodeCommand{Canvas: c, Node: n})
	if c.Graph.LinkEdges["l1"] != nil || c.Graph.LinkEdges["l2"] != nil {
		t.Fatal("link edges touching removed node survived")
	}
	if c.linkEdges["l1"] != nil || c.linkEdges["l2"] != nil {
		t.Fatal("stale link edge widgets survived")
	}
	if c.Graph.LinkEdges["l3"] == nil || c.linkEdges["l3"] == nil {
		t.Fatal("unrelated link edge removed")
	}
	if c.Graph.Annotations["an1"] != nil {
		t.Fatal("annotation anchored to cascaded link edge survived")
	}

	c.Commands.Undo()
	for _, id := range []string{"l1", "l2", "l3"} {
		if c.Graph.LinkEdges[id] == nil || c.linkEdges[id] == nil {
			t.Fatalf("undo did not restore link edge %s", id)
		}
	}
	if c.Graph.LinkEdges["l1"].Label != "kept on undo" {
		t.Fatal("restored link edge lost its label")
	}
	if c.Graph.Annotations["an1"] == nil || c.annotations["an1"] == nil {
		t.Fatal("undo did not restore anchored annotation")
	}
}

func TestDeleteSelectedLinkEdge(t *testing.T) {
	c := twoNodeCanvas(t)
	le := &dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}
	if err := c.AddLinkEdge(le); err != nil {
		t.Fatal(err)
	}
	c.linkEdges["l1"].Selected = true

	c.DeleteSelected()
	if c.Graph.LinkEdges["l1"] != nil || c.linkEdges["l1"] != nil {
		t.Fatal("DeleteSelected did not remove the selected link edge")
	}
	c.Commands.Undo()
	if c.Graph.LinkEdges["l1"] == nil || c.linkEdges["l1"] == nil {
		t.Fatal("undo did not restore the deleted link edge")
	}
}
