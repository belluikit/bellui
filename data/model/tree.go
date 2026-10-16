package model

import (
	"sort"
	"sync"

	"github.com/belluikit/bellui/data/binding"
)

// TreeSource is the read side of a hierarchical, string-keyed tree — the shape a
// widget.Tree consumes. Root returns the id of the (conventionally invisible)
// root node; Children returns the child ids of a node in source order; IsBranch
// reports whether a node may have children (so a tree can draw an expand
// affordance for it).
//
// It is the tree-shaped analogue of [Model] for the flat List/Table layer. Where
// [Model] is a flat sequence, a TreeSource is a hierarchy, so [TreeProxy] filters
// and sorts it with tree semantics a flat proxy cannot express: ancestor-keeping
// filtering and per-sibling-group ordering.
//
// Since: 1.0
type TreeSource interface {
	// Root returns the id of the root node. widget.Tree treats the root itself
	// as invisible and shows its children as the top level.
	Root() string
	// Children returns the child ids of uid in source order.
	Children(uid string) []string
	// IsBranch reports whether uid may have children.
	IsBranch(uid string) bool
}

// callbackTreeSource is a [TreeSource] backed by plain functions, mirroring the
// callback shape a widget.Tree already exposes (Root + ChildUIDs + IsBranch).
type callbackTreeSource struct {
	root     string
	children func(uid string) []string
	isBranch func(uid string) bool
}

// NewTreeSource wraps a root id and the ChildUIDs/IsBranch callbacks a
// widget.Tree already uses into a [TreeSource], so an existing tree can be put
// behind a [TreeProxy] without a new data structure. Nil callbacks are treated as
// "no children" / "not a branch".
//
// Since: 1.0
func NewTreeSource(root string, children func(uid string) []string, isBranch func(uid string) bool) TreeSource {
	return &callbackTreeSource{root: root, children: children, isBranch: isBranch}
}

func (s *callbackTreeSource) Root() string { return s.root }

func (s *callbackTreeSource) Children(uid string) []string {
	if s.children == nil {
		return nil
	}
	return s.children(uid)
}

func (s *callbackTreeSource) IsBranch(uid string) bool {
	if s.isBranch == nil {
		return false
	}
	return s.isBranch(uid)
}

// TreeProxy wraps a [TreeSource] and presents a filtered and/or sorted view of it
// that is itself a [TreeSource], so it can back a widget.Tree (see
// widget.BindTreeModel) or be composed with another proxy. It is the tree-shaped
// counterpart of [SortProxy]/[FilterProxy], with two semantics unique to a
// hierarchy:
//
//   - Filtering is ancestor-keeping. A node is visible when it matches the
//     predicate itself or has any visible (matching) descendant, so a deep match
//     stays reachable: its whole ancestor chain is kept even though those
//     ancestors do not match. Non-matching nodes with no matching descendant are
//     dropped. Set [TreeProxy.SetKeepMatchingSubtree] to additionally reveal the
//     entire subtree under a branch that matches by its own id (file-explorer
//     style), rather than only the paths that reach a match.
//
//   - Sorting is per-sibling-group: the children of each node are ordered among
//     themselves by the comparator, and the hierarchy is preserved. There is no
//     global flattened order.
//
// The comparator and predicate both receive a node id (the string key). When the
// user-visible label differs from the id, map id→label inside the closure (as
// [StringLess] with a label lookup, or via [lang.Compare]).
//
// A filter change walks the source tree once (O(n)) to recompute the visible set;
// a sort-only change is lazy (children are sorted per parent on first access and
// cached). Neither does per-frame work, so a bound widget keeps its O(visible)
// cost. Unlike the flat proxies the source here is a raw callback tree rather than
// an observable [Model], so when the underlying data changes call
// [TreeProxy.Refresh] to rebuild and notify.
//
// Since: 1.0
type TreeProxy struct {
	listeners

	mu          sync.Mutex
	src         TreeSource
	less        func(a, b string) bool
	keep        func(uid string) bool
	keepSubtree bool

	// childCache holds the sorted (and, when filtering, filtered) child list per
	// parent id. It is cleared on every rebuild; in the no-filter path it is
	// filled lazily on first access.
	childCache map[string][]string
	// visible records, when filtering, whether each reachable node is shown.
	visible   map[string]bool
	filtering bool
}

var (
	_ TreeSource       = (*TreeProxy)(nil)
	_ binding.DataItem = (*TreeProxy)(nil)
)

// NewTreeProxy returns a proxy over src. Until a comparator is set with
// [TreeProxy.SetLess] children keep their source order, and until a predicate is
// set with [TreeProxy.SetPredicate] every node is visible.
//
// Since: 1.0
func NewTreeProxy(src TreeSource) *TreeProxy {
	return &TreeProxy{src: src, childCache: map[string][]string{}}
}

// SetLess replaces the per-sibling-group comparator (nil restores source order)
// and re-sorts. It fires the change listeners.
func (p *TreeProxy) SetLess(less func(a, b string) bool) {
	p.mu.Lock()
	p.less = less
	p.rebuildLocked()
	p.mu.Unlock()
	p.notify()
}

// SetPredicate replaces the keep predicate (nil keeps every node) and re-filters
// with ancestor-keeping semantics. For search-as-you-type, debounce calls on the
// input side; each call is a single O(n) walk. It fires the change listeners.
func (p *TreeProxy) SetPredicate(keep func(uid string) bool) {
	p.mu.Lock()
	p.keep = keep
	p.rebuildLocked()
	p.mu.Unlock()
	p.notify()
}

// SetKeepMatchingSubtree controls what a branch that matches the predicate by its
// own id reveals. When false (the default) only the paths that reach a match are
// kept, so a matching branch shows just the descendants that themselves match (or
// lead to a match). When true the entire subtree under a matching branch is kept
// visible, matching the common file-explorer behaviour of "folder name matches →
// show its contents". It fires the change listeners.
func (p *TreeProxy) SetKeepMatchingSubtree(keep bool) {
	p.mu.Lock()
	p.keepSubtree = keep
	p.rebuildLocked()
	p.mu.Unlock()
	p.notify()
}

// Refresh rebuilds the view from the current source contents and fires the change
// listeners. Call it after the underlying tree data changes (the source is a raw
// callback tree, not an observable model, so the proxy cannot detect this itself).
func (p *TreeProxy) Refresh() {
	p.mu.Lock()
	p.rebuildLocked()
	p.mu.Unlock()
	p.notify()
}

// Root returns the source root id unchanged.
func (p *TreeProxy) Root() string { return p.src.Root() }

// Children returns the visible, sorted child ids of uid. The returned slice is a
// copy the caller may retain or mutate.
func (p *TreeProxy) Children(uid string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.childCache[uid]; ok {
		return append([]string(nil), c...)
	}
	if p.filtering {
		// A filtered rebuild pre-populates every reachable node; a miss means the
		// node is not visible, so it has no visible children.
		return nil
	}

	// No-filter path: sort the source children lazily and cache them.
	kids := append([]string(nil), p.src.Children(uid)...)
	p.sortLocked(kids)
	p.childCache[uid] = kids
	return append([]string(nil), kids...)
}

// IsBranch reports whether uid should be drawn as an expandable branch. Without a
// filter it passes through to the source. Under a filter it reports true only when
// uid has at least one visible child, so the tree never shows an expand
// affordance that would reveal nothing.
func (p *TreeProxy) IsBranch(uid string) bool {
	p.mu.Lock()
	filtering := p.filtering
	p.mu.Unlock()

	if !filtering {
		return p.src.IsBranch(uid)
	}
	return len(p.Children(uid)) > 0
}

// rebuildLocked recomputes the cached view. The caller must hold p.mu.
func (p *TreeProxy) rebuildLocked() {
	p.childCache = map[string][]string{}
	p.visible = nil
	p.filtering = p.keep != nil
	if p.filtering {
		p.visible = map[string]bool{}
		p.computeLocked(p.src.Root(), true, false, map[string]bool{})
	}
}

// computeLocked runs a post-order DFS from uid, recording each reachable node's
// visibility and its sorted, visible child list. matchedAncestor is true when an
// ancestor matched and keepSubtree is on (so the whole subtree is kept). It
// returns whether uid's subtree — including uid — contains any visible node. The
// root is never subjected to the predicate; it exists only to carry the top level.
func (p *TreeProxy) computeLocked(uid string, isRoot, matchedAncestor bool, seen map[string]bool) bool {
	if seen[uid] {
		return false // defensive cycle guard for a malformed source
	}
	seen[uid] = true

	selfMatch := false
	if !isRoot {
		selfMatch = p.keep(uid)
	}
	subtreeMatched := matchedAncestor || (p.keepSubtree && selfMatch)

	kids := p.src.Children(uid)
	visKids := make([]string, 0, len(kids))
	for _, k := range kids {
		if p.computeLocked(k, false, subtreeMatched, seen) {
			visKids = append(visKids, k)
		}
	}
	p.sortLocked(visKids)
	p.childCache[uid] = visKids

	vis := isRoot || selfMatch || matchedAncestor || len(visKids) > 0
	if !isRoot {
		p.visible[uid] = vis
	}
	return vis
}

// sortLocked orders ids in place by the comparator, if one is set. The caller must
// hold p.mu.
func (p *TreeProxy) sortLocked(ids []string) {
	if p.less == nil {
		return
	}
	// Stable so equal-ordering nodes keep source order — a predictable sort.
	sort.SliceStable(ids, func(i, j int) bool {
		return p.less(ids[i], ids[j])
	})
}
