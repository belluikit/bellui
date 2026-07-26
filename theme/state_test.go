package theme

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateLayerHover(t *testing.T) {
	c := StateLayerHover(color.White)
	r, g, b, a := c.RGBA()
	assert.Greater(t, a, uint32(0), "Hover layer should not be transparent")
	assert.Greater(t, r, uint32(0))
	assert.Greater(t, g, uint32(0))
	assert.Greater(t, b, uint32(0))

	// 8% of 255 = ~20
	nrgba := c.(color.NRGBA)
	assert.InDelta(t, 20, int(nrgba.A), 2, "Hover alpha should be ~8%%")
}

func TestStateLayerFocus(t *testing.T) {
	c := StateLayerFocus(color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff})
	nrgba := c.(color.NRGBA)

	assert.Equal(t, uint8(0xff), nrgba.R)
	assert.Equal(t, uint8(0x00), nrgba.G)
	// 12% of 255 = ~30
	assert.InDelta(t, 30, int(nrgba.A), 2, "Focus alpha should be ~12%%")
}

func TestStateLayerPressed(t *testing.T) {
	c := StateLayerPressed(color.NRGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff})
	nrgba := c.(color.NRGBA)

	assert.Equal(t, uint8(0x00), nrgba.R)
	assert.Equal(t, uint8(0xff), nrgba.G)
	// 12% of 255 = ~30
	assert.InDelta(t, 30, int(nrgba.A), 2, "Pressed alpha should be ~12%%")
}

func TestStateLayerDragged(t *testing.T) {
	c := StateLayerDragged(color.White)
	nrgba := c.(color.NRGBA)

	// 16% of 255 = ~40
	assert.InDelta(t, 40, int(nrgba.A), 2, "Dragged alpha should be ~16%%")
}
