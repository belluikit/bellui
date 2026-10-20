package common

import (
	"math"

	"github.com/belluikit/bellui/geo"
)

// boundaryMarginPx is how many device pixels a RepaintBoundary's texture
// extends past the boundary on each side: anti-aliased edges reach about a
// pixel past an object's geometry, and an object flush with the boundary lost
// that fringe to the texture's edge (#570).
const boundaryMarginPx = 2

// boundaryPhaseSteps is how finely BoundaryPhase tells sub-pixel offsets
// apart: the 1/256 pixel GPUs place geometry at, so offsets they rasterize
// alike compare equal.
const boundaryPhaseSteps = 256

// BoundaryPhase is the sub-pixel offset, in 1/boundaryPhaseSteps of a device
// pixel, of a RepaintBoundary's content from the pixel grid.
type BoundaryPhase struct{ X, Y int32 }

// BoundaryGeometry is where a RepaintBoundary is rendered and drawn.
type BoundaryGeometry struct {
	// Origin is the texture's top-left corner, where it is blitted: the
	// boundary's position snapped down to a device pixel, less the margin.
	Origin geo.Position
	// PixW and PixH are the texture's size in device pixels, covering the
	// boundary and the margin around it.
	PixW, PixH int
	// Size is the texture's size in canvas units.
	Size geo.Size
	// Phase is how far the boundary's content sits from the pixel grid.
	Phase BoundaryPhase
}

// BoundaryGeometryFor places a RepaintBoundary at pos with size, on a canvas
// of pxPerUnit device pixels per unit. Rendering its content at pos - Origin
// keeps the sub-pixel offset it would have drawn directly, and blitting the
// texture at Origin is a copy, pixel for pixel. A texture placed at pos
// instead drew its content on another sub-pixel grid and resampled it, so
// edges came out differently from drawing in place (#570).
func BoundaryGeometryFor(pos geo.Position, size geo.Size, pxPerUnit float32) BoundaryGeometry {
	x, y := float64(pos.X*pxPerUnit), float64(pos.Y*pxPerUnit)
	fx, fy := x-math.Floor(x), y-math.Floor(y)
	ox, oy := math.Floor(x)-boundaryMarginPx, math.Floor(y)-boundaryMarginPx
	pixW := int(math.Ceil(float64((pos.X+size.Width)*pxPerUnit)) + boundaryMarginPx - ox)
	pixH := int(math.Ceil(float64((pos.Y+size.Height)*pxPerUnit)) + boundaryMarginPx - oy)
	return BoundaryGeometry{
		Origin: geo.NewPos(float32(ox)/pxPerUnit, float32(oy)/pxPerUnit),
		PixW:   pixW,
		PixH:   pixH,
		Size:   geo.NewSize(float32(pixW)/pxPerUnit, float32(pixH)/pxPerUnit),
		Phase: BoundaryPhase{
			X: int32(math.Round(fx * boundaryPhaseSteps)),
			Y: int32(math.Round(fy * boundaryPhaseSteps)),
		},
	}
}

// BoundaryRenderedAt reports whether node's RepaintBoundary texture was
// rendered at phase. A clean texture rendered at another phase holds its
// content on another sub-pixel grid, so it must be rendered again.
func (r *RenderCacheNode) BoundaryRenderedAt(phase BoundaryPhase) bool {
	return r.boundaryPhaseSet && r.boundaryPhase == phase
}

// SetBoundaryRenderedAt records the phase node's RepaintBoundary texture was
// rendered at.
func (r *RenderCacheNode) SetBoundaryRenderedAt(phase BoundaryPhase) {
	r.boundaryPhase, r.boundaryPhaseSet = phase, true
}
