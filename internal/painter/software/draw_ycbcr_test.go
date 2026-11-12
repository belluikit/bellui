package software_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/internal/ycbcr/ycbcrtest"
	"github.com/belluikit/bellui/test"
)

// The software painter draws a raster's Y'CbCr colour bars in the right
// colours, by the matrix the raster was given (#1107).
func TestPainter_paintYCbCrRasterDrawsTheColourBars(t *testing.T) {
	const barW, h = 8, 8
	for _, tc := range []struct {
		name   string
		bars   []ycbcrtest.Bar
		matrix canvas.YCbCrMatrix
	}{
		{"BT.601 limited", ycbcrtest.BT601Limited, canvas.YCbCrBT601},
		{"BT.709 limited", ycbcrtest.BT709Limited, canvas.YCbCrBT709},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := len(tc.bars) * barW
			obj := canvas.NewRaster(nil)
			obj.Streaming = true
			obj.SetYCbCr(ycbcrtest.Frame(tc.bars, barW, h), tc.matrix, false)

			c := test.NewCanvas()
			c.SetPadded(false)
			c.SetContent(obj)
			c.Resize(geo.NewSize(float32(w), h))

			ycbcrtest.AssertBars(t, tc.bars, barW, software.NewPainter().Paint(c), 2)
		})
	}
}
