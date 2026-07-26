package dag

import "fmt"

// LinkEdge is a port-less, user-authored relation between two nodes. Unlike
// Edge it carries no dataflow semantics: it is exempt from the acyclicity and
// target-occupancy invariants (parallel edges and opposite directions between
// the same pair are legal) and is ignored by TopologicalOrder. It is persisted
// with the graph but editor-level only — the backend ignores it.
type LinkEdge struct {
	ID     string
	Source string // node ID
	Target string // node ID
	Label  string
}

// AddLinkEdge validates and adds a link edge. Only endpoint existence is
// checked; self-loops are rejected (no rendering story for them).
func (g *Graph) AddLinkEdge(le *LinkEdge) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.LinkEdges[le.ID]; exists {
		return fmt.Errorf("%w: %s", ErrLinkEdgeExists, le.ID)
	}
	if _, ok := g.Nodes[le.Source]; !ok {
		return fmt.Errorf("source: %w: %s", ErrNodeNotFound, le.Source)
	}
	if _, ok := g.Nodes[le.Target]; !ok {
		return fmt.Errorf("target: %w: %s", ErrNodeNotFound, le.Target)
	}
	if le.Source == le.Target {
		return fmt.Errorf("%w: %s", ErrLinkEdgeSelfLoop, le.Source)
	}
	g.LinkEdges[le.ID] = le
	return nil
}

// RemoveLinkEdge removes a link edge by ID, cascading removal of annotations
// anchored to it. Callers needing undo state should snapshot with
// AnnotationsFor first.
func (g *Graph) RemoveLinkEdge(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.LinkEdges[id]; !exists {
		return fmt.Errorf("%w: %s", ErrLinkEdgeNotFound, id)
	}
	for _, a := range g.annotationsFor(AnchorLinkEdge, id) {
		delete(g.Annotations, a.ID)
	}
	delete(g.LinkEdges, id)
	return nil
}

// LinkEdgesForNode returns all link edges connected to a node.
func (g *Graph) LinkEdgesForNode(nodeID string) []*LinkEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.linkEdgesForNode(nodeID)
}

// linkEdgesForNode is the internal unlocked version of LinkEdgesForNode.
func (g *Graph) linkEdgesForNode(nodeID string) []*LinkEdge {
	var result []*LinkEdge
	for _, le := range g.LinkEdges {
		if le.Source == nodeID || le.Target == nodeID {
			result = append(result, le)
		}
	}
	return result
}
