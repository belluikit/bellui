package widget

import (
	"fmt"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// TreeViewNodeID represents the unique id of a tree view node.
type TreeViewNodeID = string

const (
	allTreeViewNodesID     TreeViewNodeID = "_ALLNODES"
	onlyNewTreeViewNodesID TreeViewNodeID = "_ONLYNEWNODES"
)

// Declare conformity with interfaces
var (
	_ event.Focusable = (*TreeView)(nil)
	_ core.Widget     = (*TreeView)(nil)
	_ core.Accessible = (*TreeView)(nil)
)

// AccessibleRole returns [core.RoleTree].
func (tv *TreeView) AccessibleRole() core.AccessibleRole { return core.RoleTree }

// AccessibleLabel returns an empty string; tree views have no intrinsic label.
func (tv *TreeView) AccessibleLabel() string { return "" }

// TreeViewNode describes a single node's data in a TreeView.
type TreeViewNode struct {
	ID   TreeViewNodeID
	Text string
	Icon bellui.Resource // optional; nil means no icon
	Data any             // arbitrary user data
}

type internalNode struct {
	TreeViewNode
	parent   TreeViewNodeID
	children []TreeViewNodeID
	hasMore  bool
}

// TreeView displays hierarchical data with MD3 styling.
// It supports drag-to-reorder, inline rename, delete, icons, and chunked loading.
type TreeView struct {
	BaseWidget
	Root TreeViewNodeID // hidden root ID, default ""

	// Feature toggles
	AllowReorder   bool // enable drag-to-reorder within same parent
	AllowReparent  bool // enable drag to reparent into other branches
	AllowRename    bool // enable double-click inline rename
	AllowDelete    bool // enable Delete key and context menu delete
	ShowGuideLines bool // show vertical indent guide lines
	HideSeparators bool // hide row separators

	// Custom rendering (optional — defaults to icon + label)
	CreateNode func(branch bool) core.CanvasObject
	UpdateNode func(id TreeViewNodeID, branch bool, node core.CanvasObject)

	// ContextActions supplies extra entries for a node's right-click menu.
	// It is called per node, so the actions offered can depend on what that
	// node is, and its items are appended below the built-in Rename and
	// Delete entries with a separator between the two groups. Leaving it nil
	// — or returning no items — gives exactly the built-in menu.
	//
	// Since: 2.7
	ContextActions func(id TreeViewNodeID) []*bellui.MenuItem `json:"-"`

	// Event callbacks
	OnSelected     func(id TreeViewNodeID)                         `json:"-"`
	OnUnselected   func(id TreeViewNodeID)                         `json:"-"`
	OnBranchOpened func(id TreeViewNodeID)                         `json:"-"`
	OnBranchClosed func(id TreeViewNodeID)                         `json:"-"`
	OnMoved        func(id, newParentID TreeViewNodeID, index int) `json:"-"`
	OnRenamed      func(id TreeViewNodeID, newName string)         `json:"-"`
	OnDeleted      func(id TreeViewNodeID)                         `json:"-"`
	OnLoadMore     func(parentID TreeViewNodeID)                   `json:"-"`

	mu           sync.RWMutex
	nodeMap      map[TreeViewNodeID]*internalNode
	rootChildren []TreeViewNodeID
	open         map[TreeViewNodeID]bool
	selected     TreeViewNodeID
	currentFocus TreeViewNodeID
	focused      bool
	offset       geo.Position
	scroller     *widget.Scroll

	branchMinSize geo.Size
	leafMinSize   geo.Size

	// Drag state
	dragSourceID TreeViewNodeID
	dragActive   bool
	dropTarget   dropPosition

	// Chunked loading
	loadingInProgress map[TreeViewNodeID]bool

	// Rename requested for a node that had no row yet, applied by the content
	// renderer once that row exists.
	pendingRename TreeViewNodeID
}

type dropLocation int

const (
	dropBefore dropLocation = iota
	dropAfter
	dropInside
)

type dropPosition struct {
	targetID TreeViewNodeID
	location dropLocation
}

// NewTreeView creates an empty TreeView.
func NewTreeView() *TreeView {
	tv := &TreeView{
		nodeMap:           make(map[TreeViewNodeID]*internalNode),
		open:              make(map[TreeViewNodeID]bool),
		loadingInProgress: make(map[TreeViewNodeID]bool),
	}
	tv.ExtendBaseWidget(tv)
	return tv
}

// NewTreeViewWithNodes creates a TreeView pre-populated with root-level nodes.
func NewTreeViewWithNodes(nodes []TreeViewNode) *TreeView {
	tv := NewTreeView()
	for _, n := range nodes {
		tv.addNodeInternal("", n)
	}
	return tv
}

// === Data Model Methods ===

// AddNode adds a node as the last child of parentID.
// Use "" as parentID for root-level nodes.
func (tv *TreeView) AddNode(parentID TreeViewNodeID, node TreeViewNode) error {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	return tv.addNodeInternal(parentID, node)
}

func (tv *TreeView) addNodeInternal(parentID TreeViewNodeID, node TreeViewNode) error {
	if _, exists := tv.nodeMap[node.ID]; exists {
		return fmt.Errorf("node %q already exists", node.ID)
	}
	if parentID != "" {
		if _, ok := tv.nodeMap[parentID]; !ok {
			return fmt.Errorf("parent %q does not exist", parentID)
		}
	}

	n := &internalNode{
		TreeViewNode: node,
		parent:       parentID,
	}
	tv.nodeMap[node.ID] = n

	if parentID == "" {
		tv.rootChildren = append(tv.rootChildren, node.ID)
	} else {
		p := tv.nodeMap[parentID]
		p.children = append(p.children, node.ID)
	}
	return nil
}

// InsertNode adds a node as a child of parentID at the given index.
func (tv *TreeView) InsertNode(parentID TreeViewNodeID, index int, node TreeViewNode) error {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if _, exists := tv.nodeMap[node.ID]; exists {
		return fmt.Errorf("node %q already exists", node.ID)
	}
	if parentID != "" {
		if _, ok := tv.nodeMap[parentID]; !ok {
			return fmt.Errorf("parent %q does not exist", parentID)
		}
	}

	n := &internalNode{
		TreeViewNode: node,
		parent:       parentID,
	}
	tv.nodeMap[node.ID] = n

	if parentID == "" {
		tv.rootChildren = insertAt(tv.rootChildren, index, node.ID)
	} else {
		p := tv.nodeMap[parentID]
		p.children = insertAt(p.children, index, node.ID)
	}
	return nil
}

// SetChildren replaces all children of parentID with the given nodes.
func (tv *TreeView) SetChildren(parentID TreeViewNodeID, nodes []TreeViewNode) {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	// Remove old children recursively
	var oldChildren []TreeViewNodeID
	if parentID == "" {
		oldChildren = tv.rootChildren
	} else if p, ok := tv.nodeMap[parentID]; ok {
		oldChildren = p.children
	}
	for _, cid := range oldChildren {
		tv.removeNodeRecursive(cid)
	}

	// Add new children
	ids := make([]TreeViewNodeID, 0, len(nodes))
	for _, n := range nodes {
		in := &internalNode{
			TreeViewNode: n,
			parent:       parentID,
		}
		tv.nodeMap[n.ID] = in
		ids = append(ids, n.ID)
	}

	if parentID == "" {
		tv.rootChildren = ids
	} else if p, ok := tv.nodeMap[parentID]; ok {
		p.children = ids
	}
}

// RemoveNode removes a node and all its descendants.
func (tv *TreeView) RemoveNode(id TreeViewNodeID) {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	n, ok := tv.nodeMap[id]
	if !ok {
		return
	}

	// Remove from parent's children
	if n.parent == "" {
		tv.rootChildren = removeFrom(tv.rootChildren, id)
	} else if p, ok := tv.nodeMap[n.parent]; ok {
		p.children = removeFrom(p.children, id)
	}

	tv.removeNodeRecursive(id)
}

func (tv *TreeView) removeNodeRecursive(id TreeViewNodeID) {
	n, ok := tv.nodeMap[id]
	if !ok {
		return
	}
	for _, cid := range n.children {
		tv.removeNodeRecursive(cid)
	}
	delete(tv.nodeMap, id)
	delete(tv.open, id)
}

// UpdateNodeData updates the Text, Icon, and Data of an existing node.
func (tv *TreeView) UpdateNodeData(id TreeViewNodeID, node TreeViewNode) {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if n, ok := tv.nodeMap[id]; ok {
		n.Text = node.Text
		n.Icon = node.Icon
		n.Data = node.Data
	}
}

// GetNode returns a copy of the TreeViewNode for the given ID.
func (tv *TreeView) GetNode(id TreeViewNodeID) (TreeViewNode, bool) {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	n, ok := tv.nodeMap[id]
	if !ok {
		return TreeViewNode{}, false
	}
	return n.TreeViewNode, true
}

// ChildIDs returns the ordered list of child IDs for the given parent.
func (tv *TreeView) ChildIDs(parentID TreeViewNodeID) []TreeViewNodeID {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	if parentID == "" {
		result := make([]TreeViewNodeID, len(tv.rootChildren))
		copy(result, tv.rootChildren)
		return result
	}
	if n, ok := tv.nodeMap[parentID]; ok {
		result := make([]TreeViewNodeID, len(n.children))
		copy(result, n.children)
		return result
	}
	return nil
}

// IsBranch returns true if the node has children or hasMore is set.
func (tv *TreeView) IsBranch(id TreeViewNodeID) bool {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	n, ok := tv.nodeMap[id]
	if !ok {
		return false
	}
	return len(n.children) > 0 || n.hasMore
}

// SetHasMore marks a branch as having more children to load.
func (tv *TreeView) SetHasMore(parentID TreeViewNodeID, hasMore bool) {
	tv.mu.Lock()
	if n, ok := tv.nodeMap[parentID]; ok {
		n.hasMore = hasMore
	}
	if !hasMore {
		delete(tv.loadingInProgress, parentID)
	}
	tv.mu.Unlock()
	tv.Refresh()
}

// === Branch Control ===

// OpenBranch expands the branch with the given ID.
func (tv *TreeView) OpenBranch(id TreeViewNodeID) {
	tv.mu.Lock()
	if tv.open == nil {
		tv.open = make(map[TreeViewNodeID]bool)
	}
	tv.open[id] = true
	n, ok := tv.nodeMap[id]
	needsLoad := ok && n.hasMore && len(n.children) == 0 && !tv.loadingInProgress[id]
	if needsLoad {
		tv.loadingInProgress[id] = true
	}
	tv.mu.Unlock()

	if f := tv.OnBranchOpened; f != nil {
		f(id)
	}
	if needsLoad {
		if f := tv.OnLoadMore; f != nil {
			f(id)
		}
	}
	tv.Refresh()
}

// CloseBranch collapses the branch with the given ID.
func (tv *TreeView) CloseBranch(id TreeViewNodeID) {
	tv.mu.Lock()
	if tv.open == nil {
		tv.open = make(map[TreeViewNodeID]bool)
	}
	tv.open[id] = false
	tv.mu.Unlock()

	if f := tv.OnBranchClosed; f != nil {
		f(id)
	}
	tv.Refresh()
}

// ToggleBranch toggles the branch with the given ID.
func (tv *TreeView) ToggleBranch(id TreeViewNodeID) {
	if tv.IsBranchOpen(id) {
		tv.CloseBranch(id)
	} else {
		tv.OpenBranch(id)
	}
}

// IsBranchOpen returns true if the branch is expanded.
func (tv *TreeView) IsBranchOpen(id TreeViewNodeID) bool {
	if id == tv.Root {
		return true
	}
	tv.mu.RLock()
	v := tv.open[id]
	tv.mu.RUnlock()
	return v
}

// OpenAllBranches expands all branches recursively.
func (tv *TreeView) OpenAllBranches() {
	tv.mu.Lock()
	if tv.open == nil {
		tv.open = make(map[TreeViewNodeID]bool)
	}
	for id, n := range tv.nodeMap {
		if len(n.children) > 0 || n.hasMore {
			tv.open[id] = true
		}
	}
	tv.mu.Unlock()
	tv.Refresh()
}

// CloseAllBranches collapses all branches.
func (tv *TreeView) CloseAllBranches() {
	tv.mu.Lock()
	tv.open = make(map[TreeViewNodeID]bool)
	tv.mu.Unlock()
	tv.Refresh()
}

// === Selection ===

// Select selects the node with the given ID.
func (tv *TreeView) Select(id TreeViewNodeID) {
	tv.mu.Lock()
	if tv.selected == id {
		tv.mu.Unlock()
		return
	}
	old := tv.selected
	tv.selected = id
	tv.mu.Unlock()

	if old != "" {
		if f := tv.OnUnselected; f != nil {
			f(old)
		}
	}
	if f := tv.OnSelected; f != nil {
		f(id)
	}
	tv.ScrollTo(id)
	tv.setItemFocus(id)
	tv.Refresh()
}

// Unselect deselects the node with the given ID.
func (tv *TreeView) Unselect(id TreeViewNodeID) {
	tv.mu.Lock()
	if tv.selected != id {
		tv.mu.Unlock()
		return
	}
	tv.selected = ""
	tv.mu.Unlock()

	if f := tv.OnUnselected; f != nil {
		f(id)
	}
	tv.Refresh()
}

// UnselectAll clears all selection.
func (tv *TreeView) UnselectAll() {
	tv.mu.Lock()
	if tv.selected == "" {
		tv.mu.Unlock()
		return
	}
	old := tv.selected
	tv.selected = ""
	tv.mu.Unlock()

	if f := tv.OnUnselected; f != nil {
		f(old)
	}
	tv.Refresh()
}

// Selected returns the currently selected node ID, or "" if none.
func (tv *TreeView) Selected() TreeViewNodeID {
	tv.mu.RLock()
	s := tv.selected
	tv.mu.RUnlock()
	return s
}

// === Scrolling ===

// ScrollTo scrolls to make the node with the given ID visible.
func (tv *TreeView) ScrollTo(id TreeViewNodeID) {
	tv.mu.RLock()
	_, ok := tv.nodeMap[id]
	tv.mu.RUnlock()
	if !ok {
		return
	}

	// Ensure all ancestors are open
	tv.openAncestors(id)

	if tv.scroller == nil {
		return
	}

	// Calculate Y position of the node
	y := tv.calculateNodeY(id)
	if y < 0 {
		return
	}

	viewport := tv.scroller.Size()
	if y < tv.offset.Y {
		tv.scroller.Offset.Y = y
	} else if y+tv.leafMinSize.Height > tv.offset.Y+viewport.Height {
		tv.scroller.Offset.Y = y + tv.leafMinSize.Height - viewport.Height
	}
	tv.scroller.Refresh()
}

// ScrollToTop scrolls to the top.
func (tv *TreeView) ScrollToTop() {
	if tv.scroller != nil {
		tv.scroller.Offset.Y = 0
		tv.scroller.Refresh()
	}
}

// ScrollToBottom scrolls to the bottom.
func (tv *TreeView) ScrollToBottom() {
	if tv.scroller != nil {
		tv.scroller.ScrollToBottom()
	}
}

// ScrollToOffset scrolls to the given Y offset.
func (tv *TreeView) ScrollToOffset(offset float32) {
	if tv.scroller != nil {
		tv.scroller.Offset.Y = offset
		tv.scroller.Refresh()
	}
}

// GetScrollOffset returns the current Y scroll offset.
func (tv *TreeView) GetScrollOffset() float32 {
	return tv.offset.Y
}

// RefreshItem refreshes a single item.
func (tv *TreeView) RefreshItem(id TreeViewNodeID) {
	if r := cache.Renderer(tv); r != nil {
		tvr := r.(*treeViewRenderer)
		tvr.content.refreshForID(id)
	}
}

// === Keyboard Navigation ===

// FocusGained is called when the TreeView gains focus.
func (tv *TreeView) FocusGained() {
	if tv.currentFocus == "" {
		tv.mu.RLock()
		if len(tv.rootChildren) > 0 {
			tv.currentFocus = tv.rootChildren[0]
		}
		tv.mu.RUnlock()
	}
	tv.focused = true
	tv.RefreshItem(tv.currentFocus)
}

// FocusLost is called when the TreeView loses focus.
func (tv *TreeView) FocusLost() {
	tv.focused = false
	tv.RefreshItem(tv.currentFocus)
}

// TypedRune is a no-op.
func (tv *TreeView) TypedRune(_ rune) {}

// TypedKey handles keyboard navigation.
func (tv *TreeView) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeyDown:
		tv.focusNext()
	case event.KeyUp:
		tv.focusPrevious()
	case event.KeyRight:
		tv.mu.RLock()
		n, ok := tv.nodeMap[tv.currentFocus]
		isBranch := ok && (len(n.children) > 0 || n.hasMore)
		tv.mu.RUnlock()
		if isBranch {
			if tv.IsBranchOpen(tv.currentFocus) {
				tv.focusNext() // Move into first child
			} else {
				tv.OpenBranch(tv.currentFocus)
			}
		}
	case event.KeyLeft:
		tv.mu.RLock()
		n, ok := tv.nodeMap[tv.currentFocus]
		isBranch := ok && (len(n.children) > 0 || n.hasMore)
		parentID := ""
		if ok {
			parentID = n.parent
		}
		tv.mu.RUnlock()
		if isBranch && tv.IsBranchOpen(tv.currentFocus) {
			tv.CloseBranch(tv.currentFocus)
		} else if parentID != "" {
			tv.setItemFocus(parentID)
		}
	case event.KeySpace:
		if tv.currentFocus != "" {
			tv.Select(tv.currentFocus)
		}
	case event.KeyReturn:
		if tv.currentFocus != "" {
			tv.Select(tv.currentFocus)
		}
	case event.KeyDelete, event.KeyBackspace:
		tv.mu.RLock()
		sel := tv.selected
		tv.mu.RUnlock()
		if tv.AllowDelete && sel != "" {
			tv.RemoveNode(sel)
			if f := tv.OnDeleted; f != nil {
				f(sel)
			}
			tv.Refresh()
		}
	}
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (tv *TreeView) CreateRenderer() core.WidgetRenderer {
	tv.ExtendBaseWidget(tv)
	c := newTreeViewContent(tv)
	s := widget.NewScroll(c)
	tv.scroller = s
	r := &treeViewRenderer{
		BaseRenderer: widget.NewBaseRenderer([]core.CanvasObject{s}),
		tree:         tv,
		content:      c,
		scroller:     s,
	}
	s.OnScrolled = tv.offsetUpdated
	r.updateMinSizes()
	r.content.viewport = r.MinSize()
	return r
}

// MinSize returns the minimum size of this TreeView.
func (tv *TreeView) MinSize() geo.Size {
	tv.ExtendBaseWidget(tv)
	return tv.BaseWidget.MinSize()
}

// Resize handles size changes.
func (tv *TreeView) Resize(size geo.Size) {
	tv.BaseWidget.Resize(size)
}

// === Internal Helpers ===

func (tv *TreeView) offsetUpdated(pos geo.Position) {
	tv.offset = pos
	if r := cache.Renderer(tv); r != nil {
		tvr := r.(*treeViewRenderer)
		tvr.content.refreshForID(onlyNewTreeViewNodesID)
	}
}

func (tv *TreeView) setItemFocus(id TreeViewNodeID) {
	old := tv.currentFocus
	tv.currentFocus = id
	if old != "" && old != id {
		tv.RefreshItem(old)
	}
	if id != "" {
		tv.RefreshItem(id)
	}
}

func (tv *TreeView) focusNext() {
	found := false
	tv.walkAll(func(uid, _ TreeViewNodeID, _ bool, _ int) {
		if found {
			return
		}
		if uid == tv.currentFocus {
			found = true
			return
		}
	})
	// Walk again to find the one after currentFocus
	next := ""
	foundCurrent := false
	tv.walkAll(func(uid, _ TreeViewNodeID, _ bool, _ int) {
		if next != "" {
			return
		}
		if foundCurrent {
			next = uid
			return
		}
		if uid == tv.currentFocus {
			foundCurrent = true
		}
	})
	if next != "" {
		tv.setItemFocus(next)
		tv.ScrollTo(next)
	}
}

func (tv *TreeView) focusPrevious() {
	prev := ""
	tv.walkAll(func(uid, _ TreeViewNodeID, _ bool, _ int) {
		if uid == tv.currentFocus {
			return
		}
		prev = uid
	})
	// Walk to find the node just before currentFocus
	prev = ""
	tv.walkAll(func(uid, _ TreeViewNodeID, _ bool, _ int) {
		if uid == tv.currentFocus {
			return // stop updating prev
		}
		prev = uid
	})
	if prev != "" {
		tv.setItemFocus(prev)
		tv.ScrollTo(prev)
	}
}

func (tv *TreeView) openAncestors(id TreeViewNodeID) {
	tv.mu.RLock()
	n, ok := tv.nodeMap[id]
	tv.mu.RUnlock()
	if !ok {
		return
	}
	if n.parent != "" {
		tv.openAncestors(n.parent)
		tv.mu.Lock()
		if tv.open == nil {
			tv.open = make(map[TreeViewNodeID]bool)
		}
		tv.open[n.parent] = true
		tv.mu.Unlock()
	}
}

func (tv *TreeView) calculateNodeY(targetID TreeViewNodeID) float32 {
	th := tv.Theme()
	pad := th.Size(theme.SizeNamePadding)
	y := float32(0)
	found := float32(-1)

	tv.walkAll(func(uid, _ TreeViewNodeID, isBranch bool, depth int) {
		if found >= 0 {
			return
		}
		if tv.Root == "" && uid == "" {
			return
		}
		if y > 0 {
			y += pad // separator space
		}
		if uid == targetID {
			found = y
			return
		}
		m := tv.leafMinSize
		if isBranch {
			m = tv.branchMinSize
		}
		y += m.Height
	})
	return found
}

// walkAll walks all open nodes in pre-order DFS.
func (tv *TreeView) walkAll(fn func(uid, parentID TreeViewNodeID, isBranch bool, depth int)) {
	tv.mu.RLock()
	defer tv.mu.RUnlock()
	tv.walkChildren("", tv.rootChildren, 0, fn)
}

func (tv *TreeView) walkChildren(parentID TreeViewNodeID, children []TreeViewNodeID, depth int, fn func(uid, parentID TreeViewNodeID, isBranch bool, depth int)) {
	for _, cid := range children {
		n, ok := tv.nodeMap[cid]
		if !ok {
			continue
		}
		isBranch := len(n.children) > 0 || n.hasMore
		fn(cid, parentID, isBranch, depth)
		if isBranch && tv.open[cid] {
			tv.walkChildren(cid, n.children, depth+1, fn)
		}
	}
}

// isDescendantOf checks if candidateDescendant is a descendant of candidateAncestor.
func (tv *TreeView) isDescendantOf(candidateDescendant, candidateAncestor TreeViewNodeID) bool {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	for id := candidateDescendant; id != ""; {
		n, ok := tv.nodeMap[id]
		if !ok {
			return false
		}
		if n.parent == candidateAncestor {
			return true
		}
		id = n.parent
	}
	return false
}

// MoveNode moves a node to a new parent at the given index. Emits OnMoved.
func (tv *TreeView) MoveNode(id, newParentID TreeViewNodeID, index int) {
	tv.mu.Lock()
	n, ok := tv.nodeMap[id]
	if !ok {
		tv.mu.Unlock()
		return
	}

	// Remove from old parent
	if n.parent == "" {
		tv.rootChildren = removeFrom(tv.rootChildren, id)
	} else if p, ok := tv.nodeMap[n.parent]; ok {
		p.children = removeFrom(p.children, id)
	}

	// Insert at new parent
	n.parent = newParentID
	if newParentID == "" {
		tv.rootChildren = insertAt(tv.rootChildren, index, id)
	} else if p, ok := tv.nodeMap[newParentID]; ok {
		p.children = insertAt(p.children, index, id)
	}
	tv.mu.Unlock()

	if f := tv.OnMoved; f != nil {
		f(id, newParentID, index)
	}
	tv.Refresh()
}

// RenameNode updates a node's text. Emits OnRenamed.
func (tv *TreeView) RenameNode(id TreeViewNodeID, newName string) {
	tv.mu.Lock()
	if n, ok := tv.nodeMap[id]; ok {
		n.Text = newName
	}
	tv.mu.Unlock()

	if f := tv.OnRenamed; f != nil {
		f(id, newName)
	}
	tv.RefreshItem(id)
}

// BeginRename starts inline rename of a node, putting the row into the same
// editor a double-click or the context menu's Rename entry opens. Use it for a
// host-supplied menu item or toolbar button that should rename the way the
// built-in gesture does — [TreeView.RenameNode] writes the new text straight
// into the model, which is the wrong tool when the user is meant to type it.
//
// Ancestors are opened and the row scrolled into view first. A node added a
// moment earlier has no row yet, so the rename is held until the row exists and
// applied then; it is dropped if the row never appears. AllowRename gates the
// user's own rename gestures and does not apply here: calling this is the
// application asking for the editor.
//
// Since: 2.7
func (tv *TreeView) BeginRename(id TreeViewNodeID) {
	tv.mu.RLock()
	_, known := tv.nodeMap[id]
	tv.mu.RUnlock()
	if !known {
		return
	}

	tv.armPendingRename(id)
	tv.ScrollTo(id)

	// ScrollTo may have laid the tree out again already, in which case the row
	// exists and the layout hook has taken the request. Otherwise start it here
	// if the row is there, and leave it armed if it is not.
	if item := tv.itemForID(id); item != nil {
		switch taken := tv.takePendingRename(); taken {
		case id:
			item.startRename()
		case "":
			// The layout hook already started this rename.
		default:
			// A reentrant BeginRename superseded this one — ScrollTo fires
			// OnBranchOpened, and host code there may ask for another node.
			// The newest request wins; leave it armed.
			tv.armPendingRename(taken)
		}
	}
}

// itemForID returns the row currently rendering a node, or nil when the node
// is scrolled out of view, inside a closed branch, or not laid out yet.
func (tv *TreeView) itemForID(id TreeViewNodeID) *treeViewItem {
	r, ok := cache.Renderer(tv).(*treeViewRenderer)
	if !ok {
		return nil
	}
	cr, ok := cache.Renderer(r.content).(*treeViewContentRenderer)
	if !ok {
		return nil
	}
	return cr.items[id]
}

// armPendingRename records a rename to start as soon as the node has a row.
func (tv *TreeView) armPendingRename(id TreeViewNodeID) {
	tv.mu.Lock()
	tv.pendingRename = id
	tv.mu.Unlock()
}

// takePendingRename returns and clears any rename [TreeView.BeginRename] could
// not start because the row did not exist yet.
func (tv *TreeView) takePendingRename() TreeViewNodeID {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	id := tv.pendingRename
	tv.pendingRename = ""
	return id
}

// DeleteNode removes a node. Emits OnDeleted.
func (tv *TreeView) DeleteNode(id TreeViewNodeID) {
	tv.RemoveNode(id)
	if f := tv.OnDeleted; f != nil {
		f(id)
	}
	tv.Refresh()
}

// === Drag handling ===

func (tv *TreeView) handleDrag(sourceID TreeViewNodeID, e *event.DragEvent) {
	if !tv.dragActive {
		tv.dragSourceID = sourceID
		tv.dragActive = true
	}

	// Calculate which node the cursor is over
	// e.Position is relative to the dragged item; we need absolute Y in the tree content
	th := tv.Theme()
	pad := th.Size(theme.SizeNamePadding)
	mouseY := tv.offset.Y + e.AbsolutePosition.Y

	// Walk visible nodes to find drop target
	y := float32(0)
	newDrop := dropPosition{}

	tv.walkAll(func(uid, parentID TreeViewNodeID, isBranch bool, depth int) {
		if newDrop.targetID != "" {
			return // already found
		}
		if y > 0 {
			y += pad
		}
		m := tv.leafMinSize
		if isBranch {
			m = tv.branchMinSize
		}
		nodeTop := y
		nodeBottom := y + m.Height

		if mouseY >= nodeTop && mouseY <= nodeBottom {
			// Cursor is over this node
			if uid == tv.dragSourceID {
				y += m.Height
				return // can't drop on self
			}
			// Validate: can't drop into own descendant
			if tv.isDescendantOf(uid, tv.dragSourceID) {
				y += m.Height
				return
			}

			third := m.Height / 3
			if tv.AllowReparent && isBranch && mouseY > nodeTop+third && mouseY < nodeBottom-third {
				newDrop = dropPosition{targetID: uid, location: dropInside}
			} else if mouseY < nodeTop+m.Height/2 {
				newDrop = dropPosition{targetID: uid, location: dropBefore}
			} else {
				newDrop = dropPosition{targetID: uid, location: dropAfter}
			}
		}
		y += m.Height
	})

	if newDrop != tv.dropTarget {
		tv.dropTarget = newDrop
		tv.updateDropIndicator()
	}
}

func (tv *TreeView) finalizeDrag() {
	if !tv.dragActive {
		return
	}
	defer func() {
		tv.dragActive = false
		tv.dragSourceID = ""
		tv.dropTarget = dropPosition{}
		tv.hideDropIndicator()
		tv.Refresh()
	}()

	drop := tv.dropTarget
	if drop.targetID == "" {
		return
	}

	tv.mu.RLock()
	targetNode, targetOK := tv.nodeMap[drop.targetID]
	sourceNode, sourceOK := tv.nodeMap[tv.dragSourceID]
	tv.mu.RUnlock()
	if !targetOK || !sourceOK {
		return
	}

	switch drop.location {
	case dropInside:
		if !tv.AllowReparent {
			return
		}
		// Move source as last child of target
		tv.MoveNode(tv.dragSourceID, drop.targetID, len(targetNode.children))
		tv.mu.Lock()
		if tv.open == nil {
			tv.open = make(map[TreeViewNodeID]bool)
		}
		tv.open[drop.targetID] = true
		tv.mu.Unlock()

	case dropBefore, dropAfter:
		newParent := targetNode.parent
		sameParent := sourceNode.parent == newParent

		if sameParent && !tv.AllowReorder {
			return
		}
		if !sameParent && !tv.AllowReparent {
			return
		}

		// Find index of target in its parent's children
		tv.mu.RLock()
		var siblings []TreeViewNodeID
		if newParent == "" {
			siblings = tv.rootChildren
		} else if p, ok := tv.nodeMap[newParent]; ok {
			siblings = p.children
		}
		tv.mu.RUnlock()

		idx := 0
		for i, cid := range siblings {
			if cid == drop.targetID {
				idx = i
				break
			}
		}
		if drop.location == dropAfter {
			idx++
		}
		// Adjust index if source is before target in same parent
		if sameParent {
			for i, cid := range siblings {
				if cid == tv.dragSourceID && i < idx {
					idx--
					break
				}
			}
		}

		tv.MoveNode(tv.dragSourceID, newParent, idx)
	}
}

func (tv *TreeView) updateDropIndicator() {
	if r := cache.Renderer(tv); r != nil {
		tvr := r.(*treeViewRenderer)
		if cr := cache.Renderer(tvr.content); cr != nil {
			tcr := cr.(*treeViewContentRenderer)
			tcr.showDropIndicator(tv.dropTarget, tv)
		}
	}
}

func (tv *TreeView) hideDropIndicator() {
	if r := cache.Renderer(tv); r != nil {
		tvr := r.(*treeViewRenderer)
		if cr := cache.Renderer(tvr.content); cr != nil {
			tcr := cr.(*treeViewContentRenderer)
			tcr.hideDropIndicator()
		}
	}
}

// === Slice helpers ===

func insertAt(slice []TreeViewNodeID, index int, id TreeViewNodeID) []TreeViewNodeID {
	if index < 0 {
		index = 0
	}
	if index >= len(slice) {
		return append(slice, id)
	}
	slice = append(slice, "")
	copy(slice[index+1:], slice[index:])
	slice[index] = id
	return slice
}

func removeFrom(slice []TreeViewNodeID, id TreeViewNodeID) []TreeViewNodeID {
	for i, s := range slice {
		if s == id {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
