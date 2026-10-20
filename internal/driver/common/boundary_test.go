package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
)

// TestBoundaryGeometryFor pins where a RepaintBoundary's texture goes (#570):
// its origin snapped down to a device pixel less the margin, a size that
// covers the boundary and the margin, and a phase that a move by whole pixels
// keeps and a move by a fraction changes.
func TestBoundaryGeometryFor(t *testing.T) {
	// 2 device pixels per unit; the boundary starts 0.25 px into a pixel.
	g := BoundaryGeometryFor(geo.NewPos(10.125, 20), geo.NewSize(30, 40.25), 2)
	assert.Equal(t, geo.NewPos(9, 19), g.Origin, "floor(20.25 px) - 2 margin = 18 px = 9 units")
	assert.Equal(t, 65, g.PixW, "ceil(80.25) + 2 - 18 px")
	assert.Equal(t, 85, g.PixH, "ceil(120.5) + 2 - 38 px")
	assert.Equal(t, geo.NewSize(32.5, 42.5), g.Size)
	assert.Equal(t, BoundaryPhase{X: 64, Y: 0}, g.Phase, "0.25 px of 256 steps")

	moved := BoundaryGeometryFor(geo.NewPos(10.125+3, 20-0.5), geo.NewSize(30, 40.25), 2)
	assert.Equal(t, g.Phase, moved.Phase, "a move by whole pixels keeps the phase")
	nudged := BoundaryGeometryFor(geo.NewPos(10.375, 20), geo.NewSize(30, 40.25), 2)
	assert.NotEqual(t, g.Phase, nudged.Phase, "a move by a fraction changes it")
}
