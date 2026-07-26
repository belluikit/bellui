package dag

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// PortDirection indicates whether a port is an input or output.
type PortDirection int

const (
	PortInput  PortDirection = 0
	PortOutput PortDirection = 1
)

// PortID uniquely identifies a port within a graph.
type PortID struct {
	NodeID string
	Name   string
}

// Port represents a connection point on a node.
type Port struct {
	Name      string
	Direction PortDirection
	TypeTag   string
}

// Node represents a vertex in the DAG.
type Node struct {
	ID       string
	Label    string
	TypeTag  string
	Inputs   []Port
	Outputs  []Port
	Position [2]float32
	Size     [2]float32 // user-set card size; zero means auto-size
	Metadata json.RawMessage
}

// Edge represents a directed connection from an output port to an input port.
type Edge struct {
	ID     string
	Source PortID
	Target PortID
}

// Graph is the top-level DAG container.
// All public mutation and query methods are safe for concurrent use.
type Graph struct {
	ID          string
	Name        string
	Nodes       map[string]*Node
	Edges       map[string]*Edge
	Groups      map[string]*Group
	GroupEdges  map[string]*GroupEdge
	LinkEdges   map[string]*LinkEdge
	Annotations map[string]*Annotation
	Metadata    json.RawMessage

	// Registry is an optional component registry used to validate
	// stream type compatibility when adding edges.
	Registry *Registry

	mu sync.RWMutex
}

var (
	ErrNodeExists     = errors.New("node already exists")
	ErrNodeNotFound   = errors.New("node not found")
	ErrEdgeExists     = errors.New("edge already exists")
	ErrEdgeNotFound   = errors.New("edge not found")
	ErrPortNotFound   = errors.New("port not found")
	ErrPortDirection  = errors.New("source must be output, target must be input")
	ErrTargetOccupied = errors.New("target port already has a connection")
	ErrCycleDetected  = errors.New("adding this edge would create a cycle")
	ErrStreamIncompat = errors.New("incompatible stream types")

	ErrGroupExists        = errors.New("group already exists")
	ErrGroupNotFound      = errors.New("group not found")
	ErrGroupEdgeExists    = errors.New("group edge already exists")
	ErrGroupEdgeNotFound  = errors.New("group edge not found")
	ErrLinkEdgeExists     = errors.New("link edge already exists")
	ErrLinkEdgeNotFound   = errors.New("link edge not found")
	ErrLinkEdgeSelfLoop   = errors.New("link edge cannot connect a node to itself")
	ErrNodeGrouped        = errors.New("node already belongs to a group")
	ErrAnnotationExists   = errors.New("annotation already exists")
	ErrAnnotationNotFound = errors.New("annotation not found")
	ErrAnchorNotFound     = errors.New("annotation anchor not found")
	ErrInvalidAnchor      = errors.New("invalid annotation anchor")
)

// NewGraph creates an empty graph.
func NewGraph(id, name string) *Graph {
	return &Graph{
		ID:          id,
		Name:        name,
		Nodes:       make(map[string]*Node),
		Edges:       make(map[string]*Edge),
		Groups:      make(map[string]*Group),
		GroupEdges:  make(map[string]*GroupEdge),
		LinkEdges:   make(map[string]*LinkEdge),
		Annotations: make(map[string]*Annotation),
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(n *Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Nodes[n.ID]; exists {
		return fmt.Errorf("%w: %s", ErrNodeExists, n.ID)
	}
	g.Nodes[n.ID] = n
	return nil
}

// RemoveNode removes a node and all connected edges, returning removed edges.
// It also cascades: connected link edges are deleted, annotations anchored to
// the node or to any removed edge (link or not) are deleted, and the node is
// stripped from its group's member list. Callers needing undo state should
// snapshot with LinkEdgesForNode, AnnotationsFor, and GroupOf first.
func (g *Graph) RemoveNode(id string) ([]*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Nodes[id]; !exists {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id)
	}
	removed := g.edgesForNode(id)
	for _, e := range removed {
		delete(g.Edges, e.ID)
		for _, a := range g.annotationsFor(AnchorEdge, e.ID) {
			delete(g.Annotations, a.ID)
		}
	}
	for _, le := range g.linkEdgesForNode(id) {
		delete(g.LinkEdges, le.ID)
		for _, a := range g.annotationsFor(AnchorLinkEdge, le.ID) {
			delete(g.Annotations, a.ID)
		}
	}
	for _, a := range g.annotationsFor(AnchorNode, id) {
		delete(g.Annotations, a.ID)
	}
	if gid := g.groupOf(id); gid != "" {
		g.stripMember(gid, id)
	}
	delete(g.Nodes, id)
	return removed, nil
}

// AddEdge validates and adds an edge to the graph.
func (g *Graph) AddEdge(e *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.Edges[e.ID]; exists {
		return fmt.Errorf("%w: %s", ErrEdgeExists, e.ID)
	}

	srcPort, err := g.findPort(e.Source)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	tgtPort, err := g.findPort(e.Target)
	if err != nil {
		return fmt.Errorf("target: %w", err)
	}

	if srcPort.Direction != PortOutput || tgtPort.Direction != PortInput {
		return ErrPortDirection
	}

	for _, existing := range g.Edges {
		if existing.Target == e.Target {
			return fmt.Errorf("%w: %s.%s", ErrTargetOccupied, e.Target.NodeID, e.Target.Name)
		}
	}

	if g.Registry != nil {
		srcNode := g.Nodes[e.Source.NodeID]
		tgtNode := g.Nodes[e.Target.NodeID]
		srcStream, srcOK := g.Registry.PortStreamType(srcNode, e.Source.Name, PortOutput)
		tgtStream, tgtOK := g.Registry.PortStreamType(tgtNode, e.Target.Name, PortInput)
		if srcOK && tgtOK && !g.Registry.StreamsCompatible(srcStream, tgtStream) {
			return fmt.Errorf("%w: %s -> %s", ErrStreamIncompat, srcStream.Name, tgtStream.Name)
		}
	}

	if g.wouldCycle(e.Source.NodeID, e.Target.NodeID) {
		return ErrCycleDetected
	}

	g.Edges[e.ID] = e
	return nil
}

// RemoveEdge removes an edge by ID, cascading removal of annotations
// anchored to it. Callers needing undo state should snapshot with
// AnnotationsFor first.
func (g *Graph) RemoveEdge(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Edges[id]; !exists {
		return fmt.Errorf("%w: %s", ErrEdgeNotFound, id)
	}
	for _, a := range g.annotationsFor(AnchorEdge, id) {
		delete(g.Annotations, a.ID)
	}
	delete(g.Edges, id)
	return nil
}

// EdgesForNode returns all edges connected to a node.
func (g *Graph) EdgesForNode(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edgesForNode(nodeID)
}

func (g *Graph) edgesForNode(nodeID string) []*Edge {
	var result []*Edge
	for _, e := range g.Edges {
		if e.Source.NodeID == nodeID || e.Target.NodeID == nodeID {
			result = append(result, e)
		}
	}
	return result
}

// EdgesForPort returns edges connected to a specific port.
func (g *Graph) EdgesForPort(pid PortID) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []*Edge
	for _, e := range g.Edges {
		if e.Source == pid || e.Target == pid {
			result = append(result, e)
		}
	}
	return result
}

// findPort looks up a port in the graph. Must be called with mu held.
func (g *Graph) findPort(pid PortID) (*Port, error) {
	node, exists := g.Nodes[pid.NodeID]
	if !exists {
		return nil, fmt.Errorf("%w: node %s", ErrNodeNotFound, pid.NodeID)
	}
	for i := range node.Inputs {
		if node.Inputs[i].Name == pid.Name {
			return &node.Inputs[i], nil
		}
	}
	for i := range node.Outputs {
		if node.Outputs[i].Name == pid.Name {
			return &node.Outputs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s on node %s", ErrPortNotFound, pid.Name, pid.NodeID)
}

// WouldCycle returns true if adding an edge from srcNodeID to dstNodeID
// would create a cycle.
func (g *Graph) WouldCycle(srcNodeID, dstNodeID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.wouldCycle(srcNodeID, dstNodeID)
}

// wouldCycle is the internal unlocked version. It consults the augmented
// precedence graph, so paths through group dependency edges count.
func (g *Graph) wouldCycle(srcNodeID, dstNodeID string) bool {
	if srcNodeID == dstNodeID {
		return true
	}
	visited := make(map[vertex]bool)
	return g.effReachable(vertex{vNode, dstNodeID}, vertex{vNode, srcNodeID}, visited)
}

// Successors returns the node IDs that nodeID has edges pointing to.
func (g *Graph) Successors(nodeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.successors(nodeID)
}

func (g *Graph) successors(nodeID string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, e := range g.Edges {
		if e.Source.NodeID == nodeID && !seen[e.Target.NodeID] {
			seen[e.Target.NodeID] = true
			result = append(result, e.Target.NodeID)
		}
	}
	return result
}
