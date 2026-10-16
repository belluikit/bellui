//go:build accessibility

package accessibility

import (
	"sort"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// Node is one element of the semantic tree: the accessibility properties of
// a visible [core.Accessible] object plus its geometry within the canvas.
type Node struct {
	core.AccessibleNode

	// Pos is the object's absolute position in canvas coordinates
	// (device-independent units). Backends convert to screen pixels with
	// internal/scale.ToScreenCoordinate against their canvas.
	Pos geo.Position
	// Size is the object's current size in device-independent units.
	Size geo.Size

	// Clip is the bounds of the nearest clipping ancestor (a scrollable, a
	// clipping container or a clipping renderer) in absolute canvas
	// coordinates, valid only when HasClip is true. Backends can use it to
	// cull or trim elements scrolled out of view.
	Clip    geo.Rectangle
	HasClip bool

	// Children are the accessible descendants whose closest accessible
	// ancestor is this node, in tree (reading) order.
	Children []*Node

	// ScrollAncestor is the nearest enclosing object that handles scroll
	// (implements event.Scrollable) — a container.Scroll, or the internal
	// scroller of a List/Tree/Table — or nil when the node is not inside a
	// scroll area. Scroll containers are not themselves accessible nodes, so
	// this is the only handle a flat mobile backend has to route a screen
	// reader's scroll gesture to the right scroller (pre-rebrand tracker #762). Backends that do
	// not scroll (desktop, where the AT scrolls by following focus) ignore it.
	ScrollAncestor core.CanvasObject
}

// BuildTree walks the given object trees and returns the accessible nodes
// found in them, in tree order (see the package documentation for the
// ordering contract). Invisible objects and their subtrees are skipped.
// Objects that do not implement [core.Accessible] contribute no node but are
// descended through, so their accessible descendants attach to the closest
// accessible ancestor — or become roots of the returned forest.
//
// The walk descends both containers and rendered widgets (renderer objects),
// so accessible widgets inside scroll areas or other widgets' renderers are
// found; positions accumulate the same way the paint and hit-test walks do.
func BuildTree(roots ...core.CanvasObject) []*Node {
	var forest []*Node
	for _, root := range roots {
		if root == nil {
			continue
		}

		var stack []*Node
		var scrollStack []core.CanvasObject // nearest-scrollable-ancestor stack
		beforeChildren := func(obj core.CanvasObject, pos geo.Position, clipPos geo.Position, clipSize geo.Size) bool {
			// Capture the enclosing scroller before pushing this object, so a
			// node's ScrollAncestor is a genuine ancestor and never itself.
			var scrollAnc core.CanvasObject
			if len(scrollStack) > 0 {
				scrollAnc = scrollStack[len(scrollStack)-1]
			}
			// Track scrollers whether or not they are accessible (a
			// container.Scroll is not), so items inside them can find them.
			if _, isScroll := obj.(event.Scrollable); isScroll {
				scrollStack = append(scrollStack, obj)
			}

			snapshot, ok := core.SnapshotAccessible(obj)
			if !ok {
				return false
			}

			node := &Node{AccessibleNode: snapshot, Pos: pos, Size: obj.Size(), ScrollAncestor: scrollAnc}
			if clipSize != driver.NoClipBounds {
				node.Clip = geo.Rectangle{Pos: clipPos, Size: clipSize}
				node.HasClip = true
			}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			} else {
				forest = append(forest, node)
			}
			stack = append(stack, node)
			return false
		}
		afterChildren := func(obj core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
			if len(scrollStack) > 0 && scrollStack[len(scrollStack)-1] == obj {
				scrollStack = scrollStack[:len(scrollStack)-1]
			}
			if len(stack) > 0 && stack[len(stack)-1].Object == obj {
				top := stack[len(stack)-1]
				if top.Role.ChildrenPresentational() {
					// A leaf control (button, checkbox, tab, ...) is one
					// semantic unit: the accessible widgets its renderer is
					// built from only paint it and would duplicate its label
					// or value.
					top.Children = nil
				} else if top.Role.NonInteractiveChildrenPresentational() {
					// A text-bearing control drops the internal widgets that
					// paint its value but keeps embedded interactive controls
					// (an Entry's ActionItem button, links in rich text).
					top.Children = keepInteractiveDescendants(top.Children)
				}
				if vp, isProvider := obj.(core.AccessibleVirtualNodeProvider); isProvider {
					// Virtual nodes (e.g. headings inside RichText, pre-rebrand tracker #533) are
					// appended after the prune so it cannot drop them, then the
					// children are re-sorted by position so sequential AT
					// navigation reads headings and links in document order
					// rather than all real nodes first (pre-rebrand tracker #545).
					virtual := vp.AccessibleVirtualNodes()
					for _, v := range virtual {
						// The near-viewport window nodes inherit the provider's
						// clip so backends cull the ones scrolled out of view.
						child := virtualToNode(top, v)
						child.Clip = top.Clip
						child.HasClip = top.HasClip
						top.Children = append(top.Children, child)
					}
					if len(virtual) > 0 && len(top.Children) > len(virtual) {
						sort.SliceStable(top.Children, func(i, j int) bool {
							a, b := top.Children[i], top.Children[j]
							if a.Pos.Y != b.Pos.Y {
								return a.Pos.Y < b.Pos.Y
							}
							return a.Pos.X < b.Pos.X
						})
					}
				}
				stack = stack[:len(stack)-1]
			}
		}

		driver.WalkVisibleObjectTree(root, beforeChildren, afterChildren)
	}
	return forest
}

// virtualToNode converts a semantic node that has no CanvasObject of its own —
// a window node from [core.AccessibleVirtualNodeProvider] or an on-demand cell
// from [core.AccessibleVirtualCellRealizer] — into a wire Node positioned in
// absolute canvas coordinates relative to parent. Its stable identity
// ([core.AccessibleVirtualNode.ID]) rides the shared [core.AccessibleNode.NodeID]
// channel, the same one AccessibleIdentified realized nodes use, so platform
// bridges key virtual and realized elements uniformly. The caller sets Clip:
// window nodes inherit the provider's clip (so off-view ones are culled); a
// realized off-viewport cell carries none (it is delivered outside the walk and
// must never be culled).
func virtualToNode(parent *Node, v core.AccessibleVirtualNode) *Node {
	an := v.AccessibleNode
	an.NodeID = v.ID
	// A virtual node scrolls with its provider: its scroll target is the
	// provider itself when that handles scroll events, else the provider's own
	// nearest enclosing scroller. Without this a virtual cell/heading has no
	// ScrollAncestor at all — the walk only stacks real ancestors — so the
	// mobile backends' AT scroll gesture (pre-rebrand tracker #762 Android, pre-rebrand tracker #952 iOS) would find
	// nothing to page.
	scrollAnc := parent.ScrollAncestor
	if _, ok := parent.Object.(event.Scrollable); ok {
		scrollAnc = parent.Object
	}
	return &Node{
		AccessibleNode: an,
		Pos:            geo.NewPos(parent.Pos.X+v.Pos.X, parent.Pos.Y+v.Pos.Y),
		Size:           v.Size,
		ScrollAncestor: scrollAnc,
	}
}

// keepInteractiveDescendants filters the pruned children of a node whose
// role reports NonInteractiveChildrenPresentational: nodes that expose
// actions or respond to taps survive; purely textual nodes are dropped,
// hoisting any interactive descendants of their own into their place.
func keepInteractiveDescendants(children []*Node) []*Node {
	var kept []*Node
	for _, c := range children {
		if len(c.Actions) > 0 || isTappable(c.Object) {
			kept = append(kept, c)
			continue
		}
		kept = append(kept, keepInteractiveDescendants(c.Children)...)
	}
	return kept
}

func isTappable(obj core.CanvasObject) bool {
	_, ok := obj.(event.Tappable)
	return ok
}

// CanvasNodes builds the semantic tree for a whole canvas: the content, then
// the canvas-drawn menu (drivers that own one pass it, others pass nil), then
// each overlay in stacking order. This mirrors what the user perceives as the
// window's UI and is the entry point platform backends call.
func CanvasNodes(c core.Canvas, menu core.CanvasObject) []*Node {
	var roots []core.CanvasObject
	if content := c.Content(); content != nil {
		roots = append(roots, content)
	}
	if menu != nil {
		roots = append(roots, menu)
	}
	roots = append(roots, c.Overlays().List()...)
	return BuildTree(roots...)
}

// Flatten returns the nodes of the given forest as a single list in tree
// (reading) order, parents before children.
func Flatten(nodes []*Node) []*Node {
	var flat []*Node
	for _, n := range nodes {
		flat = append(flat, n)
		flat = append(flat, Flatten(n.Children)...)
	}
	return flat
}
