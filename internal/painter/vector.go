package painter

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// PathRasterBounds returns the path-local axis-aligned box a path must be
// rasterized into: its minimum corner (origin) and its size. The object's
// laid-out Size is expanded in every direction to also cover the path's own
// control-point bounding box, so a path whose segments fall outside its Size —
// e.g. a widget renderer drawing in canvas space with a zero-sized object, or
// segments reaching above/left of the object origin — still rasterizes in full.
// This matches the GL and WebGPU painters, which draw paths in full path-local
// space rather than clipping to Size. The origin is (0,0) unless the path
// reaches into negative coordinates.
func PathRasterBounds(path *canvas.Path) (origin geo.Position, size geo.Size) {
	s := path.Size()
	minX, minY, maxX, maxY := path.Bounds()
	loX, loY := min(float32(0), minX), min(float32(0), minY)
	hiX, hiY := max(s.Width, maxX), max(s.Height, maxY)
	return geo.NewPos(loX, loY), geo.NewSize(hiX-loX, hiY-loY)
}

// VectorPad returns the number of additional points that should be added around a texture.
// This is to accommodate overflow caused by stroke and line endings etc.
// THe result is in geo.Size type coordinates and should be scaled for output.
func VectorPad(obj core.CanvasObject) float32 {
	switch co := obj.(type) {
	case *canvas.Circle:
		if co.StrokeWidth > 0 && co.StrokeColor != nil {
			return co.StrokeWidth + 2
		}
		return 1 // anti-alias on circle fill
	case *canvas.Line:
		if co.StrokeWidth > 0 {
			return co.StrokeWidth + 2
		}
	case *canvas.Polygon:
		if co.StrokeWidth > 0 && co.StrokeColor != nil {
			return co.StrokeWidth + 2
		}
	case *canvas.Rectangle:
		if co.StrokeWidth > 0 && co.StrokeColor != nil {
			return co.StrokeWidth + 2
		}
	case *canvas.Text:
		if co.TextStyle.Italic {
			return co.TextSize / 5 // make sure that even a 20% lean does not overflow
		}
	case *canvas.Arc:
		if co.StrokeWidth > 0 && co.StrokeColor != nil {
			return co.StrokeWidth + 2
		}
	case *canvas.Path:
		if co.StrokeWidth > 0 && co.StrokeColor != nil {
			return co.StrokeWidth + 2
		}
	case *canvas.StyledText:
		for _, r := range co.Runs {
			if r.TextStyle.Italic {
				sz := r.TextSize
				if sz == 0 {
					sz = co.TextSize
				}
				return sz / 5
			}
		}
	case *canvas.TextOnPath:
		// Rotated glyphs can extend beyond path bounds.
		return co.TextSize / 2
	}

	return 0
}
