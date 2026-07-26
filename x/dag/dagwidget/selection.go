package dagwidget

import (
	"sort"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// This file is the canvas's selection API: the public Select*/ClearSelection
// methods (the programmatic path consumers and tests drive) and Tab/Shift+Tab
// keyboard traversal, which cycles the same methods over the canvas's
// elements in a stable order. Selection stays per-widget flags plus the
// shared deselectAll — known debt, revisit if multi-select ever lands.

// SelectNode makes the given node the sole selection, driving the same path
// as a tap on it: selection styling applies and OnNodeSelected fires. Unlike
// a tap it does not move keyboard focus to the canvas. Returns false (and
// leaves the selection untouched) for an unknown ID.
func (c *DAGCanvas) SelectNode(id string) bool {
	nw := c.nodes[id]
	if nw == nil {
		return false
	}
	c.deselectAll()
	nw.Selected = true
	nw.Refresh()
	if f := c.OnNodeSelected; f != nil {
		f(id)
	}
	return true
}

// SelectEdge makes the given edge the sole selection; OnEdgeSelected fires.
// See SelectNode for the shared contract.
func (c *DAGCanvas) SelectEdge(id string) bool {
	ew := c.edges[id]
	if ew == nil {
		return false
	}
	c.deselectAll()
	ew.Selected = true
	ew.Refresh()
	if f := c.OnEdgeSelected; f != nil {
		f(id)
	}
	return true
}

// SelectGroup makes the given group the sole selection; OnGroupSelected
// fires. See SelectNode for the shared contract.
func (c *DAGCanvas) SelectGroup(id string) bool {
	gw := c.groups[id]
	if gw == nil {
		return false
	}
	c.deselectAll()
	gw.Selected = true
	gw.Refresh()
	if f := c.OnGroupSelected; f != nil {
		f(id)
	}
	return true
}

// SelectGroupEdge makes the given group edge the sole selection. Group edges
// have no selection callback, matching the pointer path, which selects them
// silently. See SelectNode for the shared contract.
func (c *DAGCanvas) SelectGroupEdge(id string) bool {
	ew := c.groupEdges[id]
	if ew == nil {
		return false
	}
	c.deselectAll()
	ew.Selected = true
	ew.Refresh()
	return true
}

// SelectLinkEdge makes the given link edge the sole selection;
// OnLinkEdgeSelected fires. See SelectNode for the shared contract.
func (c *DAGCanvas) SelectLinkEdge(id string) bool {
	ew := c.linkEdges[id]
	if ew == nil {
		return false
	}
	c.deselectAll()
	ew.Selected = true
	ew.Refresh()
	if f := c.OnLinkEdgeSelected; f != nil {
		f(id)
	}
	return true
}

// SelectAnnotation makes the given annotation the sole selection;
// OnAnnotationSelected fires. See SelectNode for the shared contract.
func (c *DAGCanvas) SelectAnnotation(id string) bool {
	aw := c.annotations[id]
	if aw == nil {
		return false
	}
	c.deselectAll()
	aw.Selected = true
	aw.Refresh()
	if f := c.OnAnnotationSelected; f != nil {
		f(id)
	}
	return true
}

// ClearSelection deselects whatever is selected and fires OnSelectionCleared,
// like a tap on empty canvas; keyboard focus is untouched.
func (c *DAGCanvas) ClearSelection() {
	c.deselectAll()
	if f := c.OnSelectionCleared; f != nil {
		f()
	}
}

// --- keyboard traversal (Tab / Shift+Tab) ---

// elementKind orders the selectable element kinds for traversal and
// current-selection scans: nodes, edges, groups, group edges, link edges,
// annotations — the same order deselectAll and contextMenuSelection walk.
type elementKind int

const (
	kindNode elementKind = iota
	kindEdge
	kindGroup
	kindGroupEdge
	kindLinkEdge
	kindAnnotation
)

// selectableRef identifies one selectable element.
type selectableRef struct {
	kind elementKind
	id   string
}

// AcceptsTab keeps Tab key presses on the canvas (event.Tabbable) so
// Tab/Shift+Tab traverse elements instead of moving window focus.
func (c *DAGCanvas) AcceptsTab() bool { return true }

// traversalOrder returns every currently reachable element in the stable
// traversal order: by kind (elementKind order), then by ID within each kind.
// Elements hidden by a collapsed group — member nodes, edges and link edges
// fully inside one collapsed group — are skipped: their focus ring could
// never be seen, and the collapsed card itself is reachable as the group.
func (c *DAGCanvas) traversalOrder() []selectableRef {
	var order []selectableRef
	appendKind := func(kind elementKind, ids []string) {
		sort.Strings(ids)
		for _, id := range ids {
			order = append(order, selectableRef{kind, id})
		}
	}

	ids := make([]string, 0, len(c.nodes))
	for id := range c.nodes {
		if c.collapsedGroupOf(id) == nil {
			ids = append(ids, id)
		}
	}
	appendKind(kindNode, ids)

	ids = ids[:0]
	for id, ew := range c.edges {
		if c.hiddenInOneCollapsedGroup(ew.Source.NodeID, ew.Target.NodeID) {
			continue
		}
		ids = append(ids, id)
	}
	appendKind(kindEdge, ids)

	ids = ids[:0]
	for id := range c.groups {
		ids = append(ids, id)
	}
	appendKind(kindGroup, ids)

	ids = ids[:0]
	for id := range c.groupEdges {
		ids = append(ids, id)
	}
	appendKind(kindGroupEdge, ids)

	ids = ids[:0]
	for id := range c.linkEdges {
		le := c.Graph.LinkEdges[id]
		if le != nil && c.hiddenInOneCollapsedGroup(le.Source, le.Target) {
			continue
		}
		ids = append(ids, id)
	}
	appendKind(kindLinkEdge, ids)

	ids = ids[:0]
	for id := range c.annotations {
		ids = append(ids, id)
	}
	appendKind(kindAnnotation, ids)

	return order
}

// hiddenInOneCollapsedGroup reports whether both nodes sit inside the same
// collapsed group — the condition under which the connection between them is
// not drawn (crossing connections re-anchor to the collapsed card instead).
func (c *DAGCanvas) hiddenInOneCollapsedGroup(src, tgt string) bool {
	sg := c.collapsedGroupOf(src)
	tg := c.collapsedGroupOf(tgt)
	return sg != nil && tg != nil && sg.ID == tg.ID
}

// currentSelection returns the selected element, scanning the per-widget
// Selected flags in traversal order; ok is false when nothing is selected.
func (c *DAGCanvas) currentSelection() (selectableRef, bool) {
	for id, nw := range c.nodes {
		if nw.Selected {
			return selectableRef{kindNode, id}, true
		}
	}
	for id, ew := range c.edges {
		if ew.Selected {
			return selectableRef{kindEdge, id}, true
		}
	}
	for id, gw := range c.groups {
		if gw.Selected {
			return selectableRef{kindGroup, id}, true
		}
	}
	for id, ew := range c.groupEdges {
		if ew.Selected {
			return selectableRef{kindGroupEdge, id}, true
		}
	}
	for id, ew := range c.linkEdges {
		if ew.Selected {
			return selectableRef{kindLinkEdge, id}, true
		}
	}
	for id, aw := range c.annotations {
		if aw.Selected {
			return selectableRef{kindAnnotation, id}, true
		}
	}
	return selectableRef{}, false
}

// selectRef routes a traversal step through the matching public Select*
// method, so traversal drives the exact same path as a tap.
func (c *DAGCanvas) selectRef(ref selectableRef) {
	switch ref.kind {
	case kindNode:
		c.SelectNode(ref.id)
	case kindEdge:
		c.SelectEdge(ref.id)
	case kindGroup:
		c.SelectGroup(ref.id)
	case kindGroupEdge:
		c.SelectGroupEdge(ref.id)
	case kindLinkEdge:
		c.SelectLinkEdge(ref.id)
	case kindAnnotation:
		c.SelectAnnotation(ref.id)
	}
}

// traverseSelection advances the selection by dir (+1 Tab, -1 Shift+Tab)
// through traversalOrder, wrapping at either end. With nothing selected — or
// a selection traversal skips, like a node hidden mid-session by a collapse —
// it starts at the first (or, going backward, last) element. The landing
// element is panned into view so the focus ring is visible.
func (c *DAGCanvas) traverseSelection(dir int) {
	order := c.traversalOrder()
	if len(order) == 0 {
		return
	}
	idx := 0
	if dir < 0 {
		idx = len(order) - 1
	}
	if cur, ok := c.currentSelection(); ok {
		for i, ref := range order {
			if ref == cur {
				idx = (i + dir + len(order)) % len(order)
				break
			}
		}
	}
	c.selectRef(order[idx])
	c.ensureRefVisible(order[idx])
}

// ensureRefVisible pans the viewport (zoom untouched) to center the given
// element when its anchor point is outside the canvas, so keyboard traversal
// never lands on an invisible selection. A canvas that has no size yet (not
// laid out) is left alone.
func (c *DAGCanvas) ensureRefVisible(ref selectableRef) {
	size := c.Size()
	if size.Width <= 0 || size.Height <= 0 || c.Zoom <= 0 {
		return
	}
	center, ok := c.refScreenCenter(ref)
	if !ok {
		return
	}
	if center.X >= 0 && center.X <= size.Width && center.Y >= 0 && center.Y <= size.Height {
		return
	}
	viewCenter := geo.NewPos(size.Width/2, size.Height/2)
	delta := center.Subtract(viewCenter)
	c.Offset = c.Offset.Add(geo.NewPos(delta.X/c.Zoom, delta.Y/c.Zoom))
	c.Refresh()
}

// refScreenCenter returns an element's representative point in canvas
// (screen) coordinates, computed from model geometry like portCenterScreen —
// widget positions can be stale before the next layout pass. Connections use
// their endpoint midpoint, boxes their rect center.
func (c *DAGCanvas) refScreenCenter(ref selectableRef) (geo.Position, bool) {
	mid := func(a, b geo.Position) geo.Position {
		return geo.NewPos((a.X+b.X)/2, (a.Y+b.Y)/2)
	}
	switch ref.kind {
	case kindNode:
		if rect, ok := c.nodeVisibleScreenRect(ref.id); ok {
			return rectCenter(rect), true
		}
	case kindEdge:
		e := c.Graph.Edges[ref.id]
		if e == nil {
			break
		}
		src, okS := c.portCenterScreen(e.Source)
		tgt, okT := c.portCenterScreen(e.Target)
		if okS && okT {
			return mid(src, tgt), true
		}
	case kindGroup:
		if g := c.Graph.Groups[ref.id]; g != nil {
			return rectCenter(c.groupScreenRect(g)), true
		}
	case kindGroupEdge:
		ge := c.Graph.GroupEdges[ref.id]
		if ge == nil {
			break
		}
		src, tgt := c.Graph.Groups[ge.Source], c.Graph.Groups[ge.Target]
		if src != nil && tgt != nil {
			return mid(rectCenter(c.groupScreenRect(src)), rectCenter(c.groupScreenRect(tgt))), true
		}
	case kindLinkEdge:
		le := c.Graph.LinkEdges[ref.id]
		if le == nil {
			break
		}
		src, okS := c.nodeVisibleScreenRect(le.Source)
		tgt, okT := c.nodeVisibleScreenRect(le.Target)
		if okS && okT {
			return mid(rectCenter(src), rectCenter(tgt)), true
		}
	case kindAnnotation:
		a := c.Graph.Annotations[ref.id]
		aw := c.annotations[ref.id]
		if a == nil || aw == nil {
			break
		}
		eff := aw.effectiveSize()
		rect := geo.Rectangle{
			Pos:  c.graphToScreen(geo.NewPos(a.Position[0], a.Position[1])),
			Size: geo.NewSize(eff.Width*c.Zoom, eff.Height*c.Zoom),
		}
		return rectCenter(rect), true
	}
	return geo.Position{}, false
}

var _ event.Tabbable = (*DAGCanvas)(nil)
