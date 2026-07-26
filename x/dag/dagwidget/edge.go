package dagwidget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// EdgeWidget renders a bezier curve connection between two ports.
type EdgeWidget struct {
	widget.BaseWidget

	EdgeID   string
	Source   dag.PortID
	Target   dag.PortID
	Selected bool
	Hovered  bool

	// Label is an optional annotation drawn at the curve midpoint (kept
	// upright, tracking pan/zoom). When empty, the source port's stream type
	// is shown instead if the canvas has ShowStreamTypes enabled.
	Label string

	// Computed bezier control points (screen coords)
	p0, p1, p2, p3 geo.Position

	dagCanvas *DAGCanvas
}

var _ core.Widget = (*EdgeWidget)(nil)

// NewEdgeWidget creates a new edge connection widget.
func NewEdgeWidget(id string, src, tgt dag.PortID, dc *DAGCanvas) *EdgeWidget {
	e := &EdgeWidget{
		EdgeID:    id,
		Source:    src,
		Target:    tgt,
		dagCanvas: dc,
	}
	e.ExtendBaseWidget(e)
	return e
}

// updateEndpoints recalculates bezier control points from current port
// positions. When an endpoint's node is hidden inside a collapsed group the
// curve attaches to the group card's border instead, aimed at the opposite
// endpoint so the crossing edge stays readable.
func (e *EdgeWidget) updateEndpoints() {
	c := e.dagCanvas

	// Raw endpoint centers first, so each border anchor can aim at the
	// other side. Model geometry only — see portCenterScreen for why the
	// painted port-widget positions must not be used here.
	rawFor := func(pid dag.PortID) (geo.Position, bool) {
		if g := c.collapsedGroupOf(pid.NodeID); g != nil {
			return rectCenter(c.groupScreenRect(g)), true
		}
		return c.portCenterScreen(pid)
	}
	srcRaw, okS := rawFor(e.Source)
	tgtRaw, okT := rawFor(e.Target)
	if !okS || !okT {
		return
	}

	p0, okS := c.visibleAnchorForPort(e.Source, tgtRaw)
	p3, okT := c.visibleAnchorForPort(e.Target, srcRaw)
	if !okS || !okT {
		return
	}
	e.p0, e.p3 = p0, p3
	e.p1, e.p2 = ComputeControlPoints(e.p0, e.p3)
	e.Refresh()
}

// labelText returns the text to display on this edge: the explicit Label if
// set, otherwise the source stream type when the canvas opts in via ShowStreamTypes.
func (e *EdgeWidget) labelText() string {
	if e.Label != "" {
		return e.Label
	}
	c := e.dagCanvas
	if c == nil || !c.ShowStreamTypes {
		return ""
	}
	srcNode := c.Graph.Nodes[e.Source.NodeID]
	if srcNode == nil {
		return ""
	}
	if r := c.Graph.Registry; r != nil {
		if st, ok := r.PortStreamType(srcNode, e.Source.Name, dag.PortOutput); ok {
			return st.Name
		}
	}
	// No registry (or unknown component): fall back to the port's type tag.
	for _, p := range srcNode.Outputs {
		if p.Name == e.Source.Name {
			return p.TypeTag
		}
	}
	return ""
}

func (e *EdgeWidget) CreateRenderer() core.WidgetRenderer {
	r := &edgeRenderer{
		edge: e,
	}
	return r
}

func (e *EdgeWidget) MinSize() geo.Size {
	// Edge covers the entire canvas; size is managed by the parent
	return geo.NewSize(0, 0)
}
