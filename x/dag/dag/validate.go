package dag

// Cycle detection runs over an augmented precedence graph so that node
// edges, group dependency edges, and group membership are validated
// together. The vertex set is every node plus two synthetic vertices per
// group, G_in and G_out, connected as:
//
//	node edge X→Y      X → Y
//	membership X ∈ G   G_in → X,  X → G_out
//	group edge A→B     A_out → B_in
//	always, per group  G_in → G_out
//
// The unconditional G_in→G_out edge makes a group participate in ordering
// as a unit even when empty, so purely group-level cycles (A→B plus B→A)
// are rejected immediately.

type vertexKind uint8

const (
	vNode vertexKind = iota
	vGroupIn
	vGroupOut
)

// vertex identifies a vertex of the augmented precedence graph.
type vertex struct {
	kind vertexKind
	id   string // node ID or group ID
}

// effSuccessors returns the successors of v in the augmented precedence
// graph. Must be called with mu held (at least RLock).
func (g *Graph) effSuccessors(v vertex) []vertex {
	var result []vertex
	switch v.kind {
	case vNode:
		for _, next := range g.successors(v.id) {
			result = append(result, vertex{vNode, next})
		}
		if gid := g.groupOf(v.id); gid != "" {
			result = append(result, vertex{vGroupOut, gid})
		}
	case vGroupIn:
		if grp, ok := g.Groups[v.id]; ok {
			for _, m := range grp.Members {
				result = append(result, vertex{vNode, m})
			}
		}
		result = append(result, vertex{vGroupOut, v.id})
	case vGroupOut:
		for _, ge := range g.GroupEdges {
			if ge.Source == v.id {
				result = append(result, vertex{vGroupIn, ge.Target})
			}
		}
	}
	return result
}

// effReachable reports whether target is reachable from current in the
// augmented precedence graph. Must be called with mu held.
func (g *Graph) effReachable(current, target vertex, visited map[vertex]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true
	for _, next := range g.effSuccessors(current) {
		if g.effReachable(next, target, visited) {
			return true
		}
	}
	return false
}

// TopologicalOrder returns nodes in valid execution order using Kahn's
// algorithm over the augmented precedence graph, so group dependency edges
// constrain the order alongside node edges.
// Returns an error if the combined graph contains a cycle.
func (g *Graph) TopologicalOrder() ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[vertex]int)
	for id := range g.Nodes {
		inDegree[vertex{vNode, id}] = 0
	}
	for id := range g.Groups {
		inDegree[vertex{vGroupIn, id}] = 0
		inDegree[vertex{vGroupOut, id}] = 0
	}
	for v := range inDegree {
		for _, next := range g.effSuccessors(v) {
			inDegree[next]++
		}
	}

	var queue []vertex
	for v, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, v)
		}
	}

	processed := 0
	var order []*Node
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		processed++
		if v.kind == vNode {
			order = append(order, g.Nodes[v.id])
		}

		for _, next := range g.effSuccessors(v) {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if processed != len(g.Nodes)+2*len(g.Groups) {
		return nil, ErrCycleDetected
	}
	return order, nil
}
