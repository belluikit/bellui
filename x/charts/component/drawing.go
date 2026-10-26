package component

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/style"
)

const (
	drawingLineWidth     = float32(1.5)
	drawingSelectedWidth = float32(2.5)
	handleSize           = float32(6) // drag handle square side
	fibLabelWidth        = float32(60)
)

// RenderDrawings produces canvas objects for all committed drawings.
func RenderDrawings(
	drawings []interact.Drawing,
	selectedID interact.DrawingID,
	coordSystems []coord.System,
	plotRect coord.Rect,
	resolver *style.Resolver,
) []core.CanvasObject {
	if len(coordSystems) == 0 {
		return nil
	}
	cs := coordSystems[0]
	var objects []core.CanvasObject

	for _, d := range drawings {
		selected := d.ID() == selectedID
		objects = append(objects, renderDrawing(d, selected, cs, plotRect, resolver)...)
	}
	return objects
}

func resolveDrawingColor(d interact.Drawing, resolver *style.Resolver) color.Color {
	c := d.DrawingColor()
	if c != nil {
		return c
	}
	if resolver != nil {
		return resolver.OverlayDefaultLineColor()
	}
	return color.NRGBA{R: 91, G: 143, B: 249, A: 220}
}

func renderDrawing(d interact.Drawing, selected bool, cs coord.System, plotRect coord.Rect, resolver *style.Resolver) []core.CanvasObject {
	c := resolveDrawingColor(d, resolver)
	w := drawingLineWidth
	if selected {
		w = drawingSelectedWidth
	}

	var objects []core.CanvasObject

	switch dt := d.(type) {
	case *interact.HLineDrawing:
		objects = renderHLine(dt, c, w, cs, plotRect)
	case *interact.VLineDrawing:
		objects = renderVLine(dt, c, w, cs, plotRect)
	case *interact.TrendLineDrawing:
		objects = renderTrendLine(dt, c, w, cs, plotRect)
	case *interact.RayDrawing:
		objects = renderRay(dt, c, w, cs, plotRect)
	case *interact.RectangleDrawing:
		objects = renderRectangle(dt, c, w, cs, plotRect)
	case *interact.ChannelDrawing:
		objects = renderChannel(dt, c, w, cs, plotRect)
	case *interact.FibRetracementDrawing:
		objects = renderFibRetracement(dt, c, w, cs, plotRect)
	case *interact.TextLabelDrawing:
		objects = renderTextLabel(dt, c, cs, plotRect, resolver)
	}

	// Draw drag handles for selected drawings.
	if selected {
		objects = append(objects, renderHandles(d, cs, c)...)
	}

	// Draw label if set.
	if lbl := d.DrawingLabel(); lbl != "" {
		pts := d.DataPoints()
		if len(pts) > 0 {
			pos := cs.DataToPixel(pts[0].X, pts[0].Y)
			label := canvas.NewText(lbl, c)
			label.TextSize = 10
			label.Move(geo.NewPos(pos.X+4, pos.Y-14))
			label.Resize(geo.NewSize(80, 14))
			objects = append(objects, label)
		}
	}

	return objects
}

// --- Per-type renderers ---

func renderHLine(d *interact.HLineDrawing, c color.Color, w float32, cs coord.System, pr coord.Rect) []core.CanvasObject {
	py := cs.DataToPixel(0, d.Y).Y
	if py < pr.Y || py > pr.Y+pr.H {
		return nil
	}
	line := &canvas.Line{
		Position1:   geo.NewPos(pr.X, py),
		Position2:   geo.NewPos(pr.X+pr.W, py),
		StrokeColor: c,
		StrokeWidth: w,
	}
	return []core.CanvasObject{line}
}

func renderVLine(d *interact.VLineDrawing, c color.Color, w float32, cs coord.System, pr coord.Rect) []core.CanvasObject {
	px := cs.DataToPixel(d.X, 0).X
	if px < pr.X || px > pr.X+pr.W {
		return nil
	}
	line := &canvas.Line{
		Position1:   geo.NewPos(px, pr.Y),
		Position2:   geo.NewPos(px, pr.Y+pr.H),
		StrokeColor: c,
		StrokeWidth: w,
	}
	return []core.CanvasObject{line}
}

func renderTrendLine(d *interact.TrendLineDrawing, c color.Color, w float32, cs coord.System, _ coord.Rect) []core.CanvasObject {
	p1 := cs.DataToPixel(d.Start.X, d.Start.Y)
	p2 := cs.DataToPixel(d.End.X, d.End.Y)
	line := &canvas.Line{
		Position1:   p1,
		Position2:   p2,
		StrokeColor: c,
		StrokeWidth: w,
	}
	return []core.CanvasObject{line}
}

func renderRay(d *interact.RayDrawing, c color.Color, w float32, cs coord.System, pr coord.Rect) []core.CanvasObject {
	p1 := cs.DataToPixel(d.Start.X, d.Start.Y)
	p2 := cs.DataToPixel(d.Direction.X, d.Direction.Y)

	// Extend the ray to the plot boundary.
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	if dx == 0 && dy == 0 {
		return nil
	}

	// Find t where the ray exits the plot rect.
	tMax := float32(100000) // large default
	if dx > 0 {
		t := (pr.X + pr.W - p1.X) / dx
		if t < tMax {
			tMax = t
		}
	} else if dx < 0 {
		t := (pr.X - p1.X) / dx
		if t < tMax {
			tMax = t
		}
	}
	if dy > 0 {
		t := (pr.Y + pr.H - p1.Y) / dy
		if t < tMax {
			tMax = t
		}
	} else if dy < 0 {
		t := (pr.Y - p1.Y) / dy
		if t < tMax {
			tMax = t
		}
	}
	endX := p1.X + dx*tMax
	endY := p1.Y + dy*tMax

	line := &canvas.Line{
		Position1:   p1,
		Position2:   geo.NewPos(endX, endY),
		StrokeColor: c,
		StrokeWidth: w,
	}
	return []core.CanvasObject{line}
}

func renderRectangle(d *interact.RectangleDrawing, c color.Color, w float32, cs coord.System, _ coord.Rect) []core.CanvasObject {
	p1 := cs.DataToPixel(d.Corner1.X, d.Corner1.Y)
	p2 := cs.DataToPixel(d.Corner2.X, d.Corner2.Y)

	minX, maxX := p1.X, p2.X
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := p1.Y, p2.Y
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	// Semi-transparent fill.
	cr, cg, cb, _ := c.RGBA()
	fillColor := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 30}
	fill := canvas.NewRectangle(fillColor)
	fill.Move(geo.NewPos(minX, minY))
	fill.Resize(geo.NewSize(maxX-minX, maxY-minY))

	// Border.
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = c
	border.StrokeWidth = w
	border.Move(geo.NewPos(minX, minY))
	border.Resize(geo.NewSize(maxX-minX, maxY-minY))

	return []core.CanvasObject{fill, border}
}

func renderChannel(d *interact.ChannelDrawing, c color.Color, w float32, cs coord.System, _ coord.Rect) []core.CanvasObject {
	p1 := cs.DataToPixel(d.Line1Start.X, d.Line1Start.Y)
	p2 := cs.DataToPixel(d.Line1End.X, d.Line1End.Y)
	p3 := cs.DataToPixel(d.Line1Start.X, d.Line1Start.Y+d.OffsetY)
	p4 := cs.DataToPixel(d.Line1End.X, d.Line1End.Y+d.OffsetY)

	line1 := &canvas.Line{Position1: p1, Position2: p2, StrokeColor: c, StrokeWidth: w}
	line2 := &canvas.Line{Position1: p3, Position2: p4, StrokeColor: c, StrokeWidth: w}

	// Fill the parallelogram between the two lines using vertical strips.
	// Fyne has no polygon primitive, so we approximate with thin rectangles
	// that follow the slope of the channel.
	cr, cg, cb, _ := c.RGBA()
	fillColor := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 20}

	var objects []core.CanvasObject

	startX := min32(p1.X, p2.X)
	endX := max32(p1.X, p2.X)
	spanX := endX - startX
	if spanX < 1 {
		spanX = 1
	}

	// Use strips of ~2px width for smooth fill.
	stripW := float32(2)
	for x := startX; x < endX; x += stripW {
		t := (x - p1.X) / (p2.X - p1.X)
		if p2.X == p1.X {
			t = 0
		}
		// Y positions on line1 and line2 at this X.
		y1 := p1.Y + t*(p2.Y-p1.Y)
		y2 := p3.Y + t*(p4.Y-p3.Y)
		topY := min32(y1, y2)
		botY := max32(y1, y2)
		sw := stripW
		if x+sw > endX {
			sw = endX - x
		}
		strip := canvas.NewRectangle(fillColor)
		strip.Move(geo.NewPos(x, topY))
		strip.Resize(geo.NewSize(sw, botY-topY))
		objects = append(objects, strip)
	}

	objects = append(objects, line1, line2)
	return objects
}

func renderFibRetracement(d *interact.FibRetracementDrawing, c color.Color, w float32, cs coord.System, pr coord.Rect) []core.CanvasObject {
	span := d.HighY - d.LowY
	if span == 0 {
		return nil
	}

	var objects []core.CanvasObject
	for _, level := range interact.FibLevels {
		y := d.LowY + span*level
		py := cs.DataToPixel(0, y).Y
		if py < pr.Y || py > pr.Y+pr.H {
			continue
		}

		// Dashed line.
		objects = append(objects, renderDashedLine(
			geo.NewPos(pr.X, py), geo.NewPos(pr.X+pr.W, py),
			c, w, 6, 4,
		)...)

		// Level label.
		pct := level * 100
		text := fmt.Sprintf("%.1f%% (%.1f)", pct, y)
		label := canvas.NewText(text, c)
		label.TextSize = 9
		label.Move(geo.NewPos(pr.X+pr.W-fibLabelWidth-4, py-12))
		label.Resize(geo.NewSize(fibLabelWidth, 12))
		objects = append(objects, label)
	}
	return objects
}

func renderTextLabel(d *interact.TextLabelDrawing, c color.Color, cs coord.System, pr coord.Rect, resolver *style.Resolver) []core.CanvasObject {
	pos := cs.DataToPixel(d.Pos.X, d.Pos.Y)
	if pos.X < pr.X || pos.X > pr.X+pr.W || pos.Y < pr.Y || pos.Y > pr.Y+pr.H {
		return nil
	}

	text := d.Text
	if text == "" {
		text = "Label"
	}
	textSize := float32(12)
	labelW := float32(len(text))*7 + 8
	labelH := textSize + 6

	// Background: the annotation chip of the theme, under text in the
	// drawing's colour. It was a fixed dark grey on every theme (#343).
	bg := canvas.NewRectangle(resolver.AnnotationBackgroundColor())
	bg.CornerRadius = 3
	bg.Move(geo.NewPos(pos.X, pos.Y-labelH))
	bg.Resize(geo.NewSize(labelW, labelH))

	// Text.
	label := canvas.NewText(text, c)
	label.TextSize = textSize
	label.Move(geo.NewPos(pos.X+4, pos.Y-labelH+2))
	label.Resize(geo.NewSize(labelW-8, labelH))

	return []core.CanvasObject{bg, label}
}

// --- Selection handles ---

func renderHandles(d interact.Drawing, cs coord.System, c color.Color) []core.CanvasObject {
	pts := d.DataPoints()
	var objects []core.CanvasObject
	half := handleSize / 2

	for _, pt := range pts {
		pos := cs.DataToPixel(pt.X, pt.Y)
		handle := canvas.NewRectangle(c)
		handle.Move(geo.NewPos(pos.X-half, pos.Y-half))
		handle.Resize(geo.NewSize(handleSize, handleSize))
		objects = append(objects, handle)
	}
	return objects
}

// --- Drawing preview (ghost in-progress) ---

// RenderDrawingPreview produces canvas objects for an in-progress drawing gesture.
// cursorPos is the current mouse position in pixel space.
func RenderDrawingPreview(
	gesture *interact.DrawingGestureState,
	cursorPos geo.Position,
	coordSystems []coord.System,
	plotRect coord.Rect,
	resolver *style.Resolver,
) []core.CanvasObject {
	if gesture == nil || !gesture.Active || len(coordSystems) == 0 {
		return nil
	}
	cs := coordSystems[0]
	c := gesture.Color
	if c == nil {
		c = color.NRGBA{R: 91, G: 143, B: 249, A: 140}
	}
	w := float32(1.0)

	switch gesture.Tool {
	case interact.DrawingToolTrendLine, interact.DrawingToolRay:
		if gesture.Step >= 1 {
			p1 := cs.DataToPixel(gesture.Points[0].X, gesture.Points[0].Y)
			line := &canvas.Line{Position1: p1, Position2: cursorPos, StrokeColor: c, StrokeWidth: w}
			return []core.CanvasObject{line}
		}
	case interact.DrawingToolRectangle:
		if gesture.Step >= 1 {
			p1 := cs.DataToPixel(gesture.Points[0].X, gesture.Points[0].Y)
			return renderPreviewRect(p1, cursorPos, c)
		}
	case interact.DrawingToolChannel:
		if gesture.Step == 1 {
			p1 := cs.DataToPixel(gesture.Points[0].X, gesture.Points[0].Y)
			line := &canvas.Line{Position1: p1, Position2: cursorPos, StrokeColor: c, StrokeWidth: w}
			return []core.CanvasObject{line}
		}
		if gesture.Step == 2 {
			p1 := cs.DataToPixel(gesture.Points[0].X, gesture.Points[0].Y)
			p2 := cs.DataToPixel(gesture.Points[1].X, gesture.Points[1].Y)
			// Compute offset from the primary line at the cursor's X position,
			// not from the start point. This makes the preview match the cursor.
			lineYAtCursor := p1.Y
			if p2.X != p1.X {
				t := (cursorPos.X - p1.X) / (p2.X - p1.X)
				lineYAtCursor = p1.Y + t*(p2.Y-p1.Y)
			}
			dy := cursorPos.Y - lineYAtCursor
			p3 := geo.NewPos(p1.X, p1.Y+dy)
			p4 := geo.NewPos(p2.X, p2.Y+dy)
			line1 := &canvas.Line{Position1: p1, Position2: p2, StrokeColor: c, StrokeWidth: w}
			line2 := &canvas.Line{Position1: p3, Position2: p4, StrokeColor: c, StrokeWidth: w}
			return []core.CanvasObject{line1, line2}
		}
	case interact.DrawingToolFibRetracement:
		if gesture.Step >= 1 {
			highPy := cs.DataToPixel(0, gesture.Points[0].Y).Y
			// Preview as horizontal line at start + cursor position.
			line1 := &canvas.Line{
				Position1: geo.NewPos(plotRect.X, highPy), Position2: geo.NewPos(plotRect.X+plotRect.W, highPy),
				StrokeColor: c, StrokeWidth: w,
			}
			line2 := &canvas.Line{
				Position1: geo.NewPos(plotRect.X, cursorPos.Y), Position2: geo.NewPos(plotRect.X+plotRect.W, cursorPos.Y),
				StrokeColor: c, StrokeWidth: w,
			}
			return []core.CanvasObject{line1, line2}
		}
	case interact.DrawingToolTextLabel:
		if gesture.Step >= 1 {
			pos := cs.DataToPixel(gesture.Points[0].X, gesture.Points[0].Y)
			text := gesture.TextBuf
			if text == "" {
				text = "|"
			} else {
				text += "|"
			}
			label := canvas.NewText(text, c)
			label.TextSize = 12
			label.Move(geo.NewPos(pos.X+4, pos.Y-16))
			label.Resize(geo.NewSize(200, 16))
			bg := canvas.NewRectangle(resolver.AnnotationBackgroundColor()) // as the finished label
			bg.CornerRadius = 3
			bg.Move(geo.NewPos(pos.X, pos.Y-18))
			bg.Resize(geo.NewSize(float32(len(text))*7+12, 20))
			return []core.CanvasObject{bg, label}
		}
	}
	return nil
}

func renderPreviewRect(p1, p2 geo.Position, c color.Color) []core.CanvasObject {
	minX, maxX := p1.X, p2.X
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := p1.Y, p2.Y
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	cr, cg, cb, _ := c.RGBA()
	fillColor := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 25}
	fill := canvas.NewRectangle(fillColor)
	fill.Move(geo.NewPos(minX, minY))
	fill.Resize(geo.NewSize(maxX-minX, maxY-minY))

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = c
	border.StrokeWidth = 1
	border.Move(geo.NewPos(minX, minY))
	border.Resize(geo.NewSize(maxX-minX, maxY-minY))

	return []core.CanvasObject{fill, border}
}

// --- Measure overlay ---

// RenderMeasureOverlay produces canvas objects for the temporary measure tool.
func RenderMeasureOverlay(
	start, end geo.Position,
	coordSystems []coord.System,
) []core.CanvasObject {
	if len(coordSystems) == 0 {
		return nil
	}
	cs := coordSystems[0]

	c := color.NRGBA{R: 200, G: 200, B: 200, A: 200}

	// Dashed line between points.
	objects := renderDashedLine(start, end, c, 1, 4, 4)

	// Convert to data space for delta calculation.
	startX, startY := cs.PixelToData(start)
	endX, endY := cs.PixelToData(end)

	deltaX := endX - startX
	deltaY := endY - startY
	pctChange := float64(0)
	if startY != 0 {
		pctChange = (deltaY / startY) * 100
	}

	text := fmt.Sprintf("ΔX:%.1f  ΔY:%.2f  (%.1f%%)", deltaX, deltaY, pctChange)

	// Label box at midpoint.
	midX := (start.X + end.X) / 2
	midY := (start.Y + end.Y) / 2
	labelW := float32(len(text))*6.5 + 12
	labelH := float32(16)

	bg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 35, A: 230})
	bg.CornerRadius = 3
	bg.Move(geo.NewPos(midX-labelW/2, midY-labelH-4))
	bg.Resize(geo.NewSize(labelW, labelH))

	label := canvas.NewText(text, c)
	label.TextSize = 10
	label.Move(geo.NewPos(midX-labelW/2+4, midY-labelH-2))
	label.Resize(geo.NewSize(labelW, labelH))

	objects = append(objects, bg, label)

	// Small crosshairs at start and end.
	chSize := float32(4)
	for _, pos := range []geo.Position{start, end} {
		h := &canvas.Line{Position1: geo.NewPos(pos.X-chSize, pos.Y), Position2: geo.NewPos(pos.X+chSize, pos.Y), StrokeColor: c, StrokeWidth: 1}
		v := &canvas.Line{Position1: geo.NewPos(pos.X, pos.Y-chSize), Position2: geo.NewPos(pos.X, pos.Y+chSize), StrokeColor: c, StrokeWidth: 1}
		objects = append(objects, h, v)
	}

	return objects
}

// --- Helpers ---

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// suppress unused import warning for math
var _ = math.MaxFloat32
