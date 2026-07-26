package core

import "github.com/belluikit/bellui/geo"

// AccessibleVirtualNode is a semantic node that is not backed by its own
// CanvasObject: a region inside a widget that renders several logical
// elements itself, such as a heading inside a RichText document. Bounds are
// relative to the providing widget; tree walkers convert them to absolute
// coordinates and attach the node as a child of the provider's node.
type AccessibleVirtualNode struct {
	AccessibleNode // Object is nil for virtual nodes

	Pos  geo.Position // top-left relative to the providing widget
	Size geo.Size

	// ID is a stable identity for the region (e.g. the segment that
	// produced it), letting platform backends reconcile elements across
	// updates the way they key real nodes by CanvasObject.
	ID any
}

// AccessibleVirtualNodeProvider is implemented by widgets that expose
// semantic regions without dedicated CanvasObjects. The provider is
// consulted by bounds-aware tree walkers after the widget's real accessible
// descendants are collected.
//
// Windowing (virtualized grids/lists). A provider backing a large virtualized
// collection — a table with a million logical rows that realizes only the
// visible cells — must NOT return a node per logical item: the walk allocates
// and sorts every returned node each pass. Instead return a bounded window
// (typically the rows near the viewport) and report the full extent through
// [AccessibleGridDimensioner] on the container, so assistive tech hears
// "row R of N" without the walk touching all N. Each windowed node still
// carries its own coordinates: for a grid, emit flat [RoleTableCell] nodes
// with HasCellPosition/CellRow/CellCol set on the embedded [AccessibleNode]
// (platform bridges group them into rows by CellRow — no nested row nodes are
// needed), and set [AccessibleVirtualNode.ID] to a stable per-item key so the
// backend can track a node across walks. Nodes whose bounds fall entirely
// outside the container's clip are culled by the backends; surfacing rows
// beyond the viewport for assistive-tech to navigate into (lazy realization)
// is a platform-virtualization concern tracked separately, not something a
// provider achieves by returning off-screen nodes here.
type AccessibleVirtualNodeProvider interface {
	// AccessibleVirtualNodes returns the current virtual nodes in reading
	// order — a bounded window for virtualized collections (see the type
	// doc). Implementations return nil when nothing is rendered yet.
	AccessibleVirtualNodes() []AccessibleVirtualNode
}

// ChildrenFunc returns the child objects of a given CanvasObject.
// Callers provide this to abstract away the difference between
// containers (direct children) and widgets (renderer children).
// Implementations typically use the internal render cache.
type ChildrenFunc func(CanvasObject) []CanvasObject

// AccessibleNode provides a snapshot of accessibility properties for a
// node discovered during a tree walk. Fields are populated via type
// assertions on the underlying [CanvasObject].
type AccessibleNode struct {
	Object       CanvasObject
	Role         AccessibleRole
	Label        string
	Description  string // Empty if Object does not implement AccessibleDescriber
	Value        string // Empty if Object does not implement AccessibleValuer
	State        AccessibleState
	Actions      []AccessibleAction
	HasRange     bool // False when AccessibleRange returned the (0, 0, 0) "not applicable" sentinel
	RangeMin     float64
	RangeMax     float64
	RangeCurrent float64

	HasCellPosition  bool // True when Object implements AccessibleGridItem
	CellRow, CellCol int  // Display coordinates in current presentation order (see AccessibleGridItem)
	Level            int  // 1-based hierarchy level; 0 when Object does not implement AccessibleLeveler

	HasGridDims        bool // True when Object implements AccessibleGridDimensioner
	GridRows, GridCols int  // Current presented extent, filter-aware (see AccessibleGridDimensioner)

	// NodeID is a stable identity used by platform bridges to reconcile this
	// node's element across walks instead of keying by CanvasObject: from
	// AccessibleIdentified for a realized node, or AccessibleVirtualNode.ID for
	// a virtual one. Nil ⇒ key by Object (the default for most widgets).
	NodeID any
}

// SnapshotAccessible captures the accessibility properties of obj into an
// [AccessibleNode]. ok is false when obj does not implement [Accessible].
// The optional interfaces ([AccessibleDescriber], [AccessibleValuer],
// [AccessibleStateful], [AccessibleActioner]) are queried the same way
// [WalkAccessibleTree] does for each visited node.
func SnapshotAccessible(obj CanvasObject) (node AccessibleNode, ok bool) {
	if _, isAccessible := obj.(Accessible); !isAccessible {
		return AccessibleNode{}, false
	}
	return buildNode(obj), true
}

func buildNode(obj CanvasObject) AccessibleNode {
	acc, ok := obj.(Accessible)
	if !ok {
		return AccessibleNode{}
	}

	node := AccessibleNode{
		Object: obj,
		Role:   acc.AccessibleRole(),
		Label:  acc.AccessibleLabel(),
	}
	if o, ok := obj.(AccessibleLabelOverrider); ok {
		if label := o.AccessibilityLabel(); label != "" {
			node.Label = label
		}
	}

	if d, ok := obj.(AccessibleDescriber); ok {
		node.Description = d.AccessibleDescription()
	}
	if v, ok := obj.(AccessibleValuer); ok {
		node.Value = v.AccessibleValue()
		node.RangeMin, node.RangeMax, node.RangeCurrent = v.AccessibleRange()
		// (0, 0, 0) is the AccessibleValuer sentinel for "no numeric range"
		// (text widgets implement the interface for their string value only).
		node.HasRange = node.RangeMin != 0 || node.RangeMax != 0 || node.RangeCurrent != 0
	}
	if s, ok := obj.(AccessibleStateful); ok {
		node.State = s.AccessibleState()
	}
	if a, ok := obj.(AccessibleActioner); ok {
		node.Actions = a.AccessibleActions()
	}
	if g, ok := obj.(AccessibleGridItem); ok {
		node.CellRow, node.CellCol = g.AccessibleCellPosition()
		node.HasCellPosition = true
	}
	if d, ok := obj.(AccessibleGridDimensioner); ok {
		node.GridRows, node.GridCols = d.AccessibleGridDimensions()
		node.HasGridDims = true
	}
	if l, ok := obj.(AccessibleLeveler); ok {
		node.Level = l.AccessibleLevel()
	}
	if id, ok := obj.(AccessibleIdentified); ok {
		node.NodeID = id.AccessibleIdentity()
	}

	return node
}

// WalkAccessibleTree traverses the visible object tree starting at root,
// calling visit for each object that implements [Accessible].
// The walk is depth-first, parent before children. The int parameter to
// visit is the current depth (root = 0).
// If visit returns true, the walk stops immediately.
//
// The children function is used to obtain child objects; callers in the
// driver layer supply a function that reads the render cache, while test
// code can provide a simpler implementation.
func WalkAccessibleTree(root CanvasObject, children ChildrenFunc, visit func(AccessibleNode, int) bool) {
	walkAccessible(root, children, visit, 0)
}

func walkAccessible(obj CanvasObject, children ChildrenFunc, visit func(AccessibleNode, int) bool, depth int) bool {
	if obj == nil || !obj.Visible() {
		return false
	}

	if _, ok := obj.(Accessible); ok {
		node := buildNode(obj)
		if visit(node, depth) {
			return true
		}
	}

	for _, child := range children(obj) {
		if walkAccessible(child, children, visit, depth+1) {
			return true
		}
	}

	return false
}
