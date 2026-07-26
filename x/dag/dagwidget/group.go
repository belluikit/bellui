package dagwidget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// Group geometry in graph units (scaled by zoom at render time).
const (
	groupTitleHeight    float32 = 26
	groupMinWidth       float32 = 120
	groupMinHeight      float32 = 80
	groupCollapsedW     float32 = 180
	groupCollapsedH     float32 = 48
	connectorHandleSize float32 = 12
	collapseButtonSize  float32 = 16
	defaultGroupW       float32 = 280
	defaultGroupH       float32 = 180
	// groupContentPadding is the clearance kept between a member node and
	// its group's border when the group grows to contain a dropped node.
	groupContentPadding float32 = 12
	// nodeClearGap is the clearance left between a node and a group border
	// when a non-member node is nudged out of the group's rectangle.
	nodeClearGap float32 = 8
)

// GroupWidget renders a container of nodes as a translucent rectangle with a
// title bar. The widget itself implements no event interfaces: the body is
// transparent to input (canvas pan/zoom pass through, body taps are resolved
// by DAGCanvas.Tapped/TappedSecondary's hit-test fallback), and interaction
// lives in the title-bar children — drag to move (members follow), a
// connector handle to
// draw dependency edges to other groups, and a collapse toggle.
type GroupWidget struct {
	widget.BaseWidget

	GroupID  string
	Selected bool
	// DropHighlight marks this group as the drop target while a node drag or
	// a group-edge drag hovers over it.
	DropHighlight bool

	titleBar  *groupTitleBar
	collapse  *groupCollapseButton
	connector *groupConnectorHandle
	handles   []*HandleWidget // resize handles, visible while selected

	dagCanvas *DAGCanvas
}

var _ core.Widget = (*GroupWidget)(nil)

// NewGroupWidget creates a widget for a dag.Group.
func NewGroupWidget(g *dag.Group, dc *DAGCanvas) *GroupWidget {
	gw := &GroupWidget{
		GroupID:   g.ID,
		dagCanvas: dc,
	}
	gw.titleBar = &groupTitleBar{gw: gw}
	gw.titleBar.ExtendBaseWidget(gw.titleBar)
	gw.collapse = &groupCollapseButton{gw: gw}
	gw.collapse.ExtendBaseWidget(gw.collapse)
	gw.connector = &groupConnectorHandle{gw: gw}
	gw.connector.ExtendBaseWidget(gw.connector)
	gw.handles = newHandleSet(gw, groupHandleDirs)
	gw.ExtendBaseWidget(gw)
	return gw
}

// group returns the underlying dag.Group, or nil if it was removed.
func (g *GroupWidget) group() *dag.Group {
	if g.dagCanvas == nil {
		return nil
	}
	return g.dagCanvas.Graph.Groups[g.GroupID]
}

func (g *GroupWidget) collapsed() bool {
	grp := g.group()
	return grp != nil && grp.Collapsed
}

// effectiveSize returns the group's on-canvas size in graph units: the fixed
// compact card when collapsed, otherwise the stored size clamped to minimum.
func (g *GroupWidget) effectiveSize() geo.Size {
	grp := g.group()
	if grp == nil {
		return geo.NewSize(groupMinWidth, groupMinHeight)
	}
	if grp.Collapsed {
		return geo.NewSize(groupCollapsedW, groupCollapsedH)
	}
	return geo.NewSize(
		max32(grp.Size[0], groupMinWidth),
		max32(grp.Size[1], groupMinHeight),
	)
}

// handlesVisible reports whether the resize handles should show: selected,
// expanded (the collapsed card has a fixed size), and the canvas allows
// move/resize. Shared by the renderer's Refresh and the canvas-side
// affordance sync (see dagCanvasRenderer.Layout).
func (g *GroupWidget) handlesVisible() bool {
	return g.Selected && !g.collapsed() &&
		(g.dagCanvas == nil || g.dagCanvas.Capabilities.MoveResize)
}

func (g *GroupWidget) MinSize() geo.Size {
	return geo.NewSize(0, 0) // sized by the canvas renderer
}

func (g *GroupWidget) zoom() float32 {
	if g.dagCanvas != nil {
		return g.dagCanvas.Zoom
	}
	return 1.0
}

func (g *GroupWidget) CreateRenderer() core.WidgetRenderer {
	return newGroupRenderer(g)
}

// --- resizable interface ---

func (g *GroupWidget) resizeCanvas() *DAGCanvas { return g.dagCanvas }

func (g *GroupWidget) storedGeometry() (pos, size [2]float32, ok bool) {
	grp := g.group()
	if grp == nil {
		return pos, size, false
	}
	return grp.Position, grp.Size, true
}

func (g *GroupWidget) setStoredGeometry(pos, size [2]float32) {
	if grp := g.group(); grp != nil {
		grp.Position = pos
		grp.Size = size
	}
}

func (g *GroupWidget) effectiveGraphSize() geo.Size { return g.effectiveSize() }

func (g *GroupWidget) minGraphSize() geo.Size {
	return geo.NewSize(groupMinWidth, groupMinHeight)
}

func (g *GroupWidget) pushResizeCommand(oldPos, oldSize, newPos, newSize [2]float32) {
	g.dagCanvas.pushDone(&ResizeGroupCommand{
		Canvas:  g.dagCanvas,
		GroupID: g.GroupID,
		OldPos:  oldPos,
		NewPos:  newPos,
		OldSize: oldSize,
		NewSize: newSize,
	})
}

var _ resizable = (*GroupWidget)(nil)

// --- title bar: drag to move (members follow), tap to select ---

type groupTitleBar struct {
	widget.BaseWidget

	gw *GroupWidget

	dragging    bool
	startPos    [2]float32
	memberStart map[string][2]float32 // node positions at drag start
	taps        tapTracker            // manual double-tap detection for activation
}

var (
	_ core.Widget             = (*groupTitleBar)(nil)
	_ event.Draggable         = (*groupTitleBar)(nil)
	_ event.Tappable          = (*groupTitleBar)(nil)
	_ event.SecondaryTappable = (*groupTitleBar)(nil)
	_ desktop.Cursorable      = (*groupTitleBar)(nil)
	_ core.DragImageProvider  = (*groupTitleBar)(nil)
)

// Cursor shows an open hand over the title bar: dragging it moves the group.
func (t *groupTitleBar) Cursor() desktop.Cursor {
	return desktop.GrabCursor
}

// DragImage returns nil (the group itself moves live); implementing
// DragImageProvider switches the pointer to the grabbing cursor during the
// drag so the interaction reads as "holding" the group.
func (t *groupTitleBar) DragImage() core.CanvasObject { return nil }

// Dragged moves the group and all member nodes in graph coordinates.
func (t *groupTitleBar) Dragged(ev *event.DragEvent) {
	c := t.gw.dagCanvas
	if c == nil || !c.Capabilities.MoveResize {
		return
	}
	grp := t.gw.group()
	if grp == nil {
		return
	}
	if !t.dragging {
		t.dragging = true
		t.startPos = grp.Position
		t.memberStart = make(map[string][2]float32, len(grp.Members))
		for _, id := range grp.Members {
			if n := c.Graph.Nodes[id]; n != nil {
				t.memberStart[id] = n.Position
			}
		}
	}
	dx := ev.Dragged.DX / c.Zoom
	dy := ev.Dragged.DY / c.Zoom
	grp.Position[0] += dx
	grp.Position[1] += dy
	for id := range t.memberStart {
		if n := c.Graph.Nodes[id]; n != nil {
			n.Position[0] += dx
			n.Position[1] += dy
		}
	}
	c.Refresh()
}

func (t *groupTitleBar) DragEnd() {
	if !t.dragging {
		return
	}
	t.dragging = false
	c := t.gw.dagCanvas
	grp := t.gw.group()
	if grp == nil {
		return
	}
	if grp.Position == t.startPos {
		return
	}
	memberNew := make(map[string][2]float32, len(t.memberStart))
	for id := range t.memberStart {
		if n := c.Graph.Nodes[id]; n != nil {
			memberNew[id] = n.Position
		}
	}
	// Recorded without executing — the drag already moved everything.
	c.pushDone(&MoveGroupCommand{
		Canvas:    c,
		GroupID:   t.gw.GroupID,
		OldPos:    t.startPos,
		NewPos:    grp.Position,
		MemberOld: t.memberStart,
		MemberNew: memberNew,
	})
}

// Tapped selects the group; a second tap within the double-tap window also
// activates it (see tapTracker for why this is not event.DoubleTappable).
// Only the title bar activates — the collapsed card's body resolves through
// DAGCanvas.Tapped and selects without activating.
func (t *groupTitleBar) Tapped(ev *event.PointEvent) {
	c := t.gw.dagCanvas
	if c == nil {
		return
	}
	c.requestFocus() // so Enter/Delete act on this selection
	c.SelectGroup(t.gw.GroupID)
	if t.taps.doubleTap(ev) {
		if f := c.OnGroupActivated; f != nil {
			f(t.gw.GroupID)
		}
	}
}

// TappedSecondary selects the group like a primary tap — a context menu acts
// on the element under the pointer — then fires OnGroupSecondaryTapped. It
// never activates. Secondary taps on the group body (and on the collapsed
// card's body) resolve through DAGCanvas.TappedSecondary instead.
func (t *groupTitleBar) TappedSecondary(ev *event.PointEvent) {
	c := t.gw.dagCanvas
	if c == nil {
		return
	}
	c.requestFocus() // so Enter/Delete act on this selection
	c.SelectGroup(t.gw.GroupID)
	if f := c.OnGroupSecondaryTapped; f != nil {
		f(t.gw.GroupID, ev)
	}
}

func (t *groupTitleBar) CreateRenderer() core.WidgetRenderer {
	return newGroupTitleBarRenderer(t)
}

// --- collapse button ---

type groupCollapseButton struct {
	widget.BaseWidget

	gw *GroupWidget
}

var (
	_ core.Widget    = (*groupCollapseButton)(nil)
	_ event.Tappable = (*groupCollapseButton)(nil)
)

func (b *groupCollapseButton) Tapped(*event.PointEvent) {
	c := b.gw.dagCanvas
	grp := b.gw.group()
	if c == nil || grp == nil {
		return
	}
	c.do(&ToggleGroupCollapseCommand{Canvas: c, GroupID: b.gw.GroupID, Collapsed: !grp.Collapsed})
}

func (b *groupCollapseButton) CreateRenderer() core.WidgetRenderer {
	return newGroupCollapseRenderer(b)
}

// --- connector handle: drag to draw a dependency edge to another group ---

type groupConnectorHandle struct {
	widget.BaseWidget

	gw *GroupWidget
}

var (
	_ core.Widget            = (*groupConnectorHandle)(nil)
	_ event.Draggable        = (*groupConnectorHandle)(nil)
	_ desktop.Cursorable     = (*groupConnectorHandle)(nil)
	_ core.DragImageProvider = (*groupConnectorHandle)(nil)
)

// Cursor shows a crosshair over the connector handle to hint that a
// dependency edge can be drawn from here.
func (h *groupConnectorHandle) Cursor() desktop.Cursor {
	return desktop.CrosshairCursor
}

// DragImage returns nil (the canvas draws the dashed ghost); implementing
// DragImageProvider gives the grabbing cursor during the drag.
func (h *groupConnectorHandle) DragImage() core.CanvasObject { return nil }

func (h *groupConnectorHandle) Dragged(ev *event.DragEvent) {
	c := h.gw.dagCanvas
	if c == nil || !c.Capabilities.GroupEdges {
		return
	}
	if c.dragState != dragConnectingGroup {
		c.startGroupEdgeConnection(h.gw.GroupID, h.screenCenter())
	}
	if c.dragState == dragConnectingGroup {
		c.updatePendingGroupEdge(ev.AbsolutePosition)
	}
}

func (h *groupConnectorHandle) DragEnd() {
	c := h.gw.dagCanvas
	if c != nil && c.dragState == dragConnectingGroup {
		c.finishGroupEdgeConnection()
	}
}

// screenCenter returns the handle's center in screen coordinates relative to
// the DAGCanvas.
func (h *groupConnectorHandle) screenCenter() geo.Position {
	driver := fyneDriver()
	absSelf := driver.AbsolutePositionForObject(h)
	absCanvas := driver.AbsolutePositionForObject(h.gw.dagCanvas)
	return geo.NewPos(
		absSelf.X-absCanvas.X+h.Size().Width/2,
		absSelf.Y-absCanvas.Y+h.Size().Height/2,
	)
}

func (h *groupConnectorHandle) CreateRenderer() core.WidgetRenderer {
	return newGroupConnectorRenderer(h)
}
