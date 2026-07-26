package dagwidget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

const defaultSegments = 32

// arrowSize is the arrowhead wing length in graph units (scaled by zoom).
const arrowSize float32 = 9

// edgeLabelTextSize is the label font size in graph units (scaled by zoom).
const edgeLabelTextSize float32 = 11

type edgeRenderer struct {
	edge    *EdgeWidget
	lines   []*canvas.Line
	arrow   [2]*canvas.Line
	label   *canvas.Text
	labelBG *canvas.Rectangle
}

func (r *edgeRenderer) Layout(size geo.Size) {
	// Lines are positioned absolutely; no layout needed
}

func (r *edgeRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *edgeRenderer) Refresh() {
	e := r.edge
	th := e.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	col := th.Color(theme.ColorNameOutline, v)
	width := float32(2.0)
	if e.Selected {
		col = th.Color(theme.ColorNamePrimary, v)
		width = 3.0
	}
	if e.Hovered && !e.Selected {
		col = th.Color(theme.ColorNameForeground, v)
	}

	n := defaultSegments
	zoom := float32(1.0)
	if e.dagCanvas != nil {
		n = AdaptiveSegmentCount(e.p0, e.p3, e.dagCanvas.Zoom)
		zoom = e.dagCanvas.Zoom
	}

	points := FlattenCubicBezier(e.p0, e.p1, e.p2, e.p3, n)
	needed := len(points) - 1

	// Grow line pool if needed
	for len(r.lines) < needed {
		l := &canvas.Line{}
		r.lines = append(r.lines, l)
	}

	for i := 0; i < needed; i++ {
		l := r.lines[i]
		l.Position1 = points[i]
		l.Position2 = points[i+1]
		l.StrokeColor = col
		l.StrokeWidth = width
		l.Show()
		l.Refresh()
	}
	// Hide excess lines
	for i := needed; i < len(r.lines); i++ {
		r.lines[i].Hide()
	}

	// Arrowhead at the target end, aligned with the curve tangent. The tip is
	// pulled back so it meets the edge of the target port circle, not its center.
	if r.arrow[0] == nil {
		r.arrow[0] = &canvas.Line{}
		r.arrow[1] = &canvas.Line{}
	}
	tip := pullBack(points[needed], points[needed-1], portSize/2*zoom)
	wingL, wingR := ArrowheadPoints(points[needed-1], tip, arrowSize*zoom)
	for i, wing := range []geo.Position{wingL, wingR} {
		a := r.arrow[i]
		a.Position1 = tip
		a.Position2 = wing
		a.StrokeColor = col
		a.StrokeWidth = width
		a.Show()
		a.Refresh()
	}

	r.refreshLabel(points, zoom)
}

// refreshLabel positions the midpoint label. The label stays upright (no
// rotation along the tangent) and tracks the curve as nodes move and the
// canvas pans/zooms; a rounded background keeps it readable over the line.
func (r *edgeRenderer) refreshLabel(points []geo.Position, zoom float32) {
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

	mid := points[len(points)/2]
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

func (r *edgeRenderer) Objects() []core.CanvasObject {
	objs := make([]core.CanvasObject, 0, len(r.lines)+2)
	for _, l := range r.lines {
		objs = append(objs, l)
	}
	if r.arrow[0] != nil {
		objs = append(objs, r.arrow[0], r.arrow[1])
	}
	if r.label != nil {
		objs = append(objs, r.labelBG, r.label)
	}
	return objs
}

func (r *edgeRenderer) Destroy() {}
