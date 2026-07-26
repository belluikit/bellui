package theme

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSurfaceWithElevation_Level0(t *testing.T) {
	surface := color.NRGBA{R: 0x1C, G: 0x1B, B: 0x1F, A: 0xff}
	primary := color.NRGBA{R: 0xD0, G: 0xBC, B: 0xFF, A: 0xff}

	result := SurfaceWithElevation(surface, primary, 0)
	assert.Equal(t, surface, result, "Level 0 should return surface unchanged")
}

func TestSurfaceWithElevation_Level1Tints(t *testing.T) {
	surface := color.NRGBA{R: 0x1C, G: 0x1B, B: 0x1F, A: 0xff}
	primary := color.NRGBA{R: 0xD0, G: 0xBC, B: 0xFF, A: 0xff}

	result := SurfaceWithElevation(surface, primary, 1)
	nrgba, ok := result.(color.NRGBA)
	assert.True(t, ok)

	// Level 1 applies 5% tint; result should be slightly shifted toward primary.
	assert.Greater(t, int(nrgba.R), int(surface.R), "Level 1 R should be shifted toward primary")
	assert.Equal(t, uint8(0xff), nrgba.A, "Alpha should be preserved")
}

func TestSurfaceWithElevation_IncreasingLevels(t *testing.T) {
	surface := color.NRGBA{R: 0x1C, G: 0x1B, B: 0x1F, A: 0xff}
	primary := color.NRGBA{R: 0xD0, G: 0xBC, B: 0xFF, A: 0xff}

	prev := surface
	for level := 1; level <= 5; level++ {
		result := SurfaceWithElevation(surface, primary, level).(color.NRGBA)
		assert.GreaterOrEqual(t, int(result.R), int(prev.R),
			"Level %d should be >= level %d in R channel", level, level-1)
		prev = result
	}
}

func TestSurfaceWithElevation_OutOfRange(t *testing.T) {
	surface := color.NRGBA{R: 0x50, G: 0x50, B: 0x50, A: 0xff}
	primary := color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xff}

	assert.Equal(t, surface, SurfaceWithElevation(surface, primary, -1))
	assert.Equal(t, surface, SurfaceWithElevation(surface, primary, 6))
}
