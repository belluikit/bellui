//go:build accessibility

package accessibility

import "github.com/belluikit/bellui/core"

// RealizeCell asks a virtualized container to realize the cell at display
// (row, col) that is not in the current viewport window, returning it as a wire
// Node in absolute canvas coordinates relative to container. It is the neutral
// seam the platform bridges call from their lazy item-container callbacks — the
// UIA ItemContainer pattern's FindItemByProperty, NSAccessibility's
// accessibilityCell(forColumn:row:) — so each bridge shares one definition of
// the [core.AccessibleVirtualNode] → Node conversion instead of reimplementing
// it in cgo.
//
// ok is false when container is nil, its Object does not implement
// [core.AccessibleVirtualCellRealizer], or the coordinates are out of range. The
// (row, col) space is the display coordinate space of
// [core.AccessibleGridDimensioner] / [core.AccessibleGridItem] — filter- and
// sort-aware.
//
// The realized cell carries no clip (HasClip is false): unlike the near-viewport
// window nodes from [core.AccessibleVirtualNodeProvider], it is delivered on
// demand as a transient element outside the reconcile walk and must never be
// culled, even though its bounds are off-screen. Its NodeID is the cell's stable
// data identity, so a bridge reconciles the transient element by record across
// reindexing. Reading has no visible side effect; the explicit bring-into-view
// path is [ScrollCellIntoView]. Call on the main goroutine.
func RealizeCell(container *Node, row, col int) (*Node, bool) {
	if container == nil {
		return nil, false
	}
	realizer, ok := container.Object.(core.AccessibleVirtualCellRealizer)
	if !ok {
		return nil, false
	}
	v, ok := realizer.AccessibleRealizeCell(row, col)
	if !ok {
		return nil, false
	}
	return virtualToNode(container, v), true
}

// ScrollCellIntoView routes an explicit bring-into-view request to a virtualized
// container that implements [core.AccessibleScrollIntoView], scrolling the cell
// at display (row, col) into the viewport so it realizes. It is kept separate
// from [RealizeCell] so that merely announcing an off-screen cell as the AT
// cursor passes over it does not move the viewport — only an explicit
// realize/activate scrolls.
//
// It returns false when container is nil or its Object does not support scrolling
// into view. Out-of-range coordinates are the container's to ignore. Call on the
// main goroutine, like other accessible actions.
func ScrollCellIntoView(container *Node, row, col int) bool {
	if container == nil {
		return false
	}
	scroller, ok := container.Object.(core.AccessibleScrollIntoView)
	if !ok {
		return false
	}
	scroller.AccessibleScrollCellIntoView(row, col)
	return true
}

// FindVirtualCellRealizer returns the first node in the forest, in tree
// (reading) order, whose Object implements [core.AccessibleVirtualCellRealizer],
// or nil when none does. Platform bridges use it to locate the virtualized
// container a realize request targets when they hold only the canvas forest
// rather than the container node itself.
func FindVirtualCellRealizer(nodes []*Node) *Node {
	for _, n := range nodes {
		if _, ok := n.Object.(core.AccessibleVirtualCellRealizer); ok {
			return n
		}
		if found := FindVirtualCellRealizer(n.Children); found != nil {
			return found
		}
	}
	return nil
}
