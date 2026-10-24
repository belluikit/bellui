package widget

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// TreeNodeID represents the unique id of a tree node.
type TreeNodeID = string

const (
	// allTreeNodesID represents all tree nodes when refreshing requested nodes
	allTreeNodesID     TreeNodeID = "_ALLNODES"
	onlyNewTreeNodesID TreeNodeID = "_ONLYNEWNODES"
)

// Declare conformity with interfaces
var (
	_ event.Focusable = (*Tree)(nil)
	_ core.Accessible = (*Tree)(nil)
	_ core.Widget     = (*Tree)(nil)
)

// Tree widget displays hierarchical data.
// Each node of the tree must be identified by a Unique TreeNodeID.
//
// Since: 1.0
type Tree struct {
	BaseWidget
	Root TreeNodeID

	// HideSeparators hides the separators between tree nodes
	//
	// Since: 1.0
	HideSeparators bool

	ChildUIDs      func(uid TreeNodeID) (c []TreeNodeID)                     `json:"-"` // Return a sorted slice of Children TreeNodeIDs for the given Node TreeNodeID
	CreateNode     func(branch bool) (o core.CanvasObject)                   `json:"-"` // Return a CanvasObject that can represent a Branch (if branch is true), or a Leaf (if branch is false)
	IsBranch       func(uid TreeNodeID) (ok bool)                            `json:"-"` // Return true if the given TreeNodeID represents a Branch
	OnBranchClosed func(uid TreeNodeID)                                      `json:"-"` // Called when a Branch is closed
	OnBranchOpened func(uid TreeNodeID)                                      `json:"-"` // Called when a Branch is opened
	OnSelected     func(uid TreeNodeID)                                      `json:"-"` // Called when the Node with the given TreeNodeID is selected.
	OnUnselected   func(uid TreeNodeID)                                      `json:"-"` // Called when the Node with the given TreeNodeID is unselected.
	UpdateNode     func(uid TreeNodeID, branch bool, node core.CanvasObject) `json:"-"` // Called to update the given CanvasObject to represent the data at the given TreeNodeID

	branchMinSize  geo.Size
	currentFocus   TreeNodeID
	focused        bool
	leafMinSize    geo.Size
	offset         geo.Position
	open           map[TreeNodeID]bool
	scroller       *widget.Scroll
	selected       []TreeNodeID
	flatNodes      []TreeNodeID // cached visible-order list for keyboard navigation
	flatNodesDirty bool         // true when flatNodes needs rebuild
}

// AccessibleRole returns [core.RoleTree].
func (t *Tree) AccessibleRole() core.AccessibleRole { return core.RoleTree }

// AccessibleLabel returns an empty string; trees have no intrinsic label.
func (t *Tree) AccessibleLabel() string { return "" }

// NewTree returns a new performant tree widget defined by the passed functions.
// childUIDs returns the child TreeNodeIDs of the given node.
// isBranch returns true if the given node is a branch, false if it is a leaf.
// create returns a new template object that can be cached.
// update is used to apply data at specified data location to the passed template CanvasObject.
//
// Since: 1.0
func NewTree(childUIDs func(TreeNodeID) []TreeNodeID, isBranch func(TreeNodeID) bool, create func(bool) core.CanvasObject, update func(TreeNodeID, bool, core.CanvasObject)) *Tree {
	t := &Tree{ChildUIDs: childUIDs, IsBranch: isBranch, CreateNode: create, UpdateNode: update}
	t.ExtendBaseWidget(t)
	return t
}

// NewTreeWithData creates a new tree widget that will display the contents of the provided data.
//
// Since: 1.0
func NewTreeWithData(data binding.DataTree, createItem func(bool) core.CanvasObject, updateItem func(binding.DataItem, bool, core.CanvasObject)) *Tree {
	t := NewTree(
		data.ChildIDs,
		func(id TreeNodeID) bool {
			children := data.ChildIDs(id)
			return len(children) > 0
		},
		createItem,
		func(i TreeNodeID, branch bool, o core.CanvasObject) {
			item, err := data.GetItem(i)
			if err != nil {
				bellui.LogError(fmt.Sprintf("Error getting data item %s", i), err)
				return
			}
			updateItem(item, branch, o)
		},
	)

	data.AddListener(binding.NewDataListener(t.Refresh))
	return t
}

// NewTreeWithStrings creates a new tree with the given string map.
// Data must contain a mapping for the root, which defaults to empty string ("").
//
// Since: 1.0
func NewTreeWithStrings(data map[string][]string) (t *Tree) {
	t = &Tree{
		ChildUIDs: func(uid string) (c []string) {
			c = data[uid]
			return c
		},
		IsBranch: func(uid string) (b bool) {
			_, b = data[uid]
			return b
		},
		CreateNode: func(branch bool) core.CanvasObject {
			return NewLabel("Template Object")
		},
		UpdateNode: func(uid string, branch bool, node core.CanvasObject) {
			node.(*Label).SetText(uid)
		},
	}
	t.ExtendBaseWidget(t)
	return t
}

// CloseAllBranches closes all branches in the tree.
func (t *Tree) CloseAllBranches() {
	t.open = make(map[TreeNodeID]bool)
	t.flatNodesDirty = true
	t.Refresh()
}

// CloseBranch closes the branch with the given TreeNodeID.
func (t *Tree) CloseBranch(uid TreeNodeID) {
	t.ensureOpenMap()
	t.open[uid] = false
	t.flatNodesDirty = true
	if f := t.OnBranchClosed; f != nil {
		f(uid)
	}
	t.Refresh()
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (t *Tree) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	c := newTreeContent(t)
	s := widget.NewScroll(c)
	t.scroller = s
	r := &treeRenderer{
		BaseRenderer: widget.NewBaseRenderer([]core.CanvasObject{s}),
		tree:         t,
		content:      c,
		scroller:     s,
	}
	s.OnScrolled = t.offsetUpdated
	r.updateMinSizes()
	r.content.viewport = r.MinSize()
	return r
}

// IsBranchOpen returns true if the branch with the given TreeNodeID is expanded.
func (t *Tree) IsBranchOpen(uid TreeNodeID) bool {
	if uid == t.Root {
		return true // Root is always open
	}
	t.ensureOpenMap()
	return t.open[uid]
}

// FocusGained is called after this Tree has gained focus.
func (t *Tree) FocusGained() {
	if t.currentFocus == "" {
		if childUIDs := t.ChildUIDs; childUIDs != nil {
			if ids := childUIDs(""); len(ids) > 0 {
				t.setItemFocus(ids[0])
			}
		}
	}

	t.focused = true
	t.RefreshItem(t.currentFocus)
}

// FocusLost is called after this Tree has lost focus.
func (t *Tree) FocusLost() {
	t.focused = false
	t.Refresh() // Item(t.currentFocus)
}

// MinSize returns the size that this widget should not shrink below.
func (t *Tree) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

// RefreshItem refreshes a single item, specified by the item ID passed in.
//
// Since: 1.0
func (t *Tree) RefreshItem(id TreeNodeID) {
	t.flatNodesDirty = true
	if t.scroller == nil {
		return
	}
	t.scroller.Content.(*treeContent).refreshForID(id)
}

// focusedItemObject returns the realized tree node (branch or leaf) for the
// currently focused row, or nil when it is not currently laid out. It lets
// keyboard activation report the same node object the pointer path hit-tests
// (pre-rebrand tracker #834).
func (t *Tree) focusedItemObject() core.CanvasObject {
	if t.scroller == nil {
		return nil
	}
	tc, ok := t.scroller.Content.(*treeContent)
	if !ok {
		return nil
	}
	r, ok := cache.Renderer(tc).(*treeContentRenderer)
	if !ok {
		return nil
	}
	if b, ok := r.branches[t.currentFocus]; ok {
		return b
	}
	if l, ok := r.leaves[t.currentFocus]; ok {
		return l
	}
	return nil
}

// OpenAllBranches opens all branches in the tree.
func (t *Tree) OpenAllBranches() {
	t.ensureOpenMap()
	t.walkAll(func(uid, parent TreeNodeID, branch bool, depth int) {
		if branch {
			t.open[uid] = true
		}
	})
	t.flatNodesDirty = true
	t.Refresh()
}

// OpenBranch opens the branch with the given TreeNodeID.
func (t *Tree) OpenBranch(uid TreeNodeID) {
	t.ensureOpenMap()
	t.open[uid] = true
	t.flatNodesDirty = true
	if f := t.OnBranchOpened; f != nil {
		f(uid)
	}
	t.Refresh()
}

// Resize sets a new size for a widget.
func (t *Tree) Resize(size geo.Size) {
	if size == t.Size() {
		return
	}
	t.BaseWidget.Resize(size)
	if t.scroller == nil {
		return
	}
	t.scroller.Content.(*treeContent).refreshForID(onlyNewTreeNodesID)
}

// ScrollToBottom scrolls to the bottom of the tree.
//
// Since: 1.0
func (t *Tree) ScrollToBottom() {
	if t.scroller == nil {
		return
	}

	t.scroller.ScrollToBottom()
	t.offsetUpdated(t.scroller.Offset)
}

// ScrollTo scrolls to the node with the given id.
// If the node is inside a closed branch, all ancestor branches are opened first.
//
// Since: 1.0
func (t *Tree) ScrollTo(uid TreeNodeID) {
	if t.scroller == nil {
		return
	}

	t.openAncestors(uid)

	y, size, ok := t.offsetAndSize(uid)
	if !ok {
		return
	}

	newY := t.scroller.Offset.Y
	if y < t.scroller.Offset.Y {
		newY = y
	} else if y+size.Height > t.scroller.Offset.Y+t.scroller.Size().Height {
		newY = y + size.Height - t.scroller.Size().Height
	}

	t.scroller.ScrollToOffset(geo.NewPos(t.scroller.Offset.X, newY))
	t.offsetUpdated(t.scroller.Offset)
}

// ScrollToOffset scrolls the tree to the given offset position.
//
// Since: 1.0
func (t *Tree) ScrollToOffset(offset float32) {
	if t.scroller == nil {
		return
	}
	if offset < 0 {
		offset = 0
	}

	t.scroller.ScrollToOffset(geo.NewPos(t.scroller.Offset.X, offset))
	t.offsetUpdated(t.scroller.Offset)
}

// ScrollToTop scrolls to the top of the tree.
//
// Since: 1.0
func (t *Tree) ScrollToTop() {
	if t.scroller == nil {
		return
	}

	t.scroller.ScrollToTop()
	t.offsetUpdated(t.scroller.Offset)
}

// Select marks the specified node to be selected.
func (t *Tree) Select(uid TreeNodeID) {
	t.setItemFocus(uid)
	if len(t.selected) > 0 {
		if uid == t.selected[0] {
			return // no change
		}
		if f := t.OnUnselected; f != nil {
			f(t.selected[0])
		}
	}
	t.selected = []TreeNodeID{uid}
	t.Refresh()
	t.ScrollTo(uid)
	if f := t.OnSelected; f != nil {
		f(uid)
	}
}

func (t *Tree) setItemFocus(uid TreeNodeID) {
	if t.currentFocus == uid {
		return
	}

	previous := t.currentFocus
	t.currentFocus = uid
	t.RefreshItem(previous)
	t.ScrollTo(t.currentFocus)
	t.RefreshItem(t.currentFocus)
}

// ToggleBranch flips the state of the branch with the given TreeNodeID.
func (t *Tree) ToggleBranch(uid string) {
	if t.IsBranchOpen(uid) {
		t.CloseBranch(uid)
	} else {
		t.OpenBranch(uid)
	}
}

// TypedKey is called if a key event happens while this Tree is focused.
func (t *Tree) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeySpace:
		t.Select(t.currentFocus)
		// Report the realized tree node (RoleTreeItem) so keyboard activation
		// matches the pointer path, which hit-tests the node (pre-rebrand tracker #834, pre-rebrand tracker #700).
		if node := t.focusedItemObject(); node != nil {
			core.NotifyActivation(node, true)
		}
	case event.KeyDown:
		t.ensureFlatNodes()
		if idx := t.flatIndex(t.currentFocus); idx >= 0 && idx+1 < len(t.flatNodes) {
			t.setItemFocus(t.flatNodes[idx+1])
		}
	case event.KeyLeft:
		// If the current focus is on a branch which is open, just close it
		if t.IsBranch(t.currentFocus) && t.IsBranchOpen(t.currentFocus) {
			t.CloseBranch(t.currentFocus)
		} else {
			// Every other case should move the focus to the current parent node
			t.walk(t.Root, "", 0, func(id, p TreeNodeID, _ bool, _ int) {
				if id == t.currentFocus && p != "" {
					t.setItemFocus(p)
				}
			})
		}
	case event.KeyRight:
		if t.IsBranch(t.currentFocus) {
			t.OpenBranch(t.currentFocus)
		}
		children := []TreeNodeID{}
		if childUIDs := t.ChildUIDs; childUIDs != nil {
			children = childUIDs(t.currentFocus)
		}

		if len(children) > 0 {
			t.setItemFocus(children[0])
		}
	case event.KeyUp:
		t.ensureFlatNodes()
		if idx := t.flatIndex(t.currentFocus); idx > 0 {
			t.setItemFocus(t.flatNodes[idx-1])
		}
	}
}

// TypedRune is called if a text event happens while this Tree is focused.
func (t *Tree) TypedRune(_ rune) {
	// intentionally left blank
}

// Unselect marks the specified node to be not selected.
func (t *Tree) Unselect(uid TreeNodeID) {
	if len(t.selected) == 0 || t.selected[0] != uid {
		return
	}

	t.selected = nil
	t.Refresh()
	if f := t.OnUnselected; f != nil {
		f(uid)
	}
}

// UnselectAll sets all nodes to be not selected.
//
// Since: 1.0
func (t *Tree) UnselectAll() {
	if len(t.selected) == 0 {
		return
	}

	selected := t.selected
	t.selected = nil
	t.Refresh()
	if f := t.OnUnselected; f != nil {
		for _, uid := range selected {
			f(uid)
		}
	}
}

// findPath returns the list of ancestor IDs from root to uid (exclusive of uid).
// Returns nil if uid is not found in the tree.
func (t *Tree) findPath(uid TreeNodeID) []TreeNodeID {
	if t.ChildUIDs == nil || t.IsBranch == nil {
		return nil
	}
	var path []TreeNodeID
	if t.findPathRecursive(t.Root, uid, &path) {
		return path
	}
	return nil
}

func (t *Tree) findPathRecursive(current, target TreeNodeID, path *[]TreeNodeID) bool {
	if current == target {
		return true
	}
	if t.IsBranch == nil || !t.IsBranch(current) {
		return false
	}
	if t.ChildUIDs == nil {
		return false
	}
	*path = append(*path, current)
	for _, child := range t.ChildUIDs(current) {
		if t.findPathRecursive(child, target, path) {
			return true
		}
	}
	*path = (*path)[:len(*path)-1]
	return false
}

// openAncestors opens all ancestor branches of the given node, if they aren't already open.
func (t *Tree) openAncestors(uid TreeNodeID) {
	path := t.findPath(uid)
	if len(path) == 0 {
		return
	}
	t.ensureOpenMap()
	opened := false
	for _, ancestor := range path {
		if !t.open[ancestor] {
			t.open[ancestor] = true
			if f := t.OnBranchOpened; f != nil {
				f(ancestor)
			}
			opened = true
		}
	}
	if opened {
		t.Refresh()
	}
}

func (t *Tree) ensureOpenMap() {
	if t.open == nil {
		t.open = make(map[string]bool)
	}
}

func (t *Tree) offsetAndSize(uid TreeNodeID) (y float32, size geo.Size, found bool) {
	pad := t.Theme().Size(theme.SizeNamePadding)

	t.walkAll(func(id, _ TreeNodeID, branch bool, _ int) {
		m := t.leafMinSize
		if branch {
			m = t.branchMinSize
		}
		if id == uid {
			found = true
			size = m
		} else if !found {
			// Root node is not rendered unless it has been customized
			if t.Root == "" && id == "" {
				// This is root node, skip
				return
			}
			// If this is not the first item, add a separator
			if y > 0 {
				y += pad
			}

			y += m.Height
		}
	})
	return y, size, found
}

func (t *Tree) offsetUpdated(pos geo.Position) {
	if t.offset == pos {
		return
	}
	t.offset = pos
	t.scroller.Content.(*treeContent).refreshForID(onlyNewTreeNodesID)
}

// ensureFlatNodes rebuilds the cached flat node list if it is dirty.
func (t *Tree) ensureFlatNodes() {
	if !t.flatNodesDirty && t.flatNodes != nil {
		return
	}
	t.flatNodes = t.flatNodes[:0]
	t.walkAll(func(id, _ TreeNodeID, _ bool, _ int) {
		t.flatNodes = append(t.flatNodes, id)
	})
	t.flatNodesDirty = false
}

// flatIndex returns the index of the given node ID in the flat node list, or -1.
func (t *Tree) flatIndex(id TreeNodeID) int {
	for i, nid := range t.flatNodes {
		if nid == id {
			return i
		}
	}
	return -1
}

func (t *Tree) walk(uid, parent TreeNodeID, depth int, onNode func(TreeNodeID, TreeNodeID, bool, int)) {
	if isBranch := t.IsBranch; isBranch != nil {
		if isBranch(uid) {
			onNode(uid, parent, true, depth)
			if t.IsBranchOpen(uid) {
				if childUIDs := t.ChildUIDs; childUIDs != nil {
					for _, c := range childUIDs(uid) {
						t.walk(c, uid, depth+1, onNode)
					}
				}
			}
		} else {
			onNode(uid, parent, false, depth)
		}
	}
}

// walkAll visits every open node of the tree and calls the given callback with TreeNodeID, whether node is branch, and the depth of node.
func (t *Tree) walkAll(onNode func(TreeNodeID, TreeNodeID, bool, int)) {
	t.walk(t.Root, "", 0, onNode)
}

var _ core.WidgetRenderer = (*treeRenderer)(nil)

type treeRenderer struct {
	widget.BaseRenderer
	tree     *Tree
	content  *treeContent
	scroller *widget.Scroll
}

func (r *treeRenderer) MinSize() (min geo.Size) {
	min = r.scroller.MinSize()
	min = min.Max(r.tree.branchMinSize)
	min = min.Max(r.tree.leafMinSize)
	return min
}

func (r *treeRenderer) Layout(size geo.Size) {
	r.content.viewport = size
	r.scroller.Resize(size)
	r.tree.offsetUpdated(r.scroller.Offset)
}

func (r *treeRenderer) Refresh() {
	r.updateMinSizes()
	s := r.tree.Size()
	if s.IsZero() {
		r.tree.Resize(r.tree.MinSize())
	} else {
		r.Layout(s)
	}
	r.scroller.Refresh()
	r.content.Refresh()
	canvas.Refresh(r.tree.super())
}

func (r *treeRenderer) updateMinSizes() {
	if f := r.tree.CreateNode; f != nil {
		branch := createItemAndApplyThemeScope(func() core.CanvasObject { return f(true) }, r.tree)
		r.tree.branchMinSize = measureTemplate(newBranch(r.tree, branch))

		leaf := createItemAndApplyThemeScope(func() core.CanvasObject { return f(false) }, r.tree)
		r.tree.leafMinSize = measureTemplate(newLeaf(r.tree, leaf))
	}
}

var _ core.Widget = (*treeContent)(nil)

type treeContent struct {
	BaseWidget
	tree     *Tree
	viewport geo.Size

	nextRefreshID TreeNodeID
}

func newTreeContent(tree *Tree) (c *treeContent) {
	c = &treeContent{
		tree: tree,
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *treeContent) CreateRenderer() core.WidgetRenderer {
	return &treeContentRenderer{
		BaseRenderer: widget.BaseRenderer{},
		treeContent:  c,
		branches:     make(map[string]*branch),
		leaves:       make(map[string]*leaf),
	}
}

func (c *treeContent) Resize(size geo.Size) {
	if size == c.Size() {
		return
	}

	c.size = size

	c.Refresh() // trigger a redraw
}

func (c *treeContent) refreshForID(id TreeNodeID) {
	c.nextRefreshID = id
	c.BaseWidget.Refresh()
}

func (c *treeContent) Refresh() {
	c.nextRefreshID = allTreeNodesID
	c.BaseWidget.Refresh()
}

var _ core.WidgetRenderer = (*treeContentRenderer)(nil)

type treeContentRenderer struct {
	widget.BaseRenderer
	treeContent *treeContent
	separators  []core.CanvasObject
	objects     []core.CanvasObject
	branches    map[string]*branch
	leaves      map[string]*leaf
	branchPool  async.Pool[core.CanvasObject]
	leafPool    async.Pool[core.CanvasObject]

	wasVisible   []TreeNodeID
	visible      []TreeNodeID
	minSizeCache geo.Size
}

func (r *treeContentRenderer) Layout(size geo.Size) {
	th := r.treeContent.Theme()
	r.objects = nil
	branches := make(map[string]*branch)
	leaves := make(map[string]*leaf)

	pad := th.Size(theme.SizeNamePadding)
	offsetY := r.treeContent.tree.offset.Y
	viewport := r.treeContent.viewport
	width := geo.Max(size.Width, viewport.Width)
	separatorCount := 0
	separatorThickness := th.Size(theme.SizeNameSeparatorThickness)
	separatorSize := geo.NewSize(width, separatorThickness)
	separatorOff := (pad + separatorThickness) / 2
	hideSeparators := r.treeContent.tree.HideSeparators
	y := float32(0)

	r.wasVisible, r.visible = r.visible, r.wasVisible
	r.visible = r.visible[:0]

	// walkAll open branches and obtain nodes to render in scroller's viewport
	r.treeContent.tree.walkAll(func(uid, _ string, isBranch bool, depth int) {
		// Root node is not rendered unless it has been customized
		if r.treeContent.tree.Root == "" {
			depth = depth - 1
			if uid == "" {
				// This is root node, skip
				return
			}
		}

		// If this is not the first item, add a separator
		addSeparator := y > 0
		if addSeparator {
			y += pad
			separatorCount++
		}

		m := r.treeContent.tree.leafMinSize
		if isBranch {
			m = r.treeContent.tree.branchMinSize
		}
		if y+m.Height < offsetY {
			// Node is above viewport and not visible
		} else if y > offsetY+viewport.Height {
			// Node is below viewport and not visible
		} else {
			// Node is in viewport
			r.visible = append(r.visible, uid)

			if addSeparator && !hideSeparators {
				var separator core.CanvasObject
				if separatorCount < len(r.separators) {
					separator = r.separators[separatorCount]
					separator.Show() // it may previously have been hidden
				} else {
					separator = NewSeparator()
					r.separators = append(r.separators, separator)
				}
				separator.Move(geo.NewPos(0, y-separatorOff))
				separator.Resize(separatorSize)
				r.objects = append(r.objects, separator)
				separatorCount++
			}

			var n core.CanvasObject
			if isBranch {
				b, ok := r.branches[uid]
				if !ok {
					b = r.getBranch()
					if f := r.treeContent.tree.UpdateNode; f != nil {
						f(uid, true, b.Content())
					}
					b.update(uid, depth)
				}
				branches[uid] = b
				n = b
				r.objects = append(r.objects, b)
			} else {
				l, ok := r.leaves[uid]
				if !ok {
					l = r.getLeaf()
					if f := r.treeContent.tree.UpdateNode; f != nil {
						f(uid, false, l.Content())
					}
					l.update(uid, depth)
				}
				leaves[uid] = l
				n = l
				r.objects = append(r.objects, l)
			}
			if n != nil {
				n.Move(geo.NewPos(0, y))
				n.Resize(geo.NewSize(width, m.Height))
			}
		}
		y += m.Height
	})

	if hideSeparators {
		// start below iteration from 0 to hide all separators
		separatorCount = 0
	}
	// Hide any separators that haven't been reused
	for ; separatorCount < len(r.separators); separatorCount++ {
		r.separators[separatorCount].Hide()
	}

	// Release any nodes that haven't been reused
	for uid, b := range r.branches {
		if _, ok := branches[uid]; !ok {
			r.branchPool.Put(b)
		}
	}
	for uid, l := range r.leaves {
		if _, ok := leaves[uid]; !ok {
			r.leafPool.Put(l)
		}
	}

	r.branches = branches
	r.leaves = leaves
}

func (r *treeContentRenderer) MinSize() (min geo.Size) {
	if !r.minSizeCache.IsZero() {
		return r.minSizeCache
	}
	th := r.treeContent.Theme()
	pad := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)

	r.treeContent.tree.walkAll(func(uid, _ string, isBranch bool, depth int) {
		// Root node is not rendered unless it has been customized
		if r.treeContent.tree.Root == "" {
			depth = depth - 1
			if uid == "" {
				// This is root node, skip
				return
			}
		}

		// If this is not the first item, add a separator
		if min.Height > 0 {
			min.Height += pad
		}

		m := r.treeContent.tree.leafMinSize
		if isBranch {
			m = r.treeContent.tree.branchMinSize
		}
		m.Width += float32(depth) * (iconSize + pad)
		min.Width = geo.Max(min.Width, m.Width)
		min.Height += m.Height
	})

	r.minSizeCache = min
	return min
}

// Destroy releases the renderers of the separators the content keeps for
// reuse: those it is not showing are not among its objects, so destroying the
// tree's renderers would not reach them (#722).
func (r *treeContentRenderer) Destroy() {
	for _, s := range r.separators {
		releaseRenderer(s)
	}
}

func (r *treeContentRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *treeContentRenderer) Refresh() {
	r.minSizeCache = geo.Size{}
	r.refreshForID(r.treeContent.nextRefreshID)
	for _, s := range r.separators {
		s.Refresh()
	}
}

func (r *treeContentRenderer) refreshForID(toDraw TreeNodeID) {
	s := r.treeContent.Size()
	if s.IsZero() {
		r.treeContent.Resize(r.treeContent.MinSize().Max(r.treeContent.tree.Size()))
	} else {
		r.Layout(s)
	}

	if toDraw == onlyNewTreeNodesID {
		for id, b := range r.branches {
			if contains(r.visible, id) && !contains(r.wasVisible, id) {
				b.Refresh()
			}
		}
		return
	}

	for id, b := range r.branches {
		if toDraw != allTreeNodesID && id != toDraw {
			continue
		}

		b.Refresh()
	}
	for id, l := range r.leaves {
		if toDraw != allTreeNodesID && id != toDraw {
			continue
		}

		l.Refresh()
	}
	canvas.Refresh(r.treeContent.super())
}

func (r *treeContentRenderer) getBranch() (b *branch) {
	o := r.branchPool.Get()
	if o != nil {
		b = o.(*branch)
	} else {
		var content core.CanvasObject
		if f := r.treeContent.tree.CreateNode; f != nil {
			content = createItemAndApplyThemeScope(func() core.CanvasObject { return f(true) }, r.treeContent.tree)
		}
		b = newBranch(r.treeContent.tree, content)
	}
	return b
}

func (r *treeContentRenderer) getLeaf() (l *leaf) {
	o := r.leafPool.Get()
	if o != nil {
		l = o.(*leaf)
	} else {
		var content core.CanvasObject
		if f := r.treeContent.tree.CreateNode; f != nil {
			content = createItemAndApplyThemeScope(func() core.CanvasObject { return f(false) }, r.treeContent.tree)
		}
		l = newLeaf(r.treeContent.tree, content)
	}
	return l
}

var (
	_ event.Hoverable   = (*treeNode)(nil)
	_ core.CanvasObject = (*treeNode)(nil)
	_ event.Tappable    = (*treeNode)(nil)
)

type treeNode struct {
	BaseWidget
	tree     *Tree
	uid      string
	depth    int
	hovered  bool
	icon     core.CanvasObject
	isBranch bool
	content  core.CanvasObject
}

func (n *treeNode) Content() core.CanvasObject {
	return n.content
}

func (n *treeNode) CreateRenderer() core.WidgetRenderer {
	th := n.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	background := canvas.NewRectangle(th.Color(theme.ColorNameHover, v))
	background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	background.Hide()
	return &treeNodeRenderer{
		BaseRenderer: widget.BaseRenderer{},
		treeNode:     n,
		background:   background,
	}
}

func (n *treeNode) Indent() float32 {
	th := n.Theme()
	return float32(n.depth) * (th.Size(theme.SizeNameInlineIcon) + th.Size(theme.SizeNamePadding))
}

// HoverIn is called when a pointer enters the widget.
func (n *treeNode) HoverIn(*event.HoverEvent) {
	n.hovered = true
	n.partialRefresh()
}

// HoverMoved is called when a pointer hovers over the widget.
func (n *treeNode) HoverMoved(*event.HoverEvent) {
}

// HoverOut is called when a pointer exits the widget.
func (n *treeNode) HoverOut() {
	n.hovered = false
	n.partialRefresh()
}

func (n *treeNode) Tapped(*event.PointEvent) {
	n.tree.Select(n.uid)
	canvas := bellui.CurrentApp().Driver().CanvasForObject(n.tree.super())
	if canvas != nil && canvas.Focused() != n.tree {
		if !bellui.CurrentDevice().IsMobile() {
			canvas.Focus(n.tree.super().(event.Focusable))
		}
	}
	n.Refresh()
}

// AccessibleRole returns [core.RoleTreeItem].
func (n *treeNode) AccessibleRole() core.AccessibleRole { return core.RoleTreeItem }

// AccessibleLabel returns an empty string; the node's template content
// provides its own semantics as child nodes.
func (n *treeNode) AccessibleLabel() string { return "" }

// AccessibleState reports the selected state and, for branches, whether
// the branch is expanded.
func (n *treeNode) AccessibleState() core.AccessibleState {
	var state core.AccessibleState
	for _, sel := range n.tree.selected {
		if sel == n.uid {
			state |= core.StateSelected
			break
		}
	}
	if n.isBranch && n.tree.IsBranchOpen(n.uid) {
		state |= core.StateExpanded
	}
	return state
}

// AccessibleActions returns "select", plus "expand"/"collapse" for branches.
func (n *treeNode) AccessibleActions() []core.AccessibleAction {
	actions := []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this item"}}
	if n.isBranch {
		if n.tree.IsBranchOpen(n.uid) {
			actions = append(actions, core.AccessibleAction{Name: core.ActionCollapse, Description: "Collapse this branch"})
		} else {
			actions = append(actions, core.AccessibleAction{Name: core.ActionExpand, Description: "Expand this branch"})
		}
	}
	return actions
}

// PerformAccessibleAction selects the node or toggles branch expansion.
func (n *treeNode) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionSelect:
		n.tree.Select(n.uid)
	case core.ActionExpand:
		if n.isBranch {
			n.tree.OpenBranch(n.uid)
		}
	case core.ActionCollapse:
		if n.isBranch {
			n.tree.CloseBranch(n.uid)
		}
	}
}

// AccessibleLevel returns the node's 1-based nesting level; rendered trees
// lay rows out flat, so this carries the outline structure.
func (n *treeNode) AccessibleLevel() int { return n.depth + 1 }

func (n *treeNode) partialRefresh() {
	if r := cache.Renderer(n.super()); r != nil {
		r.(*treeNodeRenderer).partialRefresh()
	}
}

func (n *treeNode) update(uid string, depth int) {
	n.uid = uid
	n.depth = depth
	n.Hidden = false
	n.partialRefresh()
}

var _ core.WidgetRenderer = (*treeNodeRenderer)(nil)

type treeNodeRenderer struct {
	widget.BaseRenderer
	treeNode   *treeNode
	background *canvas.Rectangle
}

func (r *treeNodeRenderer) Layout(size geo.Size) {
	th := r.treeNode.Theme()
	pad := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	x := pad + r.treeNode.Indent()
	y := float32(0)
	r.background.Resize(size)
	if r.treeNode.icon != nil {
		r.treeNode.icon.Move(geo.NewPos(x, y))
		r.treeNode.icon.Resize(geo.NewSize(iconSize, size.Height))
	}
	x += iconSize
	x += pad
	if r.treeNode.content != nil {
		r.treeNode.content.Move(geo.NewPos(x, y))
		r.treeNode.content.Resize(geo.NewSize(size.Width-x, size.Height))
	}
}

func (r *treeNodeRenderer) MinSize() (min geo.Size) {
	if r.treeNode.content != nil {
		min = r.treeNode.content.MinSize()
	}
	th := r.treeNode.Theme()
	iconSize := th.Size(theme.SizeNameInlineIcon)

	min.Width += th.Size(theme.SizeNameInnerPadding) + r.treeNode.Indent() + iconSize
	min.Height = geo.Max(min.Height, iconSize)
	return min
}

func (r *treeNodeRenderer) Objects() (objects []core.CanvasObject) {
	objects = append(objects, r.background)
	if r.treeNode.content != nil {
		objects = append(objects, r.treeNode.content)
	}
	if r.treeNode.icon != nil {
		objects = append(objects, r.treeNode.icon)
	}
	return objects
}

func (r *treeNodeRenderer) Refresh() {
	if c := r.treeNode.content; c != nil {
		if f := r.treeNode.tree.UpdateNode; f != nil {
			f(r.treeNode.uid, r.treeNode.isBranch, c)
		}
	}
	r.partialRefresh()
}

func (r *treeNodeRenderer) partialRefresh() {
	th := r.treeNode.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	if r.treeNode.icon != nil {
		r.treeNode.icon.Refresh()
	}
	r.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	if len(r.treeNode.tree.selected) > 0 && r.treeNode.uid == r.treeNode.tree.selected[0] {
		r.background.FillColor = th.Color(theme.ColorNameSelection, v)
		r.background.Show()
	} else if r.treeNode.hovered || (r.treeNode.tree.focused && r.treeNode.tree.currentFocus == r.treeNode.uid) {
		r.background.FillColor = th.Color(theme.ColorNameHover, v)
		r.background.Show()
	} else {
		r.background.Hide()
	}
	r.background.Refresh()
	r.Layout(r.treeNode.Size())
	canvas.Refresh(r.treeNode.super())
}

var _ core.Widget = (*branch)(nil)

type branch struct {
	*treeNode
}

func newBranch(tree *Tree, content core.CanvasObject) (b *branch) {
	b = &branch{
		treeNode: &treeNode{
			tree:     tree,
			icon:     newBranchIcon(tree),
			isBranch: true,
			content:  content,
		},
	}
	b.ExtendBaseWidget(b)

	if cache.OverrideThemeMatchingScope(b, tree) {
		b.Refresh()
	}
	return b
}

func (b *branch) update(uid string, depth int) {
	b.treeNode.update(uid, depth)
	b.icon.(*branchIcon).update(uid)
}

var _ event.Tappable = (*branchIcon)(nil)

type branchIcon struct {
	Icon
	tree *Tree
	uid  string
}

func newBranchIcon(tree *Tree) (i *branchIcon) {
	i = &branchIcon{
		tree: tree,
	}
	i.ExtendBaseWidget(i)
	return i
}

func (i *branchIcon) Refresh() {
	if i.tree.IsBranchOpen(i.uid) {
		i.Resource = theme.MoveDownIcon()
	} else {
		i.Resource = theme.NavigateNextIcon()
	}
	i.Icon.Refresh()
}

func (i *branchIcon) Tapped(*event.PointEvent) {
	i.tree.ToggleBranch(i.uid)
}

func (i *branchIcon) update(uid string) {
	i.uid = uid
	i.Refresh()
}

var _ core.Widget = (*leaf)(nil)

type leaf struct {
	*treeNode
}

func newLeaf(tree *Tree, content core.CanvasObject) (l *leaf) {
	l = &leaf{
		&treeNode{
			tree:     tree,
			content:  content,
			isBranch: false,
		},
	}
	l.ExtendBaseWidget(l)

	if cache.OverrideThemeMatchingScope(l, tree) {
		l.Refresh()
	}
	return l
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
