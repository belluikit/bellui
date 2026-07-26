package dag

import (
	"sort"
)

// LayoutOptions controls LayeredLayout spacing. All values are in graph units.
type LayoutOptions struct {
	NodeWidth  float32    // assumed node width when a node has no stored size
	NodeHeight float32    // assumed node height when a node has no stored size
	HGap       float32    // horizontal gap between layers
	VGap       float32    // vertical gap between nodes within a layer
	Origin     [2]float32 // top-left anchor; layers flow right, centered vertically on Origin[1]

	// RankLinkEdges includes link edges in ranking and ordering, for graphs
	// whose structure lives in link edges rather than dataflow (which would
	// otherwise collapse to a single column). Link edges are exempt from
	// acyclicity, so cycles they form are broken deterministically (forcing
	// the smallest-ID stuck node into the next layer) instead of returning
	// ErrCycleDetected.
	RankLinkEdges bool
}

// DefaultLayoutOptions returns spacing that matches the default node card size.
func DefaultLayoutOptions() LayoutOptions {
	return LayoutOptions{
		NodeWidth:  160,
		NodeHeight: 80,
		HGap:       80,
		VGap:       40,
	}
}

// LayeredLayout computes a position for every node using a layered
// (Sugiyama-style) layout: nodes are ranked into layers by longest path from
// the sources, ordered within each layer by the barycenter of their
// neighbors to reduce crossings, and placed flowing left to right — matching
// the output→input port convention. The graph is not mutated; the returned
// map holds the computed position for each node ID.
// Group dependency edges are intentionally ignored: layout ranks by data
// flow only, and group boxes follow their members. Link edges are likewise
// ignored unless opts.RankLinkEdges is set.
// Returns ErrCycleDetected if the graph is not a DAG.
func LayeredLayout(g *Graph, opts LayoutOptions) (map[string][2]float32, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.Nodes) == 0 {
		return map[string][2]float32{}, nil
	}

	// Deterministic node ordering for stable results.
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	preds := make(map[string][]string)
	succs := make(map[string][]string)
	for _, e := range g.Edges {
		preds[e.Target.NodeID] = append(preds[e.Target.NodeID], e.Source.NodeID)
		succs[e.Source.NodeID] = append(succs[e.Source.NodeID], e.Target.NodeID)
	}
	if opts.RankLinkEdges {
		for _, le := range g.LinkEdges {
			preds[le.Target] = append(preds[le.Target], le.Source)
			succs[le.Source] = append(succs[le.Source], le.Target)
		}
	}
	for _, adj := range []map[string][]string{preds, succs} {
		for _, list := range adj {
			sort.Strings(list)
		}
	}

	// Rank by longest path from sources (Kahn's algorithm).
	inDegree := make(map[string]int, len(ids))
	for _, id := range ids {
		inDegree[id] = len(preds[id])
	}
	var queue []string
	for _, id := range ids {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}
	rank := make(map[string]int, len(ids))
	placed := make(map[string]bool, len(ids))
	for processed := 0; processed < len(ids); {
		if len(queue) == 0 {
			// Stalled with nodes left: a cycle. With link edges ranked that
			// is legal (they are exempt from acyclicity) — break it at the
			// smallest unplaced ID, keeping the result deterministic.
			if !opts.RankLinkEdges {
				return nil, ErrCycleDetected
			}
			for _, id := range ids {
				if !placed[id] {
					queue = append(queue, id)
					break
				}
			}
		}
		id := queue[0]
		queue = queue[1:]
		if placed[id] {
			continue
		}
		placed[id] = true
		processed++
		for _, next := range succs[id] {
			if placed[next] {
				continue // back edge around a broken cycle
			}
			if r := rank[id] + 1; r > rank[next] {
				rank[next] = r
			}
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Group into layers, initially ordered by ID.
	maxRank := 0
	for _, r := range rank {
		if r > maxRank {
			maxRank = r
		}
	}
	layers := make([][]string, maxRank+1)
	for _, id := range ids {
		layers[rank[id]] = append(layers[rank[id]], id)
	}

	// Barycenter ordering: sort each layer by the mean order index of its
	// neighbors in the previous (downward pass) or next (upward pass) layers.
	order := make(map[string]int, len(ids))
	recordOrder := func() {
		for _, layer := range layers {
			for i, id := range layer {
				order[id] = i
			}
		}
	}
	recordOrder()
	sortLayer := func(layer []string, neighbors map[string][]string) {
		sort.SliceStable(layer, func(a, b int) bool {
			return barycenter(layer[a], neighbors, order) < barycenter(layer[b], neighbors, order)
		})
	}
	for sweep := 0; sweep < 2; sweep++ {
		for l := 1; l <= maxRank; l++ {
			sortLayer(layers[l], preds)
			recordOrder()
		}
		for l := maxRank - 1; l >= 0; l-- {
			sortLayer(layers[l], succs)
			recordOrder()
		}
	}

	// Placement: layers flow left to right, each advancing by its widest
	// node; nodes stack top to bottom, centered vertically on Origin[1].
	nodeW := func(id string) float32 {
		if s := g.Nodes[id].Size[0]; s > opts.NodeWidth {
			return s
		}
		return opts.NodeWidth
	}
	nodeH := func(id string) float32 {
		if s := g.Nodes[id].Size[1]; s > opts.NodeHeight {
			return s
		}
		return opts.NodeHeight
	}

	positions := make(map[string][2]float32, len(ids))
	x := opts.Origin[0]
	for _, layer := range layers {
		layerW := float32(0)
		totalH := opts.VGap * float32(len(layer)-1)
		for _, id := range layer {
			totalH += nodeH(id)
			if w := nodeW(id); w > layerW {
				layerW = w
			}
		}
		y := opts.Origin[1] - totalH/2
		for _, id := range layer {
			positions[id] = [2]float32{x, y}
			y += nodeH(id) + opts.VGap
		}
		x += layerW + opts.HGap
	}
	return positions, nil
}

// barycenter returns the mean order index of a node's neighbors, or its own
// current index when it has none (keeping isolated nodes in place).
func barycenter(id string, neighbors map[string][]string, order map[string]int) float32 {
	ns := neighbors[id]
	if len(ns) == 0 {
		return float32(order[id])
	}
	sum := 0
	for _, n := range ns {
		sum += order[n]
	}
	return float32(sum) / float32(len(ns))
}

// NeedsLayout reports whether a graph arrived without usable stored
// positions: two or more nodes all sharing the same position (typically the
// zero value from proto or programmatic construction).
func NeedsLayout(g *Graph) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if len(g.Nodes) < 2 {
		return false
	}
	var first [2]float32
	started := false
	for _, n := range g.Nodes {
		if !started {
			first = n.Position
			started = true
			continue
		}
		if n.Position != first {
			return false
		}
	}
	return true
}
