package dagwidget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

type dagCanvasRenderer struct {
	dc *DAGCanvas
	bg *canvas.Rectangle

	// Cached object list — rebuilt only when dirty
	cachedObjects []core.CanvasObject
	dirty         bool

	// Ghost edge lines for pending connection
	ghostLines []*canvas.Line
	ghostArrow [2]*canvas.Line

	// Ghost dashed line for a pending group dependency
	ghostGroupLine  *canvas.Path
	ghostGroupArrow [2]*canvas.Line

	// Ghost solid line for a pending link edge
	ghostLinkLine  *canvas.Path
	ghostLinkArrow [2]*canvas.Line
}

// onScreen reports whether a screen rect intersects the viewport, with a
// margin so partially visible elements never pop in late.
func onScreen(pos geo.Position, size geo.Size, viewport geo.Size) bool {
	const margin = float32(100)
	return pos.X+size.Width > -margin &&
		pos.X < viewport.Width+margin &&
		pos.Y+size.Height > -margin &&
		pos.Y < viewport.Height+margin
}

func (r *dagCanvasRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	c := r.dc

	// Nodes hidden inside collapsed groups, mapped to the hiding group ID.
	hiddenNode := make(map[string]string)
	for _, g := range c.Graph.Groups {
		if !g.Collapsed {
			continue
		}
		for _, m := range g.Members {
			hiddenNode[m] = g.ID
		}
	}

	// Capability-gated affordances (connector handles, resize handles)
	// follow Capabilities from here: child renderers also gate them in their
	// Refresh, but a canvas Refresh re-lays out children without cascading
	// Refresh to them (and Move/Resize short-circuit when geometry is
	// unchanged), so this loop is what makes a Capabilities change actually
	// take effect.
	for _, gw := range c.groups {
		if c.Capabilities.GroupEdges {
			gw.connector.Show()
		} else {
			gw.connector.Hide()
		}
		setHandlesVisible(gw.handles, gw.handlesVisible())
	}
	for _, nw := range c.nodes {
		if c.Capabilities.LinkEdges {
			nw.connector.Show()
		} else {
			nw.connector.Hide()
		}
		setHandlesVisible(nw.handles, nw.handlesVisible())
	}
	for _, aw := range c.annotations {
		setHandlesVisible(aw.handles, aw.handlesVisible())
	}

	// Groups (below everything else) with offscreen culling.
	for id, gw := range c.groups {
		g := c.Graph.Groups[id]
		if g == nil {
			continue
		}
		rect := c.groupScreenRect(g)
		if onScreen(rect.Pos, rect.Size, size) {
			gw.Show()
			gw.Move(rect.Pos)
			gw.Resize(rect.Size)
		} else {
			gw.Hide()
		}
	}

	// Nodes with offscreen culling; members of collapsed groups are hidden.
	for id, nw := range c.nodes {
		node := c.Graph.Nodes[id]
		if node == nil {
			continue
		}
		if _, hidden := hiddenNode[id]; hidden {
			nw.Hide()
			continue
		}
		graphPos := geo.NewPos(node.Position[0], node.Position[1])
		screenPos := c.graphToScreen(graphPos)
		effSize := nw.effectiveSize()
		nodeSize := geo.NewSize(effSize.Width*c.Zoom, effSize.Height*c.Zoom)

		if onScreen(screenPos, nodeSize, size) {
			nw.Show()
			nw.Move(screenPos)
			nw.Resize(nodeSize)
		} else {
			nw.Hide()
		}
	}

	// Edges: hide those entirely inside one collapsed group; the rest attach
	// to port centers or, for hidden endpoints, the collapsed card's border.
	for _, ew := range c.edges {
		srcGroup, srcHidden := hiddenNode[ew.Source.NodeID]
		tgtGroup, tgtHidden := hiddenNode[ew.Target.NodeID]
		if srcHidden && tgtHidden && srcGroup == tgtGroup {
			ew.Hide()
			continue
		}
		ew.Show()
		ew.updateEndpoints()
	}

	// Group dependency edges.
	for _, ew := range c.groupEdges {
		ew.updateEndpoints()
	}

	// Link edges (node↔node, port-less).
	for _, ew := range c.linkEdges {
		ew.updateEndpoints()
	}

	// Annotations on top, then their leader lines (which need final node,
	// group, and edge geometry).
	for id, aw := range c.annotations {
		a := c.Graph.Annotations[id]
		if a == nil {
			continue
		}
		screenPos := c.graphToScreen(geo.NewPos(a.Position[0], a.Position[1]))
		eff := aw.effectiveSize()
		boxSize := geo.NewSize(eff.Width*c.Zoom, eff.Height*c.Zoom)
		if onScreen(screenPos, boxSize, size) {
			aw.Show()
			aw.Move(screenPos)
			aw.Resize(boxSize)
			aw.updateLeader()
		} else {
			aw.Hide()
		}
	}

	r.updateGhostEdge()
	r.updateGhostGroupEdge()
	r.updateGhostLinkEdge()
}

func (r *dagCanvasRenderer) MinSize() geo.Size {
	return geo.NewSize(200, 200)
}

func (r *dagCanvasRenderer) Refresh() {
	r.dirty = true // structural changes require object list rebuild
	r.applyTheme()
	r.Layout(r.dc.Size())
}

func (r *dagCanvasRenderer) Objects() []core.CanvasObject {
	if !r.dirty && r.cachedObjects != nil {
		return r.cachedObjects
	}

	c := r.dc
	// Draw order: bg → group bodies → group edges → data-flow edges →
	// link edges → ghosts → nodes → annotations. Groups are backgrounds;
	// dependency arrows sit above group fills but below data flow; link
	// edges sit with data flow (they connect nodes); annotations are
	// topmost so their text always reads.
	cap := 4 + len(c.groups) + len(c.groupEdges) + len(c.edges) +
		len(c.linkEdges) + len(r.ghostLines) + len(c.nodes) + len(c.annotations)
	objs := make([]core.CanvasObject, 0, cap)
	objs = append(objs, r.bg)

	for _, gw := range c.groups {
		objs = append(objs, gw)
	}
	for _, ew := range c.groupEdges {
		objs = append(objs, ew)
	}
	for _, ew := range c.edges {
		objs = append(objs, ew)
	}
	for _, ew := range c.linkEdges {
		objs = append(objs, ew)
	}

	// Ghost edge lines
	for _, l := range r.ghostLines {
		objs = append(objs, l)
	}
	for _, l := range r.ghostArrow {
		if l != nil {
			objs = append(objs, l)
		}
	}
	if r.ghostGroupLine != nil {
		objs = append(objs, r.ghostGroupLine)
		for _, l := range r.ghostGroupArrow {
			if l != nil {
				objs = append(objs, l)
			}
		}
	}
	if r.ghostLinkLine != nil {
		objs = append(objs, r.ghostLinkLine)
		for _, l := range r.ghostLinkArrow {
			if l != nil {
				objs = append(objs, l)
			}
		}
	}

	for _, nw := range c.nodes {
		objs = append(objs, nw)
	}
	for _, aw := range c.annotations {
		objs = append(objs, aw)
	}

	r.cachedObjects = objs
	r.dirty = false
	return objs
}

func (r *dagCanvasRenderer) Destroy() {}

func (r *dagCanvasRenderer) applyTheme() {
	th := r.dc.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameBackground, v)
	r.bg.Refresh()
}

func (r *dagCanvasRenderer) updateGhostEdge() {
	c := r.dc
	if c.pendingEdge == nil {
		for _, l := range r.ghostLines {
			l.Hide()
		}
		for _, l := range r.ghostArrow {
			if l != nil {
				l.Hide()
			}
		}
		return
	}

	src := c.pendingEdge.sourcePos
	tgt := c.pendingEdge.currentPos
	cp1, cp2 := ComputeControlPoints(src, tgt)
	n := 24
	points := FlattenCubicBezier(src, cp1, cp2, tgt, n)

	th := c.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	col := withAlpha(th.Color(theme.ColorNamePrimary, v), 128)

	// Grow ghost line pool if needed
	for len(r.ghostLines) < n {
		l := canvas.NewLine(color.Transparent)
		r.ghostLines = append(r.ghostLines, l)
		r.dirty = true // new objects added
	}

	for i := 0; i < n; i++ {
		l := r.ghostLines[i]
		l.Position1 = points[i]
		l.Position2 = points[i+1]
		l.StrokeColor = col
		l.StrokeWidth = 2
		l.Show()
		l.Refresh()
	}
	for i := n; i < len(r.ghostLines); i++ {
		r.ghostLines[i].Hide()
	}

	// Arrowhead on the ghost edge so the connection direction reads during the drag.
	if r.ghostArrow[0] == nil {
		r.ghostArrow[0] = canvas.NewLine(color.Transparent)
		r.ghostArrow[1] = canvas.NewLine(color.Transparent)
		r.dirty = true
	}
	wingL, wingR := ArrowheadPoints(points[n-1], tgt, arrowSize*c.Zoom)
	for i, wing := range []geo.Position{wingL, wingR} {
		a := r.ghostArrow[i]
		a.Position1 = tgt
		a.Position2 = wing
		a.StrokeColor = col
		a.StrokeWidth = 2
		a.Show()
		a.Refresh()
	}
}

// updateGhostGroupEdge renders the dashed preview while dragging a group
// dependency from a connector handle.
func (r *dagCanvasRenderer) updateGhostGroupEdge() {
	c := r.dc
	if r.ghostGroupLine == nil {
		r.ghostGroupLine = &canvas.Path{FillColor: color.Transparent}
		r.ghostGroupArrow[0] = canvas.NewLine(color.Transparent)
		r.ghostGroupArrow[1] = canvas.NewLine(color.Transparent)
		r.dirty = true
	}
	pending := c.pendingGroupLink
	if pending == nil {
		r.ghostGroupLine.Hide()
		r.ghostGroupArrow[0].Hide()
		r.ghostGroupArrow[1].Hide()
		return
	}

	th := c.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	col := withAlpha(th.Color(theme.ColorNamePrimary, v), 128)
	z := c.Zoom

	src := pending.sourcePos
	tgt := pending.currentPos
	r.ghostGroupLine.Reset()
	r.ghostGroupLine.MoveTo(src.X, src.Y)
	r.ghostGroupLine.LineTo(tgt.X, tgt.Y)
	r.ghostGroupLine.StrokeColor = col
	r.ghostGroupLine.StrokeWidth = 2
	r.ghostGroupLine.DashPattern = []float32{6 * z, 4 * z}
	r.ghostGroupLine.Show()
	r.ghostGroupLine.Refresh()

	wingL, wingR := ArrowheadPoints(src, tgt, arrowSize*z)
	for i, wing := range []geo.Position{wingL, wingR} {
		a := r.ghostGroupArrow[i]
		a.Position1 = tgt
		a.Position2 = wing
		a.StrokeColor = col
		a.StrokeWidth = 2
		a.Show()
		a.Refresh()
	}
}

// updateGhostLinkEdge renders the solid straight preview while dragging a
// link edge from a node's connector handle (solid, matching how link edges
// render — the dashed ghost belongs to group dependencies).
func (r *dagCanvasRenderer) updateGhostLinkEdge() {
	c := r.dc
	if r.ghostLinkLine == nil {
		r.ghostLinkLine = &canvas.Path{FillColor: color.Transparent}
		r.ghostLinkArrow[0] = canvas.NewLine(color.Transparent)
		r.ghostLinkArrow[1] = canvas.NewLine(color.Transparent)
		r.dirty = true
	}
	pending := c.pendingNodeLink
	if pending == nil {
		r.ghostLinkLine.Hide()
		r.ghostLinkArrow[0].Hide()
		r.ghostLinkArrow[1].Hide()
		return
	}

	th := c.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	col := withAlpha(th.Color(theme.ColorNamePrimary, v), 128)

	src := pending.sourcePos
	tgt := pending.currentPos
	r.ghostLinkLine.Reset()
	r.ghostLinkLine.MoveTo(src.X, src.Y)
	r.ghostLinkLine.LineTo(tgt.X, tgt.Y)
	r.ghostLinkLine.StrokeColor = col
	r.ghostLinkLine.StrokeWidth = 2
	r.ghostLinkLine.Show()
	r.ghostLinkLine.Refresh()

	wingL, wingR := ArrowheadPoints(src, tgt, arrowSize*c.Zoom)
	for i, wing := range []geo.Position{wingL, wingR} {
		a := r.ghostLinkArrow[i]
		a.Position1 = tgt
		a.Position2 = wing
		a.StrokeColor = col
		a.StrokeWidth = 2
		a.Show()
		a.Refresh()
	}
}
