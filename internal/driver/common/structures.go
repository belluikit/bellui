package common

import (
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/internal/async"
)

type deduplicatedObjectQueue struct {
	queue *async.CanvasObjectQueue

	// dedup holds the objects in the queue. A plain map, not a sync.Map: every
	// refresh adds and removes one, and a sync.Map allocates a node per store,
	// which was over a quarter of a labels frame's garbage. A map reuses its
	// slots once grown.
	mu    sync.Mutex
	dedup map[core.CanvasObject]struct{}
}

// In adds an object to the queue if it is not already present.
func (q *deduplicatedObjectQueue) In(obj core.CanvasObject) {
	q.mu.Lock()
	if _, exists := q.dedup[obj]; exists {
		q.mu.Unlock()
		return
	}
	if q.dedup == nil {
		q.dedup = make(map[core.CanvasObject]struct{})
	}
	q.dedup[obj] = struct{}{}
	q.mu.Unlock()

	q.queue.In(obj)
}

// Out removes and returns the next object from the queue.
func (q *deduplicatedObjectQueue) Out() core.CanvasObject {
	out := q.queue.Out()
	if out == nil {
		return nil
	}
	q.mu.Lock()
	delete(q.dedup, out)
	q.mu.Unlock()
	return out
}

// Len returns the number of elements in the queue.
func (q *deduplicatedObjectQueue) Len() uint64 {
	return q.queue.Len()
}

type renderCacheTree struct {
	root *RenderCacheNode
}

// RenderCacheNode represents a node in a render cache tree.
type RenderCacheNode struct {
	// structural data
	firstChild  *RenderCacheNode
	nextSibling *RenderCacheNode
	obj         core.CanvasObject
	parent      *RenderCacheNode
	// cache data
	minSize geo.Size
	// minSizeChanged marks that a child's min size changed during the current
	// EnsureMinSize walk, so this node must re-run its layout when the
	// post-order walk reaches it. A per-node flag, not a single shared
	// pending pointer: several subtrees can report changes in one walk, and a
	// later change must not cancel an earlier sibling's pending update (pre-rebrand tracker #1039).
	minSizeChanged bool
	// measured, measuredSize and minSizeDirty decide whether EnsureMinSize must
	// ask the object for its min size again (#464): a node is measured once,
	// and again when its object was refreshed (minSizeDirty, set from any
	// goroutine), its size changed, or a child's min size changed.
	measured     bool
	measuredSize geo.Size
	minSizeDirty atomic.Bool
	// descDirty marks that this node or a descendant was refreshed since the
	// last min-size pass, which must then visit it; a pass that walks only
	// changed paths skips the rest (#487). Main thread only.
	descDirty bool
	// queued marks the node as waiting in Canvas.dirtyNodes, so a node refreshed
	// many times is queued once.
	queued         atomic.Bool
	worldTransform geo.Matrix   // accumulated affine transform from root to this node
	clipPos        geo.Position // effective clip region origin (from nearest Scrollable/ClipToBounds ancestor)
	clipSize       geo.Size     // effective clip region size
	isBoundary     bool         // true if this node is a RepaintBoundary
	walkGen        uint64       // Canvas.walkGen of the last full walk that reached this node

	// partial-repaint bookkeeping: the world-space bounds this node occupied at
	// the previous paint, so a move/resize can clear where it left as well as
	// where it lands. Only maintained when the partialrepaint build tag is set.
	lastPaintPos  geo.Position
	lastPaintSize geo.Size
	lastPaintSet  bool
}

// IsBoundary returns true if this node is a RepaintBoundary.
func (r *RenderCacheNode) IsBoundary() bool {
	return r.isBoundary
}

// Obj returns the node object.
func (r *RenderCacheNode) Obj() core.CanvasObject {
	return r.obj
}

// Parent returns the parent node, or nil for root nodes.
func (r *RenderCacheNode) Parent() *RenderCacheNode {
	return r.parent
}

// WorldTransform returns the composed affine transform from root to this node.
// It incorporates all ancestor transforms plus this node's local transform.
func (r *RenderCacheNode) WorldTransform() geo.Matrix {
	return r.worldTransform
}

// dirtyRect tracks a rectangular dirty region using position + size.
// When set is false, the region is undefined (full repaint needed).
type dirtyRect struct {
	pos  geo.Position
	size geo.Size
	set  bool // true if a valid region has been computed
}

// Union expands the dirty region to include the given rectangle.
func (d *dirtyRect) Union(pos geo.Position, size geo.Size) {
	if !d.set {
		d.pos = pos
		d.size = size
		d.set = true
		return
	}

	// Compute union of two axis-aligned rectangles.
	minX := geo.Min(d.pos.X, pos.X)
	minY := geo.Min(d.pos.Y, pos.Y)
	maxX := geo.Max(d.pos.X+d.size.Width, pos.X+size.Width)
	maxY := geo.Max(d.pos.Y+d.size.Height, pos.Y+size.Height)
	d.pos = geo.NewPos(minX, minY)
	d.size = geo.NewSize(maxX-minX, maxY-minY)
}

// Intersects returns true if the given rectangle overlaps the dirty region.
func (d *dirtyRect) Intersects(pos geo.Position, size geo.Size) bool {
	if !d.set {
		return true // no region set means everything is dirty
	}
	return pos.X < d.pos.X+d.size.Width &&
		pos.X+size.Width > d.pos.X &&
		pos.Y < d.pos.Y+d.size.Height &&
		pos.Y+size.Height > d.pos.Y
}

// Reset clears the dirty region.
func (d *dirtyRect) Reset() {
	d.set = false
}

type overlayStack struct {
	internal.OverlayStack

	renderCaches []*renderCacheTree
}

func (o *overlayStack) Add(overlay core.CanvasObject) {
	if overlay == nil {
		return
	}

	o.renderCaches = append(o.renderCaches, &renderCacheTree{root: &RenderCacheNode{obj: overlay}})
	o.OverlayStack.Add(overlay)
}

func (o *overlayStack) Remove(overlay core.CanvasObject) {
	if overlay == nil || len(o.List()) == 0 {
		return
	}

	o.OverlayStack.Remove(overlay)
	overlayCount := len(o.List())

	for i := overlayCount; i < len(o.renderCaches); i++ {
		o.renderCaches[i] = nil // release memory reference to removed element
	}

	o.renderCaches = o.renderCaches[:overlayCount]
}
