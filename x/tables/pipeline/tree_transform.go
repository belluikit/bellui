package pipeline

// TreeNode represents a node in a hierarchical tree structure.
type TreeNode struct {
	// SourceIndex is the row index in the data source.
	SourceIndex int
	// Depth is the nesting level (0 = root).
	Depth int
	// HasChildren indicates this node can be expanded.
	HasChildren bool
	// Expanded indicates children are currently visible.
	Expanded bool
}

// TreeTransform converts flat rows into a tree structure based on parent-child relationships.
type TreeTransform struct {
	// GetParent returns the ID of a row's parent — the value GetID returns
	// for the parent row, not the parent's row index — or a negative value
	// for a root. A row whose parent ID matches no row becomes a root.
	//
	// The parent ID is an int and is compared with GetID's values as an
	// interface value, so it only matches IDs that GetID returns as Go ints:
	// with int64 or string IDs no parent is ever found and every row is a
	// root.
	GetParent func(rowData any) int
	// GetID returns a unique ID for a row (used for parent-child matching).
	GetID func(rowData any) any
	// Expanded tracks which nodes are expanded. Key is the row's ID.
	Expanded map[any]bool
	// TreeNodes is the output tree structure, used by the renderer.
	TreeNodes []TreeNode
}

// NewTreeTransform creates a tree transform. getParent returns a row's parent
// ID, as [TreeTransform.GetParent] describes.
func NewTreeTransform(getID func(any) any, getParent func(any) int) *TreeTransform {
	return &TreeTransform{
		GetID:     getID,
		GetParent: getParent,
		Expanded:  make(map[any]bool),
	}
}

func (t *TreeTransform) Apply(indices []int, source DataSource) []int {
	if t.GetParent == nil || t.GetID == nil {
		return indices
	}

	// Build parent-child map
	type node struct {
		idx      int
		id       any
		parentID int
		children []int // indices into nodes slice
	}

	nodes := make([]node, len(indices))
	idToIdx := make(map[any]int) // node ID -> index in nodes

	for i, srcIdx := range indices {
		row := source.Row(srcIdx)
		id := t.GetID(row)
		parent := t.GetParent(row)
		nodes[i] = node{idx: srcIdx, id: id, parentID: parent}
		idToIdx[id] = i
	}

	// Find roots (parentID == -1 or parent not found)
	var roots []int
	for i, n := range nodes {
		if n.parentID < 0 {
			roots = append(roots, i)
		} else if parentIdx, ok := idToIdx[n.parentID]; ok {
			nodes[parentIdx].children = append(nodes[parentIdx].children, i)
		} else {
			roots = append(roots, i) // orphan becomes root
		}
	}

	// DFS traversal
	var result []int
	t.TreeNodes = nil

	var walk func(nodeIdx, depth int)
	walk = func(nodeIdx, depth int) {
		n := nodes[nodeIdx]
		hasChildren := len(n.children) > 0
		expanded := t.Expanded[n.id]

		t.TreeNodes = append(t.TreeNodes, TreeNode{
			SourceIndex: n.idx,
			Depth:       depth,
			HasChildren: hasChildren,
			Expanded:    expanded,
		})
		result = append(result, n.idx)

		if expanded {
			for _, childIdx := range n.children {
				walk(childIdx, depth+1)
			}
		}
	}

	for _, rootIdx := range roots {
		walk(rootIdx, 0)
	}

	return result
}

// ToggleNode toggles the expanded state of a node by its ID.
func (t *TreeTransform) ToggleNode(id any) {
	t.Expanded[id] = !t.Expanded[id]
}

// ExpandAll expands all nodes.
func (t *TreeTransform) ExpandAll(indices []int, source DataSource) {
	for _, idx := range indices {
		row := source.Row(idx)
		id := t.GetID(row)
		t.Expanded[id] = true
	}
}

// CollapseAll collapses all nodes.
func (t *TreeTransform) CollapseAll() {
	t.Expanded = make(map[any]bool)
}
