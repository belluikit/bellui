package component

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/style"
)

// OverlayAxis specifies orientation for reference lines/bands.
type OverlayAxis = int

const (
	AxisHorizontal OverlayAxis = iota
	AxisVertical
)

// OverlayReferenceLine describes a reference line to render.
type OverlayReferenceLine struct {
	Axis       OverlayAxis
	Value      float64
	YAxisIndex int
	Color      color.Color
	Width      float32
	Dashed     bool
	// DashPattern overrides the default dash/gap lengths when Dashed is true.
	// For example, {6, 4} draws 6px dashes with 4px gaps. Nil uses default (6, 4).
	DashPattern []float32
	Label       string
}

// OverlayReferenceBand describes a reference band to render.
type OverlayReferenceBand struct {
	Axis       OverlayAxis
	From, To   float64
	YAxisIndex int
	Color      color.Color
	Label      string
}

// OverlayAnnotation describes an annotation to render.
type OverlayAnnotation struct {
	X, Y       float64
	YAxisIndex int
	Text       string
	Color      color.Color
	TextSize   float32
	Arrow      bool
}

// RenderOverlays produces canvas objects for reference lines, bands, and annotations.
// coordSystems provides one coordinate system per Y axis for data-to-pixel mapping.
func RenderOverlays(
	lines []OverlayReferenceLine,
	bands []OverlayReferenceBand,
	annotations []OverlayAnnotation,
	coordSystems []coord.System,
	plotRect coord.Rect,
	resolver *style.Resolver,
) []core.CanvasObject {
	var objects []core.CanvasObject

	for _, rl := range lines {
		objects = append(objects, renderReferenceLine(rl, coordSystems, plotRect, resolver)...)
	}
	for _, rb := range bands {
		objects = append(objects, renderReferenceBand(rb, coordSystems, plotRect, resolver)...)
	}
	for _, ann := range annotations {
		objects = append(objects, renderAnnotation(ann, coordSystems, plotRect, resolver)...)
	}

	return objects
}

func resolverLabelSize(resolver *style.Resolver) float32 {
	if resolver != nil {
		return resolver.LabelTextSize()
	}
	return 11
}

func resolverTitleColor(resolver *style.Resolver) color.Color {
	if resolver != nil {
		return resolver.TitleColor()
	}
	return color.NRGBA{R: 220, G: 220, B: 220, A: 255}
}

func getCoord(coordSystems []coord.System, yAxisIndex int) coord.System {
	if yAxisIndex >= 0 && yAxisIndex < len(coordSystems) {
		return coordSystems[yAxisIndex]
	}
	if len(coordSystems) > 0 {
		return coordSystems[0]
	}
	return nil
}

func renderReferenceLine(rl OverlayReferenceLine, coordSystems []coord.System, plotRect coord.Rect, resolver *style.Resolver) []core.CanvasObject {
	cs := getCoord(coordSystems, rl.YAxisIndex)
	if cs == nil {
		return nil
	}

	c := rl.Color
	if c == nil {
		c = resolver.OverlayDefaultLineColor()
	}
	w := rl.Width
	if w <= 0 {
		w = 1.5
	}

	var objects []core.CanvasObject

	if rl.Axis == AxisHorizontal {
		pos := cs.DataToPixel(0, rl.Value)
		py := pos.Y
		if py < plotRect.Y || py > plotRect.Y+plotRect.H {
			return nil
		}

		if rl.Dashed {
			dl, gl := resolveOverlayDash(rl.DashPattern)
			objects = append(objects, renderDashedLine(
				geo.NewPos(plotRect.X, py),
				geo.NewPos(plotRect.X+plotRect.W, py),
				c, w, dl, gl,
			)...)
		} else {
			line := &canvas.Line{
				Position1:   geo.NewPos(plotRect.X, py),
				Position2:   geo.NewPos(plotRect.X+plotRect.W, py),
				StrokeColor: c,
				StrokeWidth: w,
			}
			objects = append(objects, line)
		}

		// Label at right end
		if rl.Label != "" {
			label := canvas.NewText(rl.Label, c)
			label.TextSize = resolverLabelSize(resolver)
			labelW := float32(len(rl.Label))*7 + 4
			label.Move(geo.NewPos(plotRect.X+plotRect.W-labelW-4, py-label.TextSize-2))
			label.Resize(geo.NewSize(labelW, label.TextSize+4))
			objects = append(objects, label)
		}
	} else {
		pos := cs.DataToPixel(rl.Value, 0)
		px := pos.X
		if px < plotRect.X || px > plotRect.X+plotRect.W {
			return nil
		}

		if rl.Dashed {
			dl, gl := resolveOverlayDash(rl.DashPattern)
			objects = append(objects, renderDashedLine(
				geo.NewPos(px, plotRect.Y),
				geo.NewPos(px, plotRect.Y+plotRect.H),
				c, w, dl, gl,
			)...)
		} else {
			line := &canvas.Line{
				Position1:   geo.NewPos(px, plotRect.Y),
				Position2:   geo.NewPos(px, plotRect.Y+plotRect.H),
				StrokeColor: c,
				StrokeWidth: w,
			}
			objects = append(objects, line)
		}

		if rl.Label != "" {
			label := canvas.NewText(rl.Label, c)
			label.TextSize = resolverLabelSize(resolver)
			label.Move(geo.NewPos(px+4, plotRect.Y+2))
			label.Resize(geo.NewSize(80, label.TextSize+4))
			objects = append(objects, label)
		}
	}

	return objects
}

func renderReferenceBand(rb OverlayReferenceBand, coordSystems []coord.System, plotRect coord.Rect, resolver *style.Resolver) []core.CanvasObject {
	cs := getCoord(coordSystems, rb.YAxisIndex)
	if cs == nil {
		return nil
	}

	c := rb.Color
	if c == nil {
		c = resolver.OverlayDefaultBandColor()
	}

	var objects []core.CanvasObject

	if rb.Axis == AxisHorizontal {
		posFrom := cs.DataToPixel(0, rb.From)
		posTo := cs.DataToPixel(0, rb.To)
		y1, y2 := posFrom.Y, posTo.Y
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		// Clamp to plot area
		if y1 < plotRect.Y {
			y1 = plotRect.Y
		}
		if y2 > plotRect.Y+plotRect.H {
			y2 = plotRect.Y + plotRect.H
		}
		if y2 <= y1 {
			return nil
		}

		rect := canvas.NewRectangle(c)
		rect.Move(geo.NewPos(plotRect.X, y1))
		rect.Resize(geo.NewSize(plotRect.W, y2-y1))
		objects = append(objects, rect)

		if rb.Label != "" {
			label := canvas.NewText(rb.Label, resolver.OverlayLabelColor())
			label.TextSize = resolverLabelSize(resolver)
			label.Move(geo.NewPos(plotRect.X+4, y1+2))
			label.Resize(geo.NewSize(100, label.TextSize+4))
			objects = append(objects, label)
		}
	} else {
		posFrom := cs.DataToPixel(rb.From, 0)
		posTo := cs.DataToPixel(rb.To, 0)
		x1, x2 := posFrom.X, posTo.X
		if x1 > x2 {
			x1, x2 = x2, x1
		}
		if x1 < plotRect.X {
			x1 = plotRect.X
		}
		if x2 > plotRect.X+plotRect.W {
			x2 = plotRect.X + plotRect.W
		}
		if x2 <= x1 {
			return nil
		}

		rect := canvas.NewRectangle(c)
		rect.Move(geo.NewPos(x1, plotRect.Y))
		rect.Resize(geo.NewSize(x2-x1, plotRect.H))
		objects = append(objects, rect)

		if rb.Label != "" {
			label := canvas.NewText(rb.Label, resolver.OverlayLabelColor())
			label.TextSize = resolverLabelSize(resolver)
			label.Move(geo.NewPos(x1+4, plotRect.Y+2))
			label.Resize(geo.NewSize(100, label.TextSize+4))
			objects = append(objects, label)
		}
	}

	return objects
}

func renderAnnotation(ann OverlayAnnotation, coordSystems []coord.System, plotRect coord.Rect, resolver *style.Resolver) []core.CanvasObject {
	cs := getCoord(coordSystems, ann.YAxisIndex)
	if cs == nil {
		return nil
	}

	pos := cs.DataToPixel(ann.X, ann.Y)
	if pos.X < plotRect.X || pos.X > plotRect.X+plotRect.W ||
		pos.Y < plotRect.Y || pos.Y > plotRect.Y+plotRect.H {
		return nil
	}

	c := ann.Color
	if c == nil {
		c = resolverTitleColor(resolver)
	}
	textSize := ann.TextSize
	if textSize <= 0 {
		textSize = resolverLabelSize(resolver)
	}

	var objects []core.CanvasObject

	// Text label offset above the point
	labelW := float32(len(ann.Text))*7 + 8
	labelH := textSize + 6
	labelX := pos.X - labelW/2
	labelY := pos.Y - labelH - 8

	// Background
	bg := canvas.NewRectangle(resolver.AnnotationBackgroundColor())
	bg.CornerRadius = 3
	bg.Move(geo.NewPos(labelX, labelY))
	bg.Resize(geo.NewSize(labelW, labelH))
	objects = append(objects, bg)

	// Text
	label := canvas.NewText(ann.Text, c)
	label.TextSize = textSize
	label.Alignment = bellui.TextAlignCenter
	label.Move(geo.NewPos(labelX, labelY+2))
	label.Resize(geo.NewSize(labelW, labelH))
	objects = append(objects, label)

	// Arrow from label to point
	if ann.Arrow {
		arrow := &canvas.Line{
			Position1:   geo.NewPos(pos.X, labelY+labelH),
			Position2:   pos,
			StrokeColor: c,
			StrokeWidth: 1,
		}
		objects = append(objects, arrow)

		// Small dot at the data point
		dot := canvas.NewCircle(c)
		dot.Position1 = geo.NewPos(pos.X-3, pos.Y-3)
		dot.Position2 = geo.NewPos(pos.X+3, pos.Y+3)
		objects = append(objects, dot)
	}

	return objects
}

// resolveOverlayDash returns dash and gap lengths from a pattern, defaulting to (6, 4).
func resolveOverlayDash(pattern []float32) (dashLen, gapLen float32) {
	if len(pattern) == 0 {
		return 6, 4
	}
	dashLen = pattern[0]
	if dashLen <= 0 {
		return 6, 4
	}
	if len(pattern) >= 2 {
		gapLen = pattern[1]
	} else {
		gapLen = dashLen
	}
	return dashLen, gapLen
}

// renderDashedLine draws a dashed line as multiple short segments.
func renderDashedLine(p1, p2 geo.Position, c color.Color, width, dashLen, gapLen float32) []core.CanvasObject {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	length := float32(0)
	if dx != 0 || dy != 0 {
		length = float32(math.Sqrt(float64(dx*dx + dy*dy)))
	}
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
		seg := &canvas.Line{
			Position1:   geo.NewPos(p1.X+ux*pos, p1.Y+uy*pos),
			Position2:   geo.NewPos(p1.X+ux*end, p1.Y+uy*end),
			StrokeColor: c,
			StrokeWidth: width,
		}
		objects = append(objects, seg)
		pos = end + gapLen
	}
	return objects
}
