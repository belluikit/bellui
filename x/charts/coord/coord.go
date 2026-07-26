// Package coord provides coordinate system abstractions for mapping data to pixels.
package coord

import (
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
)

// Rect defines a pixel-space rectangle.
type Rect struct {
	X, Y, W, H float32
}

// System maps data-space coordinates to pixel-space positions and back.
type System interface {
	// DataToPixel converts data (x, y) to pixel position within the chart area.
	DataToPixel(dataX, dataY float64) geo.Position
	// PixelToData converts a pixel position back to data coordinates.
	PixelToData(pos geo.Position) (dataX, dataY float64)
	// SetBounds sets the pixel rectangle this coordinate system occupies.
	SetBounds(bounds Rect)
	// Bounds returns the current pixel rectangle.
	Bounds() Rect
	// XScale returns the X axis scale.
	XScale() axis.Scale
	// YScale returns the Y axis scale.
	YScale() axis.Scale
}

// Cartesian2D is a standard X/Y coordinate system.
type Cartesian2D struct {
	bounds  Rect
	xScale  axis.Scale
	yScale  axis.Scale
	xInvert bool
	yInvert bool
}

// NewCartesian2D creates a Cartesian2D coordinate system with the given scales.
func NewCartesian2D(xScale, yScale axis.Scale) *Cartesian2D {
	return &Cartesian2D{xScale: xScale, yScale: yScale}
}

// SetBounds sets the pixel rectangle this coordinate system occupies.
func (c *Cartesian2D) SetBounds(bounds Rect) {
	c.bounds = bounds
}

// Bounds returns the current pixel rectangle.
func (c *Cartesian2D) Bounds() Rect {
	return c.bounds
}

// XScale returns the X axis scale.
func (c *Cartesian2D) XScale() axis.Scale {
	return c.xScale
}

// YScale returns the Y axis scale.
func (c *Cartesian2D) YScale() axis.Scale {
	return c.yScale
}

// SetInvert controls axis direction reversal.
func (c *Cartesian2D) SetInvert(xInvert, yInvert bool) {
	c.xInvert = xInvert
	c.yInvert = yInvert
}

// DataToPixel converts data (x, y) to a pixel position, flipping Y for screen coordinates.
func (c *Cartesian2D) DataToPixel(dataX, dataY float64) geo.Position {
	xNorm := c.xScale.DataToNormalized(dataX)
	yNorm := c.yScale.DataToNormalized(dataY)

	if c.xInvert {
		xNorm = 1 - xNorm
	}
	if !c.yInvert {
		// Default: Y increases upward in data but downward in pixels
		yNorm = 1 - yNorm
	}

	px := c.bounds.X + float32(xNorm)*c.bounds.W
	py := c.bounds.Y + float32(yNorm)*c.bounds.H

	return geo.NewPos(px, py)
}

// PixelToData converts a pixel position back to data (x, y) coordinates.
func (c *Cartesian2D) PixelToData(pos geo.Position) (float64, float64) {
	xNorm := float64((pos.X - c.bounds.X) / c.bounds.W)
	yNorm := float64((pos.Y - c.bounds.Y) / c.bounds.H)

	if c.xInvert {
		xNorm = 1 - xNorm
	}
	if !c.yInvert {
		yNorm = 1 - yNorm
	}

	return c.xScale.NormalizedToData(xNorm), c.yScale.NormalizedToData(yNorm)
}
