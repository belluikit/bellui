package dag

import "fmt"

// Group is a flat (non-nesting) container of nodes. A node belongs to at
// most one group; membership is stored on the group. Position, Size, and
// Collapsed are mutated in place by the editor (like Node.Position) — only
// structural changes (membership, group edges) go through Graph methods.
type Group struct {
	ID        string
	Label     string
	Members   []string // node IDs; each node is in at most one group
	Position  [2]float32
	Size      [2]float32
	Collapsed bool
	Color     string // optional "#rrggbb" tint; empty means theme default
}

// GroupEdge is a user-authored dependency between groups: everything in
// Source precedes everything in Target. It is persisted with the graph but
// currently editor-level only — the backend ignores it for execution.
type GroupEdge struct {
	ID     string
	Source string // group ID
	Target string // group ID
}

// AddGroup adds a group to the graph. Every member must exist and must not
// already belong to another group. A new group cannot introduce a cycle
// (group edges can only reference existing groups), so no cycle check is
// needed here.
func (g *Graph) AddGroup(grp *Group) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Groups[grp.ID]; exists {
		return fmt.Errorf("%w: %s", ErrGroupExists, grp.ID)
	}
	seen := make(map[string]bool, len(grp.Members))
	for _, m := range grp.Members {
		if _, ok := g.Nodes[m]; !ok {
			return fmt.Errorf("member: %w: %s", ErrNodeNotFound, m)
		}
		if seen[m] || g.groupOf(m) != "" {
			return fmt.Errorf("%w: %s", ErrNodeGrouped, m)
		}
		seen[m] = true
	}
	g.Groups[grp.ID] = grp
	return nil
}

// RemoveGroup removes a group, cascading removal of group edges connected to
// it and annotations anchored to it. Both are returned so callers can undo.
// Members simply become ungrouped.
func (g *Graph) RemoveGroup(id string) ([]*GroupEdge, []*Annotation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Groups[id]; !exists {
		return nil, nil, fmt.Errorf("%w: %s", ErrGroupNotFound, id)
	}
	var edges []*GroupEdge
	for _, ge := range g.GroupEdges {
		if ge.Source == id || ge.Target == id {
			edges = append(edges, ge)
		}
	}
	for _, ge := range edges {
		delete(g.GroupEdges, ge.ID)
	}
	anns := g.annotationsFor(AnchorGroup, id)
	for _, a := range anns {
		delete(g.Annotations, a.ID)
	}
	delete(g.Groups, id)
	return edges, anns, nil
}

// AssignNodeToGroup moves a node into the given group, or out of any group
// when groupID is empty. Rejects assignments that would create a dependency
// cycle through group edges.
func (g *Graph) AssignNodeToGroup(nodeID, groupID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.Nodes[nodeID]; !ok {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}
	if groupID != "" {
		if _, ok := g.Groups[groupID]; !ok {
			return fmt.Errorf("%w: %s", ErrGroupNotFound, groupID)
		}
	}
	oldGroup := g.groupOf(nodeID)
	if oldGroup == groupID {
		return nil
	}
	if oldGroup != "" {
		g.stripMember(oldGroup, nodeID)
	}
	if groupID == "" {
		return nil
	}
	// Membership adds augmented edges G_in→node and node→G_out; check each
	// independently (a cycle needing both would imply a pre-existing
	// G_out→*→G_in path, impossible while the graph invariant holds).
	nv := vertex{vNode, nodeID}
	if g.effReachable(nv, vertex{vGroupIn, groupID}, make(map[vertex]bool)) ||
		g.effReachable(vertex{vGroupOut, groupID}, nv, make(map[vertex]bool)) {
		if oldGroup != "" {
			g.Groups[oldGroup].Members = append(g.Groups[oldGroup].Members, nodeID)
		}
		return fmt.Errorf("%w: assigning node %s to group %s", ErrCycleDetected, nodeID, groupID)
	}
	g.Groups[groupID].Members = append(g.Groups[groupID].Members, nodeID)
	return nil
}

// stripMember removes nodeID from the given group's member list.
// Must be called with mu held.
func (g *Graph) stripMember(groupID, nodeID string) {
	grp := g.Groups[groupID]
	for i, m := range grp.Members {
		if m == nodeID {
			grp.Members = append(grp.Members[:i], grp.Members[i+1:]...)
			return
		}
	}
}

// AddGroupEdge validates and adds a dependency edge between two groups.
func (g *Graph) AddGroupEdge(ge *GroupEdge) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.GroupEdges[ge.ID]; exists {
		return fmt.Errorf("%w: %s", ErrGroupEdgeExists, ge.ID)
	}
	if _, ok := g.Groups[ge.Source]; !ok {
		return fmt.Errorf("source: %w: %s", ErrGroupNotFound, ge.Source)
	}
	if _, ok := g.Groups[ge.Target]; !ok {
		return fmt.Errorf("target: %w: %s", ErrGroupNotFound, ge.Target)
	}
	if ge.Source == ge.Target {
		return fmt.Errorf("%w: group edge %s -> %s", ErrCycleDetected, ge.Source, ge.Target)
	}
	for _, existing := range g.GroupEdges {
		if existing.Source == ge.Source && existing.Target == ge.Target {
			return fmt.Errorf("%w: %s -> %s", ErrGroupEdgeExists, ge.Source, ge.Target)
		}
	}
	if g.effReachable(vertex{vGroupIn, ge.Target}, vertex{vGroupOut, ge.Source}, make(map[vertex]bool)) {
		return fmt.Errorf("%w: group edge %s -> %s", ErrCycleDetected, ge.Source, ge.Target)
	}
	g.GroupEdges[ge.ID] = ge
	return nil
}

// RemoveGroupEdge removes a group edge by ID.
func (g *Graph) RemoveGroupEdge(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.GroupEdges[id]; !exists {
		return fmt.Errorf("%w: %s", ErrGroupEdgeNotFound, id)
	}
	delete(g.GroupEdges, id)
	return nil
}

// CanAssignToGroup reports whether AssignNodeToGroup(nodeID, groupID) would
// succeed. Used by the editor to validate membership drop targets. The check
// simulates stripping the node's current membership first (exactly like the
// real assignment), so moving a node between dependent groups is judged
// correctly.
func (g *Graph) CanAssignToGroup(nodeID, groupID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.Nodes[nodeID]; !ok {
		return false
	}
	if groupID == "" {
		return true
	}
	if _, ok := g.Groups[groupID]; !ok {
		return false
	}
	oldGroup := g.groupOf(nodeID)
	if oldGroup == groupID {
		return true
	}
	if oldGroup != "" {
		g.stripMember(oldGroup, nodeID)
		defer func() {
			g.Groups[oldGroup].Members = append(g.Groups[oldGroup].Members, nodeID)
		}()
	}
	nv := vertex{vNode, nodeID}
	return !g.effReachable(nv, vertex{vGroupIn, groupID}, make(map[vertex]bool)) &&
		!g.effReachable(vertex{vGroupOut, groupID}, nv, make(map[vertex]bool))
}

// WouldGroupCycle returns true if adding a group edge from src to dst would
// create a cycle through group edges, node edges, and membership. Used by
// the editor to validate drop targets without mutating the graph.
func (g *Graph) WouldGroupCycle(src, dst string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if src == dst {
		return true
	}
	return g.effReachable(vertex{vGroupIn, dst}, vertex{vGroupOut, src}, make(map[vertex]bool))
}

// GroupOf returns the ID of the group containing nodeID, or "" if the node
// is not in any group.
func (g *Graph) GroupOf(nodeID string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.groupOf(nodeID)
}

// groupOf is the internal unlocked version of GroupOf.
func (g *Graph) groupOf(nodeID string) string {
	for id, grp := range g.Groups {
		for _, m := range grp.Members {
			if m == nodeID {
				return id
			}
		}
	}
	return ""
}

// GroupEdgesForGroup returns all group edges connected to a group.
func (g *Graph) GroupEdgesForGroup(groupID string) []*GroupEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []*GroupEdge
	for _, ge := range g.GroupEdges {
		if ge.Source == groupID || ge.Target == groupID {
			result = append(result, ge)
		}
	}
	return result
}
