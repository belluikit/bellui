package common

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/layout"
)

// The partial-repaint region is found by walking from the nodes whose objects
// reported a change since the last region, not from the roots: moving,
// resizing or hiding an object reports it (MarkPlacementDirty), and so does a
// refresh (FreeDirtyTextures). Before #599 every region walked every visible
// node, which on a phone cost as much as a millisecond per frame in a scroll
// of cards where one small object changed.
//
// What no node can report makes the next region walk everything, as all of
// them did before: showing a hidden object, which has no node (paintgen), a
// tree replaced (placeAll), and an object that cannot report at all, such as
// an app's own CanvasObject type, at a tree's root (foreignRoot). Such an
// object deeper in a tree has its parent walked by every region instead
// (foreignParents), so an app sees no change in what repaints.

// regionWalk says which trees a PartialDirtyRegion walk is in.
type regionWalk uint8

const (
	regionNone    regionWalk = iota // not in a region walk
	regionContent                   // the content, window head and menu trees
	regionOverlay                   // the overlay trees, which it walks whole
)

// placement says whether an object reports its moves to its canvas.
type placement uint8

const (
	placementUnknown placement = iota
	placementReported
	placementForeign
)

// reportsPlacement reports whether obj tells its canvas when it is moved,
// resized or hidden: Bellui's containers, canvas objects and widgets do; a
// type that implements CanvasObject itself may not. A type missing here is
// walked as if it did not, which costs time, never pixels.
func reportsPlacement(obj core.CanvasObject) bool {
	switch obj.(type) {
	case *bellui.Container, *layout.Spacer,
		*canvas.Arc, *canvas.BackdropBlur, *canvas.BoxShadow, *canvas.Circle,
		*canvas.GLView, *canvas.Image, *canvas.Line, *canvas.LinearGradient,
		*canvas.Path, *canvas.Polygon, *canvas.RadialGradient, *canvas.Raster,
		*canvas.Rectangle, *canvas.Ripple, *canvas.Scene3D, *canvas.StyledText,
		*canvas.Text, *canvas.TextOnPath:
		return true
	}
	// A widget built on widget.BaseWidget, or the internal one, reports
	// through it.
	_, ok := obj.(interface{ ExtendBaseWidget(core.Widget) })
	return ok
}

// MarkPlacementDirty records that obj may have moved, been resized or been
// hidden, so the next partial-repaint region walks from its node. An object
// with no node is not drawn, or is under one a walk reaches anyway: a new
// object is added through a container or widget that reports, and a shown one
// through paintgen. Safe from any goroutine.
func (c *Canvas) MarkPlacementDirty(obj core.CanvasObject) {
	if !build.PartialRepaint {
		return
	}
	if node, ok := c.nodeMap.Load(obj); ok {
		c.queuePlaced(node)
	}
}

// queuePlaced queues node for the next region walk, once.
func (c *Canvas) queuePlaced(node *RenderCacheNode) {
	if !node.placeQueued.Swap(true) {
		c.placeMu.Lock()
		c.placed = append(c.placed, node)
		c.placeMu.Unlock()
	}
}

// takePlaced returns the nodes queued since it was last called, and clears
// their queued marks.
func (c *Canvas) takePlaced() []*RenderCacheNode {
	c.placeMu.Lock()
	taken := c.placed
	c.placed = c.placedTaken[:0]
	c.placedTaken = taken
	c.placeMu.Unlock()
	for _, n := range taken {
		n.placeQueued.Store(false)
	}
	return taken
}

// regionRootsFor returns the nodes a region walk must start from, for the
// queued nodes placed and the parents of objects that do not report: each
// still in the tree, visible and drawn, and none below another. all is true
// when only a walk of everything will do.
func (c *Canvas) regionRootsFor(placed []*RenderCacheNode) (roots []*RenderCacheNode, all bool) {
	c.regionPass++
	pass := c.regionPass
	roots = c.regionRoots[:0]
	add := func(n *RenderCacheNode) bool {
		if cur, ok := c.nodeMap.Load(n.obj); !ok || cur != n {
			return true // left the tree, which cleared where it was drawn
		}
		// A hidden object's parent walks without it, clearing where it was.
		for !n.obj.Visible() {
			if n = n.parent; n == nil {
				return false
			}
		}
		if n.regionPass != pass {
			n.regionPass = pass
			roots = append(roots, n)
		}
		return true
	}
	for i, n := range placed {
		if !add(n) {
			all = true
		}
		placed[i] = nil
	}
	for n := range c.foreignParents {
		if cur, ok := c.nodeMap.Load(n.obj); !ok || cur != n {
			delete(c.foreignParents, n)
			n.inForeign = false
			continue
		}
		if !add(n) {
			all = true
		}
	}

	// Keep the roots no other root is above, and that are drawn.
	kept := roots[:0]
	for _, n := range roots {
		if drawnAlone(n, pass) {
			kept = append(kept, n)
		}
	}
	c.regionRoots = kept
	return kept, all
}

// drawnAlone reports whether n is drawn, under no culled or hidden ancestor,
// and no ancestor of it was chosen as a root in pass: a walk from that
// ancestor covers n.
func drawnAlone(n *RenderCacheNode, pass uint64) bool {
	for p := n.parent; p != nil; p = p.parent {
		if p.regionPass == pass || p.childrenCulled || !p.obj.Visible() {
			return false
		}
	}
	return true
}

// notePlacement records for a region walk what node's object reports, and
// marks its parent if it does not report.
func (c *Canvas) notePlacement(node *RenderCacheNode) {
	node.foreignChild = false
	if node.placement == placementUnknown {
		node.placement = placementReported
		if !reportsPlacement(node.obj) {
			node.placement = placementForeign
		}
	}
	if node.placement != placementForeign {
		return
	}
	if node.parent == nil {
		c.foreignRoot = true
	} else {
		node.parent.foreignChild = true
	}
}

// noteHiddenChild marks parent when a region walk skips a hidden child of it
// that does not report placement, and so would not report being shown.
func noteHiddenChild(parent *RenderCacheNode, obj core.CanvasObject) {
	if parent != nil && !reportsPlacement(obj) {
		parent.foreignChild = true
	}
}

// noteForeignChildren keeps node in foreignParents while a region walk finds
// a child of it that does not report.
func (c *Canvas) noteForeignChildren(node *RenderCacheNode) {
	if node.foreignChild == node.inForeign || node.childrenSkipped {
		return
	}
	node.inForeign = node.foreignChild
	if node.inForeign {
		if c.foreignParents == nil {
			c.foreignParents = make(map[*RenderCacheNode]struct{})
		}
		c.foreignParents[node] = struct{}{}
	} else {
		delete(c.foreignParents, node)
	}
}

// RegionVisits reports how many nodes the last PartialDirtyRegion visited.
func (c *Canvas) RegionVisits() int { return c.regionVisits }
