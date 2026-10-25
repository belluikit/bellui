package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/mathutil"
	"github.com/belluikit/bellui/x/charts/internal/pool"
)

// LineSeries configures a line chart series.
// Use math.NaN() in Data to represent null/missing values.
type LineSeries struct {
	SeriesName string
	Data       []float64

	XAxisIndex int
	YAxisIndex int

	Symbol     SymbolType
	SymbolSize float32
	Smooth     bool
	Step       StepType
	Stack      string // stack group name

	// ConnectNulls draws dashed lines across null gaps instead of breaking the line.
	ConnectNulls bool

	LineStyle LineStyle
	AreaStyle *AreaStyle // nil = no area fill
}

// SeriesType returns TypeLine.
func (s *LineSeries) SeriesType() Type { return TypeLine }

// Name returns the series display name.
func (s *LineSeries) Name() string { return s.SeriesName }

func (s *LineSeries) sealed() {}

// SeriesColor returns the colour set for the series, or nil for the palette
// colour (see ColoredSeries).
//
// Since: 1.3
func (s *LineSeries) SeriesColor() color.Color { return s.LineStyle.Color }

// GetYAxisIndex returns the Y axis index for this series.
func (s *LineSeries) GetYAxisIndex() int { return s.YAxisIndex }

// StackGroup returns the stack group name.
func (s *LineSeries) StackGroup() string { return s.Stack }

// StackData returns the data used for stack offset calculations.
func (s *LineSeries) StackData() []float64 { return s.Data }

// RegisterHitAreas registers point-based hit areas for the line series.
func (s *LineSeries) RegisterHitAreas(collector HitAreaCollector, seriesIndex int, coordSys coord.System, plotRect coord.Rect) {
	points := make([]HitPoint, 0, len(s.Data))
	for i, v := range s.Data {
		if math.IsNaN(v) {
			continue
		}
		pos := coordSys.DataToPixel(float64(i), v)
		points = append(points, HitPoint{
			DataIndex: i,
			PixelPos:  pos,
			Value:     HitValue{X: float64(i), Y: v},
		})
	}
	collector.AddHitPoints(seriesIndex, s.SeriesName, TypeLine, points)
}

// AppendValues appends data points to the line series.
func (s *LineSeries) AppendValues(values ...float64) {
	s.Data = append(s.Data, values...)
}

// DataRange returns the data extent for axis scaling.
func (s *LineSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
	if len(s.Data) == 0 {
		return 0, 0, 0, 0
	}
	xMin, xMax = 0, float64(len(s.Data)-1)
	yMin, yMax = math.Inf(1), math.Inf(-1)
	for _, v := range s.Data {
		if math.IsNaN(v) {
			continue
		}
		if v < yMin {
			yMin = v
		}
		if v > yMax {
			yMax = v
		}
	}
	return
}

// lineRenderer renders a LineSeries as canvas objects.
type lineRenderer struct{}

func (r *lineRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ls, ok := ctx.Series.(*LineSeries)
	if !ok || len(ls.Data) == 0 {
		return nil
	}

	var objects []core.CanvasObject
	lineColor := ResolveColor(ls.LineStyle.Color, ctx.Color)
	lineWidth := ResolveFloat32(ls.LineStyle.Width, DefaultLineWidth)
	dashLen, gapLen := resolveDash(ls.LineStyle.Dash)
	dashed := dashLen > 0

	symbolSize := ResolveFloat32(ls.SymbolSize, DefaultLineSymbolSize)

	// Auto-downsample via LTTB if data exceeds pixel width
	data := ls.Data
	dataIndices := make([]int, len(data))
	for i := range dataIndices {
		dataIndices[i] = i
	}
	threshold := int(ctx.Viewport.W)
	if threshold < 10 {
		threshold = 10
	}
	if len(data) > threshold {
		data, dataIndices = mathutil.DownsampleFloat64(ls.Data, threshold)
	}

	// Apply stack offset if stacked
	var stackBase []float64
	if ls.Stack != "" && ctx.StackOffsets != nil {
		stackBase = ctx.StackOffsets[ls.Stack]
	}

	// Compute pixel positions for data points (NaN = null/missing)
	points := make([]geo.Position, len(data))
	isNull := make([]bool, len(data))
	for di, y := range data {
		i := dataIndices[di]
		if math.IsNaN(y) {
			isNull[di] = true
			points[di] = geo.NewPos(0, 0)
			continue
		}
		offset := 0.0
		if stackBase != nil && i < len(stackBase) {
			offset = stackBase[i]
		}
		pos := ctx.Coord.DataToPixel(float64(i), y+offset)
		if ctx.Animate && ctx.AnimProgress < 1 {
			basePos := ctx.Coord.DataToPixel(float64(i), 0)
			pos.Y = mathutil.LerpF32(basePos.Y, pos.Y, ctx.AnimProgress)
		}
		points[di] = pos
	}

	// Clip bounds
	clipMinX := ctx.Viewport.X
	clipMinY := ctx.Viewport.Y
	clipMaxX := ctx.Viewport.X + ctx.Viewport.W
	clipMaxY := ctx.Viewport.Y + ctx.Viewport.H

	// Draw area fill if configured (clipped to viewport)
	if ls.AreaStyle != nil {
		objects = append(objects, r.renderArea(ctx, ls, points, lineColor, clipMinX, clipMinY, clipMaxX, clipMaxY)...)
	}

	// Draw line segments (clipped to viewport, handling null gaps)
	for i := 1; i < len(points); i++ {
		prevNull := isNull[i-1]
		curNull := isNull[i]

		if prevNull && curNull {
			continue // both null, skip
		}
		if prevNull || curNull {
			// One end is null: if ConnectNulls, find the next/prev non-null
			// and draw a dashed line; otherwise skip this segment
			if ls.ConnectNulls && !prevNull {
				// Find next non-null point to connect to
				for j := i; j < len(points); j++ {
					if !isNull[j] {
						// Draw dashed line from points[i-1] to points[j]
						dashObjs := renderDashedLineClipped(points[i-1], points[j],
							lineColor, lineWidth, 6, 4,
							clipMinX, clipMinY, clipMaxX, clipMaxY)
						objects = append(objects, dashObjs...)
						break
					}
				}
			}
			continue
		}

		p1 := points[i-1]
		p2 := points[i]

		if ls.Step != StepNone {
			mid := r.stepMidpoint(ls.Step, p1, p2)
			if dashed {
				objects = append(objects, renderDashedLineClipped(p1, mid, lineColor, lineWidth, dashLen, gapLen, clipMinX, clipMinY, clipMaxX, clipMaxY)...)
				objects = append(objects, renderDashedLineClipped(mid, p2, lineColor, lineWidth, dashLen, gapLen, clipMinX, clipMinY, clipMaxX, clipMaxY)...)
			} else {
				if seg := newPooledClippedLine(ctx.Pool, p1, mid, lineColor, lineWidth, clipMinX, clipMinY, clipMaxX, clipMaxY); seg != nil {
					objects = append(objects, seg)
				}
				if seg := newPooledClippedLine(ctx.Pool, mid, p2, lineColor, lineWidth, clipMinX, clipMinY, clipMaxX, clipMaxY); seg != nil {
					objects = append(objects, seg)
				}
			}
		} else if dashed {
			objects = append(objects, renderDashedLineClipped(p1, p2, lineColor, lineWidth, dashLen, gapLen, clipMinX, clipMinY, clipMaxX, clipMaxY)...)
		} else {
			if seg := newPooledClippedLine(ctx.Pool, p1, p2, lineColor, lineWidth, clipMinX, clipMinY, clipMaxX, clipMaxY); seg != nil {
				objects = append(objects, seg)
			}
		}
	}

	// Draw symbols at data points (skip nulls, skip if too dense, clip to viewport)
	if ls.Symbol != SymbolNone && len(points) <= 200 {
		half := symbolSize / 2
		for di, pos := range points {
			if isNull[di] {
				continue
			}
			if pos.X+half < clipMinX || pos.X-half > clipMaxX ||
				pos.Y+half < clipMinY || pos.Y-half > clipMaxY {
				continue
			}
			sym := CreatePooledSymbol(ctx.Pool, ls.Symbol, symbolSize, pos, lineColor)
			if sym != nil {
				objects = append(objects, sym)
			}
		}
	}

	return objects
}

func (r *lineRenderer) renderArea(_ RenderContext, ls *LineSeries, points []geo.Position, baseColor color.Color, clipMinX, clipMinY, clipMaxX, clipMaxY float32) []core.CanvasObject {
	if len(points) < 2 || ls.AreaStyle == nil {
		return nil
	}

	opacity := ResolveFloat32(ls.AreaStyle.Opacity, DefaultAreaOpacity)

	areaColor := ls.AreaStyle.Color
	if areaColor == nil {
		areaColor = baseColor
	}

	cr, cg, cb, _ := areaColor.RGBA()
	fillColor := color.NRGBA{
		R: uint8(cr >> 8),
		G: uint8(cg >> 8),
		B: uint8(cb >> 8),
		A: uint8(opacity * 255),
	}

	var objects []core.CanvasObject

	maxY := clipMaxY // use viewport bottom as baseline

	for i := 1; i < len(points); i++ {
		p1 := points[i-1]
		p2 := points[i]

		// Clip X to viewport
		x1 := p1.X
		x2 := p2.X
		if x2 < clipMinX || x1 > clipMaxX {
			continue
		}
		if x1 < clipMinX {
			x1 = clipMinX
		}
		if x2 > clipMaxX {
			x2 = clipMaxX
		}
		w := x2 - x1
		if w <= 0 {
			continue
		}

		topY := p1.Y
		if p2.Y < topY {
			topY = p2.Y
		}
		// Clip Y to viewport
		if topY < clipMinY {
			topY = clipMinY
		}

		rect := canvas.NewRectangle(fillColor)
		rect.Move(geo.NewPos(x1, topY))
		rect.Resize(geo.NewSize(w, maxY-topY))
		objects = append(objects, rect)
	}

	return objects
}

func (r *lineRenderer) stepMidpoint(step StepType, p1, p2 geo.Position) geo.Position {
	switch step {
	case StepStart:
		return geo.NewPos(p1.X, p2.Y)
	case StepEnd:
		return geo.NewPos(p2.X, p1.Y)
	case StepMiddle:
		midX := (p1.X + p2.X) / 2
		return geo.NewPos(midX, p1.Y)
	default:
		return p2
	}
}

func newLine(p1, p2 geo.Position, c color.Color, width float32) *canvas.Line {
	return &canvas.Line{
		Position1:   p1,
		Position2:   p2,
		StrokeColor: c,
		StrokeWidth: width,
	}
}

func newClippedLine(p1, p2 geo.Position, c color.Color, width, clipMinX, clipMinY, clipMaxX, clipMaxY float32) *canvas.Line {
	cx1, cy1, cx2, cy2, visible := mathutil.ClipLineToRect(
		p1.X, p1.Y, p2.X, p2.Y,
		clipMinX, clipMinY, clipMaxX, clipMaxY,
	)
	if !visible {
		return nil
	}
	return &canvas.Line{
		Position1:   geo.NewPos(cx1, cy1),
		Position2:   geo.NewPos(cx2, cy2),
		StrokeColor: c,
		StrokeWidth: width,
	}
}

// newPooledClippedLine is like newClippedLine but uses the pool when available.
func newPooledClippedLine(p *pool.ObjectPool, p1, p2 geo.Position, c color.Color, width, clipMinX, clipMinY, clipMaxX, clipMaxY float32) *canvas.Line {
	cx1, cy1, cx2, cy2, visible := mathutil.ClipLineToRect(
		p1.X, p1.Y, p2.X, p2.Y,
		clipMinX, clipMinY, clipMaxX, clipMaxY,
	)
	if !visible {
		return nil
	}
	if p != nil {
		return p.Line(geo.NewPos(cx1, cy1), geo.NewPos(cx2, cy2), c, width)
	}
	return &canvas.Line{
		Position1:   geo.NewPos(cx1, cy1),
		Position2:   geo.NewPos(cx2, cy2),
		StrokeColor: c,
		StrokeWidth: width,
	}
}

// resolveDash returns the dash and gap lengths from a Dash pattern.
// Returns (0, 0) for a solid line (nil or empty pattern).
func resolveDash(dash []float32) (dashLen, gapLen float32) {
	if len(dash) == 0 {
		return 0, 0
	}
	dashLen = dash[0]
	if dashLen <= 0 {
		return 0, 0
	}
	if len(dash) >= 2 {
		gapLen = dash[1]
	} else {
		gapLen = dashLen
	}
	return dashLen, gapLen
}

// renderDashedLineClipped draws a dashed line clipped to a rectangle.
func renderDashedLineClipped(p1, p2 geo.Position, c color.Color, width, dashLen, gapLen, clipMinX, clipMinY, clipMaxX, clipMaxY float32) []core.CanvasObject {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length == 0 {
		return nil
	}
	ux := dx / length
	uy := dy / length

	var objects []core.CanvasObject
	pos := float32(0)
	for pos < length {
		end := pos + dashLen
		if end > length {
			end = length
		}
		sp := geo.NewPos(p1.X+ux*pos, p1.Y+uy*pos)
		ep := geo.NewPos(p1.X+ux*end, p1.Y+uy*end)
		if seg := newClippedLine(sp, ep, c, width, clipMinX, clipMinY, clipMaxX, clipMaxY); seg != nil {
			objects = append(objects, seg)
		}
		pos = end + gapLen
	}
	return objects
}
