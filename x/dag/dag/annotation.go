package dag

import "fmt"

// AnchorKind says what an annotation is attached to. Values are wire-stable
// (they mirror the proto AnchorKind enum); do not renumber.
type AnchorKind int

const (
	AnchorNone     AnchorKind = 0 // floating
	AnchorNode     AnchorKind = 1
	AnchorEdge     AnchorKind = 2 // node edge (Edge ID); group edges are not anchorable
	AnchorGroup    AnchorKind = 3
	AnchorLinkEdge AnchorKind = 4 // link edge (LinkEdge ID)
)

// Annotation is a documentation text box on the canvas, either floating or
// anchored to a node, edge, or group (rendered with a leader line). Text,
// Position, and Size are mutated in place by the editor (like Node.Position).
type Annotation struct {
	ID         string
	Text       string
	Position   [2]float32
	Size       [2]float32
	AnchorKind AnchorKind
	AnchorID   string
}

// AddAnnotation validates and adds an annotation to the graph.
func (g *Graph) AddAnnotation(a *Annotation) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Annotations[a.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAnnotationExists, a.ID)
	}
	switch a.AnchorKind {
	case AnchorNone:
		if a.AnchorID != "" {
			return fmt.Errorf("%w: floating annotation with anchor ID %s", ErrInvalidAnchor, a.AnchorID)
		}
	case AnchorNode:
		if _, ok := g.Nodes[a.AnchorID]; !ok {
			return fmt.Errorf("%w: node %s", ErrAnchorNotFound, a.AnchorID)
		}
	case AnchorEdge:
		if _, ok := g.Edges[a.AnchorID]; !ok {
			return fmt.Errorf("%w: edge %s", ErrAnchorNotFound, a.AnchorID)
		}
	case AnchorGroup:
		if _, ok := g.Groups[a.AnchorID]; !ok {
			return fmt.Errorf("%w: group %s", ErrAnchorNotFound, a.AnchorID)
		}
	case AnchorLinkEdge:
		if _, ok := g.LinkEdges[a.AnchorID]; !ok {
			return fmt.Errorf("%w: link edge %s", ErrAnchorNotFound, a.AnchorID)
		}
	default:
		return fmt.Errorf("%w: kind %d", ErrInvalidAnchor, a.AnchorKind)
	}
	g.Annotations[a.ID] = a
	return nil
}

// RemoveAnnotation removes an annotation by ID.
func (g *Graph) RemoveAnnotation(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Annotations[id]; !exists {
		return fmt.Errorf("%w: %s", ErrAnnotationNotFound, id)
	}
	delete(g.Annotations, id)
	return nil
}

// AnnotationsFor returns the annotations anchored to the given element.
// Callers performing destructive operations (RemoveNode, RemoveEdge) can use
// it beforehand to snapshot annotations that will be cascade-deleted.
func (g *Graph) AnnotationsFor(kind AnchorKind, anchorID string) []*Annotation {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.annotationsFor(kind, anchorID)
}

// annotationsFor is the internal unlocked version of AnnotationsFor.
func (g *Graph) annotationsFor(kind AnchorKind, anchorID string) []*Annotation {
	var result []*Annotation
	for _, a := range g.Annotations {
		if a.AnchorKind == kind && a.AnchorID == anchorID {
			result = append(result, a)
		}
	}
	return result
}
