package dagwidget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// GroupEdgeWidget renders a group dependency as a straight dashed arrow
// between the borders of two group rectangles. Unlike data-flow edges it has
// no ports, no stream label, and no bezier curve — a dependency reads as a
// different kind of line. It implements no event interfaces; selection is
// hit-tested from DAGCanvas.Tapped/TappedSecondary like bezier edges.
type GroupEdgeWidget struct {
	widget.BaseWidget

	EdgeID   string
	Source   string // group ID
	Target   string // group ID
	Selected bool

	// Computed screen endpoints (border points of the two group rects).
	p0, p1 geo.Position

	dagCanvas *DAGCanvas
}

var _ core.Widget = (*GroupEdgeWidget)(nil)

// NewGroupEdgeWidget creates a widget for a dag.GroupEdge.
func NewGroupEdgeWidget(id, src, tgt string, dc *DAGCanvas) *GroupEdgeWidget {
	e := &GroupEdgeWidget{
		EdgeID:    id,
		Source:    src,
		Target:    tgt,
		dagCanvas: dc,
	}
	e.ExtendBaseWidget(e)
	return e
}

// updateEndpoints recalculates the border-to-border segment from the two
// groups' current screen rectangles.
func (e *GroupEdgeWidget) updateEndpoints() {
	c := e.dagCanvas
	src := c.Graph.Groups[e.Source]
	tgt := c.Graph.Groups[e.Target]
	if src == nil || tgt == nil {
		e.Hide()
		return
	}
	srcRect := c.groupScreenRect(src)
	tgtRect := c.groupScreenRect(tgt)
	srcCenter := rectCenter(srcRect)
	tgtCenter := rectCenter(tgtRect)
	e.p0 = rectBorderPoint(srcRect, tgtCenter)
	e.p1 = rectBorderPoint(tgtRect, srcCenter)
	e.Show()
	e.Refresh()
}

func (e *GroupEdgeWidget) MinSize() geo.Size {
	// Drawn in canvas coordinates; size is managed by the parent.
	return geo.NewSize(0, 0)
}

func (e *GroupEdgeWidget) CreateRenderer() core.WidgetRenderer {
	return &groupEdgeRenderer{
		edge:  e,
		line:  &canvas.Path{},
		arrow: [2]*canvas.Line{canvas.NewLine(color.Transparent), canvas.NewLine(color.Transparent)},
	}
}

type groupEdgeRenderer struct {
	edge  *GroupEdgeWidget
	line  *canvas.Path
	arrow [2]*canvas.Line
}

func (r *groupEdgeRenderer) Layout(geo.Size) {}

func (r *groupEdgeRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *groupEdgeRenderer) Refresh() {
	e := r.edge
	c := e.dagCanvas
	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	z := float32(1.0)
	if c != nil {
		z = c.Zoom
	}

	col := th.Color(theme.ColorNameOutline, v)
	width := 2 * z
	if e.Selected {
		col = th.Color(theme.ColorNamePrimary, v)
		width = 3 * z
	}

	// Keep the arrowhead tip just off the border so the wings don't overlap
	// the group stroke.
	tip := e.p1
	r.line.Reset()
	r.line.MoveTo(e.p0.X, e.p0.Y)
	r.line.LineTo(tip.X, tip.Y)
	r.line.StrokeColor = col
	r.line.StrokeWidth = width
	r.line.FillColor = color.Transparent
	r.line.DashPattern = []float32{6 * z, 4 * z}
	r.line.Refresh()

	wingL, wingR := ArrowheadPoints(e.p0, tip, arrowSize*z)
	for i, wing := range []geo.Position{wingL, wingR} {
		a := r.arrow[i]
		a.Position1 = tip
		a.Position2 = wing
		a.StrokeColor = col
		a.StrokeWidth = width
		a.Refresh()
	}
}

func (r *groupEdgeRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.line, r.arrow[0], r.arrow[1]}
}

func (r *groupEdgeRenderer) Destroy() {}
