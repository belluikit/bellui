//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"image"
	"image/color"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

type captureImage struct {
	pix           []uint8
	width, height int
}

func (c *captureImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (c *captureImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, c.width, c.height)
}

func (c *captureImage) At(x, y int) color.Color {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return color.RGBA{}
	}
	start := ((c.height-y-1)*c.width + x) * 4
	return color.RGBA{R: c.pix[start], G: c.pix[start+1], B: c.pix[start+2], A: c.pix[start+3]}
}

func (p *painter) Capture(c core.Canvas) image.Image {
	return p.capture(c, front)
}

// CaptureBack reads the back buffer instead of the front one. The driver uses
// it for canvas capture (#473): it paints a fresh frame and reads it before
// any swap, so the readback deterministically contains that exact frame —
// overlays and menus included — instead of racing the presentation pipeline.
func (p *painter) CaptureBack(c core.Canvas) image.Image {
	return p.capture(c, back)
}

func (p *painter) capture(c core.Canvas, buffer uint32) image.Image {
	pos := geo.NewPos(c.Size().Width, c.Size().Height)
	width, height := c.PixelCoordinateForPosition(pos)
	pixels := make([]uint8, width*height*4)

	p.contextProvider.RunWithContext(func() {
		p.ctx.ReadBuffer(buffer)
		p.logError()
		p.ctx.ReadPixels(0, 0, width, height, colorFormatRGBA, unsignedByte, pixels)
		p.logError()
	})

	return &captureImage{
		pix:    pixels,
		width:  width,
		height: height,
	}
}
