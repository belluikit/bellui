package dagwidget

import (
	"fmt"
	"image/color"
	"math"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

type canvasDragState int

const (
	dragNone            canvasDragState = iota
	dragPanning                         // middle-click or drag on empty space
	dragSelecting                       // lasso selection
	dragConnecting                      // dragging from a port to create an edge
	dragConnectingGroup                 // dragging from a group's connector handle
	dragConnectingLink                  // dragging from a node's connector handle
)

// pendingConnection tracks an in-progress connection drag.
type pendingConnection struct {
	sourcePort dag.PortID
	sourcePos  geo.Position // screen coords relative to DAGCanvas
	currentPos geo.Position // current cursor, screen coords
	snapTarget *PortWidget  // non-nil if snapped to a valid port

	// detached is the already-executed removal of the edge a detach drag
	// picked up. Nothing is recorded until the drop, which pushes the net
	// effect as one undo step: the removal alone (empty drop), a compound
	// removal+add (rerouted), or nothing (dropped back on the same port).
	detached *RemoveEdgeCommand
}

// pendingGroupEdge tracks an in-progress group dependency drag.
type pendingGroupEdge struct {
	sourceGroup string
	sourcePos   geo.Position // screen coords relative to DAGCanvas
	currentPos  geo.Position // current cursor, screen coords
	snapTarget  string       // group ID under the cursor if a valid drop, else ""
}

// pendingNodeLink tracks an in-progress link edge drag from a node's
// connector handle.
type pendingNodeLink struct {
	sourceNode string
	sourcePos  geo.Position // screen coords relative to DAGCanvas
	currentPos geo.Position // current cursor, screen coords
	snapTarget string       // node ID under the cursor if a valid drop, else ""
}

// DAGCanvas is the main editing surface for a DAG.
type DAGCanvas struct {
	widget.BaseWidget

	Graph *dag.Graph

	// View transform
	Offset geo.Position // pan offset in graph coordinates
	Zoom   float32      // zoom factor (1.0 = 100%)

	// ShowStreamTypes draws each edge's source stream type as a label at the
	// curve midpoint (edges with an explicit Label always show it). Call
	// Refresh after changing.
	ShowStreamTypes bool

	// Capabilities gates user interactions; see the type's doc. Defaults to
	// AllCapabilities. Call Refresh after changing so affordances (resize
	// handles, connector handles) update.
	Capabilities Capabilities

	// NodeStyler derives per-node presentation (icon, subtitle, accent);
	// nil renders plain cards. Call RefreshNodeStyles after changing.
	NodeStyler NodeStyler

	// Child widgets
	nodes       map[string]*NodeWidget
	edges       map[string]*EdgeWidget
	groups      map[string]*GroupWidget
	groupEdges  map[string]*GroupEdgeWidget
	linkEdges   map[string]*LinkEdgeWidget
	annotations map[string]*AnnotationWidget

	// Interaction state
	dragState        canvasDragState
	dragStart        geo.Position
	pendingEdge      *pendingConnection
	pendingGroupLink *pendingGroupEdge
	pendingNodeLink  *pendingNodeLink
	pinching         bool         // a pinch is running; pinchFocus is its last focus
	pinchFocus       geo.Position // canvas position between the fingers at the last Pinched

	// Undo/redo. NewDAGCanvas installs a stack whose OnChange is wired to
	// OnGraphChanged; a consumer replacing the stack should re-wire OnChange
	// (or set it to its own hook) to keep change notifications flowing for
	// command-driven mutations, undo, and redo.
	Commands *CommandStack

	// Callbacks
	OnNodeSelected       func(nodeID string)
	OnEdgeSelected       func(edgeID string)
	OnGroupSelected      func(groupID string)
	OnLinkEdgeSelected   func(linkEdgeID string)
	OnAnnotationSelected func(annotationID string)
	// OnNodeActivated / OnGroupActivated fire on double-tap of the element
	// and on Enter while it is the selection (e.g. open a node's source or a
	// group's blueprint). Activation always selects first, so the matching
	// On*Selected callback fires before these. Nil callbacks change nothing.
	OnNodeActivated  func(nodeID string)
	OnGroupActivated func(groupID string)
	// On*SecondaryTapped fire on right-click / long-tap of the element (e.g.
	// to anchor a context menu at ev.AbsolutePosition). A secondary tap
	// selects its target first — a context menu acts on the element under the
	// pointer — so the matching On*Selected callback fires before these. Nil
	// callbacks change nothing: the tap still selects. Secondary taps on
	// empty canvas are ignored (selection and focus stay untouched). The
	// Menu key and Shift+F10 also fire these for the current selection
	// (contextMenuSelection), without re-firing On*Selected.
	OnNodeSecondaryTapped       func(nodeID string, ev *event.PointEvent)
	OnEdgeSecondaryTapped       func(edgeID string, ev *event.PointEvent)
	OnGroupSecondaryTapped      func(groupID string, ev *event.PointEvent)
	OnLinkEdgeSecondaryTapped   func(linkEdgeID string, ev *event.PointEvent)
	OnAnnotationSecondaryTapped func(annotationID string, ev *event.PointEvent)
	// OnSelectionCleared fires when a click on empty space or a deletion
	// leaves nothing selected.
	OnSelectionCleared func()
	OnGraphChanged     func()

	focused    bool
	nextEdgeID int
}

var (
	_ core.Widget             = (*DAGCanvas)(nil)
	_ event.Draggable         = (*DAGCanvas)(nil)
	_ event.Tappable          = (*DAGCanvas)(nil)
	_ event.SecondaryTappable = (*DAGCanvas)(nil)
	_ event.Focusable         = (*DAGCanvas)(nil)
	_ event.Scrollable        = (*DAGCanvas)(nil)
	_ event.Pinchable         = (*DAGCanvas)(nil)
	_ bellui.Shortcutable     = (*DAGCanvas)(nil)
)

// NewDAGCanvas creates a new DAG editing canvas.
func NewDAGCanvas(g *dag.Graph) *DAGCanvas {
	c := &DAGCanvas{
		Graph:        g,
		Zoom:         1.0,
		Capabilities: AllCapabilities(),
		nodes:        make(map[string]*NodeWidget),
		edges:        make(map[string]*EdgeWidget),
		groups:       make(map[string]*GroupWidget),
		groupEdges:   make(map[string]*GroupEdgeWidget),
		linkEdges:    make(map[string]*LinkEdgeWidget),
		annotations:  make(map[string]*AnnotationWidget),
	}
	c.ExtendBaseWidget(c)
	c.Commands = NewCommandStack()
	c.Commands.OnChange = c.notifyGraphChanged
	c.syncFromGraph()
	return c
}

// do routes a gesture-created command through the command stack. Without a
// stack it executes directly, mirroring the stack's failure handling and
// change notification.
func (c *DAGCanvas) do(cmd Command) {
	if c.Commands != nil {
		c.Commands.Do(cmd)
		return
	}
	cmd.Execute()
	if failed(cmd) {
		return
	}
	c.notifyGraphChanged()
}

// pushDone records an already-executed gesture command. Without a stack the
// mutation still happened, so the change notification still fires.
func (c *DAGCanvas) pushDone(cmd Command) {
	if c.Commands != nil {
		c.Commands.PushDone(cmd)
		return
	}
	c.notifyGraphChanged()
}

// syncFromGraph creates widgets for all graph elements that lack one.
func (c *DAGCanvas) syncFromGraph() {
	for _, n := range c.Graph.Nodes {
		if _, exists := c.nodes[n.ID]; !exists {
			c.nodes[n.ID] = NewNodeWidget(n, c)
		}
	}
	for _, e := range c.Graph.Edges {
		if _, exists := c.edges[e.ID]; !exists {
			c.edges[e.ID] = NewEdgeWidget(e.ID, e.Source, e.Target, c)
		}
	}
	for _, g := range c.Graph.Groups {
		if _, exists := c.groups[g.ID]; !exists {
			c.groups[g.ID] = NewGroupWidget(g, c)
		}
	}
	for _, ge := range c.Graph.GroupEdges {
		if _, exists := c.groupEdges[ge.ID]; !exists {
			c.groupEdges[ge.ID] = NewGroupEdgeWidget(ge.ID, ge.Source, ge.Target, c)
		}
	}
	for _, le := range c.Graph.LinkEdges {
		if _, exists := c.linkEdges[le.ID]; !exists {
			c.linkEdges[le.ID] = NewLinkEdgeWidget(le.ID, le.Source, le.Target, c)
		}
	}
	for _, a := range c.Graph.Annotations {
		if _, exists := c.annotations[a.ID]; !exists {
			c.annotations[a.ID] = NewAnnotationWidget(a, c)
		}
	}
	c.syncPortConnections()
}

// syncPortConnections derives every port's Connected flag from the graph's
// edges, refreshing only ports whose state changed. All edge mutations funnel
// through here so the visual state cannot drift from the model.
func (c *DAGCanvas) syncPortConnections() {
	connected := make(map[dag.PortID]bool, len(c.Graph.Edges)*2)
	for _, e := range c.Graph.Edges {
		connected[e.Source] = true
		connected[e.Target] = true
	}
	for _, nw := range c.nodes {
		for _, ports := range [2][]*PortWidget{nw.Inputs, nw.Outputs} {
			for _, pw := range ports {
				if want := connected[pw.PortID]; pw.Connected != want {
					pw.Connected = want
					pw.Refresh()
				}
			}
		}
	}
}

// reconcileWidgets drops widgets whose model objects no longer exist —
// model-level removals cascade (group edges, annotations, membership), so a
// single Remove call can invalidate widgets of other kinds.
func (c *DAGCanvas) reconcileWidgets() {
	for id := range c.edges {
		if _, ok := c.Graph.Edges[id]; !ok {
			delete(c.edges, id)
		}
	}
	for id := range c.groups {
		if _, ok := c.Graph.Groups[id]; !ok {
			delete(c.groups, id)
		}
	}
	for id := range c.groupEdges {
		if _, ok := c.Graph.GroupEdges[id]; !ok {
			delete(c.groupEdges, id)
		}
	}
	for id := range c.linkEdges {
		if _, ok := c.Graph.LinkEdges[id]; !ok {
			delete(c.linkEdges, id)
		}
	}
	for id := range c.annotations {
		if _, ok := c.Graph.Annotations[id]; !ok {
			delete(c.annotations, id)
		}
	}
}

// LoadGraph replaces the current graph and rebuilds all widgets.
func (c *DAGCanvas) LoadGraph(g *dag.Graph) {
	c.Graph = g
	c.nodes = make(map[string]*NodeWidget)
	c.edges = make(map[string]*EdgeWidget)
	c.groups = make(map[string]*GroupWidget)
	c.groupEdges = make(map[string]*GroupEdgeWidget)
	c.linkEdges = make(map[string]*LinkEdgeWidget)
	c.annotations = make(map[string]*AnnotationWidget)
	c.pendingEdge = nil
	c.pendingGroupLink = nil
	c.pendingNodeLink = nil
	c.dragState = dragNone
	c.syncFromGraph()
	c.Refresh()
}

// notifyGraphChanged fires the OnGraphChanged callback. Exactly-once
// semantics: the exported CRUD methods notify directly (command-less
// programmatic use); everything that goes through the command stack is
// notified by its OnChange hook instead, so commands and gesture endpoints
// must use the silent unexported variants and never notify themselves.
func (c *DAGCanvas) notifyGraphChanged() {
	if f := c.OnGraphChanged; f != nil {
		f()
	}
}

// AddNode adds a node to the graph and creates its widget.
func (c *DAGCanvas) AddNode(n *dag.Node) error {
	if err := c.addNode(n); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) addNode(n *dag.Node) error {
	if err := c.Graph.AddNode(n); err != nil {
		return err
	}
	nw := NewNodeWidget(n, c)
	c.nodes[n.ID] = nw
	c.Refresh()
	return nil
}

// RemoveNode removes a node and its connected edges.
func (c *DAGCanvas) RemoveNode(id string) error {
	if err := c.removeNode(id); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) removeNode(id string) error {
	removed, err := c.Graph.RemoveNode(id)
	if err != nil {
		return err
	}
	delete(c.nodes, id)
	for _, e := range removed {
		delete(c.edges, e.ID)
	}
	c.reconcileWidgets() // anchored annotations cascade at the model level
	c.syncPortConnections()
	c.Refresh()
	return nil
}

// AddGroup adds a group to the graph and creates its widget.
func (c *DAGCanvas) AddGroup(g *dag.Group) error {
	if err := c.addGroup(g); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) addGroup(g *dag.Group) error {
	if err := c.Graph.AddGroup(g); err != nil {
		return err
	}
	c.groups[g.ID] = NewGroupWidget(g, c)
	c.Refresh()
	return nil
}

// RemoveGroup removes a group; connected group edges and anchored
// annotations cascade at the model level.
func (c *DAGCanvas) RemoveGroup(id string) error {
	if err := c.removeGroup(id); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) removeGroup(id string) error {
	if _, _, err := c.Graph.RemoveGroup(id); err != nil {
		return err
	}
	delete(c.groups, id)
	c.reconcileWidgets()
	c.Refresh()
	return nil
}

// AddGroupEdge adds a group dependency edge and creates its widget.
func (c *DAGCanvas) AddGroupEdge(ge *dag.GroupEdge) error {
	if err := c.addGroupEdge(ge); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) addGroupEdge(ge *dag.GroupEdge) error {
	if err := c.Graph.AddGroupEdge(ge); err != nil {
		return err
	}
	c.groupEdges[ge.ID] = NewGroupEdgeWidget(ge.ID, ge.Source, ge.Target, c)
	c.Refresh()
	return nil
}

// RemoveGroupEdge removes a group dependency edge.
func (c *DAGCanvas) RemoveGroupEdge(id string) error {
	if err := c.removeGroupEdge(id); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) removeGroupEdge(id string) error {
	if err := c.Graph.RemoveGroupEdge(id); err != nil {
		return err
	}
	delete(c.groupEdges, id)
	c.Refresh()
	return nil
}

// AddLinkEdge adds a link edge and creates its widget.
func (c *DAGCanvas) AddLinkEdge(le *dag.LinkEdge) error {
	if err := c.addLinkEdge(le); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) addLinkEdge(le *dag.LinkEdge) error {
	if err := c.Graph.AddLinkEdge(le); err != nil {
		return err
	}
	c.linkEdges[le.ID] = NewLinkEdgeWidget(le.ID, le.Source, le.Target, c)
	c.Refresh()
	return nil
}

// RemoveLinkEdge removes a link edge; anchored annotations cascade at the
// model level.
func (c *DAGCanvas) RemoveLinkEdge(id string) error {
	if err := c.removeLinkEdge(id); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) removeLinkEdge(id string) error {
	if err := c.Graph.RemoveLinkEdge(id); err != nil {
		return err
	}
	delete(c.linkEdges, id)
	c.reconcileWidgets()
	c.Refresh()
	return nil
}

// AddAnnotation adds an annotation and creates its widget.
func (c *DAGCanvas) AddAnnotation(a *dag.Annotation) error {
	if err := c.addAnnotation(a); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) addAnnotation(a *dag.Annotation) error {
	if err := c.Graph.AddAnnotation(a); err != nil {
		return err
	}
	c.annotations[a.ID] = NewAnnotationWidget(a, c)
	c.Refresh()
	return nil
}

// RemoveAnnotation removes an annotation.
func (c *DAGCanvas) RemoveAnnotation(id string) error {
	if err := c.removeAnnotation(id); err != nil {
		return err
	}
	c.notifyGraphChanged()
	return nil
}

func (c *DAGCanvas) removeAnnotation(id string) error {
	if err := c.Graph.RemoveAnnotation(id); err != nil {
		return err
	}
	delete(c.annotations, id)
	c.Refresh()
	return nil
}

// portWidget returns the PortWidget for a given port ID.
func (c *DAGCanvas) portWidget(pid dag.PortID) *PortWidget {
	nw, ok := c.nodes[pid.NodeID]
	if !ok {
		return nil
	}
	for _, pw := range nw.Inputs {
		if pw.PortID == pid {
			return pw
		}
	}
	for _, pw := range nw.Outputs {
		if pw.PortID == pid {
			return pw
		}
	}
	return nil
}

// graphToScreen converts graph coordinates to screen (widget-local) coordinates.
func (c *DAGCanvas) graphToScreen(p geo.Position) geo.Position {
	return geo.NewPos(
		(p.X-c.Offset.X)*c.Zoom,
		(p.Y-c.Offset.Y)*c.Zoom,
	)
}

// screenToGraph converts screen coordinates to graph coordinates.
func (c *DAGCanvas) screenToGraph(p geo.Position) geo.Position {
	return geo.NewPos(
		p.X/c.Zoom+c.Offset.X,
		p.Y/c.Zoom+c.Offset.Y,
	)
}

func (c *DAGCanvas) generateEdgeID() string {
	c.nextEdgeID++
	return fmt.Sprintf("e_%d", c.nextEdgeID)
}

// uniqueID returns prefix_N for the smallest N that exists() rejects. Unlike
// the plain counter above it stays collision-free after loading a graph that
// already contains generated IDs.
func uniqueID(prefix string, exists func(string) bool) string {
	for i := 1; ; i++ {
		id := fmt.Sprintf("%s_%d", prefix, i)
		if !exists(id) {
			return id
		}
	}
}

func (c *DAGCanvas) generateGroupID() string {
	return uniqueID("g", func(id string) bool {
		_, ok := c.Graph.Groups[id]
		return ok
	})
}

func (c *DAGCanvas) generateGroupEdgeID() string {
	return uniqueID("ge", func(id string) bool {
		_, ok := c.Graph.GroupEdges[id]
		return ok
	})
}

func (c *DAGCanvas) generateLinkEdgeID() string {
	return uniqueID("le", func(id string) bool {
		_, ok := c.Graph.LinkEdges[id]
		return ok
	})
}

func (c *DAGCanvas) generateAnnotationID() string {
	return uniqueID("an", func(id string) bool {
		_, ok := c.Graph.Annotations[id]
		return ok
	})
}

// --- group/annotation geometry helpers ---

// groupGraphRect returns a group's on-canvas rectangle in graph units.
func groupGraphRect(g *dag.Group) geo.Rectangle {
	w, h := groupCollapsedW, groupCollapsedH
	if !g.Collapsed {
		w = max32(g.Size[0], groupMinWidth)
		h = max32(g.Size[1], groupMinHeight)
	}
	return geo.NewRectangle(g.Position[0], g.Position[1], w, h)
}

// groupScreenRect returns a group's rectangle in screen coordinates.
func (c *DAGCanvas) groupScreenRect(g *dag.Group) geo.Rectangle {
	r := groupGraphRect(g)
	pos := c.graphToScreen(r.Pos)
	return geo.Rectangle{
		Pos:  pos,
		Size: geo.NewSize(r.Size.Width*c.Zoom, r.Size.Height*c.Zoom),
	}
}

func rectContains(r geo.Rectangle, p geo.Position) bool {
	return p.X >= r.Pos.X && p.X <= r.Pos.X+r.Size.Width &&
		p.Y >= r.Pos.Y && p.Y <= r.Pos.Y+r.Size.Height
}

// groupAtGraphPos returns the expanded group whose rectangle contains the
// given graph position, or nil. Collapsed groups are skipped — their compact
// card is not the true region, so they don't accept node drops. Overlapping
// groups resolve to the smallest area so inner targets win.
func (c *DAGCanvas) groupAtGraphPos(p geo.Position) *dag.Group {
	var best *dag.Group
	var bestArea float32
	for _, g := range c.Graph.Groups {
		if g.Collapsed {
			continue
		}
		r := groupGraphRect(g)
		if !rectContains(r, p) {
			continue
		}
		area := r.Size.Width * r.Size.Height
		if best == nil || area < bestArea {
			best = g
			bestArea = area
		}
	}
	return best
}

// groupAtScreenPos returns the group (collapsed included) whose screen
// rectangle contains the position, smallest area first. Used for tap
// selection fallback.
func (c *DAGCanvas) groupAtScreenPos(p geo.Position) *dag.Group {
	var best *dag.Group
	var bestArea float32
	for _, g := range c.Graph.Groups {
		r := c.groupScreenRect(g)
		if !rectContains(r, p) {
			continue
		}
		area := r.Size.Width * r.Size.Height
		if best == nil || area < bestArea {
			best = g
			bestArea = area
		}
	}
	return best
}

// groupOfNode returns the ID of the group containing the node, or "".
func (c *DAGCanvas) groupOfNode(nodeID string) string {
	return c.Graph.GroupOf(nodeID)
}

// groupGrowthToFit returns the geometry the group needs to fully contain the
// dropped node AND the connection curves running between members, each with
// padding (the title bar counts as reserved space at the top). Curve extents
// come from the bezier control polygon computed in graph units — the same
// math the edge renderer uses at zoom 1, so backward connections that route
// around nodes are contained too. changed is false when everything already
// fits.
func (c *DAGCanvas) groupGrowthToFit(grp *dag.Group, node *dag.Node, nodeSize geo.Size) (pos, size [2]float32, changed bool) {
	r := groupGraphRect(grp)
	left := min32(r.Pos.X, node.Position[0]-groupContentPadding)
	top := min32(r.Pos.Y, node.Position[1]-groupTitleHeight-groupContentPadding)
	right := max32(r.Pos.X+r.Size.Width, node.Position[0]+nodeSize.Width+groupContentPadding)
	bottom := max32(r.Pos.Y+r.Size.Height, node.Position[1]+nodeSize.Height+groupContentPadding)

	members := make(map[string]bool, len(grp.Members)+1)
	for _, m := range grp.Members {
		members[m] = true
	}
	members[node.ID] = true // about to join (or stay); its edges count

	for _, e := range c.Graph.Edges {
		if !members[e.Source.NodeID] || !members[e.Target.NodeID] {
			continue // crossing edges may overflow by design
		}
		srcNW, tgtNW := c.nodes[e.Source.NodeID], c.nodes[e.Target.NodeID]
		srcNode, tgtNode := c.Graph.Nodes[e.Source.NodeID], c.Graph.Nodes[e.Target.NodeID]
		if srcNW == nil || tgtNW == nil || srcNode == nil || tgtNode == nil {
			continue
		}
		p0, ok0 := portCenterGraph(srcNode, srcNW.effectiveSize(), e.Source.Name, dag.PortOutput)
		p3, ok1 := portCenterGraph(tgtNode, tgtNW.effectiveSize(), e.Target.Name, dag.PortInput)
		if !ok0 || !ok1 {
			continue
		}
		cp1, cp2 := ComputeControlPoints(p0, p3)
		bmin, bmax := BezierTightBounds(p0, cp1, cp2, p3)
		left = min32(left, bmin.X-groupContentPadding)
		top = min32(top, bmin.Y-groupTitleHeight-groupContentPadding)
		right = max32(right, bmax.X+groupContentPadding)
		bottom = max32(bottom, bmax.Y+groupContentPadding)
	}

	pos = [2]float32{left, top}
	size = [2]float32{right - left, bottom - top}
	changed = pos != [2]float32{r.Pos.X, r.Pos.Y} ||
		size != [2]float32{r.Size.Width, r.Size.Height}
	return pos, size, changed
}

// nudgeNodeOutOfGroups pushes the node clear of any expanded group whose
// rectangle it overlaps, along the axis of least displacement. Called when a
// drag ends with the node ungrouped, so a non-member never sits half over a
// group border (the mirror of groupGrowthToFit for members). Single pass —
// pathological overlapping-group layouts may still leave residual overlap.
func (c *DAGCanvas) nudgeNodeOutOfGroups(node *dag.Node, nodeSize geo.Size) bool {
	moved := false
	for _, g := range c.Graph.Groups {
		if g.Collapsed {
			continue
		}
		r := groupGraphRect(g)
		nodeLeft, nodeTop := node.Position[0], node.Position[1]
		nodeRight, nodeBottom := nodeLeft+nodeSize.Width, nodeTop+nodeSize.Height

		overlapX := min32(nodeRight, r.Pos.X+r.Size.Width) - max32(nodeLeft, r.Pos.X)
		overlapY := min32(nodeBottom, r.Pos.Y+r.Size.Height) - max32(nodeTop, r.Pos.Y)
		if overlapX <= 0 || overlapY <= 0 {
			continue
		}
		grpCenter := rectCenter(r)
		nodeCenterX := nodeLeft + nodeSize.Width/2
		nodeCenterY := nodeTop + nodeSize.Height/2
		if overlapX <= overlapY {
			if nodeCenterX >= grpCenter.X {
				node.Position[0] += overlapX + nodeClearGap
			} else {
				node.Position[0] -= overlapX + nodeClearGap
			}
		} else {
			if nodeCenterY >= grpCenter.Y {
				node.Position[1] += overlapY + nodeClearGap
			} else {
				node.Position[1] -= overlapY + nodeClearGap
			}
		}
		moved = true
	}
	return moved
}

// collapsedGroupOf returns the collapsed group hiding the given node, or nil.
func (c *DAGCanvas) collapsedGroupOf(nodeID string) *dag.Group {
	gid := c.Graph.GroupOf(nodeID)
	if gid == "" {
		return nil
	}
	g := c.Graph.Groups[gid]
	if g == nil || !g.Collapsed {
		return nil
	}
	return g
}

// portCenterScreen returns a port's center in screen coordinates, computed
// from model geometry (portCenterGraph + graphToScreen) rather than from the
// port widget's painted position. Widget positions resolve through
// driver.AbsolutePositionForObject, which is only valid once the widget has
// been laid out AND painted — on the first frame (or right after a culled
// node is shown again) it reports stale data, which used to freeze garbage
// edge curves until the next redraw. Model geometry is always current.
func (c *DAGCanvas) portCenterScreen(pid dag.PortID) (geo.Position, bool) {
	nw := c.nodes[pid.NodeID]
	node := c.Graph.Nodes[pid.NodeID]
	if nw == nil || node == nil {
		return geo.Position{}, false
	}
	eff := nw.effectiveSize()
	// Inputs first, then outputs — same resolution order as portWidget.
	if p, ok := portCenterGraph(node, eff, pid.Name, dag.PortInput); ok {
		return c.graphToScreen(p), true
	}
	if p, ok := portCenterGraph(node, eff, pid.Name, dag.PortOutput); ok {
		return c.graphToScreen(p), true
	}
	return geo.Position{}, false
}

// visibleAnchorForPort returns the screen point where an edge endpoint for
// the given port should attach: the port's center normally, or the border of
// the collapsed group card hiding the port's node (aimed toward the other
// endpoint). ok is false when the port cannot be resolved.
func (c *DAGCanvas) visibleAnchorForPort(pid dag.PortID, toward geo.Position) (geo.Position, bool) {
	if g := c.collapsedGroupOf(pid.NodeID); g != nil {
		return rectBorderPoint(c.groupScreenRect(g), toward), true
	}
	return c.portCenterScreen(pid)
}

// nodeVisibleScreenRect returns the screen rectangle where a node reads on
// screen: its own card rect normally, or the collapsed group card hiding it.
// ok is false when the node cannot be resolved.
func (c *DAGCanvas) nodeVisibleScreenRect(nodeID string) (geo.Rectangle, bool) {
	if g := c.collapsedGroupOf(nodeID); g != nil {
		return c.groupScreenRect(g), true
	}
	nw := c.nodes[nodeID]
	node := c.Graph.Nodes[nodeID]
	if nw == nil || node == nil {
		return geo.Rectangle{}, false
	}
	pos := c.graphToScreen(geo.NewPos(node.Position[0], node.Position[1]))
	eff := nw.effectiveSize()
	return geo.Rectangle{Pos: pos, Size: geo.NewSize(eff.Width*c.Zoom, eff.Height*c.Zoom)}, true
}

// anchorScreenPoint returns the screen point an annotation leader line should
// attach to for the given anchor, aimed toward the annotation box. ok is
// false when the anchor cannot be resolved (hidden edge, missing element).
func (c *DAGCanvas) anchorScreenPoint(kind dag.AnchorKind, anchorID string, toward geo.Position) (geo.Position, bool) {
	switch kind {
	case dag.AnchorNode:
		rect, ok := c.nodeVisibleScreenRect(anchorID)
		if !ok {
			return geo.Position{}, false
		}
		return rectBorderPoint(rect, toward), true
	case dag.AnchorEdge:
		ew := c.edges[anchorID]
		if ew == nil || !ew.Visible() {
			return geo.Position{}, false
		}
		// Cubic bezier midpoint at t=0.5.
		return geo.NewPos(
			(ew.p0.X+3*ew.p1.X+3*ew.p2.X+ew.p3.X)/8,
			(ew.p0.Y+3*ew.p1.Y+3*ew.p2.Y+ew.p3.Y)/8,
		), true
	case dag.AnchorGroup:
		g := c.Graph.Groups[anchorID]
		if g == nil {
			return geo.Position{}, false
		}
		return rectBorderPoint(c.groupScreenRect(g), toward), true
	case dag.AnchorLinkEdge:
		lw := c.linkEdges[anchorID]
		if lw == nil || !lw.Visible() {
			return geo.Position{}, false
		}
		return geo.NewPos((lw.p0.X+lw.p1.X)/2, (lw.p0.Y+lw.p1.Y)/2), true
	}
	return geo.Position{}, false
}

// groupEdgeAtPosition returns the GroupEdgeWidget within hit distance of the
// given screen position, or nil.
func (c *DAGCanvas) groupEdgeAtPosition(screenPos geo.Position) *GroupEdgeWidget {
	threshold := float32(8)
	for _, ew := range c.groupEdges {
		if ew.Visible() && hitTestSegment(ew.p0, ew.p1, screenPos, threshold) {
			return ew
		}
	}
	return nil
}

// linkEdgeAtPosition returns the LinkEdgeWidget nearest the given screen
// position within hit distance, or nil. Nearest (not first hit) so that
// laned edges of the same node pair, which run a few pixels apart, resolve
// to the one actually under the cursor.
func (c *DAGCanvas) linkEdgeAtPosition(screenPos geo.Position) *LinkEdgeWidget {
	bestDist := float32(8) // hit threshold
	var best *LinkEdgeWidget
	for _, ew := range c.linkEdges {
		if !ew.Visible() {
			continue
		}
		if d := pointToSegmentDist(screenPos, ew.p0, ew.p1); d < bestDist {
			best, bestDist = ew, d
		}
	}
	return best
}

// detachAndReroute removes the edge connected to a target input port,
// then starts a connection drag from the original source so the user can
// reconnect it to a different input.
func (c *DAGCanvas) detachAndReroute(inputPort dag.PortID) {
	if !c.Capabilities.ConnectEdges {
		return
	}
	// Find the edge connected to this input
	var found *dag.Edge
	for _, e := range c.Graph.Edges {
		if e.Target == inputPort {
			found = e
			break
		}
	}
	if found == nil {
		return
	}

	sourcePort := found.Source

	// Remove the edge live but record nothing yet: finishConnection pushes
	// the gesture's net effect as one undo step.
	rem := &RemoveEdgeCommand{Canvas: c, Edge: found}
	rem.Execute()

	// Now start a new connection from the original source
	srcPW := c.portWidget(sourcePort)
	if srcPW != nil {
		c.startConnection(sourcePort, srcPW.screenCenter())
	}
	if c.pendingEdge != nil {
		c.pendingEdge.detached = rem
	} else {
		// The reconnect drag could not start; keep the removal undoable.
		c.pushDone(rem)
	}
}

// startConnection begins a new connection drag from a port.
func (c *DAGCanvas) startConnection(sourcePort dag.PortID, screenPos geo.Position) {
	if !c.Capabilities.ConnectEdges {
		return
	}
	c.dragState = dragConnecting
	c.pendingEdge = &pendingConnection{
		sourcePort: sourcePort,
		sourcePos:  screenPos,
		currentPos: screenPos,
	}
	// Mark valid drop targets
	for _, nw := range c.nodes {
		for _, pw := range nw.Inputs {
			if c.canConnect(sourcePort, pw.PortID) {
				pw.ValidDrop = true
				pw.Refresh()
			}
		}
	}
	c.Refresh()
}

// updatePendingConnection updates the endpoint of the in-progress connection.
func (c *DAGCanvas) updatePendingConnection(absPos geo.Position) {
	if c.pendingEdge == nil {
		return
	}
	driver := bellui.CurrentApp().Driver()
	canvasAbs := driver.AbsolutePositionForObject(c)
	screenPos := geo.NewPos(absPos.X-canvasAbs.X, absPos.Y-canvasAbs.Y)
	c.pendingEdge.currentPos = screenPos
	c.pendingEdge.snapTarget = nil

	// Check for snap to valid target port
	snapDist := float32(20)
	for _, nw := range c.nodes {
		for _, pw := range nw.Inputs {
			if !pw.ValidDrop {
				continue
			}
			portCenter := pw.screenCenter()
			d := dist32(screenPos, portCenter)
			if d < snapDist {
				c.pendingEdge.snapTarget = pw
				c.pendingEdge.currentPos = portCenter
				snapDist = d
			}
		}
	}
	c.Refresh()
}

// finishConnection completes or cancels the in-progress connection.
func (c *DAGCanvas) finishConnection() {
	defer func() {
		// Reset all port states
		for _, nw := range c.nodes {
			for _, pw := range nw.Inputs {
				if pw.ValidDrop {
					pw.ValidDrop = false
					pw.Refresh()
				}
			}
		}
		c.dragState = dragNone
		c.pendingEdge = nil
		c.Refresh()
	}()

	if c.pendingEdge == nil {
		return
	}
	detached := c.pendingEdge.detached

	if c.pendingEdge.snapTarget == nil {
		// Dropped on empty space: a fresh drag just cancels; a detach drag
		// leaves the edge deleted, recorded as one undo step.
		if detached != nil {
			c.pushDone(detached)
		}
		return
	}
	src, tgt := c.pendingEdge.sourcePort, c.pendingEdge.snapTarget.PortID
	if detached != nil && tgt == detached.Edge.Target {
		detached.Undo() // dropped back where it started: net no-op
		return
	}
	if !c.canConnect(src, tgt) {
		// Target no longer valid since the snap marked it; treat a detach
		// drag like an empty drop so the removal stays undoable.
		if detached != nil {
			c.pushDone(detached)
		}
		return
	}

	cmd := &AddEdgeCommand{Canvas: c, Edge: &dag.Edge{
		ID:     c.generateEdgeID(),
		Source: src,
		Target: tgt,
	}}
	if detached == nil {
		c.do(cmd)
		return
	}
	// Rerouted: the removal already executed, so execute the add to match
	// and record the pair as one already-done compound step.
	cmd.Execute()
	if cmd.Failed() {
		c.pushDone(detached)
		return
	}
	c.pushDone(&CompoundCommand{Commands: []Command{detached, cmd}})
}

// startGroupEdgeConnection begins a dependency drag from a group's connector
// handle.
func (c *DAGCanvas) startGroupEdgeConnection(groupID string, screenPos geo.Position) {
	if !c.Capabilities.GroupEdges {
		return
	}
	c.dragState = dragConnectingGroup
	c.pendingGroupLink = &pendingGroupEdge{
		sourceGroup: groupID,
		sourcePos:   screenPos,
		currentPos:  screenPos,
	}
	c.Refresh()
}

// updatePendingGroupEdge updates the in-progress dependency drag, snapping
// to the group under the cursor when it is a valid target.
func (c *DAGCanvas) updatePendingGroupEdge(absPos geo.Position) {
	if c.pendingGroupLink == nil {
		return
	}
	driver := fyneDriver()
	canvasAbs := driver.AbsolutePositionForObject(c)
	screenPos := geo.NewPos(absPos.X-canvasAbs.X, absPos.Y-canvasAbs.Y)
	c.pendingGroupLink.currentPos = screenPos

	prevTarget := c.pendingGroupLink.snapTarget
	c.pendingGroupLink.snapTarget = ""
	if g := c.groupAtScreenPos(screenPos); g != nil && c.canConnectGroups(c.pendingGroupLink.sourceGroup, g.ID) {
		c.pendingGroupLink.snapTarget = g.ID
	}
	if prevTarget != c.pendingGroupLink.snapTarget {
		if gw := c.groups[prevTarget]; gw != nil {
			gw.DropHighlight = false
			gw.Refresh()
		}
		if gw := c.groups[c.pendingGroupLink.snapTarget]; gw != nil {
			gw.DropHighlight = true
			gw.Refresh()
		}
	}
	c.Refresh()
}

// finishGroupEdgeConnection completes or cancels the dependency drag.
func (c *DAGCanvas) finishGroupEdgeConnection() {
	pending := c.pendingGroupLink
	defer func() {
		if pending != nil {
			if gw := c.groups[pending.snapTarget]; gw != nil {
				gw.DropHighlight = false
				gw.Refresh()
			}
		}
		c.dragState = dragNone
		c.pendingGroupLink = nil
		c.Refresh()
	}()

	if pending == nil || pending.snapTarget == "" {
		return // cancelled — dropped on empty space or an invalid group
	}

	edge := &dag.GroupEdge{
		ID:     c.generateGroupEdgeID(),
		Source: pending.sourceGroup,
		Target: pending.snapTarget,
	}
	cmd := &AddGroupEdgeCommand{Canvas: c, Edge: edge}
	c.do(cmd)
}

// startLinkEdgeConnection begins a link edge drag from a node's connector
// handle.
func (c *DAGCanvas) startLinkEdgeConnection(nodeID string, screenPos geo.Position) {
	if !c.Capabilities.LinkEdges {
		return
	}
	c.dragState = dragConnectingLink
	c.pendingNodeLink = &pendingNodeLink{
		sourceNode: nodeID,
		sourcePos:  screenPos,
		currentPos: screenPos,
	}
	c.Refresh()
}

// updatePendingLinkEdge updates the in-progress link edge drag, snapping to
// the node under the cursor when it is a valid target.
func (c *DAGCanvas) updatePendingLinkEdge(absPos geo.Position) {
	if c.pendingNodeLink == nil {
		return
	}
	driver := fyneDriver()
	canvasAbs := driver.AbsolutePositionForObject(c)
	screenPos := geo.NewPos(absPos.X-canvasAbs.X, absPos.Y-canvasAbs.Y)
	c.pendingNodeLink.currentPos = screenPos

	prevTarget := c.pendingNodeLink.snapTarget
	c.pendingNodeLink.snapTarget = ""
	if n := c.nodeAtScreenPos(screenPos); n != nil && c.canConnectLink(c.pendingNodeLink.sourceNode, n.ID) {
		c.pendingNodeLink.snapTarget = n.ID
	}
	if prevTarget != c.pendingNodeLink.snapTarget {
		if nw := c.nodes[prevTarget]; nw != nil {
			nw.DropHighlight = false
			nw.Refresh()
		}
		if nw := c.nodes[c.pendingNodeLink.snapTarget]; nw != nil {
			nw.DropHighlight = true
			nw.Refresh()
		}
	}
	c.Refresh()
}

// finishLinkEdgeConnection completes or cancels the link edge drag.
func (c *DAGCanvas) finishLinkEdgeConnection() {
	pending := c.pendingNodeLink
	defer func() {
		if pending != nil {
			if nw := c.nodes[pending.snapTarget]; nw != nil {
				nw.DropHighlight = false
				nw.Refresh()
			}
		}
		c.dragState = dragNone
		c.pendingNodeLink = nil
		c.Refresh()
	}()

	if pending == nil || pending.snapTarget == "" {
		return // cancelled — dropped on empty space or an invalid node
	}

	edge := &dag.LinkEdge{
		ID:     c.generateLinkEdgeID(),
		Source: pending.sourceNode,
		Target: pending.snapTarget,
	}
	cmd := &AddLinkEdgeCommand{Canvas: c, Edge: edge}
	c.do(cmd)
}

// canConnectLink checks whether a link edge from src to tgt would be
// accepted. Only self-loops and missing endpoints are rejected — parallel and
// opposite-direction edges between the same pair are legal by design (each
// renders on its own lane).
func (c *DAGCanvas) canConnectLink(src, tgt string) bool {
	if src == tgt || src == "" || tgt == "" {
		return false
	}
	return c.Graph.Nodes[src] != nil && c.Graph.Nodes[tgt] != nil
}

// nodeAtScreenPos returns the node whose card rectangle contains the screen
// position, or nil. Nodes hidden inside collapsed groups are skipped — they
// have no individual on-screen presence to target.
func (c *DAGCanvas) nodeAtScreenPos(p geo.Position) *dag.Node {
	for id, nw := range c.nodes {
		node := c.Graph.Nodes[id]
		if node == nil || c.collapsedGroupOf(id) != nil {
			continue
		}
		pos := c.graphToScreen(geo.NewPos(node.Position[0], node.Position[1]))
		eff := nw.effectiveSize()
		r := geo.Rectangle{Pos: pos, Size: geo.NewSize(eff.Width*c.Zoom, eff.Height*c.Zoom)}
		if rectContains(r, p) {
			return node
		}
	}
	return nil
}

// canConnectGroups checks whether a dependency edge from src to tgt would be
// accepted: no self-loops, no duplicate pair (either direction of an existing
// pair keeps its meaning), no cycle through node edges and membership.
func (c *DAGCanvas) canConnectGroups(src, tgt string) bool {
	if src == tgt || src == "" || tgt == "" {
		return false
	}
	for _, ge := range c.Graph.GroupEdges {
		if ge.Source == src && ge.Target == tgt {
			return false
		}
	}
	if c.Graph.Groups[src] == nil || c.Graph.Groups[tgt] == nil {
		return false
	}
	return !c.Graph.WouldGroupCycle(src, tgt)
}

// canConnect checks if a connection from source to target would be valid,
// including stream type compatibility when a registry is available.
func (c *DAGCanvas) canConnect(src, tgt dag.PortID) bool {
	if src.NodeID == tgt.NodeID {
		return false // no self-connections
	}
	// Check target isn't already connected
	for _, e := range c.Graph.Edges {
		if e.Target == tgt {
			return false
		}
	}
	// Check stream type compatibility
	if r := c.Graph.Registry; r != nil {
		srcNode := c.Graph.Nodes[src.NodeID]
		tgtNode := c.Graph.Nodes[tgt.NodeID]
		if srcNode != nil && tgtNode != nil {
			srcStream, srcOK := r.PortStreamType(srcNode, src.Name, dag.PortOutput)
			tgtStream, tgtOK := r.PortStreamType(tgtNode, tgt.Name, dag.PortInput)
			if srcOK && tgtOK && !r.StreamsCompatible(srcStream, tgtStream) {
				return false
			}
		}
	}
	// Check it wouldn't create a cycle
	return !c.Graph.WouldCycle(src.NodeID, tgt.NodeID)
}

// AutoLayout assigns a position to every node using a layered left-to-right
// layout (dag.LayeredLayout). All moves land as a single undoable step. The
// manual-placement philosophy is unchanged — this is an on-demand convenience
// for graphs that arrive without positions.
func (c *DAGCanvas) AutoLayout() error {
	positions, err := dag.LayeredLayout(c.Graph, dag.DefaultLayoutOptions())
	if err != nil {
		return err
	}
	var moves []Command
	for id, pos := range positions {
		n := c.Graph.Nodes[id]
		if n == nil || n.Position == pos {
			continue
		}
		moves = append(moves, &MoveNodeCommand{Canvas: c, NodeID: id, OldPos: n.Position, NewPos: pos})
	}
	if len(moves) == 0 {
		return nil
	}
	c.do(&CompoundCommand{Commands: moves})
	return nil
}

// AddComponentNode creates a node from a component definition at the given
// graph position, as one undoable step. Errors when the canvas capabilities
// forbid adding nodes — it is a user-interaction entry point (palette), unlike
// the raw AddNode.
func (c *DAGCanvas) AddComponentNode(comp *dag.Component, pos [2]float32) (*dag.Node, error) {
	if !c.Capabilities.AddNodes {
		return nil, fmt.Errorf("canvas capabilities forbid adding nodes")
	}
	nodeID := c.generateEdgeID() // reuse ID generator
	n := comp.NewNode(nodeID, pos)
	cmd := &AddNodeCommand{Canvas: c, Node: n}
	c.do(cmd)
	return n, cmd.err
}

// addComponentNodeNear adds a component node at pos, or at the nearest spot
// that does not cover an existing node or group, so repeated palette taps fan
// out instead of stacking every node on the viewport center.
func (c *DAGCanvas) addComponentNodeNear(comp *dag.Component, pos [2]float32) (*dag.Node, error) {
	size := NewNodeWidget(comp.NewNode("", pos), c).effectiveSize()
	return c.AddComponentNode(comp, c.freeNodePosition(pos, size))
}

// freeNodePosition returns the candidate closest to pos, on a grid of
// card-sized steps around it, where a node of the given size (graph units)
// overlaps no node or group. It returns pos when the searched rings are full.
func (c *DAGCanvas) freeNodePosition(pos [2]float32, size geo.Size) [2]float32 {
	const gap, rings = 20, 6
	stepX, stepY := size.Width+gap, size.Height+gap
	type cand struct {
		pos  [2]float32
		dist float32
	}
	var cands []cand
	for dy := -rings; dy <= rings; dy++ {
		for dx := -rings; dx <= rings; dx++ {
			x, y := float32(dx)*stepX, float32(dy)*stepY
			cands = append(cands, cand{[2]float32{pos[0] + x, pos[1] + y}, x*x + y*y})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
	for _, cd := range cands {
		if !c.graphAreaOccupied(cd.pos, size, gap) {
			return cd.pos
		}
	}
	return pos
}

// graphAreaOccupied reports whether the rectangle at pos with size, grown by
// margin on every side, intersects any node card or group box.
func (c *DAGCanvas) graphAreaOccupied(pos [2]float32, size geo.Size, margin float32) bool {
	x0, y0 := pos[0]-margin, pos[1]-margin
	x1, y1 := pos[0]+size.Width+margin, pos[1]+size.Height+margin
	hits := func(p, s [2]float32) bool {
		return p[0] < x1 && p[0]+s[0] > x0 && p[1] < y1 && p[1]+s[1] > y0
	}
	for id, n := range c.Graph.Nodes {
		eff := geo.NewSize(n.Size[0], n.Size[1])
		if nw := c.nodes[id]; nw != nil {
			eff = nw.effectiveSize()
		}
		if hits(n.Position, [2]float32{eff.Width, eff.Height}) {
			return true
		}
	}
	for _, g := range c.Graph.Groups {
		if hits(g.Position, g.Size) {
			return true
		}
	}
	return false
}

// deselectAll clears the selection flag on every selectable widget kind.
// Selection is deliberately kept as per-widget flags plus this one shared
// helper rather than a central selection model — known debt, revisit if
// multi-select ever lands.
func (c *DAGCanvas) deselectAll() {
	for _, nw := range c.nodes {
		if nw.Selected {
			nw.Selected = false
			nw.Refresh()
		}
	}
	for _, ew := range c.edges {
		if ew.Selected {
			ew.Selected = false
			ew.Refresh()
		}
	}
	for _, gw := range c.groups {
		if gw.Selected {
			gw.Selected = false
			gw.Refresh()
		}
	}
	for _, ew := range c.groupEdges {
		if ew.Selected {
			ew.Selected = false
			ew.Refresh()
		}
	}
	for _, ew := range c.linkEdges {
		if ew.Selected {
			ew.Selected = false
			ew.Refresh()
		}
	}
	for _, aw := range c.annotations {
		if aw.Selected {
			aw.Selected = false
			aw.Refresh()
		}
	}
}

// Tapped handles clicks that reach the canvas: it hit-tests elements that
// have no tappable widget of their own — bezier edges, dashed group edges,
// and group bodies (their body rectangle is transparent to events so panning
// works across it) — routing hits through the public Select* methods, and
// clears the selection when nothing is hit.
func (c *DAGCanvas) Tapped(ev *event.PointEvent) {
	c.requestFocus()

	if hit := c.edgeAtPosition(ev.Position); hit != nil {
		c.SelectEdge(hit.EdgeID)
		return
	}

	if hit := c.linkEdgeAtPosition(ev.Position); hit != nil {
		c.SelectLinkEdge(hit.EdgeID)
		return
	}

	if hit := c.groupEdgeAtPosition(ev.Position); hit != nil {
		c.SelectGroupEdge(hit.EdgeID)
		return
	}

	if g := c.groupAtScreenPos(ev.Position); g != nil {
		c.SelectGroup(g.ID)
		return
	}

	c.ClearSelection()
}

// TappedSecondary handles right-clicks that reach the canvas, mirroring
// Tapped's hit-test order over the elements without a tappable widget of
// their own. A secondary tap selects its target like a primary tap — a
// context menu acts on the element under the pointer — then fires the
// matching On*SecondaryTapped hook. Unlike Tapped, a secondary tap on empty
// canvas is ignored: selection and focus stay untouched.
func (c *DAGCanvas) TappedSecondary(ev *event.PointEvent) {
	if hit := c.edgeAtPosition(ev.Position); hit != nil {
		c.requestFocus()
		c.SelectEdge(hit.EdgeID)
		if f := c.OnEdgeSecondaryTapped; f != nil {
			f(hit.EdgeID, ev)
		}
		return
	}

	if hit := c.linkEdgeAtPosition(ev.Position); hit != nil {
		c.requestFocus()
		c.SelectLinkEdge(hit.EdgeID)
		if f := c.OnLinkEdgeSecondaryTapped; f != nil {
			f(hit.EdgeID, ev)
		}
		return
	}

	if hit := c.groupEdgeAtPosition(ev.Position); hit != nil {
		c.requestFocus()
		c.SelectGroupEdge(hit.EdgeID)
		return
	}

	if g := c.groupAtScreenPos(ev.Position); g != nil {
		c.requestFocus()
		c.SelectGroup(g.ID)
		if f := c.OnGroupSecondaryTapped; f != nil {
			f(g.ID, ev)
		}
	}
}

// edgeAtPosition returns the EdgeWidget closest to the given screen position, or nil.
func (c *DAGCanvas) edgeAtPosition(screenPos geo.Position) *EdgeWidget {
	threshold := float32(8)
	for _, ew := range c.edges {
		if HitTestBezier(ew.p0, ew.p1, ew.p2, ew.p3, screenPos, threshold) {
			return ew
		}
	}
	return nil
}

// --- Focusable interface for keyboard input ---

// requestFocus directs key events (Delete, Enter) to the canvas. Called from
// taps on the canvas itself and on elements whose selection keyboard actions
// operate on.
func (c *DAGCanvas) requestFocus() {
	if cnv := bellui.CurrentApp().Driver().CanvasForObject(c); cnv != nil {
		cnv.Focus(c)
	}
}

func (c *DAGCanvas) FocusGained()   { c.focused = true }
func (c *DAGCanvas) FocusLost()     { c.focused = false }
func (c *DAGCanvas) TypedRune(rune) {}

// TypedKey handles the canvas's keyboard verbs: Tab/Shift+Tab traverse the
// canvas's elements (AcceptsTab keeps the key here instead of moving window
// focus), Escape clears the selection, Delete/Backspace removes it, Enter
// activates it, and Menu (the application key) or Shift+F10 opens its context
// menu. Anything else — including Menu / Shift+F10 with nothing selected, and
// plain F10 — falls out of the switch and does nothing, which is all
// "unhandled" can mean here: TypedKey has no consumed/propagate signal to
// hand a key back to the driver.
func (c *DAGCanvas) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeyTab:
		if ev.Modifier&event.KeyModifierShift != 0 {
			c.traverseSelection(-1)
		} else {
			c.traverseSelection(1)
		}
	case event.KeyEscape:
		c.ClearSelection()
	case event.KeyDelete, event.KeyBackspace:
		c.DeleteSelected()
	case event.KeyReturn, event.KeyEnter:
		c.activateSelection()
	case desktop.KeyMenu:
		c.contextMenuSelection()
	case event.KeyF10:
		if ev.Modifier&event.KeyModifierShift != 0 {
			c.contextMenuSelection()
		}
	}
}

// activateSelection fires the activation callback for the currently selected
// node or group (scanning the per-widget Selected flags, like deselectAll).
// Other element kinds have no activation semantics.
func (c *DAGCanvas) activateSelection() {
	for id, nw := range c.nodes {
		if nw.Selected {
			if f := c.OnNodeActivated; f != nil {
				f(id)
			}
			return
		}
	}
	for id, gw := range c.groups {
		if gw.Selected {
			if f := c.OnGroupActivated; f != nil {
				f(id)
			}
			return
		}
	}
}

// contextMenuSelection is the keyboard path to element context menus (Menu
// key / Shift+F10): it fires the matching On*SecondaryTapped hook for the
// currently selected element (scanning the per-widget Selected flags, like
// activateSelection) with a synthesized PointEvent anchored at the element's
// on-screen center. The asymmetry with the pointer path is deliberate: a
// pointer secondary tap establishes the selection, so it selects-then-fires;
// the keyboard consumes the selection, which is already the menu's target,
// so On*Selected does NOT re-fire. A selected group edge stays silent (no
// secondary hook) and nil callbacks change nothing, both matching pointer.
func (c *DAGCanvas) contextMenuSelection() {
	for id, nw := range c.nodes {
		if !nw.Selected {
			continue
		}
		if f := c.OnNodeSecondaryTapped; f != nil {
			if rect, ok := c.nodeVisibleScreenRect(id); ok {
				f(id, c.keyboardPointEvent(rectCenter(rect), rect.Pos))
			}
		}
		return
	}
	for id, ew := range c.edges {
		if !ew.Selected {
			continue
		}
		if f := c.OnEdgeSecondaryTapped; f != nil {
			// Cubic bezier midpoint at t=0.5, as in anchorScreenPoint.
			mid := geo.NewPos(
				(ew.p0.X+3*ew.p1.X+3*ew.p2.X+ew.p3.X)/8,
				(ew.p0.Y+3*ew.p1.Y+3*ew.p2.Y+ew.p3.Y)/8,
			)
			f(id, c.keyboardPointEvent(mid, geo.Position{}))
		}
		return
	}
	for _, gw := range c.groups {
		if !gw.Selected {
			continue
		}
		if f := c.OnGroupSecondaryTapped; f != nil {
			if g := c.Graph.Groups[gw.GroupID]; g != nil {
				f(gw.GroupID, c.keyboardPointEvent(rectCenter(c.groupScreenRect(g)), geo.Position{}))
			}
		}
		return
	}
	for id, lw := range c.linkEdges {
		if !lw.Selected {
			continue
		}
		if f := c.OnLinkEdgeSecondaryTapped; f != nil {
			f(id, c.keyboardPointEvent(geo.NewPos((lw.p0.X+lw.p1.X)/2, (lw.p0.Y+lw.p1.Y)/2), geo.Position{}))
		}
		return
	}
	for id, aw := range c.annotations {
		if !aw.Selected {
			continue
		}
		if f := c.OnAnnotationSecondaryTapped; f != nil {
			if a := c.Graph.Annotations[id]; a != nil {
				// Model geometry, like nodeVisibleScreenRect — always
				// current, independent of the last layout pass.
				eff := aw.effectiveSize()
				rect := geo.Rectangle{
					Pos:  c.graphToScreen(geo.NewPos(a.Position[0], a.Position[1])),
					Size: geo.NewSize(eff.Width*c.Zoom, eff.Height*c.Zoom),
				}
				f(id, c.keyboardPointEvent(rectCenter(rect), rect.Pos))
			}
		}
		return
	}
}

// keyboardPointEvent synthesizes the PointEvent a keyboard-fired secondary
// hook forwards, from an anchor in canvas (screen) coordinates: Position is
// the anchor relative to origin — the element's top-left for hooks whose
// pointer path dispatches from the element's own widget (nodes,
// annotations), the canvas origin for hooks the canvas dispatches (edges,
// link edges, groups) — and AbsolutePosition is window-absolute, the
// canvas's own absolute position plus the anchor, so
// widget.ShowPopUpMenuAtPosition anchors on the element like a real tap.
func (c *DAGCanvas) keyboardPointEvent(anchor, origin geo.Position) *event.PointEvent {
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(c)
	return &event.PointEvent{
		Position:         anchor.Subtract(origin),
		AbsolutePosition: abs.Add(anchor),
	}
}

// TypedShortcut handles Ctrl+Z (undo) and Ctrl+Shift+Z / Ctrl+Y (redo).
func (c *DAGCanvas) TypedShortcut(s core.Shortcut) {
	if c.Commands == nil {
		return
	}
	switch s.(type) {
	case *bellui.ShortcutUndo:
		c.Commands.Undo()
	case *bellui.ShortcutRedo:
		c.Commands.Redo()
	}
}

// DeleteSelected removes all selected elements through the command stack.
// Order matters: edges before the nodes/groups whose deletion would cascade
// them, so each removal command captures accurate undo state. Every command
// no-ops gracefully when its target was already cascade-deleted. Each element
// kind is skipped when the canvas capabilities forbid removing it.
func (c *DAGCanvas) DeleteSelected() {
	caps := c.Capabilities
	for _, ew := range c.groupEdges {
		if ew.Selected && caps.GroupEdges {
			if ge := c.Graph.GroupEdges[ew.EdgeID]; ge != nil {
				cmd := &RemoveGroupEdgeCommand{Canvas: c, Edge: ge}
				c.do(cmd)
			}
		}
	}
	for _, ew := range c.linkEdges {
		if ew.Selected && caps.LinkEdges {
			if le := c.Graph.LinkEdges[ew.EdgeID]; le != nil {
				cmd := &RemoveLinkEdgeCommand{Canvas: c, Edge: le}
				c.do(cmd)
			}
		}
	}
	for _, ew := range c.edges {
		if ew.Selected && caps.ConnectEdges {
			if e := c.Graph.Edges[ew.EdgeID]; e != nil {
				c.do(&RemoveEdgeCommand{Canvas: c, Edge: e})
			}
		}
	}
	for _, aw := range c.annotations {
		if aw.Selected && caps.Annotations {
			if a := c.Graph.Annotations[aw.AnnotationID]; a != nil {
				cmd := &RemoveAnnotationCommand{Canvas: c, Annotation: a}
				c.do(cmd)
			}
		}
	}
	for _, gw := range c.groups {
		if gw.Selected && caps.EditGroups {
			if g := c.Graph.Groups[gw.GroupID]; g != nil {
				cmd := &RemoveGroupCommand{Canvas: c, Group: g}
				c.do(cmd)
			}
		}
	}
	for _, nw := range c.nodes {
		if nw.Selected && caps.RemoveNodes {
			if n := c.Graph.Nodes[nw.NodeID]; n != nil {
				c.do(&RemoveNodeCommand{Canvas: c, Node: n})
			}
		}
	}
	c.Refresh()
	if f := c.OnSelectionCleared; f != nil {
		f()
	}
}

// AddGroupAtCenter creates an empty group at the viewport center as one
// undoable step. Nodes join by being dragged inside it. No-op when the
// canvas capabilities forbid editing groups.
func (c *DAGCanvas) AddGroupAtCenter() {
	if !c.Capabilities.EditGroups {
		return
	}
	size := c.Size()
	center := c.screenToGraph(geo.NewPos(size.Width/2, size.Height/2))
	g := &dag.Group{
		ID:       c.generateGroupID(),
		Label:    "Group",
		Position: [2]float32{center.X - defaultGroupW/2, center.Y - defaultGroupH/2},
		Size:     [2]float32{defaultGroupW, defaultGroupH},
	}
	cmd := &AddGroupCommand{Canvas: c, Group: g}
	c.do(cmd)
}

// AddAnnotationAtCenter creates a floating annotation at the viewport center
// as one undoable step. No-op when the canvas capabilities forbid
// annotations.
func (c *DAGCanvas) AddAnnotationAtCenter() {
	if !c.Capabilities.Annotations {
		return
	}
	size := c.Size()
	center := c.screenToGraph(geo.NewPos(size.Width/2, size.Height/2))
	a := &dag.Annotation{
		ID:       c.generateAnnotationID(),
		Text:     "Note",
		Position: [2]float32{center.X - defaultAnnotationW/2, center.Y - defaultAnnotationH/2},
		Size:     [2]float32{defaultAnnotationW, defaultAnnotationH},
	}
	cmd := &AddAnnotationCommand{Canvas: c, Annotation: a}
	c.do(cmd)
}

// Dragged handles panning the canvas.
func (c *DAGCanvas) Dragged(ev *event.DragEvent) {
	if c.dragState == dragConnecting {
		return // handled by PortWidget
	}
	if c.dragState == dragNone {
		c.dragState = dragPanning
	}
	if c.dragState == dragPanning {
		c.panBy(ev.Dragged)
	}
}

// panBy moves the view with a finger or pointer that moved by d on screen.
func (c *DAGCanvas) panBy(d geo.Delta) {
	c.Offset.X -= d.DX / c.Zoom
	c.Offset.Y -= d.DY / c.Zoom
	c.Refresh()
}

func (c *DAGCanvas) DragEnd() {
	if c.dragState == dragPanning {
		c.dragState = dragNone
	}
}

// Scrolled handles zoom.
func (c *DAGCanvas) Scrolled(ev *event.ScrollEvent) {
	c.TryScrolled(ev)
}

// TryScrolled zooms on a wheel or touchpad scroll. A touch scroll — a
// two-finger pan (#398) — pans the graph with the fingers instead, the way a
// one-finger drag does; on touch the canvas zooms by pinch.
func (c *DAGCanvas) TryScrolled(ev *event.ScrollEvent) bool {
	if ev.Source == event.ScrollSourceTouch {
		c.panBy(ev.Scrolled)
		return true
	}
	c.zoomAround(ev.Position, ev.Position, 1.0+ev.Scrolled.DY*0.002)
	return true
}

// Pinched zooms by the pinch's scale around the point between the fingers,
// and pans with them: the graph point that was between the fingers at the
// previous step is moved to where they are now.
func (c *DAGCanvas) Pinched(ev *event.PinchEvent) {
	from := ev.Position
	if c.pinching {
		from = c.pinchFocus
	}
	c.pinching = true
	c.pinchFocus = ev.Position
	c.zoomAround(from, ev.Position, ev.Scale)
}

// PinchEnded is part of event.Pinchable; the zoom is already applied.
func (c *DAGCanvas) PinchEnded() {
	c.pinching = false
}

// zoomAround multiplies the zoom by factor, clamped to 0.1–3.0, and moves the
// graph point under from to under pos (both in canvas coordinates); with from
// equal to pos that point stays where it is.
func (c *DAGCanvas) zoomAround(from, pos geo.Position, factor float32) {
	oldZoom := c.Zoom
	c.Zoom *= factor
	if c.Zoom < 0.1 {
		c.Zoom = 0.1
	}
	if c.Zoom > 3.0 {
		c.Zoom = 3.0
	}

	focus := c.screenToGraphWithZoom(from, oldZoom)
	c.Offset.X = focus.X - pos.X/c.Zoom
	c.Offset.Y = focus.Y - pos.Y/c.Zoom
	c.Refresh()
}

func (c *DAGCanvas) screenToGraphWithZoom(p geo.Position, zoom float32) geo.Position {
	return geo.NewPos(
		p.X/zoom+c.Offset.X,
		p.Y/zoom+c.Offset.Y,
	)
}

func (c *DAGCanvas) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	bg := canvas.NewRectangle(color.Transparent)
	r := &dagCanvasRenderer{
		dc: c,
		bg: bg,
	}
	r.applyTheme()
	return r
}

func dist32(a, b geo.Position) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}
