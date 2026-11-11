package software_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// TestPainter_GroupOpacity pins #1139: the software painter blends what an
// Opacity holds by its opacity, as the GL painters do, nesting multiplied. The
// red rectangle is drawn over white, so the centre pixel's green and blue are
// how much white shows through.
func TestPainter_GroupOpacity(t *testing.T) {
	for name, tc := range map[string]struct {
		opacity []float32 // outermost first
		want    color.NRGBA
	}{
		"opaque":      {[]float32{1}, color.NRGBA{R: 0xff, A: 0xff}},
		"half":        {[]float32{0.5}, color.NRGBA{R: 0xff, G: 0x80, B: 0x80, A: 0xff}},
		"transparent": {[]float32{0}, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		"nested":      {[]float32{0.5, 0.5}, color.NRGBA{R: 0xff, G: 0xbf, B: 0xbf, A: 0xff}},
	} {
		t.Run(name, func(t *testing.T) {
			c := test.NewCanvas()
			c.SetPadded(false)
			var faded core.CanvasObject = canvas.NewRectangle(color.NRGBA{R: 0xff, A: 0xff})
			for i := len(tc.opacity) - 1; i >= 0; i-- {
				faded = container.NewOpacity(faded, tc.opacity[i])
			}
			c.SetContent(container.NewStack(canvas.NewRectangle(color.White), faded))
			c.Resize(geo.NewSize(100, 100))
			img := software.NewPainter().Paint(c)

			got := color.NRGBAModel.Convert(img.At(50, 50)).(color.NRGBA)
			assert.InDelta(t, tc.want.R, got.R, 1, "red")
			assert.InDelta(t, tc.want.G, got.G, 1, "green")
			assert.InDelta(t, tc.want.B, got.B, 1, "blue")
		})
	}
}
