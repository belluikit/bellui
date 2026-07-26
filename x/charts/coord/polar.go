package coord

import (
	"math"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
)

// Polar maps (radius, angle) data to pixel positions in a circular area.
type Polar struct {
	bounds      Rect
	radiusScale axis.Scale
	angleScale  axis.Scale
	centerX     float32
	centerY     float32
	maxRadius   float32
}

// NewPolar creates a Polar coordinate system.
func NewPolar(radiusScale, angleScale axis.Scale) *Polar {
	return &Polar{radiusScale: radiusScale, angleScale: angleScale}
}

// SetBounds sets the pixel rectangle and computes the center and max radius.
func (p *Polar) SetBounds(bounds Rect) {
	p.bounds = bounds
	p.centerX = bounds.X + bounds.W/2
	p.centerY = bounds.Y + bounds.H/2
	p.maxRadius = bounds.W / 2
	if bounds.H/2 < p.maxRadius {
		p.maxRadius = bounds.H / 2
	}
}

// Bounds returns the current pixel rectangle.
func (p *Polar) Bounds() Rect { return p.bounds }

// DataToPixel maps (radius, angleDegrees) to pixel position.
// radius is normalized [0, 1] mapped through radiusScale; angle is in degrees (0=top, clockwise).
func (p *Polar) DataToPixel(radius, angleDeg float64) geo.Position {
	rNorm := p.radiusScale.DataToNormalized(radius)
	r := float64(p.maxRadius) * rNorm
	// Convert angle: 0=top, clockwise -> standard math angle
	rad := (angleDeg - 90) * math.Pi / 180
	px := float64(p.centerX) + r*math.Cos(rad)
	py := float64(p.centerY) + r*math.Sin(rad)
	return geo.NewPos(float32(px), float32(py))
}

// PixelToData converts a pixel position back to (radius, angleDegrees).
func (p *Polar) PixelToData(pos geo.Position) (float64, float64) {
	dx := float64(pos.X - p.centerX)
	dy := float64(pos.Y - p.centerY)
	r := math.Sqrt(dx*dx + dy*dy)
	rNorm := r / float64(p.maxRadius)
	angleDeg := math.Atan2(dy, dx)*180/math.Pi + 90
	if angleDeg < 0 {
		angleDeg += 360
	}
	return p.radiusScale.NormalizedToData(rNorm), angleDeg
}

// XScale returns the angle scale.
func (p *Polar) XScale() axis.Scale { return p.angleScale }

// YScale returns the radius scale.
func (p *Polar) YScale() axis.Scale { return p.radiusScale }
