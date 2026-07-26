package dagwidget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

const (
	nodeMinWidth  float32 = 160
	nodeMinHeight float32 = 80

	// Styler-driven presentation, in graph units (scaled by zoom).
	nodeIconSize     float32 = 16
	nodeSubtitleSize float32 = 10
	nodeAccentWidth  float32 = 4
	nodePadding      float32 = 8
	titleHeight      float32 = 28
)

// NodeWidget is a draggable card representing a DAG node.
type NodeWidget struct {
	widget.BaseWidget

	NodeID   string
	Label    string
	Inputs   []*PortWidget
	Outputs  []*PortWidget
	Content  core.CanvasObject
	Selected bool
	// DropHighlight marks this node as the drop target while a link edge drag
	// hovers over it.
	DropHighlight bool

	handles   []*HandleWidget      // resize handles, visible while selected
	connector *nodeConnectorHandle // drag to draw a link edge, in the title strip

	dagCanvas    *DAGCanvas
	dragStartPos [2]float32 // recorded on first Dragged event
	dragging     bool
	taps         tapTracker // manual double-tap detection for activation
}

var (
	_ core.Widget             = (*NodeWidget)(nil)
	_ event.Draggable         = (*NodeWidget)(nil)
	_ event.Tappable          = (*NodeWidget)(nil)
	_ event.SecondaryTappable = (*NodeWidget)(nil)
	_ core.DragImageProvider  = (*NodeWidget)(nil)
)

// DragImage returns nil (the node itself moves live); implementing
// DragImageProvider switches the pointer to the grabbing cursor while the
// node is being dragged.
func (n *NodeWidget) DragImage() core.CanvasObject { return nil }

// NewNodeWidget creates a new node widget from a dag.Node.
func NewNodeWidget(n *dag.Node, dc *DAGCanvas) *NodeWidget {
	nw := &NodeWidget{
		NodeID:    n.ID,
		Label:     n.Label,
		dagCanvas: dc,
	}

	for _, p := range n.Inputs {
		pw := NewPortWidget(
			dag.PortID{NodeID: n.ID, Name: p.Name},
			dag.PortInput, p.TypeTag, dc,
		)
		nw.Inputs = append(nw.Inputs, pw)
	}
	for _, p := range n.Outputs {
		pw := NewPortWidget(
			dag.PortID{NodeID: n.ID, Name: p.Name},
			dag.PortOutput, p.TypeTag, dc,
		)
		nw.Outputs = append(nw.Outputs, pw)
	}

	nw.handles = newHandleSet(nw, nodeHandleDirs)
	nw.connector = &nodeConnectorHandle{nw: nw}
	nw.connector.ExtendBaseWidget(nw.connector)

	nw.ExtendBaseWidget(nw)
	return nw
}

// --- resizable interface ---

func (n *NodeWidget) resizeCanvas() *DAGCanvas { return n.dagCanvas }

func (n *NodeWidget) storedGeometry() (pos, size [2]float32, ok bool) {
	if n.dagCanvas == nil {
		return pos, size, false
	}
	node := n.dagCanvas.Graph.Nodes[n.NodeID]
	if node == nil {
		return pos, size, false
	}
	return node.Position, node.Size, true
}

func (n *NodeWidget) setStoredGeometry(pos, size [2]float32) {
	if node := n.dagCanvas.Graph.Nodes[n.NodeID]; node != nil {
		node.Position = pos
		node.Size = size
	}
}

func (n *NodeWidget) effectiveGraphSize() geo.Size { return n.effectiveSize() }
func (n *NodeWidget) minGraphSize() geo.Size       { return n.MinSize() }

func (n *NodeWidget) pushResizeCommand(oldPos, oldSize, newPos, newSize [2]float32) {
	n.dagCanvas.pushDone(&ResizeNodeCommand{
		Canvas:  n.dagCanvas,
		NodeID:  n.NodeID,
		OldPos:  oldPos,
		NewPos:  newPos,
		OldSize: oldSize,
		NewSize: newSize,
	})
}

var _ resizable = (*NodeWidget)(nil)

// handlesVisible reports whether the resize handles should show: selected,
// and the canvas allows move/resize. Shared by the renderer's Refresh and
// the canvas-side affordance sync (see dagCanvasRenderer.Layout).
func (n *NodeWidget) handlesVisible() bool {
	return n.Selected && (n.dagCanvas == nil || n.dagCanvas.Capabilities.MoveResize)
}

// effectiveSize returns the node's user-set size clamped to MinSize, or
// MinSize when no size is stored (auto-size), in graph units.
func (n *NodeWidget) effectiveSize() geo.Size {
	ms := n.MinSize()
	if n.dagCanvas == nil {
		return ms
	}
	node := n.dagCanvas.Graph.Nodes[n.NodeID]
	if node == nil {
		return ms
	}
	w, h := node.Size[0], node.Size[1]
	if w < ms.Width {
		w = ms.Width
	}
	if h < ms.Height {
		h = ms.Height
	}
	return geo.NewSize(w, h)
}

func (n *NodeWidget) MinSize() geo.Size {
	portCount := len(n.Inputs)
	if len(n.Outputs) > portCount {
		portCount = len(n.Outputs)
	}
	minH := titleHeight + nodePadding*2 + float32(portCount)*(portSize+nodePadding)
	if minH < nodeMinHeight {
		minH = nodeMinHeight
	}
	return geo.NewSize(nodeMinWidth, minH)
}

// portCenterGraph returns a port's center in graph units, mirroring the port
// placement in nodeRenderer.Layout (keep the two in sync). Input ports center
// on the node's left border, outputs on the right border.
func portCenterGraph(node *dag.Node, eff geo.Size, portName string, dir dag.PortDirection) (geo.Position, bool) {
	ports := node.Inputs
	x := node.Position[0]
	if dir == dag.PortOutput {
		ports = node.Outputs
		x = node.Position[0] + eff.Width
	}
	idx := -1
	for i := range ports {
		if ports[i].Name == portName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return geo.Position{}, false
	}
	startY := titleHeight + nodePadding*2
	spacing := (eff.Height - startY - nodePadding) / float32(len(ports))
	y := node.Position[1] + startY + spacing*float32(idx) + spacing/2
	return geo.NewPos(x, y), true
}

// graphCenter returns the node's center in graph coordinates.
func (n *NodeWidget) graphCenter(node *dag.Node) geo.Position {
	eff := n.effectiveSize()
	return geo.NewPos(
		node.Position[0]+eff.Width/2,
		node.Position[1]+eff.Height/2,
	)
}

// Dragged moves the node in graph coordinates and highlights the group under
// its center as the membership drop target.
func (n *NodeWidget) Dragged(ev *event.DragEvent) {
	if n.dagCanvas == nil || !n.dagCanvas.Capabilities.MoveResize {
		return
	}
	c := n.dagCanvas
	node := c.Graph.Nodes[n.NodeID]
	if node == nil {
		return
	}
	// Record start position on first drag event
	if !n.dragging {
		n.dragging = true
		n.dragStartPos = node.Position
	}
	zoom := c.Zoom
	node.Position[0] += ev.Dragged.DX / zoom
	node.Position[1] += ev.Dragged.DY / zoom

	if c.Capabilities.GroupMembership {
		over := c.groupAtGraphPos(n.graphCenter(node))
		assignable := over != nil && c.Graph.CanAssignToGroup(n.NodeID, over.ID)
		for id, gw := range c.groups {
			want := assignable && id == over.ID
			if gw.DropHighlight != want {
				gw.DropHighlight = want
				gw.Refresh()
			}
		}
	}
	c.Refresh()
}

func (n *NodeWidget) DragEnd() {
	if !n.dragging {
		return
	}
	n.dragging = false
	c := n.dagCanvas
	node := c.Graph.Nodes[n.NodeID]
	if node == nil {
		return
	}
	for _, gw := range c.groups {
		if gw.DropHighlight {
			gw.DropHighlight = false
			gw.Refresh()
		}
	}

	eff := n.effectiveSize()
	oldGroup := c.groupOfNode(n.NodeID)
	newGroup := oldGroup
	var resizeCmd *ResizeGroupCommand
	if c.Capabilities.GroupMembership {
		newGroup = ""
		if g := c.groupAtGraphPos(n.graphCenter(node)); g != nil && c.Graph.CanAssignToGroup(n.NodeID, g.ID) {
			newGroup = g.ID
		}

		// Resolve boundary ambiguity at drop time: a member must sit fully
		// inside its group (the group grows around it), a non-member fully
		// outside (the node is nudged clear of any group border it
		// straddles). Both are membership semantics, so gated with it.
		if newGroup != "" {
			grp := c.Graph.Groups[newGroup]
			if pos, size, changed := c.groupGrowthToFit(grp, node, eff); changed {
				resizeCmd = &ResizeGroupCommand{
					Canvas:  c,
					GroupID: newGroup,
					OldPos:  grp.Position,
					NewPos:  pos,
					OldSize: grp.Size,
					NewSize: size,
				}
			}
		} else {
			// Applied live; the nudged position lands in the move command below.
			c.nudgeNodeOutOfGroups(node, eff)
		}
	}

	moved := node.Position != n.dragStartPos
	if !moved && newGroup == oldGroup && resizeCmd == nil {
		return
	}

	var cmds []Command
	var moveCmd *MoveNodeCommand
	if moved {
		moveCmd = &MoveNodeCommand{
			Canvas: c,
			NodeID: n.NodeID,
			OldPos: n.dragStartPos,
			NewPos: node.Position,
		}
		cmds = append(cmds, moveCmd)
	}
	if newGroup != oldGroup {
		cmds = append(cmds, &AssignNodeToGroupCommand{
			Canvas:     c,
			NodeID:     n.NodeID,
			OldGroupID: oldGroup,
			NewGroupID: newGroup,
		})
	}
	if resizeCmd != nil {
		cmds = append(cmds, resizeCmd)
	}
	if len(cmds) == 0 {
		return
	}

	if newGroup != oldGroup || resizeCmd != nil {
		// Run through Do so membership and group growth apply; the move's
		// Execute just re-sets the position already in place (idempotent).
		// Everything lands as one undo step.
		var cmd Command = &CompoundCommand{Commands: cmds}
		if len(cmds) == 1 {
			cmd = cmds[0]
		}
		c.do(cmd)
		return
	}

	// Push without executing — the position is already at NewPos
	c.pushDone(moveCmd)
}

// Tapped selects this node; a second tap within the double-tap window also
// activates it (see tapTracker for why this is not event.DoubleTappable).
func (n *NodeWidget) Tapped(ev *event.PointEvent) {
	if n.dagCanvas == nil {
		n.Selected = true
		n.Refresh()
		return
	}
	n.dagCanvas.requestFocus() // so Enter/Delete act on this selection
	n.dagCanvas.SelectNode(n.NodeID)
	if n.taps.doubleTap(ev) {
		if f := n.dagCanvas.OnNodeActivated; f != nil {
			f(n.NodeID)
		}
	}
}

// TappedSecondary selects this node like a primary tap — a context menu acts
// on the element under the pointer — then fires OnNodeSecondaryTapped. It
// never activates.
func (n *NodeWidget) TappedSecondary(ev *event.PointEvent) {
	if n.dagCanvas == nil {
		n.Selected = true
		n.Refresh()
		return
	}
	n.dagCanvas.requestFocus() // so Enter/Delete act on this selection
	n.dagCanvas.SelectNode(n.NodeID)
	if f := n.dagCanvas.OnNodeSecondaryTapped; f != nil {
		f(n.NodeID, ev)
	}
}

func (n *NodeWidget) CreateRenderer() core.WidgetRenderer {
	r := newNodeRenderer(n)
	return r
}

// zoom returns the current zoom factor from the parent canvas, or 1.0.
func (n *NodeWidget) zoom() float32 {
	if n.dagCanvas != nil {
		return n.dagCanvas.Zoom
	}
	return 1.0
}

// --- connector handle: drag to draw a link edge to another node ---

// nodeConnectorHandle is the small affordance in a node's title strip from
// which a link edge drag starts, modeled on groupConnectorHandle. Link edges
// have no ports, so without this the pipeline editor could not author one by
// hand.
type nodeConnectorHandle struct {
	widget.BaseWidget

	nw *NodeWidget
}

var (
	_ core.Widget            = (*nodeConnectorHandle)(nil)
	_ event.Draggable        = (*nodeConnectorHandle)(nil)
	_ desktop.Cursorable     = (*nodeConnectorHandle)(nil)
	_ core.DragImageProvider = (*nodeConnectorHandle)(nil)
)

// Cursor shows a crosshair over the connector handle to hint that a link
// edge can be drawn from here.
func (h *nodeConnectorHandle) Cursor() desktop.Cursor {
	return desktop.CrosshairCursor
}

// DragImage returns nil (the canvas draws the ghost line); implementing
// DragImageProvider gives the grabbing cursor during the drag.
func (h *nodeConnectorHandle) DragImage() core.CanvasObject { return nil }

func (h *nodeConnectorHandle) Dragged(ev *event.DragEvent) {
	c := h.nw.dagCanvas
	if c == nil || !c.Capabilities.LinkEdges {
		return
	}
	if c.dragState != dragConnectingLink {
		c.startLinkEdgeConnection(h.nw.NodeID, h.screenCenter())
	}
	if c.dragState == dragConnectingLink {
		c.updatePendingLinkEdge(ev.AbsolutePosition)
	}
}

func (h *nodeConnectorHandle) DragEnd() {
	c := h.nw.dagCanvas
	if c != nil && c.dragState == dragConnectingLink {
		c.finishLinkEdgeConnection()
	}
}

// screenCenter returns the handle's center in screen coordinates relative to
// the DAGCanvas.
func (h *nodeConnectorHandle) screenCenter() geo.Position {
	driver := fyneDriver()
	absSelf := driver.AbsolutePositionForObject(h)
	absCanvas := driver.AbsolutePositionForObject(h.nw.dagCanvas)
	return geo.NewPos(
		absSelf.X-absCanvas.X+h.Size().Width/2,
		absSelf.Y-absCanvas.Y+h.Size().Height/2,
	)
}

func (h *nodeConnectorHandle) CreateRenderer() core.WidgetRenderer {
	return newNodeConnectorRenderer(h)
}
