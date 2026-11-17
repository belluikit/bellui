package dagwidget

import (
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/dag/dag"
)

// Touch drag claims (event.DragClaimer, bellui#1266). A touch drag goes to the
// innermost Draggable under the finger that claims it. Each part of the canvas
// claims a drag, in any direction, exactly when its Dragged would act on it:
// when the capability its gesture needs is on (none are while the canvas is
// disabled, see caps). A part whose gesture is off leaves the drag to the
// canvas, which pans, and a disabled canvas leaves it to the scroll around it,
// as its TryScrolled leaves the wheel. Mouse drags are not offered, so the
// desktop is unchanged.

var (
	_ event.DragClaimer = (*DAGCanvas)(nil)
	_ event.DragClaimer = (*NodeWidget)(nil)
	_ event.DragClaimer = (*nodeConnectorHandle)(nil)
	_ event.DragClaimer = (*PortWidget)(nil)
	_ event.DragClaimer = (*groupTitleBar)(nil)
	_ event.DragClaimer = (*groupConnectorHandle)(nil)
	_ event.DragClaimer = (*HandleWidget)(nil)
	_ event.DragClaimer = (*AnnotationWidget)(nil)
	_ event.DragClaimer = (*paletteItem)(nil)
)

// ClaimsDrag takes every touch drag while the canvas is enabled: a drag that
// starts on empty canvas pans it, in both directions.
func (c *DAGCanvas) ClaimsDrag(geo.Delta) bool {
	return !c.inert()
}

// ClaimsDrag takes a touch drag while nodes can be moved.
func (n *NodeWidget) ClaimsDrag(geo.Delta) bool {
	return n.dagCanvas != nil && n.dagCanvas.caps().MoveResize && n.dagCanvas.Graph.Nodes[n.NodeID] != nil
}

// ClaimsDrag takes a touch drag while link edges can be drawn.
func (h *nodeConnectorHandle) ClaimsDrag(geo.Delta) bool {
	c := h.nw.dagCanvas
	return c != nil && c.caps().LinkEdges
}

// ClaimsDrag takes a touch drag while edges can be connected, from an output
// port or from a connected input port (which reroutes its edge).
func (p *PortWidget) ClaimsDrag(geo.Delta) bool {
	c := p.dagCanvas
	if c == nil || !c.caps().ConnectEdges {
		return false
	}
	return c.dragState == dragConnecting || p.Direction == dag.PortOutput || (p.Direction == dag.PortInput && p.Connected)
}

// ClaimsDrag takes a touch drag while groups can be moved.
func (t *groupTitleBar) ClaimsDrag(geo.Delta) bool {
	c := t.gw.dagCanvas
	return c != nil && c.caps().MoveResize && t.gw.group() != nil
}

// ClaimsDrag takes a touch drag while group edges can be drawn.
func (h *groupConnectorHandle) ClaimsDrag(geo.Delta) bool {
	c := h.gw.dagCanvas
	return c != nil && c.caps().GroupEdges
}

// ClaimsDrag takes a touch drag while its target can be resized.
func (h *HandleWidget) ClaimsDrag(geo.Delta) bool {
	c := h.target.resizeCanvas()
	if c == nil || !c.caps().MoveResize {
		return false
	}
	_, _, ok := h.target.storedGeometry()
	return ok
}

// ClaimsDrag takes a touch drag while annotations can be moved.
func (a *AnnotationWidget) ClaimsDrag(geo.Delta) bool {
	return a.dagCanvas != nil && a.dagCanvas.caps().MoveResize && a.annotation() != nil
}

// ClaimsDrag takes a touch drag while the palette's canvas accepts new nodes,
// the condition its DragEnd drops one under. The palette scrolls vertically
// and its entries take a drag in any direction, so a swipe that starts on an
// entry drags it rather than scrolling the palette; which way a touch drag
// out of a palette should go is an open question on bellui#1266.
func (item *paletteItem) ClaimsDrag(geo.Delta) bool {
	dc := item.palette.DAGCanvas
	return dc != nil && dc.Capabilities.AddNodes
}
