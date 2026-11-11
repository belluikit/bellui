package software

import (
	"image"
	"image/color"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/uidir"
)

// Painter is a simple software painter that can paint a canvas in memory.
type Painter struct{}

// NewPainter creates a new Painter.
func NewPainter() *Painter {
	return &Painter{}
}

// Paint is the main entry point for a simple software painter.
// The canvas to be drawn is passed in as a parameter and the return is an
// image containing the result of rendering.
func (*Painter) Paint(c core.Canvas) image.Image {
	bounds := image.Rect(0, 0, scale.ToScreenCoordinate(c, c.Size().Width), scale.ToScreenCoordinate(c, c.Size().Height))
	base := image.NewNRGBA(bounds)

	// Transform stack for composing parent + child transforms.
	wtStack := []geo.Matrix{geo.Identity()}

	// The group opacity of the subtree being drawn, from the core.OpacityGroup
	// objects above it, as the GL walks keep it. Disabled widgets are not
	// dimmed, as on mobile.
	var opacity driver.OpacityStack

	paint := func(obj core.CanvasObject, _, clipPos geo.Position, clipSize geo.Size) bool {
		// Scope the subtree's paint to a container DirectionScope's direction so
		// paint-time text base direction and leading/trailing alignment resolve
		// against the scope. paint always returns false, so afterPaint (which
		// pops) always runs and the push/pop pair balances.
		driver.PushDirectionScope(obj)
		opacity.Push(groupOpacity{}, obj)

		w := geo.Min(clipPos.X+clipSize.Width, c.Size().Width)
		h := geo.Min(clipPos.Y+clipSize.Height, c.Size().Height)
		clip := image.Rect(
			scale.ToScreenCoordinate(c, clipPos.X),
			scale.ToScreenCoordinate(c, clipPos.Y),
			scale.ToScreenCoordinate(c, w),
			scale.ToScreenCoordinate(c, h),
		)

		// Compose worldTransform: parent_WT * Translate(obj.Position()) * obj.Transform()
		parentWT := wtStack[len(wtStack)-1]
		objPos := obj.Position()
		localT := geo.NewTranslation(objPos.X, objPos.Y)
		if t, ok := obj.(core.Transformable); ok {
			lt := t.Transform()
			if !lt.IsIdentity() {
				localT = localT.Multiply(lt)
			}
		}
		wt := parentWT.Multiply(localT)
		wtStack = append(wtStack, wt)

		// The object is drawn from the worldTransform's translation, the image
		// of its origin: the walk position, moved by any transform on it or
		// above it, a pure translation included (#997).
		pos := geo.NewPos(wt.TX, wt.TY)

		if opacity.Transparent() {
			return false
		}
		a := opacity.Current()
		if a >= 1 {
			drawObject(c, obj, pos, wt, base, clip)
			return false
		}
		// Under a group opacity each draw is blended by it, as the GL painters
		// multiply each draw: drawn alone into a layer the size of its clip,
		// then composited through a uniform mask. Containers and widgets draw
		// nothing themselves, so they need no layer.
		switch obj.(type) {
		case *bellui.Container, core.Widget:
			return false
		}
		clip = clip.Intersect(base.Bounds())
		if clip.Empty() {
			return false
		}
		layer := image.NewNRGBA(clip)
		drawObject(c, obj, pos, wt, layer, clip)
		mask := image.NewUniform(color.Alpha{A: uint8(a*0xff + 0.5)})
		draw.DrawMask(base, clip, layer, clip.Min, mask, image.Point{}, draw.Over)
		return false
	}

	afterPaint := func(obj core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
		driver.PopDirectionScope(obj)
		opacity.Pop(groupOpacity{})
		if len(wtStack) > 1 {
			wtStack = wtStack[:len(wtStack)-1]
		}
	}

	// Scope the whole paint pass to the canvas' per-canvas direction (the
	// software-painter analogue of the GLFW/mobile paint wrap), so a window-wide
	// Canvas.SetDirection is honoured at paint time. A no-op for the default
	// DirectionAuto, or for canvases that don't implement the optional accessor.
	// Container DirectionScopes nest inside via the per-node push above.
	// Restore the stack to its pre-walk depth even if a draw panic unwinds past
	// the per-node PushDirectionScope/PopDirectionScope pairs in the walk below.
	defer uidir.Restore(uidir.Mark())
	if d, ok := c.(interface {
		Direction() bellui.Direction
	}); ok {
		uidir.Push(d.Direction())
		defer uidir.Pop(d.Direction())
	}

	driver.WalkVisibleObjectTree(c.Content(), paint, afterPaint)
	for _, o := range c.Overlays().List() {
		wtStack = wtStack[:1] // reset stack between overlays
		driver.WalkVisibleObjectTree(o, paint, afterPaint)
	}

	return base
}

// groupOpacity is the painter side of the walk's driver.OpacityStack. The walk
// reads the stack's Current itself, so there is nothing to tell.
type groupOpacity struct{}

func (groupOpacity) SetGroupOpacity(float32) {}
func (groupOpacity) ClearGroupOpacity()      {}

// drawObject draws obj at pos, under the world transform wt, onto base.
func drawObject(c core.Canvas, obj core.CanvasObject, pos geo.Position, wt geo.Matrix, base *image.NRGBA, clip image.Rectangle) {
	// Check if any non-trivial transform (rotation/scale/skew) is active.
	hasNonTrivialTransform := !(wt.A == 1 && wt.B == 0 && wt.C == 0 && wt.D == 1)

	if hasNonTrivialTransform {
		// The worldTransform's linear part is the matrix.
		linearOnly := geo.Matrix{A: wt.A, B: wt.B, C: wt.C, D: wt.D}
		drawTransformed(c, obj, pos, linearOnly, base, clip)
		return
	}

	switch o := obj.(type) {
	case *canvas.BoxShadow:
		drawBoxShadow(c, o, pos, base, clip)
	case *canvas.Image:
		drawImage(c, o, pos, base, clip)
	case *canvas.Text:
		drawText(c, o, pos, base, clip)
	case gradient:
		drawGradient(c, o, pos, base, clip)
	case *canvas.Circle:
		drawCircle(c, o, pos, base, clip)
	case *canvas.Line:
		drawLine(c, o, pos, base, clip)
	case *canvas.Polygon:
		drawPolygon(c, o, pos, base, clip)
	case *canvas.Raster:
		drawRaster(c, o, pos, base, clip)
	case *canvas.Rectangle:
		drawRectangle(c, o, pos, base, clip)
	case *canvas.Arc:
		drawArc(c, o, pos, base, clip)
	case *canvas.Scene3D:
		drawScene3D(c, o, pos, base, clip)
	case *canvas.Path:
		drawPath(c, o, pos, base, clip)
	case *canvas.StyledText:
		drawStyledText(c, o, pos, base, clip)
	case *canvas.TextOnPath:
		drawTextOnPath(c, o, pos, base, clip)
	case *canvas.GLView:
		// GLView requires a GL context; no-op in software painter.
	}
}

// drawTransformed renders an object at its local size to a temp image, then
// composites it onto base with the given affine transform (leaf only).
func drawTransformed(c core.Canvas, obj core.CanvasObject, pos geo.Position, m geo.Matrix, base *image.NRGBA, clip image.Rectangle) {
	objSize := obj.Size()
	localW := scale.ToScreenCoordinate(c, objSize.Width)
	localH := scale.ToScreenCoordinate(c, objSize.Height)
	if localW <= 0 || localH <= 0 {
		return
	}

	// Render the object at (0,0) into a temp image.
	tmp := image.NewNRGBA(image.Rect(0, 0, localW, localH))
	localClip := tmp.Bounds()
	localPos := geo.Position{}

	switch o := obj.(type) {
	case *canvas.Image:
		drawImage(c, o, localPos, tmp, localClip)
	case *canvas.Text:
		drawText(c, o, localPos, tmp, localClip)
	case gradient:
		drawGradient(c, o, localPos, tmp, localClip)
	case *canvas.Circle:
		drawCircle(c, o, localPos, tmp, localClip)
	case *canvas.Line:
		drawLine(c, o, localPos, tmp, localClip)
	case *canvas.Polygon:
		drawPolygon(c, o, localPos, tmp, localClip)
	case *canvas.Raster:
		drawRaster(c, o, localPos, tmp, localClip)
	case *canvas.Rectangle:
		drawRectangle(c, o, localPos, tmp, localClip)
	case *canvas.Arc:
		drawArc(c, o, localPos, tmp, localClip)
	case *canvas.Path:
		drawPath(c, o, localPos, tmp, localClip)
	case *canvas.StyledText:
		drawStyledText(c, o, localPos, tmp, localClip)
	case *canvas.TextOnPath:
		drawTextOnPath(c, o, localPos, tmp, localClip)
	default:
		return
	}

	// Build the affine transform in pixel coordinates.
	// Source and dest are both in pixel space (same scale), so the linear
	// part of the matrix (A,B,C,D) applies directly. The translation
	// is the object's absolute position in pixels.
	s := float64(c.Scale())
	px := float64(pos.X) * s
	py := float64(pos.Y) * s

	// f64.Aff3 is [a, b, tx, c, d, ty] (row-major).
	aff := f64.Aff3{
		float64(m.A), float64(m.C), px,
		float64(m.B), float64(m.D), py,
	}

	draw.BiLinear.Transform(base, aff, tmp, tmp.Bounds(), draw.Over, &draw.Options{
		SrcMask: nil,
		DstMask: nil,
	})
}
