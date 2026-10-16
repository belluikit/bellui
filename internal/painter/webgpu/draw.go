//go:build !gl

package webgpu

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

const edgeSoftness = 1.0

// drawObject dispatches a canvas object to its WebGPU draw op. Ports the GL
// painter's drawObject.
//
// An op with no case here is skipped and the object never appears, with
// nothing logged — which is how TextOnPath went unnoticed until #210. The one
// primitive this painter genuinely cannot draw, canvas.GLView, therefore says
// so out loud rather than falling through (#213).
func (p *painter) drawObject(o core.CanvasObject, pos geo.Position, frame geo.Size) {
	switch obj := o.(type) {
	case *canvas.Circle:
		p.drawCircle(obj, pos, frame)
	case *canvas.Ripple:
		p.drawRipple(obj, pos, frame)
	case *canvas.Line:
		p.drawLine(obj, pos, frame)
	case *canvas.Image:
		p.drawTexture(img2obj(obj), p.rasterizeImage, pos, obj.Size(), frame, obj.FillMode, float32(obj.Alpha()))
	case *canvas.Raster:
		p.drawRaster(obj, pos, frame)
	case *canvas.Rectangle:
		p.drawRectangle(obj, pos, frame)
	case *canvas.Text:
		p.drawText(obj, pos, frame)
	case *canvas.LinearGradient:
		p.drawTexture(obj, p.rasterizeLinearGradient, pos, obj.Size(), frame, canvas.ImageFillStretch, 1.0)
	case *canvas.RadialGradient:
		p.drawTexture(obj, p.rasterizeRadialGradient, pos, obj.Size(), frame, canvas.ImageFillStretch, 1.0)
	case *canvas.Polygon:
		p.drawPolygon(obj, pos, frame)
	case *canvas.Arc:
		p.drawArc(obj, pos, frame)
	case *canvas.Path:
		p.drawPath(obj, pos, frame)
	case *canvas.StyledText:
		p.drawStyledText(obj, pos, frame)
	case *canvas.TextOnPath:
		p.drawTextOnPath(obj, pos, frame)
	case *canvas.Scene3D:
		p.drawScene3D(obj, pos, frame)
	case *canvas.GLView:
		p.reportGLViewUnsupported(obj)
	case *canvas.BoxShadow:
		p.drawBoxShadow(obj, pos, frame)
	case *canvas.BackdropBlur:
		p.drawBackdropBlur(obj, pos, frame)
	}
}

func img2obj(i *canvas.Image) core.CanvasObject { return i }

// --- geometry + color helpers (ported from internal/painter/gl/draw.go) ---

func roundToPixel(v, pixScale float32) float32 {
	if pixScale == 1.0 {
		return float32(math.Round(float64(v)))
	}
	return float32(math.Round(float64(v*pixScale))) / pixScale
}

func roundToPixelCoords(size geo.Size, pos geo.Position, pixScale float32) (geo.Size, geo.Position) {
	end := pos.Add(size)
	end.X = roundToPixel(end.X, pixScale)
	end.Y = roundToPixel(end.Y, pixScale)
	pos.X = roundToPixel(pos.X, pixScale)
	pos.Y = roundToPixel(pos.Y, pixScale)
	size.Width = end.X - pos.X
	size.Height = end.Y - pos.Y
	return size, pos
}

// fragColor returns straight RGBA in 0..1 with the group opacity applied to alpha.
func (p *painter) fragColor(col color.Color) [4]float32 {
	if col == nil {
		return [4]float32{}
	}
	r, g, b, a := col.RGBA()
	if a == 0 {
		return [4]float32{}
	}
	alpha := float32(a)
	return [4]float32{float32(r) / alpha, float32(g) / alpha, float32(b) / alpha, p.effectiveAlpha(alpha / 0xffff)}
}

func (p *painter) hasTransform() bool {
	m := p.transform
	if m.IsZeroOrIdentity() {
		return false
	}
	return m.A != 1 || m.B != 0 || m.C != 0 || m.D != 1
}

// anchor re-expresses the active world transform about the object's canvas
// origin `pos`: T(pos)·L·T(-pos), where L is the transform's linear (scale /
// rotate / skew) part. Applied to an absolute canvas corner (pos + localOffset)
// it yields pos + L·localOffset — i.e. the object scales/rotates about its own
// top-left instead of the canvas origin.
//
// This is the fix for pre-rebrand tracker #960. The paint walk hands the painter the full
// worldTransform, which already carries Translate(pos) (worldTransform =
// parentWT · Translate(objPos) · objTransform). The draw helpers below feed that
// matrix the object's *absolute* corners, so applying the raw worldTransform
// double-counts the translation (pos·(scale-1)), pushing scaled text/shapes off
// their anchor — visible as mispositioned text and off-screen rectangles on
// every GPU backend (not just Adreno). anchor cancels the doubled translation
// and matches the software painter, which splits worldTransform into its
// translation (used as the position) and linear part (used as the matrix).
func (p *painter) anchor(pos geo.Position) geo.Matrix {
	m := p.transform
	lin := geo.Matrix{A: m.A, B: m.B, C: m.C, D: m.D}
	return geo.NewTranslation(pos.X, pos.Y).Multiply(lin).Multiply(geo.NewTranslation(-pos.X, -pos.Y))
}

func (p *painter) scaleFrameSize(frame geo.Size) (float32, float32) {
	return roundToPixel(frame.Width*p.pixScale, 1.0), roundToPixel(frame.Height*p.pixScale, 1.0)
}

func (p *painter) scaleRectCoords(x1, x2, y1, y2 float32) (float32, float32, float32, float32) {
	return roundToPixel(x1*p.pixScale, 1.0), roundToPixel(x2*p.pixScale, 1.0),
		roundToPixel(y1*p.pixScale, 1.0), roundToPixel(y2*p.pixScale, 1.0)
}

// invTransformStd140 computes the inv_transform mat3 (std140-padded) that maps
// GL-convention pixel space back to untransformed pixel space for the SDF
// shaders. Identity when there is no non-trivial transform.
func (p *painter) invTransformStd140(pos geo.Position, frame geo.Size) [12]float32 {
	if !p.hasTransform() {
		return Mat3ToStd140(geo.Identity().ToGL())
	}
	// Anchor about the object origin so the inverse maps a (correctly-placed)
	// fragment back to the absolute pixel space the SDF rect_coords live in (pre-rebrand tracker #960).
	wInv, ok := p.anchor(pos).Inverse()
	if !ok {
		return Mat3ToStd140(geo.Identity().ToGL())
	}
	s := p.pixScale
	frameHPx := frame.Height * s
	g2c := geo.Matrix{A: 1 / s, D: -1 / s, TY: frameHPx / s}
	c2g := geo.Matrix{A: s, D: -s, TY: frameHPx}
	inv := c2g.Multiply(wInv).Multiply(g2c)
	return Mat3ToStd140(inv.ToGL())
}

// vecRectCoords returns the SDF rect bounds (x1,y1,x2,y2) and the 4-vertex strip
// quad (vert+normal interleaved) for a shape object.
func (p *painter) vecRectCoords(pos geo.Position, rect core.CanvasObject, frame geo.Size, aspect float32) ([4]float32, []float32) {
	xPad, yPad := float32(0), float32(0)
	if aspect != 0 {
		inner := rect.Size()
		frameAspect := inner.Width / inner.Height
		if frameAspect > aspect {
			xPad = (inner.Width - inner.Height*aspect) / 2
		} else if frameAspect < aspect {
			yPad = (inner.Height - inner.Width/aspect) / 2
		}
	}
	return p.vecRectCoordsWithPad(pos, rect, frame, xPad, yPad)
}

func (p *painter) vecRectCoordsWithPad(pos geo.Position, rect core.CanvasObject, frame geo.Size, xPad, yPad float32) ([4]float32, []float32) {
	size := rect.Size()
	pos1 := rect.Position()
	xPosDiff := pos.X - pos1.X + xPad
	yPosDiff := pos.Y - pos1.Y + yPad
	pos1.X = roundToPixel(pos1.X+xPosDiff, p.pixScale)
	pos1.Y = roundToPixel(pos1.Y+yPosDiff, p.pixScale)
	size.Width = roundToPixel(size.Width-2*xPad, p.pixScale)
	size.Height = roundToPixel(size.Height-2*yPad, p.pixScale)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	x1Pos := pos1.X
	x1Norm := -1 + (x1Pos-edgeSoftnessScaled)*2/frame.Width
	x2Pos := pos1.X + size.Width
	x2Norm := -1 + (x2Pos+edgeSoftnessScaled)*2/frame.Width
	y1Pos := pos1.Y
	y1Norm := 1 - (y1Pos-edgeSoftnessScaled)*2/frame.Height
	y2Pos := pos1.Y + size.Height
	y2Norm := 1 - (y2Pos+edgeSoftnessScaled)*2/frame.Height

	coords := []float32{
		0, 0, x1Norm, y1Norm,
		0, 0, x2Norm, y1Norm,
		0, 0, x1Norm, y2Norm,
		0, 0, x2Norm, y2Norm,
	}
	return [4]float32{x1Pos, y1Pos, x2Pos, y2Pos}, coords
}

func (p *painter) vecSquareCoords(pos geo.Position, rect core.CanvasObject, frame geo.Size) ([4]float32, []float32) {
	return p.vecRectCoordsWithPad(pos, rect, frame, 0, 0)
}

func (p *painter) transformedAABBQuad(pos geo.Position, size geo.Size, frame geo.Size) ([4]float32, []float32) {
	wt := p.anchor(pos)
	x0, y0 := wt.TransformPoint(pos.X, pos.Y)
	x1, y1 := wt.TransformPoint(pos.X+size.Width, pos.Y)
	x2, y2 := wt.TransformPoint(pos.X, pos.Y+size.Height)
	x3, y3 := wt.TransformPoint(pos.X+size.Width, pos.Y+size.Height)
	minX := geo.Min(geo.Min(x0, x1), geo.Min(x2, x3))
	minY := geo.Min(geo.Min(y0, y1), geo.Min(y2, y3))
	maxX := geo.Max(geo.Max(x0, x1), geo.Max(x2, x3))
	maxY := geo.Max(geo.Max(y0, y1), geo.Max(y2, y3))
	pad := edgeSoftness / p.pixScale
	minX, minY, maxX, maxY = minX-pad, minY-pad, maxX+pad, maxY+pad
	ndcX1 := -1 + (minX/frame.Width)*2
	ndcX2 := -1 + (maxX/frame.Width)*2
	ndcY1 := 1 - (minY/frame.Height)*2
	ndcY2 := 1 - (maxY/frame.Height)*2
	coords := []float32{
		0, 0, ndcX1, ndcY1,
		0, 0, ndcX2, ndcY1,
		0, 0, ndcX1, ndcY2,
		0, 0, ndcX2, ndcY2,
	}
	return [4]float32{minX, minY, maxX, maxY}, coords
}

// shapeQuad returns the SDF bounds and vertex strip for a shape, choosing the
// transformed AABB quad when a non-trivial transform is active.
func (p *painter) shapeQuad(o core.CanvasObject, pos geo.Position, frame geo.Size, aspect float32) ([4]float32, []float32) {
	if p.hasTransform() {
		bounds, _ := p.vecRectCoords(pos, o, frame, aspect)
		_, pts := p.transformedAABBQuad(pos, o.Size(), frame)
		return bounds, pts
	}
	return p.vecRectCoords(pos, o, frame, aspect)
}

// --- shape draw ops ---

func (p *painter) drawRectangle(rect *canvas.Rectangle, pos geo.Position, frame geo.Size) {
	trR := paint.GetCornerRadius(rect.TopRightCornerRadius, rect.CornerRadius)
	tlR := paint.GetCornerRadius(rect.TopLeftCornerRadius, rect.CornerRadius)
	brR := paint.GetCornerRadius(rect.BottomRightCornerRadius, rect.CornerRadius)
	blR := paint.GetCornerRadius(rect.BottomLeftCornerRadius, rect.CornerRadius)
	p.drawOblong(rect, rect.FillColor, rect.StrokeColor, rect.StrokeWidth, trR, tlR, brR, blR, rect.Aspect, pos, frame)
}

func (p *painter) drawOblong(obj core.CanvasObject, fill, stroke color.Color, strokeWidth, trR, tlR, brR, blR, aspect float32, pos geo.Position, frame geo.Size) {
	if (fill == color.Transparent || fill == nil) && (stroke == color.Transparent || stroke == nil || strokeWidth == 0) {
		return
	}
	rounded := trR != 0 || tlR != 0 || brR != 0 || blR != 0
	bounds, points := p.shapeQuad(obj, pos, frame, aspect)

	fw, fh := p.scaleFrameSize(frame)
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	strokeWidthScaled := roundToPixel(strokeWidth*p.pixScale, 1.0)
	fillC := p.fragColor(fill)
	strokeC := p.fragColor(orTransparent(stroke))
	inv := p.invTransformStd140(pos, frame)

	if rounded {
		size := geo.NewSize(bounds[2]-bounds[0], bounds[3]-bounds[1])
		u := RoundRectangleUniforms{
			FrameSize:       [2]float32{fw, fh},
			StrokeWidthHalf: strokeWidthScaled * 0.5,
			EdgeSoftness:    roundToPixel(edgeSoftness*p.pixScale, 1.0),
			RectCoords:      [4]float32{x1, x2, y1, y2},
			RectSizeHalf:    [2]float32{(x2 - x1 - strokeWidthScaled) * 0.5, (y2 - y1 - strokeWidthScaled) * 0.5},
			Radius: [4]float32{
				roundToPixel(paint.GetMaximumCornerRadius(trR, tlR, brR, size)*p.pixScale, 1.0),
				roundToPixel(paint.GetMaximumCornerRadius(brR, blR, trR, size)*p.pixScale, 1.0),
				roundToPixel(paint.GetMaximumCornerRadius(tlR, trR, blR, size)*p.pixScale, 1.0),
				roundToPixel(paint.GetMaximumCornerRadius(blR, brR, tlR, size)*p.pixScale, 1.0),
			},
			FillColor:    fillC,
			StrokeColor:  strokeC,
			InvTransform: inv,
		}
		p.record("round_rectangle", points, 4, uniformBytes(&u), nil)
		return
	}
	u := RectangleUniforms{
		FrameSize:    [2]float32{fw, fh},
		StrokeWidth:  strokeWidthScaled,
		RectCoords:   [4]float32{x1, x2, y1, y2},
		FillColor:    fillC,
		StrokeColor:  strokeC,
		InvTransform: inv,
	}
	p.record("rectangle", points, 4, uniformBytes(&u), nil)
}

func (p *painter) drawCircle(circle *canvas.Circle, pos geo.Position, frame geo.Size) {
	radius := paint.GetMaximumRadius(circle.Size())
	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		bounds, _ = p.vecSquareCoords(pos, circle, frame)
		_, points = p.transformedAABBQuad(pos, circle.Size(), frame)
	} else {
		bounds, points = p.vecSquareCoords(pos, circle, frame)
	}
	fw, fh := p.scaleFrameSize(frame)
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	strokeWidthScaled := roundToPixel(circle.StrokeWidth*p.pixScale, 1.0)
	rScaled := roundToPixel(radius*p.pixScale, 1.0)
	u := RoundRectangleUniforms{
		FrameSize:       [2]float32{fw, fh},
		StrokeWidthHalf: strokeWidthScaled * 0.5,
		EdgeSoftness:    roundToPixel(edgeSoftness*p.pixScale, 1.0),
		RectCoords:      [4]float32{x1, x2, y1, y2},
		RectSizeHalf:    [2]float32{(x2 - x1 - strokeWidthScaled) * 0.5, (y2 - y1 - strokeWidthScaled) * 0.5},
		Radius:          [4]float32{rScaled, rScaled, rScaled, rScaled},
		FillColor:       p.fragColor(circle.FillColor),
		StrokeColor:     p.fragColor(orTransparent(circle.StrokeColor)),
		InvTransform:    p.invTransformStd140(pos, frame),
	}
	p.record("round_rectangle", points, 4, uniformBytes(&u), nil)
}

func (p *painter) drawPolygon(polygon *canvas.Polygon, pos geo.Position, frame geo.Size) {
	if ((polygon.FillColor == color.Transparent || polygon.FillColor == nil) && (polygon.StrokeColor == color.Transparent || polygon.StrokeColor == nil || polygon.StrokeWidth == 0)) || polygon.Sides < 3 {
		return
	}
	size := polygon.Size()
	bounds, points := p.shapeQuad(polygon, pos, frame, 0)
	fw, fh := p.scaleFrameSize(frame)
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	cornerRadius := geo.Min(paint.GetMaximumRadius(size), polygon.CornerRadius)
	u := PolygonUniforms{
		FrameSize:    [2]float32{fw, fh},
		EdgeSoftness: roundToPixel(edgeSoftness*p.pixScale, 1.0),
		OuterRadius:  roundToPixel(geo.Min(size.Width, size.Height)/2*p.pixScale, 1.0),
		RectCoords:   [4]float32{x1, x2, y1, y2},
		Angle:        polygon.Angle,
		Sides:        float32(polygon.Sides),
		CornerRadius: roundToPixel(cornerRadius*p.pixScale, 1.0),
		StrokeWidth:  roundToPixel(polygon.StrokeWidth*p.pixScale, 1.0),
		FillColor:    p.fragColor(polygon.FillColor),
		StrokeColor:  p.fragColor(orTransparent(polygon.StrokeColor)),
		InvTransform: p.invTransformStd140(pos, frame),
	}
	p.record("polygon", points, 4, uniformBytes(&u), nil)
}

func (p *painter) drawArc(arc *canvas.Arc, pos geo.Position, frame geo.Size) {
	if ((arc.FillColor == color.Transparent || arc.FillColor == nil) && (arc.StrokeColor == color.Transparent || arc.StrokeColor == nil || arc.StrokeWidth == 0)) || arc.StartAngle == arc.EndAngle {
		return
	}
	bounds, points := p.shapeQuad(arc, pos, frame, 0)
	fw, fh := p.scaleFrameSize(frame)
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	outerRadius := geo.Min(arc.Size().Width, arc.Size().Height) / 2
	innerRadius := outerRadius * float32(math.Min(1.0, math.Max(0.0, float64(arc.CutoutRatio))))
	startAngle, endAngle := paint.NormalizeArcAngles(arc.StartAngle, arc.EndAngle)
	cornerRadius := geo.Min(paint.GetMaximumRadiusArc(outerRadius, innerRadius, arc.EndAngle-arc.StartAngle), arc.CornerRadius)
	u := ArcUniforms{
		FrameSize:    [2]float32{fw, fh},
		EdgeSoftness: roundToPixel(edgeSoftness*p.pixScale, 1.0),
		InnerRadius:  roundToPixel(innerRadius*p.pixScale, 1.0),
		RectCoords:   [4]float32{x1, x2, y1, y2},
		OuterRadius:  roundToPixel(outerRadius*p.pixScale, 1.0),
		StartAngle:   startAngle,
		EndAngle:     endAngle,
		CornerRadius: roundToPixel(cornerRadius*p.pixScale, 1.0),
		FillColor:    p.fragColor(arc.FillColor),
		StrokeWidth:  roundToPixel(arc.StrokeWidth*p.pixScale, 1.0),
		StrokeColor:  p.fragColor(orTransparent(arc.StrokeColor)),
		InvTransform: p.invTransformStd140(pos, frame),
	}
	p.record("arc", points, 4, uniformBytes(&u), nil)
}

// drawRipple renders an MD3 radial touch ripple clipped to a rounded rect via
// the ripple shader (premultiplied blend). Ports gl.drawRipple.
func (p *painter) drawRipple(ripple *canvas.Ripple, pos geo.Position, frame geo.Size) {
	if ripple.Opacity <= 0 || ripple.Radius <= 0 {
		return
	}
	bounds, points := p.shapeQuad(ripple, pos, frame, 0)
	fw, fh := p.scaleFrameSize(frame)
	x1, x2, y1, y2 := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])

	// straight rgb from the ripple color, alpha from Opacity (with group opacity).
	col := p.fragColor(ripple.Color)
	col[3] = p.effectiveAlpha(ripple.Opacity)

	u := RippleUniforms{
		FrameSize:    [2]float32{fw, fh},
		Radius:       roundToPixel(ripple.CornerRadius*p.pixScale, 1.0),
		EdgeSoftness: roundToPixel(edgeSoftness*p.pixScale, 1.0),
		RectCoords:   [4]float32{x1, x2, y1, y2},
		RectSizeHalf: [2]float32{(x2 - x1) * 0.5, (y2 - y1) * 0.5},
		// ripple center in framebuffer pixels (logical pos → top-down screen px).
		RippleCenter: [2]float32{p.textureScale(pos.X + ripple.Center.X), fh - p.textureScale(pos.Y+ripple.Center.Y)},
		RippleRadius: ripple.Radius * p.pixScale,
		RippleColor:  col,
		InvTransform: p.invTransformStd140(pos, frame),
	}
	p.record("ripple", points, 4, uniformBytes(&u), nil)
}

func (p *painter) drawLine(line *canvas.Line, pos geo.Position, frame geo.Size) {
	if line.StrokeColor == color.Transparent || line.StrokeColor == nil || line.StrokeWidth == 0 {
		return
	}
	points, halfWidth, feather := p.lineCoords(pos, line.Position1, line.Position2, line.StrokeWidth, 0.5, frame)
	u := LineUniforms{Color: p.fragColor(line.StrokeColor), LineWidth: halfWidth, Feather: feather}
	p.record("line", points, 6, uniformBytes(&u), nil)
}

func (p *painter) lineCoords(pos, pos1, pos2 geo.Position, lineWidth, feather float32, frame geo.Size) ([]float32, float32, float32) {
	xPosDiff := pos.X - geo.Min(pos1.X, pos2.X)
	yPosDiff := pos.Y - geo.Min(pos1.Y, pos2.Y)
	pos1.X = roundToPixel(pos1.X+xPosDiff, p.pixScale)
	pos1.Y = roundToPixel(pos1.Y+yPosDiff, p.pixScale)
	pos2.X = roundToPixel(pos2.X+xPosDiff, p.pixScale)
	pos2.Y = roundToPixel(pos2.Y+yPosDiff, p.pixScale)
	if lineWidth <= 1 {
		offset := float32(0.5)
		if lineWidth <= 0.5 && p.pixScale > 1 {
			offset = 0.25
		}
		if pos1.X == pos2.X {
			pos1.X -= offset
			pos2.X -= offset
		}
		if pos1.Y == pos2.Y {
			pos1.Y -= offset
			pos2.Y -= offset
		}
	}
	if p.hasTransform() {
		wt := p.anchor(pos)
		pos1.X, pos1.Y = wt.TransformPoint(pos1.X, pos1.Y)
		pos2.X, pos2.Y = wt.TransformPoint(pos2.X, pos2.Y)
	}
	x1 := -1 + pos1.X/frame.Width*2
	y1 := 1 - pos1.Y/frame.Height*2
	x2 := -1 + pos2.X/frame.Width*2
	y2 := 1 - pos2.Y/frame.Height*2
	normalX := (pos2.Y - pos1.Y) / frame.Width
	normalY := (pos2.X - pos1.X) / frame.Height
	dirLength := float32(math.Sqrt(float64(normalX*normalX + normalY*normalY)))
	normalX /= dirLength
	normalY /= dirLength
	normalObjX := normalX * 0.5 * frame.Width
	normalObjY := normalY * 0.5 * frame.Height
	widthMultiplier := float32(math.Sqrt(float64(normalObjX*normalObjX + normalObjY*normalObjY)))
	halfWidth := (roundToPixel(lineWidth+feather, p.pixScale) * 0.5) / widthMultiplier
	featherWidth := feather / widthMultiplier
	return []float32{
		x1, y1, normalX, normalY,
		x2, y2, normalX, normalY,
		x2, y2, -normalX, -normalY,
		x2, y2, -normalX, -normalY,
		x1, y1, normalX, normalY,
		x1, y1, -normalX, -normalY,
	}, halfWidth, featherWidth
}

// drawPath renders a canvas.Path: a stencil-based fill (winding / even-odd) and
// an anti-aliased stroke, both via the path shader. Ports gl.drawPath.
func (p *painter) drawPath(path *canvas.Path, pos geo.Position, frame geo.Size) {
	hasFill := path.FillColor != nil && path.FillColor != color.Transparent
	hasStroke := path.StrokeColor != nil && path.StrokeColor != color.Transparent && path.StrokeWidth > 0
	if !hasFill && !hasStroke {
		return
	}
	elements := path.Elements()
	if len(elements) == 0 {
		return
	}
	subpaths := paint.FlattenPath(elements, p.pixScale)
	if len(subpaths) == 0 {
		return
	}
	fringeWidth := float32(1.0) / p.pixScale

	if hasFill {
		fanTris := paint.FanTriangulate(subpaths)
		if len(fanTris) >= 6 {
			p.drawPathFillStencil(path, fanTris, subpaths, fringeWidth, pos, frame)
		}
	}

	if hasStroke {
		strokeTris := paint.ExpandStrokeAA(subpaths, path.StrokeWidth, path.LineCap, path.LineJoin,
			path.MiterLimit, path.DashPattern, path.DashOffset, fringeWidth)
		if len(strokeTris) >= 9 {
			verts := p.pathVertsToNDCWithAlpha(strokeTris, pos, frame)
			c := p.fragColor(path.StrokeColor)
			u := PathUniforms{FillColor: c}
			p.record("path", verts, len(verts)/3, uniformBytes(&u), nil)
		}
	}
}

// drawPathFillStencil renders a path fill via the stencil buffer, mirroring
// gl.drawPathFillStencil. Pass 1 stamps the fill rule's winding into the stencil
// (color off); pass 2 draws a bounding-box quad where the stencil marks the
// interior; pass 3 draws an anti-aliased fringe along the boundary (no stencil).
// The stencil is zeroed first (clearStencil) since WebGPU can't clear mid-pass.
func (p *painter) drawPathFillStencil(path *canvas.Path, fanTris []float32, subpaths [][]geo.Position, fringeWidth float32, pos geo.Position, frame geo.Size) {
	// Pass 1: winding / even-odd into the stencil, color masked off.
	p.clearStencil()
	p.dev.SetStencilRef(0)
	if path.FillRule == canvas.FillRuleEvenOdd {
		p.stencilMode = StencilWriteInvert
	} else {
		p.stencilMode = StencilWriteIncrDecr
	}
	fan := p.pathFanToNDC(fanTris, pos, frame)
	su := PathUniforms{}
	p.record("path", fan, len(fan)/3, uniformBytes(&su), nil)

	// Pass 2: bounding-box quad where the stencil interior is non-zero, color on.
	p.stencilMode = StencilTestNotEqual
	c := p.fragColor(path.FillColor)
	fu := PathUniforms{FillColor: c}
	minX, minY, maxX, maxY := path.Bounds()
	bb := p.pathVertsToNDCWithAlpha([]float32{
		minX, minY, 1, maxX, minY, 1, maxX, maxY, 1,
		minX, minY, 1, maxX, maxY, 1, minX, maxY, 1,
	}, pos, frame)
	p.record("path", bb, len(bb)/3, uniformBytes(&fu), nil)

	// Pass 3: anti-aliased fringe along the fill boundary, no stencil.
	p.stencilMode = StencilNone
	fringe := paint.ExpandFillFringe(subpaths, fringeWidth)
	if len(fringe) >= 9 {
		fr := p.pathVertsToNDCWithAlpha(fringe, pos, frame)
		p.record("path", fr, len(fr)/3, uniformBytes(&fu), nil)
	}
}

// pathFanToNDC maps flat path-local (x, y) pairs (e.g. fan-triangulated fill
// geometry) to the path shader's NDC (x, y, alpha) triples, with alpha fixed at
// 1 (the stencil winding pass ignores color, and the bounding-box fill is fully
// opaque before its own color's alpha).
func (p *painter) pathFanToNDC(fan []float32, pos geo.Position, frame geo.Size) []float32 {
	out := make([]float32, 0, len(fan)/2*3)
	wt := p.anchor(pos)
	for i := 0; i+1 < len(fan); i += 2 {
		cx := fan[i] + pos.X
		cy := fan[i+1] + pos.Y
		if p.hasTransform() {
			cx, cy = wt.TransformPoint(cx, cy)
		}
		out = append(out, -1+cx*2/frame.Width, 1-cy*2/frame.Height, 1)
	}
	return out
}

// pathVertsToNDCWithAlpha maps path-local (x, y, alpha) triples to NDC, keeping
// alpha as the third component (path vertex layout: vert.xy + alpha).
func (p *painter) pathVertsToNDCWithAlpha(verts []float32, pos geo.Position, frame geo.Size) []float32 {
	out := make([]float32, len(verts))
	wt := p.anchor(pos)
	for i := 0; i < len(verts); i += 3 {
		cx := verts[i] + pos.X
		cy := verts[i+1] + pos.Y
		if p.hasTransform() {
			cx, cy = wt.TransformPoint(cx, cy)
		}
		out[i] = -1 + cx*2/frame.Width
		out[i+1] = 1 - cy*2/frame.Height
		out[i+2] = verts[i+2]
	}
	return out
}

// --- textured (image / text / gradient / raster) ops ---

// alignedTextPos returns the draw position for a text box of content `size`
// laid out inside a container of `containerSize`, honoring the alignment
// (horizontal leading/center/trailing plus vertical centering when the container
// is taller than the content).
//
// Under a non-trivial world transform the alignment offset is carried through
// the transform's *linear* part before being added to the box origin `pos`, so
// the scale/rotation keeps pivoting on the true text-box origin rather than the
// alignment-shifted glyph position (pre-rebrand tracker #1086, follow-up to pre-rebrand tracker #960). This matches the
// software painter, which anchors at the box origin (wt.TX/TY) and applies the
// alignment offset inside the scaled bitmap. With no transform the linear part is
// the identity, so the offset is added verbatim (behavior identical to before).
func (p *painter) alignedTextPos(pos geo.Position, align bellui.TextAlign, size, containerSize geo.Size) geo.Position {
	var dx, dy float32
	switch lang.ResolveAlign(align) {
	case bellui.TextAlignTrailing:
		dx = containerSize.Width - size.Width
	case bellui.TextAlignCenter:
		dx = (containerSize.Width - size.Width) / 2
	}
	if containerSize.Height > size.Height {
		dy = (containerSize.Height - size.Height) / 2
	}
	if p.hasTransform() {
		dx, dy = p.transform.TransformVector(dx, dy)
	}
	return geo.NewPos(pos.X+dx, pos.Y+dy)
}

func (p *painter) drawText(text *canvas.Text, pos geo.Position, frame geo.Size) {
	if text.Text == "" || text.Text == " " {
		return
	}
	size := text.MinSize()
	pos = p.alignedTextPos(pos, text.Alignment, size, text.Size())
	if text.SubPixel {
		size.Width += paint.VectorPad(text)
	} else {
		size.Width = roundToPixel(size.Width, p.pixScale)
		size.Height = roundToPixel(size.Height, p.pixScale)
		size.Width += roundToPixel(paint.VectorPad(text), p.pixScale)
	}
	p.drawTexture(text, p.rasterizeText, pos, size, frame, canvas.ImageFillStretch, 1.0)
}

func (p *painter) drawStyledText(st *canvas.StyledText, pos geo.Position, frame geo.Size) {
	if len(st.Runs) == 0 {
		return
	}
	size := st.MinSize()
	pos = p.alignedTextPos(pos, st.Alignment, size, st.Size())
	size.Width = roundToPixel(size.Width, p.pixScale)
	size.Height = roundToPixel(size.Height, p.pixScale)
	size.Width += roundToPixel(paint.VectorPad(st), p.pixScale)
	p.drawTexture(st, p.rasterizeStyledText, pos, size, frame, canvas.ImageFillStretch, 1.0)
}

// glViewWarned keys the one-shot GLView warning by object, so a view in a
// per-frame repaint reports once rather than on every frame.
var glViewWarned sync.Map

// reportGLViewUnsupported explains, once per view, that canvas.GLView cannot
// render on this painter (#213).
//
// GLView hands the application a live OpenGL context — GLSL compilation,
// buffers, framebuffers, stencil, readback — and lets it draw straight into
// the window's GL framebuffer. WebGPU has no OpenGL context to hand out, so
// supporting it here would mean implementing GL-ES over wgpu rather than
// porting an op. canvas.Scene3D is the portable primitive for GPU content,
// and this painter does draw it.
//
// Saying so is the point: a silent skip is what made the same defect in
// TextOnPath (#210) cost a bug report and a bisect to find.
func (p *painter) reportGLViewUnsupported(v *canvas.GLView) {
	if _, seen := glViewWarned.LoadOrStore(v, true); seen {
		return
	}
	bellui.LogError("canvas.GLView cannot be drawn by the WebGPU painter and will not appear: it needs "+
		"an OpenGL context, which this painter does not have. Build with -tags gl for the GL painter, "+
		"or use canvas.Scene3D for portable GPU content (#213).", nil)
}

// drawTextOnPath draws glyphs laid along a path, mirroring the GL painter's
// op: the shared painter.DrawTextOnPath does the layout and rasterisation, so
// both painters and the software one produce the same image (#210).
func (p *painter) drawTextOnPath(top *canvas.TextOnPath, pos geo.Position, frame geo.Size) {
	if top.Text == "" || len(top.Elements()) == 0 {
		return
	}
	size := top.Size()
	if size.IsZero() {
		size = top.MinSize()
	}
	p.drawTexture(top, p.rasterizeTextOnPath, pos, size, frame, canvas.ImageFillStretch, 1.0)
}

// drawTexture rasterizes an object to an image, uploads it, and draws the
// textured quad via the simple shader (ports drawTextureWithDetails).
func (p *painter) drawTexture(o core.CanvasObject, raster func(core.CanvasObject) image.Image, pos geo.Position, size, frame geo.Size, fill canvas.ImageFill, alpha float32) {
	tex, ok := p.getTexture(o, raster)
	if !ok {
		return
	}
	view := tex.View()

	cornerRadius := float32(0)
	aspect := float32(0)
	if im, ok := o.(*canvas.Image); ok {
		aspect = im.Aspect()
		if aspect == 0 {
			aspect = 1
		}
		if im.CornerRadius > 0 {
			cornerRadius = im.CornerRadius
		}
	}
	points, insets := p.rectCoords(size, pos, frame, fill, aspect, 0)
	inner, _ := rectInnerCoords(size, pos, fill, aspect)

	cornerRadius = geo.Min(paint.GetMaximumRadius(size), cornerRadius)
	u := SimpleUniforms{
		Alpha:        p.effectiveAlpha(alpha),
		CornerRadius: cornerRadius * p.pixScale,
		Size:         [2]float32{inner.Width * p.pixScale, inner.Height * p.pixScale},
		Inset:        insets,
	}
	p.record("simple", points, 4, uniformBytes(&u), view)
}

// rectCoords computes the textured quad (pos.xyz + uv per vertex) and the
// texture-coordinate insets (ports the non-transform path of rectCoords). Under a
// non-trivial world transform it delegates to transformedRectCoords so textured
// ops (image/text/gradient/raster/StyledText + the Scene3D blit) honor the
// transform like the GL backend (pre-rebrand tracker #233); the `simple` shader needs no per-fragment
// inverse — the quad corners carry the transform.
func (p *painter) rectCoords(size geo.Size, pos geo.Position, frame geo.Size, fill canvas.ImageFill, aspect, pad float32) ([]float32, [4]float32) {
	if p.hasTransform() {
		return p.transformedRectCoords(size, pos, frame, fill, aspect, pad)
	}
	size, pos = rectInnerCoords(size, pos, fill, aspect)
	size, pos = roundToPixelCoords(size, pos, p.pixScale)

	x1 := -1 + (pos.X-pad)/frame.Width*2
	x2 := -1 + (pos.X+size.Width+pad)/frame.Width*2
	y1 := 1 - (pos.Y-pad)/frame.Height*2
	y2 := 1 - (pos.Y+size.Height+pad)/frame.Height*2

	insets := coverInsets(size, fill, aspect)
	return []float32{
		x1, y2, 0, insets[0], insets[3],
		x1, y1, 0, insets[0], insets[1],
		x2, y2, 0, insets[2], insets[3],
		x2, y1, 0, insets[2], insets[1],
	}, insets
}

// transformedRectCoords builds the textured quad with the four corners run
// through the world transform before NDC conversion (ports gl.transformedRectCoords).
// Unlike the untransformed path it does not pixel-snap (the transform may rotate /
// shear, so axis-aligned snapping is meaningless).
func (p *painter) transformedRectCoords(size geo.Size, pos geo.Position, frame geo.Size, fill canvas.ImageFill, aspect, pad float32) ([]float32, [4]float32) {
	// Anchor about the object's own origin (before rectInnerCoords shifts pos for
	// letterboxing) so the scale/rotation pivots on the object, not the canvas
	// origin — the letterbox inset stays a local offset that scales with it (pre-rebrand tracker #960).
	wt := p.anchor(pos)
	size, pos = rectInnerCoords(size, pos, fill, aspect)

	tlX, tlY := wt.TransformPoint(pos.X-pad, pos.Y+size.Height+pad)
	blX, blY := wt.TransformPoint(pos.X-pad, pos.Y-pad)
	trX, trY := wt.TransformPoint(pos.X+size.Width+pad, pos.Y+size.Height+pad)
	brX, brY := wt.TransformPoint(pos.X+size.Width+pad, pos.Y-pad)

	ndc := func(x, y float32) (float32, float32) {
		return -1 + (x/frame.Width)*2, 1 - (y/frame.Height)*2
	}
	tlNX, tlNY := ndc(tlX, tlY)
	blNX, blNY := ndc(blX, blY)
	trNX, trNY := ndc(trX, trY)
	brNX, brNY := ndc(brX, brY)

	insets := coverInsets(size, fill, aspect)
	return []float32{
		tlNX, tlNY, 0, insets[0], insets[3],
		blNX, blNY, 0, insets[0], insets[1],
		trNX, trNY, 0, insets[2], insets[3],
		brNX, brNY, 0, insets[2], insets[1],
	}, insets
}

// coverInsets computes the texture-coordinate insets for ImageFillCover (zero
// insets otherwise), shared by the transformed and untransformed quad builders.
func coverInsets(size geo.Size, fill canvas.ImageFill, aspect float32) [4]float32 {
	xInset, yInset := float32(0), float32(0)
	if fill == canvas.ImageFillCover {
		viewAspect := size.Width / size.Height
		if viewAspect > aspect {
			newHeight := size.Width / aspect
			yInset = (newHeight - size.Height) / 2 / newHeight
		} else if viewAspect < aspect {
			newWidth := size.Height * aspect
			xInset = (newWidth - size.Width) / 2 / newWidth
		}
	}
	return [4]float32{xInset, yInset, 1.0 - xInset, 1.0 - yInset}
}

func rectInnerCoords(size geo.Size, pos geo.Position, fill canvas.ImageFill, aspect float32) (geo.Size, geo.Position) {
	if fill == canvas.ImageFillContain || fill == canvas.ImageFillOriginal {
		viewAspect := size.Width / size.Height
		newWidth, newHeight := size.Width, size.Height
		widthPad, heightPad := float32(0), float32(0)
		if viewAspect > aspect {
			newWidth = size.Height * aspect
			widthPad = (size.Width - newWidth) / 2
		} else if viewAspect < aspect {
			newHeight = size.Width / aspect
			heightPad = (size.Height - newHeight) / 2
		}
		return geo.NewSize(newWidth, newHeight), geo.NewPos(pos.X+widthPad, pos.Y+heightPad)
	}
	return size, pos
}

// --- rasterizers (produce image.Image, mirroring gl/texture.go creators) ---

func (p *painter) rasterizeText(o core.CanvasObject) image.Image {
	text := o.(*canvas.Text)
	clr := text.Color
	if clr == nil {
		clr = theme.Color(theme.ColorNameForeground)
	}
	bounds := text.MinSize()
	width := int(math.Ceil(float64(p.textureScale(bounds.Width) + paint.VectorPad(text))))
	height := int(math.Ceil(float64(p.textureScale(bounds.Height))))
	if width <= 0 || height <= 0 {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	face := paint.CachedFontFace(text.TextStyle, text.FontSource, text)
	paint.DrawString(img, text.Text, clr, face.Fonts, text.TextSize, p.pixScale, text.TextStyle)
	return img
}

func (p *painter) rasterizeStyledText(o core.CanvasObject) image.Image {
	st := o.(*canvas.StyledText)
	clr := st.Color
	if clr == nil {
		clr = theme.Color(theme.ColorNameForeground)
	}
	bounds := st.MinSize()
	width := int(math.Ceil(float64(p.textureScale(bounds.Width) + paint.VectorPad(st))))
	height := int(math.Ceil(float64(p.textureScale(bounds.Height))))
	if width <= 0 || height <= 0 {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	paint.DrawStyledRuns(img, st.Runs, clr, st.TextSize, p.pixScale)
	return img
}

func (p *painter) rasterizeTextOnPath(o core.CanvasObject) image.Image {
	top := o.(*canvas.TextOnPath)
	img := paint.DrawTextOnPath(top, p.pixScale)
	if img == nil {
		// A nil image would otherwise reach the uploader as a typed nil
		// inside a non-nil image.Image, which is not the same as no texture.
		return nil
	}
	return img
}

func (p *painter) rasterizeImage(o core.CanvasObject) image.Image {
	img := o.(*canvas.Image)
	w := int(p.textureScale(img.Size().Width))
	h := int(p.textureScale(img.Size().Height))
	return paint.PaintImage(img, p.canvas, w, h)
}

// drawRaster draws a raster, first refreshing the cached texture in place when
// the raster is streaming (ports the GL painter's drawRaster).
func (p *painter) drawRaster(rast *canvas.Raster, pos geo.Position, frame geo.Size) {
	if rast.Streaming {
		p.updateStreamingTexture(rast)
	}
	p.drawTexture(rast, p.rasterizeRaster, pos, rast.Size(), frame, canvas.ImageFillStretch, float32(rast.Alpha()))
}

func (p *painter) rasterizeRaster(o core.CanvasObject) image.Image {
	rast := o.(*canvas.Raster)
	w := int(p.textureScale(rast.Size().Width))
	h := int(p.textureScale(rast.Size().Height))
	// PixelImage prefers SetPixels data and tolerates a nil Generator, which a
	// purely streamed raster has by design (#34).
	return rast.PixelImage(w, h)
}

func (p *painter) rasterizeLinearGradient(o core.CanvasObject) image.Image {
	g := o.(*canvas.LinearGradient)
	w, h := g.Size().Width, g.Size().Height
	switch a := g.Angle; {
	case a == 90 || a == 270:
		h = 1
	case a == 0 || a == 180:
		w = 1
	}
	return g.Generate(int(p.textureScale(w)), int(p.textureScale(h)))
}

func (p *painter) rasterizeRadialGradient(o core.CanvasObject) image.Image {
	g := o.(*canvas.RadialGradient)
	return g.Generate(int(p.textureScale(g.Size().Width)), int(p.textureScale(g.Size().Height)))
}

func (p *painter) textureScale(v float32) float32 {
	if p.pixScale == 1.0 {
		return float32(math.Round(float64(v)))
	}
	return float32(math.Round(float64(v * p.pixScale)))
}

// record submits a DrawCall to the device using the shader's default blend mode
// (and the standard fragment entry).
func (p *painter) record(shader string, verts []float32, vertexCount int, uniform []byte, texView *wgpu.TextureView) {
	desc, ok := DescriptorFor(shader)
	if !ok {
		return
	}
	p.recordWithBlend(shader, verts, vertexCount, uniform, texView, desc.DefaultBlend)
}

// recordWithBlend submits a DrawCall with an explicit blend mode, overriding the
// shader's default (e.g. a Replace blit of the canvas FBO).
func (p *painter) recordWithBlend(shader string, verts []float32, vertexCount int, uniform []byte, texView *wgpu.TextureView, blend BlendMode) {
	if _, ok := DescriptorFor(shader); !ok {
		return
	}
	_ = p.dev.Record(DrawCall{
		Key: PipelineKey{
			Shader:   shader,
			Fragment: FragmentEntry,
			Blend:    blend,
			Stencil:  p.stencilMode,
			Format:   p.dev.Format(),
		},
		Vertices:    verts,
		VertexCount: vertexCount,
		Uniform:     uniform,
		TexView:     texView,
	})
}

func orTransparent(c color.Color) color.Color {
	if c == nil {
		return color.Transparent
	}
	return c
}
