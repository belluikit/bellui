package dagwidget

import (
	"image/color"
	"math"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// linkEdgeLaneSpacing is the perpendicular distance between neighboring link
// edge lanes in graph units (scaled by zoom at render time).
const linkEdgeLaneSpacing float32 = 12

// linkEdgeLabelStagger is the per-rank shift of a lane's midpoint label along
// the segment, as a fraction of the segment length. Neighboring lanes sit
// only linkEdgeLaneSpacing apart — less than a label background's height —
// so laned labels also fan out along the pair's canonical axis.
const linkEdgeLabelStagger float32 = 0.15

// LinkEdgeWidget renders a link edge as a straight arrow between the borders
// of two node cards, with an optional midpoint label. Unlike data-flow edges
// it has no ports and no bezier curve — a port-less relation reads as a
// different kind of line. It implements no event interfaces; selection is
// hit-tested from DAGCanvas.Tapped/TappedSecondary like bezier edges.
type LinkEdgeWidget struct {
	widget.BaseWidget

	EdgeID   string
	Source   string // node ID
	Target   string // node ID
	Selected bool
	// Dashed switches the line style. The default is solid so link edges read
	// differently from dashed group dependency edges.
	Dashed bool

	// Computed screen endpoints (border points of the two node rects).
	p0, p1 geo.Position
	// labelT is the fraction along p0→p1 where the label sits (0.5 for a
	// lone edge; laned edges fan out — see laneLabelT). Set by
	// updateEndpoints alongside the endpoints.
	labelT float32

	dagCanvas *DAGCanvas
}

var _ core.Widget = (*LinkEdgeWidget)(nil)

// NewLinkEdgeWidget creates a widget for a dag.LinkEdge.
func NewLinkEdgeWidget(id, src, tgt string, dc *DAGCanvas) *LinkEdgeWidget {
	e := &LinkEdgeWidget{
		EdgeID:    id,
		Source:    src,
		Target:    tgt,
		labelT:    0.5,
		dagCanvas: dc,
	}
	e.ExtendBaseWidget(e)
	return e
}

// updateEndpoints recalculates the border-to-border segment from the two
// nodes' current screen rectangles. Endpoints hidden inside a collapsed group
// re-anchor to the card border; when both nodes sit in the same collapsed
// group the edge hides entirely (mirroring intra-group data-flow edges).
// Edges sharing an unordered node pair are geometrically coincident, so each
// is shifted onto its own lane (laneOffset) — labels stay apart and
// hit-testing can tell the edges of a pair apart.
func (e *LinkEdgeWidget) updateEndpoints() {
	c := e.dagCanvas
	if g := c.collapsedGroupOf(e.Source); g != nil && g == c.collapsedGroupOf(e.Target) {
		e.Hide()
		return
	}
	srcRect, ok0 := c.nodeVisibleScreenRect(e.Source)
	tgtRect, ok1 := c.nodeVisibleScreenRect(e.Target)
	if !ok0 || !ok1 {
		e.Hide()
		return
	}
	srcCenter, tgtCenter := rectCenter(srcRect), rectCenter(tgtRect)
	rank, count := e.laneRank()
	dx, dy := e.laneOffset(rank, count, srcCenter, tgtCenter, srcRect, tgtRect)
	e.labelT = laneLabelT(rank, count, e.Source > e.Target)
	from := geo.NewPos(srcCenter.X+dx, srcCenter.Y+dy)
	toward := geo.NewPos(tgtCenter.X+dx, tgtCenter.Y+dy)
	e.p0 = rectBorderPointFrom(srcRect, from, toward)
	e.p1 = rectBorderPointFrom(tgtRect, toward, from)
	e.Show()
	e.Refresh()
}

// laneRank returns this edge's deterministic position (sorted by ID) among
// the link edges sharing its unordered node pair, and how many there are.
// The common case — no coincident sibling — is (0, 1).
func (e *LinkEdgeWidget) laneRank() (rank, count int) {
	var ids []string
	for id, le := range e.dagCanvas.Graph.LinkEdges {
		if (le.Source == e.Source && le.Target == e.Target) ||
			(le.Source == e.Target && le.Target == e.Source) {
			ids = append(ids, id)
		}
	}
	if len(ids) < 2 {
		return 0, 1
	}
	sort.Strings(ids)
	for i, id := range ids {
		if id == e.EdgeID {
			return i, len(ids)
		}
	}
	return 0, 1
}

// laneOffset returns the perpendicular screen-space shift for this edge's
// lane. Parallel and opposite-direction link edges between the same node pair
// are legal in the model but coincide as segments; each edge is offset by
// rank around the center line. The lane axis derives from the canonical node
// order (lower ID → higher), so both directions of a pair agree on it. A
// single edge — the common case — gets a zero offset. When the fan would push
// a shifted ray origin outside a node rect, the spacing compresses so the
// outermost lanes stay distinct instead of saturating against a clamp.
func (e *LinkEdgeWidget) laneOffset(rank, count int, srcCenter, tgtCenter geo.Position, srcRect, tgtRect geo.Rectangle) (dx, dy float32) {
	if count < 2 {
		return 0, 0
	}
	spacing := linkEdgeLaneSpacing * e.dagCanvas.Zoom
	maxOff := 0.4 * min32(
		min32(srcRect.Size.Width, srcRect.Size.Height),
		min32(tgtRect.Size.Width, tgtRect.Size.Height),
	)
	if spread := float32(count-1) * spacing; spread > 2*maxOff {
		spacing = 2 * maxOff / float32(count-1)
	}
	offset := (float32(rank) - float32(count-1)/2) * spacing

	from, to := srcCenter, tgtCenter
	if e.Source > e.Target {
		from, to = to, from
	}
	dirX, dirY := to.X-from.X, to.Y-from.Y
	length := float32(math.Hypot(float64(dirX), float64(dirY)))
	if length == 0 {
		return 0, 0
	}
	return -dirY / length * offset, dirX / length * offset
}

// laneLabelT returns the fraction along the edge's own p0→p1 segment where
// its label sits. Lanes fan their labels out around the midpoint of the
// pair's canonical axis in linkEdgeLabelStagger steps — perpendicular lane
// spacing alone is smaller than a label background's height, so neighboring
// labeled lanes would overprint. Reversed edges (Source > Target) flip the
// fraction so all edges of a pair share one canonical fan-out.
func laneLabelT(rank, count int, reversed bool) float32 {
	t := 0.5 + (float32(rank)-float32(count-1)/2)*linkEdgeLabelStagger
	t = max32(0.2, min32(t, 0.8))
	if reversed {
		t = 1 - t
	}
	return t
}

// labelText returns the midpoint label from the model, or "".
func (e *LinkEdgeWidget) labelText() string {
	if e.dagCanvas == nil {
		return ""
	}
	le := e.dagCanvas.Graph.LinkEdges[e.EdgeID]
	if le == nil {
		return ""
	}
	return le.Label
}

func (e *LinkEdgeWidget) MinSize() geo.Size {
	// Drawn in canvas coordinates; size is managed by the parent.
	return geo.NewSize(0, 0)
}

func (e *LinkEdgeWidget) CreateRenderer() core.WidgetRenderer {
	return &linkEdgeRenderer{
		edge:  e,
		line:  &canvas.Path{},
		arrow: [2]*canvas.Line{canvas.NewLine(color.Transparent), canvas.NewLine(color.Transparent)},
	}
}

type linkEdgeRenderer struct {
	edge    *LinkEdgeWidget
	line    *canvas.Path
	arrow   [2]*canvas.Line
	label   *canvas.Text
	labelBG *canvas.Rectangle
}

func (r *linkEdgeRenderer) Layout(geo.Size) {}

func (r *linkEdgeRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *linkEdgeRenderer) Refresh() {
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

	tip := e.p1
	r.line.Reset()
	r.line.MoveTo(e.p0.X, e.p0.Y)
	r.line.LineTo(tip.X, tip.Y)
	r.line.StrokeColor = col
	r.line.StrokeWidth = width
	r.line.FillColor = color.Transparent
	if e.Dashed {
		r.line.DashPattern = []float32{6 * z, 4 * z}
	} else {
		r.line.DashPattern = nil
	}
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

	r.refreshLabel(z)
}

// refreshLabel positions the optional midpoint label; a rounded background
// keeps it readable over the line (same treatment as data-flow edge labels).
func (r *linkEdgeRenderer) refreshLabel(zoom float32) {
	txt := r.edge.labelText()
	if txt == "" {
		if r.label != nil {
			r.label.Hide()
			r.labelBG.Hide()
		}
		return
	}
	if r.label == nil {
		r.label = &canvas.Text{Alignment: bellui.TextAlignCenter}
		r.labelBG = &canvas.Rectangle{}
	}

	th := r.edge.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	e := r.edge
	mid := geo.NewPos(
		e.p0.X+(e.p1.X-e.p0.X)*e.labelT,
		e.p0.Y+(e.p1.Y-e.p0.Y)*e.labelT,
	)
	size := edgeLabelTextSize * zoom
	r.label.Text = txt
	r.label.TextSize = size
	r.label.Color = th.Color(theme.ColorNameForeground, v)

	tsize := bellui.MeasureText(txt, size, r.label.TextStyle)
	padX := 6 * zoom
	padY := 2 * zoom
	r.labelBG.FillColor = withAlpha(th.Color(theme.ColorNameBackground, v), 220)
	r.labelBG.CornerRadius = 4 * zoom
	r.labelBG.Move(geo.NewPos(mid.X-tsize.Width/2-padX, mid.Y-tsize.Height/2-padY))
	r.labelBG.Resize(geo.NewSize(tsize.Width+padX*2, tsize.Height+padY*2))
	r.label.Move(geo.NewPos(mid.X-tsize.Width/2, mid.Y-tsize.Height/2))
	r.label.Resize(tsize)

	r.labelBG.Show()
	r.label.Show()
	r.labelBG.Refresh()
	r.label.Refresh()
}

func (r *linkEdgeRenderer) Objects() []core.CanvasObject {
	objs := []core.CanvasObject{r.line, r.arrow[0], r.arrow[1]}
	if r.label != nil {
		objs = append(objs, r.labelBG, r.label)
	}
	return objs
}

func (r *linkEdgeRenderer) Destroy() {}
