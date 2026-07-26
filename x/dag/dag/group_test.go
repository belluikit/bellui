package dag

import (
	"errors"
	"testing"
)

// twoGroupGraph builds a graph with nodes x, y and empty groups A, B.
func twoGroupGraph(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("x", []string{"in"}, []string{"out"}))
	g.AddNode(makeTestNode("y", []string{"in"}, []string{"out"}))
	if err := g.AddGroup(&Group{ID: "A", Label: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroup(&Group{ID: "B", Label: "B"}); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestAddRemoveGroup(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, nil))
	grp := &Group{ID: "g1", Label: "Group 1", Members: []string{"a"}}
	if err := g.AddGroup(grp); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroup(grp); !errors.Is(err, ErrGroupExists) {
		t.Fatalf("expected ErrGroupExists, got %v", err)
	}
	if got := g.GroupOf("a"); got != "g1" {
		t.Fatalf("expected a in g1, got %q", got)
	}
	if _, _, err := g.RemoveGroup("g1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := g.RemoveGroup("g1"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}
	if got := g.GroupOf("a"); got != "" {
		t.Fatalf("expected a ungrouped after removal, got %q", got)
	}
}

func TestAddGroupMemberValidation(t *testing.T) {
	g := NewGraph("test", "Test")
	g.AddNode(makeTestNode("a", nil, nil))

	if err := g.AddGroup(&Group{ID: "g1", Members: []string{"nope"}}); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
	if err := g.AddGroup(&Group{ID: "g1", Members: []string{"a", "a"}}); !errors.Is(err, ErrNodeGrouped) {
		t.Fatalf("expected ErrNodeGrouped for duplicate member, got %v", err)
	}
	if err := g.AddGroup(&Group{ID: "g1", Members: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroup(&Group{ID: "g2", Members: []string{"a"}}); !errors.Is(err, ErrNodeGrouped) {
		t.Fatalf("expected ErrNodeGrouped for cross-group member, got %v", err)
	}
}

func TestAssignNodeToGroup(t *testing.T) {
	g := twoGroupGraph(t)

	if err := g.AssignNodeToGroup("nope", "A"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
	if err := g.AssignNodeToGroup("x", "nope"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}

	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if got := g.GroupOf("x"); got != "A" {
		t.Fatalf("expected x in A, got %q", got)
	}
	// No-op re-assign.
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	// Move to B.
	if err := g.AssignNodeToGroup("x", "B"); err != nil {
		t.Fatal(err)
	}
	if got := g.GroupOf("x"); got != "B" {
		t.Fatalf("expected x in B, got %q", got)
	}
	if len(g.Groups["A"].Members) != 0 {
		t.Fatalf("expected A empty after move, got %v", g.Groups["A"].Members)
	}
	// Ungroup.
	if err := g.AssignNodeToGroup("x", ""); err != nil {
		t.Fatal(err)
	}
	if got := g.GroupOf("x"); got != "" {
		t.Fatalf("expected x ungrouped, got %q", got)
	}
}

func TestAddGroupEdgeValidation(t *testing.T) {
	g := twoGroupGraph(t)

	ge := &GroupEdge{ID: "ge1", Source: "A", Target: "B"}
	if err := g.AddGroupEdge(ge); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(ge); !errors.Is(err, ErrGroupEdgeExists) {
		t.Fatalf("expected ErrGroupEdgeExists for dup ID, got %v", err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge2", Source: "A", Target: "B"}); !errors.Is(err, ErrGroupEdgeExists) {
		t.Fatalf("expected ErrGroupEdgeExists for dup pair, got %v", err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge3", Source: "nope", Target: "B"}); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound for source, got %v", err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge4", Source: "A", Target: "nope"}); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound for target, got %v", err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge5", Source: "A", Target: "A"}); !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected for self-loop, got %v", err)
	}
	if err := g.RemoveGroupEdge("ge1"); err != nil {
		t.Fatal(err)
	}
	if err := g.RemoveGroupEdge("ge1"); !errors.Is(err, ErrGroupEdgeNotFound) {
		t.Fatalf("expected ErrGroupEdgeNotFound, got %v", err)
	}
}

func TestGroupEdgeCycleEmptyGroups(t *testing.T) {
	g := twoGroupGraph(t)

	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}
	// Even with no members, B -> A must be rejected: groups order as units.
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge2", Source: "B", Target: "A"}); !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestCombinedCycleGroupEdgeAfterNodeEdge(t *testing.T) {
	g := twoGroupGraph(t)
	// x -> y, x in A, y in B.
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"x", "out"}, Target: PortID{"y", "in"}})
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AssignNodeToGroup("y", "B"); err != nil {
		t.Fatal(err)
	}

	// B -> A closes the loop x -> y -> B_out -> A_in -> x.
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "B", Target: "A"}); !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
	// A -> B is consistent with the node edge.
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge2", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}
}

func TestCombinedCycleNodeEdgeAfterGroupEdge(t *testing.T) {
	g := twoGroupGraph(t)
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AssignNodeToGroup("y", "B"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}

	// y -> x contradicts A -> B.
	e := &Edge{ID: "e1", Source: PortID{"y", "out"}, Target: PortID{"x", "in"}}
	if err := g.AddEdge(e); !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestAssignNodeToGroupCycleDetection(t *testing.T) {
	g := twoGroupGraph(t)
	// y -> x, x in A, group edge A -> B.
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"y", "out"}, Target: PortID{"x", "in"}})
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}

	// Putting y into B creates B_in -> y -> x -> A_out -> B_in.
	if err := g.AssignNodeToGroup("y", "B"); !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
	// Rollback: y must remain ungrouped.
	if got := g.GroupOf("y"); got != "" {
		t.Fatalf("expected y ungrouped after rejected assign, got %q", got)
	}
}

func TestAssignNodeToGroupCycleRollbackToOldGroup(t *testing.T) {
	g := twoGroupGraph(t)
	g.AddGroup(&Group{ID: "C", Label: "C"})
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"y", "out"}, Target: PortID{"x", "in"}})
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AssignNodeToGroup("y", "C"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}

	// Moving y from C to B is rejected; y must stay in C.
	if err := g.AssignNodeToGroup("y", "B"); !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
	if got := g.GroupOf("y"); got != "C" {
		t.Fatalf("expected y restored to C, got %q", got)
	}
}

func TestCanAssignToGroup(t *testing.T) {
	g := twoGroupGraph(t)
	// y -> x, x in A, group edge A -> B: y cannot join B (cycle).
	g.AddEdge(&Edge{ID: "e1", Source: PortID{"y", "out"}, Target: PortID{"x", "in"}})
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}

	if g.CanAssignToGroup("y", "B") {
		t.Fatal("expected y -> B to be rejected (cycle)")
	}
	if !g.CanAssignToGroup("y", "A") {
		t.Fatal("expected y -> A to be allowed")
	}
	// Moving x from A to B simulates the membership strip first: x's own
	// A-membership must not count as a path.
	if !g.CanAssignToGroup("x", "B") {
		t.Fatal("expected x to be movable from A to B")
	}
	// The probe must not mutate membership.
	if got := g.GroupOf("x"); got != "A" {
		t.Fatalf("probe changed membership: %q", got)
	}
	if g.CanAssignToGroup("nope", "A") || g.CanAssignToGroup("x", "nope") {
		t.Fatal("unknown node/group should be unassignable")
	}
	if !g.CanAssignToGroup("x", "") || !g.CanAssignToGroup("x", "A") {
		t.Fatal("ungrouping and no-op re-assign should always be allowed")
	}
}

func TestRemoveGroupCascades(t *testing.T) {
	g := twoGroupGraph(t)
	g.AddGroup(&Group{ID: "C", Label: "C"})
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"})
	g.AddGroupEdge(&GroupEdge{ID: "ge2", Source: "C", Target: "A"})
	g.AddGroupEdge(&GroupEdge{ID: "ge3", Source: "C", Target: "B"})
	if err := g.AddAnnotation(&Annotation{ID: "an1", Text: "note", AnchorKind: AnchorGroup, AnchorID: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddAnnotation(&Annotation{ID: "an2", Text: "keep", AnchorKind: AnchorGroup, AnchorID: "B"}); err != nil {
		t.Fatal(err)
	}

	edges, anns, err := g.RemoveGroup("A")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 cascaded group edges, got %d", len(edges))
	}
	if len(anns) != 1 || anns[0].ID != "an1" {
		t.Fatalf("expected an1 cascaded, got %v", anns)
	}
	if len(g.GroupEdges) != 1 {
		t.Fatalf("expected 1 remaining group edge, got %d", len(g.GroupEdges))
	}
	if _, ok := g.Annotations["an2"]; !ok {
		t.Fatal("unrelated annotation removed")
	}
	if got := g.GroupOf("x"); got != "" {
		t.Fatalf("expected x ungrouped, got %q", got)
	}
}

func TestRemoveNodeStripsMembership(t *testing.T) {
	g := twoGroupGraph(t)
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.RemoveNode("x"); err != nil {
		t.Fatal(err)
	}
	if len(g.Groups["A"].Members) != 0 {
		t.Fatalf("expected A empty, got %v", g.Groups["A"].Members)
	}
}

func TestTopologicalOrderHonorsGroupEdges(t *testing.T) {
	g := twoGroupGraph(t)
	// No node edges at all; ordering constraint comes from B -> A only.
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AssignNodeToGroup("y", "B"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "B", Target: "A"}); err != nil {
		t.Fatal(err)
	}

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(order))
	}
	if order[0].ID != "y" || order[1].ID != "x" {
		t.Fatalf("expected y before x, got %s, %s", order[0].ID, order[1].ID)
	}
}

func TestTopologicalOrderGroupsNoEdges(t *testing.T) {
	// Regression for the augmented vertex-count cycle check: groups present
	// but no group edges must still order successfully.
	g := twoGroupGraph(t)
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(order))
	}
}

func TestWouldCycleGroupAware(t *testing.T) {
	g := twoGroupGraph(t)
	if err := g.AssignNodeToGroup("x", "A"); err != nil {
		t.Fatal(err)
	}
	if err := g.AssignNodeToGroup("y", "B"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddGroupEdge(&GroupEdge{ID: "ge1", Source: "A", Target: "B"}); err != nil {
		t.Fatal(err)
	}

	// y -> x would cycle through the group edge; the exported probe used by
	// the editor's drop-target highlighting must see it.
	if !g.WouldCycle("y", "x") {
		t.Fatal("expected WouldCycle(y, x) == true via group edge")
	}
	if g.WouldCycle("x", "y") {
		t.Fatal("expected WouldCycle(x, y) == false")
	}
}
