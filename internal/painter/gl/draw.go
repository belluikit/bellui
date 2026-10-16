//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/lang"
)

const edgeSoftness = 1.0

func (p *painter) createBuffer(size int) Buffer {
	vbo := p.ctx.CreateBuffer()
	p.logError()
	p.ctx.BindBuffer(arrayBuffer, vbo)
	p.logError()
	p.ctx.BufferData(arrayBuffer, make([]float32, size), staticDraw)
	p.logError()
	return vbo
}

func (p *painter) drawCircle(circle *canvas.Circle, pos geo.Position, frame geo.Size) {
	radius := paint.GetMaximumRadius(circle.Size())
	program := p.roundRectangleProgram

	// Vertex: BEG
	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		bounds, _ = p.vecSquareCoords(pos, circle, frame)
		_, points = p.transformedAABBQuad(pos, circle.Size(), frame)
	} else {
		bounds, points = p.vecSquareCoords(pos, circle, frame)
	}
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(program, "vert", 2, 4, 0)
	p.UpdateVertexArray(program, "normal", 2, 4, 2)

	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	p.uploadInvTransform(program, pos, frame)

	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(program, "frame_size", frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(program, "rect_coords", x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	strokeWidthScaled := roundToPixel(circle.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(program, "stroke_width_half", strokeWidthScaled*0.5)

	rectSizeWidthScaled := x2Scaled - x1Scaled - strokeWidthScaled
	rectSizeHeightScaled := y2Scaled - y1Scaled - strokeWidthScaled
	p.SetUniform2f(program, "rect_size_half", rectSizeWidthScaled*0.5, rectSizeHeightScaled*0.5)

	radiusScaled := roundToPixel(radius*p.pixScale, 1.0)
	p.SetUniform4f(program, "radius", radiusScaled, radiusScaled, radiusScaled, radiusScaled)

	r, g, b, a := getFragmentColor(circle.FillColor)
	p.SetUniform4f(program, "fill_color", r, g, b, a)

	strokeColor := circle.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(program, "stroke_color", r, g, b, a)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(program, "edge_softness", edgeSoftnessScaled)
	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawGradient(o core.CanvasObject, texCreator func(core.CanvasObject) Texture, pos geo.Position, frame geo.Size) {
	p.drawTextureWithDetails(o, texCreator, pos, o.Size(), frame, canvas.ImageFillStretch, 1.0, 0)
}

func (p *painter) drawImage(img *canvas.Image, pos geo.Position, frame geo.Size) {
	p.drawTextureWithDetails(img, p.newGlImageTexture, pos, img.Size(), frame, img.FillMode, float32(img.Alpha()), 0)
}

func (p *painter) drawLine(line *canvas.Line, pos geo.Position, frame geo.Size) {
	if line.StrokeColor == color.Transparent || line.StrokeColor == nil || line.StrokeWidth == 0 {
		return
	}
	points, halfWidth, feather := p.lineCoords(pos, line.Position1, line.Position2, line.StrokeWidth, 0.5, frame)
	p.ctx.UseProgram(p.lineProgram.ref)
	p.updateBuffer(p.lineProgram.buff, points)
	p.UpdateVertexArray(p.lineProgram, "vert", 2, 4, 0)
	p.UpdateVertexArray(p.lineProgram, "normal", 2, 4, 2)

	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()

	r, g, b, a := getFragmentColor(line.StrokeColor)
	p.SetUniform4f(p.lineProgram, "color", r, g, b, a)

	p.SetUniform1f(p.lineProgram, "lineWidth", halfWidth)

	p.SetUniform1f(p.lineProgram, "feather", feather)

	p.ctx.DrawArrays(triangles, 0, 6)
	p.logError()
}

func (p *painter) drawObject(o core.CanvasObject, pos geo.Position, frame geo.Size) {
	switch obj := o.(type) {
	case *canvas.Circle:
		p.drawCircle(obj, pos, frame)
	case *canvas.Line:
		p.drawLine(obj, pos, frame)
	case *canvas.Image:
		p.drawImage(obj, pos, frame)
	case *canvas.Raster:
		p.drawRaster(obj, pos, frame)
	case *canvas.Rectangle:
		p.drawRectangle(obj, pos, frame)
	case *canvas.Text:
		p.drawText(obj, pos, frame)
	case *canvas.LinearGradient:
		p.drawGradient(obj, p.newGlLinearGradientTexture, pos, frame)
	case *canvas.RadialGradient:
		p.drawGradient(obj, p.newGlRadialGradientTexture, pos, frame)
	case *canvas.Polygon:
		p.drawPolygon(obj, pos, frame)
	case *canvas.Arc:
		p.drawArc(obj, pos, frame)
	case *canvas.Ripple:
		p.drawRipple(obj, pos, frame)
	case *canvas.BoxShadow:
		p.drawBoxShadow(obj, pos, frame)
	case *canvas.BackdropBlur:
		p.drawBackdropBlur(obj, pos, frame)
	case *canvas.Scene3D:
		p.drawScene3D(obj, pos, frame)
	case *canvas.GLView:
		p.drawGLView(obj, pos, frame)
	case *canvas.Path:
		p.drawPath(obj, pos, frame)
	case *canvas.StyledText:
		p.drawStyledText(obj, pos, frame)
	case *canvas.TextOnPath:
		p.drawTextOnPath(obj, pos, frame)
	}
}

func (p *painter) drawGLView(view *canvas.GLView, pos geo.Position, frame geo.Size) {
	if view.Render == nil {
		return
	}
	size := view.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}

	// Calculate pixel coordinates with Y-flip (OpenGL origin is bottom-left).
	pixX := int(p.textureScale(pos.X))
	pixY := int(p.textureScale(frame.Height - pos.Y - size.Height))
	pixW := int(p.textureScale(size.Width))
	pixH := int(p.textureScale(size.Height))

	// Set viewport to the view's screen area.
	p.ctx.Viewport(pixX, pixY, pixW, pixH)

	adapter := &glContextAdapter{ctx: p.ctx}
	view.Render(adapter, pixW, pixH, p.pixScale)

	// Restore painter state.
	fullW := int(p.textureScale(frame.Width))
	fullH := int(p.textureScale(frame.Height))
	p.ctx.Viewport(0, 0, fullW, fullH)
	p.ctx.UseProgram(p.program.ref)
	p.ctx.BlendFunc(one, oneMinusSrcAlpha)
	p.logError()
}

func (p *painter) drawRaster(img *canvas.Raster, pos geo.Position, frame geo.Size) {
	if img.Streaming {
		p.updateStreamingTexture(img)
	}
	p.drawTextureWithDetails(img, p.newGlRasterTexture, pos, img.Size(), frame, canvas.ImageFillStretch, float32(img.Alpha()), 0)
}

func (p *painter) drawBoxShadow(shadow *canvas.BoxShadow, pos geo.Position, frame geo.Size) {
	if shadow.BlurRadius <= 0 {
		return
	}

	// The shadow is a two-pass FBO blur. On targets without framebuffer
	// support (wasm) the blur programs are uninitialised and the FBO ops are
	// stubs, so attempting the draw would panic on a nil program state. Skip
	// it — the shadowed widget still renders, just without its shadow (pre-rebrand tracker #118).
	if !framebufferSupported {
		return
	}

	// Determine the shadow area size (widget size + blur padding).
	spread := shadow.BlurRadius * 2
	shadowW := shadow.Size().Width + spread*2
	shadowH := shadow.Size().Height + spread*2

	// Create an offscreen texture with the shadow shape.
	texW := int(p.textureScale(shadowW))
	texH := int(p.textureScale(shadowH))
	if texW <= 0 || texH <= 0 {
		return
	}

	// Create FBO for shadow shape — defer cleanup so resources are freed
	// even if a panic occurs during blur or draw operations.
	shapeFBO := p.ctx.CreateFramebuffer()
	defer p.ctx.DeleteFramebuffer(shapeFBO)
	shapeTex := p.ctx.CreateTexture()
	defer p.ctx.DeleteTexture(shapeTex)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, shapeTex)
	p.ctx.TexImage2D(texture2D, 0, texW, texH, colorFormatRGBA, unsignedByte, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.BindFramebuffer(glFramebuffer, shapeFBO)
	p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, shapeTex, 0)
	p.checkFBOComplete()

	// An active window-space scissor clip would clip this offscreen FBO
	// render — suspend it for the shape + blur passes.
	restoreScissor := p.suspendScissor()
	defer restoreScissor()

	// Render a filled rounded rectangle into the FBO
	p.ctx.Viewport(0, 0, texW, texH)
	p.ctx.ClearColor(0, 0, 0, 0)
	p.ctx.Clear(bitColorBuffer)

	// Draw a solid rounded rect centered in the FBO (with padding for blur spread)
	shadowRect := &canvas.Rectangle{
		FillColor:    shadow.Color,
		CornerRadius: shadow.CornerRadius,
	}
	fboFrame := geo.NewSize(shadowW, shadowH)
	rectPos := geo.NewPos(spread, spread)
	shadowRect.Resize(shadow.Size())
	p.drawRectangle(shadowRect, rectPos, fboFrame)

	// Restore the active render target (canvas/boundary FBO or screen).
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.rebindRenderTarget(int(frameWidthScaled), int(frameHeightScaled))

	// Blur the shape texture — defer cleanup of the returned texture.
	blurRadius := shadow.BlurRadius * p.pixScale / float32(texW) * 2 // normalize to texture space
	blurredTex := p.blurTexture(shapeTex, texW, texH, blurRadius)
	defer p.ctx.DeleteTexture(blurredTex)

	// Restore the render target again — blurTexture leaves the blur-pass
	// viewport behind when no FBO target is on the stack (overlay pass).
	p.rebindRenderTarget(int(frameWidthScaled), int(frameHeightScaled))

	// Draw the blurred texture as a quad behind the widget
	shadowPos := geo.NewPos(pos.X+shadow.OffsetX-spread, pos.Y+shadow.OffsetY-spread)
	shadowSize := geo.NewSize(shadowW, shadowH)

	points, _ := p.rectCoords(shadowSize, shadowPos, frame, canvas.ImageFillStretch, 0, 0)
	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(p.program, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.program, "vertTexCoord", 2, 5, 3)
	p.SetUniform1f(p.program, "alpha", p.effectiveAlpha(1))
	p.SetUniform1f(p.program, "cornerRadius", 0)
	p.SetUniform2f(p.program, "size", 0, 0)
	p.SetUniform4f(p.program, "inset", 0, 0, 1, 1)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, blurredTex)
	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawBackdropBlur(blur *canvas.BackdropBlur, pos geo.Position, frame geo.Size) {
	if blur.BlurRadius <= 0 {
		return
	}

	// Backdrop blur captures the framebuffer and runs the FBO blur pipeline,
	// which is unavailable on the wasm target (stubbed FBO ops, uninitialised
	// blur programs). Skip rather than panic — the widget renders without the
	// frosted-glass effect (pre-rebrand tracker #118).
	if !framebufferSupported {
		return
	}

	// Calculate the screen-space rectangle to capture.
	x := int(p.textureScale(pos.X))
	frameH := p.textureScale(frame.Height)
	y := int(frameH - p.textureScale(pos.Y+blur.Size().Height)) // GL y-flip
	w := int(p.textureScale(blur.Size().Width))
	h := int(p.textureScale(blur.Size().Height))
	if w <= 0 || h <= 0 {
		return
	}

	// Copy framebuffer region directly into a texture on the GPU, avoiding
	// the ReadPixels GPU→CPU→GPU roundtrip that was the main performance
	// bottleneck (full pipeline stall per blur object per frame).
	// CopyTexSubImage2D reads from the current read framebuffer and copies
	// into the currently bound texture — no CPU memory or row-flip needed.
	captureTex := p.ctx.CreateTexture()
	defer p.ctx.DeleteTexture(captureTex)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, captureTex)
	p.ctx.TexImage2D(texture2D, 0, w, h, colorFormatRGBA, unsignedByte, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.Flush() // ensure all prior draws are flushed before reading the framebuffer
	p.ctx.CopyTexSubImage2D(texture2D, 0, 0, 0, x, y, w, h)
	p.logError()

	// Blur the captured texture. The shader expects radius in framebuffer pixels.
	blurredTex := p.blurTexture(captureTex, w, h, blur.BlurRadius*p.pixScale)
	defer p.ctx.DeleteTexture(blurredTex)

	// Restore viewport after blur.
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.ctx.Viewport(0, 0, int(frameWidthScaled), int(frameHeightScaled))

	// Draw the blurred texture as a quad at the same position.
	points, _ := p.rectCoords(blur.Size(), pos, frame, canvas.ImageFillStretch, 0, 0)
	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(p.program, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.program, "vertTexCoord", 2, 5, 3)
	p.SetUniform1f(p.program, "alpha", p.effectiveAlpha(1))
	p.SetUniform1f(p.program, "cornerRadius", 0)
	p.SetUniform2f(p.program, "size", 0, 0)
	p.SetUniform4f(p.program, "inset", 0, 0, 1, 1)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, blurredTex)
	p.ctx.BlendFunc(one, oneMinusSrcAlpha)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawRipple(ripple *canvas.Ripple, pos geo.Position, frame geo.Size) {
	if ripple.Opacity <= 0 || ripple.Radius <= 0 {
		return
	}

	program := p.rippleProgram

	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		bounds, _ = p.vecRectCoords(pos, ripple, frame, 0)
		_, points = p.transformedAABBQuad(pos, ripple.Size(), frame)
	} else {
		bounds, points = p.vecRectCoords(pos, ripple, frame, 0)
	}
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(program, "vert", 2, 4, 0)
	p.UpdateVertexArray(program, "normal", 2, 4, 2)

	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()

	p.uploadInvTransform(program, pos, frame)

	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(program, "frame_size", frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(program, "rect_coords", x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	rectSizeWidth := x2Scaled - x1Scaled
	rectSizeHeight := y2Scaled - y1Scaled
	p.SetUniform2f(program, "rect_size_half", rectSizeWidth*0.5, rectSizeHeight*0.5)

	cornerRadiusScaled := roundToPixel(ripple.CornerRadius*p.pixScale, 1.0)
	p.SetUniform1f(program, "radius", cornerRadiusScaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(program, "edge_softness", edgeSoftnessScaled)

	// Ripple center in screen pixels (convert from logical pos to framebuffer pos).
	centerX := p.textureScale(pos.X + ripple.Center.X)
	centerY := frameHeightScaled - p.textureScale(pos.Y+ripple.Center.Y)
	p.SetUniform2f(program, "ripple_center", centerX, centerY)

	rippleRadiusScaled := ripple.Radius * p.pixScale
	p.SetUniform1f(program, "ripple_radius", rippleRadiusScaled)

	r, g, b, _ := getFragmentColor(ripple.Color)
	p.SetUniform4f(program, "ripple_color", r, g, b, p.effectiveAlpha(ripple.Opacity))

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawRectangle(rect *canvas.Rectangle, pos geo.Position, frame geo.Size) {
	topRightRadius := paint.GetCornerRadius(rect.TopRightCornerRadius, rect.CornerRadius)
	topLeftRadius := paint.GetCornerRadius(rect.TopLeftCornerRadius, rect.CornerRadius)
	bottomRightRadius := paint.GetCornerRadius(rect.BottomRightCornerRadius, rect.CornerRadius)
	bottomLeftRadius := paint.GetCornerRadius(rect.BottomLeftCornerRadius, rect.CornerRadius)
	p.drawOblong(rect, rect.FillColor, rect.StrokeColor, rect.StrokeWidth, topRightRadius, topLeftRadius, bottomRightRadius, bottomLeftRadius, rect.Aspect, pos, frame)
}

func (p *painter) drawOblong(obj core.CanvasObject, fill, stroke color.Color, strokeWidth, topRightRadius, topLeftRadius, bottomRightRadius, bottomLeftRadius, aspect float32, pos geo.Position, frame geo.Size) {
	if (fill == color.Transparent || fill == nil) && (stroke == color.Transparent || stroke == nil || strokeWidth == 0) {
		return
	}

	roundedCorners := topRightRadius != 0 || topLeftRadius != 0 || bottomRightRadius != 0 || bottomLeftRadius != 0
	var program ProgramState
	if roundedCorners {
		program = p.roundRectangleProgram
	} else {
		program = p.rectangleProgram
	}

	// Vertex: BEG
	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		// Use expanded AABB quad to cover transformed footprint.
		// bounds stays at untransformed position for SDF evaluation.
		bounds, _ = p.vecRectCoords(pos, obj, frame, aspect)
		_, points = p.transformedAABBQuad(pos, obj.Size(), frame)
	} else {
		bounds, points = p.vecRectCoords(pos, obj, frame, aspect)
	}
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(program, "vert", 2, 4, 0)
	p.UpdateVertexArray(program, "normal", 2, 4, 2)

	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	p.uploadInvTransform(program, pos, frame)

	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(program, "frame_size", frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(program, "rect_coords", x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	strokeWidthScaled := roundToPixel(strokeWidth*p.pixScale, 1.0)
	if roundedCorners {
		p.SetUniform1f(program, "stroke_width_half", strokeWidthScaled*0.5)

		rectSizeWidthScaled := x2Scaled - x1Scaled - strokeWidthScaled
		rectSizeHeightScaled := y2Scaled - y1Scaled - strokeWidthScaled
		p.SetUniform2f(program, "rect_size_half", rectSizeWidthScaled*0.5, rectSizeHeightScaled*0.5)

		// the maximum possible corner radii for a circular shape, calculated taking into account the rect coords with aspect ratio
		size := geo.NewSize(bounds[2]-bounds[0], bounds[3]-bounds[1])
		topRightRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(topRightRadius, topLeftRadius, bottomRightRadius, size)*p.pixScale,
			1.0,
		)
		topLeftRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(topLeftRadius, topRightRadius, bottomLeftRadius, size)*p.pixScale,
			1.0,
		)
		bottomRightRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(bottomRightRadius, bottomLeftRadius, topRightRadius, size)*p.pixScale,
			1.0,
		)
		bottomLeftRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(bottomLeftRadius, bottomRightRadius, topLeftRadius, size)*p.pixScale,
			1.0,
		)
		p.SetUniform4f(program, "radius", topRightRadiusScaled, bottomRightRadiusScaled, topLeftRadiusScaled, bottomLeftRadiusScaled)

		edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
		p.SetUniform1f(program, "edge_softness", edgeSoftnessScaled)
	} else {
		p.SetUniform1f(program, "stroke_width", strokeWidthScaled)
	}

	r, g, b, a := getFragmentColor(fill)
	p.SetUniform4f(program, "fill_color", r, g, b, p.effectiveAlpha(a))

	strokeColor := stroke
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(program, "stroke_color", r, g, b, a)
	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawPolygon(polygon *canvas.Polygon, pos geo.Position, frame geo.Size) {
	if ((polygon.FillColor == color.Transparent || polygon.FillColor == nil) && (polygon.StrokeColor == color.Transparent || polygon.StrokeColor == nil || polygon.StrokeWidth == 0)) || polygon.Sides < 3 {
		return
	}
	size := polygon.Size()

	// Vertex: BEG
	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		bounds, _ = p.vecRectCoords(pos, polygon, frame, 0.0)
		_, points = p.transformedAABBQuad(pos, polygon.Size(), frame)
	} else {
		bounds, points = p.vecRectCoords(pos, polygon, frame, 0.0)
	}
	program := p.polygonProgram
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(program, "vert", 2, 4, 0)
	p.UpdateVertexArray(program, "normal", 2, 4, 2)

	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	p.uploadInvTransform(program, pos, frame)

	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(program, "frame_size", frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(program, "rect_coords", x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(program, "edge_softness", edgeSoftnessScaled)

	outerRadius := geo.Min(size.Width, size.Height) / 2
	outerRadiusScaled := roundToPixel(outerRadius*p.pixScale, 1.0)
	p.SetUniform1f(program, "outer_radius", outerRadiusScaled)

	p.SetUniform1f(program, "angle", polygon.Angle)
	p.SetUniform1f(program, "sides", float32(polygon.Sides))

	cornerRadius := geo.Min(paint.GetMaximumRadius(size), polygon.CornerRadius)
	cornerRadiusScaled := roundToPixel(cornerRadius*p.pixScale, 1.0)
	p.SetUniform1f(program, "corner_radius", cornerRadiusScaled)

	strokeWidthScaled := roundToPixel(polygon.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(program, "stroke_width", strokeWidthScaled)

	r, g, b, a := getFragmentColor(polygon.FillColor)
	p.SetUniform4f(program, "fill_color", r, g, b, a)

	strokeColor := polygon.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(program, "stroke_color", r, g, b, a)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawArc(arc *canvas.Arc, pos geo.Position, frame geo.Size) {
	if ((arc.FillColor == color.Transparent || arc.FillColor == nil) && (arc.StrokeColor == color.Transparent || arc.StrokeColor == nil || arc.StrokeWidth == 0)) || arc.StartAngle == arc.EndAngle {
		return
	}

	// Vertex: BEG
	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		bounds, _ = p.vecRectCoords(pos, arc, frame, 0.0)
		_, points = p.transformedAABBQuad(pos, arc.Size(), frame)
	} else {
		bounds, points = p.vecRectCoords(pos, arc, frame, 0.0)
	}
	program := p.arcProgram
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(program, "vert", 2, 4, 0)
	p.UpdateVertexArray(program, "normal", 2, 4, 2)

	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	p.uploadInvTransform(program, pos, frame)

	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(program, "frame_size", frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(program, "rect_coords", x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(program, "edge_softness", edgeSoftnessScaled)

	outerRadius := geo.Min(arc.Size().Width, arc.Size().Height) / 2
	outerRadiusScaled := roundToPixel(outerRadius*p.pixScale, 1.0)
	p.SetUniform1f(program, "outer_radius", outerRadiusScaled)

	innerRadius := outerRadius * float32(math.Min(1.0, math.Max(0.0, float64(arc.CutoutRatio))))
	innerRadiusScaled := roundToPixel(innerRadius*p.pixScale, 1.0)
	p.SetUniform1f(program, "inner_radius", innerRadiusScaled)

	startAngle, endAngle := paint.NormalizeArcAngles(arc.StartAngle, arc.EndAngle)
	p.SetUniform1f(program, "start_angle", startAngle)
	p.SetUniform1f(program, "end_angle", endAngle)

	cornerRadius := geo.Min(paint.GetMaximumRadiusArc(outerRadius, innerRadius, arc.EndAngle-arc.StartAngle), arc.CornerRadius)
	cornerRadiusScaled := roundToPixel(cornerRadius*p.pixScale, 1.0)
	p.SetUniform1f(program, "corner_radius", cornerRadiusScaled)

	strokeWidthScaled := roundToPixel(arc.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(program, "stroke_width", strokeWidthScaled)

	r, g, b, a := getFragmentColor(arc.FillColor)
	p.SetUniform4f(program, "fill_color", r, g, b, a)

	strokeColor := arc.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(program, "stroke_color", r, g, b, a)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

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
		dx, dy = p.currentTransform.TransformVector(dx, dy)
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
		// Snap texture quad to pixel grid for crisp rendering.
		size.Width = roundToPixel(size.Width, p.pixScale)
		size.Height = roundToPixel(size.Height, p.pixScale)
		size.Width += roundToPixel(paint.VectorPad(text), p.pixScale)
	}
	p.drawTextureWithDetails(text, p.newGlTextTexture, pos, size, frame, canvas.ImageFillStretch, 1.0, 0)
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
	p.drawTextureWithDetails(st, p.newGlStyledTextTexture, pos, size, frame, canvas.ImageFillStretch, 1.0, 0)
}

func (p *painter) drawTextOnPath(top *canvas.TextOnPath, pos geo.Position, frame geo.Size) {
	if top.Text == "" || len(top.Elements()) == 0 {
		return
	}

	size := top.Size()
	if size.IsZero() {
		size = top.MinSize()
	}
	p.drawTextureWithDetails(top, p.newGlTextOnPathTexture, pos, size, frame, canvas.ImageFillStretch, 1.0, 0)
}

func (p *painter) drawTextureWithDetails(o core.CanvasObject, creator func(canvasObject core.CanvasObject) Texture,
	pos geo.Position, size, frame geo.Size, fill canvas.ImageFill, alpha float32, pad float32,
) {
	texture, err := p.getTexture(o, creator)
	if err != nil {
		return
	}

	cornerRadius := float32(0)
	aspect := float32(0)
	if img, ok := o.(*canvas.Image); ok {
		aspect = img.Aspect()
		if aspect == 0 {
			aspect = 1 // fallback, should not occur - normally an image load error
		}
		if img.CornerRadius > 0 {
			cornerRadius = img.CornerRadius
		}
	}
	var points []float32
	var insets [4]float32
	if p.hasTransform() {
		points, insets = p.transformedRectCoords(size, pos, frame, fill, aspect, pad)
	} else {
		points, insets = p.rectCoords(size, pos, frame, fill, aspect, pad)
	}
	inner, _ := rectInnerCoords(size, pos, fill, aspect)

	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(p.program, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.program, "vertTexCoord", 2, 5, 3)

	// Set corner radius and texture size in pixels
	cornerRadius = geo.Min(paint.GetMaximumRadius(size), cornerRadius)
	p.SetUniform1f(p.program, "cornerRadius", cornerRadius*p.pixScale)
	p.SetUniform2f(p.program, "size", inner.Width*p.pixScale, inner.Height*p.pixScale)
	p.SetUniform4f(p.program, "inset", insets[0], insets[1], insets[2], insets[3]) // texture coordinate insets (minX, minY, maxX, maxY)

	p.SetUniform1f(p.program, "alpha", p.effectiveAlpha(alpha))

	p.ctx.BlendFunc(one, oneMinusSrcAlpha)
	p.logError()

	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, texture)
	p.logError()

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) lineCoords(pos, pos1, pos2 geo.Position, lineWidth, feather float32, frame geo.Size) ([]float32, float32, float32) {
	// Shift line coordinates so that they match the target position.
	xPosDiff := pos.X - geo.Min(pos1.X, pos2.X)
	yPosDiff := pos.Y - geo.Min(pos1.Y, pos2.Y)
	pos1.X = roundToPixel(pos1.X+xPosDiff, p.pixScale)
	pos1.Y = roundToPixel(pos1.Y+yPosDiff, p.pixScale)
	pos2.X = roundToPixel(pos2.X+xPosDiff, p.pixScale)
	pos2.Y = roundToPixel(pos2.Y+yPosDiff, p.pixScale)

	if lineWidth <= 1 {
		offset := float32(0.5)                  // adjust location for lines < 1pt on regular display
		if lineWidth <= 0.5 && p.pixScale > 1 { // and for 1px drawing on HiDPI (width 0.5)
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

	x1Pos := pos1.X / frame.Width
	x1 := -1 + x1Pos*2
	y1Pos := pos1.Y / frame.Height
	y1 := 1 - y1Pos*2
	x2Pos := pos2.X / frame.Width
	x2 := -1 + x2Pos*2
	y2Pos := pos2.Y / frame.Height
	y2 := 1 - y2Pos*2

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
		// coord x, y normal x, y
		x1, y1, normalX, normalY,
		x2, y2, normalX, normalY,
		x2, y2, -normalX, -normalY,
		x2, y2, -normalX, -normalY,
		x1, y1, normalX, normalY,
		x1, y1, -normalX, -normalY,
	}, halfWidth, featherWidth
}

// rectCoords calculates the openGL coordinate space of a rectangle
func (p *painter) rectCoords(size geo.Size, pos geo.Position, frame geo.Size,
	fill canvas.ImageFill, aspect float32, pad float32,
) ([]float32, [4]float32) {
	size, pos = rectInnerCoords(size, pos, fill, aspect)
	size, pos = roundToPixelCoords(size, pos, p.pixScale)

	xPos := (pos.X - pad) / frame.Width
	x1 := -1 + xPos*2
	x2Pos := (pos.X + size.Width + pad) / frame.Width
	x2 := -1 + x2Pos*2

	yPos := (pos.Y - pad) / frame.Height
	y1 := 1 - yPos*2
	y2Pos := (pos.Y + size.Height + pad) / frame.Height
	y2 := 1 - y2Pos*2

	xInset := float32(0.0)
	yInset := float32(0.0)

	if fill == canvas.ImageFillCover {
		viewAspect := size.Width / size.Height

		if viewAspect > aspect {
			newHeight := size.Width / aspect
			heightPad := (newHeight - size.Height) / 2
			yInset = heightPad / newHeight
		} else if viewAspect < aspect {
			newWidth := size.Height * aspect
			widthPad := (newWidth - size.Width) / 2
			xInset = widthPad / newWidth
		}
	}

	insets := [4]float32{xInset, yInset, 1.0 - xInset, 1.0 - yInset}

	return []float32{
		// coord x, y, z texture x, y
		x1, y2, 0, insets[0], insets[3], // top left
		x1, y1, 0, insets[0], insets[1], // bottom left
		x2, y2, 0, insets[2], insets[3], // top right
		x2, y1, 0, insets[2], insets[1], // bottom right
	}, insets
}

func rectInnerCoords(size geo.Size, pos geo.Position, fill canvas.ImageFill, aspect float32) (geo.Size, geo.Position) {
	if fill == canvas.ImageFillContain || fill == canvas.ImageFillOriginal {
		// change pos and size accordingly

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

func (p *painter) vecRectCoords(pos geo.Position, rect core.CanvasObject, frame geo.Size, aspect float32) ([4]float32, []float32) {
	xPad, yPad := float32(0), float32(0)

	if aspect != 0 {
		inner := rect.Size()
		frameAspect := inner.Width / inner.Height

		if frameAspect > aspect {
			newWidth := inner.Height * aspect
			xPad = (inner.Width - newWidth) / 2
		} else if frameAspect < aspect {
			newHeight := inner.Width / aspect
			yPad = (inner.Height - newHeight) / 2
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

	// without edge softness adjustment the rectangle has cropped edges
	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	x1Pos := pos1.X
	x1Norm := -1 + (x1Pos-edgeSoftnessScaled)*2/frame.Width
	x2Pos := pos1.X + size.Width
	x2Norm := -1 + (x2Pos+edgeSoftnessScaled)*2/frame.Width
	y1Pos := pos1.Y
	y1Norm := 1 - (y1Pos-edgeSoftnessScaled)*2/frame.Height
	y2Pos := pos1.Y + size.Height
	y2Norm := 1 - (y2Pos+edgeSoftnessScaled)*2/frame.Height

	// output a norm for the fill and the vert is unused, but we pass 0 to avoid optimisation issues
	coords := []float32{
		0, 0, x1Norm, y1Norm, // first triangle
		0, 0, x2Norm, y1Norm, // second triangle
		0, 0, x1Norm, y2Norm,
		0, 0, x2Norm, y2Norm,
	}

	return [4]float32{x1Pos, y1Pos, x2Pos, y2Pos}, coords
}

func (p *painter) vecSquareCoords(pos geo.Position, rect core.CanvasObject, frame geo.Size) ([4]float32, []float32) {
	return p.vecRectCoordsWithPad(pos, rect, frame, 0, 0)
}

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

	// Flatten curves to polylines.
	subpaths := paint.FlattenPath(elements, p.pixScale)
	if len(subpaths) == 0 {
		return
	}

	program := p.pathProgram
	p.ctx.UseProgram(program.ref)
	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()

	fringeWidth := float32(1.0) / p.pixScale

	// Draw fill using stencil-based fan triangulation (supports fill rules).
	if hasFill {
		fanTris := paint.FanTriangulate(subpaths)
		if len(fanTris) >= 6 {
			p.drawPathFillStencil(path, fanTris, subpaths, fringeWidth, pos, frame)
			// Re-activate the path program after stencil pass.
			p.ctx.UseProgram(program.ref)
		}
	}

	// Draw stroke with anti-aliased edges.
	if hasStroke {
		strokeTris := paint.ExpandStrokeAA(subpaths, path.StrokeWidth, path.LineCap, path.LineJoin,
			path.MiterLimit, path.DashPattern, path.DashOffset, fringeWidth)
		if len(strokeTris) >= 9 {
			ndcVerts := p.pathVertsToNDCWithAlpha(strokeTris, pos, frame)
			p.ctx.BindBuffer(arrayBuffer, program.buff)
			p.ctx.BufferData(arrayBuffer, ndcVerts, staticDraw)
			p.UpdateVertexArray(program, "vert", 2, 3, 0)
			p.UpdateVertexArray(program, "alpha", 1, 3, 2)

			r, g, b, a := getFragmentColor(path.StrokeColor)
			p.SetUniform4f(program, "fill_color", r, g, b, p.effectiveAlpha(a))
			p.logError()

			p.ctx.DrawArrays(triangles, 0, len(ndcVerts)/3)
			p.logError()
		}
	}
}

// drawPathFillStencil renders a path fill using the stencil buffer.
// Fan triangles are drawn to the stencil buffer (no color), then a bounding-box
// quad is drawn where the stencil indicates filled interior. An anti-aliased
// fringe is drawn along the fill boundary with alpha fade-off.
func (p *painter) drawPathFillStencil(path *canvas.Path, fanTris []float32, subpaths [][]geo.Position, fringeWidth float32, pos geo.Position, frame geo.Size) {
	program := p.pathProgram

	// Pass 1: Render fan triangles to stencil buffer only.
	p.ctx.Enable(stencilTest)
	p.ctx.ClearStencil(0)
	p.ctx.Clear(bitStencilBuffer)
	p.ctx.StencilMask(0xFF)
	p.ctx.ColorMask(false, false, false, false)
	p.ctx.StencilFunc(glAlways, 0, 0xFF)

	if path.FillRule == canvas.FillRuleEvenOdd {
		p.ctx.StencilOp(glKeep, glKeep, glInvert)
	} else {
		p.ctx.StencilOpSeparate(glFront, glKeep, glKeep, glIncrWrap)
		p.ctx.StencilOpSeparate(glBack, glKeep, glKeep, glDecrWrap)
	}

	// Fan triangles use stride-2 format (no alpha needed for stencil pass).
	ndcVerts := p.pathVertsToNDC(fanTris, pos, frame)
	p.ctx.BindBuffer(arrayBuffer, program.buff)
	p.ctx.BufferData(arrayBuffer, ndcVerts, staticDraw)
	p.UpdateVertexArray(program, "vert", 2, 2, 0)
	// Disable alpha attribute for stride-2 data; set constant alpha=1.
	p.ctx.VertexAttribPointerWithOffset(program.attributes["alpha"], 1, float, false, 0, 0)
	p.SetUniform4f(program, "fill_color", 0, 0, 0, 0)
	p.ctx.DrawArrays(triangles, 0, len(ndcVerts)/2)
	p.logError()

	// Pass 2: Draw bounding-box quad where stencil passes.
	p.ctx.ColorMask(true, true, true, true)
	p.ctx.StencilMask(0x00)

	if path.FillRule == canvas.FillRuleEvenOdd {
		p.ctx.StencilFunc(glNotEqual, 0, 0x01)
	} else {
		p.ctx.StencilFunc(glNotEqual, 0, 0xFF)
	}

	minX, minY, maxX, maxY := path.Bounds()
	// Bounding-box quad with alpha=1 (stride-3 format).
	bbVerts := p.pathVertsToNDCWithAlpha([]float32{
		minX, minY, 1, maxX, minY, 1, maxX, maxY, 1,
		minX, minY, 1, maxX, maxY, 1, minX, maxY, 1,
	}, pos, frame)
	p.ctx.BufferData(arrayBuffer, bbVerts, staticDraw)
	p.UpdateVertexArray(program, "vert", 2, 3, 0)
	p.UpdateVertexArray(program, "alpha", 1, 3, 2)

	r, g, b, a := getFragmentColor(path.FillColor)
	p.SetUniform4f(program, "fill_color", r, g, b, p.effectiveAlpha(a))
	p.logError()

	p.ctx.DrawArrays(triangles, 0, len(bbVerts)/3)
	p.logError()

	p.ctx.Disable(stencilTest)

	// Pass 3: Draw anti-aliased fringe along fill boundary.
	fringeVerts := paint.ExpandFillFringe(subpaths, fringeWidth)
	if len(fringeVerts) >= 9 {
		ndcFringe := p.pathVertsToNDCWithAlpha(fringeVerts, pos, frame)
		p.ctx.BufferData(arrayBuffer, ndcFringe, staticDraw)
		p.UpdateVertexArray(program, "vert", 2, 3, 0)
		p.UpdateVertexArray(program, "alpha", 1, 3, 2)

		p.SetUniform4f(program, "fill_color", r, g, b, p.effectiveAlpha(a))
		p.ctx.DrawArrays(triangles, 0, len(ndcFringe)/3)
		p.logError()
	}

	p.logError()
}

// pathVertsToNDC converts path-local vertex coordinates (flat x,y pairs) to
// normalized device coordinates for GL rendering. Path coordinates map directly
// into the object's local space (offset by the object's position on the canvas).
func (p *painter) pathVertsToNDC(verts []float32, pos geo.Position, frame geo.Size) []float32 {
	out := make([]float32, len(verts))
	wt := p.anchor(pos)
	for i := 0; i < len(verts); i += 2 {
		cx := verts[i] + pos.X
		cy := verts[i+1] + pos.Y
		if p.hasTransform() {
			cx, cy = wt.TransformPoint(cx, cy)
		}
		out[i] = -1 + cx*2/frame.Width
		out[i+1] = 1 - cy*2/frame.Height
	}
	return out
}

// pathVertsToNDCWithAlpha converts path-local vertex coordinates with per-vertex
// alpha (x, y, alpha triples) to NDC. The alpha value is passed through unchanged.
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
		out[i+2] = verts[i+2] // alpha passthrough
	}
	return out
}

func roundToPixel(v float32, pixScale float32) float32 {
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

// Returns FragmentColor(red,green,blue,alpha) from bellui.Color
func getFragmentColor(col color.Color) (float32, float32, float32, float32) {
	if col == nil {
		return 0, 0, 0, 0
	}
	r, g, b, a := col.RGBA()
	if a == 0 {
		return 0, 0, 0, 0
	}
	alpha := float32(a)
	return float32(r) / alpha, float32(g) / alpha, float32(b) / alpha, alpha / 0xffff
}

func (p *painter) scaleFrameSize(frame geo.Size) (float32, float32) {
	frameWidthScaled := roundToPixel(frame.Width*p.pixScale, 1.0)
	frameHeightScaled := roundToPixel(frame.Height*p.pixScale, 1.0)
	return frameWidthScaled, frameHeightScaled
}

// Returns scaled RectCoords(x1,x2,y1,y2) in same order
func (p *painter) scaleRectCoords(x1, x2, y1, y2 float32) (float32, float32, float32, float32) {
	x1Scaled := roundToPixel(x1*p.pixScale, 1.0)
	x2Scaled := roundToPixel(x2*p.pixScale, 1.0)
	y1Scaled := roundToPixel(y1*p.pixScale, 1.0)
	y2Scaled := roundToPixel(y2*p.pixScale, 1.0)
	return x1Scaled, x2Scaled, y1Scaled, y2Scaled
}

// hasTransform reports whether a non-trivial (non-translation) transform is
// active. Pure translations are already handled by the pos parameter in the
// paint walk, so only rotation, scale, or skew need special treatment.
func (p *painter) hasTransform() bool {
	m := p.currentTransform
	if m.IsZeroOrIdentity() {
		return false
	}
	// Check if the transform is a pure translation (A=1, B=0, C=0, D=1).
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
// their anchor. anchor cancels the doubled translation and matches the software
// painter, which splits worldTransform into its translation (used as the
// position) and linear part (used as the matrix).
func (p *painter) anchor(pos geo.Position) geo.Matrix {
	m := p.currentTransform
	lin := geo.Matrix{A: m.A, B: m.B, C: m.C, D: m.D}
	return geo.NewTranslation(pos.X, pos.Y).Multiply(lin).Multiply(geo.NewTranslation(-pos.X, -pos.Y))
}

// uploadInvTransform computes and uploads the inv_transform mat3 uniform
// for the given SDF program. The matrix maps GL pixel-space gl_FragCoord
// back to untransformed pixel-space so the SDF evaluates correctly.
//
// The formula is: inv_transform = C2G * W_inv * G2C
//
//	where C2G converts canvas→GL pixel coords,
//	      G2C converts GL pixel→canvas coords,
//	      W_inv is the inverse of the world transform (in canvas space).
func (p *painter) uploadInvTransform(program ProgramState, pos geo.Position, frame geo.Size) {
	if !p.hasTransform() {
		p.SetUniformMatrix3fv(program, "inv_transform", geo.Identity().ToGL())
		return
	}

	// Anchor about the object origin so the inverse maps a (correctly-placed)
	// fragment back to the absolute pixel space the SDF rect_coords live in (pre-rebrand tracker #960).
	wInv, ok := p.anchor(pos).Inverse()
	if !ok {
		// Non-invertible transform — skip rendering.
		p.SetUniformMatrix3fv(program, "inv_transform", geo.Identity().ToGL())
		return
	}

	s := p.pixScale
	frameHPx := frame.Height * s

	// G2C: GL pixel space (y-up) → canvas space (y-down)
	//   canvas_x = gl_x / s
	//   canvas_y = (frameHPx - gl_y) / s = frameH - gl_y/s
	g2c := geo.Matrix{A: 1 / s, D: -1 / s, TY: frameHPx / s}

	// C2G: canvas space (y-down) → GL pixel space (y-up)
	//   gl_x = canvas_x * s
	//   gl_y = frameHPx - canvas_y * s
	c2g := geo.Matrix{A: s, D: -s, TY: frameHPx}

	inv := c2g.Multiply(wInv).Multiply(g2c)
	p.SetUniformMatrix3fv(program, "inv_transform", inv.ToGL())
}

// transformedAABBQuad returns an expanded NDC quad covering the AABB of the
// transformed rectangle, for use as the vertex quad of SDF objects.
// Returns the AABB bounds [x1,y1,x2,y2] in canvas space and the NDC vertex data.
func (p *painter) transformedAABBQuad(pos geo.Position, size geo.Size, frame geo.Size) ([4]float32, []float32) {
	wt := p.anchor(pos)

	// Transform the 4 corners of the object rect from canvas space.
	x0, y0 := wt.TransformPoint(pos.X, pos.Y)
	x1, y1 := wt.TransformPoint(pos.X+size.Width, pos.Y)
	x2, y2 := wt.TransformPoint(pos.X, pos.Y+size.Height)
	x3, y3 := wt.TransformPoint(pos.X+size.Width, pos.Y+size.Height)

	minX := geo.Min(geo.Min(x0, x1), geo.Min(x2, x3))
	minY := geo.Min(geo.Min(y0, y1), geo.Min(y2, y3))
	maxX := geo.Max(geo.Max(x0, x1), geo.Max(x2, x3))
	maxY := geo.Max(geo.Max(y0, y1), geo.Max(y2, y3))

	// Add edge softness padding.
	pad := edgeSoftness / p.pixScale
	minX -= pad
	minY -= pad
	maxX += pad
	maxY += pad

	// Convert AABB to NDC.
	ndcX1 := -1 + (minX/frame.Width)*2
	ndcX2 := -1 + (maxX/frame.Width)*2
	ndcY1 := 1 - (minY/frame.Height)*2
	ndcY2 := 1 - (maxY/frame.Height)*2

	bounds := [4]float32{minX, minY, maxX, maxY}
	coords := []float32{
		0, 0, ndcX1, ndcY1,
		0, 0, ndcX2, ndcY1,
		0, 0, ndcX1, ndcY2,
		0, 0, ndcX2, ndcY2,
	}

	return bounds, coords
}

// transformedRectCoords computes transformed quad vertices in NDC for texture-based
// objects. Returns vertex data (pos.xyz + texcoord.uv per vertex) and texture insets.
func (p *painter) transformedRectCoords(size geo.Size, pos geo.Position, frame geo.Size,
	fill canvas.ImageFill, aspect float32, pad float32,
) ([]float32, [4]float32) {
	// Anchor about the object's own origin (before rectInnerCoords shifts pos for
	// letterboxing) so the scale/rotation pivots on the object, not the canvas
	// origin — the letterbox inset stays a local offset that scales with it (pre-rebrand tracker #960).
	wt := p.anchor(pos)
	size, pos = rectInnerCoords(size, pos, fill, aspect)

	// Compute 4 corners in canvas space and transform.
	tlX, tlY := wt.TransformPoint(pos.X-pad, pos.Y+size.Height+pad)
	blX, blY := wt.TransformPoint(pos.X-pad, pos.Y-pad)
	trX, trY := wt.TransformPoint(pos.X+size.Width+pad, pos.Y+size.Height+pad)
	brX, brY := wt.TransformPoint(pos.X+size.Width+pad, pos.Y-pad)

	// Convert to NDC.
	tlNX := -1 + (tlX/frame.Width)*2
	tlNY := 1 - (tlY/frame.Height)*2
	blNX := -1 + (blX/frame.Width)*2
	blNY := 1 - (blY/frame.Height)*2
	trNX := -1 + (trX/frame.Width)*2
	trNY := 1 - (trY/frame.Height)*2
	brNX := -1 + (brX/frame.Width)*2
	brNY := 1 - (brY/frame.Height)*2

	// Compute texture insets (same as untransformed path).
	xInset := float32(0.0)
	yInset := float32(0.0)
	if fill == canvas.ImageFillCover {
		viewAspect := size.Width / size.Height
		if viewAspect > aspect {
			newHeight := size.Width / aspect
			heightPad := (newHeight - size.Height) / 2
			yInset = heightPad / newHeight
		} else if viewAspect < aspect {
			newWidth := size.Height * aspect
			widthPad := (newWidth - size.Width) / 2
			xInset = widthPad / newWidth
		}
	}
	insets := [4]float32{xInset, yInset, 1.0 - xInset, 1.0 - yInset}

	return []float32{
		// top left
		tlNX, tlNY, 0, insets[0], insets[3],
		// bottom left
		blNX, blNY, 0, insets[0], insets[1],
		// top right
		trNX, trNY, 0, insets[2], insets[3],
		// bottom right
		brNX, brNY, 0, insets[2], insets[1],
	}, insets
}
