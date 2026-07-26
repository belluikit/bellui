package coord

import (
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
)

// None is a pass-through coordinate system for chart types that handle
// their own layout (pie, gauge, treemap, etc.).
type None struct {
	bounds Rect
}

// NewNone creates a None coordinate system.
func NewNone() *None {
	return &None{}
}

// SetBounds stores the pixel rectangle (used for viewport queries only).
func (n *None) SetBounds(bounds Rect) { n.bounds = bounds }

// Bounds returns the stored pixel rectangle.
func (n *None) Bounds() Rect { return n.bounds }

// DataToPixel treats data values as raw pixel coordinates (identity mapping).
func (n *None) DataToPixel(dataX, dataY float64) geo.Position {
	return geo.NewPos(float32(dataX), float32(dataY))
}

// PixelToData treats pixel coordinates as raw data values (identity mapping).
func (n *None) PixelToData(pos geo.Position) (float64, float64) {
	return float64(pos.X), float64(pos.Y)
}

// XScale returns nil; None has no axis scales.
func (n *None) XScale() axis.Scale { return nil }

// YScale returns nil; None has no axis scales.
func (n *None) YScale() axis.Scale { return nil }
