package component

import (
	"image"
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// DecalPattern defines a pattern type for accessibility overlays.
type DecalPattern int

const (
	DecalNone     DecalPattern = iota
	DecalStripe                // diagonal stripes
	DecalDot                   // dot grid
	DecalCross                 // cross-hatch
	DecalDiagonal              // single diagonal
)

// DecalConfig configures accessibility decal patterns.
type DecalConfig struct {
	Enabled  bool
	Patterns []DecalPattern // one per series; cycles if fewer than series count
}

// PatternForSeries returns the decal pattern for a given series index.
func (d *DecalConfig) PatternForSeries(idx int) DecalPattern {
	if d == nil || !d.Enabled || len(d.Patterns) == 0 {
		return DecalNone
	}
	return d.Patterns[idx%len(d.Patterns)]
}

// RenderDecalOverlay creates a semi-transparent decal pattern raster over a rectangular area.
func RenderDecalOverlay(pattern DecalPattern, pos geo.Position, size geo.Size, c color.Color) core.CanvasObject {
	if pattern == DecalNone || size.Width <= 0 || size.Height <= 0 {
		return nil
	}

	cr, cg, cb, _ := c.RGBA()
	patColor := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 60}

	raster := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		switch pattern {
		case DecalStripe:
			drawStripes(img, w, h, patColor, 6, 4)
		case DecalDot:
			drawDots(img, w, h, patColor, 8)
		case DecalCross:
			drawStripes(img, w, h, patColor, 6, 4)
			drawStripes90(img, w, h, patColor, 6, 4)
		case DecalDiagonal:
			drawStripes(img, w, h, patColor, 10, 6)
		}
		return img
	})
	raster.Move(pos)
	raster.Resize(size)
	raster.ScaleMode = canvas.ImageScalePixels
	return raster
}

func drawStripes(img *image.NRGBA, w, h int, c color.NRGBA, spacing, width int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%spacing < width {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func drawStripes90(img *image.NRGBA, w, h int, c color.NRGBA, spacing, width int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x-y+1000*spacing)%spacing < width {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func drawDots(img *image.NRGBA, w, h int, c color.NRGBA, spacing int) {
	r := float64(spacing) / 4
	for cy := spacing / 2; cy < h; cy += spacing {
		for cx := spacing / 2; cx < w; cx += spacing {
			for y := int(float64(cy) - r); y <= int(float64(cy)+r); y++ {
				for x := int(float64(cx) - r); x <= int(float64(cx)+r); x++ {
					if x >= 0 && x < w && y >= 0 && y < h {
						dx := float64(x) - float64(cx)
						dy := float64(y) - float64(cy)
						if math.Sqrt(dx*dx+dy*dy) <= r {
							img.SetNRGBA(x, y, c)
						}
					}
				}
			}
		}
	}
}
